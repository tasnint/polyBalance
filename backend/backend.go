package backend

import (
        "net/http/httputil"
        "net/url"
        "sync"
        "time"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
        CircuitClosed CircuitState = iota
        CircuitOpen
        CircuitHalfOpen
)

// Backend holds data and runtime state for a single upstream server
type Backend struct {
        URL    *url.URL
        Proxy  *httputil.ReverseProxy
        Weight int

        mu sync.RWMutex

        alive        bool
        Circuit      CircuitState
        LastFailure  time.Time
        FailureCount int64

        ActiveConnections int64
        AvgLatency        time.Duration
}

func NewBackend(rawURL string, weight int, proxy *httputil.ReverseProxy) (*Backend, error) {
        u, err := url.Parse(rawURL)
        if err != nil {
                return nil, err
        }
        return &Backend{
                URL:               u,
                Proxy:             proxy,
                Weight:            weight,
                alive:             true,
                Circuit:           CircuitClosed,
                ActiveConnections: 0,
                AvgLatency:        0,
        }, nil
}

// -- health & alive ---
func (b *Backend) SetAlive(alive bool) {
        b.mu.Lock()
        defer b.mu.Unlock()
        b.alive = alive
}

func (b *Backend) IsAlive() bool {
        b.mu.RLock()
        defer b.mu.RUnlock()
        if b.Circuit == CircuitOpen {
                return false
        }
        return b.alive
}

// --- connection tracking ---
func (b *Backend) IncConnections() {
        b.mu.Lock()
        b.ActiveConnections++
        b.mu.Unlock()
}

func (b *Backend) DecConnections() {
        b.mu.Lock()
        if b.ActiveConnections > 0 {
                b.ActiveConnections--
        }
        b.mu.Unlock()
}

// --- latency tracking --- (simple EWMA)

// RecordLatency updates the avg latency using exponential moving average
func (b *Backend) RecordLatency(sample time.Duration) {
        const alpha = 0.2
        b.mu.Lock()
        defer b.mu.Unlock()

        if b.AvgLatency == 0 {
                b.AvgLatency = sample
                return
        }
        b.AvgLatency = time.Duration((1-alpha)*float64(b.AvgLatency) + alpha*float64(sample))
}

// -- circuit breaker helpers --
const (
        // number of failures to trigger circuit open:
        // source:
        // https://github.com/Netflix/Hystrix/wiki/Configuration#circuitBreaker.requestVolumeThreshold
        // https://istio.io/latest/docs/reference/config/networking/destination-rule/
        MaxFailures = 5
        // time to wait before allowing a single request to test the backend: 5-10 seconds commonly used
        // source:
        // https://nginx.org/en/docs/http/ngx_http_upstream_module.html#fail_timeout
        // https://github.com/Netflix/Hystrix/wiki/Configuration#circuitBreaker.sleepWindowInMilliseconds
        OpenStateTimeout = 10 * time.Second
        // how long a backend stays in Half-Open mode before giving up and fully opening again
        // source:
        // https://docs.aws.amazon.com/apigateway/latest/developerguide/welcome.html
        // https://learn.microsoft.com/en-us/azure/architecture/patterns/circuit-breaker
        HalfOpenRetryWindow = 30 * time.Second
)

func (b *Backend) RecordFailure() {
        b.mu.Lock()
        defer b.mu.Unlock()

        b.FailureCount++
        b.LastFailure = time.Now()

        // if too many failures, open the circuit
        if b.FailureCount >= MaxFailures && b.Circuit == CircuitClosed {
                b.Circuit = CircuitOpen
        }
}

func (b *Backend) RecordSuccess() {
        b.mu.Lock()
        defer b.mu.Unlock()

        b.FailureCount = 0

        // if backend was being tested (half-open), and succeeded, close the circuit
        if b.Circuit == CircuitHalfOpen {
                b.Circuit = CircuitClosed
        }
        // Note: Do not set alive = true here. The health checker is the sole source
        // of truth for the alive status. This prevents request-level success from
        // overriding health check failures.
}

func (b *Backend) CheckCircuitState() bool {
        b.mu.RLock()
        defer b.mu.RUnlock()

        switch b.Circuit {

        case CircuitOpen:
                // return true if this backend can be trialed (but don't change state here)
                return time.Since(b.LastFailure) >= OpenStateTimeout

        case CircuitHalfOpen:
                return true

        case CircuitClosed:
                return b.alive
        }

        return false
}

func (b *Backend) CanAttemptHalfOpen() bool {
        b.mu.RLock()
        defer b.mu.RUnlock()
        return time.Since(b.LastFailure) >= OpenStateTimeout
}

func (b *Backend) SetCircuitHalfOpen() {
        b.mu.Lock()
        b.Circuit = CircuitHalfOpen
        b.mu.Unlock()
}

func (b *Backend) GetCircuitState() CircuitState {
        b.mu.RLock()
        defer b.mu.RUnlock()
        return b.Circuit
}

func (b *Backend) GetAverageLatency() time.Duration {
        b.mu.RLock()
        defer b.mu.RUnlock()
        return b.AvgLatency
}

func (b *Backend) GetActiveConnections() int64 {
        b.mu.RLock()
        defer b.mu.RUnlock()
        return b.ActiveConnections
}
