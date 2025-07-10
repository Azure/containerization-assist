# Extracted Interface Definitions

Generated on: $(date)

This document contains interface definitions extracted directly from the source code.

## From pkg/mcp/application/api/interfaces.go
```go
type Tool interface {
	// Name returns the unique identifier for this tool
	Name() string

	// Description returns a human-readable description of the tool
	Description() string

	// Execute runs the tool with the given input
	Execute(ctx context.Context, input ToolInput) (ToolOutput, error)

	// Schema returns the JSON schema for the tool's parameters and results
	Schema() ToolSchema
}
type Registry interface {
	// Register adds a tool to the registry
	Register(tool Tool, opts ...RegistryOption) error

	// Unregister removes a tool from the registry
	Unregister(name string) error

	// Get retrieves a tool by name
	Get(name string) (Tool, error)

	// List returns all registered tool names
	List() []string

	// ListByCategory returns tools filtered by category
	ListByCategory(category ToolCategory) []string

	// ListByTags returns tools that match any of the given tags
	ListByTags(tags ...string) []string

	// Execute runs a tool with the given input
	Execute(ctx context.Context, name string, input ToolInput) (ToolOutput, error)

	// ExecuteWithRetry runs a tool with automatic retry on failure
	ExecuteWithRetry(ctx context.Context, name string, input ToolInput, policy RetryPolicy) (ToolOutput, error)

	// GetMetadata returns metadata about a registered tool
	GetMetadata(name string) (ToolMetadata, error)

	// GetStatus returns the current status of a tool
	GetStatus(name string) (ToolStatus, error)

	// SetStatus updates the status of a tool
	SetStatus(name string, status ToolStatus) error

	// Close releases all resources used by the registry
	Close() error

	// GetMetrics returns registry metrics (optional monitoring)
	GetMetrics() RegistryMetrics

	// Subscribe registers a callback for registry events (optional monitoring)
	Subscribe(event RegistryEventType, callback RegistryEventCallback) error

	// Unsubscribe removes a callback (optional monitoring)
	Unsubscribe(event RegistryEventType, callback RegistryEventCallback) error
}
type Orchestrator interface {
	// RegisterTool registers a tool with the orchestrator
	RegisterTool(name string, tool Tool) error

	// ExecuteTool executes a tool with the given arguments
	ExecuteTool(ctx context.Context, toolName string, args interface{}) (interface{}, error)

	// GetTool retrieves a registered tool
	GetTool(name string) (Tool, bool)

	// ListTools returns a list of all registered tools
	ListTools() []string

	// GetStats returns orchestrator statistics
	GetStats() interface{}

	// ValidateToolArgs validates tool arguments
	ValidateToolArgs(toolName string, args interface{}) error

	// GetToolMetadata retrieves metadata for a specific tool
	GetToolMetadata(toolName string) (*ToolMetadata, error)

	// RegisterGenericTool registers a tool with generic interface
	RegisterGenericTool(name string, tool interface{}) error

	// GetTypedToolMetadata retrieves typed metadata for a specific tool
	GetTypedToolMetadata(toolName string) (*ToolMetadata, error)
}
type MCPServer interface {
	// Start starts the server
	Start(ctx context.Context) error

	// Stop gracefully shuts down the server
	Stop(ctx context.Context) error

	// RegisterTool registers a tool with the server
	RegisterTool(tool Tool) error

	// GetRegistry returns the tool registry
	GetRegistry() Registry

	// GetSessionManager returns the session manager
	GetSessionManager() interface{} // Returns session.UnifiedSessionManager to avoid import cycle

	// GetOrchestrator returns the tool orchestrator
	GetOrchestrator() Orchestrator
}
type GomcpManager interface {
	// Start starts the gomcp server
	Start(ctx context.Context) error

	// Stop stops the gomcp server
	Stop(ctx context.Context) error

	// RegisterTool registers a tool with gomcp
	RegisterTool(name, description string, handler interface{}) error

	// GetServer returns the underlying gomcp server
	GetServer() *server.Server

	// IsRunning checks if the server is running
	IsRunning() bool
}
type Transport interface {
	// Start starts the transport
	Start(ctx context.Context) error

	// Stop stops the transport
	Stop(ctx context.Context) error

	// Send sends a message
	Send(message interface{}) error

	// Receive receives a message
	Receive() (interface{}, error)

	// IsConnected checks if the transport is connected
	IsConnected() bool
}
type Logger interface {
	logging.Standards
}
type ToolFactory interface {
	// CreateTool creates a tool by category and name
	CreateTool(category string, name string) (Tool, error)

	// CreateAnalyzer creates an analyzer (special case due to interfaces)
	CreateAnalyzer(aiAnalyzer interface{}) interface{}

	// CreateEnhancedBuildAnalyzer creates an enhanced build analyzer
	CreateEnhancedBuildAnalyzer() interface{} // Returns interface{} to avoid import

	// CreateSessionStateManager creates a session state manager
	CreateSessionStateManager(sessionID string) interface{} // Returns interface{} to avoid import

	// RegisterToolCreator registers a tool creator function for a category and name
	RegisterToolCreator(category string, name string, creator ToolCreator)
}
```

## From pkg/mcp/application/services/interfaces.go
```go
type PipelineService interface {
	// Lifecycle management
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	IsRunning() bool

	// Job management
	CancelJob(ctx context.Context, jobID string) error
}
type ServiceContainer interface {
	// Core services
	SessionStore() SessionStore
	SessionState() SessionState
	BuildExecutor() BuildExecutor
	ToolRegistry() ToolRegistry
	WorkflowExecutor() WorkflowExecutor
	Scanner() Scanner
	ConfigValidator() ConfigValidator
	ErrorReporter() ErrorReporter

	// Additional services
	StateManager() StateManager
	KnowledgeBase() KnowledgeBase
	K8sClient() K8sClient
	Analyzer() Analyzer
	Persistence() Persistence
	Logger() *slog.Logger

	// Conversation services
	ConversationService() ConversationService
	PromptService() PromptService

	// Infrastructure services
	DockerService() docker.Service
	ManifestService() kubernetes.ManifestService
	DeploymentService() kubernetes.Service
	PipelineService() PipelineService
}
type ConversationService interface {
	// ProcessMessage handles a user message in a conversation
	ProcessMessage(ctx context.Context, sessionID, message string) (*ConversationResponse, error)

	// GetConversationState returns the current conversation state
	GetConversationState(ctx context.Context, sessionID string) (*ConversationState, error)

	// UpdateConversationStage updates the conversation stage
	UpdateConversationStage(ctx context.Context, sessionID string, stage shared.ConversationStage) error

	// GetConversationHistory returns conversation history
	GetConversationHistory(ctx context.Context, sessionID string, limit int) ([]ConversationTurn, error)

	// ClearConversationContext clears conversation context
	ClearConversationContext(ctx context.Context, sessionID string) error
}
type PromptService interface {
	// BuildPrompt creates a prompt for the given stage and context
	BuildPrompt(ctx context.Context, stage shared.ConversationStage, promptContext map[string]interface{}) (string, error)

	// ProcessPromptResponse processes AI response and updates state
	ProcessPromptResponse(ctx context.Context, response string, state *ConversationState) error

	// DetectWorkflowIntent detects if message indicates workflow intent
	DetectWorkflowIntent(ctx context.Context, message string) (*WorkflowIntent, error)

	// ShouldAutoAdvance determines if conversation should auto-advance
	ShouldAutoAdvance(ctx context.Context, state *ConversationState) (bool, *AutoAdvanceConfig)
}
type SessionStore interface {
	// Create creates a new session
	Create(ctx context.Context, session *api.Session) error

	// Get retrieves a session by ID
	Get(ctx context.Context, sessionID string) (*api.Session, error)

	// Update updates an existing session
	Update(ctx context.Context, session *api.Session) error

	// Delete removes a session
	Delete(ctx context.Context, sessionID string) error

	// List returns all sessions
	List(ctx context.Context) ([]*api.Session, error)
}
type SessionState interface {
	// SaveState saves the current state for a session
	SaveState(ctx context.Context, sessionID string, state map[string]interface{}) error

	// GetState retrieves the state for a session
	GetState(ctx context.Context, sessionID string) (map[string]interface{}, error)

	// CreateCheckpoint creates a checkpoint for rollback
	CreateCheckpoint(ctx context.Context, sessionID string, name string) error

	// RestoreCheckpoint restores to a previous checkpoint
	RestoreCheckpoint(ctx context.Context, sessionID string, name string) error

	// ListCheckpoints lists available checkpoints
	ListCheckpoints(ctx context.Context, sessionID string) ([]string, error)

	// GetWorkspaceDir gets the workspace directory for a session
	GetWorkspaceDir(ctx context.Context, sessionID string) (string, error)

	// SetWorkspaceDir sets the workspace directory for a session
	SetWorkspaceDir(ctx context.Context, sessionID string, dir string) error

	// GetSessionMetadata gets session metadata
	GetSessionMetadata(ctx context.Context, sessionID string) (map[string]interface{}, error)

	// UpdateSessionData updates session data
	UpdateSessionData(ctx context.Context, sessionID string, data map[string]interface{}) error
}
type BuildExecutor interface {
	// QuickBuild performs a quick build without template generation
	QuickBuild(ctx context.Context, dockerfileContent string, targetDir string, options docker.BuildOptions) (*docker.BuildResult, error)

	// QuickPush performs a quick push of an already built image
	QuickPush(ctx context.Context, imageRef string, options docker.PushOptions) (*docker.RegistryPushResult, error)

	// QuickPull performs a quick pull of an image
	QuickPull(ctx context.Context, imageRef string) (*docker.PullResult, error)
}
type ToolRegistry interface {
	// Register registers a new tool
	Register(ctx context.Context, name string, tool api.Tool) error

	// GetTool retrieves a tool by name
	GetTool(ctx context.Context, name string) (api.Tool, error)

	// ListTools lists all registered tools
	ListTools(ctx context.Context) []string

	// GetMetrics returns registry metrics
	GetMetrics(ctx context.Context) api.RegistryMetrics
}
type WorkflowExecutor interface {
	// ExecuteWorkflow runs a complete workflow
	ExecuteWorkflow(ctx context.Context, workflow *api.Workflow) (*api.WorkflowResult, error)

	// ExecuteStep executes a single workflow step
	ExecuteStep(ctx context.Context, step *api.WorkflowStep) (*api.StepResult, error)

	// ValidateWorkflow validates a workflow definition
	ValidateWorkflow(ctx context.Context, workflow *api.Workflow) error
}
type Scanner interface {
	// ScanImage scans a container image for vulnerabilities
	ScanImage(ctx context.Context, image string, options security.ScanOptionsService) (*security.ScanResult, error)

	// ScanDirectory scans a directory for secrets
	ScanDirectory(ctx context.Context, path string, options security.ScanOptionsService) (*security.ScanResult, error)

	// GetAvailableScanners returns available scanner types
	GetAvailableScanners(ctx context.Context) []string
}
type ConfigValidator interface {
	// ValidateDockerfile validates a Dockerfile
	ValidateDockerfile(ctx context.Context, content string) (*ValidationResult, error)

	// ValidateManifest validates a Kubernetes manifest
	ValidateManifest(ctx context.Context, content string) (*ValidationResult, error)

	// ValidateConfig validates general configuration
	ValidateConfig(ctx context.Context, config map[string]interface{}) (*ValidationResult, error)
}
type ErrorReporter interface {
	// ReportError reports an error with context
	ReportError(ctx context.Context, err error, context map[string]interface{})

	// GetErrorStats returns error statistics
	GetErrorStats(ctx context.Context) ErrorStats

	// SuggestFix suggests fixes for common errors
	SuggestFix(ctx context.Context, err error) []string
}
type StateManager interface {
	// SaveState saves state with a key
	SaveState(ctx context.Context, key string, state interface{}) error

	// GetState retrieves state by key
	GetState(ctx context.Context, key string, state interface{}) error

	// UpdateState updates existing state
	UpdateState(ctx context.Context, key string, tool string, state interface{}) error

	// DeleteState removes state
	DeleteState(ctx context.Context, key string) error
}
type KnowledgeBase interface {
	// Store stores a pattern or solution
	Store(ctx context.Context, key string, value interface{}) error

	// Retrieve gets a stored pattern
	Retrieve(ctx context.Context, key string) (interface{}, error)

	// Search searches for patterns
	Search(ctx context.Context, query string) ([]interface{}, error)
}
type K8sClient interface {
	// Apply applies manifests to a cluster
	Apply(ctx context.Context, manifests string, namespace string) error

	// Delete removes resources from a cluster
	Delete(ctx context.Context, manifests string, namespace string) error

	// GetStatus gets resource status
	GetStatus(ctx context.Context, resource, name, namespace string) (interface{}, error)
}
type Analyzer interface {
	// AnalyzeRepository analyzes a repository (matches core analysis.RepositoryAnalyzer)
	AnalyzeRepository(ctx context.Context, repoPath string) (*analysis.AnalysisResult, error)
}
type AnalysisService interface {
	// AnalyzeRepository analyzes a repository with progress callback
	AnalyzeRepository(ctx context.Context, path string, callback ProgressCallback) (*RepositoryAnalysis, error)

	// AnalyzeWithAI performs AI-powered analysis
	AnalyzeWithAI(ctx context.Context, content string) (*AIAnalysis, error)

	// GetAnalysisProgress gets the progress of an ongoing analysis
	GetAnalysisProgress(ctx context.Context, analysisID string) (*AnalysisProgress, error)
}
type Persistence interface {
	// Put stores a key-value pair
	Put(ctx context.Context, bucket string, key string, value interface{}) error

	// Get retrieves a value by key
	Get(ctx context.Context, bucket string, key string, result interface{}) error

	// Delete removes a key-value pair
	Delete(ctx context.Context, bucket string, key string) error

	// List returns all key-value pairs in a bucket
	List(ctx context.Context, bucket string) (map[string]interface{}, error)

	// Close closes the persistence layer
	Close() error
}
```

## Interface Statistics

- API Interfaces: 8
- Service Interfaces: 18

