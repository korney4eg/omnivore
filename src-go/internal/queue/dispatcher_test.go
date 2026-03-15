package queue

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/omnivore-app/omnivore/internal/bullmq"
	"github.com/redis/go-redis/v9"
)

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	return client
}

func TestEnqueueFetchContent(t *testing.T) {
	client := testRedis(t)
	ctx := context.Background()
	d := NewDispatcher(client)

	err := d.EnqueueFetchContent(ctx, FetchContentJobData{
		URL:           "https://example.com/article",
		UserID:        "user-1",
		SaveRequestID: "req-1",
		Source:        "api",
		Priority:      "high",
	})
	if err != nil {
		t.Fatalf("EnqueueFetchContent: %v", err)
	}

	// Verify job is in content-fetch queue
	job, err := bullmq.PopJob(ctx, client, bullmq.ContentFetchQueue)
	if err != nil {
		t.Fatalf("PopJob: %v", err)
	}
	if job == nil {
		t.Fatal("expected job in content-fetch queue")
	}
	if job.Name != bullmq.FetchContentJob {
		t.Errorf("job name: got %q, want %q", job.Name, bullmq.FetchContentJob)
	}

	var data FetchContentJobData
	if err := json.Unmarshal(job.Data, &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if data.URL != "https://example.com/article" {
		t.Errorf("URL: got %q, want %q", data.URL, "https://example.com/article")
	}
	if data.UserID != "user-1" {
		t.Errorf("UserID: got %q, want %q", data.UserID, "user-1")
	}
}

func TestEnqueueUpdateLabels(t *testing.T) {
	client := testRedis(t)
	ctx := context.Background()
	d := NewDispatcher(client)

	if err := d.EnqueueUpdateLabels(ctx, "item-1", "user-1"); err != nil {
		t.Fatalf("EnqueueUpdateLabels: %v", err)
	}

	job, err := bullmq.PopJob(ctx, client, bullmq.BackendQueue)
	if err != nil {
		t.Fatalf("PopJob: %v", err)
	}
	if job == nil {
		t.Fatal("expected job")
	}
	if job.Name != bullmq.UpdateLabelsJob {
		t.Errorf("job name: got %q, want %q", job.Name, bullmq.UpdateLabelsJob)
	}
	if job.Opts.Attempts != 6 {
		t.Errorf("attempts: got %d, want 6", job.Opts.Attempts)
	}
}

func TestEnqueueCallWebhook(t *testing.T) {
	client := testRedis(t)
	ctx := context.Background()
	d := NewDispatcher(client)

	if err := d.EnqueueCallWebhook(ctx, "user-1", "page", "created", map[string]any{"id": "item-1"}); err != nil {
		t.Fatalf("EnqueueCallWebhook: %v", err)
	}

	job, err := bullmq.PopJob(ctx, client, bullmq.BackendQueue)
	if err != nil {
		t.Fatalf("PopJob: %v", err)
	}
	if job.Name != bullmq.CallWebhookJob {
		t.Errorf("job name: got %q, want %q", job.Name, bullmq.CallWebhookJob)
	}

	var data CallWebhookJobData
	if err := json.Unmarshal(job.Data, &data); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if data.Type != "page" || data.Action != "created" {
		t.Errorf("webhook: got type=%q action=%q, want page/created", data.Type, data.Action)
	}
}

func TestEnqueueBulkAction(t *testing.T) {
	client := testRedis(t)
	ctx := context.Background()
	d := NewDispatcher(client)

	if err := d.EnqueueBulkAction(ctx, BulkActionJobData{
		Count:     50,
		UserID:    "user-1",
		Action:    "Archive",
		Query:     "in:inbox",
		BatchSize: 25,
	}); err != nil {
		t.Fatalf("EnqueueBulkAction: %v", err)
	}

	job, _ := bullmq.PopJob(ctx, client, bullmq.BackendQueue)
	if job.Name != bullmq.BulkActionJob {
		t.Errorf("name: got %q, want %q", job.Name, bullmq.BulkActionJob)
	}

	var data BulkActionJobData
	json.Unmarshal(job.Data, &data)
	if data.Count != 50 || data.Action != "Archive" {
		t.Errorf("bulk action: got count=%d action=%q", data.Count, data.Action)
	}
}

func TestEnqueueRefreshFeedPriority(t *testing.T) {
	client := testRedis(t)
	ctx := context.Background()
	d := NewDispatcher(client)

	// Low priority feed
	if err := d.EnqueueRefreshFeed(ctx, RefreshFeedJobData{
		FeedURL:  "https://example.com/feed",
		Priority: "low",
	}); err != nil {
		t.Fatalf("EnqueueRefreshFeed: %v", err)
	}

	// High priority feed
	if err := d.EnqueueRefreshFeed(ctx, RefreshFeedJobData{
		FeedURL:  "https://example.com/feed2",
		Priority: "high",
	}); err != nil {
		t.Fatalf("EnqueueRefreshFeed: %v", err)
	}

	// High priority should come first
	job1, _ := bullmq.PopJob(ctx, client, bullmq.BackendQueue)
	job2, _ := bullmq.PopJob(ctx, client, bullmq.BackendQueue)

	if job1 == nil || job2 == nil {
		t.Fatal("expected 2 jobs")
	}

	var data1, data2 RefreshFeedJobData
	json.Unmarshal(job1.Data, &data1)
	json.Unmarshal(job2.Data, &data2)

	if data1.FeedURL != "https://example.com/feed2" {
		t.Errorf("expected high-priority feed first, got %q", data1.FeedURL)
	}
	if data2.FeedURL != "https://example.com/feed" {
		t.Errorf("expected low-priority feed second, got %q", data2.FeedURL)
	}
}
