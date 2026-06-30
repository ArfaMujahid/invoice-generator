// Package client implements client (customer) management: the Client domain
// type, its validation rules, and the persistence repository (SRS Module 1).
package client

import (
	"net/mail"
	"strings"
	"time"

	"github.com/ArfaMujahid/invoice-generator/internal/apperr"
)

// Client is a customer the business issues invoices to (SRS §4.1).
type Client struct {
	ID             int64
	Name           string
	Email          string
	Phone          string
	Company        string
	BillingAddress string
	Archived       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Validate enforces the required-field and email-format rules before a client is
// saved (FR-1.6). It returns an *apperr.ValidationError describing every problem
// at once, so the form can show all messages in a single round-trip.
func (c Client) Validate() error {
	v := apperr.NewValidationError()

	if strings.TrimSpace(c.Name) == "" {
		v.Add("name", "name is required")
	}
	switch {
	case strings.TrimSpace(c.Email) == "":
		v.Add("email", "email is required")
	case !validEmail(c.Email):
		v.Add("email", "email is not a valid address")
	}

	if v.HasErrors() {
		return v
	}
	return nil
}

// validEmail reports whether addr parses as a single RFC 5322 address. Using the
// stdlib parser avoids brittle hand-rolled regexes.
func validEmail(addr string) bool {
	_, err := mail.ParseAddress(strings.TrimSpace(addr))
	return err == nil
}
