package queue

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/omnivore-app/omnivore/internal/bullmq"
)

// handleRefreshFeed fetches and processes an RSS feed for one or more subscriptions.
func (w *BackendWorker) handleRefreshFeed(job *bullmq.RawJob) error {
	data, err := unmarshalJobData[RefreshFeedJobData](job)
	if err != nil {
		return fmt.Errorf("unmarshal refresh-feed data: %w", err)
	}

	slog.Info("refresh-feed", "feedUrl", data.FeedURL, "subscriptions", len(data.SubscriptionIDs))

	// Check if feed URL is blocked due to repeated failures
	failKey := "feed-fetch-failure:" + data.FeedURL
	failCount, _ := w.redis.CacheClient.Get(w.ctx, failKey).Int()
	if failCount > 10 {
		slog.Warn("refresh-feed: feed blocked due to failures", "feedUrl", data.FeedURL, "failures", failCount)
		return nil
	}

	// Fetch feed content
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(data.FeedURL)
	if err != nil {
		// Increment failure counter
		w.redis.CacheClient.Incr(w.ctx, failKey)
		w.redis.CacheClient.Expire(w.ctx, failKey, 24*time.Hour)
		return fmt.Errorf("fetch feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		w.redis.CacheClient.Incr(w.ctx, failKey)
		w.redis.CacheClient.Expire(w.ctx, failKey, 24*time.Hour)
		return fmt.Errorf("feed returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read feed body: %w", err)
	}

	// Compute checksum
	checksum := fmt.Sprintf("%x", sha256.Sum256(body))

	// Process each subscription
	for i, subID := range data.SubscriptionIDs {
		// Skip if checksum matches (no new content)
		if i < len(data.LastFetchedChecksums) && data.LastFetchedChecksums[i] == checksum {
			slog.Debug("refresh-feed: unchanged", "subscriptionId", subID)
			continue
		}

		userID := ""
		if i < len(data.UserIDs) {
			userID = data.UserIDs[i]
		}

		// Update subscription metadata
		now := time.Now()
		w.db.Write.WithContext(w.ctx).
			Table("omnivore.subscriptions").
			Where("id = ?", subID).
			Updates(map[string]any{
				"last_fetched_checksum": checksum,
				"refreshed_at":         now,
				"updated_at":           now,
			})

		slog.Info("refresh-feed: subscription updated",
			"subscriptionId", subID, "userId", userID, "feedUrl", data.FeedURL)
	}

	// Note: Full RSS item parsing and content-fetch enqueueing requires an RSS parser library.
	// The feed body is fetched and checksums are updated. Individual item processing
	// (parse items, compare dates, enqueue fetch-content jobs) would require
	// integrating an RSS parser like gofeed. This is a functional stub that handles
	// the network and subscription bookkeeping.

	return nil
}

// handleRefreshAllFeeds triggers a refresh of all active RSS subscriptions.
func (w *BackendWorker) handleRefreshAllFeeds(job *bullmq.RawJob) error {
	slog.Info("refresh-all-feeds: starting")

	// Query all active subscriptions grouped by feed URL
	type feedGroup struct {
		FeedURL         string   `gorm:"column:url"`
		SubscriptionIDs []string
		UserIDs         []string
		Checksums       []string
	}

	var subscriptions []struct {
		ID                  string  `gorm:"column:id"`
		URL                 string  `gorm:"column:url"`
		UserID              string  `gorm:"column:user_id"`
		LastFetchedChecksum *string `gorm:"column:last_fetched_checksum"`
	}

	if err := w.db.Read.WithContext(w.ctx).
		Table("omnivore.subscriptions").
		Where("status = 'ACTIVE' AND type = 'RSS'").
		Find(&subscriptions).Error; err != nil {
		return fmt.Errorf("list subscriptions: %w", err)
	}

	if len(subscriptions) == 0 {
		slog.Info("refresh-all-feeds: no active subscriptions")
		return nil
	}

	// Group by feed URL
	groups := make(map[string]*RefreshFeedJobData)
	for _, sub := range subscriptions {
		g, ok := groups[sub.URL]
		if !ok {
			g = &RefreshFeedJobData{
				FeedURL:  sub.URL,
				Priority: "low",
			}
			groups[sub.URL] = g
		}
		g.SubscriptionIDs = append(g.SubscriptionIDs, sub.ID)
		g.UserIDs = append(g.UserIDs, sub.UserID)
		checksum := ""
		if sub.LastFetchedChecksum != nil {
			checksum = *sub.LastFetchedChecksum
		}
		g.LastFetchedChecksums = append(g.LastFetchedChecksums, checksum)
	}

	// Enqueue individual refresh-feed jobs
	dispatcher := NewDispatcher(w.redis.MQClient)
	enqueued := 0
	for _, data := range groups {
		if err := dispatcher.EnqueueRefreshFeed(w.ctx, *data); err != nil {
			slog.Error("refresh-all-feeds: enqueue failed", "err", err, "feedUrl", data.FeedURL)
			continue
		}
		enqueued++
	}

	slog.Info("refresh-all-feeds: done", "total", len(groups), "enqueued", enqueued)
	return nil
}
