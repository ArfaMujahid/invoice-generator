// Package migrations embeds the SQL schema that the store applies at startup.
// Keeping the schema as a checked-in .sql file (rather than string literals)
// makes it reviewable on its own and reusable by tooling.
package migrations

import _ "embed"

// Schema is the full DDL applied idempotently on every startup. Statements use
// "IF NOT EXISTS" so re-running them on an existing database is a no-op.
//
//go:embed schema.sql
var Schema string
