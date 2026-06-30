// Package config defines the application's runtime configuration and how it is
// loaded from flags and environment variables. Configuration has sensible
// defaults and is validated at startup so misconfiguration fails fast and loudly
// (coding-standards §10).
package config

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration for the application. Every field has a
// safe default; nothing about paths, ports, or limits is hardcoded elsewhere.
type Config struct {
	// Addr is the host:port the HTTP server listens on.
	Addr string
	// DatabasePath is the path to the single SQLite database file.
	DatabasePath string
	// Dev enables development conveniences (text logs, verbose errors).
	Dev bool
	// LogLevel is the minimum level emitted by the structured logger.
	LogLevel slog.Level
	// ReadTimeout bounds how long the server waits to read a request.
	ReadTimeout time.Duration
	// WriteTimeout bounds how long a handler may take to write a response.
	WriteTimeout time.Duration
	// IdleTimeout bounds how long an idle keep-alive connection is kept open.
	IdleTimeout time.Duration
	// MaxUploadBytes caps the size of an uploaded logo image (NFR-S2).
	MaxUploadBytes int64
}

// Load reads configuration from command-line flags, falling back to environment
// variables and then to built-in defaults, and validates the result. It returns
// an error if any value is invalid so startup aborts before serving traffic.
func Load() (Config, error) {
	cfg := defaults()

	fs := flag.NewFlagSet("invoice", flag.ContinueOnError)
	fs.StringVar(&cfg.Addr, "addr", cfg.Addr, "host:port to listen on")
	fs.StringVar(&cfg.DatabasePath, "db", cfg.DatabasePath, "path to the SQLite database file")
	fs.BoolVar(&cfg.Dev, "dev", cfg.Dev, "enable development mode (text logs)")
	if err := fs.Parse(os.Args[1:]); err != nil {
		return Config{}, fmt.Errorf("parsing flags: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// defaults returns a Config populated with the built-in default values, after
// applying any overrides present in the environment.
func defaults() Config {
	return Config{
		Addr:           env("INVOICE_ADDR", ":8080"),
		DatabasePath:   env("INVOICE_DB", "invoice.db"),
		Dev:            envBool("INVOICE_DEV", false),
		LogLevel:       slog.LevelInfo,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxUploadBytes: 5 << 20, // 5 MiB is plenty for a logo image.
	}
}

// Validate reports the first configuration value that is unusable. Keeping the
// checks here means callers can trust a returned Config completely.
func (c Config) Validate() error {
	if strings.TrimSpace(c.Addr) == "" {
		return fmt.Errorf("config: addr must not be empty")
	}
	if strings.TrimSpace(c.DatabasePath) == "" {
		return fmt.Errorf("config: db path must not be empty")
	}
	if c.MaxUploadBytes <= 0 {
		return fmt.Errorf("config: max upload bytes must be positive, got %d", c.MaxUploadBytes)
	}
	return nil
}

// env returns the environment variable named key, or def if it is unset/empty.
func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// envBool parses a boolean environment variable, returning def when unset or
// unparseable so a typo never silently flips behaviour without notice.
func envBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
