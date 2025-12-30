package middleware

import (
	"net/http"
	"sync"
	"time"
)

type RateLimiter struct {
	mu          sync.Mutex
	requests    map[string][]time.Time
	maxRequests int
	window      time.Duration
	enabled     bool
}

func NewRateLimiter(maxRequests int, window time.Duration, enabled bool) *RateLimiter {
	rl := &RateLimiter{
		requests:    make(map[string][]time.Time),
		maxRequests: maxRequests,
		window:      window,
		enabled:     enabled,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, times := range rl.requests {
			var valid []time.Time
			for _, t := range times {
				if now.Sub(t) < rl.window {
					valid = append(valid, t)
				}
			}
			if len(valid) == 0 {
				delete(rl.requests, ip)
			} else {
				rl.requests[ip] = valid
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) Allow(ip string) bool {
	if !rl.enabled {
		return true
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	times := rl.requests[ip]

	var valid []time.Time
	for _, t := range times {
		if now.Sub(t) < rl.window {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.maxRequests {
		rl.requests[ip] = valid
		return false
	}

	rl.requests[ip] = append(valid, now)
	return true
}

func (rl *RateLimiter) GetStats() map[string]interface{} {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	return map[string]interface{}{
		"enabled":      rl.enabled,
		"max_requests": rl.maxRequests,
		"window_secs":  int(rl.window.Seconds()),
		"active_ips":   len(rl.requests),
	}
}

func (rl *RateLimiter) SetEnabled(enabled bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.enabled = enabled
}

func (rl *RateLimiter) SetLimits(maxRequests int, window time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.maxRequests = maxRequests
	rl.window = window
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = forwarded
		}

		if !rl.Allow(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error": "rate limit exceeded", "retry_after": 60}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}
