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

type RuleService struct{ db *db.DB }

func newRuleService(database *db.DB) *RuleService { return &RuleService{db: database} }

func (s *RuleService) List(ctx context.Context, userID string) ([]models.Rule, error) {
	var rules []models.Rule
	err := s.db.Read.WithContext(ctx).Where("user_id = ?", userID).Order("created_at ASC").Find(&rules).Error
	return rules, err
}

func (s *RuleService) GetByID(ctx context.Context, id, userID string) (*models.Rule, error) {
	var r models.Rule
	if err := s.db.Read.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&r).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &r, nil
}

type SetRuleInput struct {
	ID          *string
	Name        string
	Filter      string
	Actions     models.JSONRaw
	EventTypes  []string
	Enabled     bool
	Description *string
}

func (s *RuleService) Set(ctx context.Context, userID string, input SetRuleInput) (*models.Rule, error) {
	now := time.Now()
	if input.ID != nil {
		updates := map[string]any{
			"name": input.Name, "filter": input.Filter,
			"actions": input.Actions, "event_types": models.StringArray(input.EventTypes),
			"enabled": input.Enabled, "updated_at": now,
		}
		if input.Description != nil {
			updates["description"] = *input.Description
		}
		if err := s.db.Write.WithContext(ctx).Model(&models.Rule{}).Where("id = ? AND user_id = ?", *input.ID, userID).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("update rule: %w", err)
		}
		return s.GetByID(ctx, *input.ID, userID)
	}

	r := &models.Rule{
		ID:          uuid.New().String(),
		UserID:      userID,
		Name:        input.Name,
		Filter:      input.Filter,
		Actions:     input.Actions,
		EventTypes:  input.EventTypes,
		Enabled:     input.Enabled,
		Description: input.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.db.Write.WithContext(ctx).Create(r).Error; err != nil {
		return nil, fmt.Errorf("create rule: %w", err)
	}
	return r, nil
}

func (s *RuleService) Delete(ctx context.Context, id, userID string) error {
	return s.db.Write.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&models.Rule{}).Error
}
