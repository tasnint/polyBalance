# PolyBalance - Tanisha's 7 Layer Load Balancer

A Go-based HTTP load balancer that supports multiple load balancing strategies including round-robin, least connections, latency-based, and consistent hashing. It includes rate limiting, request size limits, TLS termination support, and a web-based dashboard for monitoring and testing.

## Features

- **Multiple Load Balancing Strategies**: Round-robin, least connections, latency-based, consistent hashing
- **Health Checks**: Automatic backend health monitoring with circuit breaker pattern
- **Rate Limiting**: IP-based rate limiting with configurable requests per window
- **Request Limits**: Body and header size limits to prevent abuse
- **TLS Termination**: HTTPS support with auto-generated or custom certificates
- **Web Dashboard**: Real-time monitoring and testing interface
- **Prometheus Metrics**: Built-in metrics endpoint for monitoring

## Quick Start (Windows PowerShell)

### Prerequisites

1. **Install Go**: Download from https://go.dev/dl/ and install
2. **Restart PowerShell** after installation
3. Verify installation: `go version`

### Running the Load Balancer

Open PowerShell in the project directory and run:

```powershell
# Get the current directory
$dir = Get-Location

# Set environment variables
$env:LB_BACKENDS = "http://localhost:8081,http://localhost:8082,http://localhost:8083,http://localhost:8084,http://localhost:8085"
$env:LB_LISTEN_ADDR = ":8080"

# Start 5 backend servers in new windows
Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd '$dir'; go run ./cmd/backend -port 8081 -name backend-1"
Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd '$dir'; go run ./cmd/backend -port 8082 -name backend-2"
Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd '$dir'; go run ./cmd/backend -port 8083 -name backend-3"
Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd '$dir'; go run ./cmd/backend -port 8084 -name backend-4"
Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd '$dir'; go run ./cmd/backend -port 8085 -name backend-5"

# Wait for backends to start
Start-Sleep -Seconds 5

# Start the load balancer
go run ./cmd
```

### Accessing the Dashboard

Open your browser and navigate to:

```
http://localhost:8080/ui
```

## Quick Start (Linux/macOS/Bash)

```bash
# Start the load balancer with demo backends
bash start.sh
```

Then open: `http://localhost:5000/ui`

## Configuration

The load balancer is configured via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `LB_LISTEN_ADDR` | `:8080` | Address to listen on |
| `LB_BACKENDS` | (required) | Comma-separated list of backend URLs |
| `LB_WEIGHTS` | `1,1,...` | Comma-separated weights for backends |
| `LB_STRATEGY` | `round_robin` | Strategy: `round_robin`, `least_connections`, `latency`, `consistent_hash` |
| `LB_HEALTH_INTERVAL` | `2s` | Health check interval |
| `LB_HEALTH_TIMEOUT` | `1s` | Health check timeout |
| `LB_RATE_LIMIT_ENABLED` | `false` | Enable rate limiting |
| `LB_RATE_LIMIT_MAX` | `100` | Max requests per window |
| `LB_RATE_LIMIT_WINDOW` | `60s` | Rate limit time window |

## Web Dashboard Features

- **System Status**: View uptime, strategy, and backend counts
- **Backend Management**: Toggle backend health for testing failover
- **Rate Limiting**: Enable/disable and configure rate limits
- **Request Limits**: Configure body and header size limits
- **Load Testing**: Send multiple requests to test load distribution
- **Diagnostics**: Test health endpoints and connections

## Testing Health-Aware Load Balancing

1. Open the dashboard at `/ui`
2. Click "Make Unhealthy" next to any backend
3. The backend will show as unhealthy after the next health check
4. Use "Send Requests" button to see traffic only goes to healthy backends
5. Click "Make Healthy" to restore - wait ~10 seconds for circuit breaker cooldown

## Load Balancing Strategies

1. **Round Robin** (`round_robin`) - Distributes requests evenly across all backends
2. **Least Connections** (`least_connections`) - Routes to backend with fewest active connections
3. **Latency** (`latency`) - Routes to backend with lowest response latency
4. **Consistent Hash** (`consistent_hash`) - Routes based on request hash for session affinity

## API Endpoints

- `/` - Proxied requests to backends
- `/healthz` - Health check endpoint
- `/readyz` - Readiness check endpoint
- `/ui` - Web dashboard

## Stopping the Load Balancer

### Windows PowerShell

Press `Ctrl+C` in the main terminal, then close all the backend server windows.

Or run:
```powershell
Get-Process | Where-Object { $_.ProcessName -like "*go*" } | Stop-Process -Force
```

### Linux/macOS

Press `Ctrl+C` in the terminal, then:
```bash
pkill -f "go run ./cmd"
```

## Project Structure

```
polybalance/
├── cmd/           - Application entry points
│   ├── main.go    - Load balancer main
│   └── backend/   - Demo backend server
├── backend/       - Backend server management and health checking
├── internal/      - Configuration and logging utilities
├── metrics/       - Prometheus metrics integration
├── middleware/    - Rate limiting, request limits, TLS termination
├── proxy/         - Reverse proxy implementation
├── server/        - HTTP server with retry logic
├── strategy/      - Load balancing strategy implementations
└── ui/            - Web dashboard
```

## License

MIT License
