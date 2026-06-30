package server

import (
	"errors"
	"net/http"

	"github.com/ArfaMujahid/invoice-generator/internal/apperr"
	"github.com/ArfaMujahid/invoice-generator/internal/invoice"
)

// dashboardView is the data passed to the dashboard template.
type dashboardView struct {
	Title   string
	Summary invoice.Summary
}

// messageView is the data passed to the generic message/status page.
type messageView struct {
	Title   string
	Heading string
	Body    string
}

// handleHealth is a liveness probe returning 200 OK with a tiny body.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// handleDashboard renders the dashboard summary cards (FR-3.2). It is fully
// implemented: the aggregate amounts are read live and are zero on a fresh
// database.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	sum, err := s.deps.Invoices.Summary(r.Context())
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "dashboard", dashboardView{
		Title:   "Dashboard",
		Summary: sum,
	})
}

// handleError maps a domain error to an HTTP status and renders the message
// page. It centralises the error→status mapping so every handler reports
// consistently (coding-standards §3: handle each error once, at the boundary).
func (s *Server) handleError(w http.ResponseWriter, r *http.Request, err error) {
	var verr *apperr.ValidationError
	switch {
	case errors.Is(err, apperr.ErrNotImplemented):
		s.render(w, r, http.StatusNotImplemented, "message", messageView{
			Title:   "Coming soon",
			Heading: "Not implemented yet",
			Body:    "This screen is part of the project skeleton and has not been built yet.",
		})
	case errors.Is(err, apperr.ErrNotFound):
		s.render(w, r, http.StatusNotFound, "message", messageView{
			Title:   "Not found",
			Heading: "Not found",
			Body:    "The requested item does not exist.",
		})
	case errors.As(err, &verr):
		s.render(w, r, http.StatusBadRequest, "message", messageView{
			Title:   "Invalid input",
			Heading: "Please check your input",
			Body:    verr.Error(),
		})
	default:
		s.serverError(w, err)
	}
}

// serverError logs an unexpected error once and returns a generic 500 page so
// internal details never leak to the client (NFR-4).
func (s *Server) serverError(w http.ResponseWriter, err error) {
	s.deps.Logger.Error("unhandled server error", "err", err)
	// Render directly (not via render→serverError) to avoid any recursion if the
	// template itself is the failure.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write([]byte("<h1>Something went wrong</h1><p>Please try again.</p>"))
}
