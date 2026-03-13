package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/omnivore-app/omnivore/internal/api/models"
	"github.com/omnivore-app/omnivore/internal/config"
	"github.com/omnivore-app/omnivore/internal/db"
	"github.com/omnivore-app/omnivore/internal/storage"
	"gorm.io/gorm"
)

type UploadFileService struct {
	db    *db.DB
	cfg   *config.Config
	store *storage.Client
}

func newUploadFileService(database *db.DB, cfg *config.Config, store *storage.Client) *UploadFileService {
	return &UploadFileService{db: database, cfg: cfg, store: store}
}

func (s *UploadFileService) GetByID(ctx context.Context, id string) (*models.UploadFile, error) {
	var f models.UploadFile
	if err := s.db.Read.WithContext(ctx).Where("id = ?", id).First(&f).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &f, nil
}

type CreateUploadRequestInput struct {
	URL         string
	FileName    string
	ContentType string
}

type CreateUploadRequestResult struct {
	UploadFile    *models.UploadFile
	UploadFileKey string // the storage object key for this upload
}

func (s *UploadFileService) Create(ctx context.Context, userID string, input CreateUploadRequestInput) (*CreateUploadRequestResult, error) {
	id := uuid.New().String()
	now := time.Now()
	uploadKey := fmt.Sprintf("uploads/%s/%s/%s", userID, id, input.FileName)

	f := &models.UploadFile{
		ID:          id,
		UserID:      userID,
		URL:         input.URL,
		FileName:    input.FileName,
		ContentType: input.ContentType,
		Status:      "INITIALIZED",
		CreatedAt:   now,
	}
	if err := s.db.Write.WithContext(ctx).Create(f).Error; err != nil {
		return nil, fmt.Errorf("create upload_file: %w", err)
	}
	return &CreateUploadRequestResult{UploadFile: f, UploadFileKey: uploadKey}, nil
}

func (s *UploadFileService) SetComplete(ctx context.Context, id, userID string) (*models.UploadFile, error) {
	if err := s.db.Write.WithContext(ctx).Model(&models.UploadFile{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("status", "COMPLETED").Error; err != nil {
		return nil, fmt.Errorf("set upload complete: %w", err)
	}
	return s.GetByID(ctx, id)
}
