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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/donaldgifford/docz-api/internal/authorize"
	"github.com/donaldgifford/docz-api/internal/config"
	"github.com/donaldgifford/docz-api/internal/githubapp"
	"github.com/donaldgifford/docz-api/internal/httpapi"
	"github.com/donaldgifford/docz-api/internal/ingest"
	"github.com/donaldgifford/docz-api/internal/search"
	"github.com/donaldgifford/docz-api/internal/store"
)

// Build metadata, injected via -ldflags at release time (see justfile).
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// Server timeouts. readHeaderTimeout bounds slow-header (Slowloris) clients;
// shutdownTimeout bounds in-flight request draining on shutdown; readyzTimeout
// bounds the dependency check behind the readiness probe.
const (
	readHeaderTimeout = 10 * time.Second
	shutdownTimeout   = 15 * time.Second
	readyzTimeout     = 2 * time.Second
)

// readyChecker reports whether the service's downstream dependencies are
// reachable. *store.Store satisfies it via Ping; the readiness probe depends on
// this narrow interface rather than the concrete store.
type readyChecker interface {
	Ping(ctx context.Context) error
}

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
	migrateOnly := flag.Bool("migrate", false, "apply database migrations and exit")
	onboardSpec := flag.String("onboard", "",
		"seed a repo and run one ingest, then exit: owner/name@installation_id")
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

	// Apply pending migrations before serving so the schema is always current.
	// `-migrate` runs them and exits — a CI/ops pre-deploy step.
	if err := store.Migrate(context.Background(), cfg.Store.DatabaseURL); err != nil {
		return fmt.Errorf("applying migrations: %w", err)
	}
	if *migrateOnly {
		slog.Info("migrations applied; exiting")
		return nil
	}

	// Runtime connection pool (separate from the migration connection). Owned
	// here for the process lifetime; closed on shutdown.
	pool, err := store.NewPool(context.Background(), cfg.Store.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connecting to postgres: %w", err)
	}
	defer pool.Close()
	st := store.NewStore(pool)

	// Search client: ensure the documents index + settings exist before any
	// ingest writes to it or the search endpoint reads from it.
	searchClient := search.New(cfg.Meili.Host, cfg.Meili.APIKey.Reveal())
	if err := searchClient.EnsureIndex(context.Background()); err != nil {
		return fmt.Errorf("ensuring search index: %w", err)
	}

	// `-onboard` hand-seeds one repo and runs a synchronous ingest, then exits —
	// the Phase 2 manual trigger (webhooks take over in Phase 5).
	if *onboardSpec != "" {
		return runOnboard(context.Background(), st, searchClient, &cfg, *onboardSpec)
	}

	slog.Info("starting docz-api",
		"version", version,
		"commit", commit,
		"addr", cfg.HTTP.Addr,
		"auth_providers", cfg.Auth.Providers,
	)

	// Probes plus the /api/v1 read surface behind the authorize seam.
	router := newRouter(st)
	authorizer := authorize.NewAllReposAuthorizer(st)
	httpapi.NewHandler(st).Mount(router, authorize.Middleware(authorizer))

	return serve(cfg.HTTP.Addr, router)
}

// runOnboard seeds an installation + repo and runs one synchronous ingest for
// spec (owner/name@installation_id), then returns. It is the Phase 2 manual
// onboard/re-sync trigger; Phase 5 drives the same ingest from webhooks.
func runOnboard(ctx context.Context, st *store.Store, idx ingest.Indexer, cfg *config.Config, spec string) error {
	owner, name, installationID, err := parseOnboardSpec(spec)
	if err != nil {
		return fmt.Errorf("parse -onboard: %w", err)
	}

	// Account login/type are placeholders here; the installation webhook
	// (Phase 5) overwrites them with the real values on the next event.
	if uerr := st.UpsertInstallation(ctx, store.InstallationInput{
		ID:           installationID,
		AccountLogin: owner,
		AccountType:  "Organization",
	}); uerr != nil {
		return fmt.Errorf("seed installation: %w", uerr)
	}

	ghClient, err := githubapp.NewClient(
		cfg.GitHub.AppID,
		[]byte(cfg.GitHub.PrivateKey.Reveal()),
		cfg.GitHub.APIBase,
		installationID,
	)
	if err != nil {
		return fmt.Errorf("build github client: %w", err)
	}

	res, err := ingest.NewService(st, ghClient, idx).Run(ctx, installationID, owner, name)
	if err != nil {
		return fmt.Errorf("ingest %s/%s: %w", owner, name, err)
	}

	slog.Info("onboard complete",
		"repo", owner+"/"+name,
		"docs_upserted", res.DocsUpserted,
		"docs_deleted", res.DocsDeleted,
		"docs_unchanged", res.DocsUnchanged,
		"types_upserted", res.TypesUpserted,
		"types_deleted", res.TypesDeleted,
	)
	return nil
}

// parseOnboardSpec splits "owner/name@installation_id" into its parts.
func parseOnboardSpec(spec string) (owner, name string, installationID int64, err error) {
	repoPart, idPart, ok := strings.Cut(spec, "@")
	if !ok {
		return "", "", 0, fmt.Errorf("expected owner/name@installation_id, got %q", spec)
	}
	owner, name, ok = strings.Cut(repoPart, "/")
	if !ok || owner == "" || name == "" {
		return "", "", 0, fmt.Errorf("expected owner/name, got %q", repoPart)
	}
	installationID, err = strconv.ParseInt(idPart, 10, 64)
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid installation id %q: %w", idPart, err)
	}
	if installationID <= 0 {
		return "", "", 0, fmt.Errorf("installation id must be positive, got %d", installationID)
	}
	return owner, name, installationID, nil
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

// newRouter builds the base chi router with the liveness and readiness probes.
// The /api/v1 read routes are mounted onto it by the caller so they sit behind
// the authorize middleware while the probes stay open.
func newRouter(ready readyChecker) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Get("/healthz", handleHealthz)
	r.Get("/readyz", handleReadyz(ready))
	return r
}

// handleHealthz is the liveness probe: it reports that the process is up,
// independent of downstream dependencies (readiness covers those).
func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, `{"status":"ok"}`)
}

// handleReadyz is the readiness probe: healthy only when Postgres is reachable.
// It gates traffic during rollout and dependency outages. Later phases extend
// the check to cover Meilisearch and Redis.
func handleReadyz(ready readyChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), readyzTimeout)
		defer cancel()
		if err := ready.Ping(ctx); err != nil {
			slog.Warn("readiness check failed", "err", err)
			writeJSON(w, http.StatusServiceUnavailable, `{"status":"unavailable"}`)
			return
		}
		writeJSON(w, http.StatusOK, `{"status":"ok"}`)
	}
}

// writeJSON writes a status code and a JSON body, logging a failed write at
// debug (the client has already gone away; nothing else to do).
func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write([]byte(body)); err != nil {
		slog.Debug("response write failed", "status", status, "err", err)
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
