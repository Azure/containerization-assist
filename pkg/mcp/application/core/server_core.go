package core

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/core/deployment"
	"github.com/Azure/container-kit/pkg/core/git"
	"github.com/Azure/container-kit/pkg/core/kubernetes"
	coreregistry "github.com/Azure/container-kit/pkg/core/registry"
	"github.com/Azure/container-kit/pkg/core/worker"
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/commands"
	"github.com/Azure/container-kit/pkg/mcp/application/di"
	"github.com/Azure/container-kit/pkg/mcp/application/registry"
	"github.com/Azure/container-kit/pkg/mcp/application/services"
	appstate "github.com/Azure/container-kit/pkg/mcp/application/state"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	domaintypes "github.com/Azure/container-kit/pkg/mcp/domain/types"
	"go.etcd.io/bbolt"
)

// UnifiedMCPServer is the main server implementation
type UnifiedMCPServer struct {
	// Service container for dependency injection
	serviceContainer services.ServiceContainer

	// Chat mode components
	conversationService services.ConversationService
	promptService       services.PromptService
	sessionManager      session.SessionManager
	// TODO: Fix after migration - use services.PromptService instead
	// promptManager       api.PromptManager

	// Unified session management
	unifiedSessionManager session.UnifiedSessionManager

	// Workflow mode components
	workflowExecutor services.WorkflowExecutor

	// State management integration
	stateIntegration *appstate.StateManagementIntegration

	// Shared components
	toolRegistry     api.Registry
	toolOrchestrator api.Orchestrator

	// Tool management
	toolService *ToolService

	// Server state
	currentMode ServerMode
	logger      *slog.Logger
}

// getChatModeTools returns tools available in chat mode
func (s *UnifiedMCPServer) getChatModeTools() []ToolDefinition {
	if s.toolService == nil {
		return []ToolDefinition{}
	}
	return s.toolService.getChatModeTools()
}

// getWorkflowModeTools returns tools available in workflow mode
func (s *UnifiedMCPServer) getWorkflowModeTools() []ToolDefinition {
	if s.toolService == nil {
		return []ToolDefinition{}
	}
	return s.toolService.getWorkflowModeTools()
}

// isAtomicTool checks if a tool is an atomic tool
func (s *UnifiedMCPServer) isAtomicTool(toolName string) bool {
	if s.toolService == nil {
		return false
	}
	return s.toolService.isAtomicTool(toolName)
}

// buildInputSchema builds an input schema for a tool
func (s *UnifiedMCPServer) buildInputSchema(metadata *api.ToolMetadata) map[string]interface{} {
	if s.toolService == nil {
		return map[string]interface{}{}
	}
	return s.toolService.buildInputSchema(metadata)
}

// NewUnifiedMCPServer creates a new unified MCP server
func NewUnifiedMCPServer(
	db *bbolt.DB,
	logger *slog.Logger,
	mode ServerMode,
) (*UnifiedMCPServer, error) {
	return createUnifiedMCPServer(db, logger, mode, nil)
}

// NewUnifiedMCPServerWithUnifiedSessionManager creates a new unified MCP server with unified session manager
func NewUnifiedMCPServerWithUnifiedSessionManager(
	db *bbolt.DB,
	logger *slog.Logger,
	mode ServerMode,
	unifiedSessionManager session.UnifiedSessionManager,
) (*UnifiedMCPServer, error) {
	return createUnifiedMCPServer(db, logger, mode, unifiedSessionManager)
}

// createUnifiedMCPServer is the common server creation logic
func createUnifiedMCPServer(
	db *bbolt.DB,
	logger *slog.Logger,
	mode ServerMode,
	unifiedSessionManager session.UnifiedSessionManager,
) (*UnifiedMCPServer, error) {
	// Create service container with all core services
	serviceContainer := createServiceContainer(logger)

	// Set global services for lazy-loaded commands
	commands.SetGlobalServices(serviceContainer)

	// Create state management integration with service container
	stateServiceContainer := &StateServiceContainerAdapter{serviceContainer: serviceContainer}
	stateIntegration := appstate.NewStateManagementIntegrationWithContainer(stateServiceContainer, logger)

	// Initialize Wire dependency injection container
	wireContainer, err := di.InitializeContainer()
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeInternalError).
			Message("failed to initialize dependency injection container").
			Cause(err).
			Build()
	}

	// Use Wire-generated dependencies
	unifiedToolRegistry := wireContainer.ToolRegistry
	toolRegistry := registry.NewRegistryAdapter(unifiedToolRegistry)

	// Initialize commands with the unified registry
	err = commands.InitializeCommands(
		unifiedToolRegistry,
		wireContainer.SessionStore,
		wireContainer.SessionState,
		logger,
	)
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeInternalError).
			Message("failed to initialize commands").
			Cause(err).
			Build()
	}

	var actualSessionManager session.UnifiedSessionManager
	var concreteSessionManager session.SessionManager // Interface for legacy components

	if unifiedSessionManager != nil {
		logger.Info("Using provided unified session manager")
		actualSessionManager = unifiedSessionManager
		if sm, ok := unifiedSessionManager.(session.SessionManager); ok {
			concreteSessionManager = sm
		}
	} else {
		// TODO: Session manager needs to be injected as it's an interface
		// concreteSessionManager, err = session.NewSessionManager(session.SessionManagerConfig{
		// 	WorkspaceDir:      "/tmp/mcp-sessions",
		// 	MaxSessions:       100,
		// 	SessionTTL:        24 * time.Hour,
		// 	MaxDiskPerSession: 1024 * 1024 * 1024,
		// 	TotalDiskLimit:    10 * 1024 * 1024 * 1024,
		// 	StorePath:         "/tmp/mcp-sessions.db",
		// 	Logger:            logger,
		// })
		// if err != nil {
		// 	return nil, errors.NewError().Message("failed to create session manager").Cause(err).Build()
		// }
		concreteSessionManager = nil // TODO: needs concrete implementation
		actualSessionManager = concreteSessionManager
	}

	// Create a simple tool orchestrator implementation
	toolOrchestrator := &simpleToolOrchestrator{
		logger:   logger,
		registry: toolRegistry,
		timeout:  10 * time.Minute,
	}

	server := &UnifiedMCPServer{
		serviceContainer:      serviceContainer,
		stateIntegration:      stateIntegration,
		toolRegistry:          toolRegistry,
		toolOrchestrator:      toolOrchestrator,
		unifiedSessionManager: actualSessionManager,
		workflowExecutor:      wireContainer.WorkflowExecutor,
		currentMode:           mode,
		logger:                logger.With("component", "unified_mcp_server"),
	}

	server.toolService = NewToolService(server)

	if mode == ModeDual || mode == ModeChat {
		// TODO: Fix preference store after three-layer migration
		// preferenceStore, err := shared.NewPreferenceStore("/tmp/mcp-preferences.db", logger, "")
		// if err != nil {
		// 	return nil, errors.NewError().Message("failed to create preference store").Cause(err).Build()
		// }
		// var preferenceStore interface{} // temporary placeholder

		if concreteSessionManager != nil {
			// Create service implementations inline to avoid import cycle
			server.conversationService = &simpleConversationService{
				sessionManager:   concreteSessionManager,
				toolOrchestrator: toolOrchestrator,
				// preferenceStore:  preferenceStore, // TODO: Fix after migration
				logger: logger,
			}
			server.promptService = &simplePromptService{logger: logger}
		} else {
			logger.Warn("Chat mode requested but no concrete session manager available")
		}
	}

	if mode == ModeDual || mode == ModeWorkflow {
		logger.Info("Workflow manager initialization skipped - not implemented yet")
	}

	server.logger.Info("Initialized unified MCP server",
		"mode", string(mode))

	return server, nil
}

// GetCapabilities returns the server's capabilities
func (s *UnifiedMCPServer) GetCapabilities() ServerCapabilities {
	capabilities := ServerCapabilities{
		AvailableModes: make([]string, 0),
		SharedTools:    make([]string, 0),
	}

	switch s.currentMode {
	case ModeDual:
		capabilities.ChatSupport = true
		capabilities.WorkflowSupport = true
		capabilities.AvailableModes = []string{"chat", "workflow", "dual"}
	case ModeChat:
		capabilities.ChatSupport = true
		capabilities.WorkflowSupport = false
		capabilities.AvailableModes = []string{"chat"}
	case ModeWorkflow:
		capabilities.ChatSupport = false
		capabilities.WorkflowSupport = true
		capabilities.AvailableModes = []string{"workflow"}
	}

	if s.toolOrchestrator != nil {
		capabilities.SharedTools = s.toolOrchestrator.ListTools()
	}

	return capabilities
}

// GetServiceContainer returns the service container for accessing core services
func (s *UnifiedMCPServer) GetServiceContainer() services.ServiceContainer {
	return s.serviceContainer
}

// GetAvailableTools returns all available tools
func (s *UnifiedMCPServer) GetAvailableTools() []ToolDefinition {
	if s.toolService == nil {
		return []ToolDefinition{}
	}
	return s.toolService.GetAvailableTools()
}

// ExecuteTool executes a tool with the given name and arguments
func (s *UnifiedMCPServer) ExecuteTool(
	ctx context.Context,
	toolName string,
	args map[string]interface{},
) (interface{}, error) {
	if s.toolService == nil {
		return nil, errors.NewError().Message("tool manager not initialized").Build()
	}
	return s.toolService.ExecuteTool(ctx, toolName, args)
}

// ExecuteToolTyped executes a tool with typed arguments
func (s *UnifiedMCPServer) ExecuteToolTyped(
	ctx context.Context,
	toolName string,
	args TypedArgs,
) (TypedResult, error) {
	// Parse the typed arguments
	var parsedArgs map[string]interface{}
	if err := json.Unmarshal(args.Data, &parsedArgs); err != nil {
		return TypedResult{
			Success: false,
			Error:   fmt.Sprintf("failed to parse arguments: %v", err),
		}, nil
	}

	// Execute the tool
	result, err := s.ExecuteTool(ctx, toolName, parsedArgs)
	if err != nil {
		return TypedResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Serialize the result
	resultData, err := json.Marshal(result)
	if err != nil {
		return TypedResult{
			Success: false,
			Error:   fmt.Sprintf("failed to serialize result: %v", err),
		}, nil
	}

	return TypedResult{
		Success: true,
		Data:    resultData,
	}, nil
}

// GetMode returns the current server mode
func (s *UnifiedMCPServer) GetMode() ServerMode {
	return s.currentMode
}

// GetLogger returns the server logger
func (s *UnifiedMCPServer) GetLogger() *slog.Logger {
	return s.logger
}

// GetSessionManager returns the session manager
func (s *UnifiedMCPServer) GetSessionManager() session.UnifiedSessionManager {
	return s.unifiedSessionManager
}

// GetToolOrchestrator returns the tool orchestrator
func (s *UnifiedMCPServer) GetToolOrchestrator() api.Orchestrator {
	return s.toolOrchestrator
}

// GetWorkflowExecutor returns the workflow executor
func (s *UnifiedMCPServer) GetWorkflowExecutor() services.WorkflowExecutor {
	return s.workflowExecutor
}

// GetStateIntegration returns the state management integration
func (s *UnifiedMCPServer) GetStateIntegration() *appstate.StateManagementIntegration {
	return s.stateIntegration
}

// Shutdown gracefully shuts down the server
func (s *UnifiedMCPServer) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down unified MCP server")

	if s.workflowExecutor != nil {
		s.logger.Info("Shutting down workflow executor")
	}

	if s.promptService != nil {
		s.logger.Info("Shutting down prompt service")
	}

	if s.unifiedSessionManager != nil {
		if err := s.unifiedSessionManager.Stop(); err != nil {
			s.logger.Error("Failed to stop session manager", "error", err)
		}
	}

	s.logger.Info("Unified MCP server shutdown complete")
	return nil
}

// simpleToolOrchestrator is a basic implementation of api.Orchestrator
type simpleToolOrchestrator struct {
	logger   *slog.Logger
	registry api.Registry
	timeout  time.Duration
}

// ExecuteTool executes a tool with the given name and arguments
func (o *simpleToolOrchestrator) ExecuteTool(ctx context.Context, name string, args interface{}) (interface{}, error) {
	// Convert args to map[string]interface{} if needed
	var data map[string]interface{}
	switch v := args.(type) {
	case map[string]interface{}:
		data = v
	default:
		// Wrap non-map args
		data = map[string]interface{}{
			"args": args,
		}
	}

	// Create ToolInput from args
	input := api.ToolInput{
		SessionID: fmt.Sprintf("session-%d", time.Now().UnixNano()),
		Data:      data,
		Context:   make(map[string]interface{}),
	}

	// Execute through registry
	output, err := o.registry.Execute(ctx, name, input)
	if err != nil {
		return nil, errors.NewError().Messagef("failed to execute tool %s", name).Cause(err).WithLocation().Build()
	}

	return output.Data, nil
}

// RegisterTool registers a tool in the orchestrator
func (o *simpleToolOrchestrator) RegisterTool(name string, tool api.Tool) error {
	return o.registry.Register(tool)
}

// ValidateToolArgs validates tool arguments
func (o *simpleToolOrchestrator) ValidateToolArgs(name string, args interface{}) error {
	tool, err := o.registry.Get(name)
	if err != nil {
		return errors.NewError().Messagef("tool not found: %s", name).Cause(err).WithLocation().Build()
	}

	// If tool has Validate method, use it
	if validator, ok := tool.(interface{ Validate(interface{}) error }); ok {
		return validator.Validate(args)
	}

	return nil
}

// GetToolMetadata returns metadata for a tool
func (o *simpleToolOrchestrator) GetToolMetadata(name string) (*api.ToolMetadata, error) {
	metadata, err := o.registry.GetMetadata(name)
	if err != nil {
		return nil, errors.NewError().Messagef("failed to get metadata for tool %s", name).Cause(err).WithLocation().Build()
	}
	return &metadata, nil
}

// RegisterGenericTool registers a generic tool
func (o *simpleToolOrchestrator) RegisterGenericTool(name string, handler interface{}) error {
	// For now, just return an error
	return errors.NewError().Message("generic tool registration not implemented").WithLocation().Build()
}

// GetTypedToolMetadata returns typed metadata for a tool
func (o *simpleToolOrchestrator) GetTypedToolMetadata(name string) (*api.ToolMetadata, error) {
	return o.GetToolMetadata(name)
}

// GetTool retrieves a registered tool
func (o *simpleToolOrchestrator) GetTool(name string) (api.Tool, bool) {
	tool, err := o.registry.Get(name)
	if err != nil {
		return nil, false
	}
	return tool, true
}

// ListTools returns a list of all registered tools
func (o *simpleToolOrchestrator) ListTools() []string {
	return o.registry.List()
}

// GetStats returns orchestrator statistics
func (o *simpleToolOrchestrator) GetStats() interface{} {
	// Return simple stats
	return map[string]interface{}{
		"registered_tools": len(o.registry.List()),
		"timeout":          o.timeout.String(),
	}
}

// Simple service implementations to avoid import cycles

// simpleConversationService implements services.ConversationService
type simpleConversationService struct {
	sessionManager   session.SessionManager
	toolOrchestrator api.Orchestrator
	// TODO: Fix preference store after migration
	// preferenceStore  *shared.PreferenceStore
	logger *slog.Logger
}

func (cs *simpleConversationService) ProcessMessage(_ context.Context, sessionID, message string) (*services.ConversationResponse, error) {
	cs.logger.Info("Processing message", "session_id", sessionID, "message", message)
	return &services.ConversationResponse{
		SessionID:     sessionID,
		Message:       "Message processed successfully",
		Stage:         domaintypes.StageWelcome,
		Status:        "success",
		RequiresInput: false,
	}, nil
}

func (cs *simpleConversationService) GetConversationState(ctx context.Context, sessionID string) (*services.ConversationState, error) {
	cs.logger.Debug("Getting conversation state", "session_id", sessionID)
	return &services.ConversationState{
		SessionID:    sessionID,
		CurrentStage: domaintypes.StageWelcome,
		History:      []services.ConversationTurn{},
		Preferences:  domaintypes.UserPreferences{},
	}, nil
}

func (cs *simpleConversationService) UpdateConversationStage(ctx context.Context, sessionID string, stage domaintypes.ConversationStage) error {
	cs.logger.Debug("Updating conversation stage", "session_id", sessionID, "stage", stage)
	return nil
}

func (cs *simpleConversationService) GetConversationHistory(ctx context.Context, sessionID string, limit int) ([]services.ConversationTurn, error) {
	cs.logger.Debug("Getting conversation history", "session_id", sessionID, "limit", limit)
	return []services.ConversationTurn{}, nil
}

func (cs *simpleConversationService) ClearConversationContext(ctx context.Context, sessionID string) error {
	cs.logger.Debug("Clearing conversation context", "session_id", sessionID)
	return nil
}

// simplePromptService implements services.PromptService
type simplePromptService struct {
	logger *slog.Logger
}

func (ps *simplePromptService) BuildPrompt(ctx context.Context, stage domaintypes.ConversationStage, _ map[string]interface{}) (string, error) {
	ps.logger.Debug("Building prompt", "stage", stage)
	return "System prompt for stage: " + string(stage), nil
}

func (ps *simplePromptService) ProcessPromptResponse(ctx context.Context, response string, _ *services.ConversationState) error {
	ps.logger.Debug("Processing prompt response", "response_length", len(response))
	return nil
}

func (ps *simplePromptService) DetectWorkflowIntent(ctx context.Context, message string) (*services.WorkflowIntent, error) {
	ps.logger.Debug("Detecting workflow intent", "message_length", len(message))
	return &services.WorkflowIntent{
		Detected:   false,
		Workflow:   "",
		Parameters: map[string]interface{}{},
	}, nil
}

func (ps *simplePromptService) ShouldAutoAdvance(ctx context.Context, state *services.ConversationState) (bool, *services.AutoAdvanceConfig) {
	ps.logger.Debug("Checking auto-advance", "session_id", state.SessionID)
	return false, nil
}

// createServiceContainer creates and configures the service container with all core services
func createServiceContainer(logger *slog.Logger) services.ServiceContainer {
	logger.Info("Creating service container with core services")

	// Create core services
	// TODO: Need to inject clients properly
	// dockerService := docker.NewService(clients, logger)
	_ = git.NewGitService(logger)
	_ = coreregistry.NewRegistryService(logger)
	_ = deployment.NewDeploymentService(logger)
	_ = worker.NewWorkerService(logger, nil)
	// TODO: securityService := security.NewSecurityService(logger, nil) - disabled due to type conflicts

	// Create Kubernetes manifest service
	manifestService := kubernetes.NewManifestService(logger)

	// Create Kubernetes deployment service
	// TODO: Need to inject clients properly
	deploymentService := kubernetes.NewService(nil, logger)

	// Create pipeline service
	// TODO: Create pipeline service from service container
	var pipelineService services.PipelineService

	// Build service container with all services
	container := services.NewDefaultServiceContainer(logger).
		WithManifestService(manifestService).
		WithDeploymentService(deploymentService).
		WithPipelineService(pipelineService)

	logger.Info("Service container created successfully",
		"services", []string{
			"manifest", "deployment", "pipeline",
		})

	return container
}

// StateServiceContainerAdapter adapts services.ServiceContainer to appstate.ServiceContainer
type StateServiceContainerAdapter struct {
	serviceContainer services.ServiceContainer
}

// SessionStore implements appstate.StateServiceContainer interface
func (a *StateServiceContainerAdapter) SessionStore() appstate.StateSessionStore {
	return &SessionStoreAdapter{sessionStore: a.serviceContainer.SessionStore()}
}

// Logger implements appstate.ServiceContainer interface
func (a *StateServiceContainerAdapter) Logger() *slog.Logger {
	return a.serviceContainer.Logger()
}

// SessionStoreAdapter adapts services.SessionStore to appstate.SessionStore
type SessionStoreAdapter struct {
	sessionStore services.SessionStore
}

// Create implements appstate.SessionStore interface
func (a *SessionStoreAdapter) Create(ctx context.Context, session *api.Session) error {
	return a.sessionStore.Create(ctx, session)
}

// Get implements appstate.SessionStore interface
func (a *SessionStoreAdapter) Get(ctx context.Context, sessionID string) (*api.Session, error) {
	return a.sessionStore.Get(ctx, sessionID)
}

// Delete implements appstate.SessionStore interface
func (a *SessionStoreAdapter) Delete(ctx context.Context, sessionID string) error {
	return a.sessionStore.Delete(ctx, sessionID)
}

// List implements appstate.SessionStore interface
func (a *SessionStoreAdapter) List(ctx context.Context) ([]*api.Session, error) {
	return a.sessionStore.List(ctx)
}

// Update implements appstate.SessionStore interface
func (a *SessionStoreAdapter) Update(ctx context.Context, session *api.Session) error {
	return a.sessionStore.Update(ctx, session)
}
