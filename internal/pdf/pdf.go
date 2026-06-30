// Package pdf renders invoices to branded PDF documents server-side (SRS §3.2,
// FR-2.4). It depends on the invoice and settings domains for its inputs and
// returns raw PDF bytes the caller can stream to a response or attach to email.
package pdf

import (
	"context"

	"github.com/ArfaMujahid/invoice-generator/internal/apperr"
	"github.com/ArfaMujahid/invoice-generator/internal/invoice"
	"github.com/ArfaMujahid/invoice-generator/internal/settings"
)

// Generator renders invoices to PDF. It is a concrete type per
// coding-standards §5 ("return concrete types"); the server depends on a small
// interface it defines for itself.
type Generator struct{}

// NewGenerator returns a ready-to-use PDF generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// Render produces the PDF bytes for inv, branded with the business profile in
// cfg (logo, business details) and including client details, the line-item
// table, totals, and payment instructions (FR-2.4, FR-2.5).
//
// TODO(arfa): lay out the document and emit PDF bytes. Evaluate a single, small
// pure-Go PDF library (see README dependency note) before adding it.
func (g *Generator) Render(ctx context.Context, inv invoice.Invoice, cfg settings.Settings) ([]byte, error) {
	return nil, apperr.ErrNotImplemented
}
