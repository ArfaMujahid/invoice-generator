# Invoice Generator & Tracker

A single-binary web application for freelancers and small businesses to create
professional invoices, email them to clients, and track payment status. Backend
in **Go**, server-rendered **HTML/CSS** frontend, **SQLite** for storage — no
external database server, no frontend framework.

> **Status: project skeleton (v0).** The structure, data model, tooling, and a
> working dashboard are in place. Each feature module is scaffolded with
> documented interfaces and returns `501 Not Implemented` until built out. See
> [Implementation status](#implementation-status).

This repository tracks the build described in
[`docs/SRS_Invoice_Generator_Tracker.docx`](docs/SRS_Invoice_Generator_Tracker.docx)
and follows [`docs/CODING-STANDARDS.md`](docs/CODING-STANDARDS.md).

---

## Quick start

Requires **Go 1.26+**.

```bash
# Run in development mode (text logs) on http://localhost:8080
make run ARGS="-dev"

# or directly:
go run ./cmd/invoice -dev
```

Then open <http://localhost:8080> — the dashboard renders with live (zero, on a
fresh database) summary totals. A `invoice.db` SQLite file is created on first
run and the schema is applied automatically.

### Build a binary

```bash
make build        # -> bin/invoice
./bin/invoice -dev
```

The result is a single self-contained binary (templates and CSS are embedded; the
SQLite driver is pure Go, so no cgo/C toolchain is needed and it cross-compiles
to Linux trivially).

---

## Configuration

Configuration comes from flags, falling back to environment variables, then
built-in defaults. It is validated at startup (`config.Config.Validate`).

| Flag    | Env            | Default      | Description                       |
|---------|----------------|--------------|-----------------------------------|
| `-addr` | `INVOICE_ADDR` | `:8080`      | host:port to listen on            |
| `-db`   | `INVOICE_DB`   | `invoice.db` | path to the SQLite database file  |
| `-dev`  | `INVOICE_DEV`  | `false`      | development mode (human-readable text logs) |

SMTP credentials and business branding are **runtime settings** stored in the
database (SRS Module 5), not configuration flags, and are never exposed to the
frontend.

---

## Project structure

```
cmd/invoice/           Composition root (main): config → wire deps → run. No logic.
internal/
  apperr/              Shared error values/types (ErrNotFound, ValidationError, …).
  config/              Flag/env configuration with startup validation.
  store/               SQLite connection + startup schema migration.
  client/              Module 1 — client management (model, validation, repo).
  invoice/             Modules 2 & 3 — invoice model, money/totals, repo, dashboard summary.
  settings/            Module 5 — business profile, SMTP, numbering, reminders.
  pdf/                 Module 2 — branded invoice PDF generation.
  email/               Module 4 — SMTP delivery and reminders.
  server/              HTTP layer: routing, handlers, template rendering. No business logic.
web/                   Embedded templates (HTML) and static assets (CSS).
migrations/            Embedded SQL schema (schema.sql) applied at startup.
docs/                  SRS and coding standards (source of truth).
```

Architecture follows the coding standards: logic in `internal/` organized by
domain, wiring in `main`, dependencies injected via constructors, and the HTTP
layer depending on small **consumer-side interfaces** (defined in
`internal/server`) so collaborators can be faked in tests.

---

## Implementation status

Mapped to the SRS suggested build order (§8.3).

| Area | Status |
|------|--------|
| Project setup: Go server, SQLite schema, base HTML layout | ✅ Done |
| Dashboard summary cards (FR-3.2) | ✅ Done (live aggregate query) |
| Settings + business profile (Module 5) | 🟡 Scaffolded |
| Client management CRUD (Module 1) | 🟡 Scaffolded (validation done) |
| Invoice editor + live totals (Module 2) | 🟡 Scaffolded (money/total logic done) |
| PDF generation (FR-2.4) | 🟡 Scaffolded |
| Invoice list, filtering, status tracking (Module 3) | 🟡 Scaffolded |
| Email sending + reminders (Module 4) | 🟡 Scaffolded |
| Overdue detection / partial payments / charts | 🟡 Scaffolded |

🟡 = interfaces, models, and wiring exist; methods return
`apperr.ErrNotImplemented` (HTTP 501) with `TODO(arfa)` markers showing exactly
what to build.

---

## Development

```bash
make check     # pre-commit gate: gofmt-clean + go vet + go test -race
make test      # run tests
make race      # run tests under the race detector
make fmt       # gofmt -w .
make lint      # golangci-lint (see below)
make tidy      # go mod tidy
make vuln      # govulncheck
```

Optional tools used by `make lint` / `make vuln`:

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install golang.org/x/vuln/cmd/govulncheck@latest
```

---

## Dependencies

The dependency tree is intentionally lean (coding-standards §13).

| Dependency | Why |
|------------|-----|
| [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) | Pure-Go SQLite driver — no cgo, so the app builds as a single static binary and cross-compiles to Linux without a C toolchain (SRS §2.5, NFR-5). |

Everything else (HTTP, templating, structured logging, embedding) uses the
standard library. A small PDF library will be added when Module 2's PDF
generation is implemented (`internal/pdf`).

---

## License

TBD by the project owner.
