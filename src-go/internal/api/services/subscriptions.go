package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/omnivore-app/omnivore/internal/api/models"
	"github.com/omnivore-app/omnivore/internal/db"
	"gorm.io/gorm"
)

type SubscriptionService struct{ db *db.DB }

func newSubscriptionService(database *db.DB) *SubscriptionService {
	return &SubscriptionService{db: database}
}

func (s *SubscriptionService) List(ctx context.Context, userID string, subType *string) ([]models.Subscription, error) {
	q := s.db.Read.WithContext(ctx).
		Where("user_id = ? AND status != 'DELETED'", userID)
	if subType != nil {
		q = q.Where("type = ?", *subType)
	}
	var subs []models.Subscription
	if err := q.Order("name ASC").Find(&subs).Error; err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}
	return subs, nil
}

func (s *SubscriptionService) GetByID(ctx context.Context, id, userID string) (*models.Subscription, error) {
	var sub models.Subscription
	if err := s.db.Read.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&sub).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &sub, nil
}

type UpdateSubscriptionInput struct {
	Name             *string
	Description      *string
	Folder           *string
	AutoAddToLibrary *bool
	FetchContent     *bool
	IsPrivate        *bool
	Status           *string
}

func (s *SubscriptionService) Update(ctx context.Context, id, userID string, input UpdateSubscriptionInput) (*models.Subscription, error) {
	updates := map[string]any{"updated_at": time.Now()}
	if input.Name != nil {
		updates["name"] = *input.Name
	}
	if input.Description != nil {
		updates["description"] = *input.Description
	}
	if input.Folder != nil {
		updates["folder"] = *input.Folder
	}
	if input.AutoAddToLibrary != nil {
		updates["auto_add_to_library"] = *input.AutoAddToLibrary
	}
	if input.FetchContent != nil {
		updates["fetch_content"] = *input.FetchContent
	}
	if input.IsPrivate != nil {
		updates["is_private"] = *input.IsPrivate
	}
	if input.Status != nil {
		updates["status"] = *input.Status
	}

	if err := s.db.Write.WithContext(ctx).Model(&models.Subscription{}).
		Where("id = ? AND user_id = ?", id, userID).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("update subscription: %w", err)
	}
	return s.GetByID(ctx, id, userID)
}

// Unsubscribe marks a subscription as UNSUBSCRIBED.
func (s *SubscriptionService) Unsubscribe(ctx context.Context, id, userID string) error {
	return s.db.Write.WithContext(ctx).
		Model(&models.Subscription{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]any{"status": "UNSUBSCRIBED", "updated_at": time.Now()}).Error
}

// ListNewsletterEmails returns all newsletter email addresses for a user.
func (s *SubscriptionService) ListNewsletterEmails(ctx context.Context, userID string) ([]models.NewsletterEmail, error) {
	var emails []models.NewsletterEmail
	if err := s.db.Read.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&emails).Error; err != nil {
		return nil, fmt.Errorf("list newsletter emails: %w", err)
	}
	return emails, nil
}
