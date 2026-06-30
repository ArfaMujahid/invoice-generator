package server

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ArfaMujahid/invoice-generator/internal/apperr"
)

// form provides typed, validated access to submitted form values. Handlers read
// fields through it and accumulate every validation problem in one pass (rather
// than failing on the first), mirroring the domain ValidationError so a template
// can show a message next to each field.
type form struct {
	values url.Values
	// Errors collects per-field validation failures as fields are read.
	Errors *apperr.ValidationError
}

// parseForm parses an application/x-www-form-urlencoded request body and returns
// a form ready for field access.
func parseForm(r *http.Request) (*form, error) {
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("parsing form: %w", err)
	}
	return &form{values: r.PostForm, Errors: apperr.NewValidationError()}, nil
}

// parseMultipartForm parses a multipart/form-data body (used by forms with file
// uploads), keeping up to maxMemory bytes in memory before spilling to temp
// files, and returns a form over its non-file values.
func parseMultipartForm(r *http.Request, maxMemory int64) (*form, error) {
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		return nil, fmt.Errorf("parsing multipart form: %w", err)
	}
	return &form{values: r.PostForm, Errors: apperr.NewValidationError()}, nil
}

// String returns the trimmed value of field, or "" if absent.
func (f *form) String(field string) string {
	return strings.TrimSpace(f.values.Get(field))
}

// Strings returns all submitted values for a repeated field (e.g. line-item
// inputs named "description[]"), untrimmed and in submission order.
func (f *form) Strings(field string) []string {
	return f.values[field]
}

// Required returns the trimmed value of field, recording a validation error
// (labelled for display) when it is empty.
func (f *form) Required(field, label string) string {
	v := f.String(field)
	if v == "" {
		f.Errors.Add(field, label+" is required")
	}
	return v
}

// Int returns field parsed as an int, or def when the field is empty. A present
// but unparseable value records a validation error and returns def.
func (f *form) Int(field string, def int) int {
	v := f.String(field)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		f.Errors.Add(field, "must be a whole number")
		return def
	}
	return n
}

// Float returns field parsed as a float64, or def when the field is empty. A
// present but unparseable value records a validation error and returns def.
func (f *form) Float(field string, def float64) float64 {
	v := f.String(field)
	if v == "" {
		return def
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		f.Errors.Add(field, "must be a number")
		return def
	}
	return n
}

// Bool reports whether a checkbox-style field was submitted as a truthy value.
func (f *form) Bool(field string) bool {
	switch strings.ToLower(f.String(field)) {
	case "on", "true", "1", "yes":
		return true
	default:
		return false
	}
}

// Valid reports whether no validation errors have been recorded.
func (f *form) Valid() bool {
	return !f.Errors.HasErrors()
}
