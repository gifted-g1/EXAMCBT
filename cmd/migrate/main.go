// Command migrate applies the embedded MySQL migrations.
//
//	go run ./cmd/migrate up
//	go run ./cmd/migrate down
package main

import (
	"fmt"
	"log/slog"
	"os"

	"examshield/internal/config"
	"examshield/internal/store"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	dir := "up"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if cfg.DBDSN == "" {
		fmt.Fprintln(os.Stderr, "DB_DSN is required (standalone/SQLite mode migrates automatically)")
		os.Exit(2)
	}
	if err := store.MigrateMySQL(cfg.DBDSN, dir, log); err != nil {
		log.Error("migration failed", "err", err)
		os.Exit(1)
	}
}
