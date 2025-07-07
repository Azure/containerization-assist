package types

import (
	"time"
)

// BaseToolResponse provides common response structure for all tools
// Moved from core package to break import cycles
type BaseToolResponse struct {
	Success   bool              `json:"success"`
	Message   string            `json:"message,omitempty"`
	Error     string            `json:"error,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// NewBaseResponse creates a new BaseToolResponse with current timestamp
func NewBaseResponse(success bool, message string) BaseToolResponse {
	return BaseToolResponse{
		Success:   success,
		Message:   message,
		Timestamp: time.Now(),
	}
}

// NewToolResponse creates a tool response with current metadata
func NewToolResponse(tool, sessionID string, dryRun bool) BaseToolResponse {
	return BaseToolResponse{
		Success:   true,
		Message:   "",
		Timestamp: time.Now(),
	}
}

// IsSuccess returns whether the operation was successful
func (b BaseToolResponse) IsSuccess() bool {
	return b.Success
}

// Recommendation represents an AI recommendation
// Moved from core package to break import cycles
type Recommendation struct {
	Type        string            `json:"type"`
	Priority    int               `json:"priority"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Action      string            `json:"action"`
	Metadata    map[string]string `json:"metadata"`
}

// AlternativeStrategy represents an alternative approach or strategy
// Moved from core package to break import cycles
type AlternativeStrategy struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Priority    int      `json:"priority"`
	Pros        []string `json:"pros"`
	Cons        []string `json:"cons"`
}

// ToolExample represents tool usage example
// Moved from core package to break import cycles
type ToolExample struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	Output      map[string]interface{} `json:"output"`
}
