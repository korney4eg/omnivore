// Package queue provides BullMQ queue workers and dispatchers.
package queue

import (
	"context"
	"fmt"

	"github.com/omnivore-app/omnivore/internal/bullmq"
	"github.com/redis/go-redis/v9"
)

// Dispatcher enqueues jobs to BullMQ queues.
// It is used by the API layer (services/resolvers) to schedule background work.
type Dispatcher struct {
	redis *redis.Client
}

// NewDispatcher creates a new queue dispatcher.
func NewDispatcher(redisClient *redis.Client) *Dispatcher {
	return &Dispatcher{redis: redisClient}
}

// --- Priority helpers (match TS getPriority) ---

const (
	PriorityImmediate = 1   // update-labels, send-email, update-highlight
	PriorityFast      = 5   // AI tasks, high-priority RSS
	PriorityMedium    = 10  // bulk-action, trigger-rule, refresh-feed
	PriorityLow       = 50  // create-digest, low-priority refresh-feed
	PriorityBackground = 100 // refresh-all-feeds, prune-trash, expire-folders, export
)

var defaultBackoff = bullmq.BackoffOpt{Type: "exponential", Delay: 2000}

// --- Enqueue helpers for specific job types ---

// EnqueueFetchContent enqueues a content-fetch job.
func (d *Dispatcher) EnqueueFetchContent(ctx context.Context, data FetchContentJobData) error {
	priority := PriorityImmediate
	attempts := 3
	if data.RSSFeedURL != "" {
		priority = PriorityFast
		attempts = 2
	}
	return d.enqueue(ctx, bullmq.ContentFetchQueue, bullmq.FetchContentJob, data, priority, attempts)
}

// EnqueueUpdateLabels enqueues a label sync job for a library item.
func (d *Dispatcher) EnqueueUpdateLabels(ctx context.Context, libraryItemID, userID string) error {
	return d.enqueue(ctx, bullmq.BackendQueue, bullmq.UpdateLabelsJob,
		UpdateLabelsJobData{LibraryItemID: libraryItemID, UserID: userID},
		PriorityImmediate, 6)
}

// EnqueueUpdateHighlight enqueues a highlight sync job for a library item.
func (d *Dispatcher) EnqueueUpdateHighlight(ctx context.Context, libraryItemID, userID string) error {
	return d.enqueue(ctx, bullmq.BackendQueue, bullmq.UpdateHighlightJob,
		UpdateHighlightJobData{LibraryItemID: libraryItemID, UserID: userID},
		PriorityImmediate, 6)
}

// EnqueueTriggerRule enqueues a rule trigger job.
func (d *Dispatcher) EnqueueTriggerRule(ctx context.Context, userID, ruleEventType string, itemData map[string]any) error {
	return d.enqueue(ctx, bullmq.BackendQueue, bullmq.TriggerRuleJob,
		TriggerRuleJobData{UserID: userID, RuleEventType: ruleEventType, Data: itemData},
		PriorityMedium, 1)
}

// EnqueueCallWebhook enqueues a webhook dispatch job.
func (d *Dispatcher) EnqueueCallWebhook(ctx context.Context, userID, eventType, action string, data any) error {
	return d.enqueue(ctx, bullmq.BackendQueue, bullmq.CallWebhookJob,
		CallWebhookJobData{UserID: userID, Type: eventType, Action: action, Data: data},
		PriorityMedium, 1)
}

// EnqueueRefreshFeed enqueues an RSS feed refresh job.
func (d *Dispatcher) EnqueueRefreshFeed(ctx context.Context, data RefreshFeedJobData) error {
	priority := PriorityMedium
	if data.Priority == "low" {
		priority = PriorityLow
	}
	return d.enqueue(ctx, bullmq.BackendQueue, bullmq.RefreshFeedJob, data, priority, 2)
}

// EnqueueRefreshAllFeeds enqueues a job to refresh all RSS subscriptions.
func (d *Dispatcher) EnqueueRefreshAllFeeds(ctx context.Context) error {
	return d.enqueue(ctx, bullmq.BackendQueue, bullmq.RefreshAllFeedsJob,
		map[string]string{}, PriorityBackground, 1)
}

// EnqueueBulkAction enqueues a batch operation job.
func (d *Dispatcher) EnqueueBulkAction(ctx context.Context, data BulkActionJobData) error {
	return d.enqueue(ctx, bullmq.BackendQueue, bullmq.BulkActionJob,
		data, PriorityMedium, 1)
}

// EnqueueSendEmail enqueues an email sending job.
func (d *Dispatcher) EnqueueSendEmail(ctx context.Context, data SendEmailJobData) error {
	return d.enqueue(ctx, bullmq.BackendQueue, bullmq.SendEmailJob,
		data, PriorityImmediate, 1)
}

// EnqueuePruneTrash enqueues a trash cleanup job.
func (d *Dispatcher) EnqueuePruneTrash(ctx context.Context) error {
	return d.enqueue(ctx, bullmq.BackendQueue, bullmq.PruneTrashJob,
		map[string]string{}, PriorityBackground, 1)
}

// EnqueueExpireFolders enqueues a folder expiration job.
func (d *Dispatcher) EnqueueExpireFolders(ctx context.Context) error {
	return d.enqueue(ctx, bullmq.BackendQueue, bullmq.ExpireFoldersJob,
		map[string]string{}, PriorityBackground, 1)
}

// enqueue is the internal helper that adds a single job to a queue.
func (d *Dispatcher) enqueue(ctx context.Context, queueName, jobName string, data any, priority, attempts int) error {
	err := bullmq.AddBulk(ctx, d.redis, queueName, []bullmq.AddJobOpts{
		{
			Name: jobName,
			Data: data,
			Opts: bullmq.JobOpts{
				Priority: priority,
				Attempts: attempts,
				Backoff:  defaultBackoff,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("enqueue %s to %s: %w", jobName, queueName, err)
	}
	return nil
}

// --- Job data structures ---

// FetchContentJobData is enqueued when a user saves a URL.
type FetchContentJobData struct {
	URL        string `json:"url"`
	UserID     string `json:"userId,omitempty"`
	SaveRequestID string `json:"saveRequestId"`
	Labels     []LabelInput `json:"labels,omitempty"`
	Source     string `json:"source,omitempty"`
	Folder     string `json:"folder,omitempty"`
	RSSFeedURL string `json:"rssFeedUrl,omitempty"`
	SavedAt    string `json:"savedAt,omitempty"`
	PublishedAt string `json:"publishedAt,omitempty"`
	Priority   string `json:"priority"` // "high" | "low"
	TaskID     string `json:"taskId,omitempty"`
}

// LabelInput for job payloads.
type LabelInput struct {
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

// UpdateLabelsJobData is enqueued when labels change on an item.
type UpdateLabelsJobData struct {
	LibraryItemID string `json:"libraryItemId"`
	UserID        string `json:"userId"`
}

// UpdateHighlightJobData is enqueued when highlights change on an item.
type UpdateHighlightJobData struct {
	LibraryItemID string `json:"libraryItemId"`
	UserID        string `json:"userId"`
}

// TriggerRuleJobData is enqueued when an event matches rule conditions.
type TriggerRuleJobData struct {
	UserID        string         `json:"userId"`
	RuleEventType string         `json:"ruleEventType"`
	Data          map[string]any `json:"data"`
}

// CallWebhookJobData is enqueued for webhook dispatch.
type CallWebhookJobData struct {
	UserID string `json:"userId"`
	Type   string `json:"type"`   // e.g. "page", "label", "highlight"
	Action string `json:"action"` // e.g. "created", "updated"
	Data   any    `json:"data"`
}

// RefreshFeedJobData is enqueued for RSS feed refresh.
type RefreshFeedJobData struct {
	SubscriptionIDs      []string `json:"subscriptionIds"`
	FeedURL              string   `json:"feedUrl"`
	MostRecentItemDates  []int64  `json:"mostRecentItemDates"`
	ScheduledTimestamps  []int64  `json:"scheduledTimestamps"`
	LastFetchedChecksums []string `json:"lastFetchedChecksums"`
	UserIDs              []string `json:"userIds"`
	Folders              []string `json:"folders"`
	Priority             string   `json:"priority,omitempty"` // "low" | "high"
}

// BulkActionJobData is enqueued for batch operations.
type BulkActionJobData struct {
	Count     int      `json:"count"`
	UserID    string   `json:"userId"`
	Action    string   `json:"action"` // Archive, Delete, AddLabel, etc.
	Query     string   `json:"query"`
	BatchSize int      `json:"batchSize"`
	LabelIDs  []string `json:"labelIds,omitempty"`
}

// SendEmailJobData is enqueued for email sending.
type SendEmailJobData struct {
	UserID     string         `json:"userId"`
	To         string         `json:"to,omitempty"`
	From       string         `json:"from,omitempty"`
	Subject    string         `json:"subject,omitempty"`
	ReplyTo    string         `json:"replyTo,omitempty"`
	HTML       string         `json:"html,omitempty"`
	Text       string         `json:"text,omitempty"`
	TemplateID string         `json:"templateId,omitempty"`
	TemplateData map[string]any `json:"dynamicTemplateData,omitempty"`
}
