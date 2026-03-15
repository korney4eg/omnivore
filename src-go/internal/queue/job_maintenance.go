package queue

import (
	"fmt"
	"log/slog"

	"github.com/omnivore-app/omnivore/internal/bullmq"
)

// handlePruneTrash permanently removes soft-deleted library items older than 14 days.
func (w *BackendWorker) handlePruneTrash(job *bullmq.RawJob) error {
	slog.Info("prune-trash: starting")

	result := w.db.Write.WithContext(w.ctx).
		Exec("DELETE FROM omnivore.library_item WHERE state = 'DELETED' AND deleted_at < NOW() - INTERVAL '14 days'")
	if result.Error != nil {
		return fmt.Errorf("prune trash: %w", result.Error)
	}

	slog.Info("prune-trash: completed", "deleted", result.RowsAffected)
	return nil
}

// handleExpireFolders applies folder expiration policies.
func (w *BackendWorker) handleExpireFolders(job *bullmq.RawJob) error {
	slog.Info("expire-folders: starting")

	// Query folder policies and apply them
	var policies []struct {
		ID         string `gorm:"column:id"`
		UserID     string `gorm:"column:user_id"`
		Folder     string `gorm:"column:folder"`
		Action     string `gorm:"column:action_type"`
		AfterDays  int    `gorm:"column:after_days"`
	}

	if err := w.db.Read.WithContext(w.ctx).
		Table("omnivore.folder_policy").
		Find(&policies).Error; err != nil {
		slog.Warn("expire-folders: no folder_policy table or query failed", "err", err)
		return nil // Not an error if table doesn't exist yet
	}

	for _, policy := range policies {
		interval := fmt.Sprintf("%d days", policy.AfterDays)
		var result any

		switch policy.Action {
		case "ARCHIVE":
			r := w.db.Write.WithContext(w.ctx).Exec(`
				UPDATE omnivore.library_item
				SET state = 'ARCHIVED', archived_at = NOW(), updated_at = NOW()
				WHERE user_id = ? AND folder = ? AND state = 'ACTIVE'
					AND saved_at < NOW() - ?::interval`,
				policy.UserID, policy.Folder, interval)
			result = r.RowsAffected

		case "DELETE":
			r := w.db.Write.WithContext(w.ctx).Exec(`
				UPDATE omnivore.library_item
				SET state = 'DELETED', deleted_at = NOW(), updated_at = NOW()
				WHERE user_id = ? AND folder = ? AND state NOT IN ('DELETED')
					AND saved_at < NOW() - ?::interval`,
				policy.UserID, policy.Folder, interval)
			result = r.RowsAffected

		default:
			slog.Warn("expire-folders: unknown action", "action", policy.Action, "policyId", policy.ID)
			continue
		}

		slog.Info("expire-folders: policy applied",
			"policyId", policy.ID, "userId", policy.UserID,
			"folder", policy.Folder, "action", policy.Action,
			"affected", result)
	}

	slog.Info("expire-folders: completed", "policies", len(policies))
	return nil
}
