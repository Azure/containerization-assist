package mcp

import (
	"context"
	"time"
)

// Unified MCP Interfaces - Single Source of Truth
// This file consolidates all MCP interfaces as specified in REORG.md

// =============================================================================
// CORE TOOL INTERFACE
// =============================================================================

// Tool represents the unified interface for all MCP tools
// ALL tools (atomic, chat, session, server, etc.) MUST implement this interface
type Tool interface {
	// Execute runs the tool with the provided arguments
	Execute(ctx context.Context, args interface{}) (interface{}, error)

	// GetMetadata returns comprehensive metadata about the tool
	GetMetadata() ToolMetadata

	// Validate checks if the provided arguments are valid for this tool
	Validate(ctx context.Context, args interface{}) error
}

// ToolMetadata contains comprehensive information about a tool
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

// =============================================================================
// SESSION INTERFACE
// =============================================================================

// Session represents the unified interface for session management
type Session interface {
	// ID returns the unique session identifier
	ID() string

	// GetWorkspace returns the workspace directory path
	GetWorkspace() string

	// UpdateState applies a function to update the session state
	UpdateState(func(*SessionState))
}

// SessionState represents the current state of a session
type SessionState struct {
	SessionID string
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time

	// Workspace
	WorkspaceDir string

	// Repository state
	RepositoryAnalyzed bool
	RepositoryInfo     *RepositoryInfo
	RepoURL            string

	// Build state
	DockerfileGenerated bool
	DockerfilePath      string
	ImageBuilt          bool
	ImageRef            string
	ImagePushed         bool

	// Deployment state
	ManifestsGenerated  bool
	ManifestPaths       []string
	DeploymentValidated bool

	// Progress tracking
	CurrentStage string
	Status       string
	Stage        string
	Errors       []string
	Metadata     map[string]interface{}

	// Security
	SecurityScan *SecurityScanResult
}

// RepositoryInfo contains repository analysis information
type RepositoryInfo struct {
	Language    string                 `json:"language"`
	Framework   string                 `json:"framework"`
	Dependencies []string              `json:"dependencies"`
	EntryPoint  string                 `json:"entry_point"`
	Port        int                    `json:"port"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// SecurityScanResult contains security scan information
type SecurityScanResult struct {
	HasVulnerabilities bool     `json:"has_vulnerabilities"`
	CriticalCount      int      `json:"critical_count"`
	HighCount          int      `json:"high_count"`
	MediumCount        int      `json:"medium_count"`
	LowCount           int      `json:"low_count"`
	Vulnerabilities    []string `json:"vulnerabilities"`
	ScanTime           time.Time `json:"scan_time"`
}

// =============================================================================
// TRANSPORT INTERFACE
// =============================================================================

// Transport represents the unified interface for MCP transport mechanisms
type Transport interface {
	// Serve starts the transport and serves requests
	Serve(ctx context.Context) error

	// Stop gracefully stops the transport
	Stop() error
}

// =============================================================================
// ORCHESTRATOR INTERFACE
// =============================================================================

// Orchestrator represents the unified interface for tool orchestration
type Orchestrator interface {
	// ExecuteTool executes a tool by name with the provided arguments
	ExecuteTool(ctx context.Context, name string, args interface{}) (interface{}, error)

	// RegisterTool registers a tool with the orchestrator
	RegisterTool(name string, tool Tool) error
}

// =============================================================================
// TOOL ARGUMENT AND RESULT INTERFACES
// =============================================================================

// ToolArgs is a marker interface for tool-specific argument types
type ToolArgs interface {
	// GetSessionID returns the session ID for this tool execution
	GetSessionID() string
}

// ToolResult is a marker interface for tool-specific result types
type ToolResult interface {
	// GetSuccess returns whether the tool execution was successful
	GetSuccess() bool
}

// BaseToolArgs provides common fields for all tool arguments
type BaseToolArgs struct {
	SessionID string `json:"session_id" jsonschema:"required,description=Unique identifier for the session"`
}

// GetSessionID implements ToolArgs interface
func (b BaseToolArgs) GetSessionID() string {
	return b.SessionID
}

// BaseToolResponse provides common fields for all tool responses
type BaseToolResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Errors  []string               `json:"errors,omitempty"`
}

// GetSuccess implements ToolResult interface
func (b BaseToolResponse) GetSuccess() bool {
	return b.Success
}

// =============================================================================
// ERROR HANDLING INTERFACES
// =============================================================================

// RichError represents an enriched error with context
type RichError interface {
	error
	Code() string
	Context() map[string]interface{}
	Severity() string
}

// =============================================================================
// REGISTRY INTERFACE
// =============================================================================

// ToolRegistry manages tool registration and discovery
type ToolRegistry interface {
	// Register adds a tool to the registry
	Register(name string, tool Tool) error

	// Get retrieves a tool by name
	Get(name string) (Tool, bool)

	// List returns all registered tool names
	List() []string

	// GetMetadata returns metadata for a specific tool
	GetMetadata(name string) (ToolMetadata, bool)
}