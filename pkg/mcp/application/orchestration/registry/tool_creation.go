package registry

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/core"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/application/orchestration/execution"
	"github.com/Azure/container-kit/pkg/mcp/domain/containerization/analyze"
	"github.com/Azure/container-kit/pkg/mcp/domain/containerization/build"
	"github.com/Azure/container-kit/pkg/mcp/domain/containerization/deploy"
	"github.com/Azure/container-kit/pkg/mcp/domain/containerization/scan"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/services"
)

// ToolDependencies holds dependencies needed to create tools
type ToolDependencies struct {
	pipelineOperations mcptypes.TypedPipelineOperations
	serviceContainer   services.ServiceContainer
	analyzer           core.AIAnalyzer
	analyzerHelper     *execution.AnalyzerHelperFactory
	logger             *slog.Logger
}

// NewToolDependencies creates a new tool dependencies container using service container
func NewToolDependencies(
	pipelineOperations mcptypes.TypedPipelineOperations,
	serviceContainer services.ServiceContainer,
	analyzer core.AIAnalyzer,
	factory ToolFactory,
	logger *slog.Logger,
) *ToolDependencies {
	deps := &ToolDependencies{
		pipelineOperations: pipelineOperations,
		serviceContainer:   serviceContainer,
		analyzer:           analyzer,
		logger:             logger,
	}

	// TODO: Update when execution package is migrated to slog
	// deps.analyzerHelper = execution.NewAnalyzerHelperFactory(analyzer, factory, logger)

	return deps
}

// CreateAnalyzeRepositoryTool creates an instance of AtomicAnalyzeRepositoryTool
func CreateAnalyzeRepositoryTool(deps *ToolDependencies) *analyze.AtomicAnalyzeRepositoryTool {
	return analyze.NewAtomicAnalyzeRepositoryToolWithServices(deps.pipelineOperations, deps.serviceContainer, deps.logger)
}

// CreateBuildImageTool creates an instance of AtomicBuildImageTool
func CreateBuildImageTool(deps *ToolDependencies) *build.AtomicBuildImageTool {
	return build.NewAtomicBuildImageToolWithServices(deps.pipelineOperations, deps.serviceContainer, deps.logger)
}

// CreatePushImageTool creates an instance of AtomicPushImageTool
func CreatePushImageTool(deps *ToolDependencies) *build.AtomicPushImageTool {
	return build.NewAtomicPushImageToolWithServices(deps.pipelineOperations, deps.serviceContainer, deps.logger)
}

// CreatePullImageTool creates an instance of AtomicPullImageTool
func CreatePullImageTool(deps *ToolDependencies) *build.AtomicPullImageTool {
	return build.NewAtomicPullImageToolWithServices(deps.pipelineOperations, deps.serviceContainer, deps.logger)
}

// CreateTagImageTool creates an instance of AtomicTagImageTool
func CreateTagImageTool(deps *ToolDependencies) *build.AtomicTagImageTool {
	return build.NewAtomicTagImageToolWithServices(deps.pipelineOperations, deps.serviceContainer, deps.logger)
}

// CreateScanImageSecurityTool creates an instance of AtomicScanImageSecurityTool
func CreateScanImageSecurityTool(deps *ToolDependencies) *scan.AtomicScanImageSecurityTool {
	return scan.NewAtomicScanImageSecurityTool(deps.pipelineOperations, deps.serviceContainer, deps.logger)
}

// CreateScanSecretsTool creates an instance of AtomicScanSecretsTool
func CreateScanSecretsTool(deps *ToolDependencies) *scan.AtomicScanSecretsTool {
	return scan.NewAtomicScanSecretsTool(deps.pipelineOperations, deps.serviceContainer, deps.logger)
}

// CreateGenerateManifestsTool creates an instance of AtomicGenerateManifestsTool
func CreateGenerateManifestsTool(deps *ToolDependencies) *deploy.AtomicGenerateManifestsTool {
	return deploy.NewAtomicGenerateManifestsToolWithServices(deps.pipelineOperations, deps.serviceContainer, deps.logger)
}

// CreateDeployKubernetesTool creates an instance of AtomicDeployKubernetesTool
func CreateDeployKubernetesTool(deps *ToolDependencies) *deploy.AtomicDeployKubernetesTool {
	return deploy.NewAtomicDeployKubernetesToolWithServices(deps.pipelineOperations, deps.serviceContainer, deps.logger)
}

// CreateCheckHealthTool creates an instance of AtomicCheckHealthTool
func CreateCheckHealthTool(deps *ToolDependencies) *deploy.AtomicCheckHealthTool {
	return deploy.NewAtomicCheckHealthToolWithServices(deps.pipelineOperations, deps.serviceContainer, deps.logger)
}

// CreateGenerateDockerfileTool creates an instance of AtomicGenerateDockerfileTool
func CreateGenerateDockerfileTool(deps *ToolDependencies) *analyze.AtomicGenerateDockerfileTool {
	return analyze.NewAtomicGenerateDockerfileToolWithServices(deps.serviceContainer, deps.logger)
}

// CreateValidateDockerfileTool creates an instance of AtomicValidateDockerfileTool
func CreateValidateDockerfileTool(deps *ToolDependencies) *analyze.AtomicValidateDockerfileTool {
	return analyze.NewAtomicValidateDockerfileToolWithServices(deps.pipelineOperations, deps.serviceContainer, deps.logger)
}

// CreateValidateDeploymentTool creates an instance of AtomicValidateDeploymentTool
func CreateValidateDeploymentTool(deps *ToolDependencies) *deploy.AtomicValidateDeploymentTool {
	return deploy.NewAtomicValidateDeploymentTool(deps.logger, "", nil, nil)
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
		return nil, errors.NewError().Messagef("unknown tool: %s", toolName).Build()
	}
}

// GetEnhancedBuildAnalyzer returns the enhanced build analyzer instance
func GetEnhancedBuildAnalyzer(deps *ToolDependencies) interface{} {
	if deps.analyzerHelper != nil {
		return deps.analyzerHelper.GetEnhancedBuildAnalyzer()
	}
	return nil
}
