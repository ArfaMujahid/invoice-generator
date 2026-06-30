// Package email sends invoices and payment reminders to clients over SMTP using
// the credentials configured in settings (SRS Module 4). Credentials are read
// from settings at send time and never exposed to the frontend (NFR-4).
package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"mime/multipart"
	"net"
	"net/smtp"
	"net/textproto"
	"strconv"
	"time"

	"github.com/ArfaMujahid/invoice-generator/internal/apperr"
	"github.com/ArfaMujahid/invoice-generator/internal/settings"
)

// dialTimeout bounds how long we wait to establish an SMTP connection so a
// misconfigured host fails fast instead of hanging the request (NFR-3).
const dialTimeout = 15 * time.Second

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

// Send delivers msg using the SMTP settings in cfg (FR-4.1). The configured SMTP
// username is used as the From address. It reports a clear error on failure so
// the handler can surface success/failure to the user (NFR-3).
func (s *SMTPSender) Send(ctx context.Context, cfg settings.Settings, msg Message) error {
	if err := requireSMTP(cfg); err != nil {
		return err
	}
	raw, err := buildMessage(cfg.SMTPUsername, msg)
	if err != nil {
		return err
	}

	c, err := dial(ctx, cfg)
	if err != nil {
		return err
	}
	defer c.Close()

	if err := c.Auth(smtp.PlainAuth("", cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPHost)); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := c.Mail(cfg.SMTPUsername); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := c.Rcpt(msg.To); err != nil {
		return fmt.Errorf("smtp rcpt to %s: %w", msg.To, err)
	}
	wc, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := wc.Write(raw); err != nil {
		_ = wc.Close()
		return fmt.Errorf("writing message body: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("closing message body: %w", err)
	}
	return c.Quit()
}

// TestConnection verifies the SMTP settings by opening an authenticated session
// without sending mail, backing the Settings "Test connection" button (FR-5.2).
func (s *SMTPSender) TestConnection(ctx context.Context, cfg settings.Settings) error {
	if err := requireSMTP(cfg); err != nil {
		return err
	}
	c, err := dial(ctx, cfg)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := c.Auth(smtp.PlainAuth("", cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPHost)); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	return c.Quit()
}

// requireSMTP reports missing SMTP settings as a per-field validation error so
// the UI can point at what to fill in before connecting.
func requireSMTP(cfg settings.Settings) error {
	v := apperr.NewValidationError()
	if cfg.SMTPHost == "" {
		v.Add("smtp_host", "SMTP host is required")
	}
	if cfg.SMTPPort <= 0 {
		v.Add("smtp_port", "SMTP port is required")
	}
	if cfg.SMTPUsername == "" {
		v.Add("smtp_username", "SMTP username is required")
	}
	if cfg.SMTPPassword == "" {
		v.Add("smtp_password", "SMTP password is required")
	}
	if v.HasErrors() {
		return v
	}
	return nil
}

// dial opens an SMTP client connection, using implicit TLS on port 465 and
// STARTTLS (when offered) on any other port. The context bounds the dial.
func dial(ctx context.Context, cfg settings.Settings) (*smtp.Client, error) {
	addr := net.JoinHostPort(cfg.SMTPHost, strconv.Itoa(cfg.SMTPPort))
	nd := &net.Dialer{Timeout: dialTimeout}
	tlsCfg := &tls.Config{ServerName: cfg.SMTPHost}

	if cfg.SMTPPort == 465 {
		td := &tls.Dialer{NetDialer: nd, Config: tlsCfg}
		conn, err := td.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("dialing smtp over tls %s: %w", addr, err)
		}
		c, err := smtp.NewClient(conn, cfg.SMTPHost)
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("smtp client: %w", err)
		}
		return c, nil
	}

	conn, err := nd.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dialing smtp %s: %w", addr, err)
	}
	c, err := smtp.NewClient(conn, cfg.SMTPHost)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("smtp client: %w", err)
	}
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(tlsCfg); err != nil {
			_ = c.Close()
			return nil, fmt.Errorf("starttls: %w", err)
		}
	}
	return c, nil
}

// buildMessage assembles RFC 5322 message bytes from from/msg. When there are
// attachments the body is multipart/mixed; otherwise it is a plain text mail.
func buildMessage(from string, msg Message) ([]byte, error) {
	var out bytes.Buffer
	fmt.Fprintf(&out, "From: %s\r\n", from)
	fmt.Fprintf(&out, "To: %s\r\n", msg.To)
	fmt.Fprintf(&out, "Subject: %s\r\n", msg.Subject)
	out.WriteString("MIME-Version: 1.0\r\n")

	if len(msg.Attachments) == 0 {
		out.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
		out.WriteString(msg.Body)
		return out.Bytes(), nil
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := writeTextPart(mw, msg.Body); err != nil {
		return nil, err
	}
	for _, a := range msg.Attachments {
		if err := writeAttachmentPart(mw, a); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("closing mime writer: %w", err)
	}

	fmt.Fprintf(&out, "Content-Type: multipart/mixed; boundary=%s\r\n\r\n", mw.Boundary())
	out.Write(body.Bytes())
	return out.Bytes(), nil
}

// writeTextPart writes the plain-text body part of a multipart message.
func writeTextPart(mw *multipart.Writer, text string) error {
	h := textproto.MIMEHeader{}
	h.Set("Content-Type", "text/plain; charset=utf-8")
	pw, err := mw.CreatePart(h)
	if err != nil {
		return fmt.Errorf("creating text part: %w", err)
	}
	if _, err := pw.Write([]byte(text)); err != nil {
		return fmt.Errorf("writing text part: %w", err)
	}
	return nil
}

// writeAttachmentPart writes a base64-encoded, line-wrapped attachment part.
func writeAttachmentPart(mw *multipart.Writer, a Attachment) error {
	ct := a.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	h := textproto.MIMEHeader{}
	h.Set("Content-Type", ct)
	h.Set("Content-Transfer-Encoding", "base64")
	h.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", a.Filename))
	pw, err := mw.CreatePart(h)
	if err != nil {
		return fmt.Errorf("creating attachment part: %w", err)
	}
	// Wrap base64 at 76 columns per MIME conventions.
	enc := base64.StdEncoding.EncodeToString(a.Content)
	for i := 0; i < len(enc); i += 76 {
		end := i + 76
		if end > len(enc) {
			end = len(enc)
		}
		if _, err := pw.Write([]byte(enc[i:end] + "\r\n")); err != nil {
			return fmt.Errorf("writing attachment part: %w", err)
		}
	}
	return nil
}
