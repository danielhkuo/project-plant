package api

import (
	"context"
	"net/http"
	"time"
)

const healthCheckTimeout = 2 * time.Second

// HealthHandler reports service liveness and Kafka connectivity. It is
// intentionally unauthenticated so monitoring systems can probe it.
//
//	200 {"status":"ok","kafka":"connected"}        when Kafka is reachable
//	503 {"status":"degraded","kafka":"disconnected"} when it is not
func HealthHandler(producer EventProducer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), healthCheckTimeout)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")
		if err := producer.Healthy(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"degraded","kafka":"disconnected"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","kafka":"connected"}`))
	}
}
