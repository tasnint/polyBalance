package backend

import (
	"context"
	"log"
	"net/http"
	"time"
)

// health.go periodically sends HTTP GET requests to each backend to determine whether the server is alive, and updates the backend’s health/circuit-breaker state accordingly
type HealthChecker struct {
	backends []*Backend

	interval time.Duration
	timeout  time.Duration
	path     string
	client   *http.Client
}

func NewHealthChecker(backends []*Backend, interval, timeout time.Duration, path string) *HealthChecker {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if timeout <= 0 {
		timeout = 1 * time.Second
	}

	if path == "" {
		path = "/healthz"
	}

	return &HealthChecker{
		backends: backends,
		interval: interval,
		timeout:  timeout,
		path:     path,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Start function begins the periodic health check loop on a goroutine
// it stops when ctc is cancelled
func (hc *HealthChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(hc.interval)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				hc.checkAll()
			}
		}
	}()
}

// checkAll probes every backend once
func (hc *HealthChecker) checkAll() {
	for _, b := range hc.backends {
		hc.checkOne(b)
	}
}

// checkOne sends a single HTTP request to a backend's health endpoint
func (hc *HealthChecker) checkOne(b *Backend) {
	u := *b.URL
	u.Path = hc.path

	resp, err := hc.client.Get(u.String())

	if err != nil {
		log.Printf("Health check failed for backend %s: %v", b.URL.String(), err)
		b.SetAlive(false)
		b.RecordFailure()
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		log.Printf("[health] Backend %s returned status code %d", b.URL.String(), resp.StatusCode)
		b.SetAlive(false)
		b.RecordFailure()
		return
	}

	b.SetAlive(true)
	b.RecordSuccess()
}
