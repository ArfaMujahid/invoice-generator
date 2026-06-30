// Package server is the HTTP layer: it wires routes to handlers, renders the
// server-side HTML, and serves embedded static assets. It contains no business
// logic — every operation is delegated to an injected domain service, accessed
// through the small consumer-side interfaces defined here (coding-standards §5,
// §7).
package server

import (
	"context"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/ArfaMujahid/invoice-generator/internal/client"
	"github.com/ArfaMujahid/invoice-generator/internal/config"
	"github.com/ArfaMujahid/invoice-generator/internal/email"
	"github.com/ArfaMujahid/invoice-generator/internal/invoice"
	"github.com/ArfaMujahid/invoice-generator/internal/settings"
	"github.com/ArfaMujahid/invoice-generator/web"
)

// Clients is the subset of client operations the HTTP layer needs (SRS Module 1).
type Clients interface {
	List(ctx context.Context, includeArchived bool) ([]client.Client, error)
	Get(ctx context.Context, id int64) (client.Client, error)
	Create(ctx context.Context, c client.Client) (client.Client, error)
}

// Invoices is the subset of invoice operations the HTTP layer needs (SRS
// Modules 2 & 3).
type Invoices interface {
	Summary(ctx context.Context) (invoice.Summary, error)
	List(ctx context.Context, f invoice.Filter) ([]invoice.Invoice, error)
	Get(ctx context.Context, id int64) (invoice.Invoice, error)
}

// SettingsStore is the subset of settings operations the HTTP layer needs (SRS
// Module 5).
type SettingsStore interface {
	Get(ctx context.Context) (settings.Settings, error)
	SaveProfile(ctx context.Context, cfg settings.Settings) error
	SaveSMTP(ctx context.Context, cfg settings.Settings) error
}

// PDFRenderer renders an invoice to PDF bytes (FR-2.4).
type PDFRenderer interface {
	Render(ctx context.Context, inv invoice.Invoice, cfg settings.Settings) ([]byte, error)
}

// Mailer sends invoices and reminders, and verifies SMTP settings (SRS Module 4).
type Mailer interface {
	Send(ctx context.Context, cfg settings.Settings, msg email.Message) error
	TestConnection(ctx context.Context, cfg settings.Settings) error
}

// Deps are the dependencies injected into the server from the composition root.
type Deps struct {
	Config   config.Config
	Logger   *slog.Logger
	Clients  Clients
	Invoices Invoices
	Settings SettingsStore
	PDF      PDFRenderer
	Mailer   Mailer
}

// Server holds the wired dependencies and the compiled templates. Construct it
// with New.
type Server struct {
	deps      Deps
	templates map[string]*template.Template
}

// New builds a Server, compiling templates up front so a template error stops
// startup rather than surfacing on the first request.
func New(deps Deps) *Server {
	tmpls, err := parseTemplates()
	if err != nil {
		// Templates are embedded and constant; a failure here is a build/programmer
		// error, so failing fast at startup is correct.
		panic("server: " + err.Error())
	}
	return &Server{deps: deps, templates: tmpls}
}

// routes builds the request multiplexer. Routes use Go 1.22+ method-aware
// patterns so each handler declares the verb it accepts.
func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// Static assets served straight from the embedded filesystem.
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(web.Static())))

	// Uploaded assets (business logo) served from disk. http.Dir blocks path
	// traversal above the uploads directory.
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(s.deps.Config.UploadsDir))))

	// Liveness probe for deployment platforms.
	mux.HandleFunc("GET /healthz", s.handleHealth)

	// Screens (SRS §5). Implemented: dashboard, settings. The rest are wired to
	// their domain services, which currently return apperr.ErrNotImplemented
	// (HTTP 501) until each module is built.
	mux.HandleFunc("GET /", s.handleDashboard)
	mux.HandleFunc("GET /clients", s.handleClientsList)
	mux.HandleFunc("GET /invoices", s.handleInvoicesList)
	mux.HandleFunc("GET /settings", s.handleSettings)
	mux.HandleFunc("POST /settings", s.handleSettingsSave)
	mux.HandleFunc("POST /settings/smtp", s.handleSettingsSaveSMTP)
	mux.HandleFunc("POST /settings/smtp/test", s.handleSettingsTestSMTP)

	// Logging is outermost so it records the final status even when recoverer
	// converts a handler panic into a 500.
	return chain(mux, s.withLogging, s.recoverer)
}

// Run starts the HTTP server and blocks until ctx is cancelled, then shuts down
// gracefully within a bounded timeout. The server has explicit timeouts so a
// slow or stuck client cannot tie up a connection indefinitely
// (coding-standards §6).
func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:         s.deps.Config.Addr,
		Handler:      s.routes(),
		ReadTimeout:  s.deps.Config.ReadTimeout,
		WriteTimeout: s.deps.Config.WriteTimeout,
		IdleTimeout:  s.deps.Config.IdleTimeout,
	}

	// errCh carries a fatal ListenAndServe error from the serving goroutine so
	// Run can return it. Buffered so the goroutine never blocks on exit.
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		// Give in-flight requests a bounded window to finish before forcing close.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return ctx.Err()
	}
}
