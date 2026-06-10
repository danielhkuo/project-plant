// Command healthcheck probes an HTTP endpoint and exits 0 on a 200 response.
//
// It exists for container health checks: the services ship in distroless
// images with no shell or curl, so compose/orchestrators exec this static
// binary instead, e.g. ["/healthcheck", "http://localhost:8080/health"].
package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: healthcheck <url>")
		os.Exit(2)
	}

	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "unhealthy: %s\n", resp.Status)
		os.Exit(1)
	}
}
