package invoice

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ArfaMujahid/invoice-generator/internal/apperr"
	"github.com/ArfaMujahid/invoice-generator/internal/settings"
	"github.com/ArfaMujahid/invoice-generator/internal/store"
)

// dateLayout is the on-disk format for issue/due dates (SQLite TEXT).
const dateLayout = "2006-01-02"

// datetimeLayout matches SQLite's datetime('now') output, used for sent_at.
const datetimeLayout = "2006-01-02 15:04:05"

// Store persists and retrieves invoices, their line items, and payments, backed
// by the shared SQLite connection (SRS Modules 2 & 3). All queries are
// parameterized (NFR-4); multi-row writes run in a transaction (store.WithTx).
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

// Summary returns the aggregate amounts shown on the dashboard (FR-3.2).
func (s *Store) Summary(ctx context.Context) (Summary, error) {
	var sum Summary
	row := s.st.DB().QueryRowContext(ctx, summaryQuery)
	if err := row.Scan(&sum.TotalInvoiced, &sum.TotalPaid, &sum.TotalOutstanding, &sum.TotalOverdue); err != nil {
		return Summary{}, fmt.Errorf("computing dashboard summary: %w", err)
	}
	return sum, nil
}

const insertInvoiceQuery = `
INSERT INTO invoices (invoice_number, client_id, issue_date, due_date, status, currency, tax_rate, notes)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

const insertLineItemQuery = `
INSERT INTO line_items (invoice_id, description, quantity, unit_price, position)
VALUES (?, ?, ?, ?, ?)`

// Create generates the next invoice number from cfg, then inserts the invoice
// and its line items atomically (FR-2.1, FR-2.2, FR-2.7). It returns the invoice
// with its ID and number set.
func (s *Store) Create(ctx context.Context, inv Invoice, cfg settings.Settings) (Invoice, error) {
	if inv.Status == "" {
		inv.Status = StatusDraft
	}
	if err := inv.Validate(); err != nil {
		return Invoice{}, err
	}

	err := s.st.WithTx(ctx, func(tx *sql.Tx) error {
		number, err := nextNumber(ctx, tx, cfg, inv.IssueDate.Year())
		if err != nil {
			return err
		}
		inv.Number = number

		res, err := tx.ExecContext(ctx, insertInvoiceQuery,
			inv.Number, inv.ClientID, inv.IssueDate.Format(dateLayout), inv.DueDate.Format(dateLayout),
			string(inv.Status), inv.Currency, inv.TaxRate, inv.Notes,
		)
		if err != nil {
			return fmt.Errorf("inserting invoice: %w", err)
		}
		inv.ID, _ = res.LastInsertId()
		return insertLineItems(ctx, tx, inv.ID, inv.LineItems)
	})
	if err != nil {
		return Invoice{}, err
	}
	return inv, nil
}

const updateInvoiceQuery = `
UPDATE invoices
SET client_id = ?, issue_date = ?, due_date = ?, currency = ?, tax_rate = ?, notes = ?, updated_at = datetime('now')
WHERE id = ?`

// Update saves header changes and replaces the line items of an existing invoice
// atomically (FR-2.7). The invoice number and status are not changed here.
func (s *Store) Update(ctx context.Context, inv Invoice) (Invoice, error) {
	if err := inv.Validate(); err != nil {
		return Invoice{}, err
	}
	err := s.st.WithTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, updateInvoiceQuery,
			inv.ClientID, inv.IssueDate.Format(dateLayout), inv.DueDate.Format(dateLayout),
			inv.Currency, inv.TaxRate, inv.Notes, inv.ID,
		)
		if err != nil {
			return fmt.Errorf("updating invoice %d: %w", inv.ID, err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return apperr.ErrNotFound
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM line_items WHERE invoice_id = ?`, inv.ID); err != nil {
			return fmt.Errorf("clearing line items: %w", err)
		}
		return insertLineItems(ctx, tx, inv.ID, inv.LineItems)
	})
	if err != nil {
		return Invoice{}, err
	}
	return inv, nil
}

// insertLineItems writes each line item with its display position.
func insertLineItems(ctx context.Context, tx *sql.Tx, invoiceID int64, items []LineItem) error {
	for i, li := range items {
		if _, err := tx.ExecContext(ctx, insertLineItemQuery,
			invoiceID, li.Description, li.Quantity, int64(li.UnitPrice), i,
		); err != nil {
			return fmt.Errorf("inserting line item %d: %w", i, err)
		}
	}
	return nil
}

// nextNumber returns the next unique invoice number for the given year using the
// configured format, advancing the sequence until an unused number is found.
func nextNumber(ctx context.Context, tx *sql.Tx, cfg settings.Settings, year int) (string, error) {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM invoices`).Scan(&count); err != nil {
		return "", fmt.Errorf("counting invoices: %w", err)
	}
	for seq := count + 1; ; seq++ {
		number := cfg.FormatNumber(seq, year)
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM invoices WHERE invoice_number = ?`, number).Scan(&exists); err != nil {
			return "", fmt.Errorf("checking invoice number: %w", err)
		}
		if exists == 0 {
			return number, nil
		}
	}
}

const getInvoiceQuery = `
SELECT id, invoice_number, client_id, issue_date, due_date, status, currency, tax_rate, notes, sent_at, reminders_sent
FROM invoices WHERE id = ?`

// Get loads a single invoice with its line items and payments, or
// apperr.ErrNotFound.
func (s *Store) Get(ctx context.Context, id int64) (Invoice, error) {
	var (
		inv              Invoice
		issueStr, dueStr string
		statusStr        string
		sentAt           sql.NullString
	)
	err := s.st.DB().QueryRowContext(ctx, getInvoiceQuery, id).Scan(
		&inv.ID, &inv.Number, &inv.ClientID, &issueStr, &dueStr, &statusStr,
		&inv.Currency, &inv.TaxRate, &inv.Notes, &sentAt, &inv.RemindersSent,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Invoice{}, apperr.ErrNotFound
	}
	if err != nil {
		return Invoice{}, fmt.Errorf("loading invoice %d: %w", id, err)
	}
	inv.Status = Status(statusStr)
	if inv.IssueDate, err = time.Parse(dateLayout, issueStr); err != nil {
		return Invoice{}, fmt.Errorf("parsing issue date %q: %w", issueStr, err)
	}
	if inv.DueDate, err = time.Parse(dateLayout, dueStr); err != nil {
		return Invoice{}, fmt.Errorf("parsing due date %q: %w", dueStr, err)
	}
	if sentAt.Valid {
		if t, perr := time.Parse(datetimeLayout, sentAt.String); perr == nil {
			inv.SentAt = &t
		}
	}

	if inv.LineItems, err = s.lineItems(ctx, id); err != nil {
		return Invoice{}, err
	}
	if inv.Payments, err = s.payments(ctx, id); err != nil {
		return Invoice{}, err
	}
	return inv, nil
}

// lineItems loads an invoice's line items in display order.
func (s *Store) lineItems(ctx context.Context, invoiceID int64) ([]LineItem, error) {
	rows, err := s.st.DB().QueryContext(ctx,
		`SELECT id, invoice_id, description, quantity, unit_price, position FROM line_items WHERE invoice_id = ? ORDER BY position`, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("loading line items: %w", err)
	}
	defer rows.Close()
	var items []LineItem
	for rows.Next() {
		var li LineItem
		if err := rows.Scan(&li.ID, &li.InvoiceID, &li.Description, &li.Quantity, &li.UnitPrice, &li.Position); err != nil {
			return nil, fmt.Errorf("scanning line item: %w", err)
		}
		items = append(items, li)
	}
	return items, rows.Err()
}

// payments loads an invoice's payments in date order.
func (s *Store) payments(ctx context.Context, invoiceID int64) ([]Payment, error) {
	rows, err := s.st.DB().QueryContext(ctx,
		`SELECT id, invoice_id, amount, paid_on FROM payments WHERE invoice_id = ? ORDER BY paid_on`, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("loading payments: %w", err)
	}
	defer rows.Close()
	var items []Payment
	for rows.Next() {
		var (
			p       Payment
			paidStr string
		)
		if err := rows.Scan(&p.ID, &p.InvoiceID, &p.Amount, &paidStr); err != nil {
			return nil, fmt.Errorf("scanning payment: %w", err)
		}
		if p.PaidOn, err = time.Parse(dateLayout, paidStr); err != nil {
			return nil, fmt.Errorf("parsing payment date %q: %w", paidStr, err)
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

// Filter describes the optional constraints for listing invoices (FR-3.3).
type Filter struct {
	Status   Status
	ClientID int64
	Search   string
	// From and To bound issue_date (inclusive) when non-empty (ISO dates).
	From string
	To   string
}

// List returns invoices matching the filter, newest first, each with its client
// name and computed total/paid/balance (FR-3.3; also backs the client invoice
// history, FR-1.4, via the ClientID filter).
func (s *Store) List(ctx context.Context, f Filter) ([]ListItem, error) {
	var (
		where []string
		args  []any
	)
	if f.Status != "" {
		where = append(where, "i.status = ?")
		args = append(args, string(f.Status))
	}
	if f.ClientID > 0 {
		where = append(where, "i.client_id = ?")
		args = append(args, f.ClientID)
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		where = append(where, "(i.invoice_number LIKE ? OR c.name LIKE ?)")
		like := "%" + s + "%"
		args = append(args, like, like)
	}
	if f.From != "" {
		where = append(where, "i.issue_date >= ?")
		args = append(args, f.From)
	}
	if f.To != "" {
		where = append(where, "i.issue_date <= ?")
		args = append(args, f.To)
	}

	query := `
WITH inv AS (
    SELECT i.id, CAST(ROUND(COALESCE(SUM(li.quantity * li.unit_price), 0) * (1 + i.tax_rate / 100.0)) AS INTEGER) AS total
    FROM invoices i LEFT JOIN line_items li ON li.invoice_id = i.id GROUP BY i.id
),
pay AS (
    SELECT invoice_id, COALESCE(SUM(amount), 0) AS paid FROM payments GROUP BY invoice_id
)
SELECT i.id, i.invoice_number, i.client_id, c.name, i.issue_date, i.due_date, i.status, i.currency,
       COALESCE(inv.total, 0), COALESCE(pay.paid, 0), COALESCE(inv.total, 0) - COALESCE(pay.paid, 0)
FROM invoices i
JOIN clients c ON c.id = i.client_id
LEFT JOIN inv ON inv.id = i.id
LEFT JOIN pay ON pay.invoice_id = i.id`
	if len(where) > 0 {
		query += "\nWHERE " + strings.Join(where, " AND ")
	}
	query += "\nORDER BY i.id DESC"

	rows, err := s.st.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing invoices: %w", err)
	}
	defer rows.Close()

	var items []ListItem
	for rows.Next() {
		var (
			it        ListItem
			statusStr string
		)
		if err := rows.Scan(&it.ID, &it.Number, &it.ClientID, &it.ClientName, &it.IssueDate, &it.DueDate,
			&statusStr, &it.Currency, &it.Total, &it.Paid, &it.Balance); err != nil {
			return nil, fmt.Errorf("scanning invoice row: %w", err)
		}
		it.Status = Status(statusStr)
		items = append(items, it)
	}
	return items, rows.Err()
}

// SetStatus changes an invoice's status, validating the target value (FR-3.1).
func (s *Store) SetStatus(ctx context.Context, id int64, status Status) error {
	if !status.Valid() {
		return fmt.Errorf("invoice: invalid status %q", status)
	}
	res, err := s.st.DB().ExecContext(ctx,
		`UPDATE invoices SET status = ?, updated_at = datetime('now') WHERE id = ?`, string(status), id)
	if err != nil {
		return fmt.Errorf("setting invoice %d status: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.ErrNotFound
	}
	return nil
}

// MarkSent stamps the invoice as sent: status Sent and sent_at now (FR-4.2).
func (s *Store) MarkSent(ctx context.Context, id int64) error {
	res, err := s.st.DB().ExecContext(ctx,
		`UPDATE invoices SET status = 'sent', sent_at = datetime('now'), updated_at = datetime('now') WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("marking invoice %d sent: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.ErrNotFound
	}
	return nil
}

// RecordReminder increments the reminder counter for an invoice (FR-4.4).
func (s *Store) RecordReminder(ctx context.Context, id int64) error {
	if _, err := s.st.DB().ExecContext(ctx,
		`UPDATE invoices SET reminders_sent = reminders_sent + 1, updated_at = datetime('now') WHERE id = ?`, id); err != nil {
		return fmt.Errorf("recording reminder for invoice %d: %w", id, err)
	}
	return nil
}

// AddPayment records a (possibly partial) payment and returns the new balance
// (FR-3.5). When the balance reaches zero or below, the invoice is marked Paid.
func (s *Store) AddPayment(ctx context.Context, p Payment) (Money, error) {
	if p.Amount <= 0 {
		return 0, apperr.NewValidationError().Add("amount", "payment must be positive")
	}
	var balance Money
	err := s.st.WithTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO payments (invoice_id, amount, paid_on) VALUES (?, ?, ?)`,
			p.InvoiceID, int64(p.Amount), p.PaidOn.Format(dateLayout)); err != nil {
			return fmt.Errorf("inserting payment: %w", err)
		}

		var total, paid Money
		if err := tx.QueryRowContext(ctx, `
            SELECT CAST(ROUND(COALESCE(SUM(li.quantity * li.unit_price), 0) * (1 + i.tax_rate / 100.0)) AS INTEGER)
            FROM invoices i LEFT JOIN line_items li ON li.invoice_id = i.id
            WHERE i.id = ? GROUP BY i.id`, p.InvoiceID).Scan(&total); err != nil {
			return fmt.Errorf("computing invoice total: %w", err)
		}
		if err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(SUM(amount), 0) FROM payments WHERE invoice_id = ?`, p.InvoiceID).Scan(&paid); err != nil {
			return fmt.Errorf("summing payments: %w", err)
		}
		balance = total - paid
		if balance <= 0 {
			if _, err := tx.ExecContext(ctx,
				`UPDATE invoices SET status = 'paid', updated_at = datetime('now') WHERE id = ?`, p.InvoiceID); err != nil {
				return fmt.Errorf("marking invoice paid: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return balance, nil
}

// MarkOverdue flags every sent, past-due invoice as overdue and returns the
// count affected. Intended for the daily scheduler (FR-3.4). asOf is an ISO date.
func (s *Store) MarkOverdue(ctx context.Context, asOf string) (int64, error) {
	res, err := s.st.DB().ExecContext(ctx,
		`UPDATE invoices SET status = 'overdue', updated_at = datetime('now') WHERE status = 'sent' AND due_date < ?`, asOf)
	if err != nil {
		return 0, fmt.Errorf("marking overdue invoices: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
