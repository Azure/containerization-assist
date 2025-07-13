// Package application provides dependency injection and server bootstrapping for MCP.
package application

import (
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/session"
	"github.com/Azure/container-kit/pkg/mcp/domain/events"
	"github.com/Azure/container-kit/pkg/mcp/domain/saga"
	domainsampling "github.com/Azure/container-kit/pkg/mcp/domain/sampling"
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ml"
	infraprogress "github.com/Azure/container-kit/pkg/mcp/infrastructure/progress"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/prompts"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/resources"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/sampling"
)

// Dependencies holds all the server dependencies in a structured way.
type Dependencies struct {
	// Core services
	Logger         *slog.Logger
	Config         workflow.ServerConfig
	SessionManager session.SessionManager
	ResourceStore  *resources.Store

	// Domain services
	ProgressFactory *infraprogress.SinkFactory
	EventPublisher  *events.Publisher
	SagaCoordinator *saga.SagaCoordinator

	// Workflow orchestrators
	Orchestrator      *workflow.Orchestrator
	EventOrchestrator *workflow.EventOrchestrator
	SagaOrchestrator  *workflow.SagaOrchestrator

	// AI/ML services
	ErrorPatternRecognizer *ml.ErrorPatternRecognizer
	EnhancedErrorHandler   *ml.EnhancedErrorHandler
	StepEnhancer           *ml.StepEnhancer

	// Infrastructure services
	SamplingClient domainsampling.Sampler
	PromptManager  *prompts.Manager
}

// Option represents a functional option for configuring dependencies
type Option func(*Dependencies)

// WithLogger sets a custom logger
func WithLogger(logger *slog.Logger) Option {
	return func(d *Dependencies) {
		d.Logger = logger
	}
}

// WithConfig sets the server configuration
func WithConfig(config workflow.ServerConfig) Option {
	return func(d *Dependencies) {
		d.Config = config
	}
}

// WithSessionManager sets a custom session manager
func WithSessionManager(sm session.SessionManager) Option {
	return func(d *Dependencies) {
		d.SessionManager = sm
	}
}

// WithSamplingClient sets a custom sampling client
func WithSamplingClient(client domainsampling.Sampler) Option {
	return func(d *Dependencies) {
		d.SamplingClient = client
	}
}

// WithProgressFactory sets a custom progress factory
func WithProgressFactory(factory *infraprogress.SinkFactory) Option {
	return func(d *Dependencies) {
		d.ProgressFactory = factory
	}
}

// WithResourceStore sets a custom resource store
func WithResourceStore(store *resources.Store) Option {
	return func(d *Dependencies) {
		d.ResourceStore = store
	}
}

// WithPromptManager sets a custom prompt manager
func WithPromptManager(manager *prompts.Manager) Option {
	return func(d *Dependencies) {
		d.PromptManager = manager
	}
}

// Configuration options for workflow.ServerConfig

// WithWorkspace sets the workspace directory
func WithWorkspace(dir string) Option {
	return func(d *Dependencies) {
		d.Config.WorkspaceDir = dir
	}
}

// WithStorePath sets the store path for session persistence
func WithStorePath(path string) Option {
	return func(d *Dependencies) {
		d.Config.StorePath = path
	}
}

// WithMaxSessions sets the maximum number of concurrent sessions
func WithMaxSessions(max int) Option {
	return func(d *Dependencies) {
		d.Config.MaxSessions = max
	}
}

// WithSessionTTL sets the session time-to-live
func WithSessionTTL(ttl time.Duration) Option {
	return func(d *Dependencies) {
		d.Config.SessionTTL = ttl
	}
}

// WithMaxDiskPerSession sets the maximum disk usage per session
func WithMaxDiskPerSession(size int64) Option {
	return func(d *Dependencies) {
		d.Config.MaxDiskPerSession = size
	}
}

// WithTotalDiskLimit sets the total disk limit for all sessions
func WithTotalDiskLimit(size int64) Option {
	return func(d *Dependencies) {
		d.Config.TotalDiskLimit = size
	}
}

// WithTransport sets the transport type (stdio, http)
func WithTransport(transport string) Option {
	return func(d *Dependencies) {
		d.Config.TransportType = transport
	}
}

// WithHTTPAddress sets the HTTP server address
func WithHTTPAddress(addr string) Option {
	return func(d *Dependencies) {
		d.Config.HTTPAddr = addr
	}
}

// WithHTTPPort sets the HTTP server port
func WithHTTPPort(port int) Option {
	return func(d *Dependencies) {
		d.Config.HTTPPort = port
	}
}

// WithCORSOrigins sets the allowed CORS origins
func WithCORSOrigins(origins []string) Option {
	return func(d *Dependencies) {
		d.Config.CORSOrigins = origins
	}
}

// WithLogLevel sets the logging level
func WithLogLevel(level string) Option {
	return func(d *Dependencies) {
		d.Config.LogLevel = level
	}
}

// WithChatModes enables chat mode functionality
func WithChatModes(enabled bool) Option {
	return func(d *Dependencies) {
		// Chat modes are always available via standard MCP protocol
		// This option is maintained for compatibility
		d.Logger.Debug("Chat mode support is enabled via standard MCP protocol")
	}
}

// WithDependencies sets the entire Dependencies struct at once
// This is useful for Wire-based initialization
func WithDependencies(deps *Dependencies) Option {
	return func(d *Dependencies) {
		*d = *deps
	}
}

// NewDependencies creates and wires up all server dependencies using functional options.
func NewDependencies(opts ...Option) *Dependencies {
	// Initialize with sensible defaults
	d := &Dependencies{
		Logger: slog.Default(),
		Config: workflow.DefaultServerConfig(),
	}

	// Apply all options
	for _, opt := range opts {
		opt(d)
	}

	// Create base logger with component tagging
	baseLogger := d.Logger.With("component", "mcp-server")

	// Create session manager if not provided
	if d.SessionManager == nil {
		d.SessionManager = session.NewMemorySessionManager(
			baseLogger.With("service", "session"),
			d.Config.SessionTTL,
			d.Config.MaxSessions,
		)
	}

	// Create resource store if not provided
	if d.ResourceStore == nil {
		d.ResourceStore = resources.NewStore(baseLogger.With("service", "resources"))
	}

	// Create progress factory if not provided
	if d.ProgressFactory == nil {
		d.ProgressFactory = infraprogress.NewSinkFactory(baseLogger.With("service", "progress"))
	}

	// Create sampling client if not provided
	if d.SamplingClient == nil {
		infraClient := sampling.NewClient(
			baseLogger.With("service", "sampling"),
			sampling.WithMaxTokens(4000),
			sampling.WithTemperature(0.1), // Conservative for infrastructure tasks
			sampling.WithRetry(3, 10000),  // 3 attempts, 10k token budget
		)
		d.SamplingClient = sampling.NewDomainAdapter(infraClient)
	}

	// Create prompt manager if not provided
	if d.PromptManager == nil {
		promptManager, err := prompts.NewManager(
			baseLogger.With("service", "prompts"),
			prompts.ManagerConfig{
				EnableHotReload: false, // Disable in production
				AllowOverride:   false, // Use embedded templates only
			},
		)
		if err != nil {
			baseLogger.Error("Failed to create prompt manager", "error", err)
			// Continue without error - prompt manager is not critical for basic operation
		} else {
			d.PromptManager = promptManager
		}
	}

	// Create event publisher if not provided
	if d.EventPublisher == nil {
		d.EventPublisher = events.NewPublisher(baseLogger.With("service", "events"))
	}

	// Create saga coordinator if not provided
	if d.SagaCoordinator == nil {
		d.SagaCoordinator = saga.NewSagaCoordinator(
			baseLogger.With("service", "saga"),
			d.EventPublisher,
		)
	}

	// Create orchestrators if not provided
	if d.Orchestrator == nil {
		d.Orchestrator = workflow.NewOrchestrator(baseLogger.With("service", "orchestrator"))
	}

	if d.EventOrchestrator == nil {
		d.EventOrchestrator = workflow.NewEventOrchestrator(
			baseLogger.With("service", "event-orchestrator"),
			d.EventPublisher,
		)
	}

	if d.SagaOrchestrator == nil {
		d.SagaOrchestrator = workflow.NewSagaOrchestrator(
			baseLogger.With("service", "saga-orchestrator"),
			d.EventPublisher,
			d.SagaCoordinator,
		)
	}

	// Update the logger reference to use the tagged version
	d.Logger = baseLogger

	return d
}

// ChatModeConfig defines the configuration for a custom chat mode
type ChatModeConfig struct {
	Mode        string   `json:"mode"`
	Description string   `json:"description"`
	Functions   []string `json:"functions"`
}

// GetChatModeFunctions returns the function names available in chat mode
func GetChatModeFunctions() []string {
	return []string{
		"containerize_and_deploy",
	}
}

// ConvertWorkflowToolsToChat converts workflow tools to chat-compatible format
func ConvertWorkflowToolsToChat() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "containerize_and_deploy",
			Description: "Complete containerization and deployment workflow",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"repo_url": map[string]interface{}{
						"type":        "string",
						"description": "Repository URL to containerize",
					},
					"branch": map[string]interface{}{
						"type":        "string",
						"description": "Git branch to use (optional)",
					},
				},
				"required": []string{"repo_url"},
			},
		},
	}
}

// ToolDefinition represents an MCP tool definition
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// NewServer creates a new MCP server with the given dependencies using functional options.
// This replaces the ServerBuilder pattern with a simpler approach.
func NewServer(opts ...Option) *serverImpl {
	deps := NewDependencies(opts...)

	return &serverImpl{
		deps:      deps,
		startTime: time.Now(),
	}
}
