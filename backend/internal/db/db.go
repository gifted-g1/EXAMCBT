package db

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"examshield/internal/models"
)

func Connect(dsn string) (*gorm.DB, error) {
	return gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
}

// AutoMigrate creates/updates all tables. Production deployments should
// prefer versioned SQL migrations (see /migrations) driven by a tool
// such as golang-migrate; AutoMigrate is provided for fast bootstrap
// and local development.
func AutoMigrate(gdb *gorm.DB) error {
	return gdb.AutoMigrate(
		&models.Organization{},
		&models.User{},
		&models.Student{},
		&models.Lecturer{},
		&models.Administrator{},
		&models.RefreshToken{},
		&models.Exam{},
		&models.Question{},
		&models.Option{},
		&models.ExamAttempt{},
		&models.Answer{},
		&models.FaceVerification{},
		&models.MonitoringSession{},
		&models.AIEvent{},
		&models.SecurityEvent{},
		&models.AuditLog{},
	)
}
