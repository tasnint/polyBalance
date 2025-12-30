package internal

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr     string
	BackendURLs    []string
	Weights        []int
	Strategy       string
	HealthInterval time.Duration
	HealthTimeout  time.Duration
	MetricsEnabled bool
	MetricsAddr    string

	RateLimitEnabled bool
	RateLimitMax     int
	RateLimitWindow  time.Duration

	RequestLimitEnabled bool
	MaxBodySize         int64
	MaxHeaderSize       int

	TLSEnabled  bool
	TLSCertFile string
	TLSKeyFile  string
	TLSAutoGen  bool
}

func LoadConfig() *Config {
	cfg := &Config{
		ListenAddr:     getEnv("LB_LISTEN_ADDR", ":8080"),
		BackendURLs:    parseCSV(getEnv("LB_BACKENDS", "")),
		Weights:        parseIntCSV(getEnv("LB_WEIGHTS", "")),
		Strategy:       getEnv("LB_STRATEGY", "round_robin"),
		HealthInterval: getDuration("LB_HEALTH_INTERVAL", 2*time.Second),
		HealthTimeout:  getDuration("LB_HEALTH_TIMEOUT", 1*time.Second),
		MetricsEnabled: getBool("LB_METRICS_ENABLED", true),
		MetricsAddr:    getEnv("LB_METRICS_ADDR", ":9090"),

		RateLimitEnabled: getBool("LB_RATE_LIMIT_ENABLED", false),
		RateLimitMax:     getInt("LB_RATE_LIMIT_MAX", 100),
		RateLimitWindow:  getDuration("LB_RATE_LIMIT_WINDOW", 60*time.Second),

		RequestLimitEnabled: getBool("LB_REQUEST_LIMIT_ENABLED", false),
		MaxBodySize:         getInt64("LB_MAX_BODY_SIZE", 10*1024*1024),
		MaxHeaderSize:       getInt("LB_MAX_HEADER_SIZE", 8192),

		TLSEnabled:  getBool("LB_TLS_ENABLED", false),
		TLSCertFile: getEnv("LB_TLS_CERT_FILE", "cert.pem"),
		TLSKeyFile:  getEnv("LB_TLS_KEY_FILE", "key.pem"),
		TLSAutoGen:  getBool("LB_TLS_AUTO_GEN", true),
	}

	if len(cfg.BackendURLs) == 0 {
		log.Fatal("LB_BACKENDS cannot be empty (comma-separated list of backend URLs)")
	}

	return cfg
}

func getEnv(key, def string) string {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	return val
}

func getBool(key string, def bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return def
	}
	return b
}

func getInt(key string, def int) int {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return def
	}
	return n
}

func getInt64(key string, def int64) int64 {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return def
	}
	return n
}

func getDuration(key string, def time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		return def
	}
	return d
}

func parseCSV(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func parseIntCSV(s string) []int {
	if s == "" {
		return []int{}
	}
	parts := strings.Split(s, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			out = append(out, 1)
		} else {
			out = append(out, n)
		}
	}
	return out
}
