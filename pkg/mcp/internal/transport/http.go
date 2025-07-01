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

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// HTTPTransport implements core.Transport for HTTP communication
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
	handler        core.RequestHandler // Use core.RequestHandler instead of LocalRequestHandler
}

// HTTPTransportConfig holds configuration for HTTP transport
type HTTPTransportConfig struct {
	Port           int
	CORSOrigins    []string
	APIKey         string
	RateLimit      int
	Logger         zerolog.Logger
	LogBodies      bool
	MaxBodyLogSize int64
	LogLevel       string
}

// ToolHandler is the tool handler function signature
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

	if transport.maxBodyLogSize == 0 {
		transport.maxBodyLogSize = 10 * 1024
	}

	transport.setupRouter()
	return transport
}

// NewCoreHTTPTransport creates a new HTTP transport that implements core.Transport
func NewCoreHTTPTransport(config HTTPTransportConfig) core.Transport {
	return NewHTTPTransport(config)
}

// setupRouter initializes HTTP router and middleware
func (t *HTTPTransport) setupRouter() {
	t.router = chi.NewRouter()

	t.setupMiddlewareChain()

	t.router.Route("/api/v1", func(r chi.Router) {
		r.Get("/tools", t.handleListTools)
		r.Options("/tools", t.handleOptions)
		r.Post("/tools/{tool}", t.handleExecuteTool)
		r.Options("/tools/{tool}", t.handleOptions)

		r.Get("/health", t.handleHealth)
		r.Get("/status", t.handleStatus)

		r.Get("/sessions", t.handleListSessions)
		r.Options("/sessions", t.handleOptions)
		r.Get("/sessions/{sessionID}", t.handleGetSession)
		r.Options("/sessions/{sessionID}", t.handleOptions)
		r.Delete("/sessions/{sessionID}", t.handleDeleteSession)
		r.Options("/sessions/{sessionID}", t.handleOptions)
	})
}

// setupMiddlewareChain configures middleware chain
func (t *HTTPTransport) setupMiddlewareChain() {
	t.router.Use(middleware.RequestID)
	t.router.Use(middleware.RealIP)
	t.router.Use(middleware.Recoverer)

	t.router.Use(t.setupCORS())

	t.router.Use(t.rateLimitMiddleware)

	t.router.Use(t.authMiddleware)

	t.router.Use(t.loggingMiddleware)

	t.router.Use(middleware.Timeout(30 * time.Second))
}

// setupCORS creates CORS middleware
func (t *HTTPTransport) setupCORS() func(http.Handler) http.Handler {
	corsOptions := cors.Options{
		AllowedOrigins:   t.corsOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-API-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // 5 minutes
	}

	if len(t.corsOrigins) == 0 || (len(t.corsOrigins) == 1 && t.corsOrigins[0] == "*") {
		corsOptions.AllowedOrigins = []string{"*"}
		corsOptions.AllowCredentials = false
	}

	return cors.Handler(corsOptions)
}

// handleOptions handles preflight OPTIONS requests
func (t *HTTPTransport) handleOptions(w http.ResponseWriter, r *http.Request) {
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

	errCh := make(chan error, 1)
	go func() {
		t.logger.Info().Int("port", t.port).Msg("Starting HTTP transport")
		if err := t.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("failed to start HTTP server: %w", err)
		}
	}()

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
// SetHandler sets the request handler (implements core.Transport)
func (t *HTTPTransport) SetHandler(handler core.RequestHandler) {
	t.handler = handler
}

// Start starts the HTTP transport
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

// SendMessage not applicable for HTTP transport
func (t *HTTPTransport) SendMessage(message interface{}) error {
	return fmt.Errorf("SendMessage not applicable for HTTP transport")
}

// ReceiveMessage not applicable for HTTP transport
func (t *HTTPTransport) ReceiveMessage() (interface{}, error) {
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
	t.logger.Info().Str("tool", name).Msg("Registered tool with HTTP transport")
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

// GetRouter returns the HTTP router for testing
func (t *HTTPTransport) GetRouter() http.Handler {
	return t.router
}

func (t *HTTPTransport) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		requestID := middleware.GetReqID(r.Context())
		if requestID == "" {
			requestID = uuid.New().String()
		}

		logEvent := t.logger.Info().
			Str("request_id", requestID).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Str("user_agent", r.UserAgent())

		if t.logBodies {
			headers := make(map[string]string)
			for k, v := range r.Header {
				if k != "Authorization" && k != "Api-Key" {
					headers[k] = strings.Join(v, ", ")
				}
			}
			logEvent.Interface("request_headers", headers)
		}

		if t.logBodies && r.Body != nil {
			bodyReader := io.LimitReader(r.Body, t.maxBodyLogSize)
			requestBody, err := io.ReadAll(bodyReader)
			if err != nil {
				t.logger.Debug().Err(err).Msg("Failed to read request body")
			}
			if err := r.Body.Close(); err != nil {
				t.logger.Debug().Err(err).Msg("Failed to close request body")
			}

			r.Body = io.NopCloser(bytes.NewReader(requestBody))

			if len(requestBody) > 0 {
				logEvent.RawJSON("request_body", requestBody)
			}
		}

		logEvent.Msg("HTTP request received")

		wrapped := &loggingResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			logBodies:      t.logBodies,
			maxSize:        t.maxBodyLogSize,
		}

		next.ServeHTTP(wrapped, r)

		responseLog := t.logger.Info().
			Str("request_id", requestID).
			Int("status", wrapped.statusCode).
			Dur("duration", time.Since(start)).
			Int("response_size", wrapped.bytesWritten)

		if t.logBodies && len(wrapped.body) > 0 {
			responseLog.RawJSON("response_body", wrapped.body)
		}

		responseLog.Msg("HTTP response sent")

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
		if r.URL.Path == "/api/v1/health" {
			next.ServeHTTP(w, r)
			return
		}

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
		clientIP := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			clientIP = strings.Split(forwarded, ",")[0]
		}

		if !t.checkRateLimit(clientIP) {
			t.sendError(w, http.StatusTooManyRequests, "Rate limit exceeded")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (t *HTTPTransport) handleListTools(w http.ResponseWriter, r *http.Request) {
	t.toolsMutex.RLock()
	defer t.toolsMutex.RUnlock()

	t.logger.Debug().Int("tool_count", len(t.tools)).Msg("Listing tools")

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

	var args map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		t.sendError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

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

	validRequests := make([]time.Time, 0)
	for _, reqTime := range limiter.requests {
		if reqTime.After(windowStart) {
			validRequests = append(validRequests, reqTime)
		}
	}

	if len(validRequests) >= t.rateLimit {
		return false
	}

	limiter.requests = append(validRequests, now)
	return true
}

// responseWriter wraps http.ResponseWriter to capture status
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// loggingResponseWriter captures response data
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
