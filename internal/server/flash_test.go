package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestFlashRoundTrip verifies a flash set on one response is read back exactly
// once: the follow-up request sees it, and a subsequent read does not.
func TestFlashRoundTrip(t *testing.T) {
	s := &Server{}

	// Set a flash and capture the cookie it writes.
	w1 := httptest.NewRecorder()
	s.setFlash(w1, flashSuccess, "Saved successfully")
	cookies := w1.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("setFlash wrote no cookie")
	}

	// Pop it on the next request: should return the same message.
	r2 := httptest.NewRequest("GET", "/", nil)
	for _, c := range cookies {
		r2.AddCookie(c)
	}
	w2 := httptest.NewRecorder()
	got := s.popFlash(w2, r2)
	if got == nil {
		t.Fatal("popFlash() = nil; want flash")
	}
	if got.Level != flashSuccess || got.Message != "Saved successfully" {
		t.Errorf("popFlash() = %+v; want {success, Saved successfully}", got)
	}

	// popFlash must have written an expiring cookie (MaxAge < 0).
	cleared := w2.Result().Cookies()
	if len(cleared) == 0 || cleared[0].MaxAge >= 0 {
		t.Errorf("popFlash did not clear the cookie: %+v", cleared)
	}
}

// TestPopFlashAbsent confirms popFlash returns nil when no flash cookie is set.
func TestPopFlashAbsent(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest("GET", "/", nil)
	if got := s.popFlash(httptest.NewRecorder(), r); got != nil {
		t.Errorf("popFlash() = %+v; want nil", got)
	}
}

// TestRedirect checks redirect issues a 303 with the right Location.
func TestRedirect(t *testing.T) {
	s := &Server{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/clients", nil)
	s.redirect(w, r, "/clients")
	if w.Code != http.StatusSeeOther {
		t.Errorf("status = %d; want %d", w.Code, http.StatusSeeOther)
	}
	if loc := w.Header().Get("Location"); loc != "/clients" {
		t.Errorf("Location = %q; want %q", loc, "/clients")
	}
}
