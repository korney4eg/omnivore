package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/omnivore-app/omnivore/internal/api/models"
	"github.com/omnivore-app/omnivore/internal/db"
	"github.com/omnivore-app/omnivore/internal/redisutil"
	"gorm.io/gorm"
)

// LibraryItemService handles article/link CRUD and search.
type LibraryItemService struct {
	db    *db.DB
	redis *redisutil.RedisDataSource
}

func newLibraryItemService(database *db.DB, redis *redisutil.RedisDataSource) *LibraryItemService {
	return &LibraryItemService{db: database, redis: redis}
}

// GetByID returns a library item for a specific user.
func (s *LibraryItemService) GetByID(ctx context.Context, id, userID string) (*models.LibraryItem, error) {
	var item models.LibraryItem
	if err := s.db.Read.WithContext(ctx).
		Preload("Labels").
		Preload("Highlights").
		Where("id = ? AND user_id = ? AND state != 'DELETED'", id, userID).
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get library item: %w", err)
	}
	return &item, nil
}

// GetBySlug returns a library item by its slug.
func (s *LibraryItemService) GetBySlug(ctx context.Context, slug, userID string) (*models.LibraryItem, error) {
	var item models.LibraryItem
	if err := s.db.Read.WithContext(ctx).
		Preload("Labels").
		Preload("Highlights").
		Where("slug = ? AND user_id = ? AND state != 'DELETED'", slug, userID).
		First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get library item by slug: %w", err)
	}
	return &item, nil
}

// SearchInput mirrors the TypeScript search parameters.
type SearchInput struct {
	Query          string
	After          *string    // opaque cursor (numeric offset as string)
	First          *int       // page size (default 10, max 100)
	Sort           *SortInput
	Format         *string
	Folder         *string
	IncludeContent bool
	Since          *time.Time // only return items updated after this time
}

// SortInput defines sort options.
type SortInput struct {
	By    string // SAVED_AT | UPDATED_TIME | PUBLISHED_AT | SCORE
	Order string // ASCENDING | DESCENDING
}

// SearchResult is a page of library items.
type SearchResult struct {
	Items      []models.LibraryItem
	TotalCount int64
	HasNext    bool
}

// Search performs a full-text + filter search over the user's library.
func (s *LibraryItemService) Search(ctx context.Context, userID string, input SearchInput) (*SearchResult, error) {
	pageSize := 10
	if input.First != nil && *input.First > 0 && *input.First <= 100 {
		pageSize = *input.First
	}

	offset := 0
	if input.After != nil && *input.After != "" {
		fmt.Sscanf(*input.After, "%d", &offset) //nolint:errcheck
	}

	q := s.db.Read.WithContext(ctx).
		Where("user_id = ? AND state NOT IN ('DELETED')", userID)

	// Folder filter
	if input.Folder != nil && *input.Folder != "" {
		q = q.Where("folder = ?", *input.Folder)
	}

	// Since filter (used by UpdatesSince)
	if input.Since != nil {
		q = q.Where("updated_at > ?", *input.Since)
	}

	// Simple keyword search via PostgreSQL ILIKE (full-text upgrade in Phase 5+)
	if input.Query != "" {
		terms := strings.Fields(input.Query)
		for _, t := range terms {
			if strings.HasPrefix(t, "label:") {
				label := strings.TrimPrefix(t, "label:")
				q = q.Where("? = ANY(label_names)", label)
			} else if strings.HasPrefix(t, "in:") {
				folder := strings.TrimPrefix(t, "in:")
				q = q.Where("folder = ?", folder)
			} else {
				q = q.Where("(title ILIKE ? OR description ILIKE ?)",
					"%"+t+"%", "%"+t+"%")
			}
		}
	}

	// Sort
	orderBy := "saved_at DESC"
	if input.Sort != nil {
		col := sortColumn(input.Sort.By)
		dir := "DESC"
		if strings.ToUpper(input.Sort.Order) == "ASCENDING" {
			dir = "ASC"
		}
		orderBy = col + " " + dir
	}
	q = q.Order(orderBy)

	var total int64
	if err := q.Model(&models.LibraryItem{}).Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count library items: %w", err)
	}

	var items []models.LibraryItem
	q2 := q.Preload("Labels").Offset(offset).Limit(pageSize + 1)
	if !input.IncludeContent {
		q2 = q2.Omit("readable_content")
	}
	if err := q2.Find(&items).Error; err != nil {
		return nil, fmt.Errorf("search library items: %w", err)
	}

	hasNext := len(items) > pageSize
	if hasNext {
		items = items[:pageSize]
	}

	return &SearchResult{
		Items:      items,
		TotalCount: total,
		HasNext:    hasNext,
	}, nil
}

func sortColumn(by string) string {
	switch by {
	case "UPDATED_TIME":
		return "updated_at"
	case "PUBLISHED_AT":
		return "published_at"
	case "SCORE":
		return "saved_at" // score not yet implemented; fall back to saved_at
	default:
		return "saved_at"
	}
}

// SavePageInput is the minimal set of fields needed to create a library item.
type SavePageInput struct {
	URL         string
	Title       string
	Content     *string
	SavedAt     *time.Time
	PublishedAt *time.Time
	Folder      string
	State       string
	Labels      []string
}

// SavePage creates a new library item for a user.
func (s *LibraryItemService) SavePage(ctx context.Context, userID string, input SavePageInput) (*models.LibraryItem, error) {
	now := time.Now()
	savedAt := now
	if input.SavedAt != nil {
		savedAt = *input.SavedAt
	}
	state := "SUCCEEDED"
	if input.State != "" {
		state = input.State
	}
	folder := "inbox"
	if input.Folder != "" {
		folder = input.Folder
	}
	content := ""
	if input.Content != nil {
		content = *input.Content
	}

	item := &models.LibraryItem{
		ID:              uuid.New().String(),
		UserID:          userID,
		OriginalURL:     input.URL,
		Slug:            slugify(input.Title, now),
		Title:           input.Title,
		ReadableContent: content,
		SavedAt:         savedAt,
		PublishedAt:     input.PublishedAt,
		State:           state,
		Folder:          folder,
		ItemType:        "ARTICLE",
		ContentReader:   "WEB",
		Directionality:  "LTR",
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.db.Write.WithContext(ctx).Create(item).Error; err != nil {
		return nil, fmt.Errorf("save page: %w", err)
	}

	return item, nil
}

// UpdatePageInput fields (all optional; nil = no change).
type UpdatePageInput struct {
	Title       *string
	Description *string
	Author      *string
	SavedAt     *time.Time
	PublishedAt *time.Time
	State       *string
	Thumbnail   *string
}

// UpdatePage modifies mutable fields of a library item.
func (s *LibraryItemService) UpdatePage(ctx context.Context, id, userID string, input UpdatePageInput) (*models.LibraryItem, error) {
	updates := map[string]any{"updated_at": time.Now()}
	if input.Title != nil {
		updates["title"] = *input.Title
	}
	if input.Description != nil {
		updates["description"] = *input.Description
	}
	if input.Author != nil {
		updates["author"] = *input.Author
	}
	if input.SavedAt != nil {
		updates["saved_at"] = *input.SavedAt
	}
	if input.PublishedAt != nil {
		updates["published_at"] = *input.PublishedAt
	}
	if input.State != nil {
		updates["state"] = *input.State
		if *input.State == "ARCHIVED" {
			now := time.Now()
			updates["archived_at"] = now
		}
	}
	if input.Thumbnail != nil {
		updates["thumbnail"] = *input.Thumbnail
	}

	if err := s.db.Write.WithContext(ctx).
		Model(&models.LibraryItem{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update page: %w", err)
	}

	return s.GetByID(ctx, id, userID)
}

// DeleteItem soft-deletes a library item (state=DELETED).
func (s *LibraryItemService) DeleteItem(ctx context.Context, id, userID string) error {
	now := time.Now()
	return s.db.Write.WithContext(ctx).
		Model(&models.LibraryItem{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]any{
			"state":      "DELETED",
			"deleted_at": now,
			"updated_at": now,
		}).Error
}

// ArchiveItem sets the item state to ARCHIVED.
func (s *LibraryItemService) ArchiveItem(ctx context.Context, id, userID string, archive bool) error {
	now := time.Now()
	state := "ARCHIVED"
	var archivedAt any = now
	if !archive {
		state = "SUCCEEDED"
		archivedAt = nil
	}
	return s.db.Write.WithContext(ctx).
		Model(&models.LibraryItem{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]any{
			"state":       state,
			"archived_at": archivedAt,
			"updated_at":  now,
		}).Error
}

// UpdateReadingProgress stores reading progress for an item.
type ReadingProgressInput struct {
	TopPercent        float32
	BottomPercent     float32
	AnchorIndex       int
	HighestAnchor     int
}

// UpdateReadingProgress persists the reading position.
func (s *LibraryItemService) UpdateReadingProgress(ctx context.Context, id, userID string, input ReadingProgressInput) error {
	now := time.Now()
	updates := map[string]any{
		"reading_progress_top_percent":          input.TopPercent,
		"reading_progress_bottom_percent":       input.BottomPercent,
		"reading_progress_last_read_anchor":     input.AnchorIndex,
		"reading_progress_highest_read_anchor":  input.HighestAnchor,
		"updated_at":                            now,
	}
	if input.TopPercent >= 98 {
		updates["read_at"] = now
	}
	return s.db.Write.WithContext(ctx).
		Model(&models.LibraryItem{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(updates).Error
}

// MoveToFolder moves an item into a different folder.
func (s *LibraryItemService) MoveToFolder(ctx context.Context, id, userID, folder string) error {
	return s.db.Write.WithContext(ctx).
		Model(&models.LibraryItem{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]any{
			"folder":     folder,
			"updated_at": time.Now(),
		}).Error
}

// BulkAction performs an action on all items matching a filter query.
type BulkAction string

const (
	BulkActionDelete  BulkAction = "DELETE"
	BulkActionArchive BulkAction = "ARCHIVE"
	BulkActionLabel   BulkAction = "ADD_LABELS"
	BulkActionMove    BulkAction = "MOVE_TO_FOLDER"
	BulkActionMarkRead BulkAction = "MARK_AS_READ"
)

// BulkActionInput describes a bulk operation.
type BulkActionInput struct {
	Action    BulkAction
	Query     string   // filter query
	LabelIDs  []string // for BulkActionLabel
	Folder    *string  // for BulkActionMove
}

// ExecuteBulkAction performs an action on all matching items for a user.
func (s *LibraryItemService) ExecuteBulkAction(ctx context.Context, userID string, input BulkActionInput) (int64, error) {
	q := s.db.Write.WithContext(ctx).
		Where("user_id = ? AND state NOT IN ('DELETED')", userID)

	now := time.Now()
	var result *gorm.DB

	switch input.Action {
	case BulkActionDelete:
		result = q.Model(&models.LibraryItem{}).Updates(map[string]any{
			"state": "DELETED", "deleted_at": now, "updated_at": now,
		})
	case BulkActionArchive:
		result = q.Model(&models.LibraryItem{}).Updates(map[string]any{
			"state": "ARCHIVED", "archived_at": now, "updated_at": now,
		})
	case BulkActionMarkRead:
		result = q.Model(&models.LibraryItem{}).Updates(map[string]any{
			"reading_progress_top_percent":    100,
			"reading_progress_bottom_percent": 100,
			"read_at":    now,
			"updated_at": now,
		})
	case BulkActionMove:
		if input.Folder == nil {
			return 0, fmt.Errorf("folder is required for MOVE_TO_FOLDER")
		}
		result = q.Model(&models.LibraryItem{}).Updates(map[string]any{
			"folder": *input.Folder, "updated_at": now,
		})
	default:
		return 0, fmt.Errorf("unsupported bulk action: %s", input.Action)
	}

	if result.Error != nil {
		return 0, fmt.Errorf("bulk action: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// slugify produces a URL-safe slug from a title and timestamp.
func slugify(title string, t time.Time) string {
	slug := strings.ToLower(title)
	slug = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, slug)
	// Collapse multiple dashes
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")
	if len(slug) > 50 {
		slug = slug[:50]
	}
	return fmt.Sprintf("%s-%d", slug, t.UnixMilli())
}
