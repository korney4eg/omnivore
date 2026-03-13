package models

import "time"

// Reminder mirrors the omnivore.reminders table.
type Reminder struct {
	ID                  string     `gorm:"column:id;primaryKey"`
	UserID              string     `gorm:"column:user_id"`
	LibraryItemID       *string    `gorm:"column:library_item_id"`
	ArchiveUntil        *bool      `gorm:"column:archive_until"`
	SendNotification    *bool      `gorm:"column:send_notification"`
	RemindAt            time.Time  `gorm:"column:remind_at"`
	Status              *string    `gorm:"column:status"`
	CreatedAt           time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt           time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (Reminder) TableName() string { return "omnivore.reminders" }
