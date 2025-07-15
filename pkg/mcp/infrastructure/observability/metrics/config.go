// Package metrics provides configuration for LLM metrics collection
package metrics

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Config holds configuration for metrics collection
type Config struct {
	// Prometheus configuration
	EnablePrometheus bool   `json:"enable_prometheus" yaml:"enable_prometheus"`
	PrometheusPort   int    `json:"prometheus_port" yaml:"prometheus_port"`
	PrometheusPath   string `json:"prometheus_path" yaml:"prometheus_path"`

	// OpenTelemetry configuration
	EnableOTel     bool   `json:"enable_otel" yaml:"enable_otel"`
	OTelEndpoint   string `json:"otel_endpoint" yaml:"otel_endpoint"`
	ServiceName    string `json:"service_name" yaml:"service_name"`
	ServiceVersion string `json:"service_version" yaml:"service_version"`

	// Metrics collection settings
	CollectionInterval time.Duration `json:"collection_interval" yaml:"collection_interval"`
	EnableDetailed     bool          `json:"enable_detailed" yaml:"enable_detailed"`

	// Dashboard settings
	EnableDashboard bool `json:"enable_dashboard" yaml:"enable_dashboard"`
	DashboardPort   int  `json:"dashboard_port" yaml:"dashboard_port"`
}

// DefaultConfig returns default metrics configuration
func DefaultConfig() Config {
	return Config{
		EnablePrometheus:   true,
		PrometheusPort:     9090,
		PrometheusPath:     "/metrics",
		EnableOTel:         true,
		ServiceName:        "container-kit-mcp",
		ServiceVersion:     "1.0.0",
		CollectionInterval: 30 * time.Second,
		EnableDetailed:     true,
		EnableDashboard:    true,
		DashboardPort:      9091,
	}
}

// MetricsProvider manages the metrics infrastructure
type MetricsProvider struct {
	config       Config
	logger       *slog.Logger
	llmMetrics   *LLMMetrics
	promRegistry *prometheus.Registry
	httpServer   *http.Server
	dashboardSrv *http.Server
}

// NewMetricsProvider creates a new metrics provider
func NewMetricsProvider(config Config, logger *slog.Logger) (*MetricsProvider, error) {
	provider := &MetricsProvider{
		config: config,
		logger: logger.With("component", "metrics-provider"),
	}

	if err := provider.initializePrometheus(); err != nil {
		return nil, fmt.Errorf("failed to initialize Prometheus: %w", err)
	}

	if err := provider.initializeOpenTelemetry(); err != nil {
		return nil, fmt.Errorf("failed to initialize OpenTelemetry: %w", err)
	}

	if err := provider.initializeLLMMetrics(); err != nil {
		return nil, fmt.Errorf("failed to initialize LLM metrics: %w", err)
	}

	return provider, nil
}

// initializePrometheus sets up Prometheus metrics collection
func (p *MetricsProvider) initializePrometheus() error {
	if !p.config.EnablePrometheus {
		p.logger.Info("Prometheus metrics disabled")
		return nil
	}

	// Create custom registry to avoid conflicts
	p.promRegistry = prometheus.NewRegistry()

	// Register default collectors
	p.promRegistry.MustRegister(prometheus.NewGoCollector())
	p.promRegistry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	p.logger.Info("Prometheus metrics initialized",
		"port", p.config.PrometheusPort,
		"path", p.config.PrometheusPath)

	return nil
}

// initializeOpenTelemetry sets up OpenTelemetry metrics
func (p *MetricsProvider) initializeOpenTelemetry() error {
	if !p.config.EnableOTel {
		p.logger.Info("OpenTelemetry metrics disabled")
		return nil
	}

	// For now, use a no-op meter provider since full OTel setup requires
	// additional dependencies that may not be available in all environments
	// This can be enhanced later with proper OTel SDK when dependencies are resolved

	p.logger.Info("OpenTelemetry metrics initialized (basic mode)",
		"service_name", p.config.ServiceName,
		"service_version", p.config.ServiceVersion)

	return nil
}

// initializeLLMMetrics creates the LLM metrics collector
func (p *MetricsProvider) initializeLLMMetrics() error {
	var err error
	p.llmMetrics, err = NewLLMMetrics()
	if err != nil {
		return fmt.Errorf("failed to create LLM metrics: %w", err)
	}

	p.logger.Info("LLM metrics initialized")
	return nil
}

// GetLLMMetrics returns the LLM metrics collector
func (p *MetricsProvider) GetLLMMetrics() *LLMMetrics {
	return p.llmMetrics
}

// StartHTTPServer starts the metrics HTTP server
func (p *MetricsProvider) StartHTTPServer(ctx context.Context) error {
	if !p.config.EnablePrometheus {
		return nil
	}

	mux := http.NewServeMux()

	// Prometheus metrics endpoint
	if p.promRegistry != nil {
		mux.Handle(p.config.PrometheusPath, promhttp.HandlerFor(p.promRegistry, promhttp.HandlerOpts{}))
	} else {
		mux.Handle(p.config.PrometheusPath, promhttp.Handler())
	}

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Metrics overview endpoint
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		html := `<!DOCTYPE html>
<html>
<head>
    <title>Container Kit MCP - Metrics</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .endpoint { margin: 20px 0; padding: 10px; background: #f5f5f5; border-radius: 5px; }
        h1 { color: #333; }
        a { color: #0066cc; text-decoration: none; }
        a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <h1>Container Kit MCP - Metrics Endpoints</h1>
    <div class="endpoint">
        <h3><a href="` + p.config.PrometheusPath + `">Prometheus Metrics</a></h3>
        <p>Raw Prometheus metrics for scraping by monitoring systems</p>
    </div>
    <div class="endpoint">
        <h3><a href="/health">Health Check</a></h3>
        <p>Health status of the metrics service</p>
    </div>
</body>
</html>`
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	})

	addr := fmt.Sprintf(":%d", p.config.PrometheusPort)
	p.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	p.logger.Info("Starting metrics HTTP server", "addr", addr)

	go func() {
		if err := p.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			p.logger.Error("Metrics HTTP server error", "error", err)
		}
	}()

	return nil
}

// StartDashboard starts the metrics dashboard (optional)
func (p *MetricsProvider) StartDashboard(ctx context.Context) error {
	if !p.config.EnableDashboard {
		return nil
	}

	mux := http.NewServeMux()

	// Simple dashboard endpoint
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		dashboard := p.generateDashboardHTML()
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(dashboard))
	})

	// API endpoint for live metrics
	mux.HandleFunc("/api/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return JSON metrics summary (could be enhanced)
		w.Write([]byte(`{"status": "active", "timestamp": "` + time.Now().Format(time.RFC3339) + `"}`))
	})

	addr := fmt.Sprintf(":%d", p.config.DashboardPort)
	p.dashboardSrv = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	p.logger.Info("Starting metrics dashboard", "addr", addr)

	go func() {
		if err := p.dashboardSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			p.logger.Error("Metrics dashboard server error", "error", err)
		}
	}()

	return nil
}

// generateDashboardHTML creates a simple metrics dashboard
func (p *MetricsProvider) generateDashboardHTML() string {
	return `<!DOCTYPE html>
<html>
<head>
    <title>Container Kit MCP - LLM Metrics Dashboard</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; padding: 20px; background: #f8f9fa; }
        .container { max-width: 1200px; margin: 0 auto; }
        .header { background: white; padding: 20px; border-radius: 8px; margin-bottom: 20px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .metrics-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
        .metric-card { background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .metric-title { font-size: 18px; font-weight: 600; margin-bottom: 10px; color: #343a40; }
        .metric-value { font-size: 32px; font-weight: 700; color: #007bff; }
        .metric-desc { font-size: 14px; color: #6c757d; margin-top: 5px; }
        .status-ok { color: #28a745; }
        .status-warning { color: #ffc107; }
        .status-error { color: #dc3545; }
        .links { margin-top: 20px; }
        .links a { display: inline-block; margin-right: 15px; padding: 8px 16px; background: #007bff; color: white; text-decoration: none; border-radius: 4px; }
        .links a:hover { background: #0056b3; }
        .timestamp { text-align: center; margin-top: 20px; color: #6c757d; font-size: 12px; }
    </style>
    <script>
        function refreshMetrics() {
            fetch('/api/metrics')
                .then(response => response.json())
                .then(data => {
                    document.getElementById('last-updated').textContent = 'Last updated: ' + new Date().toLocaleString();
                })
                .catch(error => console.error('Error fetching metrics:', error));
        }
        
        // Refresh every 30 seconds
        setInterval(refreshMetrics, 30000);
        
        // Initial load
        document.addEventListener('DOMContentLoaded', refreshMetrics);
    </script>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Container Kit MCP - LLM Metrics Dashboard</h1>
            <p>Real-time monitoring of LLM operations and performance</p>
            <div class="links">
                <a href="http://localhost:` + fmt.Sprintf("%d", p.config.PrometheusPort) + p.config.PrometheusPath + `" target="_blank">Prometheus Metrics</a>
                <a href="#" onclick="location.reload()">Refresh</a>
            </div>
        </div>
        
        <div class="metrics-grid">
            <div class="metric-card">
                <div class="metric-title">Service Status</div>
                <div class="metric-value status-ok">Active</div>
                <div class="metric-desc">Metrics collection is running</div>
            </div>
            
            <div class="metric-card">
                <div class="metric-title">LLM Requests</div>
                <div class="metric-value">-</div>
                <div class="metric-desc">Total requests processed (connect Prometheus for live data)</div>
            </div>
            
            <div class="metric-card">
                <div class="metric-title">Success Rate</div>
                <div class="metric-value">-</div>
                <div class="metric-desc">Percentage of successful LLM requests</div>
            </div>
            
            <div class="metric-card">
                <div class="metric-title">Avg Response Time</div>
                <div class="metric-value">-</div>
                <div class="metric-desc">Average LLM response time in seconds</div>
            </div>
            
            <div class="metric-card">
                <div class="metric-title">Tokens Processed</div>
                <div class="metric-value">-</div>
                <div class="metric-desc">Total tokens processed across all requests</div>
            </div>
            
            <div class="metric-card">
                <div class="metric-title">Error Rate</div>
                <div class="metric-value">-</div>
                <div class="metric-desc">Requests that resulted in errors</div>
            </div>
        </div>
        
        <div class="timestamp" id="last-updated">
            Dashboard initialized
        </div>
    </div>
</body>
</html>`
}

// Shutdown gracefully shuts down the metrics provider
func (p *MetricsProvider) Shutdown(ctx context.Context) error {
	p.logger.Info("Shutting down metrics provider")

	var errors []error

	// Shutdown HTTP servers
	if p.httpServer != nil {
		if err := p.httpServer.Shutdown(ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to shutdown metrics server: %w", err))
		}
	}

	if p.dashboardSrv != nil {
		if err := p.dashboardSrv.Shutdown(ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to shutdown dashboard server: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %v", errors)
	}

	p.logger.Info("Metrics provider shutdown complete")
	return nil
}
