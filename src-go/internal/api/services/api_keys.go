package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/omnivore-app/omnivore/internal/api/models"
	"github.com/omnivore-app/omnivore/internal/db"
	"github.com/omnivore-app/omnivore/internal/redisutil"
	"gorm.io/gorm"
)

type APIKeyService struct {
	db    *db.DB
	redis *redisutil.RedisDataSource
}

func newAPIKeyService(database *db.DB, redis *redisutil.RedisDataSource) *APIKeyService {
	return &APIKeyService{db: database, redis: redis}
}

func (s *APIKeyService) List(ctx context.Context, userID string) ([]models.APIKey, error) {
	var keys []models.APIKey
	err := s.db.Read.WithContext(ctx).
		Where("user_id = ? AND expires_at > NOW()", userID).
		Order("created_at DESC").Find(&keys).Error
	return keys, err
}

type GenerateAPIKeyInput struct {
	Name      string
	ExpiresAt time.Time
	Scopes    []string
}

// Generate creates a new API key, returning the raw UUID (shown once to the user).
func (s *APIKeyService) Generate(ctx context.Context, userID string, input GenerateAPIKeyInput) (rawKey string, key *models.APIKey, err error) {
	raw := make([]byte, 16)
	if _, err = rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate key: %w", err)
	}
	rawKey = uuid.Must(uuid.FromBytes(raw)).String()
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	k := &models.APIKey{
		ID:        uuid.New().String(),
		UserID:    userID,
		Name:      input.Name,
		KeyHash:   keyHash,
		Scopes:    input.Scopes,
		ExpiresAt: input.ExpiresAt,
		CreatedAt: time.Now(),
	}
	if err = s.db.Write.WithContext(ctx).Create(k).Error; err != nil {
		return "", nil, fmt.Errorf("create api key: %w", err)
	}
	return rawKey, k, nil
}

// Revoke deletes an API key and removes it from the Redis cache.
func (s *APIKeyService) Revoke(ctx context.Context, id, userID string) (*models.APIKey, error) {
	var k models.APIKey
	if err := s.db.Write.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&k).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	if err := s.db.Write.WithContext(ctx).Delete(&k).Error; err != nil {
		return nil, fmt.Errorf("revoke api key: %w", err)
	}

	// Evict from cache
	_ = s.redis.CacheClient.Del(ctx, "api-key:"+k.KeyHash)

	return &k, nil
}
