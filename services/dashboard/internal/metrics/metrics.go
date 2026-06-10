// Package metrics defines the dashboard service's Prometheus collectors.
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

// Metrics holds the dashboard's collectors.
type Metrics struct {
	registry *prometheus.Registry
}

// New creates the collectors on a fresh registry. clientCount is sampled at
// scrape time for the active_websocket_connections gauge — pass the WS hub's
// ClientCount method.
func New(clientCount func() int) *Metrics {
	reg := prometheus.NewRegistry()
	// Runtime collectors: go_memstats_* backs the Step 16 soak memory check.
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "active_websocket_connections",
		Help: "WebSocket clients currently connected to the events stream.",
	}, func() float64 {
		return float64(clientCount())
	}))

	return &Metrics{registry: reg}
}

// Gather exposes the registry for test assertions.
func (m *Metrics) Gather() prometheus.Gatherer {
	return m.registry
}

// Handler serves the registry in Prometheus exposition format.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
