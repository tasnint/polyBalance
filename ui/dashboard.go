package ui

import (
        "encoding/json"
        "html/template"
        "net/http"
        "polybalance/backend"
        "polybalance/middleware"
        "strconv"
        "time"
)

type Dashboard struct {
        backends       []*backend.Backend
        rateLimiter    *middleware.RateLimiter
        requestLimiter *middleware.RequestLimiter
        tlsConfig      *middleware.TLSConfig
        strategy       string
        startTime      time.Time
        requestCount   int64
        errorCount     int64
}

func NewDashboard(backends []*backend.Backend, rl *middleware.RateLimiter, reqLim *middleware.RequestLimiter, tlsCfg *middleware.TLSConfig, strategy string) *Dashboard {
        return &Dashboard{
                backends:       backends,
                rateLimiter:    rl,
                requestLimiter: reqLim,
                tlsConfig:      tlsCfg,
                strategy:       strategy,
                startTime:      time.Now(),
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
                "strategy":         d.strategy,
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
                "strategy":      d.strategy,
        }
        json.NewEncoder(w).Encode(config)
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
    <title>PolyBalance Dashboard</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
            color: #fff;
            min-height: 100vh;
            padding: 20px;
        }
        .container { max-width: 1400px; margin: 0 auto; }
        h1 {
            text-align: center;
            margin-bottom: 30px;
            font-size: 2.5rem;
            background: linear-gradient(90deg, #00d4ff, #7b2ff7);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }
        .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(400px, 1fr)); gap: 20px; }
        .card {
            background: rgba(255, 255, 255, 0.05);
            border-radius: 16px;
            padding: 24px;
            border: 1px solid rgba(255, 255, 255, 0.1);
            backdrop-filter: blur(10px);
        }
        .card h2 {
            font-size: 1.25rem;
            margin-bottom: 16px;
            display: flex;
            align-items: center;
            gap: 10px;
        }
        .card h2::before {
            content: '';
            width: 8px;
            height: 8px;
            background: #00d4ff;
            border-radius: 50%;
        }
        .status-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 12px; }
        .status-item {
            background: rgba(0, 0, 0, 0.2);
            padding: 16px;
            border-radius: 12px;
            text-align: center;
        }
        .status-value {
            font-size: 2rem;
            font-weight: bold;
            color: #00d4ff;
        }
        .status-label { font-size: 0.875rem; color: #aaa; margin-top: 4px; }
        .backend-list { display: flex; flex-direction: column; gap: 10px; }
        .backend-item {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 12px 16px;
            background: rgba(0, 0, 0, 0.2);
            border-radius: 8px;
        }
        .backend-url { font-family: monospace; font-size: 0.9rem; }
        .badge {
            padding: 4px 12px;
            border-radius: 20px;
            font-size: 0.75rem;
            font-weight: bold;
            text-transform: uppercase;
        }
        .badge.healthy { background: rgba(0, 255, 136, 0.2); color: #00ff88; }
        .badge.unhealthy { background: rgba(255, 68, 68, 0.2); color: #ff4444; }
        .controls { display: flex; flex-direction: column; gap: 16px; }
        .control-group label { display: block; margin-bottom: 8px; font-size: 0.9rem; color: #aaa; }
        .control-row { display: flex; gap: 12px; align-items: center; }
        input[type="number"], input[type="text"] {
            background: rgba(0, 0, 0, 0.3);
            border: 1px solid rgba(255, 255, 255, 0.1);
            border-radius: 8px;
            padding: 10px 14px;
            color: #fff;
            width: 100%;
            font-size: 1rem;
        }
        input:focus { outline: none; border-color: #00d4ff; }
        .btn {
            padding: 12px 24px;
            border: none;
            border-radius: 8px;
            cursor: pointer;
            font-weight: 600;
            font-size: 0.9rem;
            transition: all 0.2s;
        }
        .btn-primary {
            background: linear-gradient(90deg, #00d4ff, #7b2ff7);
            color: #fff;
        }
        .btn-primary:hover { transform: translateY(-2px); box-shadow: 0 4px 20px rgba(0, 212, 255, 0.3); }
        .btn-secondary {
            background: rgba(255, 255, 255, 0.1);
            color: #fff;
            border: 1px solid rgba(255, 255, 255, 0.2);
        }
        .btn-secondary:hover { background: rgba(255, 255, 255, 0.15); }
        .btn-danger { background: rgba(255, 68, 68, 0.2); color: #ff4444; }
        .btn-success { background: rgba(0, 255, 136, 0.2); color: #00ff88; }
        .test-buttons { display: flex; flex-wrap: wrap; gap: 10px; margin-bottom: 16px; }
        .test-output {
            background: rgba(0, 0, 0, 0.3);
            border-radius: 8px;
            padding: 16px;
            font-family: monospace;
            font-size: 0.85rem;
            min-height: 150px;
            max-height: 300px;
            overflow-y: auto;
            white-space: pre-wrap;
        }
        .toggle {
            position: relative;
            width: 50px;
            height: 26px;
            background: rgba(0, 0, 0, 0.3);
            border-radius: 13px;
            cursor: pointer;
            transition: background 0.3s;
        }
        .toggle.active { background: rgba(0, 212, 255, 0.5); }
        .toggle::after {
            content: '';
            position: absolute;
            width: 22px;
            height: 22px;
            background: #fff;
            border-radius: 50%;
            top: 2px;
            left: 2px;
            transition: transform 0.3s;
        }
        .toggle.active::after { transform: translateX(24px); }
        .log-entry { margin-bottom: 8px; padding-bottom: 8px; border-bottom: 1px solid rgba(255,255,255,0.1); }
        .log-entry.success { color: #00ff88; }
        .log-entry.error { color: #ff4444; }
        .log-entry.info { color: #00d4ff; }
        .refresh-btn {
            position: fixed;
            bottom: 20px;
            right: 20px;
            width: 56px;
            height: 56px;
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 1.5rem;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>PolyBalance Load Balancer</h1>
        
        <div class="grid">
            <div class="card">
                <h2>System Status</h2>
                <div class="status-grid">
                    <div class="status-item">
                        <div class="status-value" id="uptime">0</div>
                        <div class="status-label">Uptime (seconds)</div>
                    </div>
                    <div class="status-item">
                        <div class="status-value" id="strategy">-</div>
                        <div class="status-label">Strategy</div>
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
                <h2>Test & Diagnostics</h2>
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
                document.getElementById('strategy').textContent = data.strategy;
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
                    '<div style="display:flex;align-items:center;gap:10px;">' +
                    '<span class="badge ' + (b.healthy ? 'healthy' : 'unhealthy') + '">' + 
                    (b.healthy ? 'Healthy' : 'Unhealthy') + '</span>' +
                    '<span style="color:#aaa;font-size:0.8rem">(' + b.connections + ' conn)</span>' +
                    '<button class="btn btn-sm ' + (b.healthy ? 'btn-danger' : 'btn-success') + '" ' +
                    'onclick="toggleBackendHealth(\'' + b.url + '\')" ' +
                    'style="padding:6px 12px;font-size:0.75rem;">' +
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
                log('Toggle result: ' + data.message, data.status === 'ok' ? 'success' : 'error');
                setTimeout(() => {
                    fetchBackends();
                    fetchStatus();
                }, 500);
            } catch (e) {
                log('Toggle failed: ' + e.message, 'error');
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
