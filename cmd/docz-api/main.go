// Command docz-api is the entry point for the docz-api service.
//
// It stays thin: parse flags, load configuration, configure the slog default
// handler, wire dependencies, and run an HTTP server with graceful shutdown.
// All real behavior lives under internal/.
package main

import (
	"context"
	"encoding/json"
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
	"github.com/donaldgifford/docz-api/internal/httpapi"
	"github.com/donaldgifford/docz-api/internal/queue"
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

// namedChecker pairs a downstream dependency's name with its readiness check.
// The readiness probe runs each one and reports per-dependency status, so an
// outage names the offender.
type namedChecker struct {
	name  string
	check func(ctx context.Context) error
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

	// Async ingest queue: the enqueue client is shared; the worker (started
	// below for the server path) drains jobs through the ingest pipeline.
	queueClient, err := queue.NewClient(cfg.Store.RedisURL, cfg.Ingest.Debounce)
	if err != nil {
		return fmt.Errorf("connecting to redis: %w", err)
	}

	// `-onboard` seeds the installation and enqueues one ingest, then exits — the
	// manual trigger now goes through the queue (a running server drains it).
	if *onboardSpec != "" {
		defer closeQueueClient(queueClient)
		return runOnboard(context.Background(), st, queueClient, *onboardSpec)
	}

	// The worker runs in-process alongside the HTTP server (single-binary ethos).
	// It builds a per-installation GitHub client per job via ingestRunner.
	runner := &ingestRunner{store: st, indexer: searchClient, github: cfg.GitHub}
	worker, err := queue.NewWorker(cfg.Store.RedisURL, workerConcurrency, runner)
	if err != nil {
		closeQueueClient(queueClient)
		return fmt.Errorf("building queue worker: %w", err)
	}
	if err := worker.Start(); err != nil {
		closeQueueClient(queueClient)
		return fmt.Errorf("starting queue worker: %w", err)
	}

	slog.Info("starting docz-api",
		"version", version,
		"commit", commit,
		"addr", cfg.HTTP.Addr,
		"auth_providers", cfg.Auth.Providers,
		"worker_concurrency", workerConcurrency,
	)

	// Probes plus the /api/v1 read + search surface behind the authorize seam.
	// Readiness covers all three durable dependencies.
	router := newRouter([]namedChecker{
		{name: "postgres", check: st.Ping},
		{name: "meilisearch", check: searchClient.Health},
		{name: "redis", check: queueClient.Ping},
	})
	authorizer := authorize.NewAllReposAuthorizer(st)
	httpapi.NewHandlerWithSearch(st, searchClient).Mount(router, authorize.Middleware(authorizer))

	return serveWithWorker(cfg.HTTP.Addr, router, worker, queueClient)
}

// workerConcurrency bounds the number of parallel ingest jobs. Two balances
// homelab resource use with throughput: each job holds a pool connection and
// issues GitHub API calls.
const workerConcurrency = 2

// runOnboard seeds an installation and enqueues one ingest for spec
// (owner/name@installation_id), then returns. The manual trigger now goes
// through the queue — a running server's worker performs the ingest. Phase 5
// drives the same enqueue from webhooks.
func runOnboard(ctx context.Context, st *store.Store, enq queue.Enqueuer, spec string) error {
	owner, name, installationID, err := parseOnboardSpec(spec)
	if err != nil {
		return fmt.Errorf("parse -onboard: %w", err)
	}

	// Account login/type are placeholders here; the installation webhook
	// (Phase 5) overwrites them with the real values on the next event. The
	// installation row must exist before the worker reconciles (repos FK it).
	if uerr := st.UpsertInstallation(ctx, store.InstallationInput{
		ID:           installationID,
		AccountLogin: owner,
		AccountType:  "Organization",
	}); uerr != nil {
		return fmt.Errorf("seed installation: %w", uerr)
	}

	if eerr := enq.EnqueueIngest(ctx, &queue.IngestJob{
		InstallationID: installationID,
		Owner:          owner,
		Name:           name,
		Reason:         "onboard",
	}); eerr != nil {
		return fmt.Errorf("enqueue onboard ingest: %w", eerr)
	}

	slog.Info("onboard enqueued; a running worker will ingest shortly",
		"repo", owner+"/"+name, "installation_id", installationID)
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
func newRouter(checkers []namedChecker) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Get("/healthz", handleHealthz)
	r.Get("/readyz", handleReadyz(checkers))
	return r
}

// handleHealthz is the liveness probe: it reports that the process is up,
// independent of downstream dependencies (readiness covers those).
func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, `{"status":"ok"}`)
}

// handleReadyz is the readiness probe: it checks every named dependency and
// reports each one's status (e.g. {"meilisearch":"ok","postgres":"ok"}). It
// returns 503 if any check fails, 200 otherwise, so a rollout or dependency
// outage gates traffic and the body names the offender.
func handleReadyz(checkers []namedChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), readyzTimeout)
		defer cancel()

		status := make(map[string]string, len(checkers))
		healthy := true
		for _, c := range checkers {
			if err := c.check(ctx); err != nil {
				slog.Warn("readiness check failed", "dep", c.name, "err", err)
				status[c.name] = "unavailable"
				healthy = false
				continue
			}
			status[c.name] = "ok"
		}

		code := http.StatusOK
		if !healthy {
			code = http.StatusServiceUnavailable
		}
		writeReadyz(w, code, status)
	}
}

// writeReadyz writes the readiness status map as JSON with the given code. Map
// keys marshal in sorted order, so the body is deterministic.
func writeReadyz(w http.ResponseWriter, code int, status map[string]string) {
	body, err := json.Marshal(status)
	if err != nil {
		// status is a map[string]string; marshaling cannot realistically fail.
		body = []byte(`{"status":"error"}`)
		code = http.StatusInternalServerError
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if _, werr := w.Write(body); werr != nil {
		slog.Debug("readyz write failed", "err", werr)
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

// serveWithWorker runs the HTTP server and the async ingest worker until an
// interrupt/terminate signal, then drains in order: HTTP first (so no new
// webhook/onboard enqueues arrive), then the worker (drain in-flight ingests),
// then the queue client. This ordering guarantees no ingest job is lost.
func serveWithWorker(addr string, handler http.Handler, w *queue.Worker, qc *queue.Client) error {
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
		w.Shutdown()
		closeQueueClient(qc)
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		slog.Info("shutdown signal received, draining")
	}

	// 1) Stop accepting HTTP and drain in-flight requests (no new enqueues).
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Warn("http shutdown did not complete cleanly", "err", err)
	}
	// 2) Drain in-flight ingest jobs.
	slog.Info("draining ingest worker")
	w.Shutdown()
	// 3) Close the enqueue/redis client.
	closeQueueClient(qc)
	slog.Info("shutdown complete")
	return nil
}

// closeQueueClient closes the queue client, logging a close error at warn.
func closeQueueClient(qc *queue.Client) {
	if err := qc.Close(); err != nil {
		slog.Warn("close queue client", "err", err)
	}
}
