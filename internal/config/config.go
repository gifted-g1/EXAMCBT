// Package config loads Exam Shield's runtime configuration from the
// environment (optionally seeded from a local .env file).
package config

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// Mode selects which infrastructure backs the API.
//
//   - standalone:  SQLite + in-process Redis + filesystem blobs + in-process
//     AI worker. One binary, zero services — for a single exam hall laptop.
//   - distributed: MySQL + Redis + MinIO + separate ai-worker processes.
type Mode string

const (
	ModeStandalone  Mode = "standalone"
	ModeDistributed Mode = "distributed"
)

type Config struct {
	Env  string
	Mode Mode

	HTTPAddr         string
	TLSCertFile      string
	TLSKeyFile       string
	HTTPRedirectAddr string // e.g. ":80" → 308 to HTTPS (only with TLS)

	// Database
	DBDSN        string // MySQL DSN (distributed)
	SQLitePath   string // SQLite file (standalone)
	DBMaxOpen    int
	DBMaxIdle    int
	AutoMigrate  bool
	SeedOnStart  bool
	MigrationDir string

	// Redis
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// Object storage
	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string
	MinIOBucket    string
	MinIOUseSSL    bool
	BlobDir        string

	// Auth
	JWTSecret    []byte
	AccessTTL    time.Duration
	RefreshTTL   time.Duration
	CookieSecure bool
	BcryptCost   int

	// Network lock
	LANCIDRs           []*net.IPNet
	LANDenyCIDRs       []*net.IPNet
	LANAdvertiseIP     string
	PublicPort         int
	PublicScheme       string // "https" when a TLS proxy fronts the server
	TrustedProxies     []string
	AllowLoopbackStaff bool
	AllowLoopbackExam  bool

	// Sessions / exam behaviour
	SessionLockTTL     time.Duration
	SubmitGrace        time.Duration
	FaceMatchThreshold float64
	FaceMatchEnforce   string // auto | always | never
	FrameMinInterval   time.Duration

	// AI
	AIWorkers   int
	AIBackend   string // auto | onnx | go
	ModelDir    string
	ONNXLibPath string
	ONNXUseCUDA bool
	InProcessAI bool

	// Compliance
	RetentionDays  int
	ConsentVersion string
}

// IsProduction reports whether strict production checks apply.
func (c *Config) IsProduction() bool { return c.Env == "production" }

// TLS reports whether the server terminates HTTPS itself.
func (c *Config) TLS() bool { return c.TLSCertFile != "" }

// Load reads configuration from the environment. A .env file in the working
// directory is read first; real environment variables always win.
func Load() (*Config, error) {
	loadDotEnv(".env")

	c := &Config{
		Env:              str("APP_ENV", "development"),
		Mode:             Mode(str("MODE", string(ModeStandalone))),
		HTTPAddr:         str("HTTP_ADDR", ":8080"),
		TLSCertFile:      str("TLS_CERT_FILE", ""),
		TLSKeyFile:       str("TLS_KEY_FILE", ""),
		HTTPRedirectAddr: str("HTTP_REDIRECT_ADDR", ""),

		DBDSN:        str("DB_DSN", ""),
		SQLitePath:   str("SQLITE_PATH", "data/examshield.db"),
		DBMaxOpen:    integer("DB_MAX_OPEN_CONNS", 50),
		DBMaxIdle:    integer("DB_MAX_IDLE_CONNS", 10),
		AutoMigrate:  boolean("DB_AUTO_MIGRATE", false),
		SeedOnStart:  boolean("SEED_ON_START", false),
		MigrationDir: str("MIGRATIONS_DIR", "migrations"),

		RedisAddr:     str("REDIS_ADDR", ""),
		RedisPassword: str("REDIS_PASSWORD", ""),
		RedisDB:       integer("REDIS_DB", 0),

		MinIOEndpoint:  str("MINIO_ENDPOINT", ""),
		MinIOAccessKey: str("MINIO_ACCESS_KEY", ""),
		MinIOSecretKey: str("MINIO_SECRET_KEY", ""),
		MinIOBucket:    str("MINIO_BUCKET", "exam-shield"),
		MinIOUseSSL:    boolean("MINIO_USE_SSL", false),
		BlobDir:        str("BLOB_DIR", "data/blobs"),

		JWTSecret:    []byte(str("JWT_SECRET", "")),
		AccessTTL:    duration("JWT_ACCESS_TTL", 15*time.Minute),
		RefreshTTL:   duration("JWT_REFRESH_TTL", 7*24*time.Hour),
		CookieSecure: boolean("COOKIE_SECURE", false),
		BcryptCost:   integer("BCRYPT_COST", 12),

		LANAdvertiseIP:     str("LAN_ADVERTISE_IP", ""),
		PublicPort:         integer("PUBLIC_PORT", 0),
		PublicScheme:       str("PUBLIC_SCHEME", ""),
		TrustedProxies:     list("TRUSTED_PROXIES"),
		AllowLoopbackStaff: boolean("ALLOW_LOOPBACK_STAFF", true),
		AllowLoopbackExam:  boolean("ALLOW_LOOPBACK_EXAM", false),

		SessionLockTTL:     duration("SESSION_LOCK_TTL", 2*time.Minute),
		SubmitGrace:        duration("SUBMIT_GRACE", 2*time.Minute),
		FaceMatchThreshold: float("FACE_MATCH_THRESHOLD", 0.75),
		FaceMatchEnforce:   str("FACE_MATCH_ENFORCE", "auto"),
		FrameMinInterval:   duration("FRAME_MIN_INTERVAL", 2*time.Second),

		AIWorkers:   integer("AI_WORKERS", 4),
		AIBackend:   str("AI_BACKEND", "auto"),
		ModelDir:    str("MODEL_DIR", "models"),
		ONNXLibPath: str("ONNXRUNTIME_LIB", ""),
		ONNXUseCUDA: boolean("ONNX_USE_CUDA", false),

		RetentionDays:  integer("RETENTION_DAYS", 30),
		ConsentVersion: str("CONSENT_VERSION", "2026-10"),
	}
	c.InProcessAI = boolean("AI_IN_PROCESS", c.Mode == ModeStandalone)
	if c.TLS() && str("COOKIE_SECURE", "") == "" {
		c.CookieSecure = true
	}

	var err error
	if c.LANCIDRs, err = parseCIDRs(list("LAN_CIDRS")); err != nil {
		return nil, fmt.Errorf("LAN_CIDRS: %w", err)
	}
	if c.LANDenyCIDRs, err = parseCIDRs(list("LAN_DENY_CIDRS")); err != nil {
		return nil, fmt.Errorf("LAN_DENY_CIDRS: %w", err)
	}
	for _, p := range c.TrustedProxies {
		if _, _, err := net.ParseCIDR(p); err != nil && net.ParseIP(p) == nil {
			return nil, fmt.Errorf("TRUSTED_PROXIES: %q is neither an IP nor a CIDR", p)
		}
	}
	return c, c.validate()
}

func (c *Config) validate() error {
	var errs []error
	switch c.Mode {
	case ModeStandalone:
	case ModeDistributed:
		if c.DBDSN == "" {
			errs = append(errs, errors.New("DB_DSN is required in distributed mode"))
		}
		if c.RedisAddr == "" {
			errs = append(errs, errors.New("REDIS_ADDR is required in distributed mode"))
		}
		if c.MinIOEndpoint == "" {
			errs = append(errs, errors.New("MINIO_ENDPOINT is required in distributed mode"))
		}
	default:
		errs = append(errs, fmt.Errorf("MODE must be standalone or distributed, got %q", c.Mode))
	}

	if len(c.JWTSecret) < 32 {
		if c.IsProduction() {
			errs = append(errs, errors.New("JWT_SECRET must be at least 32 bytes in production"))
		}
	}
	if weakSecret(string(c.JWTSecret)) && c.IsProduction() {
		errs = append(errs, errors.New("JWT_SECRET is a placeholder value; generate one with `openssl rand -hex 32`"))
	}
	if c.BcryptCost < 12 && c.IsProduction() {
		errs = append(errs, errors.New("BCRYPT_COST must be >= 12 in production"))
	}
	if (c.TLSCertFile == "") != (c.TLSKeyFile == "") {
		errs = append(errs, errors.New("set both TLS_CERT_FILE and TLS_KEY_FILE, or neither"))
	}
	switch c.FaceMatchEnforce {
	case "auto", "always", "never":
	default:
		errs = append(errs, errors.New("FACE_MATCH_ENFORCE must be auto, always or never"))
	}
	if c.FaceMatchThreshold <= 0 || c.FaceMatchThreshold > 1 {
		errs = append(errs, errors.New("FACE_MATCH_THRESHOLD must be in (0,1]"))
	}
	if c.AccessTTL <= 0 || c.RefreshTTL <= c.AccessTTL {
		errs = append(errs, errors.New("JWT_REFRESH_TTL must be longer than JWT_ACCESS_TTL"))
	}
	if c.AIWorkers < 1 {
		c.AIWorkers = 1
	}
	return errors.Join(errs...)
}

func weakSecret(s string) bool {
	l := strings.ToLower(s)
	return l == "" || strings.Contains(l, "change-me") || strings.Contains(l, "changeme") || l == "secret"
}

// ParseCIDR parses a CIDR, also accepting a bare IP (treated as /32 or /128).
func ParseCIDR(s string) (*net.IPNet, error) {
	s = strings.TrimSpace(s)
	if !strings.Contains(s, "/") {
		ip := net.ParseIP(s)
		if ip == nil {
			return nil, fmt.Errorf("invalid CIDR %q", s)
		}
		if v4 := ip.To4(); v4 != nil {
			return &net.IPNet{IP: v4, Mask: net.CIDRMask(32, 32)}, nil
		}
		return &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)}, nil
	}
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR %q: %w", s, err)
	}
	return n, nil
}

func parseCIDRs(items []string) ([]*net.IPNet, error) {
	out := make([]*net.IPNet, 0, len(items))
	for _, it := range items {
		n, err := ParseCIDR(it)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func str(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

func integer(key string, def int) int {
	if v, err := strconv.Atoi(str(key, "")); err == nil {
		return v
	}
	return def
}

func float(key string, def float64) float64 {
	if v, err := strconv.ParseFloat(str(key, ""), 64); err == nil {
		return v
	}
	return def
}

func boolean(key string, def bool) bool {
	if v, err := strconv.ParseBool(str(key, "")); err == nil {
		return v
	}
	return def
}

func duration(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(str(key, "")); err == nil {
		return v
	}
	return def
}

func list(key string) []string {
	raw := str(key, "")
	if raw == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// loadDotEnv sets variables from a KEY=VALUE file without overriding values
// already present in the process environment.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			_ = os.Setenv(k, v)
		}
	}
}
