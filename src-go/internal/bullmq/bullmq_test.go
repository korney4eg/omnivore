package bullmq

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func testRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	return client, mr
}

func TestAddBulkAndPopJob(t *testing.T) {
	client, _ := testRedis(t)
	ctx := context.Background()

	_ = EnsureQueueMeta(ctx, client, "test-queue")

	// Add a job
	err := AddBulk(ctx, client, "test-queue", []AddJobOpts{
		{
			Name: "test-job",
			Data: map[string]string{"url": "https://example.com"},
			Opts: JobOpts{Attempts: 3, Backoff: BackoffOpt{Type: "exponential", Delay: 2000}},
		},
	})
	if err != nil {
		t.Fatalf("AddBulk error: %v", err)
	}

	// Pop the job
	job, err := PopJob(ctx, client, "test-queue")
	if err != nil {
		t.Fatalf("PopJob error: %v", err)
	}
	if job == nil {
		t.Fatal("expected a job, got nil")
	}
	if job.Name != "test-job" {
		t.Errorf("job name: got %q, want %q", job.Name, "test-job")
	}

	// Verify data
	var data map[string]string
	if err := json.Unmarshal(job.Data, &data); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if data["url"] != "https://example.com" {
		t.Errorf("data url: got %q, want %q", data["url"], "https://example.com")
	}

	// Second pop should return nil (no more jobs)
	job2, err := PopJob(ctx, client, "test-queue")
	if err != nil {
		t.Fatalf("PopJob2 error: %v", err)
	}
	if job2 != nil {
		t.Error("expected nil job on empty queue")
	}
}

func TestPriorityOrdering(t *testing.T) {
	client, _ := testRedis(t)
	ctx := context.Background()

	_ = EnsureQueueMeta(ctx, client, "test-queue")

	// Add low-priority job first, then high-priority
	err := AddBulk(ctx, client, "test-queue", []AddJobOpts{
		{Name: "low-prio", Data: "low", Opts: JobOpts{Priority: 100}},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Small delay to ensure different timestamps
	time.Sleep(time.Millisecond)

	err = AddBulk(ctx, client, "test-queue", []AddJobOpts{
		{Name: "high-prio", Data: "high", Opts: JobOpts{Priority: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}

	// High-priority job should come first
	job1, err := PopJob(ctx, client, "test-queue")
	if err != nil {
		t.Fatal(err)
	}
	if job1.Name != "high-prio" {
		t.Errorf("expected high-prio first, got %q", job1.Name)
	}

	job2, err := PopJob(ctx, client, "test-queue")
	if err != nil {
		t.Fatal(err)
	}
	if job2.Name != "low-prio" {
		t.Errorf("expected low-prio second, got %q", job2.Name)
	}
}

func TestCompleteJob(t *testing.T) {
	client, _ := testRedis(t)
	ctx := context.Background()

	_ = EnsureQueueMeta(ctx, client, "test-queue")

	err := AddBulk(ctx, client, "test-queue", []AddJobOpts{
		{Name: "job1", Data: "data"},
	})
	if err != nil {
		t.Fatal(err)
	}

	job, _ := PopJob(ctx, client, "test-queue")
	if job == nil {
		t.Fatal("expected job")
	}

	if err := CompleteJob(ctx, client, "test-queue", job.ID); err != nil {
		t.Fatalf("CompleteJob error: %v", err)
	}

	// Verify job is in completed set
	score, err := client.ZScore(ctx, completedKey("test-queue"), job.ID).Result()
	if err != nil {
		t.Fatalf("ZScore error: %v", err)
	}
	if score == 0 {
		t.Error("expected non-zero score in completed set")
	}
}

func TestFailJobRetry(t *testing.T) {
	client, _ := testRedis(t)
	ctx := context.Background()

	_ = EnsureQueueMeta(ctx, client, "test-queue")

	opts := JobOpts{Attempts: 3, Backoff: BackoffOpt{Type: "exponential", Delay: 1000}}
	err := AddBulk(ctx, client, "test-queue", []AddJobOpts{
		{Name: "retry-job", Data: "data", Opts: opts},
	})
	if err != nil {
		t.Fatal(err)
	}

	job, _ := PopJob(ctx, client, "test-queue")
	if job == nil {
		t.Fatal("expected job")
	}

	// First failure should go to delayed (retry)
	if err := FailJob(ctx, client, "test-queue", job.ID, "test error", opts); err != nil {
		t.Fatalf("FailJob error: %v", err)
	}

	// Check it's in delayed, not failed
	delayedCount, _ := client.ZCard(ctx, queueKey("test-queue")+":delayed").Result()
	failedCount, _ := client.ZCard(ctx, failedKey("test-queue")).Result()
	if delayedCount != 1 {
		t.Errorf("expected 1 delayed job, got %d", delayedCount)
	}
	if failedCount != 0 {
		t.Errorf("expected 0 failed jobs, got %d", failedCount)
	}
}

func TestFailJobExhausted(t *testing.T) {
	client, _ := testRedis(t)
	ctx := context.Background()

	_ = EnsureQueueMeta(ctx, client, "test-queue")

	opts := JobOpts{Attempts: 1, Backoff: BackoffOpt{Type: "exponential", Delay: 1000}}
	err := AddBulk(ctx, client, "test-queue", []AddJobOpts{
		{Name: "fail-job", Data: "data", Opts: opts},
	})
	if err != nil {
		t.Fatal(err)
	}

	job, _ := PopJob(ctx, client, "test-queue")

	// With attempts=1, first failure should move to failed
	if err := FailJob(ctx, client, "test-queue", job.ID, "permanent error", opts); err != nil {
		t.Fatalf("FailJob error: %v", err)
	}

	failedCount, _ := client.ZCard(ctx, failedKey("test-queue")).Result()
	if failedCount != 1 {
		t.Errorf("expected 1 failed job, got %d", failedCount)
	}
}

func TestGetQueueCounts(t *testing.T) {
	client, _ := testRedis(t)
	ctx := context.Background()

	_ = EnsureQueueMeta(ctx, client, "test-queue")

	// Add 3 jobs (2 normal, 1 prioritized)
	err := AddBulk(ctx, client, "test-queue", []AddJobOpts{
		{Name: "job1", Data: "d1"},
		{Name: "job2", Data: "d2"},
		{Name: "job3", Data: "d3", Opts: JobOpts{Priority: 5}},
	})
	if err != nil {
		t.Fatal(err)
	}

	counts, err := GetQueueCounts(ctx, client, "test-queue")
	if err != nil {
		t.Fatal(err)
	}

	// 2 wait + 1 prioritized = 3 total in "prioritized" count
	if counts["prioritized"] != 3 {
		t.Errorf("expected 3 prioritized+wait, got %d", counts["prioritized"])
	}
	if counts["active"] != 0 {
		t.Errorf("expected 0 active, got %d", counts["active"])
	}
}

func TestExponentialDelay(t *testing.T) {
	tests := []struct {
		base, attempt, want int
	}{
		{2000, 0, 2000},
		{2000, 1, 4000},
		{2000, 2, 8000},
		{1000, 3, 8000},
	}
	for _, tc := range tests {
		got := exponentialDelay(tc.base, tc.attempt)
		if got != tc.want {
			t.Errorf("exponentialDelay(%d, %d) = %d, want %d", tc.base, tc.attempt, got, tc.want)
		}
	}
}
