package server

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/ArfaMujahid/invoice-generator/internal/invoice"
	"github.com/ArfaMujahid/invoice-generator/web"
)

// pages lists the content templates that are each compiled together with the
// shared layout. Add a page's base name here when you add its template file.
var pages = []string{
	"dashboard", "message", "settings",
	"clients", "client_form", "client_detail",
	"invoices", "invoice_form", "invoice_view",
}

// parseTemplates compiles every page in pages with the shared layout and the
// view helper functions, returning a name→template map. It is called once at
// startup; a parse failure there is a programmer error that should stop boot.
func parseTemplates() (map[string]*template.Template, error) {
	funcs := template.FuncMap{
		"money":   money,
		"dateISO": dateISO,
	}
	out := make(map[string]*template.Template, len(pages))
	for _, name := range pages {
		t, err := template.New(name).Funcs(funcs).ParseFS(
			web.Templates(),
			"templates/layout.html",
			"templates/"+name+".html",
		)
		if err != nil {
			return nil, fmt.Errorf("parsing template %q: %w", name, err)
		}
		out[name] = t
	}
	return out, nil
}

// templateData is the envelope passed to every page render: a one-shot flash
// message (if any), plus the page-specific view data. Templates read the page
// fields via .Data and the notification via .Flash.
type templateData struct {
	Flash *flash
	Data  any
}

// render writes the named page to w, executing the shared layout. It pops any
// pending flash (which sets a clearing cookie, so it must run before the status
// is written) and buffers the output so a template error becomes a clean 500
// instead of a half-written response.
func (s *Server) render(w http.ResponseWriter, r *http.Request, status int, name string, data any) {
	t, ok := s.templates[name]
	if !ok {
		s.serverError(w, fmt.Errorf("render: unknown template %q", name))
		return
	}

	td := templateData{Flash: s.popFlash(w, r), Data: data}

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", td); err != nil {
		s.serverError(w, fmt.Errorf("executing template %q: %w", name, err))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, err := buf.WriteTo(w); err != nil {
		// The client likely went away mid-write; log once and move on.
		s.deps.Logger.Warn("writing response body", "err", err)
	}
}

// dateISO formats a time as YYYY-MM-DD for date inputs and display; a zero time
// renders as empty so a new invoice's unset fields show blank.
func dateISO(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

// money formats a minor-unit amount as a plain decimal string (e.g. 123456 →
// "1234.56"). Currency symbols are applied in the template from the invoice's
// currency, so this stays purely numeric.
func money(m invoice.Money) string {
	neg := m < 0
	if neg {
		m = -m
	}
	major := int64(m) / 100
	minor := int64(m) % 100
	if neg {
		return fmt.Sprintf("-%d.%02d", major, minor)
	}
	return fmt.Sprintf("%d.%02d", major, minor)
}
