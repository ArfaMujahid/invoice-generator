package server

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ArfaMujahid/invoice-generator/internal/apperr"
	"github.com/ArfaMujahid/invoice-generator/internal/client"
)

// clientsView is the data for the clients list page.
type clientsView struct {
	Title   string
	Clients []client.ListItem
}

// clientFormView is the data for the new/edit client form.
type clientFormView struct {
	Title  string
	Action string // form post target
	Client client.Client
	Errors map[string]string
}

// handleClientsList renders the clients table with per-client totals (FR-1.3).
func (s *Server) handleClientsList(w http.ResponseWriter, r *http.Request) {
	clients, err := s.deps.Clients.List(r.Context(), false)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "clients", clientsView{Title: "Clients", Clients: clients})
}

// handleClientNew renders the empty create-client form (FR-1.1).
func (s *Server) handleClientNew(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "client_form", clientFormView{
		Title:  "New client",
		Action: "/clients",
		Errors: map[string]string{},
	})
}

// handleClientCreate validates and stores a new client, re-rendering the form
// with per-field errors on invalid input (FR-1.1, FR-1.6).
func (s *Server) handleClientCreate(w http.ResponseWriter, r *http.Request) {
	f, err := parseForm(r)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	c := clientFromForm(f, 0)

	if _, err := s.deps.Clients.Create(r.Context(), c); err != nil {
		var verr *apperr.ValidationError
		if errors.As(err, &verr) {
			s.render(w, r, http.StatusUnprocessableEntity, "client_form", clientFormView{
				Title: "New client", Action: "/clients", Client: c, Errors: verr.Fields,
			})
			return
		}
		s.serverError(w, err)
		return
	}
	s.setFlash(w, flashSuccess, "Client created.")
	s.redirect(w, r, "/clients")
}

// handleClientEdit renders the edit form for an existing client (FR-1.2).
func (s *Server) handleClientEdit(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.handleError(w, r, apperr.ErrNotFound)
		return
	}
	c, err := s.deps.Clients.Get(r.Context(), id)
	if err != nil {
		s.handleError(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "client_form", clientFormView{
		Title:  "Edit client",
		Action: fmt.Sprintf("/clients/%d", id),
		Client: c,
		Errors: map[string]string{},
	})
}

// handleClientUpdate validates and saves changes to a client (FR-1.2, FR-1.6).
func (s *Server) handleClientUpdate(w http.ResponseWriter, r *http.Request) {
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
	c := clientFromForm(f, id)

	if _, err := s.deps.Clients.Update(r.Context(), c); err != nil {
		var verr *apperr.ValidationError
		if errors.As(err, &verr) {
			s.render(w, r, http.StatusUnprocessableEntity, "client_form", clientFormView{
				Title: "Edit client", Action: fmt.Sprintf("/clients/%d", id), Client: c, Errors: verr.Fields,
			})
			return
		}
		s.handleError(w, r, err) // ErrNotFound -> 404, else 500
		return
	}
	s.setFlash(w, flashSuccess, "Client updated.")
	s.redirect(w, r, "/clients")
}

// handleClientArchive soft-deletes a client (FR-1.5).
func (s *Server) handleClientArchive(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.handleError(w, r, apperr.ErrNotFound)
		return
	}
	if err := s.deps.Clients.Archive(r.Context(), id); err != nil {
		s.handleError(w, r, err)
		return
	}
	s.setFlash(w, flashSuccess, "Client archived.")
	s.redirect(w, r, "/clients")
}

// clientFromForm builds a Client from submitted form fields. Validation (of the
// required name/email) happens in the store via client.Validate.
func clientFromForm(f *form, id int64) client.Client {
	return client.Client{
		ID:             id,
		Name:           f.String("name"),
		Email:          f.String("email"),
		Phone:          f.String("phone"),
		Company:        f.String("company"),
		BillingAddress: f.String("billing_address"),
	}
}

// idParam parses a numeric path parameter (e.g. {id}).
func idParam(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}
