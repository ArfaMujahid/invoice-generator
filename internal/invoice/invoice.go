// Package invoice implements invoice creation and tracking: the Invoice
// aggregate (with line items and payments), money/total calculation, and the
// persistence repository (SRS Modules 2 & 3).
package invoice

import (
	"fmt"
	"strings"
	"time"

	"github.com/ArfaMujahid/invoice-generator/internal/apperr"
)

// Money is a monetary amount in integer minor units (e.g. cents). Integers are
// used instead of floats so totals never accumulate binary rounding error.
type Money int64

// Status is the lifecycle state of an invoice (FR-3.1).
type Status string

// The four invoice statuses defined by the SRS.
const (
	StatusDraft   Status = "draft"
	StatusSent    Status = "sent"
	StatusPaid    Status = "paid"
	StatusOverdue Status = "overdue"
)

// Valid reports whether s is one of the recognised statuses.
func (s Status) Valid() bool {
	switch s {
	case StatusDraft, StatusSent, StatusPaid, StatusOverdue:
		return true
	default:
		return false
	}
}

// LineItem is a single billable row on an invoice (SRS §4.3).
type LineItem struct {
	ID          int64
	InvoiceID   int64
	Description string
	Quantity    float64
	UnitPrice   Money
	Position    int
}

// LineTotal is the row's extended amount (quantity × unit price), rounded to the
// nearest minor unit.
func (li LineItem) LineTotal() Money {
	return Money(float64(li.UnitPrice)*li.Quantity + 0.5)
}

// Payment is a (possibly partial) payment recorded against an invoice (SRS §4.4).
type Payment struct {
	ID        int64
	InvoiceID int64
	Amount    Money
	PaidOn    time.Time
}

// Invoice is the aggregate root: header fields plus its line items and payments
// (SRS §4.2).
type Invoice struct {
	ID            int64
	Number        string
	ClientID      int64
	IssueDate     time.Time
	DueDate       time.Time
	Status        Status
	Currency      string
	TaxRate       float64 // percentage, e.g. 15.0
	Notes         string
	SentAt        *time.Time
	RemindersSent int
	LineItems     []LineItem
	Payments      []Payment
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Subtotal sums every line item's total before tax (FR-2.3).
func (inv Invoice) Subtotal() Money {
	var sum Money
	for _, li := range inv.LineItems {
		sum += li.LineTotal()
	}
	return sum
}

// Tax is the tax amount derived from the subtotal and TaxRate (FR-2.3).
func (inv Invoice) Tax() Money {
	return Money(float64(inv.Subtotal())*inv.TaxRate/100 + 0.5)
}

// Total is the grand total the client owes: subtotal plus tax (FR-2.3).
func (inv Invoice) Total() Money {
	return inv.Subtotal() + inv.Tax()
}

// Paid sums all recorded payments against the invoice.
func (inv Invoice) Paid() Money {
	var sum Money
	for _, p := range inv.Payments {
		sum += p.Amount
	}
	return sum
}

// Balance is the amount still outstanding after payments (FR-3.5).
func (inv Invoice) Balance() Money {
	return inv.Total() - inv.Paid()
}

// Summary holds the dashboard aggregate amounts across all invoices (FR-3.2).
type Summary struct {
	TotalInvoiced    Money
	TotalPaid        Money
	TotalOutstanding Money
	TotalOverdue     Money
}

// ListItem is a lightweight invoice row for tables (invoice list, client
// history). It carries the client name and computed amounts so the list view
// needs no further lookups. Dates are kept as display strings.
type ListItem struct {
	ID         int64
	Number     string
	ClientID   int64
	ClientName string
	IssueDate  string
	DueDate    string
	Status     Status
	Currency   string
	Total      Money
	Paid       Money
	Balance    Money
}

// Validate checks the invoice is well-formed before it is saved: a client, both
// dates, a currency, a sane tax rate, and at least one complete line item
// (FR-2.1, FR-2.2). It returns an *apperr.ValidationError listing every problem.
func (inv Invoice) Validate() error {
	v := apperr.NewValidationError()
	if inv.ClientID <= 0 {
		v.Add("client_id", "select a client")
	}
	if inv.IssueDate.IsZero() {
		v.Add("issue_date", "issue date is required")
	}
	if inv.DueDate.IsZero() {
		v.Add("due_date", "due date is required")
	}
	if strings.TrimSpace(inv.Currency) == "" {
		v.Add("currency", "currency is required")
	}
	if inv.TaxRate < 0 || inv.TaxRate > 100 {
		v.Add("tax_rate", "tax rate must be between 0 and 100")
	}
	if len(inv.LineItems) == 0 {
		v.Add("line_items", "add at least one line item")
	}
	for i, li := range inv.LineItems {
		if strings.TrimSpace(li.Description) == "" {
			v.Add(fmt.Sprintf("line_%d_description", i), "description is required")
		}
		if li.Quantity <= 0 {
			v.Add(fmt.Sprintf("line_%d_quantity", i), "quantity must be positive")
		}
		if li.UnitPrice < 0 {
			v.Add(fmt.Sprintf("line_%d_unit_price", i), "price cannot be negative")
		}
	}
	if v.HasErrors() {
		return v
	}
	return nil
}
