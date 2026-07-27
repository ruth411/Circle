package migrations

import "embed"

// FS embeds project SQL migrations so the binary can migrate itself on startup.
//
//go:embed *.sql
var FS embed.FS
