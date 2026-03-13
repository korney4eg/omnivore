package models

import "time"

// Filter mirrors the omnivore.filters table (saved search filters).
type Filter struct {
	ID            string    `gorm:"column:id;primaryKey"`
	UserID        string    `gorm:"column:user_id"`
	Name          string    `gorm:"column:name"`
	Description   *string   `gorm:"column:description"`
	Filter        string    `gorm:"column:filter"`
	Category      string    `gorm:"column:category;default:Search"`
	Position      int       `gorm:"column:position;default:0"`
	DefaultFilter bool      `gorm:"column:default_filter;default:false"`
	Visible       bool      `gorm:"column:visible;default:true"`
	Folder        *string   `gorm:"column:folder"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (Filter) TableName() string { return "omnivore.filters" }
