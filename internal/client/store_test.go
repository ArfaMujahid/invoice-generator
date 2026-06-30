package client

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ArfaMujahid/invoice-generator/internal/apperr"
	"github.com/ArfaMujahid/invoice-generator/internal/store"
)

// newTestStore opens a client Store backed by a throwaway database.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatalf("store.Open() error: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return NewStore(st)
}

// TestClientCRUD exercises create, get, update, list, and archive.
func TestClientCRUD(t *testing.T) {
	ctx := context.Background()
	repo := newTestStore(t)

	created, err := repo.Create(ctx, Client{Name: "Acme", Email: "a@acme.test", Company: "Acme Inc"})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("Create() returned zero ID")
	}

	got, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got.Name != "Acme" || got.Email != "a@acme.test" || got.Company != "Acme Inc" {
		t.Errorf("Get() = %+v; unexpected", got)
	}

	got.Name = "Acme Renamed"
	if _, err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update() error: %v", err)
	}

	list, err := repo.List(ctx, false)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(list) != 1 || list[0].Name != "Acme Renamed" {
		t.Errorf("List() = %+v; want one renamed client", list)
	}
	if list[0].TotalInvoiced != 0 || list[0].Outstanding != 0 {
		t.Errorf("totals = %d/%d; want 0/0 with no invoices", list[0].TotalInvoiced, list[0].Outstanding)
	}

	if err := repo.Archive(ctx, created.ID); err != nil {
		t.Fatalf("Archive() error: %v", err)
	}
	active, err := repo.List(ctx, false)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("active list = %d; want 0 after archive", len(active))
	}
	if all, _ := repo.List(ctx, true); len(all) != 1 {
		t.Errorf("includeArchived list = %d; want 1", len(all))
	}
}

// TestCreateInvalid rejects a malformed client with a validation error.
func TestCreateInvalid(t *testing.T) {
	repo := newTestStore(t)
	_, err := repo.Create(context.Background(), Client{Name: "", Email: "bad"})
	var verr *apperr.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("Create() error = %v; want ValidationError", err)
	}
}

// TestGetNotFound maps a missing client to apperr.ErrNotFound.
func TestGetNotFound(t *testing.T) {
	repo := newTestStore(t)
	if _, err := repo.Get(context.Background(), 999); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("Get(missing) = %v; want ErrNotFound", err)
	}
}
