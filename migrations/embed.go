package migrations

import "embed"

//go:embed *.up.sql
var files embed.FS
