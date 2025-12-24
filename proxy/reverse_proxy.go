package proxy

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"polybalance/backend"
	"time"
)

type Proxy struct {
	backend *backend.Backend
	proxy   *httputil.ReverseProxy
}

func NewReverseProxy(rawURL string) *httputil.ReverseProxy {
	target, err := url.Parse(rawURL)
	if err != nil {
		log.Fatalf("Invalid backend URL %s: %v", rawURL, err)
	}
	return httputil.NewSingleHostReverseProxy(target)
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

// before sending request: add forwarding headers
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

	// proxy the request
	p.proxy.ServeHTTP(w, r)

	// cleanup after response
	elapsed := time.Since(start)
	b.RecordLatency(elapsed)
	b.DecConnections()
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
