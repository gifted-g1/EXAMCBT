// Command api runs the Exam Shield server: REST API, WebSocket hub, LAN gate
// and the embedded React UI, on one port.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"examshield/internal/app"
	"examshield/internal/config"
	"examshield/internal/netutil"
)

func main() {
	cfg, err := config.Load()
	log := newLogger(os.Getenv("LOG_LEVEL"))
	if err != nil {
		log.Error("invalid configuration", "err", err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := app.Build(ctx, cfg, log)
	if err != nil {
		log.Error("startup failed", "err", err)
		os.Exit(1)
	}
	defer a.Close()

	bg := a.RunBackground(ctx)
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           app.NewRouter(ctx, a),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}

	errc := make(chan error, 2)
	var redirect *http.Server
	if cfg.TLS() {
		srv.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		go func() { errc <- srv.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile) }()
		if cfg.HTTPRedirectAddr != "" {
			redirect = &http.Server{Addr: cfg.HTTPRedirectAddr, ReadHeaderTimeout: 5 * time.Second, Handler: httpsRedirect(cfg)}
			go func() { errc <- redirect.ListenAndServe() }()
		}
	} else {
		go func() { errc <- srv.ListenAndServe() }()
	}
	logStartup(log, cfg)

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Error("server failed", "err", err)
		}
		stop()
	case <-ctx.Done():
	}
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	if redirect != nil {
		_ = redirect.Shutdown(shutdownCtx)
	}
	select {
	case <-bg:
	case <-shutdownCtx.Done():
		log.Warn("background tasks did not stop in time")
	}
}

func logStartup(log *slog.Logger, cfg *config.Config) {
	ip, err := netutil.AdvertiseIP(cfg.LANAdvertiseIP)
	if err != nil {
		log.Warn("could not determine LAN address", "err", err)
		ip = "127.0.0.1"
	}
	port := cfg.PublicPort
	if port == 0 {
		if _, p, err := net.SplitHostPort(cfg.HTTPAddr); err == nil {
			port, _ = strconv.Atoi(p)
		}
	}
	scheme := "http"
	if cfg.TLS() || cfg.PublicScheme == "https" {
		scheme = "https"
	}
	url := netutil.HostURL(scheme, ip, port)
	log.Info("Exam Shield ready", "mode", cfg.Mode, "listen", cfg.HTTPAddr, "students", url, "staff", url+"/staff")
}

func newLogger(level string) *slog.Logger {
	var l slog.Level
	switch strings.ToLower(level) {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l}))
}

// httpsRedirect sends students who type http://<ip> to https://<ip>. Only the
// host part of the request is reused; the port comes from configuration.
func httpsRedirect(cfg *config.Config) http.Handler {
	_, tlsPort, _ := net.SplitHostPort(cfg.HTTPAddr)
	if cfg.PublicPort != 0 {
		tlsPort = strconv.Itoa(cfg.PublicPort)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		// Only redirect to IP literals: students type an IP, and reflecting an
		// arbitrary Host header would be an open redirect.
		if net.ParseIP(strings.Trim(host, "[]")) == nil {
			http.Error(w, "use the exam IP address", http.StatusBadRequest)
			return
		}
		target := "https://" + host
		if tlsPort != "" && tlsPort != "443" {
			target = "https://" + net.JoinHostPort(strings.Trim(host, "[]"), tlsPort)
		}
		http.Redirect(w, r, target+r.URL.RequestURI(), http.StatusPermanentRedirect)
	})
}
