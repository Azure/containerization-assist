package orchestration

import (
	"context"
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// Local type definitions to avoid import cycles

// AtomicAnalyzeRepositoryArgs defines arguments for atomic repository analysis
// This is a local copy to avoid importing the analyze package which creates cycles
type AtomicAnalyzeRepositoryArgs struct {
	types.BaseToolArgs
	RepoURL      string `json:"repo_url" description:"Repository URL (GitHub, GitLab, etc.) or local path"`
	Branch       string `json:"branch,omitempty" description:"Git branch to analyze (default: main)"`
	Context      string `json:"context,omitempty" description:"Additional context about the application"`
	LanguageHint string `json:"language_hint,omitempty" description:"Primary programming language hint"`
	Shallow      bool   `json:"shallow,omitempty" description:"Perform shallow clone for faster analysis"`
}

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

// SetPipelineOperations sets the pipeline operations and creates the tool factory
func (o *NoReflectToolOrchestrator) SetPipelineOperations(operations interface{}) {
	o.pipelineOperations = operations

	// Try to assert to the correct type
	if _, ok := operations.(mcptypes.PipelineOperations); ok {
		// Skip tool factory creation due to import cycle prevention
		// The extractConcreteSessionManager returns nil to avoid import cycles
		o.logger.Warn().Msg("Tool factory creation disabled to prevent import cycles - use SetToolFactory directly")
	} else {
		o.logger.Error().Msg("Failed to assert pipeline operations to correct type")
	}
}

// extractConcreteSessionManager attempts to extract the concrete session manager
// NOTE: This function is disabled to avoid import cycles. The tool factory
// creation is skipped when concrete session manager cannot be extracted.
func (o *NoReflectToolOrchestrator) extractConcreteSessionManager() interface{} {
	// Import cycle prevention: cannot import session.SessionManager directly
	// The orchestration.SessionManager interface works with interface{} types
	// while ToolFactory requires concrete session.SessionManager types
	o.logger.Debug().Msg("Concrete session manager extraction disabled to prevent import cycles")
	return nil
}

// SetToolFactory sets the tool factory directly (for use when we have concrete types)
func (o *NoReflectToolOrchestrator) SetToolFactory(factory *ToolFactory) {
	o.toolFactory = factory
}

// SetAnalyzer sets the AI analyzer for tool fixing capabilities
func (o *NoReflectToolOrchestrator) SetAnalyzer(analyzer mcptypes.AIAnalyzer) {
	o.analyzer = analyzer
	// Tool factory recreation disabled due to import cycle prevention
	o.logger.Debug().Msg("Tool factory recreation disabled - analyzer set for future factory creation")
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
		return nil, types.NewRichError("INVALID_ARGUMENTS_TYPE", "arguments must be a map[string]interface{}", "validation_error")
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
		return nil, types.NewRichError("UNKNOWN_TOOL", fmt.Sprintf("unknown tool: %s", toolName), "tool_error")
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
		return types.NewRichError("SESSION_ID_REQUIRED", fmt.Sprintf("session_id is required for tool %s", toolName), "validation_error")
	}

	// Tool-specific validation
	switch toolName {
	case "analyze_repository_atomic":
		if _, exists := argsMap["repo_url"]; !exists {
			return types.NewRichError("REPO_URL_REQUIRED", "repo_url is required for analyze_repository_atomic", "validation_error")
		}
	case "build_image_atomic":
		if _, exists := argsMap["image_name"]; !exists {
			return types.NewRichError("IMAGE_NAME_REQUIRED", "image_name is required for build_image_atomic", "validation_error")
		}
	case "push_image_atomic":
		if _, exists := argsMap["image_ref"]; !exists {
			return types.NewRichError("IMAGE_REF_REQUIRED", "image_ref is required for push_image_atomic", "validation_error")
		}
	case "pull_image_atomic":
		if _, exists := argsMap["image_ref"]; !exists {
			return types.NewRichError("IMAGE_REF_REQUIRED", "image_ref is required for pull_image_atomic", "validation_error")
		}
	case "tag_image_atomic":
		if _, exists := argsMap["image_ref"]; !exists {
			return types.NewRichError("IMAGE_REF_REQUIRED", "image_ref is required for tag_image_atomic", "validation_error")
		}
		if _, exists := argsMap["new_tag"]; !exists {
			return types.NewRichError("NEW_TAG_REQUIRED", "new_tag is required for tag_image_atomic", "validation_error")
		}
	case "scan_image_security_atomic":
		if _, exists := argsMap["image_ref"]; !exists {
			return types.NewRichError("IMAGE_REF_REQUIRED", "image_ref is required for scan_image_security_atomic", "validation_error")
		}
	case "generate_manifests_atomic":
		if _, exists := argsMap["image_ref"]; !exists {
			return types.NewRichError("IMAGE_REF_REQUIRED", "image_ref is required for generate_manifests_atomic", "validation_error")
		}
		if _, exists := argsMap["app_name"]; !exists {
			return types.NewRichError("APP_NAME_REQUIRED", "app_name is required for generate_manifests_atomic", "validation_error")
		}
	case "deploy_kubernetes_atomic":
		if _, exists := argsMap["manifest_path"]; !exists {
			return types.NewRichError("MANIFEST_PATH_REQUIRED", "manifest_path is required for deploy_kubernetes_atomic", "validation_error")
		}
	}

	return nil
}

// Tool-specific execution methods

func (o *NoReflectToolOrchestrator) executeAnalyzeRepository(ctx context.Context, argsMap map[string]interface{}) (interface{}, error) {
	if o.toolFactory == nil {
		return nil, types.NewRichError("TOOL_FACTORY_NOT_INITIALIZED", "tool factory not initialized", "configuration_error")
	}

	// Convert args to typed struct
	sessionID, _ := getString(argsMap, "session_id")
	repoURL, _ := getString(argsMap, "repo_url")
	branch, _ := getString(argsMap, "branch")
	context, _ := getString(argsMap, "context")
	languageHint, _ := getString(argsMap, "language_hint")
	shallow, _ := getBool(argsMap, "shallow")

	args := &AtomicAnalyzeRepositoryArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: sessionID,
		},
		RepoURL:      repoURL,
		Branch:       branch,
		Context:      context,
		LanguageHint: languageHint,
		Shallow:      shallow,
	}

	// Create and execute the tool
	tool := o.toolFactory.CreateAnalyzeRepositoryTool()
	return tool.Execute(ctx, args)
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
