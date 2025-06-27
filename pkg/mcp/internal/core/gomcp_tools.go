package core

import (
	"context"
	"fmt"

	"github.com/Azure/container-kit/pkg/clients"
	coredocker "github.com/Azure/container-kit/pkg/core/docker"
	"github.com/Azure/container-kit/pkg/docker"
	"github.com/Azure/container-kit/pkg/k8s"
	"github.com/Azure/container-kit/pkg/kind"
	"github.com/Azure/container-kit/pkg/mcp/internal/analyze"
	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	"github.com/Azure/container-kit/pkg/mcp/internal/deploy"
	"github.com/Azure/container-kit/pkg/mcp/internal/orchestration"
	"github.com/Azure/container-kit/pkg/mcp/internal/pipeline"
	"github.com/Azure/container-kit/pkg/mcp/internal/runtime"
	"github.com/Azure/container-kit/pkg/mcp/internal/scan"
	mcpserver "github.com/Azure/container-kit/pkg/mcp/internal/server"
	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	sessiontypes "github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/Azure/container-kit/pkg/runner"
	gomcpserver "github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// contextKey is used as a key for context values to avoid collisions
type contextKey string

const mcpContextKey contextKey = "mcp_context"

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
		return types.NewErrorBuilder("manager_not_initialized", "Manager must be initialized before registering tools", "initialization").
			WithSeverity("high").
			WithOperation("register_tools").
			WithStage("initialization_check").
			WithRootCause("GomcpManager.Initialize() was not called").
			WithImmediateStep(1, "Initialize manager", "Call GomcpManager.Initialize() before registering tools").
			WithImmediateStep(2, "Check setup", "Verify all dependencies are properly configured").
			Build()
	}

	// Create dependencies for tools
	deps := gm.createToolDependencies(s)

	// Set pipeline operations on the orchestrator for type-safe dispatch
	if deps.ToolOrchestrator != nil && deps.PipelineOperations != nil && deps.AtomicSessionMgr != nil {
		deps.ToolOrchestrator.SetPipelineOperations(deps.PipelineOperations)

		// Create and set the tool factory with concrete types
		toolFactory := orchestration.NewToolFactory(deps.PipelineOperations, deps.AtomicSessionMgr, deps.MCPClients.Analyzer, deps.Logger)

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
		return types.NewErrorBuilder("core_tools_registration_failed", "Failed to register core tools", "registration").
			WithSeverity("high").
			WithOperation("register_tools").
			WithStage("core_tools").
			WithRootCause(fmt.Sprintf("Core tool registration failed: %v", err)).
			WithImmediateStep(1, "Check dependencies", "Verify all core tool dependencies are available").
			WithImmediateStep(2, "Check configuration", "Ensure tool configuration is valid").
			Build()
	}
	deps.Logger.Info().Msg("Core tools registered successfully")

	// Register atomic tools
	deps.Logger.Info().Msg("Registering atomic tools")
	if err := gm.registerAtomicTools(deps); err != nil {
		return types.NewErrorBuilder("atomic_tools_registration_failed", "Failed to register atomic tools", "registration").
			WithSeverity("high").
			WithOperation("register_tools").
			WithStage("atomic_tools").
			WithRootCause(fmt.Sprintf("Atomic tool registration failed: %v", err)).
			WithImmediateStep(1, "Check dependencies", "Verify all atomic tool dependencies are available").
			WithImmediateStep(2, "Check Docker/K8s", "Ensure Docker and Kubernetes clients are functional").
			Build()
	}
	deps.Logger.Info().Msg("Atomic tools registered successfully")

	// Register utility tools
	deps.Logger.Info().Msg("Registering utility tools")
	if err := gm.registerUtilityTools(deps); err != nil {
		return types.NewErrorBuilder("utility_tools_registration_failed", "Failed to register utility tools", "registration").
			WithSeverity("high").
			WithOperation("register_tools").
			WithStage("utility_tools").
			WithRootCause(fmt.Sprintf("Utility tool registration failed: %v", err)).
			WithImmediateStep(1, "Check dependencies", "Verify all utility tool dependencies are available").
			WithImmediateStep(2, "Check configuration", "Ensure utility tool configuration is valid").
			Build()
	}
	deps.Logger.Info().Msg("Utility tools registered successfully")

	// Register conversation tools if enabled
	if s.IsConversationModeEnabled() {
		if err := gm.registerConversationTools(deps); err != nil {
			return types.NewErrorBuilder("conversation_tools_registration_failed", "Failed to register conversation tools", "registration").
				WithSeverity("high").
				WithOperation("register_tools").
				WithStage("conversation_tools").
				WithRootCause(fmt.Sprintf("Conversation tool registration failed: %v", err)).
				WithImmediateStep(1, "Check mode", "Verify conversation mode is properly enabled").
				WithImmediateStep(2, "Check dependencies", "Ensure conversation tool dependencies are available").
				Build()
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
	MCPClients         *mcptypes.MCPClients
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
	mcpClients := mcptypes.NewMCPClients(
		docker.NewDockerCmdRunner(cmdRunner),
		kind.NewKindCmdRunner(cmdRunner),
		k8s.NewKubeCmdRunner(cmdRunner),
	)

	// Validate analyzer configuration for production use
	if err := mcpClients.ValidateAnalyzerForProduction(s.logger); err != nil {
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
	registrar := runtime.NewStandardToolRegistrar(gm.server, deps.Logger)

	// Server health/status tool
	runtime.RegisterSimpleTool(registrar, "server_status",
		"[Advanced] Diagnostic tool for debugging server issues - not needed for normal operations",
		func(ctx *gomcpserver.Context, args *ServerStatusArgs) (*ServerStatusResult, error) {
			return gm.handleServerStatus(deps, args)
		})

	// Session management tools
	runtime.RegisterSimpleTool(registrar, "list_sessions",
		"List all active containerization sessions with their metadata and status",
		func(ctx *gomcpserver.Context, args *SessionListArgs) (*SessionListResult, error) {
			return gm.handleListSessions(deps, args)
		})

	runtime.RegisterSimpleTool(registrar, "delete_session",
		"Delete a containerization session and clean up its resources",
		func(ctx *gomcpserver.Context, args *SessionDeleteArgs) (*SessionDeleteResult, error) {
			return gm.handleDeleteSession(deps, args)
		})

	return nil
}

// registerAtomicTools registers containerization workflow tools via auto-registration
func (gm *GomcpManager) registerAtomicTools(deps *ToolDependencies) error {
	// Create registrar for this function
	registrar := runtime.NewStandardToolRegistrar(gm.server, deps.Logger)

	// Register atomic tools with orchestrator
	if err := gm.registerAtomicToolsWithOrchestrator(deps); err != nil {
		return err
	}

	// Register GoMCP handlers
	if err := gm.registerBasicTools(registrar, deps); err != nil {
		return err
	}

	if err := gm.registerValidationTool(registrar, deps); err != nil {
		return err
	}

	if err := gm.registerFixedSchemaTools(registrar, deps); err != nil {
		return err
	}

	return nil
}

// registerAtomicToolsWithOrchestrator creates and registers atomic tools with the orchestrator
func (gm *GomcpManager) registerAtomicToolsWithOrchestrator(deps *ToolDependencies) error {
	atomicTools := map[string]interface{}{
		"analyze_repository_atomic": analyze.NewAtomicAnalyzeRepositoryTool(
			deps.PipelineOperations,
			deps.AtomicSessionMgr,
			deps.Logger.With().Str("tool", "analyze_repository_atomic").Logger(),
		),
		"build_image_atomic": build.NewAtomicBuildImageTool(
			deps.PipelineOperations,
			deps.AtomicSessionMgr,
			deps.Logger.With().Str("tool", "build_image_atomic").Logger(),
		),
		"generate_dockerfile_atomic": analyze.NewGenerateDockerfileTool(
			deps.AtomicSessionMgr,
			deps.Logger.With().Str("tool", "generate_dockerfile_atomic").Logger(),
		),
		"deploy_kubernetes_atomic": deploy.NewAtomicDeployKubernetesTool(
			deps.PipelineOperations,
			deps.AtomicSessionMgr,
			deps.Logger.With().Str("tool", "deploy_kubernetes_atomic").Logger(),
		),
		"validate_dockerfile_atomic": analyze.NewAtomicValidateDockerfileTool(
			deps.PipelineOperations,
			deps.AtomicSessionMgr,
			deps.Logger.With().Str("tool", "validate_dockerfile_atomic").Logger(),
		),
		"pull_image_atomic": build.NewAtomicPullImageTool(
			deps.PipelineOperations,
			deps.AtomicSessionMgr,
			deps.Logger.With().Str("tool", "pull_image_atomic").Logger(),
		),
		"tag_image_atomic": build.NewAtomicTagImageTool(
			deps.PipelineOperations,
			deps.AtomicSessionMgr,
			deps.Logger.With().Str("tool", "tag_image_atomic").Logger(),
		),
		"scan_image_security_atomic": scan.NewAtomicScanImageSecurityTool(
			deps.PipelineOperations,
			deps.AtomicSessionMgr,
			deps.Logger.With().Str("tool", "scan_image_security_atomic").Logger(),
		),
		"scan_secrets_atomic": scan.NewAtomicScanSecretsTool(
			deps.PipelineOperations,
			deps.AtomicSessionMgr,
			deps.Logger.With().Str("tool", "scan_secrets_atomic").Logger(),
		),
		"generate_manifests_atomic": deploy.NewAtomicGenerateManifestsTool(
			deps.PipelineOperations,
			deps.AtomicSessionMgr,
			deps.Logger.With().Str("tool", "generate_manifests_atomic").Logger(),
		),
		"push_image_atomic": build.NewAtomicPushImageTool(
			deps.PipelineOperations,
			deps.AtomicSessionMgr,
			deps.Logger.With().Str("tool", "push_image_atomic").Logger(),
		),
	}

	// Register tools with the orchestrator's tool registry
	for name, tool := range atomicTools {
		if err := deps.ToolRegistry.RegisterTool(name, tool); err != nil {
			deps.Logger.Error().Err(err).Str("tool", name).Msg("Failed to register atomic tool")
		} else {
			deps.Logger.Info().Str("tool", name).Msg("Registered atomic tool successfully")
		}
	}
	return nil
}

// registerBasicTools registers basic tools with simple schema
func (gm *GomcpManager) registerBasicTools(registrar *runtime.StandardToolRegistrar, deps *ToolDependencies) error {

	gm.registerAnalyzeRepository(registrar, deps)
	gm.registerGenerateDockerfile(registrar, deps)
	gm.registerBuildImage(registrar, deps)
	gm.registerPullImage(registrar, deps)
	gm.registerTagImage(registrar, deps)
	gm.registerPushImage(registrar, deps)
	return nil
}

// ensureSessionID ensures args have a valid session ID, creating one if needed
func (gm *GomcpManager) ensureSessionID(sessionID string, deps *ToolDependencies, toolName string) (string, error) {
	if sessionID == "" {
		sessionInterface, err := deps.SessionManager.GetOrCreateSession("")
		if err != nil {
			return "", types.NewErrorBuilder("session_creation_failed", "Failed to create containerization session", "session").
				WithOperation("handle_containerize_repository").
				WithStage("session_creation").
				WithRootCause(fmt.Sprintf("Session creation failed: %v", err)).
				WithImmediateStep(1, "Check resources", "Verify system resources are available for new session").
				WithImmediateStep(2, "Check limits", "Ensure session limits are not exceeded").
				WithImmediateStep(3, "Retry creation", "Retry session creation after brief delay").
				Build()
		}
		if session, ok := sessionInterface.(*sessiontypes.SessionState); ok {
			deps.Logger.Info().Str("session_id", session.SessionID).Str("tool", toolName).Msg("Created new session")
			return session.SessionID, nil
		}
	}
	return sessionID, nil
}

// registerAnalyzeRepository registers the analyze_repository tool
func (gm *GomcpManager) registerAnalyzeRepository(registrar *runtime.StandardToolRegistrar, deps *ToolDependencies) {
	runtime.RegisterSimpleTool(registrar, "analyze_repository",
		"Analyze a repository to detect language, framework, and containerization requirements",
		func(ctx *gomcpserver.Context, args *analyze.AtomicAnalyzeRepositoryArgs) (*analyze.AtomicAnalysisResult, error) {
			sessionID, err := gm.ensureSessionID(args.SessionID, deps, "analyze_repository")
			if err != nil {
				return nil, err
			}
			args.SessionID = sessionID

			argsMap, err := BuildArgsMap(args)
			if err != nil {
				return nil, fmt.Errorf("failed to build arguments map: %w", err)
			}

			goCtx := context.WithValue(context.Background(), mcpContextKey, ctx)
			result, err := deps.ToolOrchestrator.ExecuteTool(goCtx, "analyze_repository_atomic", argsMap, nil)
			if err != nil {
				return nil, err
			}
			if analysisResult, ok := result.(*analyze.AtomicAnalysisResult); ok {
				return analysisResult, nil
			}
			return nil, types.NewErrorBuilder("unexpected_result_type", "Unexpected result type from analyze_repository_atomic tool", "tool_execution").
				WithField("expected_type", "*types.AnalyzeRepositoryResult").
				WithField("actual_type", fmt.Sprintf("%T", result)).
				WithOperation("handle_analyze_repository").
				WithStage("result_processing").
				WithRootCause(fmt.Sprintf("Tool returned unexpected type %T instead of expected result type", result)).
				WithImmediateStep(1, "Check tool", "Verify analyze_repository_atomic tool implementation").
				WithImmediateStep(2, "Check version", "Ensure tool version compatibility with expected interface").
				Build()
		})
}

// registerGenerateDockerfile registers the generate_dockerfile tool
func (gm *GomcpManager) registerGenerateDockerfile(registrar *runtime.StandardToolRegistrar, deps *ToolDependencies) {
	runtime.RegisterSimpleTool(registrar, "generate_dockerfile",
		"Generate a Dockerfile for the analyzed repository",
		func(ctx *gomcpserver.Context, args *analyze.GenerateDockerfileArgs) (*analyze.GenerateDockerfileResult, error) {
			sessionID, err := gm.ensureSessionID(args.SessionID, deps, "generate_dockerfile")
			if err != nil {
				return nil, err
			}
			args.SessionID = sessionID

			argsMap, err := BuildArgsMap(args)
			if err != nil {
				return nil, fmt.Errorf("failed to build arguments map: %w", err)
			}

			goCtx := context.WithValue(context.Background(), mcpContextKey, ctx)
			result, err := deps.ToolOrchestrator.ExecuteTool(goCtx, "generate_dockerfile", argsMap, nil)
			if err != nil {
				return nil, err
			}
			if dockerfileResult, ok := result.(*analyze.GenerateDockerfileResult); ok {
				return dockerfileResult, nil
			}
			return nil, fmt.Errorf("unexpected result type from generate_dockerfile: %T", result)
		})
}

// registerBuildImage registers the build_image tool
func (gm *GomcpManager) registerBuildImage(registrar *runtime.StandardToolRegistrar, deps *ToolDependencies) {
	runtime.RegisterSimpleTool(registrar, "build_image",
		"Build a Docker image from the analyzed repository using generated Dockerfile",
		func(ctx *gomcpserver.Context, args *build.AtomicBuildImageArgs) (*build.AtomicBuildImageResult, error) {
			sessionID, err := gm.ensureSessionID(args.SessionID, deps, "build_image")
			if err != nil {
				return nil, err
			}
			args.SessionID = sessionID

			argsMap, err := BuildArgsMap(args)
			if err != nil {
				return nil, fmt.Errorf("failed to build arguments map: %w", err)
			}

			goCtx := context.WithValue(context.Background(), mcpContextKey, ctx)
			result, err := deps.ToolOrchestrator.ExecuteTool(goCtx, "build_image_atomic", argsMap, nil)
			if err != nil {
				return nil, err
			}
			if buildResult, ok := result.(*build.AtomicBuildImageResult); ok {
				return buildResult, nil
			}
			return nil, fmt.Errorf("unexpected result type from build_image_atomic: %T", result)
		})
}

// registerPullImage registers the pull_image tool
func (gm *GomcpManager) registerPullImage(registrar *runtime.StandardToolRegistrar, deps *ToolDependencies) {
	runtime.RegisterSimpleTool(registrar, "pull_image",
		"Pull a Docker image from a container registry",
		func(ctx *gomcpserver.Context, args *build.AtomicPullImageArgs) (*build.AtomicPullImageResult, error) {
			sessionID, err := gm.ensureSessionID(args.SessionID, deps, "pull_image")
			if err != nil {
				return nil, err
			}
			args.SessionID = sessionID

			argsMap, err := BuildArgsMap(args)
			if err != nil {
				return nil, fmt.Errorf("failed to build arguments map: %w", err)
			}

			goCtx := context.WithValue(context.Background(), mcpContextKey, ctx)
			result, err := deps.ToolOrchestrator.ExecuteTool(goCtx, "pull_image_atomic", argsMap, nil)
			if err != nil {
				return nil, err
			}
			if pullResult, ok := result.(*build.AtomicPullImageResult); ok {
				return pullResult, nil
			}
			return nil, fmt.Errorf("unexpected result type from pull_image_atomic: %T", result)
		})
}

// registerTagImage registers the tag_image tool
func (gm *GomcpManager) registerTagImage(registrar *runtime.StandardToolRegistrar, deps *ToolDependencies) {
	runtime.RegisterSimpleTool(registrar, "tag_image",
		"Tag a Docker image with a new name or reference",
		func(ctx *gomcpserver.Context, args *build.AtomicTagImageArgs) (*build.AtomicTagImageResult, error) {
			sessionID, err := gm.ensureSessionID(args.SessionID, deps, "tag_image")
			if err != nil {
				return nil, err
			}
			args.SessionID = sessionID

			argsMap, err := BuildArgsMap(args)
			if err != nil {
				return nil, fmt.Errorf("failed to build arguments map: %w", err)
			}

			goCtx := context.WithValue(context.Background(), mcpContextKey, ctx)
			result, err := deps.ToolOrchestrator.ExecuteTool(goCtx, "tag_image_atomic", argsMap, nil)
			if err != nil {
				return nil, err
			}
			if tagResult, ok := result.(*build.AtomicTagImageResult); ok {
				return tagResult, nil
			}
			return nil, fmt.Errorf("unexpected result type from tag_image_atomic: %T", result)
		})
}

// registerPushImage registers the push_image tool
func (gm *GomcpManager) registerPushImage(registrar *runtime.StandardToolRegistrar, deps *ToolDependencies) {
	runtime.RegisterSimpleTool(registrar, "push_image",
		"Push the built Docker image to a container registry",
		func(ctx *gomcpserver.Context, args *build.AtomicPushImageArgs) (*build.AtomicPushImageResult, error) {
			sessionID, err := gm.ensureSessionID(args.SessionID, deps, "push_image")
			if err != nil {
				return nil, err
			}
			args.SessionID = sessionID

			argsMap, err := BuildArgsMap(args)
			if err != nil {
				return nil, fmt.Errorf("failed to build arguments map: %w", err)
			}

			goCtx := context.WithValue(context.Background(), mcpContextKey, ctx)
			result, err := deps.ToolOrchestrator.ExecuteTool(goCtx, "push_image_atomic", argsMap, nil)
			if err != nil {
				return nil, err
			}
			if pushResult, ok := result.(*build.AtomicPushImageResult); ok {
				return pushResult, nil
			}
			return nil, fmt.Errorf("unexpected result type from push_image_atomic: %T", result)
		})
}

// registerValidationTool registers the validation tool
func (gm *GomcpManager) registerValidationTool(registrar *runtime.StandardToolRegistrar, deps *ToolDependencies) error {

	runtime.RegisterSimpleTool(registrar, "validate_deployment",
		"Validate Kubernetes deployment by deploying to a local Kind cluster",
		func(ctx *gomcpserver.Context, args *deploy.AtomicDeployKubernetesArgs) (*deploy.AtomicDeployKubernetesResult, error) {
			argsMap, err := BuildArgsMap(args)
			if err != nil {
				return nil, fmt.Errorf("failed to build arguments map: %w", err)
			}
			// Force dry_run to true for validation
			argsMap["dry_run"] = true

			goCtx := context.WithValue(context.Background(), mcpContextKey, ctx)
			result, err := deps.ToolOrchestrator.ExecuteTool(goCtx, "deploy_kubernetes_atomic", argsMap, nil)
			if err != nil {
				return nil, err
			}
			if deployResult, ok := result.(*deploy.AtomicDeployKubernetesResult); ok {
				return deployResult, nil
			}
			return nil, fmt.Errorf("unexpected result type from deploy_kubernetes_atomic: %T", result)
		})
	return nil
}

// registerFixedSchemaTools registers tools with fixed schema
func (gm *GomcpManager) registerFixedSchemaTools(registrar *runtime.StandardToolRegistrar, deps *ToolDependencies) error {

	gm.registerGenerateManifests(registrar, deps)
	gm.registerValidateDockerfile(registrar, deps)
	gm.registerScanImageSecurity(registrar, deps)
	gm.registerScanSecrets(registrar, deps)
	return nil
}

// registerGenerateManifests registers the generate_manifests tool
func (gm *GomcpManager) registerGenerateManifests(registrar *runtime.StandardToolRegistrar, deps *ToolDependencies) {
	runtime.RegisterSimpleToolWithFixedSchema(registrar, "generate_manifests",
		"Generate Kubernetes manifests for the containerized application",
		func(ctx *gomcpserver.Context, args *deploy.AtomicGenerateManifestsArgs) (*deploy.AtomicGenerateManifestsResult, error) {
			// Ensure session ID is set
			sessionID, err := gm.ensureSessionID(args.SessionID, deps, "generate_manifests")
			if err != nil {
				return nil, err
			}
			args.SessionID = sessionID

			argsMap, err := BuildArgsMap(args)
			if err != nil {
				return nil, fmt.Errorf("failed to build arguments map: %w", err)
			}
			// Special handling for image_ref field
			argsMap["image_ref"] = args.ImageRef.Repository

			goCtx := context.WithValue(context.Background(), mcpContextKey, ctx)
			result, err := deps.ToolOrchestrator.ExecuteTool(goCtx, "generate_manifests_atomic", argsMap, nil)
			if err != nil {
				return nil, err
			}
			if manifestsResult, ok := result.(*deploy.AtomicGenerateManifestsResult); ok {
				return manifestsResult, nil
			}
			return nil, fmt.Errorf("unexpected result type from generate_manifests_atomic: %T", result)
		})
}

// registerValidateDockerfile registers the validate_dockerfile tool
func (gm *GomcpManager) registerValidateDockerfile(registrar *runtime.StandardToolRegistrar, deps *ToolDependencies) {
	runtime.RegisterSimpleToolWithFixedSchema(registrar, "validate_dockerfile",
		"Validate a Dockerfile for best practices and potential issues",
		func(ctx *gomcpserver.Context, args *analyze.AtomicValidateDockerfileArgs) (*analyze.AtomicValidateDockerfileResult, error) {
			// Ensure session ID is set
			sessionID, err := gm.ensureSessionID(args.SessionID, deps, "validate_dockerfile")
			if err != nil {
				return nil, err
			}
			args.SessionID = sessionID

			argsMap, err := BuildArgsMap(args)
			if err != nil {
				return nil, fmt.Errorf("failed to build arguments map: %w", err)
			}

			goCtx := context.WithValue(context.Background(), mcpContextKey, ctx)
			result, err := deps.ToolOrchestrator.ExecuteTool(goCtx, "validate_dockerfile_atomic", argsMap, nil)
			if err != nil {
				return nil, err
			}
			if validateResult, ok := result.(*analyze.AtomicValidateDockerfileResult); ok {
				return validateResult, nil
			}
			return nil, fmt.Errorf("unexpected result type from validate_dockerfile_atomic: %T", result)
		})
}

// registerScanImageSecurity registers the scan_image_security tool
func (gm *GomcpManager) registerScanImageSecurity(registrar *runtime.StandardToolRegistrar, deps *ToolDependencies) {
	runtime.RegisterSimpleToolWithFixedSchema(registrar, "scan_image_security",
		"Scan Docker images for security vulnerabilities using Trivy",
		func(ctx *gomcpserver.Context, args *scan.AtomicScanImageSecurityArgs) (*scan.AtomicScanImageSecurityResult, error) {
			// Ensure session ID is set
			sessionID, err := gm.ensureSessionID(args.SessionID, deps, "scan_image_security")
			if err != nil {
				return nil, err
			}
			args.SessionID = sessionID

			argsMap, err := BuildArgsMap(args)
			if err != nil {
				return nil, fmt.Errorf("failed to build arguments map: %w", err)
			}

			goCtx := context.WithValue(context.Background(), mcpContextKey, ctx)
			result, err := deps.ToolOrchestrator.ExecuteTool(goCtx, "scan_image_security_atomic", argsMap, nil)
			if err != nil {
				return nil, err
			}
			if scanResult, ok := result.(*scan.AtomicScanImageSecurityResult); ok {
				return scanResult, nil
			}
			return nil, fmt.Errorf("unexpected result type from scan_image_security_atomic: %T", result)
		})
}

// registerScanSecrets registers the scan_secrets tool
func (gm *GomcpManager) registerScanSecrets(registrar *runtime.StandardToolRegistrar, deps *ToolDependencies) {
	runtime.RegisterSimpleToolWithFixedSchema(registrar, "scan_secrets",
		"Scan source code and configuration files for exposed secrets",
		func(ctx *gomcpserver.Context, args *scan.AtomicScanSecretsArgs) (*scan.AtomicScanSecretsResult, error) {
			// Ensure session ID is set
			sessionID, err := gm.ensureSessionID(args.SessionID, deps, "scan_secrets")
			if err != nil {
				return nil, err
			}
			args.SessionID = sessionID

			argsMap, err := BuildArgsMap(args)
			if err != nil {
				return nil, fmt.Errorf("failed to build arguments map: %w", err)
			}

			goCtx := context.WithValue(context.Background(), mcpContextKey, ctx)
			result, err := deps.ToolOrchestrator.ExecuteTool(goCtx, "scan_secrets_atomic", argsMap, nil)
			if err != nil {
				return nil, err
			}
			if scanResult, ok := result.(*scan.AtomicScanSecretsResult); ok {
				return scanResult, nil
			}
			return nil, fmt.Errorf("unexpected result type from scan_secrets_atomic: %T", result)
		})
}

// registerUtilityTools registers utility and management tools using standardized patterns
func (gm *GomcpManager) registerUtilityTools(deps *ToolDependencies) error {
	// Create registrar for this function
	registrar := runtime.NewStandardToolRegistrar(gm.server, deps.Logger)

	// Job management
	runtime.RegisterSimpleTool(registrar, "get_job_status",
		"Get the status of a running or completed job",
		func(ctx *gomcpserver.Context, args *JobStatusArgs) (*JobStatusResult, error) {
			return gm.handleJobStatus(deps, args)
		})

	// Register GoMCP Resources instead of tools for logs and telemetry
	return gm.registerResources(registrar, deps)
}

// registerResources registers GoMCP resources for streaming access to logs and telemetry
func (gm *GomcpManager) registerResources(registrar *runtime.StandardToolRegistrar, deps *ToolDependencies) error {
	// Logs Resource - provides streaming access to server logs
	logProvider := mcpserver.CreateGlobalLogProvider()
	runtime.RegisterResource(registrar, "logs/{level}", "Server logs filtered by level (trace, debug, info, warn, error)",
		func(ctx *gomcpserver.Context, args struct {
			Level     string `path:"level"`
			Pattern   string `json:"pattern,omitempty"`
			TimeRange string `json:"time_range,omitempty"`
			Limit     int    `json:"limit,omitempty"`
			Format    string `json:"format,omitempty"`
		}) (interface{}, error) {
			// Convert to tool args format for compatibility
			toolArgs := mcpserver.GetLogsArgs{
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

			logsTool := mcpserver.NewGetLogsTool(
				deps.Logger.With().Str("resource", "logs").Logger(),
				logProvider,
			)
			return logsTool.ExecuteTyped(context.Background(), toolArgs)
		})

	// Simplified logs resource for direct access
	runtime.RegisterResource(registrar, "logs", "All server logs with default filtering",
		func(ctx *gomcpserver.Context, args struct {
			Pattern   string `json:"pattern,omitempty"`
			TimeRange string `json:"time_range,omitempty"`
			Limit     int    `json:"limit,omitempty"`
			Format    string `json:"format,omitempty"`
		}) (interface{}, error) {
			toolArgs := mcpserver.GetLogsArgs{
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

			logsTool := mcpserver.NewGetLogsTool(
				deps.Logger.With().Str("resource", "logs").Logger(),
				logProvider,
			)
			return logsTool.ExecuteTyped(context.Background(), toolArgs)
		})

	// Session label management tools - using standardized utility registration
	sessionLabelManager := &sessionLabelManagerWrapper{sm: deps.SessionManager}

	// Register session label tools using utility pattern
	runtime.RegisterSimpleTool(registrar, "add_session_label",
		"Add a label to a session for organization and filtering",
		func(ctx *gomcpserver.Context, args *sessiontypes.AddSessionLabelArgs) (*sessiontypes.AddSessionLabelResult, error) {
			addLabelTool := sessiontypes.NewAddSessionLabelTool(
				deps.Logger.With().Str("tool", "add_session_label").Logger(),
				sessionLabelManager,
			)
			return addLabelTool.ExecuteTyped(context.Background(), args)
		})

	runtime.RegisterSimpleTool(registrar, "remove_session_label",
		"Remove a label from a session",
		func(ctx *gomcpserver.Context, args *sessiontypes.RemoveSessionLabelArgs) (*sessiontypes.RemoveSessionLabelResult, error) {
			removeLabelTool := sessiontypes.NewRemoveSessionLabelTool(
				deps.Logger.With().Str("tool", "remove_session_label").Logger(),
				sessionLabelManager,
			)
			return removeLabelTool.ExecuteTyped(context.Background(), args)
		})

	runtime.RegisterSimpleToolWithFixedSchema(registrar, "update_session_labels",
		"Update all labels on a session (replace existing labels)",
		func(ctx *gomcpserver.Context, args *sessiontypes.UpdateSessionLabelsArgs) (*sessiontypes.UpdateSessionLabelsResult, error) {
			updateLabelsTool := sessiontypes.NewUpdateSessionLabelsTool(
				deps.Logger.With().Str("tool", "update_session_labels").Logger(),
				sessionLabelManager,
			)
			return updateLabelsTool.ExecuteTyped(context.Background(), *args)
		})

	runtime.RegisterSimpleTool(registrar, "list_session_labels",
		"List all labels across sessions with optional usage statistics",
		func(ctx *gomcpserver.Context, args *sessiontypes.ListSessionLabelsArgs) (*sessiontypes.ListSessionLabelsResult, error) {
			listLabelsTool := sessiontypes.NewListSessionLabelsTool(
				deps.Logger.With().Str("tool", "list_session_labels").Logger(),
				sessionLabelManager,
			)
			return listLabelsTool.ExecuteTyped(context.Background(), args)
		})

	// Telemetry Resource (if enabled)
	if deps.Server.IsConversationModeEnabled() &&
		deps.Server.conversationComponents != nil &&
		deps.Server.conversationComponents.Telemetry != nil {

		runtime.RegisterResource(registrar, "telemetry/metrics", "Prometheus telemetry metrics from the MCP server",
			func(ctx *gomcpserver.Context, args struct {
				Format       string   `json:"format,omitempty"`
				MetricNames  []string `json:"metric_names,omitempty"`
				IncludeHelp  bool     `json:"include_help,omitempty"`
				TimeRange    string   `json:"time_range,omitempty"`
				IncludeEmpty bool     `json:"include_empty,omitempty"`
			}) (interface{}, error) {
				toolArgs := mcpserver.GetTelemetryMetricsArgs{
					Format:       args.Format,
					MetricNames:  args.MetricNames,
					IncludeHelp:  args.IncludeHelp,
					TimeRange:    args.TimeRange,
					IncludeEmpty: args.IncludeEmpty,
				}

				if toolArgs.Format == "" {
					toolArgs.Format = "prometheus"
				}

				telemetryTool := mcpserver.NewGetTelemetryMetricsTool(
					deps.Logger.With().Str("resource", "telemetry").Logger(),
					deps.Server.conversationComponents.Telemetry,
				)
				return telemetryTool.ExecuteTyped(context.Background(), toolArgs)
			})

		// Metrics by specific name pattern
		runtime.RegisterResource(registrar, "telemetry/metrics/{name}", "Specific telemetry metric by name pattern",
			func(ctx *gomcpserver.Context, args struct {
				Name         string `path:"name"`
				Format       string `json:"format,omitempty"`
				IncludeHelp  bool   `json:"include_help,omitempty"`
				IncludeEmpty bool   `json:"include_empty,omitempty"`
			}) (interface{}, error) {
				toolArgs := mcpserver.GetTelemetryMetricsArgs{
					Format:       args.Format,
					MetricNames:  []string{args.Name},
					IncludeHelp:  args.IncludeHelp,
					IncludeEmpty: args.IncludeEmpty,
				}

				if toolArgs.Format == "" {
					toolArgs.Format = "prometheus"
				}

				telemetryTool := mcpserver.NewGetTelemetryMetricsTool(
					deps.Logger.With().Str("resource", "telemetry").Logger(),
					deps.Server.conversationComponents.Telemetry,
				)
				return telemetryTool.ExecuteTyped(context.Background(), toolArgs)
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
	registrar := runtime.NewStandardToolRegistrar(gm.server, deps.Logger)

	runtime.RegisterSimpleTool(registrar, "chat",
		"Interact with the AI assistant for guided containerization workflow",
		func(ctx *gomcpserver.Context, args *ChatArgs) (*ChatResult, error) {
			return gm.handleChat(deps, args)
		})

	return nil
}

// sessionLabelManagerWrapper adapts session.SessionManager to runtime.SessionLabelManager interface
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

func (w *sessionLabelManagerWrapper) GetSession(sessionID string) (sessiontypes.SessionLabelData, error) {
	sessionInterface, err := w.sm.GetSession(sessionID)
	if err != nil {
		return sessiontypes.SessionLabelData{}, err
	}

	session, ok := sessionInterface.(*sessiontypes.SessionState)
	if !ok {
		return sessiontypes.SessionLabelData{}, types.NewErrorBuilder("unexpected_session_type", "Unexpected session type encountered", "session").
			WithOperation("get_session_label_data").
			WithStage("type_detection").
			WithRootCause("Session does not match any known session type patterns").
			WithImmediateStep(1, "Check session", "Verify session is properly initialized and typed").
			WithImmediateStep(2, "Check types", "Ensure session implements expected interface").
			WithImmediateStep(3, "Check version", "Verify session type compatibility with current version").
			Build()
	}

	return sessiontypes.SessionLabelData{
		SessionID: session.SessionID,
		Labels:    session.Labels,
	}, nil
}

func (w *sessionLabelManagerWrapper) ListSessions() []sessiontypes.SessionLabelData {
	summaries := w.sm.ListSessionSummaries()
	result := make([]sessiontypes.SessionLabelData, len(summaries))
	for i, summary := range summaries {
		result[i] = sessiontypes.SessionLabelData{
			SessionID: summary.SessionID,
			Labels:    summary.Labels,
		}
	}
	return result
}

// registerOrchestratorTool creates a GoMCP handler that delegates to the orchestrator
func (gm *GomcpManager) registerOrchestratorTool(registrar *runtime.StandardToolRegistrar, toolName, atomicToolName, description string, deps *ToolDependencies) {
	deps.Logger.Debug().
		Str("tool", toolName).
		Str("atomic_tool", atomicToolName).
		Msg("Registering orchestrator-delegated tool")

	gm.server.Tool(toolName, description, func(ctx *gomcpserver.Context, args interface{}) (interface{}, error) {
		// Execute through the canonical orchestrator - create proper context
		goCtx := context.WithValue(context.Background(), mcpContextKey, ctx)
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
