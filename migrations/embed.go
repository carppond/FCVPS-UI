// Package migrations exposes the embedded SQL migration files via FS.
//
// Storage layer reads these on hub startup; see internal/storage/migrate.go.
package migrations

import "embed"

// FS contains every *.sql file in this directory at compile time.
//
//go:embed *.sql
var FS embed.FS
