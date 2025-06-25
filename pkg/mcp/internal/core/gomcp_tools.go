package core

import (
	"context"
	"fmt"

	"github.com/Azure/container-copilot/pkg/clients"
	coredocker "github.com/Azure/container-copilot/pkg/core/docker"
	"github.com/Azure/container-copilot/pkg/docker"
	"github.com/Azure/container-copilot/pkg/k8s"
	"github.com/Azure/container-copilot/pkg/kind"
	"github.com/Azure/container-copilot/pkg/mcp/internal/adapter"
	"github.com/Azure/container-copilot/pkg/mcp/internal/orchestration"
	"github.com/Azure/container-copilot/pkg/mcp/internal/pipeline"
	"github.com/Azure/container-copilot/pkg/mcp/internal/store/session"
	"github.com/Azure/container-copilot/pkg/mcp/internal/tools"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/types/session"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/Azure/container-copilot/pkg/runner"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// Typed args and result structs for GoMCP tools

// ServerStatusArgs defines arguments for server status tool
type ServerStatusArgs struct {
	SessionID        string `json:"session_id,omitempty" description:"Session ID for detailed analysis"`
	IncludeDetails   bool   `json:"include_details,omitempty" description:"Include detailed server information"`
	DetailedAnalysis bool   `json:"detailed_analysis,omitempty" description:"Perform detailed health analysis"`
	DryRun           bool   `json:"dry_run,omitempty" description:"Perform dry run without side effects"`
}

// ServerStatusResult defines result for server status tool
type ServerStatusResult struct {
	Healthy   bool                   `json:"healthy"`
	Status    string                 `json:"status"`
	Version   string                 `json:"version"`
	SessionID string                 `json:"session_id,omitempty"`
	DryRun    bool                   `json:"dry_run,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// SessionListArgs defines arguments for list sessions tool
type SessionListArgs struct {
	IncludeInactive bool `json:"include_inactive,omitempty" description:"Include inactive sessions in results"`
	Limit           int  `json:"limit,omitempty" description:"Maximum number of sessions to return"`
}

// SessionListResult defines result for list sessions tool
type SessionListResult struct {
	Sessions []map[string]interface{} `json:"sessions"`
	Total    int                      `json:"total"`
}

// SessionDeleteArgs defines arguments for delete session tool
type SessionDeleteArgs struct {
	SessionID string `json:"session_id" description:"Session ID to delete"`
	Force     bool   `json:"force,omitempty" description:"Force deletion even if session is active"`
}

// SessionDeleteResult defines result for delete session tool
type SessionDeleteResult struct {
	Success   bool   `json:"success"`
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

// JobStatusArgs defines arguments for job status tool
type JobStatusArgs struct {
	JobID string `json:"job_id" description:"Job ID to check status for"`
}

// JobStatusResult defines result for job status tool
type JobStatusResult struct {
	JobID   string                 `json:"job_id"`
	Status  string                 `json:"status"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// ChatArgs defines arguments for chat tool
type ChatArgs struct {
	Message   string `json:"message" description:"Message to send to the AI assistant"`
	SessionID string `json:"session_id,omitempty" description:"Session ID for conversation context"`
}

// ChatResult defines result for chat tool
type ChatResult struct {
	Response  string `json:"response"`
	SessionID string `json:"session_id,omitempty"`
}

// Tool registration methods

// RegisterTools registers all available tools with the gomcp server
func (gm *GomcpManager) RegisterTools(s *Server) error {
	if !gm.isInitialized {
		return fmt.Errorf("manager must be initialized before registering tools")
	}

	// Create dependencies for tools
	deps := gm.createToolDependencies(s)

	// Set pipeline operations on the orchestrator for type-safe dispatch
	if deps.ToolOrchestrator != nil && deps.PipelineOperations != nil && deps.AtomicSessionMgr != nil {
		deps.ToolOrchestrator.SetPipelineOperations(deps.PipelineOperations)

		// Create and set the tool factory with concrete types
		toolFactory := orchestration.NewToolFactory(deps.PipelineOperations, deps.AtomicSessionMgr, deps.Logger)

		// Get the no-reflect dispatcher from the orchestrator and set the factory
		// This is a workaround for the interface/concrete type mismatch
		if dispatcher := getNoReflectDispatcher(deps.ToolOrchestrator); dispatcher != nil {
			dispatcher.SetToolFactory(toolFactory)
			deps.Logger.Info().Msg("Tool factory set on no-reflect dispatcher")
		}

		deps.Logger.Info().Msg("Pipeline operations set on tool orchestrator")
	}

	// Register core tools
	deps.Logger.Info().Msg("Registering core tools")
	if err := gm.registerCoreTools(deps); err != nil {
		return fmt.Errorf("failed to register core tools: %w", err)
	}
	deps.Logger.Info().Msg("Core tools registered successfully")

	// Register atomic tools
	deps.Logger.Info().Msg("Registering atomic tools")
	if err := gm.registerAtomicTools(deps); err != nil {
		return fmt.Errorf("failed to register atomic tools: %w", err)
	}
	deps.Logger.Info().Msg("Atomic tools registered successfully")

	// Register utility tools
	deps.Logger.Info().Msg("Registering utility tools")
	if err := gm.registerUtilityTools(deps); err != nil {
		return fmt.Errorf("failed to register utility tools: %w", err)
	}
	deps.Logger.Info().Msg("Utility tools registered successfully")

	// Register conversation tools if enabled
	if s.IsConversationModeEnabled() {
		if err := gm.registerConversationTools(deps); err != nil {
			return fmt.Errorf("failed to register conversation tools: %w", err)
		}
	}

	// All tools are now registered using standardized patterns
	deps.Logger.Info().Msg("All tools registered successfully with standardized patterns")

	return nil
}

// ToolDependencies holds shared dependencies for tool creation
type ToolDependencies struct {
	Server             *Server
	SessionManager     *session.SessionManager
	ToolOrchestrator   *orchestration.MCPToolOrchestrator
	ToolRegistry       *orchestration.MCPToolRegistry
	PipelineOperations mcptypes.PipelineOperations // Direct pipeline operations without adapter
	AtomicSessionMgr   *session.SessionManager
	MCPClients         *adapter.MCPClients
	RegistryManager    *coredocker.RegistryManager
	Logger             zerolog.Logger
}

// getNoReflectDispatcher extracts the no-reflect dispatcher from the orchestrator
func getNoReflectDispatcher(orchestrator *orchestration.MCPToolOrchestrator) *orchestration.NoReflectToolOrchestrator {
	// Use the proper getter method to access the dispatcher
	return orchestrator.GetDispatcher()
}

// createToolDependencies creates shared dependencies for tools
func (gm *GomcpManager) createToolDependencies(s *Server) *ToolDependencies {
	// Create clients for atomic tools
	cmdRunner := &runner.DefaultCommandRunner{}
	mcpClients := adapter.NewMCPClients(
		docker.NewDockerCmdRunner(cmdRunner),
		kind.NewKindCmdRunner(cmdRunner),
		k8s.NewKubeCmdRunner(cmdRunner),
	)

	// Validate analyzer configuration for production use
	transportEnabled := s.IsConversationModeEnabled() || gm.transport != nil
	if err := mcpClients.ValidateAnalyzerForProduction(s.logger, transportEnabled); err != nil {
		// Log critical error but don't fail startup - let it continue with warning
		s.logger.Error().Err(err).Msg("Analyzer validation failed")
	}

	// Create pipeline operations (no adapter needed)
	pipelineOps := pipeline.NewOperations(
		s.sessionManager,
		mcpClients,
		s.logger,
	)

	// Use session manager directly - no adapter needed
	atomicSessionMgr := s.sessionManager

	// Create legacy clients for registry manager (which still uses old interface)
	legacyClients := &clients.Clients{
		AzOpenAIClient: nil, // No AI for atomic tools
		Docker:         docker.NewDockerCmdRunner(cmdRunner),
		Kind:           kind.NewKindCmdRunner(cmdRunner),
		Kube:           k8s.NewKubeCmdRunner(cmdRunner),
	}

	// Create registry manager
	registryManager := coredocker.NewRegistryManager(legacyClients, s.logger)

	return &ToolDependencies{
		Server:             s,
		SessionManager:     s.sessionManager,
		ToolOrchestrator:   s.toolOrchestrator,
		ToolRegistry:       s.toolRegistry,
		PipelineOperations: pipelineOps, // Direct pipeline operations
		AtomicSessionMgr:   atomicSessionMgr,
		MCPClients:         mcpClients,
		RegistryManager:    registryManager,
		Logger:             s.logger,
	}
}

// registerCoreTools registers essential core tools using standardized patterns
func (gm *GomcpManager) registerCoreTools(deps *ToolDependencies) error {
	// Create registrar for this function
	registrar := tools.NewStandardToolRegistrar(gm.server, deps.Logger)

	// Server health/status tool
	tools.RegisterSimpleTool(registrar, "server_status",
		"[Advanced] Diagnostic tool for debugging server issues - not needed for normal operations",
		func(ctx *server.Context, args *ServerStatusArgs) (*ServerStatusResult, error) {
			return gm.handleServerStatus(deps, args)
		})

	// Session management tools
	tools.RegisterSimpleTool(registrar, "list_sessions",
		"List all active containerization sessions with their metadata and status",
		func(ctx *server.Context, args *SessionListArgs) (*SessionListResult, error) {
			return gm.handleListSessions(deps, args)
		})

	tools.RegisterSimpleTool(registrar, "delete_session",
		"Delete a containerization session and clean up its resources",
		func(ctx *server.Context, args *SessionDeleteArgs) (*SessionDeleteResult, error) {
			return gm.handleDeleteSession(deps, args)
		})

	return nil
}

// registerAtomicTools registers containerization workflow tools via auto-registration
func (gm *GomcpManager) registerAtomicTools(deps *ToolDependencies) error {
	// Create registrar for this function
	registrar := tools.NewStandardToolRegistrar(gm.server, deps.Logger)

	// Use auto-registration adapter for zero-code approach
	autoRegistry := tools.NewAutoRegistrationAdapter()
	
	// Create adapter for orchestrator registry to unified interface
	registryAdapter := tools.NewOrchestratorRegistryAdapter(deps.ToolRegistry)
	
	deps.Logger.Info().Msg("🚀 Starting auto-registration of atomic tools")
	
	// Register tools ready for auto-registration
	if err := autoRegistry.RegisterAtomicTools(registryAdapter); err != nil {
		deps.Logger.Error().Err(err).Msg("Failed to auto-register atomic tools")
		return fmt.Errorf("auto-registration failed: %w", err)
	}

	// For tools not yet migrated to unified interface, create them manually (temporary)
	pendingTools := map[string]interface{}{
		"generate_dockerfile_atomic": tools.NewGenerateDockerfileTool(
			deps.AtomicSessionMgr,
			deps.Logger.With().Str("tool", "generate_dockerfile_atomic").Logger(),
		),
		"generate_manifests_atomic": tools.NewAtomicGenerateManifestsTool(
			deps.PipelineOperations,
			deps.AtomicSessionMgr,
			deps.Logger.With().Str("tool", "generate_manifests_atomic").Logger(),
		),
		"scan_secrets_atomic": tools.NewAtomicScanSecretsTool(
			deps.PipelineOperations,
			deps.AtomicSessionMgr,
			deps.Logger.With().Str("tool", "scan_secrets_atomic").Logger(),
		),
		"validate_dockerfile_atomic": tools.NewAtomicValidateDockerfileTool(
			deps.PipelineOperations,
			deps.AtomicSessionMgr,
			deps.Logger.With().Str("tool", "validate_dockerfile_atomic").Logger(),
		),
	}

	// Register pending tools manually until they're migrated to unified interface
	for name, tool := range pendingTools {
		if err := deps.ToolRegistry.RegisterTool(name, tool); err != nil {
			deps.Logger.Error().Err(err).Str("tool", name).Msg("Failed to register pending atomic tool")
		} else {
			deps.Logger.Info().Str("tool", name).Msg("⏳ Registered pending tool (awaiting interface migration)")
		}
	}

	// Now create GoMCP handlers that delegate to the orchestrator
	toolDescriptions := map[string]string{
		"analyze_repository":  "Analyze a repository to detect language, framework, and containerization requirements",
		"build_image":         "Build a Docker image from the analyzed repository using generated Dockerfile",
		"generate_dockerfile": "Generate a Dockerfile for the analyzed repository",
		"validate_dockerfile": "Validate a Dockerfile for best practices and potential issues",
		"pull_image":          "Pull a Docker image from a container registry",
		"tag_image":           "Tag a Docker image with a new name or reference",
		"scan_image_security": "Scan Docker images for security vulnerabilities using Trivy",
		"scan_secrets":        "Scan source code and configuration files for exposed secrets",
		"generate_manifests":  "Generate Kubernetes manifests for the containerized application",
		"push_image":          "Push the built Docker image to a container registry",
	}

	// Register GoMCP handlers that delegate to orchestrator
	for toolName, description := range toolDescriptions {
		atomicToolName := toolName + "_atomic"
		gm.registerOrchestratorTool(registrar, toolName, atomicToolName, description, deps)
	}

	// Special validation tool that delegates to orchestrator with modified args
	tools.RegisterSimpleTool(registrar, "validate_deployment",
		"Validate Kubernetes deployment by deploying to a local Kind cluster",
		func(ctx *server.Context, args *tools.AtomicDeployKubernetesArgs) (*tools.AtomicDeployKubernetesResult, error) {
			// Set dry run mode for validation
			args.DryRun = true

			// Call through orchestrator - create proper context
			goCtx := context.WithValue(context.Background(), "mcp_context", ctx)
			result, err := deps.ToolOrchestrator.ExecuteTool(goCtx, "deploy_kubernetes_atomic", *args, nil)
			if err != nil {
				return nil, err
			}

			// Type assert the result
			if deployResult, ok := result.(*tools.AtomicDeployKubernetesResult); ok {
				return deployResult, nil
			}

			return nil, fmt.Errorf("unexpected result type from deploy_kubernetes_atomic: %T", result)
		})

	return nil
}

// registerUtilityTools registers utility and management tools using standardized patterns
func (gm *GomcpManager) registerUtilityTools(deps *ToolDependencies) error {
	// Create registrar for this function
	registrar := tools.NewStandardToolRegistrar(gm.server, deps.Logger)

	// Job management
	tools.RegisterSimpleTool(registrar, "get_job_status",
		"Get the status of a running or completed job",
		func(ctx *server.Context, args *JobStatusArgs) (*JobStatusResult, error) {
			return gm.handleJobStatus(deps, args)
		})

	// Register GoMCP Resources instead of tools for logs and telemetry
	return gm.registerResources(registrar, deps)
}

// registerResources registers GoMCP resources for streaming access to logs and telemetry
func (gm *GomcpManager) registerResources(registrar *tools.StandardToolRegistrar, deps *ToolDependencies) error {
	// Logs Resource - provides streaming access to server logs
	logProvider := tools.CreateGlobalLogProvider()
	tools.RegisterResource(registrar, "logs/{level}", "Server logs filtered by level (trace, debug, info, warn, error)",
		func(ctx *server.Context, args struct {
			Level     string `path:"level"`
			Pattern   string `json:"pattern,omitempty"`
			TimeRange string `json:"time_range,omitempty"`
			Limit     int    `json:"limit,omitempty"`
			Format    string `json:"format,omitempty"`
		}) (interface{}, error) {
			// Convert to tool args format for compatibility
			toolArgs := tools.GetLogsArgs{
				Level:     args.Level,
				Pattern:   args.Pattern,
				TimeRange: args.TimeRange,
				Limit:     args.Limit,
				Format:    args.Format,
			}

			// Set defaults
			if toolArgs.Level == "" {
				toolArgs.Level = "info"
			}
			if toolArgs.Format == "" {
				toolArgs.Format = "json"
			}
			if toolArgs.Limit == 0 {
				toolArgs.Limit = 100
			}

			logsTool := tools.NewGetLogsTool(
				deps.Logger.With().Str("resource", "logs").Logger(),
				logProvider,
			)
			return logsTool.Execute(context.Background(), toolArgs)
		})

	// Simplified logs resource for direct access
	tools.RegisterResource(registrar, "logs", "All server logs with default filtering",
		func(ctx *server.Context, args struct {
			Pattern   string `json:"pattern,omitempty"`
			TimeRange string `json:"time_range,omitempty"`
			Limit     int    `json:"limit,omitempty"`
			Format    string `json:"format,omitempty"`
		}) (interface{}, error) {
			toolArgs := tools.GetLogsArgs{
				Level:     "info",
				Pattern:   args.Pattern,
				TimeRange: args.TimeRange,
				Limit:     args.Limit,
				Format:    args.Format,
			}

			if toolArgs.Format == "" {
				toolArgs.Format = "json"
			}
			if toolArgs.Limit == 0 {
				toolArgs.Limit = 100
			}

			logsTool := tools.NewGetLogsTool(
				deps.Logger.With().Str("resource", "logs").Logger(),
				logProvider,
			)
			return logsTool.Execute(context.Background(), toolArgs)
		})

	// Session label management tools - using standardized utility registration
	sessionLabelManager := &sessionLabelManagerWrapper{sm: deps.SessionManager}

	// Register session label tools using utility pattern
	tools.RegisterSimpleTool(registrar, "add_session_label",
		"Add a label to a session for organization and filtering",
		func(ctx *server.Context, args *tools.AddSessionLabelArgs) (*tools.AddSessionLabelResult, error) {
			addLabelTool := tools.NewAddSessionLabelTool(
				deps.Logger.With().Str("tool", "add_session_label").Logger(),
				sessionLabelManager,
			)
			return addLabelTool.Execute(context.Background(), *args)
		})

	tools.RegisterSimpleTool(registrar, "remove_session_label",
		"Remove a label from a session",
		func(ctx *server.Context, args *tools.RemoveSessionLabelArgs) (*tools.RemoveSessionLabelResult, error) {
			removeLabelTool := tools.NewRemoveSessionLabelTool(
				deps.Logger.With().Str("tool", "remove_session_label").Logger(),
				sessionLabelManager,
			)
			return removeLabelTool.Execute(context.Background(), *args)
		})

	tools.RegisterSimpleTool(registrar, "update_session_labels",
		"Update all labels on a session (replace existing labels)",
		func(ctx *server.Context, args *tools.UpdateSessionLabelsArgs) (*tools.UpdateSessionLabelsResult, error) {
			updateLabelsTool := tools.NewUpdateSessionLabelsTool(
				deps.Logger.With().Str("tool", "update_session_labels").Logger(),
				sessionLabelManager,
			)
			return updateLabelsTool.Execute(context.Background(), *args)
		})

	tools.RegisterSimpleTool(registrar, "list_session_labels",
		"List all labels across sessions with optional usage statistics",
		func(ctx *server.Context, args *tools.ListSessionLabelsArgs) (*tools.ListSessionLabelsResult, error) {
			listLabelsTool := tools.NewListSessionLabelsTool(
				deps.Logger.With().Str("tool", "list_session_labels").Logger(),
				sessionLabelManager,
			)
			return listLabelsTool.Execute(context.Background(), *args)
		})

	// Telemetry Resource (if enabled)
	if deps.Server.IsConversationModeEnabled() &&
		deps.Server.conversationComponents != nil &&
		deps.Server.conversationComponents.Telemetry != nil {

		tools.RegisterResource(registrar, "telemetry/metrics", "Prometheus telemetry metrics from the MCP server",
			func(ctx *server.Context, args struct {
				Format       string   `json:"format,omitempty"`
				MetricNames  []string `json:"metric_names,omitempty"`
				IncludeHelp  bool     `json:"include_help,omitempty"`
				TimeRange    string   `json:"time_range,omitempty"`
				IncludeEmpty bool     `json:"include_empty,omitempty"`
			}) (interface{}, error) {
				toolArgs := tools.GetTelemetryMetricsArgs{
					Format:       args.Format,
					MetricNames:  args.MetricNames,
					IncludeHelp:  args.IncludeHelp,
					TimeRange:    args.TimeRange,
					IncludeEmpty: args.IncludeEmpty,
				}

				if toolArgs.Format == "" {
					toolArgs.Format = "prometheus"
				}

				telemetryTool := tools.NewGetTelemetryMetricsTool(
					deps.Logger.With().Str("resource", "telemetry").Logger(),
					deps.Server.conversationComponents.Telemetry,
				)
				return telemetryTool.Execute(context.Background(), toolArgs)
			})

		// Metrics by specific name pattern
		tools.RegisterResource(registrar, "telemetry/metrics/{name}", "Specific telemetry metric by name pattern",
			func(ctx *server.Context, args struct {
				Name         string `path:"name"`
				Format       string `json:"format,omitempty"`
				IncludeHelp  bool   `json:"include_help,omitempty"`
				IncludeEmpty bool   `json:"include_empty,omitempty"`
			}) (interface{}, error) {
				toolArgs := tools.GetTelemetryMetricsArgs{
					Format:       args.Format,
					MetricNames:  []string{args.Name},
					IncludeHelp:  args.IncludeHelp,
					IncludeEmpty: args.IncludeEmpty,
				}

				if toolArgs.Format == "" {
					toolArgs.Format = "prometheus"
				}

				telemetryTool := tools.NewGetTelemetryMetricsTool(
					deps.Logger.With().Str("resource", "telemetry").Logger(),
					deps.Server.conversationComponents.Telemetry,
				)
				return telemetryTool.Execute(context.Background(), toolArgs)
			})
	}

	return nil
}

// registerConversationTools registers conversation mode tools using standardized patterns
func (gm *GomcpManager) registerConversationTools(deps *ToolDependencies) error {
	if deps.Server.conversationComponents == nil {
		return nil
	}

	// Create registrar for this function
	registrar := tools.NewStandardToolRegistrar(gm.server, deps.Logger)

	tools.RegisterSimpleTool(registrar, "chat",
		"Interact with the AI assistant for guided containerization workflow",
		func(ctx *server.Context, args *ChatArgs) (*ChatResult, error) {
			return gm.handleChat(deps, args)
		})

	return nil
}

// sessionLabelManagerWrapper adapts session.SessionManager to tools.SessionLabelManager interface
type sessionLabelManagerWrapper struct {
	sm *session.SessionManager
}

func (w *sessionLabelManagerWrapper) AddSessionLabel(sessionID, label string) error {
	return w.sm.AddSessionLabel(sessionID, label)
}

func (w *sessionLabelManagerWrapper) RemoveSessionLabel(sessionID, label string) error {
	return w.sm.RemoveSessionLabel(sessionID, label)
}

func (w *sessionLabelManagerWrapper) SetSessionLabels(sessionID string, labels []string) error {
	return w.sm.SetSessionLabels(sessionID, labels)
}

func (w *sessionLabelManagerWrapper) GetAllLabels() []string {
	return w.sm.GetAllLabels()
}

func (w *sessionLabelManagerWrapper) GetSession(sessionID string) (tools.SessionLabelData, error) {
	sessionInterface, err := w.sm.GetSession(sessionID)
	if err != nil {
		return tools.SessionLabelData{}, err
	}

	session, ok := sessionInterface.(*sessiontypes.SessionState)
	if !ok {
		return tools.SessionLabelData{}, fmt.Errorf("unexpected session type")
	}

	return tools.SessionLabelData{
		SessionID: session.SessionID,
		Labels:    session.Labels,
	}, nil
}

func (w *sessionLabelManagerWrapper) ListSessions() []tools.SessionLabelData {
	summaries := w.sm.ListSessionSummaries()
	result := make([]tools.SessionLabelData, len(summaries))
	for i, summary := range summaries {
		result[i] = tools.SessionLabelData{
			SessionID: summary.SessionID,
			Labels:    summary.Labels,
		}
	}
	return result
}

// registerOrchestratorTool creates a GoMCP handler that delegates to the orchestrator
func (gm *GomcpManager) registerOrchestratorTool(registrar *tools.StandardToolRegistrar, toolName, atomicToolName, description string, deps *ToolDependencies) {
	deps.Logger.Debug().
		Str("tool", toolName).
		Str("atomic_tool", atomicToolName).
		Msg("Registering orchestrator-delegated tool")

	gm.server.Tool(toolName, description, func(ctx *server.Context, args interface{}) (interface{}, error) {
		// Execute through the canonical orchestrator - create proper context
		goCtx := context.WithValue(context.Background(), "mcp_context", ctx)
		result, err := deps.ToolOrchestrator.ExecuteTool(goCtx, atomicToolName, args, nil)
		if err != nil {
			deps.Logger.Error().
				Err(err).
				Str("tool", toolName).
				Str("atomic_tool", atomicToolName).
				Msg("Tool execution failed through orchestrator")
			return nil, err
		}

		deps.Logger.Debug().
			Str("tool", toolName).
			Str("atomic_tool", atomicToolName).
			Msg("Tool executed successfully through orchestrator")

		return result, nil
	})

	deps.Logger.Info().
		Str("tool", toolName).
		Str("atomic_tool", atomicToolName).
		Msg("Orchestrator-delegated tool registered successfully")
}
