package models

import "time"

// Label mirrors the omnivore.labels table.
type Label struct {
	ID          string     `gorm:"column:id;primaryKey"`
	UserID      string     `gorm:"column:user_id"`
	Name        string     `gorm:"column:name"`
	Color       string     `gorm:"column:color"`
	Description *string    `gorm:"column:description"`
	Position    int        `gorm:"column:position;default:0"`
	Internal    bool       `gorm:"column:internal;default:false"`
	CreatedAt   time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   *time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (Label) TableName() string { return "omnivore.labels" }

// EntityLabel mirrors the omnivore.entity_labels join table.
type EntityLabel struct {
	ID            string  `gorm:"column:id;primaryKey"`
	LabelID       string  `gorm:"column:label_id"`
	LibraryItemID *string `gorm:"column:library_item_id"`
	HighlightID   *string `gorm:"column:highlight_id"`
	Source        string  `gorm:"column:source;default:user"`
}

func (EntityLabel) TableName() string { return "omnivore.entity_labels" }
