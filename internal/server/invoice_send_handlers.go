package server

import (
	"fmt"
	"net/http"

	"github.com/ArfaMujahid/invoice-generator/internal/apperr"
	"github.com/ArfaMujahid/invoice-generator/internal/client"
	"github.com/ArfaMujahid/invoice-generator/internal/email"
	"github.com/ArfaMujahid/invoice-generator/internal/invoice"
	"github.com/ArfaMujahid/invoice-generator/internal/settings"
)

// handleInvoiceSend emails the invoice (PDF attached) to its client and, on
// success, marks it Sent (FR-4.1, FR-4.2).
func (s *Server) handleInvoiceSend(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.handleError(w, r, apperr.ErrNotFound)
		return
	}
	inv, cl, cfg, ok := s.loadForSend(w, r, id)
	if !ok {
		return
	}
	if cl.Email == "" {
		s.setFlash(w, flashError, "This client has no email address.")
		s.redirect(w, r, fmt.Sprintf("/invoices/%d", id))
		return
	}

	pdfBytes, err := s.deps.PDF.Render(r.Context(), inv, cl, cfg)
	if err != nil {
		s.serverError(w, err)
		return
	}
	msg := email.Message{
		To:      cl.Email,
		Subject: fmt.Sprintf("Invoice %s from %s", inv.Number, businessName(cfg)),
		Body:    invoiceEmailBody(inv, cfg),
		Attachments: []email.Attachment{{
			Filename:    inv.Number + ".pdf",
			ContentType: "application/pdf",
			Content:     pdfBytes,
		}},
	}
	if err := s.sendAndFlash(w, r, cfg, msg, id); err != nil {
		return
	}
	if err := s.deps.Invoices.MarkSent(r.Context(), id); err != nil {
		s.serverError(w, err)
		return
	}
	s.setFlash(w, flashSuccess, "Invoice sent to "+cl.Email+".")
	s.redirect(w, r, fmt.Sprintf("/invoices/%d", id))
}

// handleInvoiceRemind emails a payment reminder for an invoice and records that
// a reminder was sent (FR-4.3, FR-4.4).
func (s *Server) handleInvoiceRemind(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.handleError(w, r, apperr.ErrNotFound)
		return
	}
	inv, cl, cfg, ok := s.loadForSend(w, r, id)
	if !ok {
		return
	}
	if cl.Email == "" {
		s.setFlash(w, flashError, "This client has no email address.")
		s.redirect(w, r, fmt.Sprintf("/invoices/%d", id))
		return
	}

	msg := email.Message{
		To:      cl.Email,
		Subject: fmt.Sprintf("Reminder: invoice %s is due", inv.Number),
		Body:    reminderEmailBody(inv, cfg),
	}
	if err := s.sendAndFlash(w, r, cfg, msg, id); err != nil {
		return
	}
	if err := s.deps.Invoices.RecordReminder(r.Context(), id); err != nil {
		s.serverError(w, err)
		return
	}
	s.setFlash(w, flashSuccess, "Reminder sent to "+cl.Email+".")
	s.redirect(w, r, fmt.Sprintf("/invoices/%d", id))
}

// loadForSend fetches the invoice, its client, and settings, handling any error
// by writing a response and returning ok=false.
func (s *Server) loadForSend(w http.ResponseWriter, r *http.Request, id int64) (invoice.Invoice, client.Client, settings.Settings, bool) {
	inv, err := s.deps.Invoices.Get(r.Context(), id)
	if err != nil {
		s.handleError(w, r, err)
		return invoice.Invoice{}, client.Client{}, settings.Settings{}, false
	}
	cl, err := s.deps.Clients.Get(r.Context(), inv.ClientID)
	if err != nil {
		s.handleError(w, r, err)
		return invoice.Invoice{}, client.Client{}, settings.Settings{}, false
	}
	cfg, err := s.deps.Settings.Get(r.Context())
	if err != nil {
		s.handleError(w, r, err)
		return invoice.Invoice{}, client.Client{}, settings.Settings{}, false
	}
	return inv, cl, cfg, true
}

// sendAndFlash sends msg, and on failure sets an error flash, redirects, and
// returns the error so the caller stops. On success it returns nil.
func (s *Server) sendAndFlash(w http.ResponseWriter, r *http.Request, cfg settings.Settings, msg email.Message, id int64) error {
	if err := s.deps.Mailer.Send(r.Context(), cfg, msg); err != nil {
		s.setFlash(w, flashError, "Could not send email: "+err.Error())
		s.redirect(w, r, fmt.Sprintf("/invoices/%d", id))
		return err
	}
	return nil
}

// invoiceEmailBody is the default message accompanying a sent invoice.
func invoiceEmailBody(inv invoice.Invoice, cfg settings.Settings) string {
	body := fmt.Sprintf(
		"Dear %s,\n\nPlease find attached invoice %s for %s %s, due %s.\n",
		"customer", inv.Number, inv.Currency, money(inv.Total()), inv.DueDate.Format("2006-01-02"))
	if inv.Notes != "" {
		body += "\n" + inv.Notes + "\n"
	}
	body += "\nThank you,\n" + businessName(cfg)
	return body
}

// reminderEmailBody is the default payment-reminder message (FR-4.3).
func reminderEmailBody(inv invoice.Invoice, cfg settings.Settings) string {
	return fmt.Sprintf(
		"Dear customer,\n\nThis is a friendly reminder that invoice %s for %s %s was due on %s and remains outstanding (balance %s %s).\n\nThank you,\n%s",
		inv.Number, inv.Currency, money(inv.Total()), inv.DueDate.Format("2006-01-02"),
		inv.Currency, money(inv.Balance()), businessName(cfg))
}

// businessName returns the configured business name or a neutral default.
func businessName(cfg settings.Settings) string {
	if cfg.BusinessName != "" {
		return cfg.BusinessName
	}
	return "Accounts"
}
