package models

import "time"

// Rule mirrors the omnivore.rules table.
type Rule struct {
	ID          string      `gorm:"column:id;primaryKey"`
	UserID      string      `gorm:"column:user_id"`
	Name        string      `gorm:"column:name"`
	Filter      string      `gorm:"column:filter"`
	Actions     JSONMap     `gorm:"column:actions;type:jsonb"`
	Description *string     `gorm:"column:description"`
	EventTypes  StringArray `gorm:"column:event_types;type:text[]"`
	Enabled     bool        `gorm:"column:enabled;default:true"`
	FailedAt    *time.Time  `gorm:"column:failed_at"`
	CreatedAt   time.Time   `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time   `gorm:"column:updated_at;autoUpdateTime"`
}

func (Rule) TableName() string { return "omnivore.rules" }
