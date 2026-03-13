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

type IntegrationService struct{ db *db.DB }

func newIntegrationService(database *db.DB) *IntegrationService {
	return &IntegrationService{db: database}
}

func (s *IntegrationService) List(ctx context.Context, userID string) ([]models.Integration, error) {
	var items []models.Integration
	err := s.db.Read.WithContext(ctx).Where("user_id = ? AND enabled = true", userID).Find(&items).Error
	return items, err
}

func (s *IntegrationService) GetByID(ctx context.Context, id, userID string) (*models.Integration, error) {
	var i models.Integration
	if err := s.db.Read.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&i).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &i, nil
}

type SetIntegrationInput struct {
	ID              *string
	Name            string
	Token           string
	Type            string
	Enabled         bool
	Settings        models.JSONMap
	ImportItemState *string
}

func (s *IntegrationService) Set(ctx context.Context, userID string, input SetIntegrationInput) (*models.Integration, error) {
	now := time.Now()
	if input.ID != nil {
		updates := map[string]any{
			"name": input.Name, "token": input.Token,
			"enabled": input.Enabled, "settings": input.Settings,
			"updated_at": now,
		}
		if input.ImportItemState != nil {
			updates["import_item_state"] = *input.ImportItemState
		}
		if err := s.db.Write.WithContext(ctx).Model(&models.Integration{}).
			Where("id = ? AND user_id = ?", *input.ID, userID).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("update integration: %w", err)
		}
		return s.GetByID(ctx, *input.ID, userID)
	}

	i := &models.Integration{
		ID:        uuid.New().String(),
		UserID:    userID,
		Name:      input.Name,
		Token:     input.Token,
		Type:      input.Type,
		Enabled:   input.Enabled,
		Settings:  input.Settings,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if input.ImportItemState != nil {
		i.ImportItemState = input.ImportItemState
	}
	if err := s.db.Write.WithContext(ctx).Create(i).Error; err != nil {
		return nil, fmt.Errorf("create integration: %w", err)
	}
	return i, nil
}

func (s *IntegrationService) Delete(ctx context.Context, id, userID string) error {
	return s.db.Write.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&models.Integration{}).Error
}
