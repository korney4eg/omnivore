package models

import "time"

// Feed mirrors the omnivore.feeds table (RSS/Atom feeds).
type Feed struct {
	ID          string     `gorm:"column:id;primaryKey"`
	Title       string     `gorm:"column:title"`
	URL         string     `gorm:"column:url"`
	Author      *string    `gorm:"column:author"`
	Description *string    `gorm:"column:description"`
	Image       *string    `gorm:"column:image"`
	PublishedAt *time.Time `gorm:"column:published_at"`
	CreatedAt   time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (Feed) TableName() string { return "omnivore.feeds" }
