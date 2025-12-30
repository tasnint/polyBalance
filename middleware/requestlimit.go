package middleware

import (
	"net/http"
	"sync"
)

type RequestLimiter struct {
	mu             sync.Mutex
	maxBodySize    int64
	maxHeaderSize  int
	enabled        bool
}

func NewRequestLimiter(maxBodySize int64, maxHeaderSize int, enabled bool) *RequestLimiter {
	return &RequestLimiter{
		maxBodySize:   maxBodySize,
		maxHeaderSize: maxHeaderSize,
		enabled:       enabled,
	}
}

func (rl *RequestLimiter) GetStats() map[string]interface{} {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	return map[string]interface{}{
		"enabled":         rl.enabled,
		"max_body_bytes":  rl.maxBodySize,
		"max_header_size": rl.maxHeaderSize,
	}
}

func (rl *RequestLimiter) SetEnabled(enabled bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.enabled = enabled
}

func (rl *RequestLimiter) SetLimits(maxBodySize int64, maxHeaderSize int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.maxBodySize = maxBodySize
	rl.maxHeaderSize = maxHeaderSize
}

func (rl *RequestLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.enabled {
			next.ServeHTTP(w, r)
			return
		}

		var headerSize int
		for name, values := range r.Header {
			headerSize += len(name)
			for _, v := range values {
				headerSize += len(v)
			}
		}

		if headerSize > rl.maxHeaderSize {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusRequestHeaderFieldsTooLarge)
			w.Write([]byte(`{"error": "request headers too large"}`))
			return
		}

		if r.ContentLength > rl.maxBodySize {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			w.Write([]byte(`{"error": "request body too large"}`))
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, rl.maxBodySize)

		next.ServeHTTP(w, r)
	})
}
