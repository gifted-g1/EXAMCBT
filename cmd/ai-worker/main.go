// Command ai-worker runs AI inference jobs from the events.frames stream.
// Run several replicas to scale; Redis consumer groups split the work.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"examshield/internal/ai"
	"examshield/internal/config"
	"examshield/internal/services"
	"examshield/internal/store"
	"examshield/internal/worker"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("ai-worker failed", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.RedisAddr == "" {
		return fmt.Errorf("REDIS_ADDR is required (standalone mode runs the worker inside the API instead)")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rdb, err := store.OpenRedis(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		return err
	}
	defer rdb.Close()

	var blobs store.Blobs
	if cfg.MinIOEndpoint != "" {
		blobs, err = store.OpenMinIO(ctx, cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOBucket, cfg.MinIOUseSSL)
	} else {
		blobs, err = store.OpenDisk(cfg.BlobDir)
	}
	if err != nil {
		return err
	}

	engine, err := ai.Load(ai.Options{Backend: cfg.AIBackend, ModelDir: cfg.ModelDir, ONNXLib: cfg.ONNXLibPath,
		UseCUDA: cfg.ONNXUseCUDA, Log: log}, ai.NeedAll())
	if err != nil {
		return err
	}
	defer engine.Close()

	host, _ := os.Hostname()
	consumer := os.Getenv("CONSUMER_NAME")
	if consumer == "" {
		consumer = store.ConsumerName("ai", host, os.Getpid())
	}
	matcher := services.NewFaceMatcher(engine, cfg.FaceMatchThreshold, cfg.FaceMatchEnforce)
	log.Info("ai-worker started", "consumer", consumer, "pool", cfg.AIWorkers, "backends", engine.Backends())
	return worker.New(rdb, blobs, engine, matcher, consumer, cfg.AIWorkers, log).Run(ctx)
}
