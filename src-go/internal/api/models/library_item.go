package models

import "time"

// LibraryItem mirrors the omnivore.library_item table.
type LibraryItem struct {
	ID            string     `gorm:"column:id;primaryKey"`
	UserID        string     `gorm:"column:user_id"`
	State         string     `gorm:"column:state;default:SUCCEEDED"`
	OriginalURL   string     `gorm:"column:original_url"`
	Slug          string     `gorm:"column:slug"`
	Title         string     `gorm:"column:title"`
	Author        *string    `gorm:"column:author"`
	Description   *string    `gorm:"column:description"`
	SavedAt       time.Time  `gorm:"column:saved_at"`
	CreatedAt     time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;autoUpdateTime"`
	PublishedAt   *time.Time `gorm:"column:published_at"`
	ArchivedAt    *time.Time `gorm:"column:archived_at"`
	DeletedAt     *time.Time `gorm:"column:deleted_at"`
	ReadAt        *time.Time `gorm:"column:read_at"`

	ItemLanguage    *string `gorm:"column:item_language"`
	WordCount       *int    `gorm:"column:word_count"`
	SiteName        *string `gorm:"column:site_name"`
	SiteIcon        *string `gorm:"column:site_icon"`
	Thumbnail       *string `gorm:"column:thumbnail"`
	ItemType        string  `gorm:"column:item_type"`
	ContentReader   string  `gorm:"column:content_reader;default:WEB"`
	ReadableContent string  `gorm:"column:readable_content"`
	TextContentHash *string `gorm:"column:text_content_hash"`
	Subscription    *string `gorm:"column:subscription"`
	Note            *string `gorm:"column:note"`
	Folder          string  `gorm:"column:folder"`
	Directionality  string  `gorm:"column:directionality;default:LTR"`
	FeedContent     *string `gorm:"column:feed_content"`

	ReadingProgressTopPercent        float32 `gorm:"column:reading_progress_top_percent"`
	ReadingProgressBottomPercent     float32 `gorm:"column:reading_progress_bottom_percent"`
	ReadingProgressLastReadAnchor    int     `gorm:"column:reading_progress_last_read_anchor"`
	ReadingProgressHighestReadAnchor int     `gorm:"column:reading_progress_highest_read_anchor"`

	UploadFileID *string `gorm:"column:upload_file_id"`

	// Denormalized arrays (populated by DB triggers / service layer)
	LabelNames           StringArray `gorm:"column:label_names;type:text[]"`
	HighlightAnnotations StringArray `gorm:"column:highlight_annotations;type:text[]"`

	// Associations
	Labels     []Label     `gorm:"many2many:omnivore.entity_labels;joinForeignKey:library_item_id;joinReferences:label_id"`
	Highlights []Highlight `gorm:"foreignKey:LibraryItemID"`
}

func (LibraryItem) TableName() string { return "omnivore.library_item" }
