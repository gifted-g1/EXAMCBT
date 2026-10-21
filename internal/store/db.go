package store

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/golang-migrate/migrate/v4"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"examshield/internal/models"
	"examshield/migrations"
)

// OpenMySQL opens a pooled GORM connection. parseTime=true and loc=UTC are
// added when the DSN omits them; keep loc=UTC if you set it yourself.
func OpenMySQL(dsn string, maxOpen, maxIdle int) (*gorm.DB, error) {
	dsn = withParam(dsn, "parseTime", "true")
	dsn = withParam(dsn, "loc", "UTC")
	dsn = withParam(dsn, "charset", "utf8mb4")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger:  logger.Default.LogMode(logger.Warn),
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	// Keep the pool well under MySQL's max_connections (151 by default, 300 in
	// our compose file) even with several API replicas.
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	return db, nil
}

// OpenSQLite opens the standalone-mode database and AutoMigrates it.
func OpenSQLite(path string) (*gorm.DB, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return nil, err
		}
	}
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)"
	if path == ":memory:" {
		dsn = "file::memory:?_pragma=foreign_keys(1)"
	}
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:  logger.Default.LogMode(logger.Warn),
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	// SQLite has a single writer; one connection avoids SQLITE_BUSY on
	// read->write lock upgrades and is plenty for one exam hall.
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(models.All()...); err != nil {
		return nil, fmt.Errorf("sqlite automigrate: %w", err)
	}
	return db, nil
}

// MigrateMySQL applies the embedded golang-migrate migrations ("up" or "down").
// It uses its own short-lived connection with multiStatements=true; the
// application pool never enables multi-statements (stacked-query SQLi).
func MigrateMySQL(dsn, direction string, log *slog.Logger) error {
	dsn = withParam(withParam(dsn, "multiStatements", "true"), "parseTime", "true")
	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return err
	}
	drv, err := migratemysql.WithInstance(sqlDB, &migratemysql.Config{})
	if err != nil {
		return err
	}
	m, err := migrate.NewWithInstance("iofs", src, "mysql", drv)
	if err != nil {
		return err
	}
	switch direction {
	case "up":
		err = m.Up()
	case "down":
		err = m.Down()
	default:
		return fmt.Errorf("unknown migration direction %q", direction)
	}
	if errors.Is(err, migrate.ErrNoChange) {
		log.Info("migrations: no change")
		return nil
	}
	if err != nil {
		return err
	}
	v, dirty, _ := m.Version()
	log.Info("migrations applied", "version", v, "dirty", dirty)
	return nil
}

func withParam(dsn, key, val string) string {
	if strings.Contains(dsn, key+"=") {
		return dsn
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + key + "=" + val
}
