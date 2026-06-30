package pdf

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/ArfaMujahid/invoice-generator/internal/client"
	"github.com/ArfaMujahid/invoice-generator/internal/invoice"
	"github.com/ArfaMujahid/invoice-generator/internal/settings"
)

// TestRenderProducesPDF checks Render returns non-empty, well-formed PDF bytes.
func TestRenderProducesPDF(t *testing.T) {
	g := NewGenerator(t.TempDir())
	inv := invoice.Invoice{
		Number: "INV-2025-0001", Currency: "USD", TaxRate: 10,
		IssueDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		DueDate:   time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
		Notes:     "Pay within 30 days.",
		LineItems: []invoice.LineItem{
			{Description: "Design", Quantity: 2, UnitPrice: 5000},
		},
	}
	cl := client.Client{Name: "Globex", Email: "ap@globex.test", BillingAddress: "1 Main St\nMetropolis"}
	cfg := settings.Settings{BusinessName: "Acme Studio", BusinessAddress: "2 Market St"}

	data, err := g.Render(context.Background(), inv, cl, cfg)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Render() returned no bytes")
	}
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Errorf("output does not start with %%PDF header: %q", data[:min(8, len(data))])
	}
}
