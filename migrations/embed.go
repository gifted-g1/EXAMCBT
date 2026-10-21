// Package migrations embeds the golang-migrate SQL files so the binary can
// migrate MySQL without the files being present on disk.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
