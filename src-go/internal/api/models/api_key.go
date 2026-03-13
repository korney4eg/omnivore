package models

import "time"

// APIKey mirrors the omnivore.api_keys table.
type APIKey struct {
	ID        string     `gorm:"column:id;primaryKey"`
	UserID    string     `gorm:"column:user_id"`
	Name      string     `gorm:"column:name"`
	KeyHash   string     `gorm:"column:key_hash"` // SHA256 of the raw UUID key
	Scopes    StringArray `gorm:"column:scopes;type:text[]"`
	ExpiresAt time.Time  `gorm:"column:expires_at"`
	UsedAt    *time.Time `gorm:"column:used_at"`
	CreatedAt time.Time  `gorm:"column:created_at;autoCreateTime"`
}

func (APIKey) TableName() string { return "omnivore.api_keys" }
