package orchestration

import (
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/core"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/core/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/analyze"
	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	"github.com/Azure/container-kit/pkg/mcp/internal/deploy"
	"github.com/Azure/container-kit/pkg/mcp/internal/scan"
	"github.com/rs/zerolog"
)

// ToolDependencies holds dependencies needed to create tools
type ToolDependencies struct {
	pipelineOperations mcptypes.PipelineOperations
	sessionManager     *session.SessionManager
	analyzer           core.AIAnalyzer
	analyzerHelper     *AnalyzerHelper
	logger             zerolog.Logger
}

// NewToolDependencies creates a new tool dependencies container
func NewToolDependencies(
	pipelineOperations mcptypes.PipelineOperations,
	sessionManager *session.SessionManager,
	analyzer core.AIAnalyzer,
	logger zerolog.Logger,
) *ToolDependencies {
	deps := &ToolDependencies{
		pipelineOperations: pipelineOperations,
		sessionManager:     sessionManager,
		analyzer:           analyzer,
		logger:             logger,
	}

	// Create analyzer helper with dependencies for repository adapter
	deps.analyzerHelper = NewAnalyzerHelperWithDependencies(analyzer, deps, sessionManager, logger)

	return deps
}

// CreateAnalyzeRepositoryTool creates an instance of AtomicAnalyzeRepositoryTool
func CreateAnalyzeRepositoryTool(deps *ToolDependencies) *analyze.AtomicAnalyzeRepositoryTool {
	return analyze.NewAtomicAnalyzeRepositoryTool(deps.pipelineOperations, deps.sessionManager, deps.logger)
}

// CreateBuildImageTool creates an instance of AtomicBuildImageTool
func CreateBuildImageTool(deps *ToolDependencies) *build.AtomicBuildImageTool {
	tool := build.NewAtomicBuildImageTool(deps.pipelineOperations, deps.sessionManager, deps.logger)
	if deps.analyzer != nil {
		// Get enhanced analyzer for build integration
		enhancedAnalyzer := deps.analyzerHelper.GetEnhancedBuildAnalyzer()
		tool.SetAnalyzer(enhancedAnalyzer)
	}
	return tool
}

// CreatePushImageTool creates an instance of AtomicPushImageTool
func CreatePushImageTool(deps *ToolDependencies) *build.AtomicPushImageTool {
	tool := build.NewAtomicPushImageTool(deps.pipelineOperations, deps.sessionManager, deps.logger)
	initializer := NewBuildToolInitializer(deps.analyzerHelper)
	initializer.SetupAnalyzer(tool, "push_image")
	return tool
}

// CreatePullImageTool creates an instance of AtomicPullImageTool
func CreatePullImageTool(deps *ToolDependencies) *build.AtomicPullImageTool {
	tool := build.NewAtomicPullImageTool(deps.pipelineOperations, deps.sessionManager, deps.logger)
	initializer := NewBuildToolInitializer(deps.analyzerHelper)
	initializer.SetupAnalyzer(tool, "pull_image")
	return tool
}

// CreateTagImageTool creates an instance of AtomicTagImageTool
func CreateTagImageTool(deps *ToolDependencies) *build.AtomicTagImageTool {
	tool := build.NewAtomicTagImageTool(deps.pipelineOperations, deps.sessionManager, deps.logger)
	initializer := NewBuildToolInitializer(deps.analyzerHelper)
	initializer.SetupAnalyzer(tool, "tag_image")
	return tool
}

// CreateScanImageSecurityTool creates an instance of AtomicScanImageSecurityTool
func CreateScanImageSecurityTool(deps *ToolDependencies) *scan.AtomicScanImageSecurityTool {
	tool := scan.NewAtomicScanImageSecurityTool(deps.pipelineOperations, deps.sessionManager, deps.logger)
	initializer := NewScanToolInitializer(deps.analyzerHelper)
	initializer.SetupAnalyzer(tool, "scan_image_security")
	return tool
}

// CreateScanSecretsTool creates an instance of AtomicScanSecretsTool
func CreateScanSecretsTool(deps *ToolDependencies) *scan.AtomicScanSecretsTool {
	tool := scan.NewAtomicScanSecretsTool(deps.pipelineOperations, deps.sessionManager, deps.logger)
	initializer := NewScanToolInitializer(deps.analyzerHelper)
	initializer.SetupAnalyzer(tool, "scan_secrets")
	return tool
}

// CreateGenerateManifestsTool creates an instance of AtomicGenerateManifestsTool
func CreateGenerateManifestsTool(deps *ToolDependencies) *deploy.AtomicGenerateManifestsTool {
	tool := deploy.NewAtomicGenerateManifestsTool(deps.pipelineOperations, deps.sessionManager, deps.logger)
	initializer := NewDeployToolInitializer(deps.analyzerHelper)
	initializer.SetupAnalyzer(tool, "generate_manifests")
	return tool
}

// CreateDeployKubernetesTool creates an instance of AtomicDeployKubernetesTool
func CreateDeployKubernetesTool(deps *ToolDependencies) *deploy.AtomicDeployKubernetesTool {
	tool := deploy.NewAtomicDeployKubernetesTool(deps.pipelineOperations, deps.sessionManager, deps.logger)
	initializer := NewDeployToolInitializer(deps.analyzerHelper)
	initializer.SetupAnalyzer(tool, "deploy_kubernetes")
	return tool
}

// CreateCheckHealthTool creates an instance of AtomicCheckHealthTool
func CreateCheckHealthTool(deps *ToolDependencies) *deploy.AtomicCheckHealthTool {
	tool := deploy.NewAtomicCheckHealthTool(deps.pipelineOperations, deps.sessionManager, deps.logger)
	initializer := NewDeployToolInitializer(deps.analyzerHelper)
	initializer.SetupAnalyzer(tool, "check_health")
	return tool
}

// CreateGenerateDockerfileTool creates an instance of AtomicGenerateDockerfileTool
func CreateGenerateDockerfileTool(deps *ToolDependencies) *analyze.AtomicGenerateDockerfileTool {
	return analyze.NewAtomicGenerateDockerfileTool(deps.sessionManager, deps.logger)
}

// CreateValidateDockerfileTool creates an instance of AtomicValidateDockerfileTool
func CreateValidateDockerfileTool(deps *ToolDependencies) *analyze.AtomicValidateDockerfileTool {
	tool := analyze.NewAtomicValidateDockerfileTool(deps.pipelineOperations, deps.sessionManager, deps.logger)
	if deps.analyzer != nil {
		// Create a default analyzer for analyze tools
		analyzer := deps.analyzerHelper.SetupAnalyzeToolAnalyzer("validate_dockerfile")
		if analyzer != nil {
			tool.SetAnalyzer(analyzer)
		}
	}
	return tool
}

// CreateTool creates a tool by name
func CreateTool(deps *ToolDependencies, toolName string) (interface{}, error) {
	switch toolName {
	case "analyze_repository":
		return CreateAnalyzeRepositoryTool(deps), nil
	case "build_image":
		return CreateBuildImageTool(deps), nil
	case "push_image":
		return CreatePushImageTool(deps), nil
	case "pull_image":
		return CreatePullImageTool(deps), nil
	case "tag_image":
		return CreateTagImageTool(deps), nil
	case "scan_image_security":
		return CreateScanImageSecurityTool(deps), nil
	case "scan_secrets":
		return CreateScanSecretsTool(deps), nil
	case "generate_manifests":
		return CreateGenerateManifestsTool(deps), nil
	case "deploy_kubernetes":
		return CreateDeployKubernetesTool(deps), nil
	case "check_health":
		return CreateCheckHealthTool(deps), nil
	case "generate_dockerfile":
		return CreateGenerateDockerfileTool(deps), nil
	case "validate_dockerfile":
		return CreateValidateDockerfileTool(deps), nil
	case "validate_deployment":
		return CreateValidateDeploymentTool(deps), nil
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

// CreateValidateDeploymentTool creates an instance of AtomicValidateDeploymentTool
func CreateValidateDeploymentTool(deps *ToolDependencies) *deploy.AtomicValidateDeploymentTool {
	tool := deploy.NewAtomicValidateDeploymentTool(deps.logger, "", nil, nil)
	initializer := NewDeployToolInitializer(deps.analyzerHelper)
	initializer.SetupAnalyzer(tool, "validate_deployment")
	return tool
}

// GetEnhancedBuildAnalyzer returns the enhanced build analyzer instance
func GetEnhancedBuildAnalyzer(deps *ToolDependencies) *build.EnhancedBuildAnalyzer {
	return deps.analyzerHelper.GetEnhancedBuildAnalyzer()
}
