package email

import (
	"strings"
	"testing"

	"github.com/ArfaMujahid/invoice-generator/internal/settings"
)

// TestBuildMessagePlain checks a body-only message is a single text/plain part.
func TestBuildMessagePlain(t *testing.T) {
	raw, err := buildMessage("biz@example.com", Message{
		To:      "client@example.com",
		Subject: "Invoice INV-1",
		Body:    "Please find your invoice.",
	})
	if err != nil {
		t.Fatalf("buildMessage() error: %v", err)
	}
	got := string(raw)
	for _, want := range []string{
		"From: biz@example.com\r\n",
		"To: client@example.com\r\n",
		"Subject: Invoice INV-1\r\n",
		"Content-Type: text/plain; charset=utf-8\r\n",
		"Please find your invoice.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("message missing %q\n---\n%s", want, got)
		}
	}
	if strings.Contains(got, "multipart") {
		t.Errorf("plain message should not be multipart:\n%s", got)
	}
}

// TestBuildMessageWithAttachment checks an attachment produces a multipart/mixed
// message carrying a base64-encoded PDF part.
func TestBuildMessageWithAttachment(t *testing.T) {
	raw, err := buildMessage("biz@example.com", Message{
		To:      "client@example.com",
		Subject: "Invoice INV-2",
		Body:    "See attached.",
		Attachments: []Attachment{{
			Filename:    "INV-2.pdf",
			ContentType: "application/pdf",
			Content:     []byte("%PDF-1.4 fake pdf bytes"),
		}},
	})
	if err != nil {
		t.Fatalf("buildMessage() error: %v", err)
	}
	got := string(raw)
	for _, want := range []string{
		"Content-Type: multipart/mixed; boundary=",
		"Content-Type: application/pdf",
		"Content-Transfer-Encoding: base64",
		`filename="INV-2.pdf"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("message missing %q\n---\n%s", want, got)
		}
	}
}

// TestRequireSMTP reports each missing credential field as a validation error.
func TestRequireSMTP(t *testing.T) {
	err := requireSMTP(settings.Settings{})
	if err == nil {
		t.Fatal("requireSMTP() = nil; want validation error for empty config")
	}
	if !strings.Contains(err.Error(), "SMTP host is required") {
		t.Errorf("error = %v; want it to mention the missing host", err)
	}
}
