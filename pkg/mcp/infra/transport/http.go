package transport

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors/codes"
	"github.com/Azure/container-kit/pkg/mcp/domain/types"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// Core HTTP transport implementation

// NewHTTPTransport creates a new HTTP transport
func NewHTTPTransport(config HTTPTransportConfig) *HTTPTransport {
	if config.Port == 0 {
		config.Port = types.DefaultHTTPPort
	}
	if config.RateLimit == 0 {
		config.RateLimit = types.DefaultRateLimitPerMinute
	}

	transport := &HTTPTransport{
		tools:          make(map[string]*ToolInfo),
		logger:         config.Logger.With("component", "http_transport"),
		port:           config.Port,
		corsOrigins:    config.CORSOrigins,
		apiKey:         config.APIKey,
		rateLimit:      config.RateLimit,
		rateLimiter:    make(map[string]*rateLimiter),
		logBodies:      config.LogBodies,
		maxBodyLogSize: config.MaxBodyLogSize,
		startTime:      time.Now(),
	}

	if transport.maxBodyLogSize == 0 {
		transport.maxBodyLogSize = types.DefaultMaxBodyLogSize
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
		r.Get("/tools/schemas", t.handleGetAllToolSchemas)
		r.Options("/tools/schemas", t.handleOptions)
		r.Get("/tools/{tool}/schema", t.handleGetToolSchema)
		r.Options("/tools/{tool}/schema", t.handleOptions)
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

	t.router.Use(middleware.Timeout(types.HTTPTimeout))
}

// setupCORS creates CORS middleware
func (t *HTTPTransport) setupCORS() func(http.Handler) http.Handler {
	corsOptions := cors.Options{
		AllowedOrigins:   t.corsOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-API-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           int(types.CORSMaxAge.Seconds()),
	}

	if len(t.corsOrigins) == 0 || (len(t.corsOrigins) == 1 && t.corsOrigins[0] == "*") {
		corsOptions.AllowedOrigins = []string{"*"}
		corsOptions.AllowCredentials = false
	}

	return cors.Handler(corsOptions)
}

// Serve starts the HTTP server and handles requests
func (t *HTTPTransport) Serve(ctx context.Context) error {
	if t.handler == nil {
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
			"Request handler not set",
			nil,
		)
		systemErr.Context["component"] = "http_transport"
		return systemErr
	}
	t.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", t.port),
		Handler:      t.router,
		ReadTimeout:  types.HTTPTimeout,
		WriteTimeout: types.HTTPTimeout,
		IdleTimeout:  types.HTTPIdleTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		t.logger.Info("Starting HTTP transport", "port", t.port)
		if err := t.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			networkErr := errors.NetworkError(
				codes.NETWORK_ERROR,
				"Failed to start HTTP server",
				err,
			)
			networkErr.Context["address"] = t.server.Addr
			networkErr.Context["component"] = "http_transport"
			errCh <- networkErr
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

	ctx, cancel := context.WithTimeout(context.Background(), types.HTTPTimeout)
	defer cancel()

	t.logger.Info("Stopping HTTP transport")
	return t.server.Shutdown(ctx)
}

// SetHandler sets the request handler for this transport
// SetHandler sets the request handler (implements core.Transport)
func (t *HTTPTransport) SetHandler(handler core.RequestHandler) {
	t.handler = handler
}

// HandleRequest handles MCP requests directly (consolidated from RequestHandler)
func (t *HTTPTransport) HandleRequest(ctx context.Context, request *core.MCPRequest) (*core.MCPResponse, error) {
	if t.handler == nil {
		return nil, errors.NewError().Messagef("no request handler configured").Build()
	}
	return t.handler.HandleRequest(ctx, request)
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

	t.logger.Info("Stopping HTTP transport")
	return t.server.Shutdown(ctx)
}

// SendMessage not applicable for HTTP transport (legacy method for backward compatibility)
func (t *HTTPTransport) SendMessage(message interface{}) error {
	systemErr := errors.SystemError(
		codes.SYSTEM_ERROR,
		"SendMessage not applicable for HTTP transport",
		nil,
	)
	systemErr.Context["component"] = "http_transport"
	systemErr.Suggestions = append(systemErr.Suggestions, "HTTP transport uses request/response pattern")
	return systemErr
}

// SendTypedMessage provides typed alternative to SendMessage
func (t *HTTPTransport) SendTypedMessage(message *ToolExecutionRequest) (*ToolExecutionResponse, error) {
	systemErr := errors.SystemError(
		codes.SYSTEM_ERROR,
		"SendTypedMessage not applicable for HTTP transport",
		nil,
	)
	systemErr.Context["component"] = "http_transport"
	systemErr.Suggestions = append(systemErr.Suggestions, "HTTP transport uses request/response pattern")
	return nil, systemErr
}

// ReceiveMessage not applicable for HTTP transport (legacy method for backward compatibility)
func (t *HTTPTransport) ReceiveMessage() (interface{}, error) {
	systemErr := errors.SystemError(
		codes.SYSTEM_ERROR,
		"ReceiveMessage not applicable for HTTP transport",
		nil,
	)
	systemErr.Context["component"] = "http_transport"
	systemErr.Suggestions = append(systemErr.Suggestions, "HTTP transport uses request/response pattern")
	return nil, systemErr
}

// ReceiveTypedMessage provides typed alternative to ReceiveMessage
func (t *HTTPTransport) ReceiveTypedMessage() (*ToolExecutionRequest, error) {
	systemErr := errors.SystemError(
		codes.SYSTEM_ERROR,
		"ReceiveTypedMessage not applicable for HTTP transport",
		nil,
	)
	systemErr.Context["component"] = "http_transport"
	systemErr.Suggestions = append(systemErr.Suggestions, "HTTP transport uses request/response pattern")
	return nil, systemErr
}

// Name returns the transport name
func (t *HTTPTransport) Name() string {
	return "http"
}

// RegisterTool registers a tool handler (legacy method for backward compatibility)
func (t *HTTPTransport) RegisterTool(name, description string, handler interface{}) error {
	return t.RegisterToolTyped(name, description, handler)
}

// RegisterToolTyped registers a tool handler with type safety (legacy interface{} parameter for backward compatibility)
func (t *HTTPTransport) RegisterToolTyped(name, description string, handler interface{}) error {
	t.toolsMutex.Lock()
	defer t.toolsMutex.Unlock()

	toolHandler, ok := handler.(ToolHandler)
	if !ok {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Message("Handler must be of type ToolHandler").
			Context("expected_type", "ToolHandler").
			Context("actual_type", fmt.Sprintf("%T", handler)).
			Build()
	}

	t.tools[name] = &ToolInfo{
		Handler:     toolHandler,
		Description: description,
	}
	t.logger.Info("Registered tool with HTTP transport", "tool", name, "description", description)
	return nil
}

// RegisterTypedToolHandler registers a tool handler with full type safety
func (t *HTTPTransport) RegisterTypedToolHandler(name, description string, handler ToolHandler) error {
	t.toolsMutex.Lock()
	defer t.toolsMutex.Unlock()

	t.tools[name] = &ToolInfo{
		Handler:     handler,
		Description: description,
	}
	t.logger.Info("Registered typed tool handler with HTTP transport", "tool", name, "description", description)
	return nil
}

// SetServer sets the MCP server for integration with gomcp (legacy method for backward compatibility)
func (t *HTTPTransport) SetServer(srv interface{}) {
	if coreServer, ok := srv.(core.Server); ok {
		t.mcpServer = coreServer
		t.logger.Debug("MCP server set for HTTP transport")
	} else {
		t.logger.Warn("Server does not implement core.Server interface")
	}
}

// SetTypedServer sets the MCP server with type safety
func (t *HTTPTransport) SetTypedServer(srv core.Server) {
	t.mcpServer = srv
	t.logger.Debug("Typed MCP server set for HTTP transport")
}

// GetServer returns the underlying MCP server
func (t *HTTPTransport) GetServer() core.Server {
	return t.mcpServer
}

// GetServerAsInterface returns the server as interface{} for backward compatibility
func (t *HTTPTransport) GetServerAsInterface() interface{} {
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
