package models

import "time"

// Highlight mirrors the omnivore.highlight table.
type Highlight struct {
	ID                        string     `gorm:"column:id;primaryKey"`
	ShortID                   string     `gorm:"column:short_id"`
	UserID                    string     `gorm:"column:user_id"`
	LibraryItemID             string     `gorm:"column:library_item_id"`
	Quote                     *string    `gorm:"column:quote"`
	Prefix                    *string    `gorm:"column:prefix"`
	Suffix                    *string    `gorm:"column:suffix"`
	Patch                     *string    `gorm:"column:patch"`
	Annotation                *string    `gorm:"column:annotation"`
	SharedAt                  *time.Time `gorm:"column:shared_at"`
	HighlightPositionPercent  float32    `gorm:"column:highlight_position_percent;default:0"`
	HighlightPositionAnchorIndex int     `gorm:"column:highlight_position_anchor_index;default:0"`
	HighlightType             string     `gorm:"column:highlight_type;default:HIGHLIGHT"`
	HTML                      *string    `gorm:"column:html"`
	Color                     *string    `gorm:"column:color"`
	Representation            string     `gorm:"column:representation;default:CONTENT"`
	CreatedAt                 time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt                 time.Time  `gorm:"column:updated_at;autoUpdateTime"`

	// Associations
	Labels []Label `gorm:"many2many:omnivore.entity_labels;joinForeignKey:highlight_id;joinReferences:label_id"`
}

func (Highlight) TableName() string { return "omnivore.highlight" }
