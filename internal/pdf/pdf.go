// Package pdf renders invoices to branded PDF documents server-side (SRS §3.2,
// FR-2.4). It depends on the client, invoice, and settings domains for its
// inputs and returns raw PDF bytes the caller can stream to a response or attach
// to email.
package pdf

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-pdf/fpdf"

	"github.com/ArfaMujahid/invoice-generator/internal/client"
	"github.com/ArfaMujahid/invoice-generator/internal/invoice"
	"github.com/ArfaMujahid/invoice-generator/internal/settings"
)

// Generator renders invoices to PDF. It is a concrete type per
// coding-standards §5 ("return concrete types"); the server depends on a small
// interface it defines for itself.
type Generator struct {
	// uploadsDir is where the business logo lives on disk, so the renderer can
	// resolve cfg.LogoPath (a bare filename) to a full path.
	uploadsDir string
}

// NewGenerator returns a PDF generator that resolves logo files from uploadsDir.
func NewGenerator(uploadsDir string) *Generator {
	return &Generator{uploadsDir: uploadsDir}
}

// Render produces the PDF bytes for inv, branded with the business profile in
// cfg (logo, business details) and including client details, the line-item
// table, totals, and payment instructions (FR-2.4, FR-2.5).
func (g *Generator) Render(ctx context.Context, inv invoice.Invoice, cl client.Client, cfg settings.Settings) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()
	// tr maps UTF-8 text to the core font's encoding so accented characters in
	// names/descriptions render instead of breaking.
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	g.header(pdf, tr, inv, cfg)
	billTo(pdf, tr, cl)
	lineItems(pdf, tr, inv)
	totals(pdf, inv)
	notes(pdf, tr, inv)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("rendering invoice pdf: %w", err)
	}
	return buf.Bytes(), nil
}

// header draws the business identity (logo + details) and the invoice title.
func (g *Generator) header(pdf *fpdf.Fpdf, tr func(string) string, inv invoice.Invoice, cfg settings.Settings) {
	if cfg.LogoPath != "" {
		path := filepath.Join(g.uploadsDir, cfg.LogoPath)
		if _, err := os.Stat(path); err == nil {
			pdf.ImageOptions(path, 15, 14, 32, 0, false, fpdf.ImageOptions{}, 0, "")
		}
	}

	pdf.SetXY(110, 14)
	pdf.SetFont("Arial", "B", 12)
	business := cfg.BusinessName
	if business == "" {
		business = "Your Business"
	}
	pdf.CellFormat(85, 6, tr(business), "", 2, "R", false, 0, "")
	pdf.SetFont("Arial", "", 9)
	for _, line := range splitLines(cfg.BusinessAddress) {
		pdf.CellFormat(85, 5, tr(line), "", 2, "R", false, 0, "")
	}
	if cfg.TaxID != "" {
		pdf.CellFormat(85, 5, tr("Tax ID: "+cfg.TaxID), "", 2, "R", false, 0, "")
	}

	pdf.SetXY(15, 40)
	pdf.SetFont("Arial", "B", 20)
	pdf.CellFormat(90, 10, "INVOICE", "", 2, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(90, 6, tr(inv.Number), "", 2, "L", false, 0, "")
	pdf.CellFormat(90, 6, "Issued: "+inv.IssueDate.Format("2006-01-02"), "", 2, "L", false, 0, "")
	pdf.CellFormat(90, 6, "Due: "+inv.DueDate.Format("2006-01-02"), "", 2, "L", false, 0, "")
	pdf.Ln(4)
}

// billTo draws the client block.
func billTo(pdf *fpdf.Fpdf, tr func(string) string, cl client.Client) {
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 6, "Bill to", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 5, tr(cl.Name), "", 1, "L", false, 0, "")
	if cl.Company != "" {
		pdf.CellFormat(0, 5, tr(cl.Company), "", 1, "L", false, 0, "")
	}
	if cl.Email != "" {
		pdf.CellFormat(0, 5, tr(cl.Email), "", 1, "L", false, 0, "")
	}
	for _, line := range splitLines(cl.BillingAddress) {
		pdf.CellFormat(0, 5, tr(line), "", 1, "L", false, 0, "")
	}
	pdf.Ln(4)
}

// lineItems draws the line-item table.
func lineItems(pdf *fpdf.Fpdf, tr func(string) string, inv invoice.Invoice) {
	pdf.SetFont("Arial", "B", 10)
	pdf.SetFillColor(245, 246, 248)
	pdf.CellFormat(90, 8, "Description", "1", 0, "L", true, 0, "")
	pdf.CellFormat(20, 8, "Qty", "1", 0, "R", true, 0, "")
	pdf.CellFormat(35, 8, "Unit price", "1", 0, "R", true, 0, "")
	pdf.CellFormat(35, 8, "Total", "1", 1, "R", true, 0, "")

	pdf.SetFont("Arial", "", 10)
	for _, li := range inv.LineItems {
		pdf.CellFormat(90, 7, tr(li.Description), "1", 0, "L", false, 0, "")
		pdf.CellFormat(20, 7, formatQty(li.Quantity), "1", 0, "R", false, 0, "")
		pdf.CellFormat(35, 7, fmtMoney(li.UnitPrice), "1", 0, "R", false, 0, "")
		pdf.CellFormat(35, 7, fmtMoney(li.LineTotal()), "1", 1, "R", false, 0, "")
	}
	pdf.Ln(2)
}

// totals draws the subtotal/tax/total/paid/balance summary, right-aligned.
func totals(pdf *fpdf.Fpdf, inv invoice.Invoice) {
	row := func(label, value string, bold bool) {
		style := ""
		if bold {
			style = "B"
		}
		pdf.SetFont("Arial", style, 10)
		pdf.CellFormat(110, 7, "", "", 0, "L", false, 0, "")
		pdf.CellFormat(35, 7, label, "", 0, "R", false, 0, "")
		pdf.CellFormat(35, 7, inv.Currency+" "+value, "", 1, "R", false, 0, "")
	}
	row("Subtotal", fmtMoney(inv.Subtotal()), false)
	row(fmt.Sprintf("Tax (%s%%)", trimFloat(inv.TaxRate)), fmtMoney(inv.Tax()), false)
	row("Total", fmtMoney(inv.Total()), true)
	if inv.Paid() > 0 {
		row("Paid", fmtMoney(inv.Paid()), false)
		row("Balance due", fmtMoney(inv.Balance()), true)
	}
}

// notes draws the free-text notes / payment instructions block.
func notes(pdf *fpdf.Fpdf, tr func(string) string, inv invoice.Invoice) {
	if strings.TrimSpace(inv.Notes) == "" {
		return
	}
	pdf.Ln(6)
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(0, 6, "Notes", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.MultiCell(0, 5, tr(inv.Notes), "", "L", false)
}

// fmtMoney formats minor units as a decimal string (e.g. 123456 -> "1234.56").
func fmtMoney(m invoice.Money) string {
	neg := m < 0
	if neg {
		m = -m
	}
	s := fmt.Sprintf("%d.%02d", int64(m)/100, int64(m)%100)
	if neg {
		return "-" + s
	}
	return s
}

// formatQty renders a quantity without a trailing ".0" for whole numbers.
func formatQty(q float64) string {
	return trimFloat(q)
}

// trimFloat formats a float without trailing zeros (e.g. 2 -> "2", 1.5 -> "1.5").
func trimFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// splitLines splits a multi-line string into non-empty trimmed lines.
func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			out = append(out, t)
		}
	}
	return out
}
