package server

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"polybalance/backend"
	"polybalance/proxy"
	"polybalance/strategy"
)

type Server struct {
	Backends []*backend.Backend
	Strategy strategy.Strategy
}

// NewServer creates a new HTTP server with the given backends and strategy
func NewServer(backendURLS []string, weights []int, strat strategy.Strategy) (*Server, error) {
	// input Validation:
	if len(backendURLS) == 0 {
		return nil, fmt.Errorf("no backend URLs provided")
	}

	if len(weights) != 0 && len(weights) != len(backendURLS) {
		return nil, fmt.Errorf("number of weights must match number of backend URLs")
	}

	// create a backend slice, preallocate memory for it for efficiency
	backends := make([]*backend.Backend, 0, len(backendURLS))

	// iterates over each backend URL, create a backend instance and add it to the slice
	for i, rawURL := range backendURLS {
		// parse the URL and create a reverse proxy
		target := rawURL
		rp := httputil.NewSingleHostReverseProxy(mustParseURL(target))

		weight := 1
		if len(weights) != 0 {
			weight = weights[i]
		}

		b, err := backend.NewBackend(rawURL, weight, rp)
		if err != nil {
			return nil, err
		}
		backends = append(backends, b)
	}
	s := &Server{
		Backends: backends,
		Strategy: strat,
	}
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	b := s.Strategy.NextBackend(s.Backends)
	if b == nil {
		http.Error(w, "No backend available", http.StatusServiceUnavailable)
		return
	}

	// wrap with proxy layer
	p := proxy.NewProxy(b)

	// Sends request to chosen backend server
	p.ServeHTTP(w, r)
}

// Must parse URL helper
func mustParseURL(raw string) *url.URL {
	parsed, err := url.Parse(raw)
	if err != nil {
		log.Fatalf("invalid backend URL %s: %v", raw, err)
	}
	return parsed
}
