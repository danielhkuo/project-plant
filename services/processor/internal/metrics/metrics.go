// Package metrics defines the processor's Prometheus collectors and
// implements the consumer.Metrics observer.
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

// Metrics holds the processor's collectors.
type Metrics struct {
	registry        *prometheus.Registry
	eventsProcessed *prometheus.CounterVec
	alertsFired     *prometheus.CounterVec
	consumerLag     prometheus.Gauge
}

// New creates the collectors on a fresh registry.
func New() *Metrics {
	reg := prometheus.NewRegistry()
	// Runtime collectors: go_memstats_* backs the Step 16 soak memory check.
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	eventsProcessed := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "events_processed_total",
		Help: "Telemetry messages consumed from Kafka, by outcome.",
	}, []string{"result"})
	for _, r := range []string{"success", "error"} {
		eventsProcessed.WithLabelValues(r)
	}

	alertsFired := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "alerts_fired_total",
		Help: "Alerts triggered by the rule engine, by rule and severity.",
	}, []string{"rule", "severity"})

	consumerLag := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "kafka_consumer_lag",
		Help: "Messages between the latest offset and this consumer group's position.",
	})

	reg.MustRegister(eventsProcessed, alertsFired, consumerLag)
	return &Metrics{
		registry:        reg,
		eventsProcessed: eventsProcessed,
		alertsFired:     alertsFired,
		consumerLag:     consumerLag,
	}
}

// EventProcessed implements consumer.Metrics.
func (m *Metrics) EventProcessed(result string) {
	m.eventsProcessed.WithLabelValues(result).Inc()
}

// AlertFired implements consumer.Metrics.
func (m *Metrics) AlertFired(rule, severity string) {
	m.alertsFired.WithLabelValues(rule, severity).Inc()
}

// SetConsumerLag updates the lag gauge from the Kafka reader's stats.
func (m *Metrics) SetConsumerLag(lag int64) {
	m.consumerLag.Set(float64(lag))
}

// EventsProcessedCounter exposes one result child for test assertions.
func (m *Metrics) EventsProcessedCounter(result string) prometheus.Counter {
	return m.eventsProcessed.WithLabelValues(result)
}

// AlertsFiredCounter exposes one (rule, severity) child for test assertions.
func (m *Metrics) AlertsFiredCounter(rule, severity string) prometheus.Counter {
	return m.alertsFired.WithLabelValues(rule, severity)
}

// Handler serves the registry in Prometheus exposition format.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
