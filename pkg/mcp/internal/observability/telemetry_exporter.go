package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/sdk/trace"
)

// TelemetryExporter provides advanced telemetry export capabilities
type TelemetryExporter struct {
	enhancedManager *EnhancedTelemetryManager
	dashboardData   *DashboardData
	alertRules      []AlertRule
	mu              sync.RWMutex
}

// DashboardData holds pre-computed dashboard metrics
type DashboardData struct {
	LastUpdated time.Time                 `json:"last_updated"`
	Summary     map[string]interface{}    `json:"summary"`
	Trends      map[string]TrendData      `json:"trends"`
	Alerts      []Alert                   `json:"alerts"`
	SLOStatus   map[string]SLOStatus      `json:"slo_status"`
}

// TrendData represents metric trends
type TrendData struct {
	Current  float64   `json:"current"`
	Previous float64   `json:"previous"`
	Change   float64   `json:"change"`
	Trend    string    `json:"trend"` // up, down, stable
	Sparkline []float64 `json:"sparkline"`
}

// Alert represents an active alert
type Alert struct {
	Name        string    `json:"name"`
	Severity    string    `json:"severity"`
	Message     string    `json:"message"`
	StartTime   time.Time `json:"start_time"`
	Value       float64   `json:"value"`
	Threshold   float64   `json:"threshold"`
}

// SLOStatus represents SLO compliance status
type SLOStatus struct {
	Name            string  `json:"name"`
	Target          float64 `json:"target"`
	Current         float64 `json:"current"`
	Compliant       bool    `json:"compliant"`
	ErrorBudgetLeft float64 `json:"error_budget_left"`
	BurnRate        float64 `json:"burn_rate"`
}

// AlertRule defines alerting conditions
type AlertRule struct {
	Name       string
	Query      string
	Threshold  float64
	Comparator string // >, <, >=, <=, ==, !=
	Duration   time.Duration
	Severity   string
	Message    string
}

// NewTelemetryExporter creates a new telemetry exporter
func NewTelemetryExporter(enhancedManager *EnhancedTelemetryManager) *TelemetryExporter {
	exporter := &TelemetryExporter{
		enhancedManager: enhancedManager,
		dashboardData: &DashboardData{
			Summary:   make(map[string]interface{}),
			Trends:    make(map[string]TrendData),
			Alerts:    []Alert{},
			SLOStatus: make(map[string]SLOStatus),
		},
	}
	
	// Define default alert rules
	exporter.alertRules = []AlertRule{
		{
			Name:       "High Error Rate",
			Query:      "error_rate",
			Threshold:  5.0, // 5 errors per second
			Comparator: ">",
			Duration:   5 * time.Minute,
			Severity:   "critical",
			Message:    "Error rate exceeds 5/s for 5 minutes",
		},
		{
			Name:       "Low Test Coverage",
			Query:      "test_coverage",
			Threshold:  50.0,
			Comparator: "<",
			Duration:   1 * time.Hour,
			Severity:   "warning",
			Message:    "Test coverage below 50%",
		},
		{
			Name:       "High P95 Latency",
			Query:      "p95_latency",
			Threshold:  1.0, // 1 second
			Comparator: ">",
			Duration:   10 * time.Minute,
			Severity:   "warning",
			Message:    "P95 latency exceeds 1s",
		},
		{
			Name:       "Memory Pressure",
			Query:      "memory_utilization",
			Threshold:  80.0,
			Comparator: ">",
			Duration:   5 * time.Minute,
			Severity:   "warning",
			Message:    "Memory utilization above 80%",
		},
		{
			Name:       "SLO Violation",
			Query:      "slo_compliance",
			Threshold:  99.0,
			Comparator: "<",
			Duration:   15 * time.Minute,
			Severity:   "critical",
			Message:    "SLO compliance below target",
		},
	}
	
	// Start background updater
	go exporter.startDashboardUpdater()
	
	return exporter
}

// ServeHTTP implements http.Handler for the telemetry exporter
func (te *TelemetryExporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	
	switch {
	case path == "/metrics":
		te.servePrometheusMetrics(w, r)
	case path == "/metrics/enhanced":
		te.serveEnhancedMetrics(w, r)
	case path == "/dashboard":
		te.serveDashboard(w, r)
	case path == "/health":
		te.serveHealth(w, r)
	case path == "/alerts":
		te.serveAlerts(w, r)
	case path == "/slo":
		te.serveSLOStatus(w, r)
	case strings.HasPrefix(path, "/api/v1/"):
		te.serveAPI(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (te *TelemetryExporter) servePrometheusMetrics(w http.ResponseWriter, r *http.Request) {
	// Standard Prometheus metrics endpoint
	gatherer := prometheus.DefaultGatherer
	mfs, err := gatherer.Gather()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	contentType := expfmt.Negotiate(r.Header)
	encoder := expfmt.NewEncoder(w, contentType)
	
	for _, mf := range mfs {
		if err := encoder.Encode(mf); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (te *TelemetryExporter) serveEnhancedMetrics(w http.ResponseWriter, r *http.Request) {
	// Enhanced metrics with additional context
	metrics := te.enhancedManager.GetEnhancedMetrics()
	
	// Add dashboard data
	te.mu.RLock()
	metrics["dashboard"] = te.dashboardData
	te.mu.RUnlock()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func (te *TelemetryExporter) serveDashboard(w http.ResponseWriter, r *http.Request) {
	// Serve dashboard HTML
	dashboardHTML := `<!DOCTYPE html>
<html>
<head>
    <title>MCP Telemetry Dashboard</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            margin: 0;
            padding: 20px;
            background: #f5f7fa;
        }
        .container { max-width: 1400px; margin: 0 auto; }
        .header { 
            background: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            margin-bottom: 20px;
        }
        .metrics-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 20px;
            margin-bottom: 20px;
        }
        .metric-card {
            background: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .metric-value {
            font-size: 2.5em;
            font-weight: bold;
            margin: 10px 0;
        }
        .metric-label {
            color: #666;
            font-size: 0.9em;
        }
        .trend {
            font-size: 0.9em;
            margin-top: 5px;
        }
        .trend.up { color: #10b981; }
        .trend.down { color: #ef4444; }
        .trend.stable { color: #6b7280; }
        .alerts {
            background: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            margin-bottom: 20px;
        }
        .alert {
            padding: 10px;
            margin: 5px 0;
            border-radius: 4px;
            border-left: 4px solid;
        }
        .alert.critical { 
            background: #fee; 
            border-color: #ef4444;
        }
        .alert.warning { 
            background: #fef3c7; 
            border-color: #f59e0b;
        }
        .slo-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 15px;
        }
        .slo-item {
            background: white;
            padding: 15px;
            border-radius: 6px;
            box-shadow: 0 1px 3px rgba(0,0,0,0.1);
        }
        .slo-bar {
            height: 20px;
            background: #e5e7eb;
            border-radius: 10px;
            overflow: hidden;
            margin: 10px 0;
        }
        .slo-fill {
            height: 100%;
            background: #10b981;
            transition: width 0.3s;
        }
        .slo-fill.warning { background: #f59e0b; }
        .slo-fill.critical { background: #ef4444; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>MCP Telemetry Dashboard</h1>
            <p>Real-time observability and monitoring</p>
        </div>
        
        <div id="metrics-container">Loading...</div>
        <div id="alerts-container"></div>
        <div id="slo-container"></div>
    </div>
    
    <script>
        async function updateDashboard() {
            try {
                const response = await fetch('/metrics/enhanced');
                const data = await response.json();
                
                // Update metrics
                const metricsHtml = generateMetricsHTML(data);
                document.getElementById('metrics-container').innerHTML = metricsHtml;
                
                // Update alerts
                const alertsHtml = generateAlertsHTML(data.dashboard?.alerts || []);
                document.getElementById('alerts-container').innerHTML = alertsHtml;
                
                // Update SLOs
                const sloHtml = generateSLOHTML(data.dashboard?.slo_status || {});
                document.getElementById('slo-container').innerHTML = sloHtml;
                
            } catch (error) {
                console.error('Failed to update dashboard:', error);
            }
        }
        
        function generateMetricsHTML(data) {
            const metrics = [
                {
                    label: 'Error Rate',
                    value: (data.performance?.error_rate || 0).toFixed(2) + '/s',
                    trend: data.dashboard?.trends?.error_rate
                },
                {
                    label: 'Throughput',
                    value: (data.performance?.throughput || 0).toFixed(0) + '/s',
                    trend: data.dashboard?.trends?.throughput
                },
                {
                    label: 'Availability',
                    value: (data.performance?.availability || 0).toFixed(2) + '%',
                    trend: data.dashboard?.trends?.availability
                },
                {
                    label: 'Code Quality Score',
                    value: (data.quality?.overall_score || 0).toFixed(1),
                    trend: data.dashboard?.trends?.quality_score
                },
                {
                    label: 'Test Coverage',
                    value: (data.quality?.test_coverage || 0).toFixed(1) + '%',
                    trend: data.dashboard?.trends?.test_coverage
                },
                {
                    label: 'Active Goroutines',
                    value: Math.round(data.performance?.goroutines || 0),
                    trend: data.dashboard?.trends?.goroutines
                }
            ];
            
            return '<div class="metrics-grid">' + 
                metrics.map(m => generateMetricCard(m)).join('') + 
                '</div>';
        }
        
        function generateMetricCard(metric) {
            const trendClass = metric.trend?.trend || 'stable';
            const trendSymbol = trendClass === 'up' ? '↑' : trendClass === 'down' ? '↓' : '→';
            const trendText = metric.trend ? 
                \`<div class="trend \${trendClass}">\${trendSymbol} \${Math.abs(metric.trend.change).toFixed(1)}%</div>\` : '';
            
            return \`
                <div class="metric-card">
                    <div class="metric-label">\${metric.label}</div>
                    <div class="metric-value">\${metric.value}</div>
                    \${trendText}
                </div>
            \`;
        }
        
        function generateAlertsHTML(alerts) {
            if (!alerts || alerts.length === 0) {
                return '';
            }
            
            return \`
                <div class="alerts">
                    <h2>Active Alerts</h2>
                    \${alerts.map(alert => \`
                        <div class="alert \${alert.severity}">
                            <strong>\${alert.name}</strong>: \${alert.message}
                            <br>
                            <small>Since: \${new Date(alert.start_time).toLocaleString()}</small>
                        </div>
                    \`).join('')}
                </div>
            \`;
        }
        
        function generateSLOHTML(sloStatus) {
            const slos = Object.entries(sloStatus);
            if (slos.length === 0) {
                return '';
            }
            
            return \`
                <div class="alerts">
                    <h2>SLO Status</h2>
                    <div class="slo-grid">
                        \${slos.map(([name, slo]) => {
                            const fillClass = slo.compliant ? '' : 
                                slo.error_budget_left < 20 ? 'critical' : 'warning';
                            return \`
                                <div class="slo-item">
                                    <strong>\${slo.name}</strong>
                                    <div class="slo-bar">
                                        <div class="slo-fill \${fillClass}" 
                                             style="width: \${slo.current}%"></div>
                                    </div>
                                    <small>
                                        Current: \${slo.current.toFixed(2)}% | 
                                        Target: \${slo.target}% | 
                                        Budget: \${slo.error_budget_left.toFixed(1)}%
                                    </small>
                                </div>
                            \`;
                        }).join('')}
                    </div>
                </div>
            \`;
        }
        
        // Update every 10 seconds
        updateDashboard();
        setInterval(updateDashboard, 10000);
    </script>
</body>
</html>`
	
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(dashboardHTML))
}

func (te *TelemetryExporter) serveHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status": "healthy",
		"timestamp": time.Now(),
		"checks": map[string]string{
			"telemetry": "ok",
			"dashboard": "ok",
			"alerts": "ok",
		},
	}
	
	// Check if any critical alerts are active
	te.mu.RLock()
	criticalAlerts := 0
	for _, alert := range te.dashboardData.Alerts {
		if alert.Severity == "critical" {
			criticalAlerts++
		}
	}
	te.mu.RUnlock()
	
	if criticalAlerts > 0 {
		health["status"] = "degraded"
		health["critical_alerts"] = criticalAlerts
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func (te *TelemetryExporter) serveAlerts(w http.ResponseWriter, r *http.Request) {
	te.mu.RLock()
	alerts := te.dashboardData.Alerts
	te.mu.RUnlock()
	
	response := map[string]interface{}{
		"alerts": alerts,
		"total": len(alerts),
		"by_severity": map[string]int{},
	}
	
	for _, alert := range alerts {
		response["by_severity"].(map[string]int)[alert.Severity]++
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (te *TelemetryExporter) serveSLOStatus(w http.ResponseWriter, r *http.Request) {
	te.mu.RLock()
	sloStatus := te.dashboardData.SLOStatus
	te.mu.RUnlock()
	
	// Calculate summary
	totalSLOs := len(sloStatus)
	compliantSLOs := 0
	for _, slo := range sloStatus {
		if slo.Compliant {
			compliantSLOs++
		}
	}
	
	response := map[string]interface{}{
		"slos": sloStatus,
		"summary": map[string]interface{}{
			"total": totalSLOs,
			"compliant": compliantSLOs,
			"compliance_rate": float64(compliantSLOs) / float64(totalSLOs) * 100,
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (te *TelemetryExporter) serveAPI(w http.ResponseWriter, r *http.Request) {
	// API endpoints for programmatic access
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
	
	switch path {
	case "query":
		te.handleQuery(w, r)
	case "export":
		te.handleExport(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (te *TelemetryExporter) handleQuery(w http.ResponseWriter, r *http.Request) {
	// Simple query interface
	metric := r.URL.Query().Get("metric")
	if metric == "" {
		http.Error(w, "metric parameter required", http.StatusBadRequest)
		return
	}
	
	// Get metric value
	metrics := te.enhancedManager.GetEnhancedMetrics()
	value := extractMetricValue(metrics, metric)
	
	response := map[string]interface{}{
		"metric": metric,
		"value": value,
		"timestamp": time.Now(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (te *TelemetryExporter) handleExport(w http.ResponseWriter, r *http.Request) {
	// Export telemetry data
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}
	
	switch format {
	case "json":
		te.serveEnhancedMetrics(w, r)
	case "prometheus":
		te.servePrometheusMetrics(w, r)
	default:
		http.Error(w, "unsupported format", http.StatusBadRequest)
	}
}

func (te *TelemetryExporter) startDashboardUpdater() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		te.updateDashboard()
		te.checkAlerts()
		te.updateSLOStatus()
	}
}

func (te *TelemetryExporter) updateDashboard() {
	metrics := te.enhancedManager.GetEnhancedMetrics()
	
	te.mu.Lock()
	defer te.mu.Unlock()
	
	// Update summary
	te.dashboardData.Summary = metrics
	te.dashboardData.LastUpdated = time.Now()
	
	// Calculate trends (simplified - in production, store historical data)
	te.dashboardData.Trends["error_rate"] = TrendData{
		Current: metrics["performance"].(map[string]interface{})["error_rate"].(float64),
		Previous: 0, // Would be from historical data
		Change: 0,
		Trend: "stable",
	}
	
	te.dashboardData.Trends["throughput"] = TrendData{
		Current: metrics["performance"].(map[string]interface{})["throughput"].(float64),
		Previous: 0,
		Change: 0,
		Trend: "stable",
	}
	
	te.dashboardData.Trends["availability"] = TrendData{
		Current: metrics["performance"].(map[string]interface{})["availability"].(float64),
		Previous: 99.9,
		Change: 0.05,
		Trend: "up",
	}
}

func (te *TelemetryExporter) checkAlerts() {
	metrics := te.enhancedManager.GetEnhancedMetrics()
	newAlerts := []Alert{}
	
	for _, rule := range te.alertRules {
		value := extractMetricValue(metrics, rule.Query)
		if evaluateCondition(value, rule.Threshold, rule.Comparator) {
			// Check if alert already exists
			exists := false
			for _, existing := range te.dashboardData.Alerts {
				if existing.Name == rule.Name {
					exists = true
					break
				}
			}
			
			if !exists {
				newAlerts = append(newAlerts, Alert{
					Name:      rule.Name,
					Severity:  rule.Severity,
					Message:   rule.Message,
					StartTime: time.Now(),
					Value:     value,
					Threshold: rule.Threshold,
				})
			}
		}
	}
	
	te.mu.Lock()
	te.dashboardData.Alerts = newAlerts
	te.mu.Unlock()
}

func (te *TelemetryExporter) updateSLOStatus() {
	te.mu.Lock()
	defer te.mu.Unlock()
	
	// Example SLO calculations
	te.dashboardData.SLOStatus["availability"] = SLOStatus{
		Name:            "Availability",
		Target:          99.9,
		Current:         99.95,
		Compliant:       true,
		ErrorBudgetLeft: 50.0, // 50% of error budget remaining
		BurnRate:        0.5,  // Burning error budget at 0.5x rate
	}
	
	te.dashboardData.SLOStatus["latency_p95"] = SLOStatus{
		Name:            "P95 Latency < 1s",
		Target:          95.0,
		Current:         96.5,
		Compliant:       true,
		ErrorBudgetLeft: 70.0,
		BurnRate:        0.3,
	}
	
	te.dashboardData.SLOStatus["error_rate"] = SLOStatus{
		Name:            "Error Rate < 1%",
		Target:          99.0,
		Current:         98.5,
		Compliant:       false,
		ErrorBudgetLeft: -50.0, // Exceeded budget
		BurnRate:        1.5,
	}
}

// Helper functions

func extractMetricValue(metrics map[string]interface{}, path string) float64 {
	parts := strings.Split(path, ".")
	current := metrics
	
	for i, part := range parts {
		if i == len(parts)-1 {
			if val, ok := current[part].(float64); ok {
				return val
			}
		} else {
			if next, ok := current[part].(map[string]interface{}); ok {
				current = next
			} else {
				return 0
			}
		}
	}
	
	return 0
}

func evaluateCondition(value, threshold float64, comparator string) bool {
	switch comparator {
	case ">":
		return value > threshold
	case "<":
		return value < threshold
	case ">=":
		return value >= threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	case "!=":
		return value != threshold
	default:
		return false
	}
}

// RegisterTelemetryEndpoints registers HTTP endpoints for telemetry
func RegisterTelemetryEndpoints(mux *http.ServeMux, exporter *TelemetryExporter) {
	mux.Handle("/metrics", exporter)
	mux.Handle("/metrics/enhanced", exporter)
	mux.Handle("/dashboard", exporter)
	mux.Handle("/health", exporter)
	mux.Handle("/alerts", exporter)
	mux.Handle("/slo", exporter)
	mux.Handle("/api/v1/", exporter)
}