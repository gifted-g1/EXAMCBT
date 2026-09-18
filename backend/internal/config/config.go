package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration, loaded exclusively from
// environment variables. No secrets or environment-specific values are
// hard-coded anywhere else in the application.
type Config struct {
	Env                string
	HTTPPort           string
	DatabaseURL        string
	JWTSecret          string
	JWTAccessTTL       time.Duration
	JWTRefreshTTL      time.Duration
	AIServiceURL       string
	CORSAllowedOrigins []string
	ExamMode           string // "online" | "lan"
	LANPortRangeStart  int
	LANPortRangeEnd    int
	LogLevel           string
}

func Load() *Config {
	return &Config{
		Env:                getEnv("APP_ENV", "development"),
		HTTPPort:           getEnv("HTTP_PORT", "8080"),
		DatabaseURL:        getEnv("DATABASE_URL", "host=localhost user=examshield password=examshield dbname=examshield port=5432 sslmode=disable"),
		JWTSecret:          getEnv("JWT_SECRET", "change-me-in-production"),
		JWTAccessTTL:       getDurationMinutes("JWT_ACCESS_TTL_MINUTES", 15),
		JWTRefreshTTL:      getDurationMinutes("JWT_REFRESH_TTL_MINUTES", 60*24*7),
		AIServiceURL:       getEnv("AI_SERVICE_URL", "http://localhost:9000"),
		CORSAllowedOrigins: []string{getEnv("CORS_ALLOWED_ORIGIN", "http://localhost:5173")},
		ExamMode:           getEnv("EXAM_MODE", "online"),
		LANPortRangeStart:  getEnvInt("LAN_PORT_RANGE_START", 8181),
		LANPortRangeEnd:    getEnvInt("LAN_PORT_RANGE_END", 8199),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getDurationMinutes(key string, fallbackMinutes int) time.Duration {
	minutes := getEnvInt(key, fallbackMinutes)
	return time.Duration(minutes) * time.Minute
}
