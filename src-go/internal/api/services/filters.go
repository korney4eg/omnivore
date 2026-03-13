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

type FilterService struct{ db *db.DB }

func newFilterService(database *db.DB) *FilterService {
	return &FilterService{db: database}
}

func (s *FilterService) List(ctx context.Context, userID string) ([]models.Filter, error) {
	var items []models.Filter
	err := s.db.Read.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("position ASC").
		Find(&items).Error
	return items, err
}

func (s *FilterService) GetByID(ctx context.Context, id, userID string) (*models.Filter, error) {
	var f models.Filter
	if err := s.db.Read.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&f).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &f, nil
}

type CreateFilterInput struct {
	Name          string
	Description   *string
	Filter        string
	Category      string
	Position      int
	DefaultFilter bool
	Visible       bool
	Folder        *string
}

func (s *FilterService) Create(ctx context.Context, userID string, input CreateFilterInput) (*models.Filter, error) {
	now := time.Now()
	f := &models.Filter{
		ID:            uuid.New().String(),
		UserID:        userID,
		Name:          input.Name,
		Description:   input.Description,
		Filter:        input.Filter,
		Category:      input.Category,
		Position:      input.Position,
		DefaultFilter: input.DefaultFilter,
		Visible:       input.Visible,
		Folder:        input.Folder,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.db.Write.WithContext(ctx).Create(f).Error; err != nil {
		return nil, fmt.Errorf("create filter: %w", err)
	}
	return f, nil
}

type UpdateFilterInput struct {
	Name          *string
	Description   *string
	Filter        *string
	Category      *string
	Position      *int
	DefaultFilter *bool
	Visible       *bool
	Folder        *string
}

func (s *FilterService) Update(ctx context.Context, id, userID string, input UpdateFilterInput) (*models.Filter, error) {
	updates := map[string]any{"updated_at": time.Now()}
	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.Description != nil {
		updates["description"] = *input.Description
	}
	if input.Filter != nil {
		updates["filter"] = *input.Filter
	}
	if input.Category != nil {
		updates["category"] = *input.Category
	}
	if input.Position != nil {
		updates["position"] = *input.Position
	}
	if input.DefaultFilter != nil {
		updates["default_filter"] = *input.DefaultFilter
	}
	if input.Visible != nil {
		updates["visible"] = *input.Visible
	}
	if input.Folder != nil {
		updates["folder"] = *input.Folder
	}
	if err := s.db.Write.WithContext(ctx).Model(&models.Filter{}).
		Where("id = ? AND user_id = ?", id, userID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update filter: %w", err)
	}
	return s.GetByID(ctx, id, userID)
}

func (s *FilterService) Delete(ctx context.Context, id, userID string) error {
	return s.db.Write.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&models.Filter{}).Error
}
