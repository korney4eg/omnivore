package models

import "time"

// Integration mirrors the omnivore.integrations table.
type Integration struct {
	ID              string     `gorm:"column:id;primaryKey"`
	UserID          string     `gorm:"column:user_id"`
	Name            string     `gorm:"column:name"`
	Type            string     `gorm:"column:type;default:EXPORT"`
	Token           string     `gorm:"column:token"`
	Enabled         bool       `gorm:"column:enabled;default:true"`
	SyncedAt        *time.Time `gorm:"column:synced_at"`
	TaskName        *string    `gorm:"column:task_name"`
	ImportItemState *string    `gorm:"column:import_item_state"`
	Settings        JSONMap    `gorm:"column:settings;type:jsonb"`
	CreatedAt       time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (Integration) TableName() string { return "omnivore.integrations" }
