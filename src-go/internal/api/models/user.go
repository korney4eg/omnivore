package models

import "time"

// User mirrors the omnivore.users table.
type User struct {
	ID           string    `gorm:"column:id;primaryKey"`
	Name         string    `gorm:"column:name"`
	Source       string    `gorm:"column:source"`
	Email        string    `gorm:"column:email"`
	SourceUserID string    `gorm:"column:source_user_id"`
	Password     *string   `gorm:"column:password"`
	Status       string    `gorm:"column:status"`
	Role         string    `gorm:"column:role"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime"`

	// Associations
	Profile           *UserProfile       `gorm:"foreignKey:UserID"`
	UserPersonalization *UserPersonalization `gorm:"foreignKey:UserID"`
}

func (User) TableName() string { return "omnivore.users" }

// UserProfile mirrors the omnivore.user_profile table.
type UserProfile struct {
	ID         string    `gorm:"column:id;primaryKey"`
	UserID     string    `gorm:"column:user_id"`
	Username   string    `gorm:"column:username"`
	Bio        *string   `gorm:"column:bio"`
	PictureURL *string   `gorm:"column:picture_url"`
	Private    bool      `gorm:"column:private;default:false"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (UserProfile) TableName() string { return "omnivore.user_profile" }

// UserPersonalization mirrors the omnivore.user_personalization table.
type UserPersonalization struct {
	ID                  string          `gorm:"column:id;primaryKey"`
	UserID              string          `gorm:"column:user_id"`
	FontFamily          *string         `gorm:"column:font_family"`
	FontSize            *int            `gorm:"column:font_size"`
	Margin              *int            `gorm:"column:margin"`
	Theme               *string         `gorm:"column:theme"`
	LibraryLayoutType   *string         `gorm:"column:library_layout_type"`
	LibrarySortOrder    *string         `gorm:"column:library_sort_order"`
	SpeechVoice         *string         `gorm:"column:speech_voice"`
	SpeechSecondaryVoice *string        `gorm:"column:speech_secondary_voice"`
	SpeechRate          *string         `gorm:"column:speech_rate"`
	SpeechVolume        *string         `gorm:"column:speech_volume"`
	Shortcuts           JSONMap         `gorm:"column:shortcuts;type:jsonb"`
	DigestConfig        JSONMap         `gorm:"column:digest_config;type:jsonb"`
	Fields              JSONMap         `gorm:"column:fields;type:jsonb"`
	CreatedAt           time.Time       `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt           time.Time       `gorm:"column:updated_at;autoUpdateTime"`
}

func (UserPersonalization) TableName() string { return "omnivore.user_personalization" }
