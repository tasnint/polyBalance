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
}

// LoadConfig loads configuration from environment variables.
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
	}

	if len(cfg.BackendURLs) == 0 {
		log.Fatal("LB_BACKENDS cannot be empty (comma-separated list of backend URLs)")
	}

	return cfg
}

// --- helpers ---

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
			out = append(out, 1) // default weight
		} else {
			out = append(out, n)
		}
	}
	return out
}
