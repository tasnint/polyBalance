package main

import (
        "context"
        "log"
        "net/http"
        "os"
        "os/signal"
        "syscall"
        "time"

        "flag"
        "polybalance/backend"
        "polybalance/internal"
        "polybalance/metrics"
        "polybalance/middleware"
        "polybalance/proxy"
        "polybalance/server"
        "polybalance/strategy"
        "polybalance/ui"
)

func main() {

        strategyFlag := flag.String("strategy", "", "Load balancing strategy (round_robin, least_connections, latency, consistent_hash)")

        flag.Parse()
        // ------------------------------
        // 1) Load configuration
        // ------------------------------
        cfg := internal.LoadConfig()
        logger := internal.NewLogger("MAIN")

        // Override strategy if CLI flag provided
        if *strategyFlag != "" {
                cfg.Strategy = *strategyFlag
        }

        logger.Info("Starting PolyBalance Load Balancer...")
        logger.Info("Using strategy: %s", cfg.Strategy)

        // ------------------------------
        // 2) Create backend objects
        // ------------------------------
        backends := make([]*backend.Backend, 0, len(cfg.BackendURLs))

        for i, rawURL := range cfg.BackendURLs {
                rp := proxy.NewReverseProxy(rawURL)
                weight := 1

                if i < len(cfg.Weights) {
                        weight = cfg.Weights[i]
                }

                b, err := backend.NewBackend(rawURL, weight, rp)
                if err != nil {
                        logger.Error("Failed to create backend: %v", err)
                        continue
                }

                backends = append(backends, b)
        }

        if len(backends) == 0 {
                log.Fatal("No valid backends available — shutting down.")
        }

        // ------------------------------
        // 3) Select strategy
        // ------------------------------
        var strat strategy.Strategy

        switch cfg.Strategy {
        case "round_robin":
                strat = strategy.NewRoundRobin()
        case "least_connections":
                strat = strategy.NewLeastConnections()
        case "latency":
                strat = strategy.NewLatencyStrategy()
        case "consistent_hash":
                strat = strategy.NewConsistentHash(50)
        default:
                logger.Info("Unknown strategy '%s', defaulting to round_robin", cfg.Strategy)
                strat = strategy.NewRoundRobin()
        }

        // ------------------------------
        // 4) Create HTTP server wrapper
        // ------------------------------
        lbServer, err := server.NewServer(cfg.BackendURLs, cfg.Weights, strat)
        if err != nil {
                logger.Error("Failed to create load balancer server: %v", err)
                return
        }

        // ------------------------------
        // 5) Initialize Middleware
        // ------------------------------
        rateLimiter := middleware.NewRateLimiter(
                cfg.RateLimitMax,
                cfg.RateLimitWindow,
                cfg.RateLimitEnabled,
        )
        logger.Info("Rate limiter initialized (enabled=%v, max=%d, window=%v)",
                cfg.RateLimitEnabled, cfg.RateLimitMax, cfg.RateLimitWindow)

        requestLimiter := middleware.NewRequestLimiter(
                cfg.MaxBodySize,
                cfg.MaxHeaderSize,
                cfg.RequestLimitEnabled,
        )
        logger.Info("Request limiter initialized (enabled=%v, maxBody=%d, maxHeader=%d)",
                cfg.RequestLimitEnabled, cfg.MaxBodySize, cfg.MaxHeaderSize)

        tlsConfig := middleware.NewTLSConfig(
                cfg.TLSEnabled,
                cfg.TLSCertFile,
                cfg.TLSKeyFile,
                cfg.TLSAutoGen,
        )
        logger.Info("TLS config initialized (enabled=%v, autoGen=%v)", cfg.TLSEnabled, cfg.TLSAutoGen)

        // ------------------------------
        // 6) Start Health Checker
        // ------------------------------
        ctx, cancel := context.WithCancel(context.Background())
        hc := backend.NewHealthChecker(
                backends,
                cfg.HealthInterval,
                cfg.HealthTimeout,
                "/healthz",
        )
        hc.Start(ctx)
        logger.Info("Health checker initialized.")

        // ------------------------------
        // 7) Start Metrics Server (optional)
        // ------------------------------
        if cfg.MetricsEnabled {
                go func() {
                        logger.Info("Starting Prometheus metrics server on %s", cfg.MetricsAddr)
                        if err := metrics.StartMetricsServer(cfg.MetricsAddr); err != nil {
                                logger.Error("Metrics server error: %v", err)
                        }
                }()
        }

        // ------------------------------
        // 8) Create Dashboard
        // ------------------------------
        dashboard := ui.NewDashboard(backends, rateLimiter, requestLimiter, tlsConfig, cfg.Strategy)

        // ------------------------------
        // 9) Start Main Load Balancer Server
        // ------------------------------
        go func() {
                mux := http.NewServeMux()

                lbServer.RegisterHealthEndpoints(mux)

                dashboard.RegisterRoutes(mux)

                mux.Handle("/", lbServer)

                var handler http.Handler = mux
                handler = requestLimiter.Middleware(handler)
                handler = rateLimiter.Middleware(handler)

                if cfg.TLSEnabled {
                        tlsCfg, err := tlsConfig.GetTLSConfig()
                        if err != nil {
                                logger.Error("Failed to load TLS config: %v", err)
                                cancel()
                                return
                        }

                        server := &http.Server{
                                Addr:      cfg.ListenAddr,
                                Handler:   handler,
                                TLSConfig: tlsCfg,
                        }

                        logger.Info("Load balancer listening on %s (TLS enabled)", cfg.ListenAddr)
                        if err := server.ListenAndServeTLS("", ""); err != nil {
                                logger.Error("HTTPS server stopped: %v", err)
                        }
                } else {
                        logger.Info("Load balancer listening on %s", cfg.ListenAddr)
                        if err := http.ListenAndServe(cfg.ListenAddr, handler); err != nil {
                                logger.Error("HTTP server stopped: %v", err)
                        }
                }
                cancel()
        }()

        // ------------------------------
        // 10) Graceful shutdown on CTRL+C
        // ------------------------------
        sig := make(chan os.Signal, 1)
        signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

        <-sig
        logger.Info("Shutting down load balancer...")
        cancel()
        time.Sleep(500 * time.Millisecond)
}
