// Package health provides the shared dependency-aware health endpoint used by
// every service. Each service passes the checks it owns (kafka, postgres,
// redis, ...) and gets a uniform JSON response:
//
//	200 {"status":"ok","kafka":"connected",...}
//	503 {"status":"degraded","kafka":"disconnected",...}
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// checkTimeout bounds the whole health evaluation; a dependency that cannot
// answer within it is reported as disconnected rather than hanging the probe
// (monitoring clients poll with short timeouts of their own).
const checkTimeout = 2 * time.Second

// Check reports whether one dependency is reachable. pgxpool.Pool.Ping and
// the ingestion producer's Healthy method satisfy it as method values.
type Check func(ctx context.Context) error

// Handler builds the health endpoint for the given dependency checks.
// Checks run concurrently under a shared deadline; one that has not returned
// by the deadline counts as disconnected.
func Handler(checks map[string]Check) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), checkTimeout)
		defer cancel()

		type result struct {
			name string
			err  error
		}
		results := make(chan result, len(checks))

		var wg sync.WaitGroup
		for name, check := range checks {
			wg.Add(1)
			go func(name string, check Check) {
				defer wg.Done()
				results <- result{name: name, err: check(ctx)}
			}(name, check)
		}
		go func() {
			wg.Wait()
			close(results)
		}()

		// Pessimistic default: anything that hasn't reported by the deadline
		// stays disconnected.
		body := make(map[string]string, len(checks)+1)
		for name := range checks {
			body[name] = "disconnected"
		}

		healthy := true
	collect:
		for range checks {
			select {
			case res := <-results:
				if res.err == nil {
					body[res.name] = "connected"
				} else {
					healthy = false
				}
			case <-ctx.Done():
				healthy = false
				break collect
			}
		}

		status := http.StatusOK
		body["status"] = "ok"
		if !healthy {
			status = http.StatusServiceUnavailable
			body["status"] = "degraded"
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(body)
	}
}
