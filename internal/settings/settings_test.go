package settings

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ArfaMujahid/invoice-generator/internal/store"
)

// newTestStore opens a settings Store backed by a throwaway database.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatalf("store.Open() error: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return NewStore(st)
}

// TestGetReturnsSeededDefaults checks the schema-seeded settings row is readable
// with its default values.
func TestGetReturnsSeededDefaults(t *testing.T) {
	repo := newTestStore(t)
	cfg, err := repo.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if cfg.InvoicePrefix != "INV" {
		t.Errorf("InvoicePrefix = %q; want INV", cfg.InvoicePrefix)
	}
	if cfg.SMTPPort != 587 {
		t.Errorf("SMTPPort = %d; want 587", cfg.SMTPPort)
	}
	if cfg.BusinessName != "" {
		t.Errorf("BusinessName = %q; want empty", cfg.BusinessName)
	}
}

// TestSaveProfileRoundTrip verifies the business profile persists and that
// SaveProfile leaves unrelated settings (SMTP, numbering) untouched.
func TestSaveProfileRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo := newTestStore(t)

	in := Settings{
		BusinessName:    "Acme Studio",
		BusinessAddress: "1 Market St\nSan Francisco",
		TaxID:           "TX-123",
		LogoPath:        "logo.png",
	}
	if err := repo.SaveProfile(ctx, in); err != nil {
		t.Fatalf("SaveProfile() error: %v", err)
	}

	got, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got.BusinessName != in.BusinessName ||
		got.BusinessAddress != in.BusinessAddress ||
		got.TaxID != in.TaxID ||
		got.LogoPath != in.LogoPath {
		t.Errorf("profile not persisted: got %+v; want %+v", got, in)
	}
	// Unrelated defaults must survive a profile save.
	if got.InvoicePrefix != "INV" || got.SMTPPort != 587 {
		t.Errorf("SaveProfile clobbered unrelated settings: prefix=%q port=%d", got.InvoicePrefix, got.SMTPPort)
	}
}
