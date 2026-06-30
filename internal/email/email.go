// Package email sends invoices and payment reminders to clients over SMTP using
// the credentials configured in settings (SRS Module 4). Credentials are read
// from settings at send time and never exposed to the frontend (NFR-4).
package email

import (
	"context"

	"github.com/ArfaMujahid/invoice-generator/internal/apperr"
	"github.com/ArfaMujahid/invoice-generator/internal/settings"
)

// Attachment is a single file attached to an outgoing message (e.g. the invoice
// PDF).
type Attachment struct {
	Filename    string
	ContentType string
	Content     []byte
}

// Message is an outgoing email assembled by the caller.
type Message struct {
	To          string
	Subject     string
	Body        string
	Attachments []Attachment
}

// SMTPSender sends mail through the configured SMTP server. It is concrete; the
// server depends on a small interface it defines for its own use.
type SMTPSender struct{}

// NewSMTPSender returns a ready-to-use SMTP sender.
func NewSMTPSender() *SMTPSender {
	return &SMTPSender{}
}

// Send delivers msg using the SMTP settings in cfg (FR-4.1). It reports a clear
// error on failure so the handler can surface success/failure to the user
// (NFR-3).
//
// TODO(arfa): build the MIME message and send via net/smtp with auth + TLS.
func (s *SMTPSender) Send(ctx context.Context, cfg settings.Settings, msg Message) error {
	return apperr.ErrNotImplemented
}

// TestConnection verifies the SMTP settings by opening an authenticated session
// without sending mail, backing the Settings "Test connection" button (FR-5.2).
//
// TODO(arfa): dial, STARTTLS, AUTH, then QUIT; return any failure verbatim.
func (s *SMTPSender) TestConnection(ctx context.Context, cfg settings.Settings) error {
	return apperr.ErrNotImplemented
}
