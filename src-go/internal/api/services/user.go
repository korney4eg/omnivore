package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/omnivore-app/omnivore/internal/api/models"
	"github.com/omnivore-app/omnivore/internal/db"
	"github.com/omnivore-app/omnivore/internal/redisutil"
	"gorm.io/gorm"
)

// UserService handles user account operations.
type UserService struct {
	db    *db.DB
	redis *redisutil.RedisDataSource
}

func newUserService(database *db.DB, redis *redisutil.RedisDataSource) *UserService {
	return &UserService{db: database, redis: redis}
}

// GetByID returns a user by primary key.
func (s *UserService) GetByID(ctx context.Context, id string) (*models.User, error) {
	var u models.User
	if err := s.db.Read.WithContext(ctx).
		Preload("Profile").
		Preload("UserPersonalization").
		Where("id = ? AND status != 'DELETED'", id).
		First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

// GetByEmail returns a user by email address.
func (s *UserService) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	if err := s.db.Read.WithContext(ctx).
		Preload("Profile").
		Where("email = ? AND status != 'DELETED'", email).
		First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}

// GetByUsername returns a user by profile username.
func (s *UserService) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	var profile models.UserProfile
	if err := s.db.Read.WithContext(ctx).
		Where("username = ?", username).
		First(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get profile by username: %w", err)
	}
	return s.GetByID(ctx, profile.UserID)
}

// UpdateNameInput is the input for updating a user's display name.
type UpdateNameInput struct {
	Name string
}

// UpdateUser updates mutable user fields.
func (s *UserService) UpdateUser(ctx context.Context, userID string, input UpdateNameInput) (*models.User, error) {
	if err := s.db.Write.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", userID).
		Update("name", input.Name).Error; err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	return s.GetByID(ctx, userID)
}

// UpdateProfileInput is the input for updating a user's profile.
type UpdateProfileInput struct {
	Username   *string
	Bio        *string
	PictureURL *string
}

// UpdateProfile updates the user's profile.
func (s *UserService) UpdateProfile(ctx context.Context, userID string, input UpdateProfileInput) (*models.User, error) {
	updates := map[string]any{}
	if input.Username != nil {
		updates["username"] = *input.Username
	}
	if input.Bio != nil {
		updates["bio"] = *input.Bio
	}
	if input.PictureURL != nil {
		updates["picture_url"] = *input.PictureURL
	}

	if len(updates) > 0 {
		if err := s.db.Write.WithContext(ctx).
			Model(&models.UserProfile{}).
			Where("user_id = ?", userID).
			Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("update profile: %w", err)
		}
	}
	return s.GetByID(ctx, userID)
}

// DeleteUser soft-deletes a user (sets status=DELETED).
func (s *UserService) DeleteUser(ctx context.Context, userID string) error {
	return s.db.Write.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", userID).
		Update("status", "DELETED").Error
}

// GetPersonalization returns the user's personalization settings.
func (s *UserService) GetPersonalization(ctx context.Context, userID string) (*models.UserPersonalization, error) {
	var p models.UserPersonalization
	if err := s.db.Read.WithContext(ctx).
		Where("user_id = ?", userID).
		First(&p).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get personalization: %w", err)
	}
	return &p, nil
}

// SetPersonalizationInput holds all personalisation fields.
type SetPersonalizationInput struct {
	FontFamily          *string
	FontSize            *int
	Margin              *int
	Theme               *string
	LibraryLayoutType   *string
	LibrarySortOrder    *string
	SpeechVoice         *string
	SpeechSecondaryVoice *string
	SpeechRate          *string
	SpeechVolume        *string
	Shortcuts           models.JSONMap
	DigestConfig        models.JSONMap
}

// SetPersonalization upserts the user's personalization settings.
func (s *UserService) SetPersonalization(ctx context.Context, userID string, input SetPersonalizationInput) (*models.UserPersonalization, error) {
	updates := map[string]any{
		"user_id":    userID,
		"updated_at": time.Now(),
	}
	if input.FontFamily != nil {
		updates["font_family"] = *input.FontFamily
	}
	if input.FontSize != nil {
		updates["font_size"] = *input.FontSize
	}
	if input.Margin != nil {
		updates["margin"] = *input.Margin
	}
	if input.Theme != nil {
		updates["theme"] = *input.Theme
	}
	if input.LibraryLayoutType != nil {
		updates["library_layout_type"] = *input.LibraryLayoutType
	}
	if input.LibrarySortOrder != nil {
		updates["library_sort_order"] = *input.LibrarySortOrder
	}
	if input.SpeechVoice != nil {
		updates["speech_voice"] = *input.SpeechVoice
	}
	if input.SpeechSecondaryVoice != nil {
		updates["speech_secondary_voice"] = *input.SpeechSecondaryVoice
	}
	if input.SpeechRate != nil {
		updates["speech_rate"] = *input.SpeechRate
	}
	if input.SpeechVolume != nil {
		updates["speech_volume"] = *input.SpeechVolume
	}
	if input.Shortcuts != nil {
		updates["shortcuts"] = input.Shortcuts
	}
	if input.DigestConfig != nil {
		updates["digest_config"] = input.DigestConfig
	}

	// Upsert: INSERT ... ON CONFLICT (user_id) DO UPDATE
	err := s.db.Write.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.UserPersonalization
		if err := tx.Where("user_id = ?", userID).First(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				updates["id"] = uuid.New().String()
				updates["created_at"] = time.Now()
				return tx.Model(&models.UserPersonalization{}).Create(updates).Error
			}
			return err
		}
		return tx.Model(&existing).Updates(updates).Error
	})
	if err != nil {
		return nil, fmt.Errorf("set personalization: %w", err)
	}
	return s.GetPersonalization(ctx, userID)
}
