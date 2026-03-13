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

type ReminderService struct{ db *db.DB }

func newReminderService(database *db.DB) *ReminderService { return &ReminderService{db: database} }

func (s *ReminderService) List(ctx context.Context, userID string) ([]models.Reminder, error) {
	var rs []models.Reminder
	err := s.db.Read.WithContext(ctx).Where("user_id = ?", userID).Order("remind_at ASC").Find(&rs).Error
	return rs, err
}

func (s *ReminderService) GetByID(ctx context.Context, id, userID string) (*models.Reminder, error) {
	var r models.Reminder
	if err := s.db.Read.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&r).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &r, nil
}

type CreateReminderInput struct {
	LibraryItemID    *string
	RemindAt         time.Time
	ArchiveUntil     *bool
	SendNotification *bool
}

func (s *ReminderService) Create(ctx context.Context, userID string, input CreateReminderInput) (*models.Reminder, error) {
	r := &models.Reminder{
		ID:               uuid.New().String(),
		UserID:           userID,
		LibraryItemID:    input.LibraryItemID,
		RemindAt:         input.RemindAt,
		ArchiveUntil:     input.ArchiveUntil,
		SendNotification: input.SendNotification,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := s.db.Write.WithContext(ctx).Create(r).Error; err != nil {
		return nil, fmt.Errorf("create reminder: %w", err)
	}
	return r, nil
}

type UpdateReminderInput struct {
	RemindAt         *time.Time
	ArchiveUntil     *bool
	SendNotification *bool
}

func (s *ReminderService) Update(ctx context.Context, id, userID string, input UpdateReminderInput) (*models.Reminder, error) {
	updates := map[string]any{"updated_at": time.Now()}
	if input.RemindAt != nil {
		updates["remind_at"] = *input.RemindAt
	}
	if input.ArchiveUntil != nil {
		updates["archive_until"] = *input.ArchiveUntil
	}
	if input.SendNotification != nil {
		updates["send_notification"] = *input.SendNotification
	}
	if err := s.db.Write.WithContext(ctx).Model(&models.Reminder{}).Where("id = ? AND user_id = ?", id, userID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update reminder: %w", err)
	}
	return s.GetByID(ctx, id, userID)
}

func (s *ReminderService) Delete(ctx context.Context, id, userID string) error {
	return s.db.Write.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&models.Reminder{}).Error
}
