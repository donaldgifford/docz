// Package webhook receives and verifies GitHub App webhooks, then drives
// onboarding and incremental refresh. This route has no bearer auth: every
// request is authenticated by an HMAC-SHA256 signature over the raw body, and a
// mismatch is rejected with 401 before any work. Verified deliveries are
// deduplicated by their X-GitHub-Delivery id (webhook_deliveries) so a replay is
// a no-op, then routed by event type — installation and
// installation_repositories drive onboard/offboard, push enqueues an
// incremental re-ingest, and release is logged only (versions are deferred).
//
// The handler never ingests inline: onboarding and push both enqueue a job on
// the async queue and return promptly, keeping webhook latency low and letting
// the worker, debounce window, and content-hash gate do the heavy lifting.
package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/go-github/v88/github"

	"github.com/donaldgifford/docz-api/internal/queue"
	"github.com/donaldgifford/docz-api/internal/search"
	"github.com/donaldgifford/docz-api/internal/store"
)

const (
	// maxBodyBytes caps the webhook request body. Real GitHub payloads are far
	// smaller; the cap bounds memory from a hostile or misconfigured sender.
	maxBodyBytes = 5 << 20 // 5 MiB
	// signaturePrefix is the algorithm marker on the X-Hub-Signature-256 header.
	signaturePrefix = "sha256="
	// doczConfigFile is the repo-root manifest whose change forces a re-ingest.
	doczConfigFile = ".docz.yaml"
)

// webhookStore is the persistence surface the handler needs: delivery
// idempotency, installation upsert/delete, repo lookup/delete, and repo-id
// enumeration for index purges. *store.Store satisfies it.
type webhookStore interface {
	RecordDelivery(ctx context.Context, deliveryID, event string) (bool, error)
	UpsertInstallation(ctx context.Context, in store.InstallationInput) error
	DeleteInstallation(ctx context.Context, id int64) error
	ListRepoIDsByInstallation(ctx context.Context, installationID int64) ([]int64, error)
	GetRepo(ctx context.Context, owner, name string) (store.Repo, error)
	DeleteRepo(ctx context.Context, owner, name string) (int64, error)
}

// enqueuer schedules an ingest job. *queue.Client satisfies it (re-declaring
// queue.Enqueuer consumer-side keeps webhook's dependency explicit).
type enqueuer interface {
	EnqueueIngest(ctx context.Context, job *queue.IngestJob) error
}

// indexPurger removes a repo's documents from the search index during
// offboarding. *search.Client satisfies it.
type indexPurger interface {
	DeleteRepoDocuments(ctx context.Context, repoID int64) error
}

// The production implementations satisfy the consumer interfaces above.
var (
	_ webhookStore = (*store.Store)(nil)
	_ enqueuer     = (*queue.Client)(nil)
	_ indexPurger  = (*search.Client)(nil)
)

// Handler is the http.Handler for POST /webhooks/github.
type Handler struct {
	secret []byte
	store  webhookStore
	enq    enqueuer
	purger indexPurger
}

var _ http.Handler = (*Handler)(nil)

// New builds a webhook Handler. secret is the GitHub App webhook secret (raw
// bytes) used for HMAC verification. purger may be nil to disable search-index
// cleanup on offboarding (used by tests without a Meilisearch backend).
func New(secret []byte, st webhookStore, enq enqueuer, purger indexPurger) *Handler {
	return &Handler{secret: secret, store: st, enq: enq, purger: purger}
}

// ServeHTTP verifies the signature, deduplicates the delivery, parses the event,
// and routes it. The raw body is read exactly once — HMAC is computed over the
// precise bytes GitHub signed, and the same bytes are reused for parsing.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		writeStatus(w, http.StatusBadRequest, "read body")
		return
	}

	if !verifyHMAC(h.secret, body, r.Header.Get("X-Hub-Signature-256")) {
		// The sender is unauthenticated; never log the payload it sent.
		slog.Warn("webhook signature verification failed",
			"delivery", r.Header.Get("X-GitHub-Delivery"),
			"event", r.Header.Get("X-GitHub-Event"))
		writeStatus(w, http.StatusUnauthorized, "invalid signature")
		return
	}

	event := r.Header.Get("X-GitHub-Event")
	if deliveryID := r.Header.Get("X-GitHub-Delivery"); deliveryID != "" {
		isNew, rerr := h.store.RecordDelivery(r.Context(), deliveryID, event)
		if rerr != nil {
			slog.Error("recording webhook delivery", "delivery", deliveryID, "err", rerr)
			writeStatus(w, http.StatusInternalServerError, "record delivery")
			return
		}
		if !isNew {
			slog.Debug("replayed webhook delivery ignored", "delivery", deliveryID, "event", event)
			writeStatus(w, http.StatusOK, "already processed")
			return
		}
	}

	payload, err := github.ParseWebHook(event, body)
	if err != nil {
		writeStatus(w, http.StatusBadRequest, "parse payload")
		return
	}

	if err := h.route(r.Context(), payload); err != nil {
		slog.Error("webhook handling failed", "event", event, "err", err)
		writeStatus(w, http.StatusInternalServerError, "handling failed")
		return
	}
	writeStatus(w, http.StatusAccepted, "accepted")
}

// verifyHMAC reports whether sigHeader is a valid "sha256=<hex>" HMAC of body
// under secret. The comparison uses hmac.Equal (constant-time) so a caller
// cannot learn the expected signature by timing. A malformed or missing header
// fails closed.
func verifyHMAC(secret, body []byte, sigHeader string) bool {
	hexSig, ok := strings.CutPrefix(sigHeader, signaturePrefix)
	if !ok {
		return false
	}
	want, err := hex.DecodeString(hexSig)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	// hash.Hash.Write is documented never to return an error.
	if _, err := mac.Write(body); err != nil {
		return false
	}
	return hmac.Equal(mac.Sum(nil), want)
}

// writeStatus writes a small JSON status envelope. status is a fixed,
// caller-supplied literal, so it needs no escaping.
func writeStatus(w http.ResponseWriter, code int, status string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if _, err := w.Write([]byte(`{"status":"` + status + `"}`)); err != nil {
		slog.Debug("webhook response write failed", "err", err)
	}
}
