package invoice

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/ArfaMujahid/invoice-generator/internal/apperr"
	"github.com/ArfaMujahid/invoice-generator/internal/settings"
	"github.com/ArfaMujahid/invoice-generator/internal/store"
)

// setup opens a store, inserts a client, and returns the invoice store + client ID.
func setup(t *testing.T) (*Store, int64) {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "inv.db"))
	if err != nil {
		t.Fatalf("store.Open() error: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	res, err := st.DB().Exec(`INSERT INTO clients (name, email) VALUES ('Client', 'c@x.test')`)
	if err != nil {
		t.Fatalf("seeding client: %v", err)
	}
	cid, _ := res.LastInsertId()
	return NewStore(st), cid
}

var testCfg = settings.Settings{InvoicePrefix: "INV", InvoiceFormat: "{PREFIX}-{SEQ}"}

// sampleInvoice builds an invoice for clientID with two line items and 10% tax.
func sampleInvoice(clientID int64, due time.Time) Invoice {
	return Invoice{
		ClientID: clientID, Currency: "USD", TaxRate: 10,
		IssueDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		DueDate:   due,
		LineItems: []LineItem{
			{Description: "Design", Quantity: 2, UnitPrice: 5000},  // 100.00
			{Description: "Hosting", Quantity: 1, UnitPrice: 2500}, // 25.00
		},
	}
}

// TestCreateAndGetTotals checks creation, number assignment, and total math.
func TestCreateAndGetTotals(t *testing.T) {
	ctx := context.Background()
	repo, cid := setup(t)

	created, err := repo.Create(ctx, sampleInvoice(cid, time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)), testCfg)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if created.Number != "INV-0001" {
		t.Errorf("number = %q; want INV-0001", created.Number)
	}

	got, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got.Subtotal() != 12500 || got.Tax() != 1250 || got.Total() != 13750 {
		t.Errorf("totals: subtotal=%d tax=%d total=%d; want 12500/1250/13750",
			got.Subtotal(), got.Tax(), got.Total())
	}
	if len(got.LineItems) != 2 {
		t.Errorf("line items = %d; want 2", len(got.LineItems))
	}
}

// TestPaymentsAndStatus checks partial payment, balance, and auto-paid status.
func TestPaymentsAndStatus(t *testing.T) {
	ctx := context.Background()
	repo, cid := setup(t)
	inv, err := repo.Create(ctx, sampleInvoice(cid, time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)), testCfg)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := repo.SetStatus(ctx, inv.ID, StatusSent); err != nil {
		t.Fatalf("SetStatus() error: %v", err)
	}

	bal, err := repo.AddPayment(ctx, Payment{InvoiceID: inv.ID, Amount: 10000, PaidOn: time.Now()})
	if err != nil {
		t.Fatalf("AddPayment() error: %v", err)
	}
	if bal != 3750 {
		t.Errorf("balance after partial = %d; want 3750", bal)
	}

	bal, err = repo.AddPayment(ctx, Payment{InvoiceID: inv.ID, Amount: 3750, PaidOn: time.Now()})
	if err != nil {
		t.Fatalf("AddPayment() error: %v", err)
	}
	if bal != 0 {
		t.Errorf("balance after full = %d; want 0", bal)
	}
	paid, _ := repo.Get(ctx, inv.ID)
	if paid.Status != StatusPaid {
		t.Errorf("status = %q; want paid after full payment", paid.Status)
	}
}

// TestAddPaymentRejectsNonPositive validates the payment amount.
func TestAddPaymentRejectsNonPositive(t *testing.T) {
	ctx := context.Background()
	repo, cid := setup(t)
	inv, _ := repo.Create(ctx, sampleInvoice(cid, time.Now()), testCfg)
	_, err := repo.AddPayment(ctx, Payment{InvoiceID: inv.ID, Amount: 0, PaidOn: time.Now()})
	var verr *apperr.ValidationError
	if !errors.As(err, &verr) {
		t.Errorf("AddPayment(0) = %v; want ValidationError", err)
	}
}

// TestMarkOverdue marks only sent, past-due invoices.
func TestMarkOverdue(t *testing.T) {
	ctx := context.Background()
	repo, cid := setup(t)
	inv, _ := repo.Create(ctx, sampleInvoice(cid, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)), testCfg)
	if err := repo.SetStatus(ctx, inv.ID, StatusSent); err != nil {
		t.Fatalf("SetStatus() error: %v", err)
	}
	n, err := repo.MarkOverdue(ctx, "2025-01-01")
	if err != nil {
		t.Fatalf("MarkOverdue() error: %v", err)
	}
	if n != 1 {
		t.Errorf("marked = %d; want 1", n)
	}
	got, _ := repo.Get(ctx, inv.ID)
	if got.Status != StatusOverdue {
		t.Errorf("status = %q; want overdue", got.Status)
	}
}

// TestListFilters checks status and client filtering.
func TestListFilters(t *testing.T) {
	ctx := context.Background()
	repo, cid := setup(t)
	a, _ := repo.Create(ctx, sampleInvoice(cid, time.Now()), testCfg)
	_, _ = repo.Create(ctx, sampleInvoice(cid, time.Now()), testCfg)
	_ = repo.SetStatus(ctx, a.ID, StatusSent)

	all, err := repo.List(ctx, Filter{})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("List(all) = %d; want 2", len(all))
	}
	sent, _ := repo.List(ctx, Filter{Status: StatusSent})
	if len(sent) != 1 || sent[0].ID != a.ID {
		t.Errorf("List(sent) = %+v; want only invoice %d", sent, a.ID)
	}
	if got, _ := repo.List(ctx, Filter{ClientID: cid}); len(got) != 2 {
		t.Errorf("List(client) = %d; want 2", len(got))
	}
}
