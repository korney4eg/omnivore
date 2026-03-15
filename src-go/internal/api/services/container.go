// Package services contains the business logic for the Omnivore API.
// Each service receives *db.DB and *redis.Client; no HTTP concerns here.
package services

import (
	"github.com/omnivore-app/omnivore/internal/config"
	"github.com/omnivore-app/omnivore/internal/db"
	"github.com/omnivore-app/omnivore/internal/queue"
	"github.com/omnivore-app/omnivore/internal/redisutil"
	"github.com/omnivore-app/omnivore/internal/storage"
)

// Container holds all service instances and is passed to resolvers.
type Container struct {
	Users         *UserService
	LibraryItems  *LibraryItemService
	Labels        *LabelService
	Highlights    *HighlightService
	Subscriptions *SubscriptionService
	Webhooks      *WebhookService
	Rules         *RuleService
	APIKeys       *APIKeyService
	Reminders     *ReminderService
	Integrations  *IntegrationService
	Filters       *FilterService
	UploadFiles   *UploadFileService
	Queue         *queue.Dispatcher
}

// New initialises all services with shared infrastructure.
func New(cfg *config.Config, database *db.DB, redis *redisutil.RedisDataSource, store *storage.Client) *Container {
	return &Container{
		Users:         newUserService(database, redis),
		LibraryItems:  newLibraryItemService(database, redis),
		Labels:        newLabelService(database),
		Highlights:    newHighlightService(database),
		Subscriptions: newSubscriptionService(database),
		Webhooks:      newWebhookService(database),
		Rules:         newRuleService(database),
		APIKeys:       newAPIKeyService(database, redis),
		Reminders:     newReminderService(database),
		Integrations:  newIntegrationService(database),
		Filters:       newFilterService(database),
		UploadFiles:   newUploadFileService(database, cfg, store),
		Queue:         queue.NewDispatcher(redis.MQClient),
	}
}
