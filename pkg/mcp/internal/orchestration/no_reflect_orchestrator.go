package orchestration

import (
	"context"
	"fmt"

	"github.com/Azure/container-copilot/pkg/mcp/internal/analyze"
	"github.com/Azure/container-copilot/pkg/mcp/internal/session/session"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// NoReflectToolOrchestrator provides type-safe tool execution without reflection
type NoReflectToolOrchestrator struct {
	toolRegistry       *MCPToolRegistry
	sessionManager     SessionManager
	analyzer           mcptypes.AIAnalyzer
	logger             zerolog.Logger
	toolFactory        *ToolFactory
	pipelineOperations interface{}
}

// NewNoReflectToolOrchestrator creates a new orchestrator without reflection
func NewNoReflectToolOrchestrator(
	toolRegistry *MCPToolRegistry,
	sessionManager SessionManager,
	logger zerolog.Logger,
) *NoReflectToolOrchestrator {
	return &NoReflectToolOrchestrator{
		toolRegistry:   toolRegistry,
		sessionManager: sessionManager,
		logger:         logger.With().Str("component", "no_reflect_orchestrator").Logger(),
	}
}

// SetPipelineAdapter sets the pipeline adapter (deprecated - use SetPipelineOperations)
func (o *NoReflectToolOrchestrator) SetPipelineAdapter(adapter interface{}) {
	o.SetPipelineOperations(adapter)
}

// SetPipelineOperations sets the pipeline operations and creates the tool factory
func (o *NoReflectToolOrchestrator) SetPipelineOperations(operations interface{}) {
	o.pipelineOperations = operations

	// Try to assert to the correct type
	if pipelineOps, ok := operations.(mcptypes.PipelineOperations); ok {
		// Extract concrete session manager from the wrapper
		if concreteSessionManager := o.extractConcreteSessionManager(); concreteSessionManager != nil {
			o.toolFactory = NewToolFactory(pipelineOps, concreteSessionManager, o.analyzer, o.logger)
			o.logger.Debug().Msg("Tool factory successfully initialized with concrete session manager")
		} else {
			o.logger.Warn().Msg("Tool factory initialization requires concrete session manager - factory not created")
		}
	} else {
		o.logger.Error().Msg("Failed to assert pipeline operations to correct type")
	}
}

// extractConcreteSessionManager attempts to extract the concrete session manager
func (o *NoReflectToolOrchestrator) extractConcreteSessionManager() *session.SessionManager {
	// Try to extract from sessionManagerAdapterImpl if it exists
	if adapter, ok := o.sessionManager.(interface {
		GetConcreteSessionManager() *session.SessionManager
	}); ok {
		return adapter.GetConcreteSessionManager()
	}

	// Since the interfaces have different signatures, we cannot directly type assert
	// The orchestration.SessionManager interface is designed to work with interface{}
	// while the concrete session.SessionManager works with typed SessionState
	o.logger.Debug().Msg("Cannot extract concrete session manager due to interface signature mismatch")
	return nil
}

// SetToolFactory sets the tool factory directly (for use when we have concrete types)
func (o *NoReflectToolOrchestrator) SetToolFactory(factory *ToolFactory) {
	o.toolFactory = factory
}

// SetAnalyzer sets the AI analyzer for tool fixing capabilities
func (o *NoReflectToolOrchestrator) SetAnalyzer(analyzer mcptypes.AIAnalyzer) {
	o.analyzer = analyzer
	// If tool factory already exists, recreate it with the analyzer
	if o.toolFactory != nil && o.pipelineOperations != nil {
		if pipelineOps, ok := o.pipelineOperations.(mcptypes.PipelineOperations); ok {
			if concreteSessionManager := o.extractConcreteSessionManager(); concreteSessionManager != nil {
				o.toolFactory = NewToolFactory(pipelineOps, concreteSessionManager, o.analyzer, o.logger)
				o.logger.Debug().Msg("Tool factory recreated with analyzer")
			}
		}
	}
}

// ExecuteTool executes a tool using type-safe dispatch without reflection
func (o *NoReflectToolOrchestrator) ExecuteTool(
	ctx context.Context,
	toolName string,
	args interface{},
	session interface{},
) (interface{}, error) {
	// Get the args map
	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("arguments must be a map[string]interface{}")
	}

	// Type-safe dispatch based on tool name
	switch toolName {
	case "analyze_repository_atomic":
		return o.executeAnalyzeRepository(ctx, argsMap)
	case "build_image_atomic":
		return o.executeBuildImage(ctx, argsMap)
	case "push_image_atomic":
		return o.executePushImage(ctx, argsMap)
	case "pull_image_atomic":
		return o.executePullImage(ctx, argsMap)
	case "tag_image_atomic":
		return o.executeTagImage(ctx, argsMap)
	case "scan_image_security_atomic":
		return o.executeScanImageSecurity(ctx, argsMap)
	case "scan_secrets_atomic":
		return o.executeScanSecrets(ctx, argsMap)
	case "generate_manifests_atomic":
		return o.executeGenerateManifests(ctx, argsMap)
	case "deploy_kubernetes_atomic":
		return o.executeDeployKubernetes(ctx, argsMap)
	case "check_health_atomic":
		return o.executeCheckHealth(ctx, argsMap)
	case "generate_dockerfile":
		return o.executeGenerateDockerfile(ctx, argsMap)
	case "validate_dockerfile_atomic":
		return o.executeValidateDockerfile(ctx, argsMap)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

// ValidateToolArgs validates arguments for a specific tool
func (o *NoReflectToolOrchestrator) ValidateToolArgs(toolName string, args interface{}) error {
	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return fmt.Errorf("arguments must be a map[string]interface{}")
	}

	// Check for session_id (required for all tools)
	if _, exists := argsMap["session_id"]; !exists {
		return fmt.Errorf("session_id is required for tool %s", toolName)
	}

	// Tool-specific validation
	switch toolName {
	case "analyze_repository_atomic":
		if _, exists := argsMap["repo_url"]; !exists {
			return fmt.Errorf("repo_url is required for analyze_repository_atomic")
		}
	case "build_image_atomic":
		if _, exists := argsMap["image_name"]; !exists {
			return fmt.Errorf("image_name is required for build_image_atomic")
		}
	case "push_image_atomic":
		if _, exists := argsMap["image_ref"]; !exists {
			return fmt.Errorf("image_ref is required for push_image_atomic")
		}
	case "pull_image_atomic":
		if _, exists := argsMap["image_ref"]; !exists {
			return fmt.Errorf("image_ref is required for pull_image_atomic")
		}
	case "tag_image_atomic":
		if _, exists := argsMap["image_ref"]; !exists {
			return fmt.Errorf("image_ref is required for tag_image_atomic")
		}
		if _, exists := argsMap["new_tag"]; !exists {
			return fmt.Errorf("new_tag is required for tag_image_atomic")
		}
	case "scan_image_security_atomic":
		if _, exists := argsMap["image_ref"]; !exists {
			return fmt.Errorf("image_ref is required for scan_image_security_atomic")
		}
	case "generate_manifests_atomic":
		if _, exists := argsMap["image_ref"]; !exists {
			return fmt.Errorf("image_ref is required for generate_manifests_atomic")
		}
		if _, exists := argsMap["app_name"]; !exists {
			return fmt.Errorf("app_name is required for generate_manifests_atomic")
		}
	case "deploy_kubernetes_atomic":
		if _, exists := argsMap["manifest_path"]; !exists {
			return fmt.Errorf("manifest_path is required for deploy_kubernetes_atomic")
		}
	}

	return nil
}

// Tool-specific execution methods

func (o *NoReflectToolOrchestrator) executeAnalyzeRepository(ctx context.Context, argsMap map[string]interface{}) (interface{}, error) {
	if o.toolFactory == nil {
		return nil, fmt.Errorf("tool factory not initialized")
	}

	// Create tool instance
	tool := o.toolFactory.CreateAnalyzeRepositoryTool()

	// Build typed arguments
	args := analyze.AtomicAnalyzeRepositoryArgs{}

	// Extract required fields
	if sessionID, ok := getString(argsMap, "session_id"); ok {
		args.SessionID = sessionID
	} else {
		return nil, fmt.Errorf("session_id is required")
	}

	if repoURL, ok := getString(argsMap, "repo_url"); ok {
		args.RepoURL = repoURL
	} else {
		return nil, fmt.Errorf("repo_url is required")
	}

	// Extract optional fields
	if branch, ok := getString(argsMap, "branch"); ok {
		args.Branch = branch
	}

	if context, ok := getString(argsMap, "context"); ok {
		args.Context = context
	}

	if languageHint, ok := getString(argsMap, "language_hint"); ok {
		args.LanguageHint = languageHint
	}

	if shallow, ok := getBool(argsMap, "shallow"); ok {
		args.Shallow = shallow
	}

	// Execute the tool
	return tool.ExecuteRepositoryAnalysis(ctx, args)
}

// Tool execution implementations are in no_reflect_orchestrator_impl.go

// Helper methods for type conversion

func getString(m map[string]interface{}, key string) (string, bool) {
	if v, ok := m[key]; ok {
		if str, ok := v.(string); ok {
			return str, true
		}
	}
	return "", false
}

func getInt(m map[string]interface{}, key string) (int, bool) {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case int:
			return val, true
		case float64:
			return int(val), true
		}
	}
	return 0, false
}

func getBool(m map[string]interface{}, key string) (bool, bool) {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b, true
		}
	}
	return false, false
}
