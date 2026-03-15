package queue

import (
	"fmt"
	"log/slog"

	"github.com/omnivore-app/omnivore/internal/bullmq"
)

// handleUpdateLabels syncs the label_names denormalized column on a library item.
// This runs the same SQL as the TS update-labels job.
func (w *BackendWorker) handleUpdateLabels(job *bullmq.RawJob) error {
	data, err := unmarshalJobData[UpdateLabelsJobData](job)
	if err != nil {
		return fmt.Errorf("unmarshal update-labels data: %w", err)
	}

	slog.Debug("update-labels", "itemId", data.LibraryItemID, "userId", data.UserID)

	result := w.db.Write.WithContext(w.ctx).Exec(`
		UPDATE omnivore.library_item
		SET label_names = COALESCE((
			SELECT array_agg(DISTINCT l.name)
			FROM omnivore.labels l
			INNER JOIN omnivore.entity_labels el
				ON el.label_id = l.id
				AND el.library_item_id = ?
		), ARRAY[]::TEXT[])
		WHERE id = ?
	`, data.LibraryItemID, data.LibraryItemID)

	if result.Error != nil {
		return fmt.Errorf("update label_names: %w", result.Error)
	}
	return nil
}

// handleUpdateHighlight syncs the highlight_annotations denormalized column on a library item.
func (w *BackendWorker) handleUpdateHighlight(job *bullmq.RawJob) error {
	data, err := unmarshalJobData[UpdateHighlightJobData](job)
	if err != nil {
		return fmt.Errorf("unmarshal update-highlight data: %w", err)
	}

	slog.Debug("update-highlight", "itemId", data.LibraryItemID, "userId", data.UserID)

	result := w.db.Write.WithContext(w.ctx).Exec(`
		UPDATE omnivore.library_item
		SET highlight_annotations = COALESCE((
			SELECT array_agg(COALESCE(annotation, ''))
			FROM omnivore.highlight
			WHERE library_item_id = ?
		), ARRAY[]::TEXT[])
		WHERE id = ?
	`, data.LibraryItemID, data.LibraryItemID)

	if result.Error != nil {
		return fmt.Errorf("update highlight_annotations: %w", result.Error)
	}
	return nil
}
