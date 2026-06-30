// Package invoice implements invoice creation and tracking: the Invoice
// aggregate (with line items and payments), money/total calculation, and the
// persistence repository (SRS Modules 2 & 3).
package invoice

import "time"

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
