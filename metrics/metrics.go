package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// -------------------------------
//      METRIC DEFINITIONS
// -------------------------------

// Total number of requests handled by the load balancer
var TotalRequests = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "polybalance_requests_total",
		Help: "Total number of HTTP requests processed by the load balancer",
	},
)

// Failures per backend (indexed by backend URL)
var BackendFailures = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "polybalance_backend_failures_total",
		Help: "Number of failed requests per backend",
	},
	[]string{"backend"},
)

// Number of active proxied requests
var ActiveConnections = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "polybalance_active_connections",
		Help: "Current number of active proxied requests",
	},
)

// Request duration histogram
var RequestDuration = prometheus.NewHistogram(
	prometheus.HistogramOpts{
		Name:    "polybalance_request_duration_seconds",
		Help:    "Histogram of request durations through the load balancer",
		Buckets: prometheus.DefBuckets,
	},
)

// Backend health: 1 = healthy, 0 = unhealthy
var BackendHealth = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "polybalance_backend_health",
		Help: "Backend health status (1 = healthy, 0 = unhealthy)",
	},
	[]string{"backend"},
)

// -------------------------------
//      REGISTER METRICS
// -------------------------------

func init() {
	prometheus.MustRegister(TotalRequests)
	prometheus.MustRegister(BackendFailures)
	prometheus.MustRegister(ActiveConnections)
	prometheus.MustRegister(RequestDuration)
	prometheus.MustRegister(BackendHealth)
}

// -------------------------------
//      EXPOSE METRICS ENDPOINT
// -------------------------------

// Handler returns an HTTP handler for exposing /metrics
func Handler() http.Handler {
	return promhttp.Handler()
}

// StartMetricsServer starts an HTTP server to expose metrics at the given address
func StartMetricsServer(addr string) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", Handler())
	return http.ListenAndServe(addr, mux)
}
