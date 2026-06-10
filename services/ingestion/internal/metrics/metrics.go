// Package metrics defines the ingestion service's Prometheus collectors.
//
// Each Metrics value owns a private registry (never the global default) so
// tests can construct fresh instances without duplicate-registration panics.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Status label values for the events_ingested_total counter.
const (
	StatusAccepted = "accepted" // validated and published
	StatusRejected = "rejected" // failed validation (client error)
	StatusError    = "error"    // valid but publish failed (server error)
)

// Metrics holds the ingestion service's collectors. A nil *Metrics is valid
// and turns every method into a no-op, so handlers can be constructed without
// metrics in tests.
type Metrics struct {
	registry       *prometheus.Registry
	eventsIngested *prometheus.CounterVec
	latency        prometheus.Histogram
}

// New creates the collectors on a fresh registry. The status label children
// are pre-initialized so the counter family is visible at zero and tests can
// read a single child deterministically.
func New() *Metrics {
	reg := prometheus.NewRegistry()
	// Runtime collectors: go_memstats_* backs the Step 16 soak memory check.
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	eventsIngested := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "events_ingested_total",
		Help: "Telemetry events received, by outcome.",
	}, []string{"status"})
	for _, s := range []string{StatusAccepted, StatusRejected, StatusError} {
		eventsIngested.WithLabelValues(s)
	}

	latency := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "ingestion_latency_seconds",
		Help: "End-to-end ingest request latency.",
		// Finer than DefBuckets around the 50ms SLO (local p99 is ~10ms).
		Buckets: []float64{.001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2.5},
	})

	reg.MustRegister(eventsIngested, latency)
	return &Metrics{registry: reg, eventsIngested: eventsIngested, latency: latency}
}

// EventsIngested adds n events with the given status.
func (m *Metrics) EventsIngested(status string, n int) {
	if m == nil {
		return
	}
	m.eventsIngested.WithLabelValues(status).Add(float64(n))
}

// EventsIngestedCounter exposes one status child for test assertions.
func (m *Metrics) EventsIngestedCounter(status string) prometheus.Counter {
	return m.eventsIngested.WithLabelValues(status)
}

// InstrumentIngest observes request latency for the telemetry routes. Applied
// per-route so /health and /metrics probes don't pollute the histogram.
func (m *Metrics) InstrumentIngest(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timer := prometheus.NewTimer(m.latency)
		defer timer.ObserveDuration()
		next.ServeHTTP(w, r)
	})
}

// Handler serves the registry in Prometheus exposition format.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
