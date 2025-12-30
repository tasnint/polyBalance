package server

import (
        "fmt"
        "net/http"
        "polybalance/backend"
        "polybalance/proxy"
        "polybalance/strategy"
)

type Server struct {
        Backends []*backend.Backend
        Strategy strategy.Strategy
}

const (
        maxRetries = 2
)

func (s *Server) RegisterHealthEndpoints(mux *http.ServeMux) {

        mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
                w.WriteHeader(http.StatusOK)
                w.Write([]byte("ok"))
        })

        mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
                if len(s.Backends) == 0 {
                        http.Error(w, "no backends available", http.StatusServiceUnavailable)
                        return
                }
                w.WriteHeader(http.StatusOK)
                w.Write([]byte("ready"))
        })
}

func isRetryableMethod(method string) bool {
        switch method {
        case http.MethodGet, http.MethodHead, http.MethodOptions:
                return true
        default:
                return false
        }
}

func isRetryableStatus(code int) bool {
        switch code {
        case http.StatusBadGateway,
                http.StatusServiceUnavailable,
                http.StatusGatewayTimeout:
                return true
        default:
                return false
        }
}

type responseRecorder struct {
        http.ResponseWriter
        status int
}

func (r *responseRecorder) WriteHeader(code int) {
        r.status = code
        r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
        if r.status == 0 {
                r.status = http.StatusOK
        }
        return r.ResponseWriter.Write(b)
}

// NewServer creates a new HTTP server with the given backends and strategy
func NewServer(backends []*backend.Backend, strat strategy.Strategy) (*Server, error) {
        if len(backends) == 0 {
                return nil, fmt.Errorf("no backends provided")
        }

        s := &Server{
                Backends: backends,
                Strategy: strat,
        }
        return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {

        // Non-idempotent → single attempt only
        if !isRetryableMethod(r.Method) {
                b := s.Strategy.NextBackend(s.Backends)
                if b == nil {
                        http.Error(w, "No backend available", http.StatusServiceUnavailable)
                        return
                }
                proxy := proxy.NewProxy(b)
                proxy.ServeHTTP(w, r)
                return
        }

        var lastStatus int

        for attempt := 0; attempt <= maxRetries; attempt++ {

                b := s.Strategy.NextBackend(s.Backends)
                if b == nil {
                        http.Error(w, "No backend available", http.StatusServiceUnavailable)
                        return
                }

                rec := &responseRecorder{ResponseWriter: w}

                p := proxy.NewProxy(b)
                p.ServeHTTP(rec, r)

                // Success or non-retryable failure → send response
                if !isRetryableStatus(rec.status) {
                        if rec.status != 0 {
                                w.WriteHeader(rec.status)
                        }
                        return
                }
                lastStatus = rec.status
        }

        http.Error(w, "Request failed after retries", lastStatus)
}
