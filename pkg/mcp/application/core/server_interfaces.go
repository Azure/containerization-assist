package core

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/rs/zerolog"
)

// Transport provides unified transport abstraction with integrated request handling.
type Transport interface {
	Serve(ctx context.Context) error
	Stop(ctx context.Context) error
	Name() string

	HandleRequest(ctx context.Context, request *MCPRequest) (*MCPResponse, error)
	SetHandler(handler RequestHandler)
}

// ToolOrchestrator provides unified tool orchestration functionality with both legacy and type-safe methods.
type ToolOrchestrator interface {
	ExecuteTool(ctx context.Context, toolName string, args interface{}) (interface{}, error)
	RegisterTool(name string, tool api.Tool) error
	ValidateToolArgs(toolName string, args interface{}) error
	GetToolMetadata(toolName string) (*api.ToolMetadata, error)

	RegisterGenericTool(name string, tool interface{}) error
	GetTypedToolMetadata(toolName string) (*api.ToolMetadata, error)
}

// TypedPipelineOperations provides type-safe pipeline operation functionality
type TypedPipelineOperations interface {
	GetSessionWorkspace(sessionID string) string
	UpdateSessionState(sessionID string, updateFunc func(*SessionState)) error

	BuildImageTyped(ctx context.Context, sessionID string, params BuildImageParams) (*BuildImageResult, error)
	PushImageTyped(ctx context.Context, sessionID string, params PushImageParams) (*PushImageResult, error)
	PullImageTyped(ctx context.Context, sessionID string, params PullImageParams) (*PullImageResult, error)
	TagImageTyped(ctx context.Context, sessionID string, params TagImageParams) (*TagImageResult, error)

	GenerateManifestsTyped(ctx context.Context, sessionID string, params GenerateManifestsParams) (*GenerateManifestsResult, error)
	DeployKubernetesTyped(ctx context.Context, sessionID string, params DeployParams) (*DeployResult, error)
	CheckHealthTyped(ctx context.Context, sessionID string, params HealthCheckParams) (*HealthCheckResult, error)

	AnalyzeRepositoryTyped(ctx context.Context, sessionID string, params AnalyzeParams) (*AnalyzeResult, error)
	ValidateDockerfileTyped(ctx context.Context, sessionID string, params ValidateParams) (*ConsolidatedValidateResult, error)

	ScanSecurityTyped(ctx context.Context, sessionID string, params ConsolidatedScanParams) (*ScanResult, error)
	ScanSecretsTyped(ctx context.Context, sessionID string, params ScanSecretsParams) (*ScanSecretsResult, error)
}

// Server represents the MCP server interface
type Server interface {
	Start(ctx context.Context) error
	Stop() error
	Shutdown(ctx context.Context) error
	EnableConversationMode(config ConsolidatedConversationConfig) error
	GetStats() *ServerStats
	GetSessionManagerStats() *SessionManagerStats
	GetWorkspaceStats() *WorkspaceStats
	GetLogger() interface{}
}

// ServerConfig holds configuration for the MCP server
type ServerConfig struct {
	WorkspaceDir      string
	MaxSessions       int
	SessionTTL        time.Duration
	MaxDiskPerSession int64
	TotalDiskLimit    int64

	StorePath string

	TransportType   string
	HTTPAddr        string
	HTTPPort        int
	CORSOrigins     []string
	APIKey          string
	RateLimit       int
	SandboxEnabled  bool
	LogLevel        string
	LogHTTPBodies   bool
	MaxBodyLogSize  int64
	CleanupInterval time.Duration
	MaxWorkers      int
	JobTTL          time.Duration
	ServiceName     string
	ServiceVersion  string
	Environment     string
	TraceSampleRate float64
}

// ServerStats represents overall server statistics
type ServerStats struct {
	Transport string               `json:"transport"`
	Sessions  *SessionManagerStats `json:"sessions"`
	Workspace *WorkspaceStats      `json:"workspace"`
	Uptime    time.Duration        `json:"uptime"`
	StartTime time.Time            `json:"start_time"`
}

// RequestHandler provides unified request handling interface
type RequestHandler interface {
	HandleRequest(ctx context.Context, request *MCPRequest) (*MCPResponse, error)
}

const (
	ConversationStagePreFlight  ConsolidatedConversationStage = "preflight"
	ConversationStageAnalyze    ConsolidatedConversationStage = "analyze"
	ConversationStageDockerfile ConsolidatedConversationStage = "dockerfile"
	ConversationStageBuild      ConsolidatedConversationStage = "build"
	ConversationStagePush       ConsolidatedConversationStage = "push"
	ConversationStageManifests  ConsolidatedConversationStage = "manifests"
	ConversationStageDeploy     ConsolidatedConversationStage = "deploy"
	ConversationStageScan       ConsolidatedConversationStage = "scan"
	ConversationStageCompleted  ConsolidatedConversationStage = "completed"
	ConversationStageError      ConsolidatedConversationStage = "error"
)

// UnifiedToolInput implements core.api.ToolInput for mcp.api.ToolInput
type UnifiedToolInput struct {
	SessionID string                 `json:"session_id"`
	Data      map[string]interface{} `json:"data"`
}

// Validate implements core.api.ToolInput
func (u *UnifiedToolInput) Validate() error {
	return nil
}

// GetSessionID implements core.api.ToolInput
func (u *UnifiedToolInput) GetSessionID() string {
	return u.SessionID
}

// UnifiedToolOutput implements core.api.ToolOutput for mcp.api.ToolOutput
type UnifiedToolOutput struct {
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data"`
	Error   string                 `json:"error,omitempty"`
}

// IsSuccess implements core.api.ToolOutput
func (u *UnifiedToolOutput) IsSuccess() bool {
	return u.Success
}

// GetData implements core.api.ToolOutput
func (u *UnifiedToolOutput) GetData() interface{} {
	return u.Data
}

// MCPUnifiedToolAdapter adapts mcp.UnifiedTool to core.UnifiedTool
type MCPUnifiedToolAdapter struct {
	mcpTool interface{}
	logger  zerolog.Logger
}

// NewMCPUnifiedToolAdapter creates an adapter for mcp.UnifiedTool
func NewMCPUnifiedToolAdapter(mcpTool interface{}, logger zerolog.Logger) *MCPUnifiedToolAdapter {
	return &MCPUnifiedToolAdapter{
		mcpTool: mcpTool,
		logger:  logger,
	}
}

// Name implements core.UnifiedTool
func (a *MCPUnifiedToolAdapter) Name() string {
	if tool, ok := a.mcpTool.(interface{ Name() string }); ok {
		return tool.Name()
	}
	return "unknown"
}

// Description implements core.UnifiedTool
func (a *MCPUnifiedToolAdapter) Description() string {
	if tool, ok := a.mcpTool.(interface{ Description() string }); ok {
		return tool.Description()
	}
	return "No description available"
}

// Schema implements core.UnifiedTool
func (a *MCPUnifiedToolAdapter) Schema() interface{} {
	if tool, ok := a.mcpTool.(interface{ GetSchema() interface{} }); ok {
		return tool.GetSchema()
	}
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"args": map[string]interface{}{
				"type":        "object",
				"description": "api.Tool arguments",
			},
		},
	}
}

// Execute implements core.UnifiedTool
func (a *MCPUnifiedToolAdapter) Execute(input api.ToolInput) (api.ToolOutput, error) {
	mcpInput := map[string]interface{}{
		"session_id": input.SessionID,
		"data":       input.Data,
	}

	if tool, ok := a.mcpTool.(interface {
		Execute(interface{}) (interface{}, error)
	}); ok {
		result, err := tool.Execute(mcpInput)
		if err != nil {
			return api.ToolOutput{
				Success: false,
				Error:   err.Error(),
			}, err
		}

		return api.ToolOutput{
			Success: true,
			Data:    map[string]interface{}{"result": result},
		}, nil
	}

	return api.ToolOutput{Success: false, Error: "tool does not implement Execute method"}, errors.NewError().Messagef("tool does not implement Execute method").WithLocation().Build()
}

func RegisterMCPUnifiedTool(registry *UnifiedRegistry, mcpTool interface{}, logger zerolog.Logger, opts ...RegistryOption) error {
	return errors.NewError().Messagef("RegisterMCPUnifiedTool temporarily disabled during migration").WithLocation().Build()
}
