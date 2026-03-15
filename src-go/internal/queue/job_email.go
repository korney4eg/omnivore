package queue

import (
	"fmt"
	"log/slog"

	"github.com/omnivore-app/omnivore/internal/bullmq"
)

// handleSendEmail sends a transactional email.
// Currently logs the email request. Actual sending requires SES/SMTP integration.
func (w *BackendWorker) handleSendEmail(job *bullmq.RawJob) error {
	data, err := unmarshalJobData[SendEmailJobData](job)
	if err != nil {
		return fmt.Errorf("unmarshal send-email data: %w", err)
	}

	to := data.To
	if to == "" {
		// Look up user's email
		var email string
		if err := w.db.Read.WithContext(w.ctx).
			Raw("SELECT email FROM omnivore.user WHERE id = ? AND status = 'ACTIVE'", data.UserID).
			Scan(&email).Error; err != nil || email == "" {
			slog.Warn("send-email: user email not found", "userId", data.UserID)
			return nil // Don't retry
		}
		to = email
	}

	slog.Info("send-email",
		"to", to,
		"subject", data.Subject,
		"userId", data.UserID,
		"hasHTML", data.HTML != "",
		"hasText", data.Text != "",
		"templateId", data.TemplateID,
	)

	// TODO: Integrate with SES, SMTP, or MailJet for actual email delivery.
	// For now, log the email details so the queue infrastructure is complete.
	// The email sending implementation depends on the chosen email provider:
	//   - AWS SES: github.com/aws/aws-sdk-go-v2/service/ses
	//   - SMTP: net/smtp
	//   - MailJet: github.com/mailjet/mailjet-apiv3-go
	//
	// The TS API uses SendGrid/MailJet. The Go version should use the same
	// provider configured via environment variables.

	slog.Info("send-email: email would be sent (provider not configured)",
		"to", to, "subject", data.Subject)

	return nil
}
