package client

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ArfaMujahid/invoice-generator/internal/apperr"
	"github.com/ArfaMujahid/invoice-generator/internal/invoice"
	"github.com/ArfaMujahid/invoice-generator/internal/store"
)

// Store persists and retrieves clients, backed by the shared SQLite connection
// (SRS Module 1). All queries are parameterized (NFR-4).
type Store struct {
	st *store.Store
}

// NewStore returns a client Store backed by the shared data store.
func NewStore(st *store.Store) *Store {
	return &Store{st: st}
}

// ListItem is a client plus the aggregate amounts shown in the list view: total
// invoiced and current outstanding balance (FR-1.3).
type ListItem struct {
	Client
	TotalInvoiced invoice.Money
	Outstanding   invoice.Money
}

const insertQuery = `
INSERT INTO clients (name, email, phone, company, billing_address)
VALUES (?, ?, ?, ?, ?)
RETURNING id`

// Create validates and inserts a new client, returning it with its ID set
// (FR-1.1, FR-1.6).
func (s *Store) Create(ctx context.Context, c Client) (Client, error) {
	if err := c.Validate(); err != nil {
		return Client{}, err
	}
	if err := s.st.DB().QueryRowContext(ctx, insertQuery,
		c.Name, c.Email, c.Phone, c.Company, c.BillingAddress,
	).Scan(&c.ID); err != nil {
		return Client{}, fmt.Errorf("creating client: %w", err)
	}
	return c, nil
}

const updateQuery = `
UPDATE clients
SET name = ?, email = ?, phone = ?, company = ?, billing_address = ?, updated_at = datetime('now')
WHERE id = ?`

// Update validates and saves changes to an existing client (FR-1.2, FR-1.6). It
// returns apperr.ErrNotFound if no client has that ID.
func (s *Store) Update(ctx context.Context, c Client) (Client, error) {
	if err := c.Validate(); err != nil {
		return Client{}, err
	}
	res, err := s.st.DB().ExecContext(ctx, updateQuery,
		c.Name, c.Email, c.Phone, c.Company, c.BillingAddress, c.ID,
	)
	if err != nil {
		return Client{}, fmt.Errorf("updating client %d: %w", c.ID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Client{}, apperr.ErrNotFound
	}
	return c, nil
}

const getQuery = `
SELECT id, name, email, phone, company, billing_address, archived
FROM clients WHERE id = ?`

// Get returns the client with the given id, or apperr.ErrNotFound.
func (s *Store) Get(ctx context.Context, id int64) (Client, error) {
	var c Client
	var archived int64
	err := s.st.DB().QueryRowContext(ctx, getQuery, id).Scan(
		&c.ID, &c.Name, &c.Email, &c.Phone, &c.Company, &c.BillingAddress, &archived,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Client{}, apperr.ErrNotFound
	}
	if err != nil {
		return Client{}, fmt.Errorf("loading client %d: %w", id, err)
	}
	c.Archived = archived != 0
	return c, nil
}

// listQuery returns each client with its invoiced total (excluding drafts) and
// outstanding balance (invoiced minus payments). The ? parameter, when 1,
// includes archived clients.
const listQuery = `
WITH inv AS (
    SELECT i.id, i.client_id, i.status,
           CAST(ROUND(COALESCE(SUM(li.quantity * li.unit_price), 0) * (1 + i.tax_rate / 100.0)) AS INTEGER) AS total
    FROM invoices i
    LEFT JOIN line_items li ON li.invoice_id = i.id
    GROUP BY i.id
),
invoiced AS (
    SELECT client_id, COALESCE(SUM(CASE WHEN status != 'draft' THEN total ELSE 0 END), 0) AS amt
    FROM inv GROUP BY client_id
),
paid AS (
    SELECT i2.client_id, COALESCE(SUM(p.amount), 0) AS amt
    FROM payments p JOIN invoices i2 ON i2.id = p.invoice_id
    GROUP BY i2.client_id
)
SELECT c.id, c.name, c.email, c.phone, c.company, c.billing_address, c.archived,
       COALESCE(invoiced.amt, 0)                          AS invoiced_total,
       COALESCE(invoiced.amt, 0) - COALESCE(paid.amt, 0)  AS outstanding
FROM clients c
LEFT JOIN invoiced ON invoiced.client_id = c.id
LEFT JOIN paid     ON paid.client_id = c.id
WHERE (? = 1 OR c.archived = 0)
ORDER BY c.name COLLATE NOCASE`

// List returns clients with their invoiced and outstanding totals (FR-1.3),
// optionally including archived ones (FR-1.5).
func (s *Store) List(ctx context.Context, includeArchived bool) ([]ListItem, error) {
	inc := 0
	if includeArchived {
		inc = 1
	}
	rows, err := s.st.DB().QueryContext(ctx, listQuery, inc)
	if err != nil {
		return nil, fmt.Errorf("listing clients: %w", err)
	}
	defer rows.Close()

	var items []ListItem
	for rows.Next() {
		var it ListItem
		var archived int64
		if err := rows.Scan(
			&it.ID, &it.Name, &it.Email, &it.Phone, &it.Company, &it.BillingAddress, &archived,
			&it.TotalInvoiced, &it.Outstanding,
		); err != nil {
			return nil, fmt.Errorf("scanning client row: %w", err)
		}
		it.Archived = archived != 0
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating clients: %w", err)
	}
	return items, nil
}

const archiveQuery = `UPDATE clients SET archived = 1, updated_at = datetime('now') WHERE id = ?`

// Archive soft-deletes a client, hiding it from the active list while preserving
// its invoices (FR-1.5). It returns apperr.ErrNotFound if no client has that ID.
func (s *Store) Archive(ctx context.Context, id int64) error {
	res, err := s.st.DB().ExecContext(ctx, archiveQuery, id)
	if err != nil {
		return fmt.Errorf("archiving client %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.ErrNotFound
	}
	return nil
}
