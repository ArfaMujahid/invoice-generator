package server

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestForm builds a form from a urlencoded body, exercising parseForm.
func newTestForm(t *testing.T, body string) *form {
	t.Helper()
	r := httptest.NewRequest("POST", "/", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	f, err := parseForm(r)
	if err != nil {
		t.Fatalf("parseForm() error: %v", err)
	}
	return f
}

// TestFormAccessors checks typed access, trimming, defaults, and that required
// and unparseable fields accumulate validation errors instead of failing fast.
func TestFormAccessors(t *testing.T) {
	t.Run("typed values and trimming", func(t *testing.T) {
		f := newTestForm(t, "name=%20Acme%20&qty=3&price=9.99&active=on")
		if got := f.String("name"); got != "Acme" {
			t.Errorf("String(name) = %q; want %q", got, "Acme")
		}
		if got := f.Int("qty", 0); got != 3 {
			t.Errorf("Int(qty) = %d; want 3", got)
		}
		if got := f.Float("price", 0); got != 9.99 {
			t.Errorf("Float(price) = %v; want 9.99", got)
		}
		if !f.Bool("active") {
			t.Errorf("Bool(active) = false; want true")
		}
		if !f.Valid() {
			t.Errorf("Valid() = false; want true, errors: %v", f.Errors.Fields)
		}
	})

	t.Run("defaults for empty fields", func(t *testing.T) {
		f := newTestForm(t, "")
		if got := f.Int("qty", 7); got != 7 {
			t.Errorf("Int(qty) = %d; want default 7", got)
		}
		if got := f.Float("price", 1.5); got != 1.5 {
			t.Errorf("Float(price) = %v; want default 1.5", got)
		}
		if f.Bool("active") {
			t.Errorf("Bool(active) = true; want false")
		}
	})

	t.Run("accumulates validation errors", func(t *testing.T) {
		f := newTestForm(t, "qty=abc&price=xyz")
		f.Required("name", "Name")
		f.Int("qty", 0)
		f.Float("price", 0)
		if f.Valid() {
			t.Fatalf("Valid() = true; want false")
		}
		for _, field := range []string{"name", "qty", "price"} {
			if _, ok := f.Errors.Fields[field]; !ok {
				t.Errorf("missing error for %q; got %v", field, f.Errors.Fields)
			}
		}
	})
}
