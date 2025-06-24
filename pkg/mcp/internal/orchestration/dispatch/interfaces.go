package dispatch

import (
	"context"
)

// Tool represents a common interface for all MCP atomic tools
type Tool interface {
	// Execute runs the tool with the provided arguments
	Execute(ctx context.Context, args interface{}) (interface{}, error)

	// GetMetadata returns metadata about the tool
	GetMetadata() ToolMetadata
}

// ToolArgs is a marker interface for tool-specific argument types
type ToolArgs interface {
	// Validate checks if the arguments are valid
	Validate() error
}

// ToolResult is a marker interface for tool-specific result types
type ToolResult interface {
	// IsSuccess indicates if the tool execution was successful
	IsSuccess() bool

	// GetError returns any error that occurred during execution
	GetError() error
}

// ToolMetadata contains information about a tool
type ToolMetadata struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Version      string            `json:"version"`
	Category     string            `json:"category"`
	Dependencies []string          `json:"dependencies"`
	Capabilities []string          `json:"capabilities"`
	Requirements []string          `json:"requirements"`
	Parameters   map[string]string `json:"parameters"`
	Examples     []ToolExample     `json:"examples"`
}

// ToolExample represents an example usage of a tool
type ToolExample struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	Output      map[string]interface{} `json:"output"`
}

// ToolFactory creates a new instance of a tool
type ToolFactory func() Tool

// ArgConverter converts generic arguments to tool-specific types
type ArgConverter func(args map[string]interface{}) (ToolArgs, error)

// ResultConverter converts tool-specific results to generic types
type ResultConverter func(result ToolResult) (map[string]interface{}, error)
