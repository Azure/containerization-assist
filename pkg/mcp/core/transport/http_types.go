package transport

import (
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
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
	Errors  []HTTPValidationError `json:"errors"`
	Message string                `json:"message"`
}

// ConvertCoreMetadata safely converts core.ToolMetadata to HTTPToolMetadata
func ConvertCoreMetadata(metadata core.ToolMetadata) HTTPToolMetadata {
	httpParams := HTTPToolParameterSchema{
		Type:       "object",
		Properties: make(map[string]HTTPParameterProperty),
		Required:   []string{},
	}

	// Convert parameters map to structured schema
	for paramName, paramDesc := range metadata.Parameters {
		httpParams.Properties[paramName] = HTTPParameterProperty{
			Type:        "string", // Default to string, could be enhanced with type detection
			Description: paramDesc,
		}
	}

	// Convert examples
	httpExamples := make([]HTTPToolExample, len(metadata.Examples))
	for i, example := range metadata.Examples {
		httpExamples[i] = HTTPToolExample{
			Name:        example.Name,
			Description: example.Description,
			Input:       example.Input,
			Output:      example.Output,
		}
	}

	return HTTPToolMetadata{
		Name:         metadata.Name,
		Description:  metadata.Description,
		Version:      metadata.Version,
		Category:     metadata.Category,
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
