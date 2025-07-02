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
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ToolInfo stores tool metadata
type ToolInfo struct {
	Handler     ToolHandler
	Description string
}

// HTTPTransport implements core.Transport for HTTP communication
type HTTPTransport struct {
	server         *http.Server
	mcpServer      interface{}
	router         chi.Router
	tools          map[string]*ToolInfo
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
		config.Port = types.DefaultHTTPPort
	}
	if config.RateLimit == 0 {
		config.RateLimit = types.DefaultRateLimitPerMinute
	}

	transport := &HTTPTransport{
		tools:          make(map[string]*ToolInfo),
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

	t.router.Use(middleware.Timeout(types.HTTPTimeoutSeconds))
}

// setupCORS creates CORS middleware
func (t *HTTPTransport) setupCORS() func(http.Handler) http.Handler {
	corsOptions := cors.Options{
		AllowedOrigins:   t.corsOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-API-Key"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           int(types.CORSMaxAgeSeconds.Seconds()),
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
		ReadTimeout:  types.HTTPTimeoutSeconds,
		WriteTimeout: types.HTTPTimeoutSeconds,
		IdleTimeout:  types.HTTPIdleTimeoutSeconds,
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

	ctx, cancel := context.WithTimeout(context.Background(), types.HTTPTimeoutSeconds)
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

	t.tools[name] = &ToolInfo{
		Handler:     toolHandler,
		Description: description,
	}
	t.logger.Info().Str("tool", name).Str("description", description).Msg("Registered tool with HTTP transport")
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

// getToolMetadata retrieves tool metadata from the MCP server if available
func (t *HTTPTransport) getToolMetadata(toolName string) *HTTPToolMetadata {
	if t.mcpServer == nil {
		return nil
	}

	// Try to get the tool registry which has full schema information
	type serverWithRegistry interface {
		GetToolRegistry() interface {
			GetToolSchema(string) (map[string]interface{}, error)
		}
	}

	if server, ok := t.mcpServer.(serverWithRegistry); ok {
		if registry := server.GetToolRegistry(); registry != nil {
			if schema, err := registry.GetToolSchema(toolName); err == nil {
				// Convert to HTTPToolMetadata
				metadata := t.convertSchemaToHTTPMetadata(schema, toolName)
				return metadata
			}
		}
	}

	// Fallback: Try to get the server instance
	type serverInterface interface {
		GetToolOrchestrator() interface {
			GetToolMetadata(string) (interface{}, error)
		}
	}

	if server, ok := t.mcpServer.(serverInterface); ok {
		if orchestrator := server.GetToolOrchestrator(); orchestrator != nil {
			if metadata, err := orchestrator.GetToolMetadata(toolName); err == nil {
				// Convert to HTTPToolMetadata
				if coreMetadata, ok := metadata.(core.ToolMetadata); ok {
					converted := ConvertCoreMetadata(coreMetadata)
					return &converted
				}
				// If not core.ToolMetadata, try map conversion
				if metaMap, ok := metadata.(map[string]interface{}); ok {
					return t.convertMapToHTTPMetadata(metaMap, toolName)
				}
			}
		}
	}

	return nil
}

// convertSchemaToHTTPMetadata converts a schema map to HTTPToolMetadata
func (t *HTTPTransport) convertSchemaToHTTPMetadata(schema map[string]interface{}, toolName string) *HTTPToolMetadata {
	metadata := &HTTPToolMetadata{
		Name: toolName,
	}

	// Extract basic fields with type safety
	if desc, ok := schema["description"].(string); ok && desc != "" {
		metadata.Description = desc
	} else {
		// Fall back to HTTP transport description if missing or empty
		t.toolsMutex.RLock()
		if info, exists := t.tools[toolName]; exists {
			metadata.Description = info.Description
		}
		t.toolsMutex.RUnlock()
	}

	if version, ok := schema["version"].(string); ok {
		metadata.Version = version
	}

	if category, ok := schema["category"].(string); ok {
		metadata.Category = category
	}

	// Convert dependencies
	if deps, ok := schema["dependencies"].([]interface{}); ok {
		metadata.Dependencies = make([]string, 0, len(deps))
		for _, dep := range deps {
			if depStr, ok := dep.(string); ok {
				metadata.Dependencies = append(metadata.Dependencies, depStr)
			}
		}
	}

	// Convert capabilities
	if caps, ok := schema["capabilities"].([]interface{}); ok {
		metadata.Capabilities = make([]string, 0, len(caps))
		for _, cap := range caps {
			if capStr, ok := cap.(string); ok {
				metadata.Capabilities = append(metadata.Capabilities, capStr)
			}
		}
	}

	// Convert requirements
	if reqs, ok := schema["requirements"].([]interface{}); ok {
		metadata.Requirements = make([]string, 0, len(reqs))
		for _, req := range reqs {
			if reqStr, ok := req.(string); ok {
				metadata.Requirements = append(metadata.Requirements, reqStr)
			}
		}
	}

	// Convert parameters schema
	if params, ok := schema["parameters"].(map[string]interface{}); ok {
		metadata.Parameters = t.convertToParameterSchema(params)
	}

	// Convert examples
	if examples, ok := schema["examples"].([]interface{}); ok {
		metadata.Examples = make([]HTTPToolExample, 0, len(examples))
		for _, ex := range examples {
			if exMap, ok := ex.(map[string]interface{}); ok {
				example := HTTPToolExample{}
				if name, ok := exMap["name"].(string); ok {
					example.Name = name
				}
				if desc, ok := exMap["description"].(string); ok {
					example.Description = desc
				}
				example.Input = exMap["input"]
				example.Output = exMap["output"]
				metadata.Examples = append(metadata.Examples, example)
			}
		}
	}

	return metadata
}

// convertMapToHTTPMetadata converts a generic map to HTTPToolMetadata
func (t *HTTPTransport) convertMapToHTTPMetadata(metaMap map[string]interface{}, toolName string) *HTTPToolMetadata {
	// Reuse the schema conversion logic since maps have similar structure
	return t.convertSchemaToHTTPMetadata(metaMap, toolName)
}

// convertToParameterSchema converts a parameters map to HTTPToolParameterSchema
func (t *HTTPTransport) convertToParameterSchema(params map[string]interface{}) HTTPToolParameterSchema {
	schema := HTTPToolParameterSchema{
		Type:       "object",
		Properties: make(map[string]HTTPParameterProperty),
		Required:   []string{},
	}

	if props, ok := params["properties"].(map[string]interface{}); ok {
		for propName, propData := range props {
			if propMap, ok := propData.(map[string]interface{}); ok {
				prop := HTTPParameterProperty{}

				if propType, ok := propMap["type"].(string); ok {
					prop.Type = propType
				}
				if desc, ok := propMap["description"].(string); ok {
					prop.Description = desc
				}
				if def := propMap["default"]; def != nil {
					prop.Default = def
				}
				if req, ok := propMap["required"].(bool); ok {
					prop.Required = req
				}
				if format, ok := propMap["format"].(string); ok {
					prop.Format = format
				}
				if pattern, ok := propMap["pattern"].(string); ok {
					prop.Pattern = pattern
				}
				if minLen, ok := propMap["minLength"].(float64); ok {
					minLenInt := int(minLen)
					prop.MinLength = &minLenInt
				}
				if maxLen, ok := propMap["maxLength"].(float64); ok {
					maxLenInt := int(maxLen)
					prop.MaxLength = &maxLenInt
				}
				if min, ok := propMap["minimum"].(float64); ok {
					prop.Minimum = &min
				}
				if max, ok := propMap["maximum"].(float64); ok {
					prop.Maximum = &max
				}

				schema.Properties[propName] = prop
			}
		}
	}

	if required, ok := params["required"].([]interface{}); ok {
		for _, req := range required {
			if reqStr, ok := req.(string); ok {
				schema.Required = append(schema.Required, reqStr)
			}
		}
	}

	return schema
}

// handleGetToolSchema returns the schema for a specific tool
func (t *HTTPTransport) handleGetToolSchema(w http.ResponseWriter, r *http.Request) {
	toolName := chi.URLParam(r, "tool")

	t.toolsMutex.RLock()
	toolInfo, exists := t.tools[toolName]
	t.toolsMutex.RUnlock()

	if !exists {
		t.sendError(w, http.StatusNotFound, fmt.Sprintf("Tool '%s' not found", toolName))
		return
	}

	response := map[string]interface{}{
		"name":        toolName,
		"description": toolInfo.Description,
	}

	// Get detailed metadata from the MCP server
	if metadata := t.getToolMetadata(toolName); metadata != nil {
		response["metadata"] = metadata
	}

	// Try to get the tool registry directly for more detailed schema
	if t.mcpServer != nil {
		type serverWithRegistry interface {
			GetToolRegistry() interface {
				GetToolSchema(string) (map[string]interface{}, error)
			}
		}

		if server, ok := t.mcpServer.(serverWithRegistry); ok {
			if registry := server.GetToolRegistry(); registry != nil {
				// Get full tool schema including parameters and output
				if schema, err := registry.GetToolSchema(toolName); err == nil {
					response["schema"] = schema
				} else {
					t.logger.Error().Err(err).Str("tool", toolName).Msg("Failed to get tool schema")
				}
			}
		}
	}

	t.sendJSON(w, http.StatusOK, response)
}

// structToMap converts a struct to a map using JSON marshaling
func (t *HTTPTransport) structToMap(v interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// handleGetAllToolSchemas returns schemas for all registered tools
func (t *HTTPTransport) handleGetAllToolSchemas(w http.ResponseWriter, r *http.Request) {
	t.toolsMutex.RLock()
	toolNames := make([]string, 0, len(t.tools))
	for name := range t.tools {
		toolNames = append(toolNames, name)
	}
	t.toolsMutex.RUnlock()

	schemas := make(map[string]interface{})

	// Get schemas from the tool registry
	if t.mcpServer != nil {
		type serverWithRegistry interface {
			GetToolRegistry() interface {
				GetToolSchema(string) (map[string]interface{}, error)
			}
		}

		if server, ok := t.mcpServer.(serverWithRegistry); ok {
			if registry := server.GetToolRegistry(); registry != nil {
				for _, toolName := range toolNames {
					if schema, err := registry.GetToolSchema(toolName); err == nil {
						// Ensure description is included from HTTP transport if missing
						if desc, ok := schema["description"].(string); !ok || desc == "" {
							t.toolsMutex.RLock()
							if info, exists := t.tools[toolName]; exists && info.Description != "" {
								schema["description"] = info.Description
							}
							t.toolsMutex.RUnlock()
						}
						schemas[toolName] = schema
					} else {
						t.logger.Error().Err(err).Str("tool", toolName).Msg("Failed to get tool schema")
						// Add basic info even if schema fails
						t.toolsMutex.RLock()
						if info, exists := t.tools[toolName]; exists {
							schemas[toolName] = map[string]interface{}{
								"name":        toolName,
								"description": info.Description,
								"error":       "Schema unavailable",
							}
						}
						t.toolsMutex.RUnlock()
					}
				}
			}
		}
	}

	// If no schemas were retrieved, return basic tool info
	if len(schemas) == 0 {
		t.toolsMutex.RLock()
		for name, info := range t.tools {
			schemas[name] = map[string]interface{}{
				"name":        name,
				"description": info.Description,
				"endpoint":    fmt.Sprintf("/api/v1/tools/%s", name),
			}
		}
		t.toolsMutex.RUnlock()
	}

	t.sendJSON(w, http.StatusOK, map[string]interface{}{
		"schemas": schemas,
		"count":   len(schemas),
	})
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

	// Check if detailed schema is requested
	includeSchema := r.URL.Query().Get("include_schema") == "true"

	tools := make([]map[string]interface{}, 0, len(t.tools))
	for name, info := range t.tools {
		toolInfo := map[string]interface{}{
			"name":        name,
			"description": info.Description,
			"endpoint":    fmt.Sprintf("/api/v1/tools/%s", name),
		}

		// Always include parameters for test compatibility
		if t.mcpServer != nil {
			if metadata := t.getToolMetadata(name); metadata != nil {
				// metadata is now *HTTPToolMetadata, so access fields directly
				toolInfo["parameters"] = metadata.Parameters

				// Include additional schema info if requested
				if includeSchema {
					toolInfo["schema"] = metadata.Parameters
					toolInfo["category"] = metadata.Category
					toolInfo["version"] = metadata.Version
				}
			} else {
				// Provide empty parameters if metadata not available
				toolInfo["parameters"] = HTTPToolParameterSchema{
					Type:       "object",
					Properties: make(map[string]HTTPParameterProperty),
					Required:   []string{},
				}
			}
		} else {
			// Provide empty parameters if server not available
			toolInfo["parameters"] = HTTPToolParameterSchema{
				Type:       "object",
				Properties: make(map[string]HTTPParameterProperty),
				Required:   []string{},
			}
		}

		tools = append(tools, toolInfo)
	}

	t.sendJSON(w, http.StatusOK, map[string]interface{}{
		"tools": tools,
		"count": len(tools),
	})
}

func (t *HTTPTransport) handleExecuteTool(w http.ResponseWriter, r *http.Request) {
	toolName := chi.URLParam(r, "tool")

	t.toolsMutex.RLock()
	toolInfo, exists := t.tools[toolName]
	t.toolsMutex.RUnlock()

	if !exists {
		t.sendError(w, http.StatusNotFound, fmt.Sprintf("Tool '%s' not found", toolName))
		return
	}

	// Use type-safe request parsing
	var executeRequest HTTPToolExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&executeRequest); err != nil {
		t.sendError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	// Validate the request
	if validationErrors := ValidateToolExecuteRequest(&executeRequest); len(validationErrors) > 0 {
		t.sendJSON(w, http.StatusBadRequest, HTTPValidationResponse{
			Success: false,
			Errors:  validationErrors,
			Message: "Request validation failed",
		})
		return
	}

	// Sanitize parameters
	sanitizedParams := SanitizeParameters(executeRequest.Parameters)

	ctx := r.Context()
	result, err := toolInfo.Handler(ctx, sanitizedParams)
	if err != nil {
		// Create structured error response
		httpErr := &HTTPError{
			Code:    500,
			Message: fmt.Sprintf("Tool execution failed: %v", err),
			Type:    "execution_error",
		}

		response := HTTPToolExecuteResponse{
			Success:     false,
			Error:       httpErr,
			ExecutionID: uuid.New().String(),
			Timestamp:   time.Now(),
		}

		t.sendJSON(w, http.StatusInternalServerError, response)
		return
	}

	// Create successful response
	response := HTTPToolExecuteResponse{
		Success:     true,
		Result:      result,
		ExecutionID: uuid.New().String(),
		Timestamp:   time.Now(),
	}

	t.sendJSON(w, http.StatusOK, response)
}

func (t *HTTPTransport) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := HTTPServerInfo{
		Name:      "Container Kit MCP Server",
		Version:   "1.0.0",
		Status:    "healthy",
		StartTime: time.Now(),             // This should be the actual start time in production
		Uptime:    time.Since(time.Now()), // This should be calculated from actual start time
	}
	t.sendJSON(w, http.StatusOK, response)
}

func (t *HTTPTransport) handleStatus(w http.ResponseWriter, r *http.Request) {
	t.toolsMutex.RLock()
	toolCount := len(t.tools)
	t.toolsMutex.RUnlock()

	response := HTTPServerInfo{
		Name:         "Container Kit MCP Server",
		Version:      "1.0.0",
		Status:       "running",
		StartTime:    time.Now(),             // This should be the actual start time in production
		Uptime:       time.Since(time.Now()), // This should be calculated from actual start time
		Capabilities: []string{"tool_execution", "http_transport"},
		Metadata: map[string]string{
			"tools_registered": fmt.Sprintf("%d", toolCount),
			"transport":        "http",
			"port":             fmt.Sprintf("%d", t.port),
			"rate_limit":       fmt.Sprintf("%d", t.rateLimit),
		},
	}
	t.sendJSON(w, http.StatusOK, response)
}

func (t *HTTPTransport) handleListSessions(w http.ResponseWriter, r *http.Request) {
	t.toolsMutex.RLock()
	handler, exists := t.tools["list_sessions"]
	t.toolsMutex.RUnlock()

	if !exists {
		t.sendError(w, http.StatusNotFound, "Session management not available")
		return
	}

	result, err := handler.Handler(r.Context(), map[string]interface{}{})
	if err != nil {
		t.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list sessions: %v", err))
		return
	}

	t.sendJSON(w, http.StatusOK, result)
}

func (t *HTTPTransport) handleGetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	// Get session details tool - this needs to be a specific tool for getting a single session
	if getSessionTool, exists := t.tools["get_session"]; exists {
		response, err := getSessionTool.Handler(r.Context(), map[string]interface{}{
			"session_id": sessionID,
		})
		if err != nil {
			t.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get session %s: %v", sessionID, err))
			return
		}

		t.sendJSON(w, http.StatusOK, response)
		return
	}

	// Fallback: try to use list_sessions and filter
	if listTool, exists := t.tools["list_sessions"]; exists {
		listResponse, err := listTool.Handler(r.Context(), map[string]interface{}{})
		if err != nil {
			t.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list sessions: %v", err))
			return
		}

		// Extract sessions from response and find the one we want
		if respMap, ok := listResponse.(map[string]interface{}); ok {
			if sessions, ok := respMap["sessions"].([]map[string]interface{}); ok {
				for _, session := range sessions {
					if sid, ok := session["session_id"].(string); ok && sid == sessionID {
						t.sendJSON(w, http.StatusOK, session)
						return
					}
				}
			}
		}

		t.sendError(w, http.StatusNotFound, fmt.Sprintf("Session %s not found", sessionID))
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

	result, err := handler.Handler(r.Context(), map[string]interface{}{
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
