package registry

// ToolInput represents the input parameters for tool execution
type ToolInput struct {
	SessionID  string                 `json:"session_id"`
	Name       string                 `json:"name"`
	Parameters map[string]interface{} `json:"parameters"`
}
