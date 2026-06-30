-- Schema for the Invoice Generator & Tracker.
--
-- Design notes:
--  * Money is stored in integer minor units (e.g. cents) to avoid binary
--    floating-point rounding errors on financial amounts. Quantities and tax
--    rates, which are genuinely fractional, are stored as REAL.
--  * Foreign keys are declared; the store enables `PRAGMA foreign_keys = ON`
--    per connection because SQLite does not enforce them by default.
--  * All statements are idempotent (IF NOT EXISTS) so this file doubles as the
--    startup migration, applied on every boot.

-- Clients the business issues invoices to (SRS §4.1).
CREATE TABLE IF NOT EXISTS clients (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT    NOT NULL,
    email           TEXT    NOT NULL,
    phone           TEXT    NOT NULL DEFAULT '',
    company         TEXT    NOT NULL DEFAULT '',
    billing_address TEXT    NOT NULL DEFAULT '',
    archived        INTEGER NOT NULL DEFAULT 0,  -- boolean: 0/1
    created_at      TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- Invoices issued to clients (SRS §4.2). status is one of:
-- 'draft', 'sent', 'paid', 'overdue'.
CREATE TABLE IF NOT EXISTS invoices (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    invoice_number  TEXT    NOT NULL UNIQUE,
    client_id       INTEGER NOT NULL REFERENCES clients(id),
    issue_date      TEXT    NOT NULL,            -- ISO-8601 date
    due_date        TEXT    NOT NULL,            -- ISO-8601 date
    status          TEXT    NOT NULL DEFAULT 'draft',
    currency        TEXT    NOT NULL DEFAULT 'USD',
    tax_rate        REAL    NOT NULL DEFAULT 0,  -- percentage, e.g. 15.0
    notes           TEXT    NOT NULL DEFAULT '',
    sent_at         TEXT,                        -- nullable datetime
    reminders_sent  INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_invoices_client_id ON invoices(client_id);
CREATE INDEX IF NOT EXISTS idx_invoices_status    ON invoices(status);
CREATE INDEX IF NOT EXISTS idx_invoices_due_date  ON invoices(due_date);

-- Billable rows on an invoice (SRS §4.3). unit_price is in minor units.
CREATE TABLE IF NOT EXISTS line_items (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    invoice_id  INTEGER NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    description TEXT    NOT NULL,
    quantity    REAL    NOT NULL,
    unit_price  INTEGER NOT NULL,                -- minor units
    position    INTEGER NOT NULL DEFAULT 0        -- display order
);

CREATE INDEX IF NOT EXISTS idx_line_items_invoice_id ON line_items(invoice_id);

-- Payments recorded against an invoice (SRS §4.4). Supports partial payments.
CREATE TABLE IF NOT EXISTS payments (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    invoice_id  INTEGER NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    amount      INTEGER NOT NULL,                -- minor units
    paid_on     TEXT    NOT NULL                 -- ISO-8601 date
);

CREATE INDEX IF NOT EXISTS idx_payments_invoice_id ON payments(invoice_id);

-- Single-row business/application settings (SRS §4.5). The CHECK constraint
-- pins this table to exactly one row (id = 1).
CREATE TABLE IF NOT EXISTS settings (
    id               INTEGER PRIMARY KEY CHECK (id = 1),
    business_name    TEXT    NOT NULL DEFAULT '',
    business_address TEXT    NOT NULL DEFAULT '',
    tax_id           TEXT    NOT NULL DEFAULT '',
    logo_path        TEXT    NOT NULL DEFAULT '',
    smtp_host        TEXT    NOT NULL DEFAULT '',
    smtp_port        INTEGER NOT NULL DEFAULT 587,
    smtp_username    TEXT    NOT NULL DEFAULT '',
    smtp_password    TEXT    NOT NULL DEFAULT '',
    invoice_prefix   TEXT    NOT NULL DEFAULT 'INV',
    invoice_format   TEXT    NOT NULL DEFAULT 'INV-{YYYY}-{SEQ}',
    default_tax_rate REAL    NOT NULL DEFAULT 0,
    reminder_days_before INTEGER NOT NULL DEFAULT 0,
    reminder_days_after  INTEGER NOT NULL DEFAULT 0
);

-- Seed the single settings row if it does not yet exist.
INSERT OR IGNORE INTO settings (id) VALUES (1);
