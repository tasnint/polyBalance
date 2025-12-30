# PolyBalance Load Balancer

## Overview
PolyBalance is a Go-based HTTP load balancer that supports multiple load balancing strategies including round-robin, least connections, latency-based, and consistent hashing. It includes rate limiting, request size limits, TLS termination support, and a web-based dashboard for monitoring and testing.

## Project Structure
```
polybalance/
├── cmd/           - Application entry point (main.go)
├── backend/       - Backend server management and health checking
├── internal/      - Configuration and logging utilities
├── metrics/       - Prometheus metrics integration
├── middleware/    - Rate limiting, request limits, TLS termination
├── proxy/         - Reverse proxy implementation
├── server/        - HTTP server with retry logic
├── strategy/      - Load balancing strategy implementations
├── ui/            - Web dashboard for monitoring and testing
├── k8/            - Kubernetes deployment manifests
├── k8s/           - Alternative K8s manifests
└── deployments/   - Docker Compose configuration
```

## Configuration
The load balancer is configured via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `LB_LISTEN_ADDR` | `:8080` | Address to listen on (set to `0.0.0.0:5000` for Replit) |
| `LB_BACKENDS` | (required) | Comma-separated list of backend URLs |
| `LB_WEIGHTS` | `1,1,...` | Comma-separated weights for backends |
| `LB_STRATEGY` | `round_robin` | Strategy: `round_robin`, `least_connections`, `latency`, `consistent_hash` |
| `LB_HEALTH_INTERVAL` | `2s` | Health check interval |
| `LB_HEALTH_TIMEOUT` | `1s` | Health check timeout |
| `LB_METRICS_ENABLED` | `true` | Enable Prometheus metrics |
| `LB_METRICS_ADDR` | `:9090` | Metrics server address |
| `LB_RATE_LIMIT_ENABLED` | `false` | Enable rate limiting |
| `LB_RATE_LIMIT_MAX` | `100` | Max requests per window |
| `LB_RATE_LIMIT_WINDOW` | `60s` | Rate limit time window |
| `LB_REQUEST_LIMIT_ENABLED` | `false` | Enable request size limits |
| `LB_MAX_BODY_SIZE` | `10485760` | Max request body size (10MB) |
| `LB_MAX_HEADER_SIZE` | `8192` | Max header size (8KB) |
| `LB_TLS_ENABLED` | `false` | Enable TLS termination |
| `LB_TLS_CERT_FILE` | `cert.pem` | TLS certificate file |
| `LB_TLS_KEY_FILE` | `key.pem` | TLS private key file |
| `LB_TLS_AUTO_GEN` | `true` | Auto-generate self-signed cert |

## Endpoints
- `/` - Proxied requests to backends
- `/healthz` - Health check endpoint
- `/readyz` - Readiness check endpoint
- `/ui` - Web dashboard for monitoring and testing

## Web Dashboard
Access the dashboard at `/ui` to:
- View system status (uptime, strategy, backend counts)
- Monitor backend server health in real-time
- **Toggle backend health** with one-click buttons (Make Unhealthy/Make Healthy)
- Enable/disable rate limiting with configurable limits
- Enable/disable request size limits
- Run diagnostic tests (health checks, rate limit tests, connection tests)
- Test endpoints directly from the UI

## Running Locally
```bash
go run ./cmd
```

## Load Balancing Strategies
1. **Round Robin** - Distributes requests evenly across all backends
2. **Least Connections** - Routes to backend with fewest active connections
3. **Latency** - Routes to backend with lowest response latency
4. **Consistent Hash** - Routes based on request hash for session affinity

## Features
- **Rate Limiting** - IP-based rate limiting with configurable requests/window
- **Request Limits** - Body and header size limits to prevent abuse
- **TLS Termination** - HTTPS support with auto-generated or custom certificates
- **Circuit Breaker** - Automatic backend removal on repeated failures
- **Health Checks** - Continuous backend health monitoring
- **Prometheus Metrics** - Built-in metrics endpoint for monitoring

## Running with Backend Servers
The project includes demo backend servers that respond to /healthz and regular requests:
```bash
bash start.sh
```
This starts:
- 2 backend servers on ports 8081 and 8082
- The load balancer on port 5000

## Testing Health-Aware Load Balancing
1. Visit `/ui` to see the dashboard with healthy backends
2. Click the "Make Unhealthy" button next to any backend
3. Watch the dashboard - the backend will show as unhealthy after the next health check
4. Requests will only go to healthy backends (test with the "Test /" button)
5. Click "Make Healthy" to restore the backend - it will re-enter rotation after ~10 seconds (circuit breaker cooldown)

## Recent Changes
- 2025-12-30: Added one-click toggle buttons in UI for backend health testing
- 2025-12-30: Fixed critical bug - server and health checker now share same backend instances
- 2025-12-30: Added demo backend servers with /healthz and /toggle-health endpoints
- 2025-12-30: Added request ID headers (X-Request-ID) to all proxied requests
- 2025-12-30: Added logging for which backend handles each request
- 2025-12-30: Added web UI dashboard for monitoring and testing
- 2025-12-30: Added rate limiting middleware
- 2025-12-30: Added request size limits middleware
- 2025-12-30: Added TLS termination support
- 2025-12-30: Configured for Replit environment (port 5000)
