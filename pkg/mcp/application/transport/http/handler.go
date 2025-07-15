// Package http provides HTTP transport implementation with JSON-RPC bridge
package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/health"
	"github.com/mark3labs/mcp-go/server"
)

// Handler implements HTTP transport for MCP with JSON-RPC bridge
type Handler struct {
	logger        *slog.Logger
	mcpServer     *server.MCPServer
	healthMonitor health.Monitor
	port          int
	mu            sync.RWMutex // Protects mcpServer field
}

// NewHandler creates a new HTTP handler
func NewHandler(logger *slog.Logger, port int, healthMonitor health.Monitor) *Handler {
	if port == 0 {
		port = 8080 // Default port
	}

	return &Handler{
		logger:        logger.With("component", "http_handler"),
		healthMonitor: healthMonitor,
		port:          port,
	}
}

// Serve starts the HTTP server with MCP endpoints
func (h *Handler) Serve(ctx context.Context, mcpServer *server.MCPServer) error {
	h.mu.Lock()
	h.mcpServer = mcpServer
	h.mu.Unlock()
	h.logger.Info("Starting HTTP transport with MCP endpoints", "port", h.port)

	mux := http.NewServeMux()

	// Mount MCP endpoints
	mux.HandleFunc("/rpc", h.handleRPC)
	mux.HandleFunc("/healthz", h.handleHealth)
	mux.HandleFunc("/readyz", h.handleReady)
	mux.HandleFunc("/metrics", h.handleMetrics)
	mux.HandleFunc("/", h.handleRoot)

	// Create HTTP server with timeouts
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", h.port),
		Handler:      h.withMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Create error channel for transport
	transportDone := make(chan error, 1)

	// Run transport in goroutine
	go func() {
		transportDone <- httpServer.ListenAndServe()
	}()

	// Wait for context cancellation or transport error
	select {
	case <-ctx.Done():
		h.logger.Info("Shutting down HTTP transport")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	case err := <-transportDone:
		if err != nil && err != http.ErrServerClosed {
			h.logger.Error("HTTP transport stopped with error", "error", err)
		} else {
			h.logger.Info("HTTP transport stopped gracefully")
		}
		return err
	}
}

// withMiddleware adds common middleware to the handler
func (h *Handler) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Log request
		start := time.Now()
		h.logger.Debug("HTTP request",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr)

		// Call next handler
		next.ServeHTTP(w, r)

		// Log response time
		duration := time.Since(start)
		h.logger.Debug("HTTP request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"duration", duration)
	})
}

// handleRPC handles JSON-RPC requests and bridges them to MCP
func (h *Handler) handleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Parse JSON-RPC request
	var rpcReq struct {
		JSONRPC string          `json:"jsonrpc"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
		ID      interface{}     `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&rpcReq); err != nil {
		h.logger.Error("Failed to decode JSON-RPC request", "error", err)
		h.writeJSONRPCError(w, rpcReq.ID, -32700, "Parse error", nil)
		return
	}

	h.logger.Debug("Received JSON-RPC request", "method", rpcReq.Method, "id", rpcReq.ID)

	// Convert to MCP request format
	// Note: This is a simplified bridge implementation
	// In a full implementation, you'd need to properly map JSON-RPC to MCP protocol
	switch rpcReq.Method {
	case "initialize":
		h.handleInitialize(w, rpcReq.ID, rpcReq.Params)
	case "tools/list":
		h.handleListTools(w, rpcReq.ID)
	case "tools/call":
		h.handleCallTool(w, rpcReq.ID, rpcReq.Params)
	default:
		h.writeJSONRPCError(w, rpcReq.ID, -32601, "Method not found", nil)
	}
}

// handleInitialize handles MCP initialize requests
func (h *Handler) handleInitialize(w http.ResponseWriter, id interface{}, params json.RawMessage) {
	// Simplified initialize response
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "container-kit-mcp",
				"version": "0.0.6",
			},
		},
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode initialize response", "error", err)
	}
}

// handleListTools handles tools/list requests
func (h *Handler) handleListTools(w http.ResponseWriter, id interface{}) {
	// This would integrate with the actual MCP server's tool list
	// For now, return a simplified response
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]interface{}{
			"tools": []interface{}{
				map[string]interface{}{
					"name":        "containerize_and_deploy",
					"description": "Complete end-to-end containerization and deployment",
				},
			},
		},
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode list tools response", "error", err)
	}
}

// handleCallTool handles tools/call requests
func (h *Handler) handleCallTool(w http.ResponseWriter, id interface{}, params json.RawMessage) {
	// This would integrate with the actual MCP server's tool execution
	// For now, return a simplified response
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "Tool execution not yet implemented in HTTP transport",
				},
			},
		},
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode call tool response", "error", err)
	}
}

// writeJSONRPCError writes a JSON-RPC error response
func (h *Handler) writeJSONRPCError(w http.ResponseWriter, id interface{}, code int, message string, data interface{}) {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}

	if data != nil {
		response["error"].(map[string]interface{})["data"] = data
	}

	w.WriteHeader(http.StatusOK) // JSON-RPC errors are still HTTP 200
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode error response", "error", err)
	}
}

// handleHealth handles health check requests
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Run health checks
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	report := h.healthMonitor.GetHealth(ctx)

	// Set HTTP status based on health status
	var statusCode int
	switch report.Status {
	case health.StatusUnhealthy:
		statusCode = http.StatusServiceUnavailable
	case health.StatusDegraded:
		statusCode = http.StatusOK // Still available but degraded
	default:
		statusCode = http.StatusOK
	}

	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(report); err != nil {
		h.logger.Error("Failed to encode health response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleReady handles readiness probe requests
func (h *Handler) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Check if MCP server is initialized
	h.mu.RLock()
	serverReady := h.mcpServer != nil
	h.mu.RUnlock()

	if !serverReady {
		w.WriteHeader(http.StatusServiceUnavailable)
		response := map[string]interface{}{
			"ready":  false,
			"reason": "MCP server not initialized",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Run minimal health checks for readiness
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	report := h.healthMonitor.GetHealth(ctx)

	// For readiness, we only care if critical components are healthy
	ready := report.Status != health.StatusUnhealthy

	statusCode := http.StatusOK
	if !ready {
		statusCode = http.StatusServiceUnavailable
	}

	w.WriteHeader(statusCode)

	response := map[string]interface{}{
		"ready":     ready,
		"status":    string(report.Status),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode ready response", "error", err)
	}
}

// handleMetrics handles Prometheus metrics requests
func (h *Handler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	// Get current health report for metrics
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	report := h.healthMonitor.GetHealth(ctx)
	// For now, just use 0 for uptime since it's not in the interface
	uptimeSeconds := 0

	// Count components by status
	healthy, degraded, unhealthy := 0, 0, 0
	for _, comp := range report.Components {
		switch comp.Status {
		case health.StatusHealthy:
			healthy++
		case health.StatusDegraded:
			degraded++
		case health.StatusUnhealthy:
			unhealthy++
		}
	}
	total := len(report.Components)

	// Generate Prometheus metrics
	metrics := fmt.Sprintf(`# HELP container_kit_mcp_uptime_seconds MCP server uptime in seconds
# TYPE container_kit_mcp_uptime_seconds gauge
container_kit_mcp_uptime_seconds %d

# HELP container_kit_mcp_info MCP server information
# TYPE container_kit_mcp_info gauge
container_kit_mcp_info{version="0.0.6",transport="http"} 1

# HELP container_kit_mcp_health_status MCP server health status (1=healthy, 0.5=degraded, 0=unhealthy)
# TYPE container_kit_mcp_health_status gauge
container_kit_mcp_health_status %g

# HELP container_kit_mcp_health_checks_total Total number of health checks
# TYPE container_kit_mcp_health_checks_total gauge
container_kit_mcp_health_checks_total %d

# HELP container_kit_mcp_health_checks_healthy Number of healthy checks
# TYPE container_kit_mcp_health_checks_healthy gauge
container_kit_mcp_health_checks_healthy %d

# HELP container_kit_mcp_health_checks_degraded Number of degraded checks
# TYPE container_kit_mcp_health_checks_degraded gauge
container_kit_mcp_health_checks_degraded %d

# HELP container_kit_mcp_health_checks_unhealthy Number of unhealthy checks
# TYPE container_kit_mcp_health_checks_unhealthy gauge
container_kit_mcp_health_checks_unhealthy %d
`,
		uptimeSeconds,
		h.healthStatusToFloat(report.Status),
		total,
		healthy,
		degraded,
		unhealthy,
	)

	if _, err := w.Write([]byte(metrics)); err != nil {
		h.logger.Error("Failed to write metrics response", "error", err)
	}
}

// healthStatusToFloat converts health status to a float for Prometheus
func (h *Handler) healthStatusToFloat(status health.Status) float64 {
	switch status {
	case health.StatusHealthy:
		return 1.0
	case health.StatusDegraded:
		return 0.5
	case health.StatusUnhealthy:
		return 0.0
	default:
		return 0.0
	}
}

// handleRoot handles root path requests with API documentation
func (h *Handler) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	apiInfo := map[string]interface{}{
		"name":    "Container Kit MCP Server",
		"version": "0.0.6",
		"endpoints": map[string]interface{}{
			"/rpc":     "JSON-RPC bridge to MCP",
			"/healthz": "Health check endpoint (liveness probe)",
			"/readyz":  "Readiness probe endpoint",
			"/metrics": "Prometheus metrics endpoint",
		},
		"documentation": "https://github.com/Azure/container-kit",
	}

	if err := json.NewEncoder(w).Encode(apiInfo); err != nil {
		h.logger.Error("Failed to encode API info response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
