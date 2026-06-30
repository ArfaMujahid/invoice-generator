package server

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ArfaMujahid/invoice-generator/internal/apperr"
	"github.com/ArfaMujahid/invoice-generator/internal/client"
	"github.com/ArfaMujahid/invoice-generator/internal/invoice"
)

// currencies offered in the invoice editor's currency selector (FR-2.6).
var currencies = []string{"USD", "EUR", "GBP", "PKR", "AED"}

// invoicesView is the data for the invoice list page (FR-3.3).
type invoicesView struct {
	Title    string
	Invoices []invoice.ListItem
	Clients  []client.ListItem
	Filter   invoice.Filter
	Statuses []invoice.Status
}

// invoiceFormView is the data for the invoice editor (new/edit).
type invoiceFormView struct {
	Title      string
	Action     string
	Invoice    invoice.Invoice
	Clients    []client.ListItem
	Currencies []string
	Errors     map[string]string
}

// invoiceView is the data for the invoice preview page.
type invoiceView struct {
	Title   string
	Invoice invoice.Invoice
	Client  client.Client
}

// handleInvoicesList renders the filterable, searchable invoice table (FR-3.3).
func (s *Server) handleInvoicesList(w http.ResponseWriter, r *http.Request) {
	f := invoice.Filter{
		Status:   invoice.Status(r.URL.Query().Get("status")),
		Search:   r.URL.Query().Get("q"),
		From:     r.URL.Query().Get("from"),
		To:       r.URL.Query().Get("to"),
		ClientID: parseInt64(r.URL.Query().Get("client_id")),
	}
	invoices, err := s.deps.Invoices.List(r.Context(), f)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	clients, err := s.deps.Clients.List(r.Context(), false)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "invoices", invoicesView{
		Title:    "Invoices",
		Invoices: invoices,
		Clients:  clients,
		Filter:   f,
		Statuses: []invoice.Status{invoice.StatusDraft, invoice.StatusSent, invoice.StatusPaid, invoice.StatusOverdue},
	})
}

// handleInvoiceNew renders the editor for a new invoice, pre-filled with today's
// issue date, a +30-day due date, and the default tax rate (FR-2.1).
func (s *Server) handleInvoiceNew(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.deps.Settings.Get(r.Context())
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	clients, err := s.deps.Clients.List(r.Context(), false)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	now := time.Now()
	inv := invoice.Invoice{
		Status:    invoice.StatusDraft,
		IssueDate: now,
		DueDate:   now.AddDate(0, 0, 30),
		Currency:  "USD",
		TaxRate:   cfg.DefaultTaxRate,
	}
	s.render(w, r, http.StatusOK, "invoice_form", invoiceFormView{
		Title: "New invoice", Action: "/invoices", Invoice: inv,
		Clients: clients, Currencies: currencies, Errors: map[string]string{},
	})
}

// handleInvoiceCreate validates and stores a new invoice with its line items,
// then redirects to its preview (FR-2.1, FR-2.2, FR-2.3, FR-2.5, FR-2.6, FR-2.7).
func (s *Server) handleInvoiceCreate(w http.ResponseWriter, r *http.Request) {
	f, err := parseForm(r)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	cfg, err := s.deps.Settings.Get(r.Context())
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	inv := s.invoiceFromForm(f, 0)

	created, err := s.deps.Invoices.Create(r.Context(), inv, cfg)
	if err != nil {
		if s.renderInvoiceFormError(w, r, err, "New invoice", "/invoices", inv) {
			return
		}
		s.serverError(w, err)
		return
	}
	s.setFlash(w, flashSuccess, "Invoice "+created.Number+" created.")
	s.redirect(w, r, fmt.Sprintf("/invoices/%d", created.ID))
}

// handleInvoiceView renders the invoice preview with computed totals (FR-2.4
// preview; the PDF is generated from the same data).
func (s *Server) handleInvoiceView(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.handleError(w, r, apperr.ErrNotFound)
		return
	}
	inv, err := s.deps.Invoices.Get(r.Context(), id)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	cl, err := s.deps.Clients.Get(r.Context(), inv.ClientID)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "invoice_view", invoiceView{
		Title:   inv.Number,
		Invoice: inv,
		Client:  cl,
	})
}

// handleInvoiceEdit renders the editor populated with an existing invoice (FR-2.7).
func (s *Server) handleInvoiceEdit(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.handleError(w, r, apperr.ErrNotFound)
		return
	}
	inv, err := s.deps.Invoices.Get(r.Context(), id)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	clients, err := s.deps.Clients.List(r.Context(), false)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "invoice_form", invoiceFormView{
		Title: "Edit " + inv.Number, Action: fmt.Sprintf("/invoices/%d", id), Invoice: inv,
		Clients: clients, Currencies: currencies, Errors: map[string]string{},
	})
}

// handleInvoiceUpdate validates and saves edits to an existing invoice (FR-2.7).
func (s *Server) handleInvoiceUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.handleError(w, r, apperr.ErrNotFound)
		return
	}
	f, err := parseForm(r)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	inv := s.invoiceFromForm(f, id)

	if _, err := s.deps.Invoices.Update(r.Context(), inv); err != nil {
		if s.renderInvoiceFormError(w, r, err, "Edit invoice", fmt.Sprintf("/invoices/%d", id), inv) {
			return
		}
		s.handleError(w, r, err)
		return
	}
	s.setFlash(w, flashSuccess, "Invoice updated.")
	s.redirect(w, r, fmt.Sprintf("/invoices/%d", id))
}

// handleInvoiceStatus changes an invoice's status (FR-3.1).
func (s *Server) handleInvoiceStatus(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.handleError(w, r, apperr.ErrNotFound)
		return
	}
	f, err := parseForm(r)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	if err := s.deps.Invoices.SetStatus(r.Context(), id, invoice.Status(f.String("status"))); err != nil {
		s.handleError(w, r, err)
		return
	}
	s.setFlash(w, flashSuccess, "Status updated.")
	s.redirect(w, r, fmt.Sprintf("/invoices/%d", id))
}

// handleInvoicePayment records a (possibly partial) payment against an invoice
// and reports the remaining balance (FR-3.5).
func (s *Server) handleInvoicePayment(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.handleError(w, r, apperr.ErrNotFound)
		return
	}
	f, err := parseForm(r)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	amount, err := parseMoney(f.String("amount"))
	if err != nil {
		s.setFlash(w, flashError, "Payment amount is not a valid number.")
		s.redirect(w, r, fmt.Sprintf("/invoices/%d", id))
		return
	}
	paidOn := time.Now()
	if d := f.String("paid_on"); d != "" {
		if t, perr := time.Parse("2006-01-02", d); perr == nil {
			paidOn = t
		}
	}

	balance, err := s.deps.Invoices.AddPayment(r.Context(), invoice.Payment{
		InvoiceID: id, Amount: amount, PaidOn: paidOn,
	})
	if err != nil {
		var verr *apperr.ValidationError
		if errors.As(err, &verr) {
			s.setFlash(w, flashError, verr.Error())
			s.redirect(w, r, fmt.Sprintf("/invoices/%d", id))
			return
		}
		s.handleError(w, r, err)
		return
	}
	if balance <= 0 {
		s.setFlash(w, flashSuccess, "Payment recorded. Invoice is fully paid.")
	} else {
		s.setFlash(w, flashSuccess, "Payment recorded. Balance remaining: "+money(balance))
	}
	s.redirect(w, r, fmt.Sprintf("/invoices/%d", id))
}

// invoiceFromForm builds an Invoice (with line items) from submitted fields.
// Dates and money are parsed here; domain validation happens in the store.
func (s *Server) invoiceFromForm(f *form, id int64) invoice.Invoice {
	inv := invoice.Invoice{
		ID:       id,
		ClientID: parseInt64(f.String("client_id")),
		Currency: f.String("currency"),
		TaxRate:  f.Float("tax_rate", 0),
		Notes:    f.String("notes"),
	}
	if t, err := time.Parse("2006-01-02", f.String("issue_date")); err == nil {
		inv.IssueDate = t
	}
	if t, err := time.Parse("2006-01-02", f.String("due_date")); err == nil {
		inv.DueDate = t
	}

	descs := f.Strings("description[]")
	qtys := f.Strings("quantity[]")
	prices := f.Strings("unit_price[]")
	for i := range descs {
		desc := strings.TrimSpace(descs[i])
		qty := atProvider(qtys, i)
		price := atProvider(prices, i)
		if desc == "" && qty == "" && price == "" {
			continue // skip blank rows
		}
		q, _ := strconv.ParseFloat(strings.TrimSpace(qty), 64)
		p, _ := parseMoney(price)
		inv.LineItems = append(inv.LineItems, invoice.LineItem{
			Description: desc, Quantity: q, UnitPrice: p, Position: i,
		})
	}
	return inv
}

// renderInvoiceFormError re-renders the editor with per-field errors when err is
// a validation error, returning true if it handled the error.
func (s *Server) renderInvoiceFormError(w http.ResponseWriter, r *http.Request, err error, title, action string, inv invoice.Invoice) bool {
	var verr *apperr.ValidationError
	if !errors.As(err, &verr) {
		return false
	}
	clients, lerr := s.deps.Clients.List(r.Context(), false)
	if lerr != nil {
		s.serverError(w, lerr)
		return true
	}
	s.render(w, r, http.StatusUnprocessableEntity, "invoice_form", invoiceFormView{
		Title: title, Action: action, Invoice: inv,
		Clients: clients, Currencies: currencies, Errors: verr.Fields,
	})
	return true
}

// atProvider returns s[i] or "" if i is out of range.
func atProvider(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return ""
}

// parseMoney converts a decimal currency string (e.g. "9.99") to minor units.
func parseMoney(s string) (invoice.Money, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return invoice.Money(math.Round(v * 100)), nil
}

// parseInt64 parses a base-10 int64, returning 0 on error (for optional params).
func parseInt64(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}
