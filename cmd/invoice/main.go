// Command invoice is the entry point and composition root for the Invoice
// Generator & Tracker. It does thin orchestration only: load configuration,
// open the data store, wire dependencies, and run the HTTP server until the
// process is asked to stop. All business logic lives in internal packages.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/ArfaMujahid/invoice-generator/internal/client"
	"github.com/ArfaMujahid/invoice-generator/internal/config"
	"github.com/ArfaMujahid/invoice-generator/internal/email"
	"github.com/ArfaMujahid/invoice-generator/internal/invoice"
	"github.com/ArfaMujahid/invoice-generator/internal/pdf"
	"github.com/ArfaMujahid/invoice-generator/internal/scheduler"
	"github.com/ArfaMujahid/invoice-generator/internal/server"
	"github.com/ArfaMujahid/invoice-generator/internal/settings"
	"github.com/ArfaMujahid/invoice-generator/internal/store"
)

// main wires the application together and exits non-zero on any startup or
// runtime failure so process supervisors can observe and restart it.
func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

// run performs the actual startup sequence. It is split out from main so the
// single top-level error is logged in exactly one place (per coding-standards
// §3: handle each error once, at the boundary).
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := newLogger(cfg)
	slog.SetDefault(logger)

	// ctx is cancelled on SIGINT/SIGTERM, giving every goroutine and the HTTP
	// server a single, guaranteed stop signal for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer st.Close()

	// Wire the domain stores and services. Each depends only on the shared
	// *store.Store (a thin wrapper over *sql.DB) and is injected into the
	// server, which owns no business logic of its own.
	clients := client.NewStore(st)
	invoices := invoice.NewStore(st)
	cfgStore := settings.NewStore(st)
	pdfGen := pdf.NewGenerator(cfg.UploadsDir)
	mailer := email.NewSMTPSender()

	// Background scheduler: daily overdue marking (FR-3.4) and auto-reminders
	// (FR-4.5). It stops when ctx is cancelled, alongside the server.
	sched := scheduler.New(scheduler.Deps{
		Invoices: invoices,
		Settings: cfgStore,
		Clients:  clients,
		Mailer:   mailer,
		Logger:   logger,
	})
	go sched.Run(ctx)

	srv := server.New(server.Deps{
		Config:   cfg,
		Logger:   logger,
		Clients:  clients,
		Invoices: invoices,
		Settings: cfgStore,
		PDF:      pdfGen,
		Mailer:   mailer,
	})

	logger.Info("starting server", "addr", cfg.Addr, "db", cfg.DatabasePath)
	if err := srv.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	logger.Info("shutdown complete")
	return nil
}

// newLogger builds the structured logger, using JSON in production-like runs and
// a friendlier text handler in development (coding-standards §9).
func newLogger(cfg config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	if cfg.Dev {
		return slog.New(slog.NewTextHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}
