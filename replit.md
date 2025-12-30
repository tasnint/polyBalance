# PolyBalance Load Balancer

## Overview
PolyBalance is a Go-based HTTP load balancer that supports multiple load balancing strategies including round-robin, least connections, latency-based, and consistent hashing.

## Project Structure
```
polybalance/
├── cmd/           - Application entry point (main.go)
├── backend/       - Backend server management and health checking
├── internal/      - Configuration and logging utilities
├── metrics/       - Prometheus metrics integration
├── proxy/         - Reverse proxy implementation
├── server/        - HTTP server with retry logic
├── strategy/      - Load balancing strategy implementations
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

## Endpoints
- `/` - Proxied requests to backends
- `/healthz` - Health check endpoint
- `/readyz` - Readiness check endpoint

## Running Locally
```bash
go run ./cmd
```

## Load Balancing Strategies
1. **Round Robin** - Distributes requests evenly across all backends
2. **Least Connections** - Routes to backend with fewest active connections
3. **Latency** - Routes to backend with lowest response latency
4. **Consistent Hash** - Routes based on request hash for session affinity

## Recent Changes
- 2025-12-30: Configured for Replit environment (port 5000)
