// Package apperr defines the small set of cross-cutting error values and types
// shared by the domain packages, so callers can branch on errors with
// errors.Is / errors.As regardless of which layer produced them
// (coding-standards §3).
package apperr

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNotFound is returned when a requested record does not exist. Repositories
// wrap it (with %w) so handlers can map it to an HTTP 404 via errors.Is.
var ErrNotFound = errors.New("not found")

// ErrNotImplemented marks a scaffolded code path that has not been built yet.
// Handlers map it to HTTP 501 so the running skeleton reports honestly which
// features remain to be implemented.
var ErrNotImplemented = errors.New("not implemented")

// ValidationError reports that user-supplied input failed a domain rule. It
// carries per-field messages so a form handler can show them next to the field.
type ValidationError struct {
	// Fields maps a field name to the reason it is invalid.
	Fields map[string]string
}

// Error implements the error interface with a stable, composable message.
func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Fields))
	for field, msg := range e.Fields {
		parts = append(parts, fmt.Sprintf("%s: %s", field, msg))
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

// NewValidationError builds an empty ValidationError ready to collect failures.
func NewValidationError() *ValidationError {
	return &ValidationError{Fields: make(map[string]string)}
}

// Add records that field is invalid for the given reason and returns the
// receiver so checks can be chained.
func (e *ValidationError) Add(field, msg string) *ValidationError {
	e.Fields[field] = msg
	return e
}

// HasErrors reports whether any field failed validation.
func (e *ValidationError) HasErrors() bool {
	return len(e.Fields) > 0
}
