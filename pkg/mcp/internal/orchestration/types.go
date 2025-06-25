package orchestration

import (
	"context"
)

// SessionManager interface for MCP session management
type SessionManager interface {
	GetSession(sessionID string) (interface{}, error)
	UpdateSession(session interface{}) error
}

// NOTE: ToolRegistry and Orchestrator interfaces are now defined in pkg/mcp/interfaces.go
// Use mcp.ToolRegistry and mcp.Orchestrator for the unified interfaces

// ToolMetadata contains metadata about a tool
type ToolMetadata struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Version      string                 `json:"version"`
	Category     string                 `json:"category"`
	Dependencies []string               `json:"dependencies"`
	Capabilities []string               `json:"capabilities"`
	Requirements []string               `json:"requirements"`
	Parameters   map[string]interface{} `json:"parameters"`
	OutputSchema map[string]interface{} `json:"output_schema"`
	Examples     []ToolExample          `json:"examples"`
}

// ToolExample represents an example of tool usage
type ToolExample struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Input       interface{} `json:"input"`
	Output      interface{} `json:"output"`
}
