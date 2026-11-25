// Package migrations embeds the SQL schema files into the binary so the
// server can bring the database up to date on startup with no extra tools.
package migrations

import "embed"

// Files holds every *.sql migration, applied in file-name order.
//
//go:embed *.sql
var Files embed.FS
