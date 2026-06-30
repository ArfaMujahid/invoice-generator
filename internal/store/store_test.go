package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

// openTestStore opens a Store backed by a throwaway database file under the
// test's temp dir, with the schema applied. t.Cleanup closes it.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// countClients returns the number of rows in the clients table.
func countClients(t *testing.T, st *Store) int {
	t.Helper()
	var n int
	if err := st.DB().QueryRow("SELECT count(*) FROM clients").Scan(&n); err != nil {
		t.Fatalf("counting clients: %v", err)
	}
	return n
}

// TestWithTx verifies that WithTx commits on success and rolls back on error,
// so a failed multi-statement write leaves no partial data behind.
func TestWithTx(t *testing.T) {
	ctx := context.Background()

	t.Run("commit on success", func(t *testing.T) {
		st := openTestStore(t)
		err := st.WithTx(ctx, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx,
				"INSERT INTO clients (name, email) VALUES (?, ?)", "Acme", "a@acme.test")
			return err
		})
		if err != nil {
			t.Fatalf("WithTx() = %v; want nil", err)
		}
		if got := countClients(t, st); got != 1 {
			t.Errorf("clients count = %d; want 1 (row should be committed)", got)
		}
	})

	t.Run("rollback on error", func(t *testing.T) {
		st := openTestStore(t)
		sentinel := errors.New("boom")
		err := st.WithTx(ctx, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx,
				"INSERT INTO clients (name, email) VALUES (?, ?)", "Acme", "a@acme.test"); err != nil {
				return err
			}
			return sentinel // force a rollback after a successful insert
		})
		if !errors.Is(err, sentinel) {
			t.Fatalf("WithTx() = %v; want sentinel error", err)
		}
		if got := countClients(t, st); got != 0 {
			t.Errorf("clients count = %d; want 0 (insert should be rolled back)", got)
		}
	})
}
