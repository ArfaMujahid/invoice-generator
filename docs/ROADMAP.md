# Implementation Roadmap

How we build the Invoice Generator & Tracker: **one commit per requirement**, in
the SRS suggested build order (§8.3). Each requirement is implemented end-to-end
(store → handler → template → test where it applies) before moving on.

Priorities from the SRS: **Must** (required for v1.0), **Should** (important),
**Nice** (optional). We complete all Must items first, then Should, then Nice.

Legend: ✅ done · 🟡 in progress · ⬜ todo

---

## Phase 0 — Foundation (architecture)

| # | Item | Req | Status |
|---|------|-----|--------|
| 0.1 | Project setup: Go server, SQLite schema, embedded templates, base layout, tooling | §8.3 step 1 | ✅ |
| 0.2 | Dashboard summary cards with live aggregates | FR-3.2 | ✅ |
| 0.3 | Panic-recovery guard + middleware chain | NFR-R3 | ✅ |
| 0.4 | Data-access foundation: `store.WithTx` transaction helper, form-decode + flash/redirect (PRG) helpers | NFR-6 | ⬜ |

## Phase 1 — Settings (Module 5) — *needed by PDF & email*

| # | Item | Req | Pri | Status |
|---|------|-----|-----|--------|
| 1.1 | Business profile (name, address, tax ID) + logo upload; shown on PDFs | FR-5.1 | Must | ⬜ |
| 1.2 | SMTP settings (host/port/user/pass) + "Test connection" | FR-5.2 | Must | ⬜ |
| 1.3 | Invoice-number format/prefix with auto-increment | FR-5.3 | Should | ⬜ |
| 1.4 | Default tax rate pre-filled on new invoices | FR-5.4 | Nice | ⬜ |

## Phase 2 — Client management (Module 1)

| # | Item | Req | Pri | Status |
|---|------|-----|-----|--------|
| 2.1 | Create client (name, email, phone, company, billing address) | FR-1.1 | Must | ⬜ |
| 2.2 | Validate email format + required fields before save | FR-1.6 | Must | ⬜ (logic done in `client.Validate`) |
| 2.3 | Edit/update client | FR-1.2 | Must | ⬜ |
| 2.4 | Clients list table with invoiced + outstanding totals | FR-1.3 | Must | ⬜ |
| 2.5 | Client detail: full invoice history | FR-1.4 | Should | ⬜ |
| 2.6 | Archive (soft-delete) client | FR-1.5 | Nice | ⬜ |

## Phase 3 — Invoice creation (Module 2)

| # | Item | Req | Pri | Status |
|---|------|-----|-----|--------|
| 3.1 | Create invoice: select client, auto number, issue/due dates | FR-2.1 | Must | ⬜ |
| 3.2 | Add/remove line items; per-row total (qty × price) | FR-2.2 | Must | ⬜ |
| 3.3 | Subtotal + tax + grand total, live recalculation | FR-2.3 | Must | ⬜ (money/total logic done) |
| 3.4 | Save invoice as Draft and re-edit | FR-2.7 | Must | ⬜ |
| 3.5 | Notes / payment-terms free-text field | FR-2.5 | Should | ⬜ |
| 3.6 | Per-invoice currency selection | FR-2.6 | Should | ⬜ |
| 3.7 | Duplicate an existing invoice | FR-2.8 | Nice | ⬜ |

## Phase 4 — PDF generation (Module 2)

| # | Item | Req | Pri | Status |
|---|------|-----|-----|--------|
| 4.1 | Branded PDF: logo, business + client details, line items, totals, payment instructions | FR-2.4 | Must | ⬜ |

## Phase 5 — Invoice list, dashboard & tracking (Module 3)

| # | Item | Req | Pri | Status |
|---|------|-----|-----|--------|
| 5.1 | Invoice status Draft/Sent/Paid/Overdue + manual change | FR-3.1 | Must | ⬜ |
| 5.2 | Invoice list: filter by status/client/date, search | FR-3.3 | Must | ⬜ |
| 5.3 | Record partial payments; show remaining balance | FR-3.5 | Should | ⬜ (balance logic done) |
| 5.4 | Daily job: mark unpaid past-due invoices Overdue | FR-3.4 | Should | ⬜ |
| 5.5 | Dashboard monthly invoiced-vs-collected chart | FR-3.6 | Nice | ⬜ |

## Phase 6 — Email & reminders (Module 4)

| # | Item | Req | Pri | Status |
|---|------|-----|-----|--------|
| 6.1 | Email invoice (PDF attached) via SMTP, one click | FR-4.1 | Must | ⬜ |
| 6.2 | Sending sets status to Sent | FR-4.2 | Must | ⬜ |
| 6.3 | Manual payment-reminder email (template) | FR-4.3 | Should | ⬜ |
| 6.4 | Record emailed-at + reminder count | FR-4.4 | Should | ⬜ |
| 6.5 | Auto-reminders N days before/after due date | FR-4.5 | Nice | ⬜ |

---

## Non-functional requirements (verified continuously, not separate commits)

NFR-1 usability · NFR-2 performance (<2s, thousands of invoices) · NFR-3 clear
PDF/email success-failure messages · NFR-4 SMTP creds never in frontend +
parameterized queries · NFR-5 single Linux binary + portable DB file · NFR-6
clear package layout + comments · NFR-7 single-file backup.

## Acceptance (SRS §8.2)

v1.0 is accepted when all **Must** items work: configure settings → add client →
create invoice → correct PDF → email it → track to Paid → accurate dashboard.
