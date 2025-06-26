package observability

import (
	"embed"
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"
)

//go:embed dashboard_assets/*
var dashboardAssets embed.FS

// EnhancedDashboard provides an advanced metrics dashboard
type EnhancedDashboard struct {
	telemetryManager *EnhancedTelemetryManager
	tracingManager   *TracingManager
	exporter         *TelemetryExporter

	// Real-time data
	realtimeMetrics *RealtimeMetrics
	historicalData  *HistoricalMetrics

	// WebSocket connections
	wsConnections map[string]*WSConnection
	wsLock        sync.RWMutex

	// Dashboard configuration
	config *DashboardConfig
}

// RealtimeMetrics holds real-time metric data
type RealtimeMetrics struct {
	mu               sync.RWMutex
	Throughput       *SlidingWindow
	ErrorRate        *SlidingWindow
	Latency          *LatencyTracker
	ActiveOperations map[string]*OperationMetrics
}

// HistoricalMetrics stores historical data
type HistoricalMetrics struct {
	mu            sync.RWMutex
	HourlyMetrics map[time.Time]*MetricSnapshot
	DailyMetrics  map[time.Time]*MetricSnapshot
	WeeklyMetrics map[time.Time]*MetricSnapshot
}

// MetricSnapshot represents metrics at a point in time
type MetricSnapshot struct {
	Timestamp      time.Time
	ErrorRate      float64
	Throughput     float64
	AvgLatency     float64
	P95Latency     float64
	P99Latency     float64
	ErrorHandling  float64
	TestCoverage   float64
	ActiveSessions int
	MemoryUsage    int64
	CPUUsage       float64
}

// OperationMetrics tracks metrics for an ongoing operation
type OperationMetrics struct {
	Name          string
	StartTime     time.Time
	Duration      time.Duration
	Status        string
	Progress      float64
	SubOperations []*SubOperation
}

// SubOperation represents a sub-operation within a larger operation
type SubOperation struct {
	Name      string
	StartTime time.Time
	EndTime   time.Time
	Status    string
	Error     string
}

// WSConnection represents a WebSocket connection
type WSConnection struct {
	ID            string
	Conn          interface{} // Would be *websocket.Conn
	LastPing      time.Time
	Subscriptions []string
}

// DashboardConfig holds dashboard configuration
type DashboardConfig struct {
	RefreshInterval  time.Duration
	RetentionPeriod  time.Duration
	MaxConnections   int
	EnableRealtime   bool
	EnableHistorical bool
	Theme            string
}

// SlidingWindow tracks values over a time window
type SlidingWindow struct {
	window  time.Duration
	buckets map[time.Time]float64
	mu      sync.RWMutex
}

// LatencyTracker tracks latency distributions
type LatencyTracker struct {
	buckets map[string]*LatencyBucket
	mu      sync.RWMutex
}

// LatencyBucket holds latency data for a specific operation
type LatencyBucket struct {
	Values     []float64
	LastUpdate time.Time
}

// NewEnhancedDashboard creates a new enhanced dashboard
func NewEnhancedDashboard(telemetry *EnhancedTelemetryManager, tracing *TracingManager, exporter *TelemetryExporter) *EnhancedDashboard {
	config := &DashboardConfig{
		RefreshInterval:  5 * time.Second,
		RetentionPeriod:  7 * 24 * time.Hour,
		MaxConnections:   100,
		EnableRealtime:   true,
		EnableHistorical: true,
		Theme:            "dark",
	}

	dashboard := &EnhancedDashboard{
		telemetryManager: telemetry,
		tracingManager:   tracing,
		exporter:         exporter,
		config:           config,
		wsConnections:    make(map[string]*WSConnection),
		realtimeMetrics: &RealtimeMetrics{
			Throughput:       NewSlidingWindow(5 * time.Minute),
			ErrorRate:        NewSlidingWindow(5 * time.Minute),
			Latency:          &LatencyTracker{buckets: make(map[string]*LatencyBucket)},
			ActiveOperations: make(map[string]*OperationMetrics),
		},
		historicalData: &HistoricalMetrics{
			HourlyMetrics: make(map[time.Time]*MetricSnapshot),
			DailyMetrics:  make(map[time.Time]*MetricSnapshot),
			WeeklyMetrics: make(map[time.Time]*MetricSnapshot),
		},
	}

	// Start background workers
	go dashboard.startMetricsCollector()
	go dashboard.startHistoricalAggregator()
	go dashboard.startWebSocketBroadcaster()

	return dashboard
}

// ServeHTTP handles HTTP requests for the dashboard
func (ed *EnhancedDashboard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	switch path {
	case "/dashboard":
		ed.serveDashboardHTML(w, r)
	case "/api/metrics/realtime":
		ed.serveRealtimeMetrics(w, r)
	case "/api/metrics/historical":
		ed.serveHistoricalMetrics(w, r)
	case "/api/operations":
		ed.serveActiveOperations(w, r)
	case "/api/traces":
		ed.serveTraces(w, r)
	case "/api/alerts":
		ed.serveAlerts(w, r)
	case "/ws":
		ed.handleWebSocket(w, r)
	default:
		// Serve static assets
		ed.serveStaticAssets(w, r)
	}
}

func (ed *EnhancedDashboard) serveDashboardHTML(w http.ResponseWriter, r *http.Request) {
	dashboardHTML := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>MCP Enhanced Metrics Dashboard</title>
    <style>
        :root {
            --bg-primary: #0f1419;
            --bg-secondary: #1a1f2e;
            --bg-tertiary: #232937;
            --text-primary: #e1e8ed;
            --text-secondary: #8899a6;
            --accent-primary: #1da1f2;
            --accent-success: #17bf63;
            --accent-warning: #ffad1f;
            --accent-error: #e0245e;
            --border-color: #38444d;
        }

        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: var(--bg-primary);
            color: var(--text-primary);
            line-height: 1.6;
        }

        .dashboard {
            display: grid;
            grid-template-columns: 250px 1fr;
            height: 100vh;
        }

        .sidebar {
            background: var(--bg-secondary);
            padding: 20px;
            border-right: 1px solid var(--border-color);
        }

        .logo {
            font-size: 24px;
            font-weight: bold;
            margin-bottom: 30px;
            color: var(--accent-primary);
        }

        .nav-item {
            display: block;
            padding: 12px 16px;
            margin: 4px 0;
            color: var(--text-secondary);
            text-decoration: none;
            border-radius: 8px;
            transition: all 0.2s;
        }

        .nav-item:hover {
            background: var(--bg-tertiary);
            color: var(--text-primary);
        }

        .nav-item.active {
            background: var(--accent-primary);
            color: white;
        }

        .main-content {
            padding: 20px;
            overflow-y: auto;
        }

        .header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 30px;
        }

        .header h1 {
            font-size: 28px;
            font-weight: 600;
        }

        .time-range {
            display: flex;
            gap: 10px;
        }

        .time-btn {
            padding: 8px 16px;
            background: var(--bg-tertiary);
            border: 1px solid var(--border-color);
            color: var(--text-secondary);
            border-radius: 6px;
            cursor: pointer;
            transition: all 0.2s;
        }

        .time-btn:hover {
            background: var(--bg-secondary);
            color: var(--text-primary);
        }

        .time-btn.active {
            background: var(--accent-primary);
            color: white;
            border-color: var(--accent-primary);
        }

        .metrics-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 20px;
            margin-bottom: 30px;
        }

        .metric-card {
            background: var(--bg-secondary);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            padding: 24px;
            position: relative;
            overflow: hidden;
        }

        .metric-card.alert {
            border-color: var(--accent-error);
        }

        .metric-header {
            display: flex;
            justify-content: space-between;
            align-items: flex-start;
            margin-bottom: 16px;
        }

        .metric-title {
            color: var(--text-secondary);
            font-size: 14px;
            font-weight: 500;
        }

        .metric-badge {
            padding: 4px 8px;
            border-radius: 4px;
            font-size: 12px;
            font-weight: 600;
        }

        .metric-badge.success {
            background: rgba(23, 191, 99, 0.2);
            color: var(--accent-success);
        }

        .metric-badge.warning {
            background: rgba(255, 173, 31, 0.2);
            color: var(--accent-warning);
        }

        .metric-badge.error {
            background: rgba(224, 36, 94, 0.2);
            color: var(--accent-error);
        }

        .metric-value {
            font-size: 36px;
            font-weight: 700;
            margin-bottom: 8px;
        }

        .metric-trend {
            display: flex;
            align-items: center;
            gap: 8px;
            color: var(--text-secondary);
            font-size: 14px;
        }

        .trend-icon {
            font-size: 16px;
        }

        .trend-up {
            color: var(--accent-success);
        }

        .trend-down {
            color: var(--accent-error);
        }

        .chart-container {
            background: var(--bg-secondary);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            padding: 24px;
            margin-bottom: 20px;
            height: 400px;
        }

        .chart-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 20px;
        }

        .chart-title {
            font-size: 18px;
            font-weight: 600;
        }

        .chart-legend {
            display: flex;
            gap: 20px;
        }

        .legend-item {
            display: flex;
            align-items: center;
            gap: 8px;
            font-size: 14px;
            color: var(--text-secondary);
        }

        .legend-dot {
            width: 12px;
            height: 12px;
            border-radius: 50%;
        }

        .operations-list {
            background: var(--bg-secondary);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            padding: 24px;
        }

        .operation-item {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 16px 0;
            border-bottom: 1px solid var(--border-color);
        }

        .operation-item:last-child {
            border-bottom: none;
        }

        .operation-name {
            font-weight: 500;
        }

        .operation-status {
            display: flex;
            align-items: center;
            gap: 12px;
        }

        .status-indicator {
            width: 8px;
            height: 8px;
            border-radius: 50%;
            animation: pulse 2s infinite;
        }

        .status-running {
            background: var(--accent-primary);
        }

        .status-success {
            background: var(--accent-success);
        }

        .status-error {
            background: var(--accent-error);
        }

        @keyframes pulse {
            0% { opacity: 1; }
            50% { opacity: 0.5; }
            100% { opacity: 1; }
        }

        .progress-bar {
            width: 100px;
            height: 4px;
            background: var(--bg-tertiary);
            border-radius: 2px;
            overflow: hidden;
        }

        .progress-fill {
            height: 100%;
            background: var(--accent-primary);
            transition: width 0.3s ease;
        }

        #chart {
            width: 100%;
            height: 100%;
        }

        .loading {
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100%;
            color: var(--text-secondary);
        }

        .spinner {
            width: 40px;
            height: 40px;
            border: 3px solid var(--border-color);
            border-top-color: var(--accent-primary);
            border-radius: 50%;
            animation: spin 1s linear infinite;
        }

        @keyframes spin {
            to { transform: rotate(360deg); }
        }
    </style>
</head>
<body>
    <div class="dashboard">
        <aside class="sidebar">
            <div class="logo">MCP Metrics</div>
            <nav>
                <a href="#overview" class="nav-item active">Overview</a>
                <a href="#performance" class="nav-item">Performance</a>
                <a href="#errors" class="nav-item">Error Analysis</a>
                <a href="#traces" class="nav-item">Distributed Traces</a>
                <a href="#operations" class="nav-item">Active Operations</a>
                <a href="#quality" class="nav-item">Code Quality</a>
                <a href="#slo" class="nav-item">SLO Status</a>
                <a href="#alerts" class="nav-item">Alerts</a>
            </nav>
        </aside>

        <main class="main-content">
            <div class="header">
                <h1>System Overview</h1>
                <div class="time-range">
                    <button class="time-btn" data-range="1h">1H</button>
                    <button class="time-btn active" data-range="24h">24H</button>
                    <button class="time-btn" data-range="7d">7D</button>
                    <button class="time-btn" data-range="30d">30D</button>
                </div>
            </div>

            <div class="metrics-grid" id="metrics-grid">
                <div class="loading">
                    <div class="spinner"></div>
                </div>
            </div>

            <div class="chart-container">
                <div class="chart-header">
                    <h2 class="chart-title">System Performance</h2>
                    <div class="chart-legend">
                        <div class="legend-item">
                            <div class="legend-dot" style="background: #1da1f2"></div>
                            <span>Throughput</span>
                        </div>
                        <div class="legend-item">
                            <div class="legend-dot" style="background: #e0245e"></div>
                            <span>Error Rate</span>
                        </div>
                        <div class="legend-item">
                            <div class="legend-dot" style="background: #17bf63"></div>
                            <span>P95 Latency</span>
                        </div>
                    </div>
                </div>
                <div id="chart">
                    <div class="loading">
                        <div class="spinner"></div>
                    </div>
                </div>
            </div>

            <div class="operations-list" id="operations-list">
                <h2 style="margin-bottom: 20px;">Active Operations</h2>
                <div class="loading">
                    <div class="spinner"></div>
                </div>
            </div>
        </main>
    </div>

    <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js"></script>
    <script>
        // Dashboard JavaScript
        class Dashboard {
            constructor() {
                this.ws = null;
                this.chart = null;
                this.timeRange = '24h';
                this.init();
            }

            init() {
                this.connectWebSocket();
                this.loadMetrics();
                this.setupEventListeners();
                this.startPolling();
            }

            connectWebSocket() {
                const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
                this.ws = new WebSocket(` + "`${protocol}//${window.location.host}/ws`" + `);

                this.ws.onopen = () => {
                    console.log('WebSocket connected');
                    this.ws.send(JSON.stringify({ type: 'subscribe', topics: ['metrics', 'operations'] }));
                };

                this.ws.onmessage = (event) => {
                    const data = JSON.parse(event.data);
                    this.handleRealtimeUpdate(data);
                };

                this.ws.onclose = () => {
                    console.log('WebSocket disconnected, reconnecting...');
                    setTimeout(() => this.connectWebSocket(), 5000);
                };
            }

            async loadMetrics() {
                try {
                    const response = await fetch('/api/metrics/realtime');
                    const data = await response.json();
                    this.updateMetricsGrid(data);
                    this.updateChart(data);
                } catch (error) {
                    console.error('Failed to load metrics:', error);
                }
            }

            async loadOperations() {
                try {
                    const response = await fetch('/api/operations');
                    const data = await response.json();
                    this.updateOperationsList(data);
                } catch (error) {
                    console.error('Failed to load operations:', error);
                }
            }

            updateMetricsGrid(data) {
                const grid = document.getElementById('metrics-grid');
                const metrics = [
                    {
                        title: 'Throughput',
                        value: data.throughput?.toFixed(0) || '0',
                        unit: 'req/s',
                        trend: data.throughput_trend || 0,
                        status: 'success'
                    },
                    {
                        title: 'Error Rate',
                        value: data.error_rate?.toFixed(2) || '0.00',
                        unit: '%',
                        trend: data.error_rate_trend || 0,
                        status: data.error_rate > 5 ? 'error' : 'success'
                    },
                    {
                        title: 'P95 Latency',
                        value: data.p95_latency?.toFixed(0) || '0',
                        unit: 'ms',
                        trend: data.latency_trend || 0,
                        status: data.p95_latency > 1000 ? 'warning' : 'success'
                    },
                    {
                        title: 'Active Sessions',
                        value: data.active_sessions || '0',
                        unit: '',
                        trend: 0,
                        status: 'success'
                    },
                    {
                        title: 'Memory Usage',
                        value: ((data.memory_usage || 0) / 1024 / 1024).toFixed(0),
                        unit: 'MB',
                        trend: data.memory_trend || 0,
                        status: data.memory_usage > 1024*1024*1024 ? 'warning' : 'success'
                    },
                    {
                        title: 'Code Quality Score',
                        value: data.quality_score?.toFixed(1) || '0.0',
                        unit: '/100',
                        trend: data.quality_trend || 0,
                        status: data.quality_score < 60 ? 'warning' : 'success'
                    }
                ];

                grid.innerHTML = metrics.map(metric => this.createMetricCard(metric)).join('');
            }

            createMetricCard(metric) {
                const trendIcon = metric.trend > 0 ? '↑' : metric.trend < 0 ? '↓' : '→';
                const trendClass = metric.trend > 0 ? 'trend-up' : metric.trend < 0 ? 'trend-down' : '';

                return ` + "`" + `
                    <div class="metric-card ${metric.status === 'error' ? 'alert' : ''}">
                        <div class="metric-header">
                            <div class="metric-title">${metric.title}</div>
                            <div class="metric-badge ${metric.status}">${metric.status.toUpperCase()}</div>
                        </div>
                        <div class="metric-value">${metric.value}${metric.unit}</div>
                        <div class="metric-trend">
                            <span class="trend-icon ${trendClass}">${trendIcon}</span>
                            <span>${Math.abs(metric.trend).toFixed(1)}% from previous period</span>
                        </div>
                    </div>
                ` + "`" + `;
            }

            updateChart(data) {
                const ctx = document.getElementById('chart');
                if (!this.chart) {
                    this.chart = new Chart(ctx, {
                        type: 'line',
                        data: {
                            labels: [],
                            datasets: [
                                {
                                    label: 'Throughput',
                                    data: [],
                                    borderColor: '#1da1f2',
                                    backgroundColor: 'rgba(29, 161, 242, 0.1)',
                                    yAxisID: 'y1',
                                },
                                {
                                    label: 'Error Rate',
                                    data: [],
                                    borderColor: '#e0245e',
                                    backgroundColor: 'rgba(224, 36, 94, 0.1)',
                                    yAxisID: 'y2',
                                },
                                {
                                    label: 'P95 Latency',
                                    data: [],
                                    borderColor: '#17bf63',
                                    backgroundColor: 'rgba(23, 191, 99, 0.1)',
                                    yAxisID: 'y3',
                                }
                            ]
                        },
                        options: {
                            responsive: true,
                            maintainAspectRatio: false,
                            interaction: {
                                mode: 'index',
                                intersect: false,
                            },
                            scales: {
                                x: {
                                    grid: {
                                        color: 'rgba(56, 68, 77, 0.5)',
                                    },
                                    ticks: {
                                        color: '#8899a6',
                                    }
                                },
                                y1: {
                                    type: 'linear',
                                    display: true,
                                    position: 'left',
                                    grid: {
                                        color: 'rgba(56, 68, 77, 0.5)',
                                    },
                                    ticks: {
                                        color: '#8899a6',
                                    }
                                },
                                y2: {
                                    type: 'linear',
                                    display: true,
                                    position: 'right',
                                    grid: {
                                        drawOnChartArea: false,
                                    },
                                    ticks: {
                                        color: '#8899a6',
                                    }
                                },
                                y3: {
                                    type: 'linear',
                                    display: false,
                                }
                            }
                        }
                    });
                }

                // Update with historical data
                if (data.historical) {
                    this.chart.data.labels = data.historical.timestamps;
                    this.chart.data.datasets[0].data = data.historical.throughput;
                    this.chart.data.datasets[1].data = data.historical.error_rate;
                    this.chart.data.datasets[2].data = data.historical.latency;
                    this.chart.update();
                }
            }

            updateOperationsList(operations) {
                const container = document.getElementById('operations-list');
                if (!operations || operations.length === 0) {
                    container.innerHTML = '<h2>Active Operations</h2><p style="color: var(--text-secondary);">No active operations</p>';
                    return;
                }

                const operationsHTML = operations.map(op => ` + "`" + `
                    <div class="operation-item">
                        <div class="operation-name">${op.name}</div>
                        <div class="operation-status">
                            <div class="status-indicator status-${op.status}"></div>
                            <div class="progress-bar">
                                <div class="progress-fill" style="width: ${op.progress}%"></div>
                            </div>
                            <span>${op.duration || '0s'}</span>
                        </div>
                    </div>
                ` + "`" + `).join('');

                container.innerHTML = '<h2>Active Operations</h2>' + operationsHTML;
            }

            handleRealtimeUpdate(data) {
                if (data.type === 'metrics') {
                    this.updateMetricsGrid(data.metrics);
                } else if (data.type === 'operations') {
                    this.updateOperationsList(data.operations);
                }
            }

            setupEventListeners() {
                document.querySelectorAll('.time-btn').forEach(btn => {
                    btn.addEventListener('click', (e) => {
                        document.querySelectorAll('.time-btn').forEach(b => b.classList.remove('active'));
                        e.target.classList.add('active');
                        this.timeRange = e.target.dataset.range;
                        this.loadMetrics();
                    });
                });

                document.querySelectorAll('.nav-item').forEach(item => {
                    item.addEventListener('click', (e) => {
                        e.preventDefault();
                        document.querySelectorAll('.nav-item').forEach(i => i.classList.remove('active'));
                        e.target.classList.add('active');
                        // Handle navigation
                    });
                });
            }

            startPolling() {
                setInterval(() => {
                    this.loadMetrics();
                    this.loadOperations();
                }, 5000);
            }
        }

        // Initialize dashboard
        new Dashboard();
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(dashboardHTML))
}

func (ed *EnhancedDashboard) serveRealtimeMetrics(w http.ResponseWriter, r *http.Request) {
	ed.realtimeMetrics.mu.RLock()
	defer ed.realtimeMetrics.mu.RUnlock()

	// Get current metrics from telemetry manager
	telemetryMetrics := ed.telemetryManager.GetEnhancedMetrics()

	// Combine with realtime data
	response := map[string]interface{}{
		"timestamp":       time.Now(),
		"throughput":      ed.realtimeMetrics.Throughput.Rate(),
		"error_rate":      ed.realtimeMetrics.ErrorRate.Rate(),
		"p95_latency":     ed.getLatencyPercentile(95),
		"p99_latency":     ed.getLatencyPercentile(99),
		"active_sessions": len(ed.realtimeMetrics.ActiveOperations),
		"memory_usage":    getMemoryUsage(),
		"quality_score":   telemetryMetrics["quality"].(map[string]float64)["overall_score"],
		"historical":      ed.getHistoricalData(r.URL.Query().Get("range")),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (ed *EnhancedDashboard) serveHistoricalMetrics(w http.ResponseWriter, r *http.Request) {
	timeRange := r.URL.Query().Get("range")
	if timeRange == "" {
		timeRange = "24h"
	}

	ed.historicalData.mu.RLock()
	defer ed.historicalData.mu.RUnlock()

	var metrics []*MetricSnapshot
	now := time.Now()

	switch timeRange {
	case "1h":
		// Get hourly metrics for last hour
		for t, m := range ed.historicalData.HourlyMetrics {
			if t.After(now.Add(-1 * time.Hour)) {
				metrics = append(metrics, m)
			}
		}
	case "24h":
		// Get hourly metrics for last 24 hours
		for t, m := range ed.historicalData.HourlyMetrics {
			if t.After(now.Add(-24 * time.Hour)) {
				metrics = append(metrics, m)
			}
		}
	case "7d":
		// Get daily metrics for last 7 days
		for t, m := range ed.historicalData.DailyMetrics {
			if t.After(now.Add(-7 * 24 * time.Hour)) {
				metrics = append(metrics, m)
			}
		}
	case "30d":
		// Get daily metrics for last 30 days
		for t, m := range ed.historicalData.DailyMetrics {
			if t.After(now.Add(-30 * 24 * time.Hour)) {
				metrics = append(metrics, m)
			}
		}
	}

	// Sort by timestamp
	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].Timestamp.Before(metrics[j].Timestamp)
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"range":   timeRange,
		"metrics": metrics,
	})
}

func (ed *EnhancedDashboard) serveActiveOperations(w http.ResponseWriter, r *http.Request) {
	ed.realtimeMetrics.mu.RLock()
	defer ed.realtimeMetrics.mu.RUnlock()

	operations := make([]*OperationMetrics, 0, len(ed.realtimeMetrics.ActiveOperations))
	for _, op := range ed.realtimeMetrics.ActiveOperations {
		operations = append(operations, op)
	}

	// Sort by start time
	sort.Slice(operations, func(i, j int) bool {
		return operations[i].StartTime.After(operations[j].StartTime)
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(operations)
}

func (ed *EnhancedDashboard) serveTraces(w http.ResponseWriter, r *http.Request) {
	// Would integrate with actual trace storage
	traces := []map[string]interface{}{
		{
			"trace_id":   "abc123",
			"span_count": 15,
			"duration":   "234ms",
			"status":     "success",
			"service":    "mcp-server",
			"operation":  "tool.docker_build.execute",
			"timestamp":  time.Now().Add(-5 * time.Minute),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(traces)
}

func (ed *EnhancedDashboard) serveAlerts(w http.ResponseWriter, r *http.Request) {
	// Get alerts from exporter
	// This would be implemented based on the alert system
	alerts := []map[string]interface{}{
		{
			"id":       "alert-1",
			"name":     "High Error Rate",
			"severity": "warning",
			"message":  "Error rate exceeded 5% for 10 minutes",
			"started":  time.Now().Add(-15 * time.Minute),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(alerts)
}

func (ed *EnhancedDashboard) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// WebSocket implementation would go here
	// For now, return not implemented
	http.Error(w, "WebSocket not implemented", http.StatusNotImplemented)
}

func (ed *EnhancedDashboard) serveStaticAssets(w http.ResponseWriter, r *http.Request) {
	// Serve embedded assets
	http.FileServer(http.FS(dashboardAssets)).ServeHTTP(w, r)
}

// Background workers

func (ed *EnhancedDashboard) startMetricsCollector() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ed.collectRealtimeMetrics()
	}
}

func (ed *EnhancedDashboard) startHistoricalAggregator() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		ed.aggregateHistoricalMetrics()
	}
}

func (ed *EnhancedDashboard) startWebSocketBroadcaster() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ed.broadcastMetrics()
	}
}

func (ed *EnhancedDashboard) collectRealtimeMetrics() {
	// Collect metrics from various sources
	// Update realtime metrics
}

func (ed *EnhancedDashboard) aggregateHistoricalMetrics() {
	ed.historicalData.mu.Lock()
	defer ed.historicalData.mu.Unlock()

	now := time.Now()
	hourKey := now.Truncate(time.Hour)
	dayKey := now.Truncate(24 * time.Hour)

	// Create snapshot
	snapshot := &MetricSnapshot{
		Timestamp:   now,
		ErrorRate:   ed.realtimeMetrics.ErrorRate.Rate(),
		Throughput:  ed.realtimeMetrics.Throughput.Rate(),
		AvgLatency:  ed.getLatencyPercentile(50),
		P95Latency:  ed.getLatencyPercentile(95),
		P99Latency:  ed.getLatencyPercentile(99),
		MemoryUsage: getMemoryUsage(),
		CPUUsage:    getCPUUsage(),
	}

	// Store hourly
	ed.historicalData.HourlyMetrics[hourKey] = snapshot

	// Store daily average
	if _, exists := ed.historicalData.DailyMetrics[dayKey]; !exists {
		ed.historicalData.DailyMetrics[dayKey] = snapshot
	}

	// Clean old data
	ed.cleanOldMetrics()
}

func (ed *EnhancedDashboard) broadcastMetrics() {
	// Broadcast to WebSocket connections
	ed.wsLock.RLock()
	defer ed.wsLock.RUnlock()

	metricsData := map[string]interface{}{
		"type": "metrics",
		"metrics": map[string]interface{}{
			"throughput":  ed.realtimeMetrics.Throughput.Rate(),
			"error_rate":  ed.realtimeMetrics.ErrorRate.Rate(),
			"p95_latency": ed.getLatencyPercentile(95),
		},
	}

	for _, conn := range ed.wsConnections {
		// Send metrics to connection (stub implementation)
		// In a real implementation, this would use websocket.Conn.WriteJSON
		_ = conn
		_ = metricsData
	}
}

func (ed *EnhancedDashboard) cleanOldMetrics() {
	cutoff := time.Now().Add(-ed.config.RetentionPeriod)

	// Clean hourly metrics older than 7 days
	for t := range ed.historicalData.HourlyMetrics {
		if t.Before(cutoff) {
			delete(ed.historicalData.HourlyMetrics, t)
		}
	}

	// Clean daily metrics older than retention period
	for t := range ed.historicalData.DailyMetrics {
		if t.Before(cutoff) {
			delete(ed.historicalData.DailyMetrics, t)
		}
	}
}

func (ed *EnhancedDashboard) getLatencyPercentile(percentile int) float64 {
	// Aggregate latency from all buckets
	// This is a simplified implementation
	return float64(percentile) * 10 // Placeholder
}

func (ed *EnhancedDashboard) getHistoricalData(timeRange string) map[string]interface{} {
	// Return formatted historical data for charts
	return map[string]interface{}{
		"timestamps": []string{"10:00", "10:05", "10:10", "10:15", "10:20"},
		"throughput": []float64{100, 120, 115, 130, 125},
		"error_rate": []float64{0.5, 0.8, 0.3, 0.6, 0.4},
		"latency":    []float64{45, 52, 48, 55, 50},
	}
}

// Helper functions

func NewSlidingWindow(window time.Duration) *SlidingWindow {
	return &SlidingWindow{
		window:  window,
		buckets: make(map[time.Time]float64),
	}
}

func (sw *SlidingWindow) Add(value float64) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	sw.buckets[now] = value

	// Clean old entries
	cutoff := now.Add(-sw.window)
	for t := range sw.buckets {
		if t.Before(cutoff) {
			delete(sw.buckets, t)
		}
	}
}

func (sw *SlidingWindow) Rate() float64 {
	sw.mu.RLock()
	defer sw.mu.RUnlock()

	if len(sw.buckets) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range sw.buckets {
		sum += v
	}

	return sum / sw.window.Seconds()
}

func getMemoryUsage() int64 {
	// Placeholder - would get actual memory usage
	return 512 * 1024 * 1024 // 512 MB
}

func getCPUUsage() float64 {
	// Placeholder - would get actual CPU usage
	return 25.5 // 25.5%
}
