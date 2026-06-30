// Package settings implements the single-row business configuration: business
// identity and branding, SMTP delivery settings, invoice numbering, and reminder
// preferences (SRS Module 5).
package settings

import (
	"context"
	"fmt"
	"strings"

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

// getQuery reads the single settings row (id = 1), which the schema seeds so it
// always exists.
const getQuery = `
SELECT business_name, business_address, tax_id, logo_path,
       smtp_host, smtp_port, smtp_username, smtp_password,
       invoice_prefix, invoice_format, default_tax_rate,
       reminder_days_before, reminder_days_after
FROM settings WHERE id = 1`

// Get returns the current settings (always row id = 1).
func (s *Store) Get(ctx context.Context) (Settings, error) {
	var cfg Settings
	row := s.st.DB().QueryRowContext(ctx, getQuery)
	if err := row.Scan(
		&cfg.BusinessName, &cfg.BusinessAddress, &cfg.TaxID, &cfg.LogoPath,
		&cfg.SMTPHost, &cfg.SMTPPort, &cfg.SMTPUsername, &cfg.SMTPPassword,
		&cfg.InvoicePrefix, &cfg.InvoiceFormat, &cfg.DefaultTaxRate,
		&cfg.ReminderDaysBefore, &cfg.ReminderDaysAfter,
	); err != nil {
		return Settings{}, fmt.Errorf("loading settings: %w", err)
	}
	return cfg, nil
}

// saveProfileQuery updates only the business-profile columns, leaving SMTP,
// numbering, and reminder settings (owned by other forms) untouched.
const saveProfileQuery = `
UPDATE settings
SET business_name = ?, business_address = ?, tax_id = ?, logo_path = ?
WHERE id = 1`

// SaveProfile persists the business identity shown on every PDF: name, address,
// tax ID, and logo path (FR-5.1).
func (s *Store) SaveProfile(ctx context.Context, cfg Settings) error {
	if _, err := s.st.DB().ExecContext(ctx, saveProfileQuery,
		cfg.BusinessName, cfg.BusinessAddress, cfg.TaxID, cfg.LogoPath,
	); err != nil {
		return fmt.Errorf("saving business profile: %w", err)
	}
	return nil
}

// saveSMTPQuery updates only the SMTP delivery columns.
const saveSMTPQuery = `
UPDATE settings
SET smtp_host = ?, smtp_port = ?, smtp_username = ?, smtp_password = ?
WHERE id = 1`

// SaveSMTP persists only the SMTP delivery settings (FR-5.2), kept separate so
// credentials are handled on their own code path.
func (s *Store) SaveSMTP(ctx context.Context, cfg Settings) error {
	if _, err := s.st.DB().ExecContext(ctx, saveSMTPQuery,
		cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword,
	); err != nil {
		return fmt.Errorf("saving smtp settings: %w", err)
	}
	return nil
}

// savePrefsQuery updates the invoicing preferences: number prefix/format and the
// default tax rate.
const savePrefsQuery = `
UPDATE settings
SET invoice_prefix = ?, invoice_format = ?, default_tax_rate = ?
WHERE id = 1`

// SavePreferences persists the invoice-numbering format and prefix (FR-5.3) and
// the default tax rate pre-filled on new invoices (FR-5.4).
func (s *Store) SavePreferences(ctx context.Context, cfg Settings) error {
	if _, err := s.st.DB().ExecContext(ctx, savePrefsQuery,
		cfg.InvoicePrefix, cfg.InvoiceFormat, cfg.DefaultTaxRate,
	); err != nil {
		return fmt.Errorf("saving invoicing preferences: %w", err)
	}
	return nil
}

// FormatNumber expands the configured invoice-number format for the given
// sequence number and year (FR-5.3). Supported tokens: {PREFIX}, {YYYY}, {YY},
// and {SEQ} (zero-padded to four digits). An empty format falls back to a sane
// default.
func (cfg Settings) FormatNumber(seq, year int) string {
	format := cfg.InvoiceFormat
	if strings.TrimSpace(format) == "" {
		format = "{PREFIX}-{YYYY}-{SEQ}"
	}
	return strings.NewReplacer(
		"{PREFIX}", cfg.InvoicePrefix,
		"{YYYY}", fmt.Sprintf("%04d", year),
		"{YY}", fmt.Sprintf("%02d", year%100),
		"{SEQ}", fmt.Sprintf("%04d", seq),
	).Replace(format)
}
