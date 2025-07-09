package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/domain/tools"
	"github.com/go-chi/chi/v5"
)

// HTTPToolMetadata represents type-safe tool metadata for HTTP responses
type HTTPToolMetadata struct {
	Name         string                  `json:"name"`
	Description  string                  `json:"description"`
	Version      string                  `json:"version"`
	Category     string                  `json:"category"`
	Dependencies []string                `json:"dependencies"`
	Capabilities []string                `json:"capabilities"`
	Requirements []string                `json:"requirements"`
	Parameters   HTTPToolParameterSchema `json:"parameters"`
	Examples     []HTTPToolExample       `json:"examples"`
}

// HTTPToolParameterSchema represents the JSON schema for tool parameters
type HTTPToolParameterSchema struct {
	Type       string                           `json:"type"`
	Properties map[string]HTTPParameterProperty `json:"properties"`
	Required   []string                         `json:"required"`
}

// HTTPParameterProperty represents a single parameter property
type HTTPParameterProperty struct {
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Default     interface{} `json:"default,omitempty"`
	Required    bool        `json:"required,omitempty"`
	Format      string      `json:"format,omitempty"`
	Pattern     string      `json:"pattern,omitempty"`
	MinLength   *int        `json:"minLength,omitempty"`
	MaxLength   *int        `json:"maxLength,omitempty"`
	Minimum     *float64    `json:"minimum,omitempty"`
	Maximum     *float64    `json:"maximum,omitempty"`
}

// HTTPToolExample represents tool usage example for HTTP responses
type HTTPToolExample struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Input       interface{} `json:"input"`  // Still interface{} for JSON compatibility but validated
	Output      interface{} `json:"output"` // Still interface{} for JSON compatibility but validated
}

// HTTPToolExecuteRequest represents a type-safe HTTP tool execution request
type HTTPToolExecuteRequest struct {
	Parameters map[string]interface{} `json:"parameters"` // Tool-specific parameters
	Options    HTTPExecuteOptions     `json:"options,omitempty"`
}

// HTTPExecuteOptions represents execution options for HTTP tool requests
type HTTPExecuteOptions struct {
	DryRun    bool              `json:"dry_run,omitempty"`
	Timeout   time.Duration     `json:"timeout,omitempty"`
	SessionID string            `json:"session_id,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// HTTPToolExecuteResponse represents a type-safe HTTP tool execution response
type HTTPToolExecuteResponse struct {
	Success     bool              `json:"success"`
	Result      interface{}       `json:"result,omitempty"`
	Error       *HTTPError        `json:"error,omitempty"`
	ExecutionID string            `json:"execution_id,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
	Duration    time.Duration     `json:"duration,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// HTTPError represents a structured error response
type HTTPError struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Type    string                 `json:"type,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// HTTPToolListResponse represents the response for listing tools
type HTTPToolListResponse struct {
	Tools []HTTPToolInfo `json:"tools"`
	Total int            `json:"total"`
}

// HTTPToolInfo represents basic tool information for listings
type HTTPToolInfo struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Version      string   `json:"version"`
	Category     string   `json:"category"`
	Capabilities []string `json:"capabilities"`
}

// HTTPServerInfo represents server information for health checks
type HTTPServerInfo struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Status       string            `json:"status"`
	Uptime       time.Duration     `json:"uptime"`
	StartTime    time.Time         `json:"start_time"`
	Capabilities []string          `json:"capabilities"`
	Metadata     map[string]string `json:"metadata"`
}

// HTTPValidationError represents validation errors in HTTP requests
type HTTPValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// HTTPValidationResponse represents validation error response
type HTTPValidationResponse struct {
	Success bool                  `json:"success"`
	Errors  []HTTPValidationError `json:"github.com/Azure/container-kit/pkg/mcp/application/internal"`
	Message string                `json:"message"`
}

// ConvertCoreMetadata safely converts api.ToolMetadata to HTTPToolMetadata
func ConvertCoreMetadata(metadata api.ToolMetadata) HTTPToolMetadata {
	httpParams := HTTPToolParameterSchema{
		Type:       "object",
		Properties: make(map[string]HTTPParameterProperty),
		Required:   []string{},
	}

	// Note: api.ToolMetadata doesn't have Parameters or Examples fields
	// Using empty defaults for HTTP representation
	httpExamples := make([]HTTPToolExample, 0)

	return HTTPToolMetadata{
		Name:         metadata.Name,
		Description:  metadata.Description,
		Version:      metadata.Version,
		Category:     string(metadata.Category),
		Dependencies: metadata.Dependencies,
		Capabilities: metadata.Capabilities,
		Requirements: metadata.Requirements,
		Parameters:   httpParams,
		Examples:     httpExamples,
	}
}

// ValidateToolExecuteRequest validates incoming tool execution requests
func ValidateToolExecuteRequest(req *HTTPToolExecuteRequest) []HTTPValidationError {
	var errors []HTTPValidationError

	// Validate required fields and basic structure
	if req.Parameters == nil {
		errors = append(errors, HTTPValidationError{
			Field:   "parameters",
			Message: "parameters field is required",
			Code:    "MISSING_REQUIRED_FIELD",
		})
	}

	// Validate options if provided
	if req.Options.Timeout < 0 {
		errors = append(errors, HTTPValidationError{
			Field:   "options.timeout",
			Message: "timeout must be non-negative",
			Code:    "INVALID_VALUE",
		})
	}

	// Validate metadata values are strings only
	for key, value := range req.Options.Metadata {
		if key == "" {
			errors = append(errors, HTTPValidationError{
				Field:   "options.metadata",
				Message: "metadata keys cannot be empty",
				Code:    "INVALID_KEY",
			})
		}
		if len(value) > 1000 { // Reasonable limit
			errors = append(errors, HTTPValidationError{
				Field:   "options.metadata." + key,
				Message: "metadata value too long (max 1000 characters)",
				Code:    "VALUE_TOO_LONG",
			})
		}
	}

	return errors
}

// SanitizeParameters removes potentially dangerous values from parameters
func SanitizeParameters(params map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})

	for key, value := range params {
		// Skip empty keys
		if key == "" {
			continue
		}

		// Recursively sanitize nested maps
		if nestedMap, ok := value.(map[string]interface{}); ok {
			sanitized[key] = SanitizeParameters(nestedMap)
			continue
		}

		// Sanitize string values
		if strVal, ok := value.(string); ok {
			// Remove null bytes and excessive whitespace
			sanitized[key] = sanitizeString(strVal)
			continue
		}

		// Keep other primitive types as-is (numbers, booleans)
		switch value.(type) {
		case int, int8, int16, int32, int64:
			sanitized[key] = value
		case uint, uint8, uint16, uint32, uint64:
			sanitized[key] = value
		case float32, float64:
			sanitized[key] = value
		case bool:
			sanitized[key] = value
		case []interface{}:
			// Sanitize array elements
			if arr, ok := value.([]interface{}); ok {
				sanitizedArr := make([]interface{}, len(arr))
				for i, elem := range arr {
					if elemMap, ok := elem.(map[string]interface{}); ok {
						sanitizedArr[i] = SanitizeParameters(elemMap)
					} else if elemStr, ok := elem.(string); ok {
						sanitizedArr[i] = sanitizeString(elemStr)
					} else {
						sanitizedArr[i] = elem
					}
				}
				sanitized[key] = sanitizedArr
			}
		default:
			// Skip unknown types for security
			continue
		}
	}

	return sanitized
}

// sanitizeString removes potentially dangerous characters from strings
func sanitizeString(s string) string {
	// Remove null bytes
	result := ""
	for _, r := range s {
		if r != 0 {
			result += string(r)
		}
	}

	// Limit length to prevent excessive memory usage
	if len(result) > 10000 {
		result = result[:10000]
	}

	return result
}

// Additional types from main http.go file

// ToolExecutionRequest represents a typed tool execution request (MCP style)
type ToolExecutionRequest struct {
	Args      json.RawMessage  `json:"args"`
	SessionID string           `json:"session_id,omitempty"`
	Options   ExecutionOptions `json:"options,omitempty"`
}

// ExecutionOptions represents tool execution options (MCP style)
type ExecutionOptions struct {
	Timeout *time.Duration `json:"timeout,omitempty"`
	DryRun  bool           `json:"dry_run,omitempty"`
	Verbose bool           `json:"verbose,omitempty"`
	Async   bool           `json:"async,omitempty"`
}

// ToolExecutionResponse represents a typed tool execution response (MCP style)
type ToolExecutionResponse struct {
	Success     bool            `json:"success"`
	Result      json.RawMessage `json:"result,omitempty"`
	Error       *ErrorResponse  `json:"error,omitempty"`
	ExecutionID string          `json:"execution_id,omitempty"`
	Duration    time.Duration   `json:"duration"`
	Timestamp   time.Time       `json:"timestamp"`
}

// ErrorResponse represents a typed error response (MCP style)
type ErrorResponse struct {
	Code    string                   `json:"code"`
	Message string                   `json:"message"`
	Details *tools.TypedErrorDetails `json:"details,omitempty"`
	Type    string                   `json:"type,omitempty"`
}

// ToolListResponse represents the response for listing tools (MCP style)
type ToolListResponse struct {
	Tools []ToolDescription `json:"tools"`
	Count int               `json:"count"`
}

// ToolDescription represents a tool's metadata (MCP style)
type ToolDescription struct {
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Version     string                  `json:"version,omitempty"`
	Category    string                  `json:"category,omitempty"`
	Schema      *tools.JSONSchema       `json:"schema,omitempty"`
	Example     *tools.TypedToolExample `json:"example,omitempty"`
}

// HealthResponse represents health check response
type HealthResponse struct {
	Status    string           `json:"status"`
	Timestamp time.Time        `json:"timestamp"`
	Version   string           `json:"version,omitempty"`
	Uptime    time.Duration    `json:"uptime"`
	Metrics   map[string]int64 `json:"metrics,omitempty"`
}

// SessionListResponse represents session list response
type SessionListResponse struct {
	Sessions []SessionInfo `json:"sessions"`
	Count    int           `json:"count"`
}

// SessionInfo represents session information
type SessionInfo struct {
	ID         string                      `json:"id"`
	CreatedAt  time.Time                   `json:"created_at"`
	LastAccess time.Time                   `json:"last_access"`
	Status     string                      `json:"status"`
	Metadata   *tools.TypedSessionMetadata `json:"metadata,omitempty"`
}

// ToolInfo stores tool metadata
type ToolInfo struct {
	Handler     ToolHandler
	Description string
}

// HTTPTransport implements core.Transport for HTTP communication
type HTTPTransport struct {
	server         *http.Server
	mcpServer      core.Server
	router         chi.Router
	tools          map[string]*ToolInfo
	toolsMutex     sync.RWMutex
	logger         *slog.Logger
	port           int
	corsOrigins    []string
	apiKey         string
	rateLimit      int
	rateLimiter    map[string]*rateLimiter
	logBodies      bool
	maxBodyLogSize int64
	handler        core.RequestHandler
	startTime      time.Time
}

// HTTPTransportConfig holds configuration for HTTP transport
type HTTPTransportConfig struct {
	Port           int
	CORSOrigins    []string
	APIKey         string
	RateLimit      int
	Logger         *slog.Logger
	LogBodies      bool
	MaxBodyLogSize int64
	LogLevel       string
}

// ToolHandler is the tool handler function signature
type ToolHandler func(ctx context.Context, args interface{}) (interface{}, error)

// TypedToolHandler is the typed tool handler function signature
type TypedToolHandler func(ctx context.Context, req *ToolExecutionRequest) (*ToolExecutionResponse, error)

// rateLimiter tracks request rates
type rateLimiter struct {
	requests []time.Time
	mutex    sync.Mutex
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

// TypedServerRegistry has been consolidated into the core mcp.Registry interface

// NOTE: TypedServerInterface, TypedOrchestrator, TypedMessageHandler, and TypedMessageTransport
// were removed as they were unused interfaces. If needed, use the canonical interfaces
// from pkg/mcp/api/interfaces.go instead.
