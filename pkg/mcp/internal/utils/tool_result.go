package utils

import (
	"fmt"
	"time"
)

// StandardToolResult provides a consistent structure for tool execution results
type StandardToolResult struct {
	Success   bool                   `json:"success"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Duration  time.Duration          `json:"duration"`
	Timestamp time.Time              `json:"timestamp"`
}

// NewSuccessResult creates a successful tool result
func NewSuccessResult(message string, data map[string]interface{}) *StandardToolResult {
	return &StandardToolResult{
		Success:   true,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// NewErrorResult creates a failed tool result
func NewErrorResult(message string, err error) *StandardToolResult {
	return &StandardToolResult{
		Success:   false,
		Message:   message,
		Error:     err.Error(),
		Timestamp: time.Now(),
	}
}

// WithDuration adds execution duration to the result
func (r *StandardToolResult) WithDuration(duration time.Duration) *StandardToolResult {
	r.Duration = duration
	return r
}

// ToMap converts the result to a map for compatibility with existing code
func (r *StandardToolResult) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"success":   r.Success,
		"message":   r.Message,
		"timestamp": r.Timestamp,
	}

	if r.Duration > 0 {
		result["duration"] = r.Duration.Seconds()
	}

	if r.Data != nil {
		for k, v := range r.Data {
			result[k] = v
		}
	}

	if r.Error != "" {
		result["error"] = r.Error
	}

	return result
}

// String returns a string representation of the result
func (r *StandardToolResult) String() string {
	if r.Success {
		return fmt.Sprintf("SUCCESS: %s", r.Message)
	}
	return fmt.Sprintf("ERROR: %s (%s)", r.Message, r.Error)
}
