package queue

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/omnivore-app/omnivore/internal/bullmq"
	"github.com/redis/go-redis/v9"
)

// savePageJobData mirrors the TS SavePageJobData from job.ts.
type savePageJobData struct {
	UserID                 string       `json:"userId"`
	URL                    string       `json:"url"`
	FinalURL               string       `json:"finalUrl"`
	ArticleSavingRequestID string       `json:"articleSavingRequestId"`
	State                  *string      `json:"state,omitempty"`
	Labels                 []LabelInput `json:"labels,omitempty"`
	Source                 string       `json:"source"`
	Folder                 *string      `json:"folder,omitempty"`
	RSSFeedURL             *string      `json:"rssFeedUrl,omitempty"`
	SavedAt                *string      `json:"savedAt,omitempty"`
	PublishedAt            *string      `json:"publishedAt,omitempty"`
	TaskID                 *string      `json:"taskId,omitempty"`
	Title                  string       `json:"title,omitempty"`
	ContentType            string       `json:"contentType,omitempty"`
	CacheKey               string       `json:"cacheKey,omitempty"`
}

// handleSavePage processes content that was fetched by the content-fetcher.
// It retrieves the cached content from Redis, then saves/updates the library item.
func (w *BackendWorker) handleSavePage(job *bullmq.RawJob) error {
	data, err := unmarshalJobData[savePageJobData](job)
	if err != nil {
		return fmt.Errorf("unmarshal save-page data: %w", err)
	}

	slog.Info("save-page", "url", data.URL, "userId", data.UserID, "itemId", data.ArticleSavingRequestID)

	// Verify user exists
	var userExists bool
	if err := w.db.Read.WithContext(w.ctx).
		Raw("SELECT EXISTS(SELECT 1 FROM omnivore.user WHERE id = ? AND status = 'ACTIVE')", data.UserID).
		Scan(&userExists).Error; err != nil {
		return fmt.Errorf("check user: %w", err)
	}
	if !userExists {
		slog.Warn("save-page: user not found, skipping", "userId", data.UserID)
		return nil // Don't retry
	}

	// Retrieve cached content from Redis
	var content string
	if data.CacheKey != "" {
		val, err := w.redis.CacheClient.Get(w.ctx, data.CacheKey).Result()
		if err == nil {
			var cached struct {
				Content string `json:"content"`
			}
			if json.Unmarshal([]byte(val), &cached) == nil {
				content = cached.Content
			}
		} else if err != redis.Nil {
			slog.Error("save-page: cache read error", "err", err, "key", data.CacheKey)
		}
	}

	// Download content from object storage if not in cache
	if content == "" && w.store != nil {
		contentPath := fmt.Sprintf("%s/content.html", data.ArticleSavingRequestID)
		data, err := w.store.Download(w.ctx, contentPath)
		if err != nil {
			slog.Warn("save-page: storage download failed", "err", err, "path", contentPath)
		} else {
			content = string(data)
		}
	}

	// Determine folder
	folder := "inbox"
	if data.Folder != nil && *data.Folder != "" {
		folder = *data.Folder
	}

	// Determine saved-at timestamp
	savedAt := time.Now()
	if data.SavedAt != nil && *data.SavedAt != "" {
		if t, err := time.Parse(time.RFC3339, *data.SavedAt); err == nil {
			savedAt = t
		}
	}

	// Determine published-at timestamp
	var publishedAt *time.Time
	if data.PublishedAt != nil && *data.PublishedAt != "" {
		if t, err := time.Parse(time.RFC3339, *data.PublishedAt); err == nil {
			publishedAt = &t
		}
	}

	// Update the library item with fetched content
	state := "SUCCEEDED"
	if data.State != nil && *data.State != "" {
		state = *data.State
	}

	updates := map[string]any{
		"state":            state,
		"readable_content": content,
		"original_url":     data.FinalURL,
		"updated_at":       time.Now(),
	}
	if data.Title != "" {
		updates["title"] = data.Title
	}
	if data.ContentType != "" {
		updates["content_reader"] = contentReaderFromType(data.ContentType)
	}
	if publishedAt != nil {
		updates["published_at"] = publishedAt
	}

	result := w.db.Write.WithContext(w.ctx).
		Table("omnivore.library_item").
		Where("id = ? AND user_id = ?", data.ArticleSavingRequestID, data.UserID).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update library item: %w", result.Error)
	}

	// If it's a new RSS item, also update the saved_at
	if data.RSSFeedURL != nil && *data.RSSFeedURL != "" {
		w.db.Write.WithContext(w.ctx).
			Table("omnivore.library_item").
			Where("id = ?", data.ArticleSavingRequestID).
			Updates(map[string]any{
				"saved_at": savedAt,
				"folder":   folder,
			})
	}

	// Set labels if provided
	if len(data.Labels) > 0 {
		w.setLabelsForSavedPage(data.UserID, data.ArticleSavingRequestID, data.Labels)
	}

	slog.Info("save-page completed", "itemId", data.ArticleSavingRequestID)
	return nil
}

// setLabelsForSavedPage creates labels if needed and assigns them to the library item.
func (w *BackendWorker) setLabelsForSavedPage(userID, itemID string, labels []LabelInput) {
	for _, label := range labels {
		// Find or create the label
		var labelID string
		err := w.db.Read.WithContext(w.ctx).
			Raw("SELECT id FROM omnivore.labels WHERE user_id = ? AND name = ?", userID, label.Name).
			Scan(&labelID).Error
		if err != nil || labelID == "" {
			// Create the label
			color := "#07D2D1" // default color
			if label.Color != "" {
				color = label.Color
			}
			result := w.db.Write.WithContext(w.ctx).
				Exec("INSERT INTO omnivore.labels (id, user_id, name, color, source) VALUES (gen_random_uuid(), ?, ?, ?, 'system') ON CONFLICT (user_id, name) DO NOTHING RETURNING id",
					userID, label.Name, color)
			if result.Error != nil {
				slog.Error("save-page: create label", "err", result.Error, "name", label.Name)
				continue
			}
			// Re-read the label ID
			w.db.Read.WithContext(w.ctx).
				Raw("SELECT id FROM omnivore.labels WHERE user_id = ? AND name = ?", userID, label.Name).
				Scan(&labelID)
			if labelID == "" {
				continue
			}
		}

		// Create the entity_label association
		w.db.Write.WithContext(w.ctx).
			Exec("INSERT INTO omnivore.entity_labels (id, label_id, library_item_id, source) VALUES (gen_random_uuid(), ?, ?, 'system') ON CONFLICT DO NOTHING",
				labelID, itemID)
	}

	// Trigger label denormalization update
	w.db.Write.WithContext(w.ctx).
		Exec(`UPDATE omnivore.library_item
			SET label_names = COALESCE((
				SELECT array_agg(DISTINCT l.name)
				FROM omnivore.labels l
				INNER JOIN omnivore.entity_labels el ON el.label_id = l.id AND el.library_item_id = ?
			), ARRAY[]::TEXT[])
			WHERE id = ?`, itemID, itemID)
}

// contentReaderFromType maps content types to reader types.
func contentReaderFromType(ct string) string {
	switch ct {
	case "application/pdf":
		return "PDF"
	case "application/epub+zip":
		return "EPUB"
	default:
		return "WEB"
	}
}
