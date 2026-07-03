// Command docz-api is the entry point for the docz-api service.
//
// It stays thin: parse flags, load configuration, configure the slog default
// handler, wire dependencies, and run an HTTP server with graceful shutdown.
// All real behavior lives under internal/.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/donaldgifford/docz-api/internal/config"
)

// Build metadata, injected via -ldflags at release time (see justfile).
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// Server timeouts. readHeaderTimeout bounds slow-header (Slowloris) clients;
// shutdownTimeout bounds in-flight request draining on shutdown.
const (
	readHeaderTimeout = 10 * time.Second
	shutdownTimeout   = 15 * time.Second
)

func main() {
	if err := run(); err != nil {
		// run() configures slog once config loads; a failure before that still
		// reaches the bootstrap default handler.
		slog.Error("docz-api exited with error", "err", err)
		os.Exit(1)
	}
}

// run parses flags, loads config, configures logging, and serves until a
// termination signal arrives.
func run() error {
	showVersion := flag.Bool("version", false, "print version information and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("docz-api %s (commit %s, built %s)\n", version, commit, date)
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger, err := newLogger(cfg.Log)
	if err != nil {
		return err
	}
	slog.SetDefault(logger)

	slog.Info("starting docz-api",
		"version", version,
		"commit", commit,
		"addr", cfg.HTTP.Addr,
		"auth_providers", cfg.Auth.Providers,
	)

	return serve(cfg.HTTP.Addr, newRouter())
}

// newLogger builds the slog handler selected by the log config. Config
// validation already constrains level/format, so the error paths are a
// defensive backstop.
func newLogger(cfg config.LogConfig) (*slog.Logger, error) {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		return nil, fmt.Errorf("invalid log level %q", cfg.Level)
	}

	opts := &slog.HandlerOptions{Level: level}
	switch cfg.Format {
	case "json":
		return slog.New(slog.NewJSONHandler(os.Stdout, opts)), nil
	case "text":
		return slog.New(slog.NewTextHandler(os.Stdout, opts)), nil
	default:
		return nil, fmt.Errorf("invalid log format %q", cfg.Format)
	}
}

// newRouter builds the HTTP handler. At this phase it serves only the liveness
// probe; read endpoints, readiness, and the auth surface land in later phases.
func newRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Get("/healthz", handleHealthz)
	return r
}

// handleHealthz is the liveness probe: it reports that the process is up,
// independent of downstream dependencies (readiness covers those).
func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
		slog.Debug("healthz response write failed", "err", err)
	}
}

// serve runs srv until an interrupt/terminate signal, then drains in-flight
// requests within shutdownTimeout.
func serve(addr string, handler http.Handler) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("http server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		slog.Info("shutdown signal received, draining")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	slog.Info("shutdown complete")
	return nil
}
