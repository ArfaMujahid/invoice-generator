// Package settings implements the single-row business configuration: business
// identity and branding, SMTP delivery settings, invoice numbering, and reminder
// preferences (SRS Module 5).
package settings

import (
	"context"

	"github.com/ArfaMujahid/invoice-generator/internal/apperr"
	"github.com/ArfaMujahid/invoice-generator/internal/store"
)

// Settings is the single application settings record (SRS §4.5). SMTP
// credentials live here and must never be rendered into client-facing pages
// (NFR-4).
type Settings struct {
	BusinessName    string
	BusinessAddress string
	TaxID           string
	LogoPath        string

	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string

	InvoicePrefix  string
	InvoiceFormat  string
	DefaultTaxRate float64

	ReminderDaysBefore int
	ReminderDaysAfter  int
}

// Store loads and saves the single settings row backed by the shared SQLite
// connection.
//
// Methods are scaffolded and return apperr.ErrNotImplemented until the queries
// are written; the seeded row (id = 1) created by the schema guarantees Get will
// always find a record once implemented.
type Store struct {
	st *store.Store
}

// NewStore returns a settings Store backed by the shared data store.
func NewStore(st *store.Store) *Store {
	return &Store{st: st}
}

// Get returns the current settings (always row id = 1).
//
// TODO(arfa): SELECT the single settings row.
func (s *Store) Get(ctx context.Context) (Settings, error) {
	return Settings{}, apperr.ErrNotImplemented
}

// Save persists the business profile, numbering, and reminder settings (FR-5.1,
// FR-5.3, FR-5.4).
//
// TODO(arfa): UPDATE settings SET ... WHERE id = 1.
func (s *Store) Save(ctx context.Context, cfg Settings) error {
	return apperr.ErrNotImplemented
}

// SaveSMTP persists only the SMTP delivery settings (FR-5.2), kept separate so
// credentials are handled on their own code path.
//
// TODO(arfa): UPDATE settings SET smtp_* WHERE id = 1.
func (s *Store) SaveSMTP(ctx context.Context, cfg Settings) error {
	return apperr.ErrNotImplemented
}
