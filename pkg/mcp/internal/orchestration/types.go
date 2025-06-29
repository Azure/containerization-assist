package orchestration

// SessionManager interface for MCP session management
type SessionManager interface {
	GetSession(sessionID string) (interface{}, error)
	UpdateSession(sessionID string, updater func(interface{})) error
}

// NOTE: InternalToolRegistry and InternalToolOrchestrator interfaces are now defined in interfaces.go
// These local interfaces help avoid import cycles with pkg/mcp

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
