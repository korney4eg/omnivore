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

// HighlightService handles highlight CRUD.
type HighlightService struct{ db *db.DB }

func newHighlightService(database *db.DB) *HighlightService { return &HighlightService{db: database} }

// ListByItem returns all highlights for a library item.
func (s *HighlightService) ListByItem(ctx context.Context, itemID, userID string) ([]models.Highlight, error) {
	var hs []models.Highlight
	if err := s.db.Read.WithContext(ctx).
		Preload("Labels").
		Where("library_item_id = ? AND user_id = ?", itemID, userID).
		Order("highlight_position_percent ASC").
		Find(&hs).Error; err != nil {
		return nil, fmt.Errorf("list highlights: %w", err)
	}
	return hs, nil
}

// GetByID returns a single highlight.
func (s *HighlightService) GetByID(ctx context.Context, id, userID string) (*models.Highlight, error) {
	var h models.Highlight
	if err := s.db.Read.WithContext(ctx).
		Preload("Labels").
		Where("id = ? AND user_id = ?", id, userID).
		First(&h).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get highlight: %w", err)
	}
	return &h, nil
}

// CreateHighlightInput is the input for creating a highlight.
type CreateHighlightInput struct {
	LibraryItemID             string
	ShortID                   string
	Quote                     *string
	Prefix                    *string
	Suffix                    *string
	Patch                     *string
	Annotation                *string
	HighlightType             string
	HighlightPositionPercent  float32
	HighlightPositionAnchorIndex int
	HTML                      *string
	Color                     *string
	Representation            string
}

// Create creates a new highlight.
func (s *HighlightService) Create(ctx context.Context, userID string, input CreateHighlightInput) (*models.Highlight, error) {
	highlightType := input.HighlightType
	if highlightType == "" {
		highlightType = "HIGHLIGHT"
	}
	representation := input.Representation
	if representation == "" {
		representation = "CONTENT"
	}

	h := &models.Highlight{
		ID:                        uuid.New().String(),
		ShortID:                   input.ShortID,
		UserID:                    userID,
		LibraryItemID:             input.LibraryItemID,
		Quote:                     input.Quote,
		Prefix:                    input.Prefix,
		Suffix:                    input.Suffix,
		Patch:                     input.Patch,
		Annotation:                input.Annotation,
		HighlightType:             highlightType,
		HighlightPositionPercent:  input.HighlightPositionPercent,
		HighlightPositionAnchorIndex: input.HighlightPositionAnchorIndex,
		HTML:                      input.HTML,
		Color:                     input.Color,
		Representation:            representation,
		CreatedAt:                 time.Now(),
		UpdatedAt:                 time.Now(),
	}

	if err := s.db.Write.WithContext(ctx).Create(h).Error; err != nil {
		return nil, fmt.Errorf("create highlight: %w", err)
	}
	return h, nil
}

// UpdateHighlightInput is the input for updating a highlight.
type UpdateHighlightInput struct {
	Annotation *string
	Color      *string
	HTML       *string
}

// Update modifies a highlight.
func (s *HighlightService) Update(ctx context.Context, id, userID string, input UpdateHighlightInput) (*models.Highlight, error) {
	updates := map[string]any{"updated_at": time.Now()}
	if input.Annotation != nil {
		updates["annotation"] = *input.Annotation
	}
	if input.Color != nil {
		updates["color"] = *input.Color
	}
	if input.HTML != nil {
		updates["html"] = *input.HTML
	}

	if err := s.db.Write.WithContext(ctx).
		Model(&models.Highlight{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update highlight: %w", err)
	}
	return s.GetByID(ctx, id, userID)
}

// Delete removes a highlight and its label associations.
func (s *HighlightService) Delete(ctx context.Context, id, userID string) error {
	return s.db.Write.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("highlight_id = ?", id).Delete(&models.EntityLabel{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ? AND user_id = ?", id, userID).Delete(&models.Highlight{}).Error
	})
}

// Search returns highlights for a user with optional keyword filter and pagination.
func (s *HighlightService) Search(ctx context.Context, userID, query string, limit, offset int) ([]models.Highlight, int, error) {
	q := s.db.Read.WithContext(ctx).
		Preload("Labels").
		Where("user_id = ?", userID)

	if query != "" {
		q = q.Where("annotation ILIKE ? OR quote ILIKE ?", "%"+query+"%", "%"+query+"%")
	}

	var total int64
	if err := q.Model(&models.Highlight{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count highlights: %w", err)
	}

	var hs []models.Highlight
	if err := q.Order("created_at DESC").Offset(offset).Limit(limit).Find(&hs).Error; err != nil {
		return nil, 0, fmt.Errorf("search highlights: %w", err)
	}
	return hs, int(total), nil
}
