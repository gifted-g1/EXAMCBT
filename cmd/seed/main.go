// Command seed loads the 8 mock staff and 12 mock students (idempotent).
//
//	go run ./cmd/seed               # seed the configured database
//	go run ./cmd/seed -demo-exam    # also add a DRAFT demo exam
//	go run ./cmd/seed -sql > seed/seed.sql   # regenerate the SQL seed file
package main

import (
	"flag"
	"fmt"
	"os"

	"gorm.io/gorm"

	"examshield/internal/config"
	"examshield/internal/store"
	"examshield/seed"
)

func main() {
	sqlOut := flag.Bool("sql", false, "print an idempotent MySQL seed script instead of seeding")
	demo := flag.Bool("demo-exam", false, "also create a DRAFT demo exam with 5 questions")
	cidr := flag.String("demo-cidr", "", "lan_cidr for the demo exam (default: first LAN_CIDRS entry or 192.168.0.0/16)")
	force := flag.Bool("force", false, "allow seeding when APP_ENV=production (mock passwords are public!)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *sqlOut {
		if err := seed.WriteSQL(os.Stdout, cfg.BcryptCost); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if cfg.IsProduction() && !*force {
		fmt.Fprintln(os.Stderr, "refusing to seed mock staff with published passwords in production; pass -force if you really mean it")
		os.Exit(2)
	}

	var db *gorm.DB
	if cfg.Mode == config.ModeStandalone {
		db, err = store.OpenSQLite(cfg.SQLitePath)
	} else {
		db, err = store.OpenMySQL(cfg.DBDSN, 2, 1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "database:", err)
		os.Exit(1)
	}
	if *cidr == "" && len(cfg.LANCIDRs) > 0 {
		*cidr = cfg.LANCIDRs[0].String()
	}
	if err := seed.Run(db, seed.Options{BcryptCost: cfg.BcryptCost, DemoExam: *demo, DemoCIDR: *cidr}); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
	fmt.Printf("seeded %d staff and %d students\n", len(seed.Staff), len(seed.Students))
}
