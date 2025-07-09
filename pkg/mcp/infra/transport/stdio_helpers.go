package transport

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors/codes"
	"github.com/rs/zerolog"
)

// JSONRPCResponse represents a standard JSON-RPC response
type JSONRPCResponse struct {
	ID      interface{}   `json:"id"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
	Version string        `json:"jsonrpc"`
}

// JSONRPCError represents a JSON-RPC error
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// CreateSuccessResponse creates a standard JSON-RPC success response
func CreateSuccessResponse(id interface{}, result interface{}) map[string]interface{} {
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}
}

// CreateErrorResponse creates a standard JSON-RPC error response
func CreateErrorResponse(id interface{}, code int, message string, data interface{}) map[string]interface{} {
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

	return response
}

// CreateErrorResponseFromError creates a JSON-RPC error response from a Go error
func CreateErrorResponseFromError(id interface{}, err error) map[string]interface{} {
	return CreateErrorResponse(id, -32000, err.Error(), nil)
}

// FormatMCPMessage formats a message for MCP protocol transmission
func FormatMCPMessage(message interface{}) ([]byte, error) {
	data, err := json.Marshal(message)
	if err != nil {
		networkErr := errors.NetworkError(
			codes.NETWORK_ERROR,
			"Failed to marshal MCP message",
			err,
		)
		networkErr.Context["message_type"] = fmt.Sprintf("%T", message)
		return nil, networkErr
	}

	// Add newline for stdio line-based communication
	data = append(data, '\n')
	return data, nil
}

// ParseJSONMessage parses a JSON message from bytes
func ParseJSONMessage(data []byte) (map[string]interface{}, error) {
	var message map[string]interface{}
	if err := json.Unmarshal(data, &message); err != nil {
		networkErr := errors.NetworkError(
			codes.NETWORK_ERROR,
			"Failed to parse JSON message",
			err,
		)
		networkErr.Context["data_length"] = len(data)
		return nil, networkErr
	}
	return message, nil
}

// LogTransportEvent logs a transport-related event with structured data
func LogTransportEvent(logger zerolog.Logger, event string, details map[string]interface{}) {
	logEvent := logger.Info().
		Str("event", event).
		Timestamp()

	// Add details to log
	for key, value := range details {
		switch v := value.(type) {
		case string:
			logEvent = logEvent.Str(key, v)
		case int:
			logEvent = logEvent.Int(key, v)
		case int64:
			logEvent = logEvent.Int64(key, v)
		case bool:
			logEvent = logEvent.Bool(key, v)
		case time.Duration:
			logEvent = logEvent.Dur(key, v)
		case error:
			logEvent = logEvent.Err(v)
		default:
			logEvent = logEvent.Interface(key, v)
		}
	}

	logEvent.Msg("Transport event")
}

// LogTransportError logs a transport-related error with context
func LogTransportError(logger zerolog.Logger, operation string, err error, context map[string]interface{}) {
	logEvent := logger.Error().
		Err(err).
		Str("operation", operation).
		Timestamp()

	// Add context to log
	for key, value := range context {
		switch v := value.(type) {
		case string:
			logEvent = logEvent.Str(key, v)
		case int:
			logEvent = logEvent.Int(key, v)
		case bool:
			logEvent = logEvent.Bool(key, v)
		default:
			logEvent = logEvent.Interface(key, v)
		}
	}

	logEvent.Msg("Transport operation failed")
}

// ValidateJSONRPCRequest validates basic JSON-RPC request structure
func ValidateJSONRPCRequest(request map[string]interface{}) error {
	if request == nil {
		return errors.NewError().
			Code(codes.VALIDATION_REQUIRED_MISSING).
			Message("Request cannot be nil").
			Build()
	}

	// Check for required fields
	if _, ok := request["method"]; !ok {
		return errors.NewError().
			Code(codes.VALIDATION_REQUIRED_MISSING).
			Message("Request missing 'method' field").
			Context("field", "method").
			Build()
	}

	if version, ok := request["jsonrpc"]; ok {
		if v, ok := version.(string); !ok || v != "2.0" {
			return errors.NewError().
				Code(codes.VALIDATION_FORMAT_INVALID).
				Message("Invalid jsonrpc version, expected '2.0'").
				Context("version", version).
				Suggestion("Set jsonrpc field to '2.0'").
				Build()
		}
	}

	return nil
}
