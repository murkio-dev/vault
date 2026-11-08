package migrations_embed

import "embed"

//go:embed sqlite/*.sql
var FS embed.FS
