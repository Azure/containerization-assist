package tools

import (
	"context"
	"encoding/json"
)

// Tool represents the canonical interface for all MCP tools
type Tool interface {
	// Core identification
	Name() string
	Description() string

	// Schema and validation
	InputSchema() *json.RawMessage
	Execute(ctx context.Context, input json.RawMessage) (*ExecutionResult, error)

	// Metadata
	Category() string
	Tags() []string
	Version() string
}

// ExecutionResult represents the result of tool execution
type ExecutionResult struct {
	Content  []ContentBlock `json:"content"`
	IsError  bool           `json:"isError,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// ContentBlock represents a piece of content in the result
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Data any    `json:"data,omitempty"`
}
