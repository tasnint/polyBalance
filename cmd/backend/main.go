package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

var requestCount uint64
var serverName string
var healthy int32 = 1

func main() {
	port := flag.Int("port", 8081, "Port to listen on")
	name := flag.String("name", "", "Server name (defaults to backend-<port>)")
	flag.Parse()

	if *name == "" {
		serverName = fmt.Sprintf("backend-%d", *port)
	} else {
		serverName = *name
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/readyz", handleReadyz)
	mux.HandleFunc("/toggle-health", handleToggleHealth)
	mux.HandleFunc("/", handleRoot)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("[%s] Starting backend server on %s", serverName, addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
		os.Exit(1)
	}
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	if atomic.LoadInt32(&healthy) == 1 {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("unhealthy"))
	}
}

func handleReadyz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ready"))
}

func handleToggleHealth(w http.ResponseWriter, r *http.Request) {
	current := atomic.LoadInt32(&healthy)
	if current == 1 {
		atomic.StoreInt32(&healthy, 0)
		log.Printf("[%s] Health toggled to UNHEALTHY", serverName)
		w.Write([]byte("Now unhealthy"))
	} else {
		atomic.StoreInt32(&healthy, 1)
		log.Printf("[%s] Health toggled to HEALTHY", serverName)
		w.Write([]byte("Now healthy"))
	}
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	count := atomic.AddUint64(&requestCount, 1)

	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = fmt.Sprintf("%s-%d-%d", serverName, time.Now().UnixNano(), count)
	}

	log.Printf("[%s] Request #%d: %s %s (X-Request-ID: %s)", serverName, count, r.Method, r.URL.Path, requestID)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Backend-Server", serverName)
	w.Header().Set("X-Request-ID", requestID)

	response := map[string]interface{}{
		"server":        serverName,
		"request_count": count,
		"request_id":    requestID,
		"path":          r.URL.Path,
		"method":        r.Method,
		"timestamp":     time.Now().Format(time.RFC3339),
		"message":       fmt.Sprintf("Hello from %s!", serverName),
	}

	json.NewEncoder(w).Encode(response)
}
