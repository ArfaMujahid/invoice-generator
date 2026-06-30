package server

import (
	"encoding/base64"
	"net/http"
	"strings"
)

// flash is a one-time notification shown to the user after a redirect, the
// "message" half of the post-redirect-get (PRG) pattern used by form handlers.
type flash struct {
	// Level is the visual category: flashSuccess, flashError, or flashInfo.
	Level   string
	Message string
}

// flashCookie is the name of the one-shot flash cookie.
const flashCookie = "flash"

// Flash levels, used as both the cookie payload and a CSS modifier class.
const (
	flashSuccess = "success"
	flashError   = "error"
	flashInfo    = "info"
)

// setFlash stores a one-time message to display after the next redirect. It must
// be called before the response is written. The value is base64-encoded so it is
// safe to carry in a cookie.
func (s *Server) setFlash(w http.ResponseWriter, level, message string) {
	raw := level + "\n" + message
	http.SetCookie(w, &http.Cookie{
		Name:     flashCookie,
		Value:    base64.URLEncoding.EncodeToString([]byte(raw)),
		Path:     "/",
		MaxAge:   60, // long enough to survive the redirect, short enough to expire
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// popFlash returns and clears the flash message, if any. It must be called
// before the response status is written so the clearing cookie is sent, which is
// why render invokes it up front.
func (s *Server) popFlash(w http.ResponseWriter, r *http.Request) *flash {
	c, err := r.Cookie(flashCookie)
	if err != nil || c.Value == "" {
		return nil
	}

	// Expire the cookie so the message is shown exactly once.
	http.SetCookie(w, &http.Cookie{
		Name:     flashCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	raw, err := base64.URLEncoding.DecodeString(c.Value)
	if err != nil {
		return nil // tampered/garbled cookie: ignore rather than error
	}
	level, message, ok := strings.Cut(string(raw), "\n")
	if !ok {
		return nil
	}
	return &flash{Level: level, Message: message}
}

// redirect issues a 303 See Other to path — the correct status for the GET that
// follows a successful POST (post-redirect-get), so a browser refresh does not
// resubmit the form.
func (s *Server) redirect(w http.ResponseWriter, r *http.Request, path string) {
	http.Redirect(w, r, path, http.StatusSeeOther)
}
