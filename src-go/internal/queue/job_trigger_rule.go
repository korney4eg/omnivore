package queue

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/omnivore-app/omnivore/internal/bullmq"
)

// ruleRecord represents a rule row from the database.
type ruleRecord struct {
	ID          string          `gorm:"column:id"`
	Filter      string          `gorm:"column:filter"`
	Actions     json.RawMessage `gorm:"column:actions"`
	EventTypes  []string        `gorm:"column:event_types;type:text[]"`
	FailedAt    *time.Time      `gorm:"column:failed_at"`
}

// ruleAction represents a single action in a rule.
type ruleAction struct {
	Type   string   `json:"type"`
	Params []string `json:"params"`
}

// handleTriggerRule executes automation rules matching the event.
func (w *BackendWorker) handleTriggerRule(job *bullmq.RawJob) error {
	data, err := unmarshalJobData[TriggerRuleJobData](job)
	if err != nil {
		return fmt.Errorf("unmarshal trigger-rule data: %w", err)
	}

	slog.Info("trigger-rule", "userId", data.UserID, "eventType", data.RuleEventType)

	// Find enabled rules for this user and event type
	var rules []ruleRecord
	if err := w.db.Read.WithContext(w.ctx).
		Table("omnivore.rules").
		Where("user_id = ? AND enabled = true AND failed_at IS NULL", data.UserID).
		Find(&rules).Error; err != nil {
		return fmt.Errorf("find rules: %w", err)
	}

	if len(rules) == 0 {
		slog.Debug("trigger-rule: no enabled rules found", "userId", data.UserID)
		return nil
	}

	itemID, _ := data.Data["libraryItemId"].(string)
	if itemID == "" {
		itemID, _ = data.Data["id"].(string)
	}

	for _, rule := range rules {
		// Parse rule actions
		var actions []ruleAction
		if err := json.Unmarshal(rule.Actions, &actions); err != nil {
			slog.Error("trigger-rule: parse actions", "err", err, "ruleId", rule.ID)
			w.markRuleFailed(rule.ID)
			continue
		}

		// Execute each action
		for _, action := range actions {
			if err := w.executeRuleAction(data.UserID, itemID, action); err != nil {
				slog.Error("trigger-rule: action failed",
					"err", err, "ruleId", rule.ID, "action", action.Type, "itemId", itemID)
			}
		}
	}

	return nil
}

// executeRuleAction performs a single rule action.
func (w *BackendWorker) executeRuleAction(userID, itemID string, action ruleAction) error {
	switch action.Type {
	case "ADD_LABEL":
		return w.ruleAddLabel(userID, itemID, action.Params)
	case "ARCHIVE":
		return w.ruleArchive(userID, itemID)
	case "DELETE":
		return w.ruleDelete(userID, itemID)
	case "MARK_AS_READ":
		return w.ruleMarkAsRead(userID, itemID)
	case "SEND_NOTIFICATION":
		slog.Info("trigger-rule: SEND_NOTIFICATION not yet implemented", "itemId", itemID)
		return nil
	case "WEBHOOK":
		if len(action.Params) > 0 {
			return w.ruleWebhook(userID, itemID, action.Params[0])
		}
		return nil
	case "EXPORT":
		slog.Info("trigger-rule: EXPORT not yet implemented", "itemId", itemID)
		return nil
	default:
		slog.Warn("trigger-rule: unknown action type", "type", action.Type)
		return nil
	}
}

func (w *BackendWorker) ruleAddLabel(userID, itemID string, labelIDs []string) error {
	for _, labelID := range labelIDs {
		w.db.Write.WithContext(w.ctx).
			Exec(`INSERT INTO omnivore.entity_labels (id, label_id, library_item_id, source)
				VALUES (gen_random_uuid(), ?, ?, 'rule') ON CONFLICT DO NOTHING`, labelID, itemID)
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
	return nil
}

func (w *BackendWorker) ruleArchive(userID, itemID string) error {
	now := time.Now()
	return w.db.Write.WithContext(w.ctx).
		Table("omnivore.library_item").
		Where("id = ? AND user_id = ?", itemID, userID).
		Updates(map[string]any{
			"archived_at": now,
			"state":       "ARCHIVED",
			"updated_at":  now,
		}).Error
}

func (w *BackendWorker) ruleDelete(userID, itemID string) error {
	now := time.Now()
	return w.db.Write.WithContext(w.ctx).
		Table("omnivore.library_item").
		Where("id = ? AND user_id = ?", itemID, userID).
		Updates(map[string]any{
			"deleted_at": now,
			"state":      "DELETED",
			"updated_at": now,
		}).Error
}

func (w *BackendWorker) ruleMarkAsRead(userID, itemID string) error {
	now := time.Now()
	return w.db.Write.WithContext(w.ctx).
		Table("omnivore.library_item").
		Where("id = ? AND user_id = ?", itemID, userID).
		Updates(map[string]any{
			"reading_progress_top_percent":    100,
			"reading_progress_bottom_percent": 100,
			"read_at":     now,
			"updated_at":  now,
		}).Error
}

func (w *BackendWorker) ruleWebhook(userID, itemID, webhookURL string) error {
	// Read the item to build event payload
	var item struct {
		Title string `gorm:"column:title"`
		URL   string `gorm:"column:original_url"`
	}
	if err := w.db.Read.WithContext(w.ctx).
		Table("omnivore.library_item").
		Where("id = ?", itemID).
		First(&item).Error; err != nil {
		return err
	}

	payload := map[string]any{
		"action": "created",
		"userId": userID,
		"page": map[string]any{
			"id":    itemID,
			"title": item.Title,
			"url":   item.URL,
		},
	}
	return postWebhook(webhookURL, payload)
}

func (w *BackendWorker) markRuleFailed(ruleID string) {
	w.db.Write.WithContext(w.ctx).
		Table("omnivore.rules").
		Where("id = ?", ruleID).
		Update("failed_at", time.Now())
}
