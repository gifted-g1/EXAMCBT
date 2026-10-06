Placeholder for versioned SQL migrations (e.g. via golang-migrate).
The application currently uses GORM AutoMigrate on startup — see
internal/db/db.go — for fast bootstrap and local development.
