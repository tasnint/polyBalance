package proxy

import (
        "fmt"
        "log"
        "net"
        "net/http"
        "net/http/httputil"
        "net/url"
        "polybalance/backend"
        "sync/atomic"
        "time"
)

var requestCounter uint64

type Proxy struct {
        backend *backend.Backend
        proxy   *httputil.ReverseProxy
}

func newUpstreamTransport() *http.Transport {
        return &http.Transport{
                DialContext: (&net.Dialer{
                        Timeout:   5 * time.Second,
                        KeepAlive: 30 * time.Second,
                }).DialContext,

                TLSHandshakeTimeout: 5 * time.Second,

                ResponseHeaderTimeout: 5 * time.Second,
                ExpectContinueTimeout: 1 * time.Second,

                MaxIdleConns:        100,
                MaxIdleConnsPerHost: 10,
                IdleConnTimeout:     90 * time.Second,
        }
}

func NewReverseProxy(rawURL string) *httputil.ReverseProxy {
        target, err := url.Parse(rawURL)
        if err != nil {
                log.Fatalf("Invalid backend URL %s: %v", rawURL, err)
        }
        proxy := httputil.NewSingleHostReverseProxy(target)
        proxy.Transport = newUpstreamTransport()
        return proxy
}

// Reverse proxy is middleware that forwards requests from client to a backend server and returns repsonses from backend to client
// NewProxy wraps a reverse proxy with LB logic
func NewProxy(b *backend.Backend) *Proxy {
        p := &Proxy{
                backend: b,
                proxy:   b.Proxy,
        }

        // Modify requests before forwarding to backend
        p.proxy.ModifyResponse = p.handleSuccess
        p.proxy.ErrorHandler = p.handleError

        return p
}

// before sending request: add forwarding headers and request ID
func (p *Proxy) prepareRequest(r *http.Request) {
        // Ensure X-Forwarded-For is set
        if clientIP, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
                prior := r.Header.Get("X-Forwarded-For")
                if prior != "" {
                        r.Header.Set("X-Forwarded-For", prior+", "+clientIP)
                } else {
                        r.Header.Set("X-Forwarded-For", clientIP)
                }
        }

        // Add request ID if not present
        if r.Header.Get("X-Request-ID") == "" {
                reqID := atomic.AddUint64(&requestCounter, 1)
                r.Header.Set("X-Request-ID", fmt.Sprintf("lb-%d-%d", time.Now().UnixNano(), reqID))
        }
}

// serveHTTP is the entry point for proxying a request
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
        b := p.backend

        // circuit breaker gate
        if !b.CheckCircuitState() {
                http.Error(w, "Backend temporarily unavailable", http.StatusServiceUnavailable)
                return
        }

        // if backend was open but cooldown passed, transition to half open
        if b.GetCircuitState() == backend.CircuitOpen && b.CanAttemptHalfOpen() {
                b.SetCircuitHalfOpen()
        }

        // track active connections
        b.IncConnections()
        start := time.Now()

        // prepare headers
        p.prepareRequest(r)

        requestID := r.Header.Get("X-Request-ID")
        log.Printf("[PROXY] Request %s -> Backend %s (%s %s)", requestID, b.URL.Host, r.Method, r.URL.Path)

        // proxy the request
        p.proxy.ServeHTTP(w, r)

        // cleanup after response
        elapsed := time.Since(start)
        b.RecordLatency(elapsed)
        b.DecConnections()

        log.Printf("[PROXY] Request %s completed in %v", requestID, elapsed)
}

func (p *Proxy) handleSuccess(resp *http.Response) error {
        b := p.backend

        b.RecordSuccess()

        return nil
}

func (p *Proxy) handleError(w http.ResponseWriter, r *http.Request, err error) {
        b := p.backend

        // Record failure for circuit breaker logic
        b.RecordFailure()

        http.Error(w, "Error contacting backend server", http.StatusBadGateway)
}
