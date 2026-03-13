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

// LabelService handles label CRUD and assignment.
type LabelService struct{ db *db.DB }

func newLabelService(database *db.DB) *LabelService { return &LabelService{db: database} }

// ListByUser returns all labels for a user, ordered by position then name.
func (s *LabelService) ListByUser(ctx context.Context, userID string) ([]models.Label, error) {
	var labels []models.Label
	if err := s.db.Read.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("position ASC, name ASC").
		Find(&labels).Error; err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}
	return labels, nil
}

// GetByID returns a single label owned by the user.
func (s *LabelService) GetByID(ctx context.Context, id, userID string) (*models.Label, error) {
	var l models.Label
	if err := s.db.Read.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&l).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get label: %w", err)
	}
	return &l, nil
}

// CreateLabelInput is the input for creating a label.
type CreateLabelInput struct {
	Name        string
	Color       string
	Description *string
}

// Create creates a new label for a user.
func (s *LabelService) Create(ctx context.Context, userID string, input CreateLabelInput) (*models.Label, error) {
	label := &models.Label{
		ID:          uuid.New().String(),
		UserID:      userID,
		Name:        input.Name,
		Color:       input.Color,
		Description: input.Description,
		CreatedAt:   time.Now(),
	}
	if err := s.db.Write.WithContext(ctx).Create(label).Error; err != nil {
		return nil, fmt.Errorf("create label: %w", err)
	}
	return label, nil
}

// UpdateLabelInput is the input for updating a label.
type UpdateLabelInput struct {
	Name        *string
	Color       *string
	Description *string
}

// Update modifies a label.
func (s *LabelService) Update(ctx context.Context, id, userID string, input UpdateLabelInput) (*models.Label, error) {
	updates := map[string]any{"updated_at": time.Now()}
	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.Color != nil {
		updates["color"] = *input.Color
	}
	if input.Description != nil {
		updates["description"] = *input.Description
	}

	if err := s.db.Write.WithContext(ctx).
		Model(&models.Label{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update label: %w", err)
	}
	return s.GetByID(ctx, id, userID)
}

// Delete removes a label and its entity_label associations.
func (s *LabelService) Delete(ctx context.Context, id, userID string) error {
	return s.db.Write.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("label_id = ?", id).Delete(&models.EntityLabel{}).Error; err != nil {
			return fmt.Errorf("delete entity_labels: %w", err)
		}
		return tx.Where("id = ? AND user_id = ?", id, userID).Delete(&models.Label{}).Error
	})
}

// SetLabelsForItem replaces all labels on a library item.
func (s *LabelService) SetLabelsForItem(ctx context.Context, itemID, userID string, labelIDs []string) ([]models.Label, error) {
	var labels []models.Label
	err := s.db.Write.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Remove existing
		if err := tx.Where("library_item_id = ?", itemID).Delete(&models.EntityLabel{}).Error; err != nil {
			return err
		}
		if len(labelIDs) == 0 {
			return nil
		}
		// Verify labels belong to user
		if err := tx.Where("id IN ? AND user_id = ?", labelIDs, userID).Find(&labels).Error; err != nil {
			return err
		}
		// Insert new associations
		rows := make([]models.EntityLabel, 0, len(labels))
		for _, l := range labels {
			rows = append(rows, models.EntityLabel{
				ID:            uuid.New().String(),
				LabelID:       l.ID,
				LibraryItemID: &itemID,
				Source:        "user",
			})
		}
		return tx.Create(&rows).Error
	})
	if err != nil {
		return nil, fmt.Errorf("set labels for item: %w", err)
	}
	return labels, nil
}

// SetLabelsForHighlight replaces all labels on a highlight.
func (s *LabelService) SetLabelsForHighlight(ctx context.Context, highlightID, userID string, labelIDs []string) ([]models.Label, error) {
	var labels []models.Label
	err := s.db.Write.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("highlight_id = ?", highlightID).Delete(&models.EntityLabel{}).Error; err != nil {
			return err
		}
		if len(labelIDs) == 0 {
			return nil
		}
		if err := tx.Where("id IN ? AND user_id = ?", labelIDs, userID).Find(&labels).Error; err != nil {
			return err
		}
		rows := make([]models.EntityLabel, 0, len(labels))
		for _, l := range labels {
			rows = append(rows, models.EntityLabel{
				ID:          uuid.New().String(),
				LabelID:     l.ID,
				HighlightID: &highlightID,
				Source:      "user",
			})
		}
		return tx.Create(&rows).Error
	})
	if err != nil {
		return nil, fmt.Errorf("set labels for highlight: %w", err)
	}
	return labels, nil
}

// ListForItem returns all labels assigned to a library item.
func (s *LabelService) ListForItem(ctx context.Context, itemID string) ([]models.Label, error) {
	var labels []models.Label
	if err := s.db.Read.WithContext(ctx).
		Joins("JOIN omnivore.entity_labels el ON el.label_id = omnivore.labels.id").
		Where("el.library_item_id = ?", itemID).
		Find(&labels).Error; err != nil {
		return nil, fmt.Errorf("list labels for item: %w", err)
	}
	return labels, nil
}

// ListForHighlight returns all labels assigned to a highlight.
func (s *LabelService) ListForHighlight(ctx context.Context, highlightID string) ([]models.Label, error) {
	var labels []models.Label
	if err := s.db.Read.WithContext(ctx).
		Joins("JOIN omnivore.entity_labels el ON el.label_id = omnivore.labels.id").
		Where("el.highlight_id = ?", highlightID).
		Find(&labels).Error; err != nil {
		return nil, fmt.Errorf("list labels for highlight: %w", err)
	}
	return labels, nil
}
