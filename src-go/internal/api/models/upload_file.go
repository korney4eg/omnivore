package models

import "time"

// UploadFile mirrors the omnivore.upload_files table.
type UploadFile struct {
	ID          string     `gorm:"column:id;primaryKey"`
	UserID      string     `gorm:"column:user_id"`
	URL         string     `gorm:"column:url"`
	FileName    string     `gorm:"column:file_name"`
	ContentType string     `gorm:"column:content_type"`
	Status      string     `gorm:"column:status"`
	CreatedAt   time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   *time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (UploadFile) TableName() string { return "omnivore.upload_files" }
