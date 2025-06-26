package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// LocalRequestHandler processes MCP requests (local interface to avoid import cycles)
type LocalRequestHandler interface {
	HandleRequest(ctx context.Context, req *mcptypes.MCPRequest) (*mcptypes.MCPResponse, error)
}

// HTTPTransport implements the Transport interface for HTTP/REST communication
type HTTPTransport struct {
	server         *http.Server
	mcpServer      interface{}
	router         chi.Router
	tools          map[string]ToolHandler
	toolsMutex     sync.RWMutex
	logger         zerolog.Logger
	port           int
	corsOrigins    []string
	apiKey         string
	rateLimit      int
	rateLimiter    map[string]*rateLimiter
	logBodies      bool
	maxBodyLogSize int64
	handler        LocalRequestHandler
}

// HTTPTransportConfig holds configuration for HTTP transport
type HTTPTransportConfig struct {
	Port           int
	CORSOrigins    []string
	APIKey         string
	RateLimit      int // requests per minute per IP
	Logger         zerolog.Logger
	LogBodies      bool
	MaxBodyLogSize int64  // Maximum size of request/response bodies to log
	LogLevel       string // "debug", "info", "warn", "error"
}

// ToolHandler is the function signature for tool handlers
type ToolHandler func(ctx context.Context, args interface{}) (interface{}, error)

// rateLimiter tracks request rates
type rateLimiter struct {
	requests []time.Time
	mutex    sync.Mutex
}

// NewHTTPTransport creates a new HTTP transport
func NewHTTPTransport(config HTTPTransportConfig) *HTTPTransport {
	if config.Port == 0 {
		config.Port = 8080
	}
	if config.RateLimit == 0 {
		config.RateLimit = 60 // 60 requests per minute default
	}

	transport := &HTTPTransport{
		tools:          make(map[string]ToolHandler),
		logger:         config.Logger.With().Str("component", "http_transport").Logger(),
		port:           config.Port,
		corsOrigins:    config.CORSOrigins,
		apiKey:         config.APIKey,
		rateLimit:      config.RateLimit,
		rateLimiter:    make(map[string]*rateLimiter),
		logBodies:      config.LogBodies,
		maxBodyLogSize: config.MaxBodyLogSize,
	}

	// Set default max body log size if not specified
	if transport.maxBodyLogSize == 0 {
		transport.maxBodyLogSize = 10 * 1024 // Default 10KB
	}

	transport.setupRouter()
	return transport
}

// setupRouter initializes the HTTP router and middleware
func (t *HTTPTransport) setupRouter() {
	t.router = chi.NewRouter()

	// Standard middleware chain: CORS → rate-limit → auth → telemetry
	t.setupMiddlewareChain()

	// API v1 routes
	t.router.Route("/api/v1", func(r chi.Router) {
		// Tool endpoints
		r.Get("/tools", t.handleListTools)
		r.Options("/tools", t.handleOptions)
		r.Post("/tools/{tool}", t.handleExecuteTool)
		r.Options("/tools/{tool}", t.handleOptions)

		// Health and status
		r.Get("/health", t.handleHealth)
		r.Get("/status", t.handleStatus)

		// Session management
		r.Get("/sessions", t.handleListSessions)
		r.Options("/sessions", t.handleOptions)
		r.Get("/sessions/{sessionID}", t.handleGetSession)
		r.Options("/sessions/{sessionID}", t.handleOptions)
		r.Delete("/sessions/{sessionID}", t.handleDeleteSession)
		r.Options("/sessions/{sessionID}", t.handleOptions)
	})
}

// setupMiddlewareChain configures the middleware chain in the proper order
func (t *HTTPTransport) setupMiddlewareChain() {
	// 1. Basic Chi middleware
	t.router.Use(middleware.RequestID)
	t.router.Use(middleware.RealIP)
	t.router.Use(middleware.Recoverer)

	// 2. CORS (first in chain to handle preflight requests)
	t.router.Use(t.setupCORS())

	// 3. Rate limiting (after CORS, before auth)
	t.router.Use(t.rateLimitMiddleware)

	// 4. Authentication (after rate limiting)
	t.router.Use(t.authMiddleware)

	// 5. Telemetry/Logging (last, to capture complete request flow)
	t.router.Use(t.loggingMiddleware)

	// 6. Timeout
	t.router.Use(middleware.Timeout(30 * time.Second))
}

// setupCORS creates and configures the CORS middleware
func (t *HTTPTransport) setupCORS() func(http.Handler) http.Handler {
	// Default CORS options
	corsOptions := cors.Options{
		AllowedOrigins:   t.corsOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-API-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // 5 minutes
	}

	// If no origins specified, allow all (for development)
	if len(t.corsOrigins) == 0 || (len(t.corsOrigins) == 1 && t.corsOrigins[0] == "*") {
		corsOptions.AllowedOrigins = []string{"*"}
		corsOptions.AllowCredentials = false // Cannot use credentials with wildcard origin
	}

	return cors.Handler(corsOptions)
}

// handleOptions handles preflight OPTIONS requests
func (t *HTTPTransport) handleOptions(w http.ResponseWriter, r *http.Request) {
	// CORS headers are already handled by the CORS middleware
	w.WriteHeader(http.StatusOK)
}

// Serve starts the HTTP server and handles requests
func (t *HTTPTransport) Serve(ctx context.Context) error {
	if t.handler == nil {
		return fmt.Errorf("request handler not set")
	}
	t.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", t.port),
		Handler:      t.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		t.logger.Info().Int("port", t.port).Msg("Starting HTTP transport")
		if err := t.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("failed to start HTTP server: %w", err)
		}
	}()

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		return t.Close()
	case err := <-errCh:
		return err
	}
}

// Close gracefully shuts down the HTTP server
func (t *HTTPTransport) Close() error {
	if t.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.logger.Info().Msg("Stopping HTTP transport")
	return t.server.Shutdown(ctx)
}

// SetHandler sets the request handler for this transport
func (t *HTTPTransport) SetHandler(handler LocalRequestHandler) {
	t.handler = handler
}

// Start starts the HTTP transport - alias for Serve
func (t *HTTPTransport) Start(ctx context.Context) error {
	return t.Serve(ctx)
}

// Stop gracefully shuts down the HTTP transport
func (t *HTTPTransport) Stop(ctx context.Context) error {
	if t.server == nil {
		return nil
	}

	t.logger.Info().Msg("Stopping HTTP transport")
	return t.server.Shutdown(ctx)
}

// SendMessage sends a message via HTTP (not applicable for HTTP REST API)
func (t *HTTPTransport) SendMessage(message interface{}) error {
	// HTTP transport doesn't use message-based communication
	// Messages are sent via HTTP responses
	return fmt.Errorf("SendMessage not applicable for HTTP transport")
}

// ReceiveMessage receives a message via HTTP (not applicable for HTTP REST API)
func (t *HTTPTransport) ReceiveMessage() (interface{}, error) {
	// HTTP transport doesn't use message-based communication
	// Messages are received via HTTP requests
	return nil, fmt.Errorf("ReceiveMessage not applicable for HTTP transport")
}

// Name returns the transport name
func (t *HTTPTransport) Name() string {
	return "http"
}

// RegisterTool registers a tool handler
func (t *HTTPTransport) RegisterTool(name, description string, handler interface{}) error {
	t.toolsMutex.Lock()
	defer t.toolsMutex.Unlock()

	toolHandler, ok := handler.(ToolHandler)
	if !ok {
		return fmt.Errorf("handler must be of type ToolHandler")
	}

	t.tools[name] = toolHandler
	t.logger.Info().Str("tool", name).Msg("Registered tool")
	return nil
}

// SetServer sets the MCP server for integration with gomcp
func (t *HTTPTransport) SetServer(srv interface{}) {
	t.mcpServer = srv
	t.logger.Debug().Msg("MCP server set for HTTP transport")
}

// GetServer returns the underlying MCP server
func (t *HTTPTransport) GetServer() interface{} {
	return t.mcpServer
}

// GetPort returns the HTTP transport port
func (t *HTTPTransport) GetPort() int {
	return t.port
}

// Middleware

func (t *HTTPTransport) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Get request ID from chi middleware (if available)
		requestID := middleware.GetReqID(r.Context())
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Prepare request log event
		logEvent := t.logger.Info().
			Str("request_id", requestID).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Str("user_agent", r.UserAgent())

		// Add headers to log (security audit trail)
		if t.logBodies {
			headers := make(map[string]string)
			for k, v := range r.Header {
				if k != "Authorization" && k != "Api-Key" { // Don't log sensitive headers
					headers[k] = strings.Join(v, ", ")
				}
			}
			logEvent.Interface("request_headers", headers)
		}

		// Read and log request body if enabled
		if t.logBodies && r.Body != nil {
			bodyReader := io.LimitReader(r.Body, t.maxBodyLogSize)
			requestBody, err := io.ReadAll(bodyReader)
			if err != nil {
				t.logger.Debug().Err(err).Msg("Failed to read request body")
			}
			if err := r.Body.Close(); err != nil {
				t.logger.Debug().Err(err).Msg("Failed to close request body")
			}

			// Restore body for handler
			r.Body = io.NopCloser(bytes.NewReader(requestBody))

			// Log body if not empty
			if len(requestBody) > 0 {
				logEvent.RawJSON("request_body", requestBody)
			}
		}

		logEvent.Msg("HTTP request received")

		// Wrap response writer to capture status and body
		wrapped := &loggingResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			logBodies:      t.logBodies,
			maxSize:        t.maxBodyLogSize,
		}

		// Process request
		next.ServeHTTP(wrapped, r)

		// Log response
		responseLog := t.logger.Info().
			Str("request_id", requestID).
			Int("status", wrapped.statusCode).
			Dur("duration", time.Since(start)).
			Int("response_size", wrapped.bytesWritten)

		// Add response body to log if enabled
		if t.logBodies && len(wrapped.body) > 0 {
			responseLog.RawJSON("response_body", wrapped.body)
		}

		responseLog.Msg("HTTP response sent")

		// Log security audit trail for important operations
		if wrapped.statusCode >= 400 || r.Method != "GET" {
			t.logger.Warn().
				Str("request_id", requestID).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("remote_addr", r.RemoteAddr).
				Int("status", wrapped.statusCode).
				Msg("Security audit: Non-GET request or error response")
		}
	})
}

func (t *HTTPTransport) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health endpoint
		if r.URL.Path == "/api/v1/health" {
			next.ServeHTTP(w, r)
			return
		}

		// Check API key if configured
		if t.apiKey != "" {
			providedKey := r.Header.Get("X-API-Key")
			if providedKey == "" {
				providedKey = r.URL.Query().Get("api_key")
			}

			if providedKey != t.apiKey {
				t.sendError(w, http.StatusUnauthorized, "Invalid or missing API key")
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (t *HTTPTransport) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get client IP
		clientIP := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			clientIP = strings.Split(forwarded, ",")[0]
		}

		// Check rate limit
		if !t.checkRateLimit(clientIP) {
			t.sendError(w, http.StatusTooManyRequests, "Rate limit exceeded")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Handler methods

func (t *HTTPTransport) handleListTools(w http.ResponseWriter, r *http.Request) {
	t.toolsMutex.RLock()
	defer t.toolsMutex.RUnlock()

	tools := make([]map[string]string, 0, len(t.tools))
	for name := range t.tools {
		tools = append(tools, map[string]string{
			"name":     name,
			"endpoint": fmt.Sprintf("/api/v1/tools/%s", name),
		})
	}

	t.sendJSON(w, http.StatusOK, map[string]interface{}{
		"tools": tools,
		"count": len(tools),
	})
}

func (t *HTTPTransport) handleExecuteTool(w http.ResponseWriter, r *http.Request) {
	toolName := chi.URLParam(r, "tool")

	t.toolsMutex.RLock()
	handler, exists := t.tools[toolName]
	t.toolsMutex.RUnlock()

	if !exists {
		t.sendError(w, http.StatusNotFound, fmt.Sprintf("Tool '%s' not found", toolName))
		return
	}

	// Parse request body
	var args map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		t.sendError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	// Execute tool
	ctx := r.Context()
	result, err := handler(ctx, args)
	if err != nil {
		t.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Tool execution failed: %v", err))
		return
	}

	t.sendJSON(w, http.StatusOK, result)
}

func (t *HTTPTransport) handleHealth(w http.ResponseWriter, r *http.Request) {
	t.sendJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	})
}

func (t *HTTPTransport) handleStatus(w http.ResponseWriter, r *http.Request) {
	t.toolsMutex.RLock()
	toolCount := len(t.tools)
	t.toolsMutex.RUnlock()

	t.sendJSON(w, http.StatusOK, map[string]interface{}{
		"status":           "running",
		"version":          "1.0.0",
		"tools_registered": toolCount,
		"transport":        "http",
		"port":             t.port,
		"rate_limit":       t.rateLimit,
		"timestamp":        time.Now().Unix(),
	})
}

func (t *HTTPTransport) handleListSessions(w http.ResponseWriter, r *http.Request) {
	// This would call the list_sessions tool
	t.toolsMutex.RLock()
	handler, exists := t.tools["list_sessions"]
	t.toolsMutex.RUnlock()

	if !exists {
		t.sendError(w, http.StatusNotFound, "Session management not available")
		return
	}

	result, err := handler(r.Context(), map[string]interface{}{})
	if err != nil {
		t.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list sessions: %v", err))
		return
	}

	t.sendJSON(w, http.StatusOK, result)
}

func (t *HTTPTransport) handleGetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	// Use list_sessions tool to get session details
	if listTool, exists := t.tools["list_sessions"]; exists {
		listResponse, err := listTool(r.Context(), map[string]interface{}{
			"session_id": sessionID,
		})
		if err != nil {
			t.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get session %s: %v", sessionID, err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(listResponse)
		return
	}

	t.sendError(w, http.StatusServiceUnavailable, "Session management not available")
}

func (t *HTTPTransport) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	t.toolsMutex.RLock()
	handler, exists := t.tools["delete_session"]
	t.toolsMutex.RUnlock()

	if !exists {
		t.sendError(w, http.StatusNotFound, "Session management not available")
		return
	}

	result, err := handler(r.Context(), map[string]interface{}{
		"session_id": sessionID,
	})
	if err != nil {
		t.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete session: %v", err))
		return
	}

	t.sendJSON(w, http.StatusOK, result)
}

// Helper methods

func (t *HTTPTransport) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		t.logger.Error().Err(err).Msg("Failed to encode JSON response")
	}
}

func (t *HTTPTransport) sendError(w http.ResponseWriter, status int, message string) {
	t.sendJSON(w, status, map[string]interface{}{
		"error":     message,
		"status":    status,
		"timestamp": time.Now().Unix(),
	})
}

func (t *HTTPTransport) checkRateLimit(clientIP string) bool {
	// Get or create rate limiter for this IP
	limiter, exists := t.rateLimiter[clientIP]
	if !exists {
		limiter = &rateLimiter{
			requests: make([]time.Time, 0),
		}
		t.rateLimiter[clientIP] = limiter
	}

	limiter.mutex.Lock()
	defer limiter.mutex.Unlock()

	now := time.Now()
	windowStart := now.Add(-1 * time.Minute)

	// Remove old requests
	validRequests := make([]time.Time, 0)
	for _, reqTime := range limiter.requests {
		if reqTime.After(windowStart) {
			validRequests = append(validRequests, reqTime)
		}
	}

	// Check if under limit
	if len(validRequests) >= t.rateLimit {
		return false
	}

	// Add current request
	limiter.requests = append(validRequests, now)
	return true
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// loggingResponseWriter captures response data for logging
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	body         []byte
	bytesWritten int
	logBodies    bool
	maxSize      int64
}

func (w *loggingResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *loggingResponseWriter) Write(data []byte) (int, error) {
	// Capture body for logging if enabled
	if w.logBodies && int64(len(w.body)) < w.maxSize {
		remaining := w.maxSize - int64(len(w.body))
		if remaining > 0 {
			toCopy := int64(len(data))
			if toCopy > remaining {
				toCopy = remaining
			}
			w.body = append(w.body, data[:toCopy]...)
		}
	}

	n, err := w.ResponseWriter.Write(data)
	w.bytesWritten += n
	return n, err
}
