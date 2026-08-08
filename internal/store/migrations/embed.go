package migrations

import "embed"

// Files contains ordered SQL migrations applied by the store package.
//
//go:embed *.sql
var Files embed.FS
