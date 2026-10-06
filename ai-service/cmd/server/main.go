// Exam Shield AI Service — Go implementation.
//
// Built on the standard library net/http rather than a web framework,
// matching the platform's minimalist-stack philosophy. This service
// exposes an internal-only HTTP surface (/verify-face, /analyze-frame,
// /detect-face, /detect-multiple-faces) that only the Go backend
// calls; the React frontend never talks to it directly.
package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"examshield-ai/internal/behavior"
	"examshield-ai/internal/facedetect"
	"examshield-ai/internal/faceverify"
	"examshield-ai/internal/httpserver"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cascadePath := getEnv("CASCADE_PATH", "data/facefinder")
	port := getEnv("AI_SERVICE_PORT", "9000")
	host := getEnv("AI_SERVICE_HOST", "0.0.0.0")

	detector, err := facedetect.New(cascadePath)
	if err != nil {
		logger.Error("failed to load face detection cascade", "path", cascadePath, "error", err)
		os.Exit(1)
	}

	verifier := faceverify.New()
	analyzer := behavior.New(detector)
	server := httpserver.New(detector, verifier, analyzer, logger)

	srv := &http.Server{
		Addr:         host + ":" + port,
		Handler:      server.Routes(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logger.Info("examshield-ai (Go) listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
