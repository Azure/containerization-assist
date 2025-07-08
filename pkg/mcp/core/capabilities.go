package core

// ToolCapabilities defines the operational capabilities of a tool
// This is part of the contract between MCP server and hosting LLM
type ToolCapabilities struct {
	SupportsDryRun    bool `json:"supports_dry_run"`   // Tool can simulate operations without side effects
	SupportsStreaming bool `json:"supports_streaming"` // Tool can provide streaming progress updates
	IsLongRunning     bool `json:"is_long_running"`    // Tool may take significant time to complete
	RequiresAuth      bool `json:"requires_auth"`      // Tool requires authentication/credentials
}
