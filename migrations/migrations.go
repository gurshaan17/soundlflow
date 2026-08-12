package migrations

import "embed"

// FS embeds all .sql migration files so the API binary can apply them without
// a separate golang-migrate CLI install.
//
//go:embed *.sql
var FS embed.FS
