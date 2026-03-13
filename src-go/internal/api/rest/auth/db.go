package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/omnivore-app/omnivore/internal/config"
	"github.com/omnivore-app/omnivore/internal/db"
	"github.com/omnivore-app/omnivore/internal/redisutil"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// userRow is the minimal projection of the users table needed for auth.
type userRow struct {
	ID       string  `gorm:"column:id"`
	Email    string  `gorm:"column:email"`
	Password *string `gorm:"column:password"`
	Status   string  `gorm:"column:status"`
	Role     string  `gorm:"column:role"`
}

func (userRow) TableName() string { return "omnivore.users" }

func findUserByEmail(ctx context.Context, database *db.DB, email string) (*userRow, error) {
	var u userRow
	if err := database.Read.WithContext(ctx).
		Select("id, email, password, status, role").
		Where("email = ? AND status != 'DELETED'", email).
		First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func findUserByID(ctx context.Context, database *db.DB, id string) (*userRow, error) {
	var u userRow
	if err := database.Read.WithContext(ctx).
		Select("id, email, password, status, role").
		Where("id = ?", id).
		First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func emailExists(ctx context.Context, database *db.DB, email string) (bool, error) {
	var count int64
	err := database.Read.WithContext(ctx).
		Model(&userRow{}).
		Where("email = ?", email).
		Count(&count).Error
	return count > 0, err
}

// profileRow is used to check username uniqueness.
type profileRow struct {
	Username string `gorm:"column:username"`
}

func (profileRow) TableName() string { return "omnivore.user_profile" }

func usernameExists(ctx context.Context, database *db.DB, username string) (bool, error) {
	var count int64
	err := database.Read.WithContext(ctx).
		Model(&profileRow{}).
		Where("username = ?", username).
		Count(&count).Error
	return count > 0, err
}

// createUser inserts a new user plus their profile in a transaction.
func createUser(ctx context.Context, database *db.DB, name, email, username, passwordHash string, verified bool) (*userRow, error) {
	id := uuid.New().String()
	now := time.Now()

	status := "PENDING"
	if verified {
		status = "ACTIVE"
	}

	err := database.Write.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(
			`INSERT INTO omnivore.users (id, name, email, source, source_user_id, password, status, created_at, updated_at)
			 VALUES (?, ?, ?, 'EMAIL', ?, ?, ?, ?, ?)`,
			id, name, email, email, passwordHash, status, now, now,
		).Error; err != nil {
			return fmt.Errorf("insert user: %w", err)
		}

		profileID := uuid.New().String()
		if err := tx.Exec(
			`INSERT INTO omnivore.user_profile (id, user_id, username, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?)`,
			profileID, id, username, now, now,
		).Error; err != nil {
			return fmt.Errorf("insert profile: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &userRow{ID: id, Email: email, Status: status}, nil
}

func updatePassword(ctx context.Context, database *db.DB, userID, passwordHash string) error {
	return database.Write.WithContext(ctx).
		Model(&userRow{}).
		Where("id = ?", userID).
		Update("password", passwordHash).Error
}

const (
	tokenTTL         = 24 * time.Hour
	resetTokenPrefix = "password-reset:"
	verifyTokenPrefix = "email-verify:"
)

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func verifyEmailToken(ctx context.Context, database *db.DB, redis *redisutil.RedisDataSource, token string) error {
	key := verifyTokenPrefix + token
	userID, err := redis.CacheClient.GetDel(ctx, key).Result()
	if err != nil || userID == "" {
		return fmt.Errorf("invalid or expired token")
	}
	return database.Write.WithContext(ctx).
		Model(&userRow{}).
		Where("id = ?", userID).
		Update("status", "ACTIVE").Error
}

func sendPasswordResetEmail(ctx context.Context, database *db.DB, redis *redisutil.RedisDataSource, email string, cfg *config.Config) error {
	user, err := findUserByEmail(ctx, database, email)
	if err != nil || user == nil {
		return nil // silent: do not leak email existence
	}

	token, err := generateToken()
	if err != nil {
		return err
	}

	key := resetTokenPrefix + token
	if err := redis.CacheClient.Set(ctx, key, user.ID, tokenTTL).Err(); err != nil {
		return err
	}

	// TODO: integrate Sendgrid email sending
	_ = cfg.SendgridAPIKey
	return nil
}

func resetPassword(ctx context.Context, database *db.DB, redis *redisutil.RedisDataSource, token, newHash string) error {
	key := resetTokenPrefix + token
	userID, err := redis.CacheClient.GetDel(ctx, key).Result()
	if isRedisNotFound(err) || err != nil || userID == "" {
		return fmt.Errorf("invalid or expired token")
	}
	return updatePassword(ctx, database, userID, newHash)
}

// isRedisNotFound returns true when the error is redis.Nil (key not found).
func isRedisNotFound(err error) bool {
	return err == redis.Nil
}
