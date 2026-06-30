package invoice

import (
	"context"
	"fmt"

	"github.com/ArfaMujahid/invoice-generator/internal/apperr"
	"github.com/ArfaMujahid/invoice-generator/internal/store"
)

// Store persists and retrieves invoices, their line items, and payments. It is
// backed by the shared SQLite connection (SRS Modules 2 & 3).
//
// Most write/read methods are scaffolded and return apperr.ErrNotImplemented;
// Summary is implemented so the dashboard renders real (zero, on an empty
// database) aggregates. All queries use parameterized statements (NFR-4).
type Store struct {
	st *store.Store
}

// NewStore returns an invoice Store backed by the shared data store.
func NewStore(st *store.Store) *Store {
	return &Store{st: st}
}

// summaryQuery computes the four dashboard amounts in a single pass. Per-invoice
// totals are derived from line items and the invoice tax rate; drafts are
// excluded from invoiced/outstanding because they have not been issued yet.
const summaryQuery = `
WITH inv_totals AS (
    SELECT i.id, i.status,
           CAST(ROUND(COALESCE(SUM(li.quantity * li.unit_price), 0) * (1 + i.tax_rate / 100.0)) AS INTEGER) AS total
    FROM invoices i
    LEFT JOIN line_items li ON li.invoice_id = i.id
    GROUP BY i.id
),
paid AS (
    SELECT invoice_id, COALESCE(SUM(amount), 0) AS amt FROM payments GROUP BY invoice_id
)
SELECT
    COALESCE(SUM(CASE WHEN t.status != 'draft' THEN t.total ELSE 0 END), 0)                          AS invoiced,
    COALESCE(SUM(COALESCE(p.amt, 0)), 0)                                                              AS paid,
    COALESCE(SUM(CASE WHEN t.status != 'draft' THEN t.total - COALESCE(p.amt, 0) ELSE 0 END), 0)      AS outstanding,
    COALESCE(SUM(CASE WHEN t.status  = 'overdue' THEN t.total - COALESCE(p.amt, 0) ELSE 0 END), 0)    AS overdue
FROM inv_totals t
LEFT JOIN paid p ON p.invoice_id = t.id;`

// Summary returns the aggregate amounts shown on the dashboard's summary cards
// (FR-3.2). On an empty database every total is zero.
func (s *Store) Summary(ctx context.Context) (Summary, error) {
	var sum Summary
	row := s.st.DB().QueryRowContext(ctx, summaryQuery)
	if err := row.Scan(&sum.TotalInvoiced, &sum.TotalPaid, &sum.TotalOutstanding, &sum.TotalOverdue); err != nil {
		return Summary{}, fmt.Errorf("computing dashboard summary: %w", err)
	}
	return sum, nil
}

// Create inserts a new invoice with its line items in a single transaction and
// assigns the next invoice number from settings.
//
// TODO(arfa): generate number, INSERT invoice + line_items in a tx (FR-2.1, FR-2.7).
func (s *Store) Create(ctx context.Context, inv Invoice) (Invoice, error) {
	return Invoice{}, apperr.ErrNotImplemented
}

// Update replaces an invoice's header fields and line items (FR-2.7).
//
// TODO(arfa): UPDATE header + replace line_items within a tx.
func (s *Store) Update(ctx context.Context, inv Invoice) (Invoice, error) {
	return Invoice{}, apperr.ErrNotImplemented
}

// Get loads a single invoice with its line items and payments, or
// apperr.ErrNotFound.
//
// TODO(arfa): SELECT invoice + line_items + payments; map no-rows to ErrNotFound.
func (s *Store) Get(ctx context.Context, id int64) (Invoice, error) {
	return Invoice{}, apperr.ErrNotImplemented
}

// Filter describes the optional constraints for listing invoices (FR-3.3).
type Filter struct {
	Status   Status
	ClientID int64
	Search   string
	// From and To bound issue_date (inclusive) when non-empty.
	From string
	To   string
}

// List returns invoices matching the filter, most recent first (FR-3.3).
//
// TODO(arfa): build a parameterized WHERE from the non-zero Filter fields.
func (s *Store) List(ctx context.Context, f Filter) ([]Invoice, error) {
	return nil, apperr.ErrNotImplemented
}

// SetStatus changes an invoice's status, validating the target value (FR-3.1).
//
// TODO(arfa): UPDATE invoices SET status = ? WHERE id = ?.
func (s *Store) SetStatus(ctx context.Context, id int64, status Status) error {
	if !status.Valid() {
		return fmt.Errorf("invoice: invalid status %q", status)
	}
	return apperr.ErrNotImplemented
}

// AddPayment records a payment against an invoice and returns the new balance
// (FR-3.5).
//
// TODO(arfa): INSERT payment; recompute and return balance.
func (s *Store) AddPayment(ctx context.Context, p Payment) (Money, error) {
	return 0, apperr.ErrNotImplemented
}

// MarkOverdue flags every unpaid, past-due, non-draft invoice as overdue and
// returns the count affected. Intended to be called by the daily scheduler
// (FR-3.4).
//
// TODO(arfa): UPDATE invoices SET status='overdue' WHERE due_date < ? AND ...
func (s *Store) MarkOverdue(ctx context.Context, asOf string) (int64, error) {
	return 0, apperr.ErrNotImplemented
}
