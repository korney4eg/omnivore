package queue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/omnivore-app/omnivore/internal/bullmq"
)

// webhookRecord represents a webhook row from the database.
type webhookRecord struct {
	ID          string `gorm:"column:id"`
	URL         string `gorm:"column:url"`
	Method      string `gorm:"column:method"`
	ContentType string `gorm:"column:content_type"`
}

// handleCallWebhook dispatches webhooks for a given event.
func (w *BackendWorker) handleCallWebhook(job *bullmq.RawJob) error {
	data, err := unmarshalJobData[CallWebhookJobData](job)
	if err != nil {
		return fmt.Errorf("unmarshal call-webhook data: %w", err)
	}

	eventType := strings.ToUpper(data.Type + "_" + data.Action)
	slog.Info("call-webhook", "userId", data.UserID, "eventType", eventType)

	// Find webhooks matching this event type
	var webhooks []webhookRecord
	if err := w.db.Read.WithContext(w.ctx).
		Table("omnivore.webhooks").
		Where("user_id = ? AND enabled = true AND event_types @> ARRAY[?]::text[]", data.UserID, eventType).
		Find(&webhooks).Error; err != nil {
		return fmt.Errorf("find webhooks: %w", err)
	}

	if len(webhooks) == 0 {
		slog.Debug("call-webhook: no webhooks found", "userId", data.UserID, "eventType", eventType)
		return nil
	}

	// Dispatch all webhooks (log errors but don't fail the job)
	body := map[string]any{
		"action": data.Action,
		"userId": data.UserID,
		data.Type: data.Data,
	}

	for _, wh := range webhooks {
		if err := postWebhookWithMethod(wh.URL, wh.Method, wh.ContentType, body); err != nil {
			slog.Error("call-webhook: dispatch failed",
				"err", err, "webhookId", wh.ID, "url", wh.URL)
		}
	}

	return nil
}

// postWebhook sends a POST request to a webhook URL with JSON payload.
func postWebhook(url string, body any) error {
	return postWebhookWithMethod(url, "POST", "application/json", body)
}

// postWebhookWithMethod sends an HTTP request to a webhook URL with configurable method.
func postWebhookWithMethod(url, method, contentType string, body any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal webhook body: %w", err)
	}

	if method == "" {
		method = "POST"
	}
	if contentType == "" {
		contentType = "application/json"
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}
