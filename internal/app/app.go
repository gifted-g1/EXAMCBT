// Package app wires configuration, infrastructure and services into a
// runnable API process.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/gorm"

	"examshield/internal/ai"
	"examshield/internal/auth"
	"examshield/internal/config"
	"examshield/internal/handlers"
	"examshield/internal/middleware"
	"examshield/internal/netutil"
	"examshield/internal/realtime"
	"examshield/internal/repositories"
	"examshield/internal/services"
	"examshield/internal/store"
	"examshield/internal/worker"
	"examshield/seed"
)

type App struct {
	Cfg      *config.Config
	Log      *slog.Logger
	DB       *gorm.DB
	Redis    *store.Redis
	Blobs    store.Blobs
	Engine   ai.Engine
	Tokens   *auth.Tokens
	Hub      *realtime.Hub
	Gate     *middleware.LANGate
	Handlers *handlers.Handlers
	LANNets  []*net.IPNet

	findings  *services.FindingsConsumer
	scheduler *services.Scheduler
	worker    *worker.Worker
}

// Build opens every dependency. Call Close when done.
func Build(ctx context.Context, cfg *config.Config, log *slog.Logger) (*App, error) {
	a := &App{Cfg: cfg, Log: log}
	ok := false
	defer func() {
		if !ok {
			a.Close()
		}
	}()

	if len(cfg.JWTSecret) < 32 {
		if cfg.Mode == config.ModeStandalone {
			// Keep the key next to the database so a server restart or power
			// cut does not log every student and proctor out mid-exam.
			secret, err := loadOrCreateSecret(filepath.Join(filepath.Dir(cfg.SQLitePath), ".jwt_secret"))
			if err != nil {
				return nil, fmt.Errorf("session key: %w", err)
			}
			cfg.JWTSecret = secret
			log.Info("JWT_SECRET not set: using the persistent key stored beside the database")
		} else {
			cfg.JWTSecret = []byte(auth.RandomHex(32))
			log.Warn("JWT_SECRET not set: using a random per-process secret (all sessions end on restart)")
		}
	}

	// LAN networks: explicit config wins; otherwise every private interface.
	a.LANNets = cfg.LANCIDRs
	if len(a.LANNets) == 0 {
		nets, err := netutil.DetectLANNets()
		if err != nil {
			if cfg.IsProduction() && !cfg.OpenAccess() {
				return nil, err
			}
			log.Warn("no LAN detected; only loopback clients will be admitted", "err", err)
		}
		a.LANNets = nets
	}
	if cfg.OpenAccess() {
		log.Warn("ACCESS_MODE=open: the exam-hall network lock is DISABLED - anyone who can reach this server can open exams (PIN, identity and proctoring checks still apply). Use ACCESS_MODE=lan in exam halls.",
			"trusted_proxies", cfg.TrustedProxies, "public_url", cfg.PublicURL)
	}
	log.Info("LAN gate", "allow", cidrStrings(a.LANNets), "deny", cidrStrings(cfg.LANDenyCIDRs),
		"trusted_proxies", cfg.TrustedProxies)

	var err error
	switch cfg.Mode {
	case config.ModeStandalone:
		a.DB, err = store.OpenSQLite(cfg.SQLitePath)
	default:
		if cfg.AutoMigrate {
			if err := store.MigrateMySQL(cfg.DBDSN, "up", log); err != nil {
				return nil, fmt.Errorf("migrate: %w", err)
			}
		}
		a.DB, err = store.OpenMySQL(cfg.DBDSN, cfg.DBMaxOpen, cfg.DBMaxIdle)
	}
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}
	if cfg.SeedOnStart && cfg.IsProduction() {
		return nil, errors.New("SEED_ON_START would install mock staff with published passwords; it is refused when APP_ENV=production")
	}
	if cfg.SeedOnStart {
		cidr := ""
		if len(a.LANNets) > 0 {
			cidr = a.LANNets[0].String()
		}
		if err := seed.Run(a.DB, seed.Options{BcryptCost: cfg.BcryptCost, DemoExam: true, DemoCIDR: cidr}); err != nil {
			return nil, fmt.Errorf("seed: %w", err)
		}
	}

	if a.Redis, err = store.OpenRedis(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB); err != nil {
		return nil, err
	}
	if cfg.MinIOEndpoint != "" {
		a.Blobs, err = store.OpenMinIO(ctx, cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOBucket, cfg.MinIOUseSSL)
	} else {
		a.Blobs, err = store.OpenDisk(cfg.BlobDir)
	}
	if err != nil {
		return nil, fmt.Errorf("blob store: %w", err)
	}

	need := ai.Need{Faces: true, Embed: true}
	if cfg.InProcessAI {
		need = ai.NeedAll()
	}
	a.Engine, err = ai.Load(ai.Options{Backend: cfg.AIBackend, ModelDir: cfg.ModelDir, ONNXLib: cfg.ONNXLibPath,
		UseCUDA: cfg.ONNXUseCUDA, Log: log}, need)
	if err != nil {
		return nil, fmt.Errorf("ai engine: %w", err)
	}

	repo := repositories.New(a.DB)
	a.Tokens = auth.NewTokens(cfg.JWTSecret, cfg.AccessTTL)
	a.Hub = realtime.NewHub(log)
	matcher := services.NewFaceMatcher(a.Engine, cfg.FaceMatchThreshold, cfg.FaceMatchEnforce)
	recorder := services.NewRecorder(repo, a.Redis, a.Hub, log)
	exams := services.NewExamService(repo, a.Redis, a.Hub, cfg, a.LANNets, log)
	students := services.NewStudentService(repo, a.Redis, a.Blobs, a.Tokens, matcher, recorder, a.Hub, cfg, log)
	exams.OnClosed = students.AutoSubmitExam
	a.scheduler = services.NewScheduler(repo, a.Redis, a.Blobs, exams, cfg.SubmitGrace, cfg.RetentionDays, log)

	host, _ := os.Hostname()
	consumer := store.ConsumerName("api", host, os.Getpid())
	if v := os.Getenv("CONSUMER_NAME"); v != "" {
		consumer = v
	}
	a.findings = services.NewFindingsConsumer(repo, a.Redis, recorder, a.Hub, consumer, log)
	if cfg.InProcessAI {
		a.worker = worker.New(a.Redis, a.Blobs, a.Engine, matcher, consumer+"-ai", cfg.AIWorkers, log)
	}

	a.Gate = middleware.NewLANGate(cfg, a.LANNets, exams, a.Redis, log)
	a.Handlers = &handlers.Handlers{
		Cfg: cfg, Repo: repo, Redis: a.Redis, Blobs: a.Blobs, Tokens: a.Tokens,
		Auth:  services.NewAuthService(repo, a.Redis, a.Tokens, cfg.RefreshTTL, cfg.BcryptCost),
		Exams: exams, Students: students, Roster: services.NewRosterService(repo, a.Blobs, log),
		Proctor: services.NewProctorService(repo, recorder, a.Hub), Scheduler: a.scheduler,
		Matcher: matcher, Engine: a.Engine, Hub: a.Hub, Log: log,
	}
	ok = true
	return a, nil
}

// RunBackground starts the hub, findings consumer, scheduler and (in
// standalone mode) the AI worker. Each returns when ctx is cancelled; the
// returned channel closes once all have stopped.
func (a *App) RunBackground(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	n := 3
	if a.worker != nil {
		n++
	}
	finished := make(chan struct{}, n)
	go func() { a.Hub.Run(ctx); finished <- struct{}{} }()
	go func() { a.scheduler.Run(ctx); finished <- struct{}{} }()
	go func() {
		if err := a.findings.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			a.Log.Error("findings consumer stopped", "err", err)
		}
		finished <- struct{}{}
	}()
	if a.worker != nil {
		go func() {
			if err := a.worker.Run(ctx); err != nil {
				a.Log.Error("in-process AI worker stopped", "err", err)
			}
			finished <- struct{}{}
		}()
	}
	go func() {
		for i := 0; i < n; i++ {
			<-finished
		}
		close(done)
	}()
	return done
}

func (a *App) Close() {
	if a.Engine != nil {
		_ = a.Engine.Close()
	}
	if a.Redis != nil {
		_ = a.Redis.Close()
	}
	if a.DB != nil {
		if sqlDB, err := a.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
}

func cidrStrings(nets []*net.IPNet) []string {
	out := make([]string, len(nets))
	for i, n := range nets {
		out[i] = n.String()
	}
	return out
}

// loadOrCreateSecret reads a 64-hex-char key from path, creating it (0600)
// on first use. Used only in standalone mode when JWT_SECRET is unset.
func loadOrCreateSecret(path string) ([]byte, error) {
	if b, err := os.ReadFile(path); err == nil {
		if s := strings.TrimSpace(string(b)); len(s) >= 32 {
			return []byte(s), nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	s := auth.RandomHex(32)
	if err := os.WriteFile(path, []byte(s+"\n"), 0o600); err != nil {
		return nil, err
	}
	return []byte(s), nil
}
