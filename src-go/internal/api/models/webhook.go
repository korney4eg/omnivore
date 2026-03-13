package models

import "time"

// Webhook mirrors the omnivore.webhooks table.
type Webhook struct {
	ID          string      `gorm:"column:id;primaryKey"`
	UserID      string      `gorm:"column:user_id"`
	URL         string      `gorm:"column:url"`
	EventTypes  StringArray `gorm:"column:event_types;type:text[]"`
	Method      string      `gorm:"column:method;default:POST"`
	ContentType string      `gorm:"column:content_type;default:application/json"`
	Enabled     bool        `gorm:"column:enabled;default:true"`
	CreatedAt   time.Time   `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time   `gorm:"column:updated_at;autoUpdateTime"`
}

func (Webhook) TableName() string { return "omnivore.webhooks" }
