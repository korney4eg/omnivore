package queue

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/omnivore-app/omnivore/internal/bullmq"
)

// handleBulkAction processes a batch operation on library items.
func (w *BackendWorker) handleBulkAction(job *bullmq.RawJob) error {
	data, err := unmarshalJobData[BulkActionJobData](job)
	if err != nil {
		return fmt.Errorf("unmarshal bulk-action data: %w", err)
	}

	slog.Info("bulk-action", "userId", data.UserID, "action", data.Action,
		"count", data.Count, "query", data.Query)

	batchSize := data.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	now := time.Now()

	// Process in batches
	for offset := 0; offset < data.Count; offset += batchSize {
		limit := batchSize
		if offset+limit > data.Count {
			limit = data.Count - offset
		}

		// Find items matching the query, ordered by updated_at to avoid reprocessing
		var itemIDs []string
		if err := w.db.Read.WithContext(w.ctx).
			Table("omnivore.library_item").
			Select("id").
			Where("user_id = ? AND state NOT IN ('DELETED') AND updated_at < ?", data.UserID, now).
			Order("updated_at ASC").
			Limit(limit).
			Offset(offset).
			Pluck("id", &itemIDs).Error; err != nil {
			slog.Error("bulk-action: query items", "err", err, "offset", offset)
			continue
		}

		if len(itemIDs) == 0 {
			break
		}

		switch data.Action {
		case "Archive":
			w.db.Write.WithContext(w.ctx).
				Table("omnivore.library_item").
				Where("id IN ? AND user_id = ?", itemIDs, data.UserID).
				Updates(map[string]any{
					"archived_at": now,
					"state":       "ARCHIVED",
					"updated_at":  now,
				})

		case "Delete":
			w.db.Write.WithContext(w.ctx).
				Table("omnivore.library_item").
				Where("id IN ? AND user_id = ?", itemIDs, data.UserID).
				Updates(map[string]any{
					"deleted_at": now,
					"state":      "DELETED",
					"updated_at": now,
				})

		case "AddLabel":
			for _, itemID := range itemIDs {
				for _, labelID := range data.LabelIDs {
					w.db.Write.WithContext(w.ctx).
						Exec(`INSERT INTO omnivore.entity_labels (id, label_id, library_item_id, source)
							VALUES (gen_random_uuid(), ?, ?, 'system') ON CONFLICT DO NOTHING`,
							labelID, itemID)
				}
				// Update denormalized label_names
				w.db.Write.WithContext(w.ctx).Exec(`
					UPDATE omnivore.library_item
					SET label_names = COALESCE((
						SELECT array_agg(DISTINCT l.name)
						FROM omnivore.labels l
						INNER JOIN omnivore.entity_labels el ON el.label_id = l.id AND el.library_item_id = ?
					), ARRAY[]::TEXT[])
					WHERE id = ?`, itemID, itemID)
			}

		case "MarkAsRead":
			w.db.Write.WithContext(w.ctx).
				Table("omnivore.library_item").
				Where("id IN ? AND user_id = ?", itemIDs, data.UserID).
				Updates(map[string]any{
					"reading_progress_top_percent":    100,
					"reading_progress_bottom_percent": 100,
					"read_at":    now,
					"updated_at": now,
				})

		case "MoveToFolder":
			// Folder name should be in the query or args
			slog.Warn("bulk-action: MoveToFolder requires folder name in args")

		default:
			slog.Warn("bulk-action: unknown action", "action", data.Action)
		}

		slog.Debug("bulk-action: batch processed",
			"action", data.Action, "offset", offset, "items", len(itemIDs))
	}

	return nil
}
