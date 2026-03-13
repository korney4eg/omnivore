package models

import "time"

// Subscription mirrors the omnivore.subscriptions table (email + RSS).
type Subscription struct {
	ID                 string     `gorm:"column:id;primaryKey"`
	UserID             string     `gorm:"column:user_id"`
	Name               string     `gorm:"column:name"`
	Status             string     `gorm:"column:status;default:ACTIVE"`
	Type               string     `gorm:"column:type;default:NEWSLETTER"`
	Description        *string    `gorm:"column:description"`
	URL                *string    `gorm:"column:url"`
	Count              int        `gorm:"column:count;default:0"`
	Folder             *string    `gorm:"column:folder"`
	Icon               *string    `gorm:"column:icon"`
	NewsletterEmailID  *string    `gorm:"column:newsletter_email_id"`
	IsPrivate          bool       `gorm:"column:is_private;default:false"`
	AutoAddToLibrary   *bool      `gorm:"column:auto_add_to_library"`
	FetchContent       bool       `gorm:"column:fetch_content;default:false"`
	FetchContentType   string     `gorm:"column:fetch_content_type;default:WHEN_EMPTY"`
	ScheduledAt        *time.Time `gorm:"column:scheduled_at"`
	RefreshedAt        *time.Time `gorm:"column:refreshed_at"`
	FailedAt           *time.Time `gorm:"column:failed_at"`
	CreatedAt          time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (Subscription) TableName() string { return "omnivore.subscriptions" }

// NewsletterEmail mirrors the omnivore.newsletter_emails table.
type NewsletterEmail struct {
	ID               string    `gorm:"column:id;primaryKey"`
	UserID           string    `gorm:"column:user_id"`
	Address          string    `gorm:"column:address"`
	ConfirmationCode *string   `gorm:"column:confirmation_code"`
	Name             *string   `gorm:"column:name"`
	Description      *string   `gorm:"column:description"`
	Folder           *string   `gorm:"column:folder"`
	CreatedAt        time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt        time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (NewsletterEmail) TableName() string { return "omnivore.newsletter_emails" }
