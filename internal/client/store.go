package client

import (
	"context"

	"github.com/ArfaMujahid/invoice-generator/internal/apperr"
	"github.com/ArfaMujahid/invoice-generator/internal/store"
)

// Store persists and retrieves clients. It is backed by the shared SQLite
// connection and exposes the operations behind Module 1 (SRS §3.1).
//
// The method bodies below are scaffolded: signatures, docs, and contracts are
// fixed, while the SQL is still to be written. Each unimplemented method returns
// apperr.ErrNotImplemented so the running server reports its status honestly.
// All future queries MUST use parameterized statements (NFR-4).
type Store struct {
	st *store.Store
}

// NewStore returns a client Store backed by the shared data store.
func NewStore(st *store.Store) *Store {
	return &Store{st: st}
}

// Create inserts a new client after validation and returns it with its ID set.
//
// TODO(arfa): validate, then INSERT ... RETURNING id (FR-1.1, FR-1.6).
func (s *Store) Create(ctx context.Context, c Client) (Client, error) {
	if err := c.Validate(); err != nil {
		return Client{}, err
	}
	return Client{}, apperr.ErrNotImplemented
}

// Update saves changes to an existing client after validation.
//
// TODO(arfa): validate, then UPDATE clients SET ... WHERE id = ? (FR-1.2).
func (s *Store) Update(ctx context.Context, c Client) (Client, error) {
	if err := c.Validate(); err != nil {
		return Client{}, err
	}
	return Client{}, apperr.ErrNotImplemented
}

// Get returns the client with the given id, or apperr.ErrNotFound.
//
// TODO(arfa): SELECT ... WHERE id = ?; map sql.ErrNoRows to apperr.ErrNotFound.
func (s *Store) Get(ctx context.Context, id int64) (Client, error) {
	return Client{}, apperr.ErrNotImplemented
}

// List returns clients, optionally including archived ones, each annotated with
// its invoiced and outstanding totals for the list view (FR-1.3).
//
// TODO(arfa): SELECT with LEFT JOIN aggregates over invoices/payments.
func (s *Store) List(ctx context.Context, includeArchived bool) ([]Client, error) {
	return nil, apperr.ErrNotImplemented
}

// Archive soft-deletes a client, hiding it from the active list while preserving
// its invoices (FR-1.5).
//
// TODO(arfa): UPDATE clients SET archived = 1 WHERE id = ?.
func (s *Store) Archive(ctx context.Context, id int64) error {
	return apperr.ErrNotImplemented
}
