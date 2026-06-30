// Package scheduler runs daily maintenance jobs in the background: marking
// past-due invoices overdue (FR-3.4) and sending automatic payment reminders
// (FR-4.5). Each job is driven off the consumer-side interfaces below so it can
// be tested with fakes.
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ArfaMujahid/invoice-generator/internal/email"
	"github.com/ArfaMujahid/invoice-generator/internal/invoice"
	"github.com/ArfaMujahid/invoice-generator/internal/settings"
)

// InvoiceJobs is the subset of invoice operations the scheduler needs.
type InvoiceJobs interface {
	MarkOverdue(ctx context.Context, asOf string) (int64, error)
	List(ctx context.Context, f invoice.Filter) ([]invoice.ListItem, error)
	RecordReminder(ctx context.Context, id int64) error
}

// SettingsReader reads the current settings (reminder cadence, business name).
type SettingsReader interface {
	Get(ctx context.Context) (settings.Settings, error)
}

// ClientReader resolves a client's email for reminder delivery.
type ClientReader interface {
	Email(ctx context.Context, id int64) (string, error)
}

// Mailer sends a reminder message.
type Mailer interface {
	Send(ctx context.Context, cfg settings.Settings, msg email.Message) error
}

// Deps are the scheduler's injected dependencies.
type Deps struct {
	Invoices InvoiceJobs
	Settings SettingsReader
	Clients  ClientReader
	Mailer   Mailer
	Logger   *slog.Logger
	// Interval between runs; defaults to 24h when zero.
	Interval time.Duration
}

// Scheduler runs the daily jobs until its context is cancelled.
type Scheduler struct {
	deps Deps
}

// New returns a Scheduler with the given dependencies.
func New(deps Deps) *Scheduler {
	if deps.Interval == 0 {
		deps.Interval = 24 * time.Hour
	}
	return &Scheduler{deps: deps}
}

// Run executes the jobs once immediately, then on each interval tick, returning
// when ctx is cancelled. This is the goroutine's single, guaranteed stop.
func (s *Scheduler) Run(ctx context.Context) {
	s.runOnce(ctx)
	ticker := time.NewTicker(s.deps.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

// runOnce runs every daily job, logging (not propagating) failures so one bad
// run never stops the loop.
func (s *Scheduler) runOnce(ctx context.Context) {
	now := time.Now()
	if n, err := s.deps.Invoices.MarkOverdue(ctx, now.Format("2006-01-02")); err != nil {
		s.deps.Logger.Error("mark overdue job failed", "err", err)
	} else if n > 0 {
		s.deps.Logger.Info("marked invoices overdue", "count", n)
	}
	if err := s.sendAutoReminders(ctx, now); err != nil {
		s.deps.Logger.Error("auto-reminder job failed", "err", err)
	}
}

// sendAutoReminders emails reminders for invoices that fall due (before) or are
// overdue on a configured cadence (after), when enabled in settings (FR-4.5).
func (s *Scheduler) sendAutoReminders(ctx context.Context, now time.Time) error {
	cfg, err := s.deps.Settings.Get(ctx)
	if err != nil {
		return fmt.Errorf("loading settings: %w", err)
	}
	if cfg.ReminderDaysBefore <= 0 && cfg.ReminderDaysAfter <= 0 {
		return nil // auto-reminders disabled
	}

	// Consider only issued, unpaid invoices.
	invoices, err := s.deps.Invoices.List(ctx, invoice.Filter{})
	if err != nil {
		return fmt.Errorf("listing invoices: %w", err)
	}
	for _, it := range invoices {
		due, derr := time.Parse("2006-01-02", it.DueDate)
		if derr != nil {
			continue
		}
		if !reminderDue(string(it.Status), due, now, cfg.ReminderDaysBefore, cfg.ReminderDaysAfter) {
			continue
		}
		s.sendReminder(ctx, cfg, it)
	}
	return nil
}

// sendReminder emails one reminder and records it, logging any failure.
func (s *Scheduler) sendReminder(ctx context.Context, cfg settings.Settings, it invoice.ListItem) {
	to, err := s.deps.Clients.Email(ctx, it.ClientID)
	if err != nil || to == "" {
		return
	}
	msg := email.Message{
		To:      to,
		Subject: fmt.Sprintf("Reminder: invoice %s", it.Number),
		Body: fmt.Sprintf("Dear customer,\n\nA reminder that invoice %s (%s %s) is due on %s. Outstanding balance: %s %s.\n\nThank you,\n%s",
			it.Number, it.Currency, formatMoney(it.Total), it.DueDate, it.Currency, formatMoney(it.Balance), businessName(cfg)),
	}
	if err := s.deps.Mailer.Send(ctx, cfg, msg); err != nil {
		s.deps.Logger.Warn("auto-reminder send failed", "invoice", it.Number, "err", err)
		return
	}
	if err := s.deps.Invoices.RecordReminder(ctx, it.ID); err != nil {
		s.deps.Logger.Warn("recording reminder failed", "invoice", it.Number, "err", err)
	}
	s.deps.Logger.Info("auto-reminder sent", "invoice", it.Number, "to", to)
}

// reminderDue reports whether a reminder should go out today for an invoice with
// the given status and due date: once "before" days ahead of the due date (while
// still only Sent), and every "after" days once overdue. Zero disables a side.
func reminderDue(status string, due, today time.Time, before, after int) bool {
	daysUntil := dayDiff(due, today) // >0 future, <0 past due
	if before > 0 && status == "sent" && daysUntil == before {
		return true
	}
	if after > 0 && daysUntil < 0 && (status == "sent" || status == "overdue") {
		overdueBy := -daysUntil
		return overdueBy%after == 0
	}
	return false
}

// dayDiff returns the whole-day difference a-b, ignoring clock time.
func dayDiff(a, b time.Time) int {
	a = time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.UTC)
	b = time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.UTC)
	return int(a.Sub(b).Hours() / 24)
}

// businessName returns the configured business name or a neutral default.
func businessName(cfg settings.Settings) string {
	if cfg.BusinessName != "" {
		return cfg.BusinessName
	}
	return "Accounts"
}

// formatMoney formats minor units as a decimal string.
func formatMoney(m invoice.Money) string {
	neg := m < 0
	if neg {
		m = -m
	}
	s := fmt.Sprintf("%d.%02d", int64(m)/100, int64(m)%100)
	if neg {
		return "-" + s
	}
	return s
}
