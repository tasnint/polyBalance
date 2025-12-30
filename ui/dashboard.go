package ui

import (
        "encoding/json"
        "html/template"
        "net/http"
        "polybalance/backend"
        "polybalance/middleware"
        "polybalance/server"
        "strconv"
        "time"
)

type Dashboard struct {
        backends           []*backend.Backend
        rateLimiter        *middleware.RateLimiter
        requestLimiter     *middleware.RequestLimiter
        tlsConfig          *middleware.TLSConfig
        strategyController *server.StrategyController
        startTime          time.Time
        requestCount       int64
        errorCount         int64
}

func NewDashboard(backends []*backend.Backend, rl *middleware.RateLimiter, reqLim *middleware.RequestLimiter, tlsCfg *middleware.TLSConfig, stratCtrl *server.StrategyController) *Dashboard {
        return &Dashboard{
                backends:           backends,
                rateLimiter:        rl,
                requestLimiter:     reqLim,
                tlsConfig:          tlsCfg,
                strategyController: stratCtrl,
                startTime:          time.Now(),
        }
}

func (d *Dashboard) RegisterRoutes(mux *http.ServeMux) {
        mux.HandleFunc("/ui", d.handleDashboard)
        mux.HandleFunc("/ui/", d.handleDashboard)
        mux.HandleFunc("/api/status", d.handleStatus)
        mux.HandleFunc("/api/backends", d.handleBackends)
        mux.HandleFunc("/api/backends/toggle", d.handleToggleBackend)
        mux.HandleFunc("/api/config", d.handleConfig)
        mux.HandleFunc("/api/test", d.handleTest)
        mux.HandleFunc("/api/send-requests", d.handleSendRequests)
        mux.HandleFunc("/api/strategy", d.handleStrategy)
}

func (d *Dashboard) handleDashboard(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

        tmpl := template.Must(template.New("dashboard").Parse(dashboardHTML))
        tmpl.Execute(w, nil)
}

func (d *Dashboard) handleStatus(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Header().Set("Cache-Control", "no-cache")

        healthyCount := 0
        for _, b := range d.backends {
                if b.IsAlive() {
                        healthyCount++
                }
        }

        status := map[string]interface{}{
                "uptime_seconds":   int(time.Since(d.startTime).Seconds()),
                "strategy":         d.strategyController.Name(),
                "strategies":       d.strategyController.AvailableStrategies(),
                "total_backends":   len(d.backends),
                "healthy_backends": healthyCount,
                "rate_limit":       d.rateLimiter.GetStats(),
                "request_limit":    d.requestLimiter.GetStats(),
                "tls":              d.tlsConfig.GetStats(),
        }

        json.NewEncoder(w).Encode(status)
}

func (d *Dashboard) handleBackends(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Header().Set("Cache-Control", "no-cache")

        backends := make([]map[string]interface{}, 0, len(d.backends))
        for i, b := range d.backends {
                backends = append(backends, map[string]interface{}{
                        "id":          i,
                        "url":         b.URL.String(),
                        "healthy":     b.IsAlive(),
                        "weight":      b.Weight,
                        "connections": b.GetActiveConnections(),
                })
        }

        json.NewEncoder(w).Encode(backends)
}

func (d *Dashboard) handleToggleBackend(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")

        if r.Method != http.MethodPost {
                http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
                return
        }

        backendURL := r.URL.Query().Get("url")
        if backendURL == "" {
                json.NewEncoder(w).Encode(map[string]interface{}{
                        "status":  "error",
                        "message": "Backend URL required",
                })
                return
        }

        client := &http.Client{Timeout: 5 * time.Second}
        resp, err := client.Get(backendURL + "/toggle-health")
        if err != nil {
                json.NewEncoder(w).Encode(map[string]interface{}{
                        "status":  "error",
                        "message": "Failed to toggle: " + err.Error(),
                })
                return
        }
        defer resp.Body.Close()

        var result string
        buf := make([]byte, 256)
        n, _ := resp.Body.Read(buf)
        result = string(buf[:n])

        json.NewEncoder(w).Encode(map[string]interface{}{
                "status":  "ok",
                "message": result,
        })
}

func (d *Dashboard) handleSendRequests(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")

        countStr := r.URL.Query().Get("count")
        count := 10
        if countStr != "" {
                if val, err := strconv.Atoi(countStr); err == nil && val > 0 && val <= 100 {
                        count = val
                }
        }

        host := r.Host
        scheme := "http"
        if r.TLS != nil {
                scheme = "https"
        }
        baseURL := scheme + "://" + host

        results := make([]map[string]interface{}, 0, count)
        client := &http.Client{Timeout: 5 * time.Second}

        for i := 0; i < count; i++ {
                start := time.Now()
                resp, err := client.Get(baseURL + "/")
                elapsed := time.Since(start)

                result := map[string]interface{}{
                        "request_num": i + 1,
                        "elapsed_ms":  elapsed.Milliseconds(),
                }

                if err != nil {
                        result["status"] = "error"
                        result["error"] = err.Error()
                } else {
                        result["status_code"] = resp.StatusCode
                        var data map[string]interface{}
                        json.NewDecoder(resp.Body).Decode(&data)
                        resp.Body.Close()
                        if server, ok := data["server"]; ok {
                                result["backend"] = server
                        }
                        result["status"] = "ok"
                }
                results = append(results, result)
        }

        json.NewEncoder(w).Encode(map[string]interface{}{
                "status":   "ok",
                "count":    count,
                "requests": results,
        })
}

func (d *Dashboard) handleConfig(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodPost {
                if err := r.ParseForm(); err != nil {
                        http.Error(w, err.Error(), http.StatusBadRequest)
                        return
                }

                if rl := r.FormValue("rate_limit_enabled"); rl != "" {
                        d.rateLimiter.SetEnabled(rl == "true")
                }
                if maxReq := r.FormValue("rate_limit_max"); maxReq != "" {
                        if val, err := strconv.Atoi(maxReq); err == nil {
                                window := 60 * time.Second
                                if w := r.FormValue("rate_limit_window"); w != "" {
                                        if secs, err := strconv.Atoi(w); err == nil {
                                                window = time.Duration(secs) * time.Second
                                        }
                                }
                                d.rateLimiter.SetLimits(val, window)
                        }
                }

                if rl := r.FormValue("request_limit_enabled"); rl != "" {
                        d.requestLimiter.SetEnabled(rl == "true")
                }
                if maxBody := r.FormValue("max_body_size"); maxBody != "" {
                        if val, err := strconv.ParseInt(maxBody, 10, 64); err == nil {
                                maxHeader := 8192
                                if mh := r.FormValue("max_header_size"); mh != "" {
                                        if v, err := strconv.Atoi(mh); err == nil {
                                                maxHeader = v
                                        }
                                }
                                d.requestLimiter.SetLimits(val, maxHeader)
                        }
                }

                w.Header().Set("Content-Type", "application/json")
                w.Write([]byte(`{"status": "ok"}`))
                return
        }

        w.Header().Set("Content-Type", "application/json")
        config := map[string]interface{}{
                "rate_limit":    d.rateLimiter.GetStats(),
                "request_limit": d.requestLimiter.GetStats(),
                "tls":           d.tlsConfig.GetStats(),
                "strategy":      d.strategyController.Name(),
        }
        json.NewEncoder(w).Encode(config)
}

func (d *Dashboard) handleStrategy(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")

        if r.Method == http.MethodPost {
                if err := r.ParseForm(); err != nil {
                        http.Error(w, err.Error(), http.StatusBadRequest)
                        return
                }

                newStrategy := r.FormValue("strategy")
                if newStrategy == "" {
                        json.NewEncoder(w).Encode(map[string]interface{}{
                                "status":  "error",
                                "message": "Strategy name required",
                        })
                        return
                }

                if d.strategyController.Set(newStrategy) {
                        json.NewEncoder(w).Encode(map[string]interface{}{
                                "status":   "ok",
                                "strategy": newStrategy,
                                "message":  "Strategy changed to " + newStrategy,
                        })
                } else {
                        json.NewEncoder(w).Encode(map[string]interface{}{
                                "status":  "error",
                                "message": "Invalid strategy: " + newStrategy,
                        })
                }
                return
        }

        json.NewEncoder(w).Encode(map[string]interface{}{
                "strategy":   d.strategyController.Name(),
                "strategies": d.strategyController.AvailableStrategies(),
        })
}

func (d *Dashboard) handleTest(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")

        testType := r.URL.Query().Get("type")
        result := map[string]interface{}{
                "test_type": testType,
                "timestamp": time.Now().Format(time.RFC3339),
        }

        switch testType {
        case "health":
                healthy := 0
                unhealthy := 0
                for _, b := range d.backends {
                        if b.IsAlive() {
                                healthy++
                        } else {
                                unhealthy++
                        }
                }
                result["healthy"] = healthy
                result["unhealthy"] = unhealthy
                result["status"] = "ok"

        case "ratelimit":
                testIP := "test-" + time.Now().Format("150405")
                allowed := 0
                blocked := 0
                for i := 0; i < 20; i++ {
                        if d.rateLimiter.Allow(testIP) {
                                allowed++
                        } else {
                                blocked++
                        }
                }
                result["allowed"] = allowed
                result["blocked"] = blocked
                result["status"] = "ok"

        case "connection":
                for _, b := range d.backends {
                        if b.IsAlive() {
                                result["status"] = "ok"
                                result["message"] = "At least one backend is healthy"
                                json.NewEncoder(w).Encode(result)
                                return
                        }
                }
                result["status"] = "error"
                result["message"] = "No healthy backends available"

        default:
                result["status"] = "ok"
                result["message"] = "Load balancer is running"
        }

        json.NewEncoder(w).Encode(result)
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>PolyBalance - Tanisha's Load Balancer</title>
    <link href="https://fonts.googleapis.com/css2?family=Poppins:wght@400;600;700&family=Roboto+Mono&display=swap" rel="stylesheet">
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: 'Poppins', -apple-system, BlinkMacSystemFont, sans-serif;
            background: #f5f7fa;
            color: #333;
            min-height: 100vh;
            padding: 20px;
        }
        .container { max-width: 1400px; margin: 0 auto; }
        .header {
            text-align: center;
            margin-bottom: 30px;
            padding: 20px;
        }
        .logo {
            font-size: 3rem;
            font-weight: 700;
            margin-bottom: 5px;
        }
        .logo-poly { color: #5DADE2; }
        .logo-balance { color: #2C3E50; font-weight: 400; }
        .subtitle {
            font-size: 1.25rem;
            color: #2C3E50;
            font-weight: 600;
        }
        .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(380px, 1fr)); gap: 20px; }
        .card {
            background: #fff;
            border-radius: 12px;
            padding: 24px;
            box-shadow: 0 2px 12px rgba(0,0,0,0.08);
            border: 1px solid #e8ecf1;
        }
        .card h2 {
            font-size: 1.1rem;
            margin-bottom: 16px;
            color: #2C3E50;
            display: flex;
            align-items: center;
            gap: 10px;
            font-weight: 600;
        }
        .card h2::before {
            content: '';
            width: 8px;
            height: 8px;
            background: #5DADE2;
            border-radius: 50%;
        }
        .status-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 12px; }
        .status-item {
            background: #f8fafc;
            padding: 16px;
            border-radius: 10px;
            text-align: center;
            border: 1px solid #e8ecf1;
        }
        .status-value {
            font-size: 1.75rem;
            font-weight: 700;
            color: #5DADE2;
        }
        .status-label { font-size: 0.8rem; color: #7f8c8d; margin-top: 4px; }
        .backend-list { display: flex; flex-direction: column; gap: 8px; max-height: 300px; overflow-y: auto; }
        .backend-item {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 10px 14px;
            background: #f8fafc;
            border-radius: 8px;
            border: 1px solid #e8ecf1;
        }
        .backend-url { font-family: 'Roboto Mono', monospace; font-size: 0.85rem; color: #2C3E50; }
        .badge {
            padding: 4px 10px;
            border-radius: 20px;
            font-size: 0.7rem;
            font-weight: 600;
            text-transform: uppercase;
        }
        .badge.healthy { background: #d4edda; color: #155724; }
        .badge.unhealthy { background: #f8d7da; color: #721c24; }
        .controls { display: flex; flex-direction: column; gap: 14px; }
        .control-group label { display: block; margin-bottom: 6px; font-size: 0.85rem; color: #7f8c8d; font-weight: 500; }
        .control-row { display: flex; gap: 10px; align-items: center; }
        input[type="number"], input[type="text"], select {
            background: #f8fafc;
            border: 1px solid #dce4ec;
            border-radius: 6px;
            padding: 10px 12px;
            color: #2C3E50;
            width: 100%;
            font-size: 0.95rem;
            font-family: inherit;
        }
        input:focus, select:focus { outline: none; border-color: #5DADE2; }
        .btn {
            padding: 10px 20px;
            border: none;
            border-radius: 6px;
            cursor: pointer;
            font-weight: 600;
            font-size: 0.85rem;
            transition: all 0.2s;
            font-family: inherit;
        }
        .btn-primary {
            background: #5DADE2;
            color: #fff;
        }
        .btn-primary:hover { background: #3498db; transform: translateY(-1px); }
        .btn-secondary {
            background: #f8fafc;
            color: #2C3E50;
            border: 1px solid #dce4ec;
        }
        .btn-secondary:hover { background: #e8ecf1; }
        .btn-danger { background: #f8d7da; color: #721c24; }
        .btn-danger:hover { background: #f5c6cb; }
        .btn-success { background: #d4edda; color: #155724; }
        .btn-success:hover { background: #c3e6cb; }
        .test-buttons { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 14px; }
        .test-output {
            background: #2C3E50;
            border-radius: 8px;
            padding: 14px;
            font-family: 'Roboto Mono', monospace;
            font-size: 0.8rem;
            min-height: 180px;
            max-height: 300px;
            overflow-y: auto;
            white-space: pre-wrap;
            color: #ecf0f1;
        }
        .toggle {
            position: relative;
            width: 46px;
            height: 24px;
            background: #dce4ec;
            border-radius: 12px;
            cursor: pointer;
            transition: background 0.3s;
        }
        .toggle.active { background: #5DADE2; }
        .toggle::after {
            content: '';
            position: absolute;
            width: 20px;
            height: 20px;
            background: #fff;
            border-radius: 50%;
            top: 2px;
            left: 2px;
            transition: transform 0.3s;
            box-shadow: 0 1px 3px rgba(0,0,0,0.2);
        }
        .toggle.active::after { transform: translateX(22px); }
        .log-entry { margin-bottom: 6px; padding-bottom: 6px; border-bottom: 1px solid rgba(255,255,255,0.1); }
        .log-entry.success { color: #2ecc71; }
        .log-entry.error { color: #e74c3c; }
        .log-entry.info { color: #5DADE2; }
        .refresh-btn {
            position: fixed;
            bottom: 20px;
            right: 20px;
            width: 50px;
            height: 50px;
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 1.3rem;
            box-shadow: 0 4px 12px rgba(93,173,226,0.4);
        }
        .request-tester {
            display: flex;
            gap: 10px;
            align-items: center;
            margin-bottom: 14px;
            flex-wrap: wrap;
        }
        .request-tester input {
            width: 80px;
        }
        .note {
            font-size: 0.75rem;
            color: #7f8c8d;
            margin-top: 8px;
            font-style: italic;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="logo">
                <span class="logo-poly">Poly</span><span class="logo-balance">Balance</span>
            </div>
            <div class="subtitle">Tanisha's 7 Layer Load Balancer</div>
        </div>
        
        <div class="grid">
            <div class="card">
                <h2>System Status</h2>
                <div class="status-grid">
                    <div class="status-item">
                        <div class="status-value" id="uptime">0</div>
                        <div class="status-label">Uptime (seconds)</div>
                    </div>
                    <div class="status-item">
                        <select id="strategy" class="status-value" style="font-size:1rem;padding:8px;text-align:center;" onchange="changeStrategy(this.value)">
                            <option value="round_robin">Round Robin</option>
                            <option value="least_connections">Least Connections</option>
                            <option value="latency">Latency</option>
                            <option value="consistent_hash">Consistent Hash</option>
                        </select>
                        <div class="status-label">Strategy (click to change)</div>
                    </div>
                    <div class="status-item">
                        <div class="status-value" id="healthy-count">0</div>
                        <div class="status-label">Healthy Backends</div>
                    </div>
                    <div class="status-item">
                        <div class="status-value" id="total-count">0</div>
                        <div class="status-label">Total Backends</div>
                    </div>
                </div>
            </div>

            <div class="card">
                <h2>Backend Servers</h2>
                <div class="backend-list" id="backends">
                    <div class="backend-item">Loading...</div>
                </div>
                <p class="note">Note: After toggling to healthy, wait ~10 seconds for the circuit breaker to allow traffic again.</p>
            </div>

            <div class="card">
                <h2>Rate Limiting</h2>
                <div class="controls">
                    <div class="control-group">
                        <label>Enable Rate Limiting</label>
                        <div class="toggle" id="rate-limit-toggle" onclick="toggleRateLimit()"></div>
                    </div>
                    <div class="control-group">
                        <label>Max Requests per Window</label>
                        <div class="control-row">
                            <input type="number" id="rate-limit-max" value="100" min="1">
                            <span>requests</span>
                        </div>
                    </div>
                    <div class="control-group">
                        <label>Time Window (seconds)</label>
                        <div class="control-row">
                            <input type="number" id="rate-limit-window" value="60" min="1">
                            <span>seconds</span>
                        </div>
                    </div>
                    <button class="btn btn-primary" onclick="updateRateLimit()">Apply Rate Limit Settings</button>
                </div>
            </div>

            <div class="card">
                <h2>Request Limits</h2>
                <div class="controls">
                    <div class="control-group">
                        <label>Enable Request Limits</label>
                        <div class="toggle" id="request-limit-toggle" onclick="toggleRequestLimit()"></div>
                    </div>
                    <div class="control-group">
                        <label>Max Body Size (bytes)</label>
                        <input type="number" id="max-body-size" value="10485760" min="1024">
                    </div>
                    <div class="control-group">
                        <label>Max Header Size (bytes)</label>
                        <input type="number" id="max-header-size" value="8192" min="256">
                    </div>
                    <button class="btn btn-primary" onclick="updateRequestLimit()">Apply Request Limit Settings</button>
                </div>
            </div>

            <div class="card" style="grid-column: 1 / -1;">
                <h2>Load Balancer Testing</h2>
                <div class="request-tester">
                    <label>Send</label>
                    <input type="number" id="request-count" value="10" min="1" max="100">
                    <label>requests to test load balancing</label>
                    <button class="btn btn-primary" onclick="sendTestRequests()">Send Requests</button>
                </div>
                <div class="test-buttons">
                    <button class="btn btn-secondary" onclick="runTest('health')">Test Health Check</button>
                    <button class="btn btn-secondary" onclick="runTest('ratelimit')">Test Rate Limiting</button>
                    <button class="btn btn-secondary" onclick="runTest('connection')">Test Connection</button>
                    <button class="btn btn-secondary" onclick="testEndpoint('/healthz')">Hit /healthz</button>
                    <button class="btn btn-secondary" onclick="testEndpoint('/readyz')">Hit /readyz</button>
                    <button class="btn btn-secondary" onclick="testEndpoint('/')">Hit / (Proxy)</button>
                    <button class="btn btn-danger" onclick="clearLogs()">Clear Logs</button>
                </div>
                <div class="test-output" id="test-output">Ready for testing...</div>
            </div>
        </div>
    </div>

    <button class="btn btn-primary refresh-btn" onclick="refreshAll()">↻</button>

    <script>
        let rateLimitEnabled = false;
        let requestLimitEnabled = false;

        function log(message, type = 'info') {
            const output = document.getElementById('test-output');
            const timestamp = new Date().toLocaleTimeString();
            const entry = document.createElement('div');
            entry.className = 'log-entry ' + type;
            entry.textContent = '[' + timestamp + '] ' + message;
            output.insertBefore(entry, output.firstChild);
        }

        function clearLogs() {
            document.getElementById('test-output').innerHTML = 'Logs cleared.';
        }

        async function fetchStatus() {
            try {
                const res = await fetch('/api/status');
                const data = await res.json();
                
                document.getElementById('uptime').textContent = data.uptime_seconds;
                document.getElementById('strategy').value = data.strategy;
                document.getElementById('healthy-count').textContent = data.healthy_backends;
                document.getElementById('total-count').textContent = data.total_backends;

                rateLimitEnabled = data.rate_limit.enabled;
                requestLimitEnabled = data.request_limit.enabled;
                
                document.getElementById('rate-limit-toggle').classList.toggle('active', rateLimitEnabled);
                document.getElementById('request-limit-toggle').classList.toggle('active', requestLimitEnabled);
                
                document.getElementById('rate-limit-max').value = data.rate_limit.max_requests;
                document.getElementById('rate-limit-window').value = data.rate_limit.window_secs;
                document.getElementById('max-body-size').value = data.request_limit.max_body_bytes;
                document.getElementById('max-header-size').value = data.request_limit.max_header_size;
            } catch (e) {
                log('Failed to fetch status: ' + e.message, 'error');
            }
        }

        async function fetchBackends() {
            try {
                const res = await fetch('/api/backends');
                const backends = await res.json();
                
                const container = document.getElementById('backends');
                container.innerHTML = backends.map(b => 
                    '<div class="backend-item">' +
                    '<span class="backend-url">' + b.url + '</span>' +
                    '<div style="display:flex;align-items:center;gap:8px;">' +
                    '<span class="badge ' + (b.healthy ? 'healthy' : 'unhealthy') + '">' + 
                    (b.healthy ? 'Healthy' : 'Unhealthy') + '</span>' +
                    '<span style="color:#7f8c8d;font-size:0.75rem">(' + b.connections + ' conn)</span>' +
                    '<button class="btn ' + (b.healthy ? 'btn-danger' : 'btn-success') + '" ' +
                    'onclick="toggleBackendHealth(\'' + b.url + '\')" ' +
                    'style="padding:5px 10px;font-size:0.7rem;">' +
                    (b.healthy ? 'Make Unhealthy' : 'Make Healthy') + '</button>' +
                    '</div>' +
                    '</div>'
                ).join('');
            } catch (e) {
                log('Failed to fetch backends: ' + e.message, 'error');
            }
        }

        async function toggleBackendHealth(url) {
            try {
                log('Toggling health for ' + url + '...', 'info');
                const res = await fetch('/api/backends/toggle?url=' + encodeURIComponent(url), { method: 'POST' });
                const data = await res.json();
                if (data.message && data.message.includes('healthy')) {
                    log('Toggle result: ' + data.message + ' (wait ~10s for circuit breaker)', data.status === 'ok' ? 'success' : 'error');
                } else {
                    log('Toggle result: ' + data.message, data.status === 'ok' ? 'success' : 'error');
                }
                setTimeout(() => {
                    fetchBackends();
                    fetchStatus();
                }, 500);
            } catch (e) {
                log('Toggle failed: ' + e.message, 'error');
            }
        }

        async function sendTestRequests() {
            const count = parseInt(document.getElementById('request-count').value) || 10;
            const strategy = document.getElementById('strategy').value;
            const timestamp = new Date().toLocaleTimeString();
            
            log('[' + timestamp + '] Sending ' + count + ' requests using ' + strategy + ' strategy...', 'info');
            
            try {
                const res = await fetch('/api/send-requests?count=' + count);
                const data = await res.json();
                
                if (data.requests) {
                    const summary = {};
                    let errors = 0;
                    data.requests.forEach(r => {
                        if (r.error) {
                            errors++;
                        } else {
                            const backend = r.backend || 'unknown';
                            summary[backend] = (summary[backend] || 0) + 1;
                        }
                    });
                    
                    const sortedBackends = Object.entries(summary).sort((a, b) => b[1] - a[1]);
                    const maxCount = sortedBackends.length > 0 ? sortedBackends[0][1] : 0;
                    
                    let resultText = '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n';
                    resultText += 'LOAD DISTRIBUTION REPORT\n';
                    resultText += 'Time: ' + timestamp + ' | Strategy: ' + strategy + '\n';
                    resultText += 'Total Requests: ' + count + ' | Successful: ' + (count - errors) + ' | Failed: ' + errors + '\n';
                    resultText += '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n';
                    
                    sortedBackends.forEach(([backend, cnt]) => {
                        const pct = Math.round(cnt/count*100);
                        const barLen = Math.round((cnt/maxCount) * 20);
                        const bar = '█'.repeat(barLen) + '░'.repeat(20 - barLen);
                        const port = backend.split(':').pop();
                        resultText += 'Backend :' + port + ' │ ' + bar + ' │ ' + cnt.toString().padStart(3) + ' reqs (' + pct.toString().padStart(2) + '%)\n';
                    });
                    
                    if (errors > 0) {
                        resultText += '\nErrors: ' + errors + ' requests failed (no healthy backend)\n';
                    }
                    
                    resultText += '\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━';
                    log(resultText, errors > 0 ? 'error' : 'success');
                }
            } catch (e) {
                log('[' + timestamp + '] Test requests failed: ' + e.message, 'error');
            }
            
            fetchBackends();
            fetchStatus();
        }

        async function changeStrategy(strategy) {
            try {
                log('Changing strategy to ' + strategy + '...', 'info');
                const form = new URLSearchParams();
                form.append('strategy', strategy);
                const res = await fetch('/api/strategy', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                    body: form
                });
                const data = await res.json();
                log('Strategy changed: ' + data.message, data.status === 'ok' ? 'success' : 'error');
                fetchStatus();
            } catch (e) {
                log('Failed to change strategy: ' + e.message, 'error');
            }
        }

        function toggleRateLimit() {
            rateLimitEnabled = !rateLimitEnabled;
            document.getElementById('rate-limit-toggle').classList.toggle('active', rateLimitEnabled);
            updateConfig({ rate_limit_enabled: rateLimitEnabled });
        }

        function toggleRequestLimit() {
            requestLimitEnabled = !requestLimitEnabled;
            document.getElementById('request-limit-toggle').classList.toggle('active', requestLimitEnabled);
            updateConfig({ request_limit_enabled: requestLimitEnabled });
        }

        async function updateRateLimit() {
            await updateConfig({
                rate_limit_enabled: rateLimitEnabled,
                rate_limit_max: document.getElementById('rate-limit-max').value,
                rate_limit_window: document.getElementById('rate-limit-window').value
            });
            log('Rate limit settings updated', 'success');
        }

        async function updateRequestLimit() {
            await updateConfig({
                request_limit_enabled: requestLimitEnabled,
                max_body_size: document.getElementById('max-body-size').value,
                max_header_size: document.getElementById('max-header-size').value
            });
            log('Request limit settings updated', 'success');
        }

        async function updateConfig(config) {
            try {
                const form = new URLSearchParams();
                for (const [key, value] of Object.entries(config)) {
                    form.append(key, value);
                }
                await fetch('/api/config', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                    body: form
                });
            } catch (e) {
                log('Failed to update config: ' + e.message, 'error');
            }
        }

        async function runTest(type) {
            try {
                log('Running ' + type + ' test...', 'info');
                const res = await fetch('/api/test?type=' + type);
                const data = await res.json();
                log(JSON.stringify(data, null, 2), data.status === 'ok' ? 'success' : 'error');
            } catch (e) {
                log('Test failed: ' + e.message, 'error');
            }
        }

        async function testEndpoint(path) {
            try {
                log('Testing endpoint: ' + path, 'info');
                const start = Date.now();
                const res = await fetch(path);
                const elapsed = Date.now() - start;
                const text = await res.text();
                log('Status: ' + res.status + ' (' + elapsed + 'ms)\n' + text.substring(0, 500), res.ok ? 'success' : 'error');
            } catch (e) {
                log('Request failed: ' + e.message, 'error');
            }
        }

        function refreshAll() {
            fetchStatus();
            fetchBackends();
            log('Dashboard refreshed', 'info');
        }

        fetchStatus();
        fetchBackends();
        setInterval(fetchStatus, 5000);
        setInterval(fetchBackends, 5000);
    </script>
</body>
</html>`
