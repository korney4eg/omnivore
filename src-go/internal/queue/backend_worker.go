package queue

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/omnivore-app/omnivore/internal/bullmq"
	"github.com/omnivore-app/omnivore/internal/config"
	"github.com/omnivore-app/omnivore/internal/db"
	"github.com/omnivore-app/omnivore/internal/redisutil"
	"github.com/omnivore-app/omnivore/internal/storage"
)

const (
	backendConcurrency  = 2
	backendPollInterval = 500 * time.Millisecond
)

// BackendWorker processes jobs from the omnivore-backend-queue.
// It handles save-page, update-labels, trigger-rule, webhooks, etc.
type BackendWorker struct {
	ctx     context.Context
	cfg     *config.Config
	db      *db.DB
	redis   *redisutil.RedisDataSource
	store   *storage.Client
	wg      sync.WaitGroup
	sem     chan struct{}
}

// NewBackendWorker creates a new backend queue worker.
func NewBackendWorker(
	ctx context.Context,
	cfg *config.Config,
	database *db.DB,
	redis *redisutil.RedisDataSource,
	store *storage.Client,
) *BackendWorker {
	return &BackendWorker{
		ctx:   ctx,
		cfg:   cfg,
		db:    database,
		redis: redis,
		store: store,
		sem:   make(chan struct{}, backendConcurrency),
	}
}

// Start begins processing jobs in the background.
func (w *BackendWorker) Start() {
	w.wg.Add(1)
	go w.run()
}

// Wait blocks until the worker has drained all active jobs.
func (w *BackendWorker) Wait() {
	w.wg.Wait()
}

func (w *BackendWorker) run() {
	defer w.wg.Done()

	slog.Info("backend queue worker started")
	_ = bullmq.EnsureQueueMeta(w.ctx, w.redis.MQClient, bullmq.BackendQueue)

	for {
		select {
		case <-w.ctx.Done():
			slog.Info("backend queue worker stopping, draining active slots...")
			for i := 0; i < backendConcurrency; i++ {
				w.sem <- struct{}{}
			}
			slog.Info("backend queue worker stopped")
			return
		default:
		}

		job, err := bullmq.PopJob(w.ctx, w.redis.MQClient, bullmq.BackendQueue)
		if err != nil {
			slog.Error("error popping backend job", "err", err)
			time.Sleep(backendPollInterval)
			continue
		}
		if job == nil {
			time.Sleep(backendPollInterval)
			continue
		}

		w.sem <- struct{}{}
		w.wg.Add(1)
		go func(j *bullmq.RawJob) {
			defer func() { <-w.sem }()
			defer w.wg.Done()
			w.processJob(j)
		}(job)
	}
}

func (w *BackendWorker) processJob(job *bullmq.RawJob) {
	slog.Info("processing backend job", "id", job.ID, "name", job.Name)

	var err error
	switch job.Name {
	case bullmq.SavePageJob:
		err = w.handleSavePage(job)
	case bullmq.UpdateLabelsJob:
		err = w.handleUpdateLabels(job)
	case bullmq.UpdateHighlightJob:
		err = w.handleUpdateHighlight(job)
	case bullmq.TriggerRuleJob:
		err = w.handleTriggerRule(job)
	case bullmq.CallWebhookJob:
		err = w.handleCallWebhook(job)
	case bullmq.RefreshFeedJob:
		err = w.handleRefreshFeed(job)
	case bullmq.RefreshAllFeedsJob:
		err = w.handleRefreshAllFeeds(job)
	case bullmq.BulkActionJob:
		err = w.handleBulkAction(job)
	case bullmq.SendEmailJob:
		err = w.handleSendEmail(job)
	case bullmq.PruneTrashJob:
		err = w.handlePruneTrash(job)
	case bullmq.ExpireFoldersJob:
		err = w.handleExpireFolders(job)
	default:
		slog.Warn("unknown job type", "name", job.Name, "id", job.ID)
		_ = bullmq.CompleteJob(w.ctx, w.redis.MQClient, bullmq.BackendQueue, job.ID)
		return
	}

	if err != nil {
		slog.Error("job failed", "id", job.ID, "name", job.Name, "err", err)
		_ = bullmq.FailJob(w.ctx, w.redis.MQClient, bullmq.BackendQueue, job.ID, err.Error(), job.Opts)
		return
	}

	_ = bullmq.CompleteJob(w.ctx, w.redis.MQClient, bullmq.BackendQueue, job.ID)
	slog.Info("job completed", "id", job.ID, "name", job.Name)
}

// unmarshalJobData is a generic helper for unmarshalling job data.
func unmarshalJobData[T any](job *bullmq.RawJob) (*T, error) {
	var data T
	if err := json.Unmarshal(job.Data, &data); err != nil {
		return nil, err
	}
	return &data, nil
}
