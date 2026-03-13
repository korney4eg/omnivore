package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/omnivore-app/omnivore/internal/api/models"
	"github.com/omnivore-app/omnivore/internal/db"
	"gorm.io/gorm"
)

type WebhookService struct{ db *db.DB }

func newWebhookService(database *db.DB) *WebhookService { return &WebhookService{db: database} }

func (s *WebhookService) List(ctx context.Context, userID string) ([]models.Webhook, error) {
	var ws []models.Webhook
	err := s.db.Read.WithContext(ctx).Where("user_id = ?", userID).Order("created_at ASC").Find(&ws).Error
	return ws, err
}

func (s *WebhookService) GetByID(ctx context.Context, id, userID string) (*models.Webhook, error) {
	var w models.Webhook
	if err := s.db.Read.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&w).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &w, nil
}

type SetWebhookInput struct {
	ID          *string // nil = create
	URL         string
	EventTypes  []string
	Method      *string
	ContentType *string
	Enabled     *bool
}

func (s *WebhookService) Set(ctx context.Context, userID string, input SetWebhookInput) (*models.Webhook, error) {
	now := time.Now()
	if input.ID != nil {
		// Update
		updates := map[string]any{"url": input.URL, "event_types": models.StringArray(input.EventTypes), "updated_at": now}
		if input.Method != nil {
			updates["method"] = *input.Method
		}
		if input.ContentType != nil {
			updates["content_type"] = *input.ContentType
		}
		if input.Enabled != nil {
			updates["enabled"] = *input.Enabled
		}
		if err := s.db.Write.WithContext(ctx).Model(&models.Webhook{}).Where("id = ? AND user_id = ?", *input.ID, userID).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("update webhook: %w", err)
		}
		return s.GetByID(ctx, *input.ID, userID)
	}

	// Create
	method := "POST"
	if input.Method != nil {
		method = *input.Method
	}
	ct := "application/json"
	if input.ContentType != nil {
		ct = *input.ContentType
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	w := &models.Webhook{
		ID:          uuid.New().String(),
		UserID:      userID,
		URL:         input.URL,
		EventTypes:  input.EventTypes,
		Method:      method,
		ContentType: ct,
		Enabled:     enabled,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.db.Write.WithContext(ctx).Create(w).Error; err != nil {
		return nil, fmt.Errorf("create webhook: %w", err)
	}
	return w, nil
}

func (s *WebhookService) Delete(ctx context.Context, id, userID string) error {
	return s.db.Write.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&models.Webhook{}).Error
}
