package telemetry

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// The Prometheus instruments are package-level and registered once on the
// default registry (the idiomatic client_golang pattern). Route/reason labels
// are bounded (chi route templates, a small set of reasons) so cardinality
// stays flat. The default registry also carries Go-runtime and process
// collectors, so /metrics exposes them for free.
var (
	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "docz_api_http_requests_total",
		Help: "Total HTTP requests by method, matched route, and status code.",
	}, []string{"method", "route", "status"})

	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "docz_api_http_request_duration_seconds",
		Help:    "HTTP request duration in seconds by method and matched route.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route"})

	ingestJobs = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "docz_api_ingest_jobs_total",
		Help: "Total ingest jobs processed by trigger reason and outcome.",
	}, []string{"reason", "status"})

	ingestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "docz_api_ingest_job_duration_seconds",
		Help: "Ingest job duration in seconds by trigger reason.",
		// Wider than DefBuckets (which tops out at 10s): a GitHub-bound ingest
		// of a large repo can run tens of seconds, and those must not all
		// collapse into the +Inf bucket where the tail is invisible.
		Buckets: []float64{0.1, 0.5, 1, 2.5, 5, 10, 30, 60, 120},
	}, []string{"reason"})
)

// MetricsHandler serves the Prometheus exposition of the default registry
// (docz-api instruments plus Go-runtime and process collectors). The caller
// mounts it at /metrics when metrics are enabled.
func MetricsHandler() http.Handler { return promhttp.Handler() }

// observeHTTP records one completed HTTP request.
func observeHTTP(method, route string, status int, dur time.Duration) {
	httpRequests.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	httpDuration.WithLabelValues(method, route).Observe(dur.Seconds())
}

// ObserveIngest records one completed ingest job. status is "success" or
// "failure". It is called by the queue worker after each job.
func ObserveIngest(reason, status string, dur time.Duration) {
	ingestJobs.WithLabelValues(reason, status).Inc()
	ingestDuration.WithLabelValues(reason).Observe(dur.Seconds())
}
