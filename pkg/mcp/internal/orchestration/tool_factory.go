package orchestration

import (
	"fmt"

	"github.com/Azure/container-copilot/pkg/mcp/internal/store/session"
	"github.com/Azure/container-copilot/pkg/mcp/internal/tools"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// ToolFactory creates tool instances with proper dependencies
type ToolFactory struct {
	pipelineOperations mcptypes.PipelineOperations
	sessionManager     *session.SessionManager
	logger             zerolog.Logger
}

// NewToolFactory creates a new tool factory
func NewToolFactory(
	pipelineOperations mcptypes.PipelineOperations,
	sessionManager *session.SessionManager,
	logger zerolog.Logger,
) *ToolFactory {
	return &ToolFactory{
		pipelineOperations: pipelineOperations,
		sessionManager:     sessionManager,
		logger:             logger,
	}
}

// CreateAnalyzeRepositoryTool creates an instance of AtomicAnalyzeRepositoryTool
func (f *ToolFactory) CreateAnalyzeRepositoryTool() *tools.AtomicAnalyzeRepositoryTool {
	return tools.NewAtomicAnalyzeRepositoryTool(f.pipelineOperations, f.sessionManager, f.logger)
}

// CreateBuildImageTool creates an instance of AtomicBuildImageTool
func (f *ToolFactory) CreateBuildImageTool() *tools.AtomicBuildImageTool {
	return tools.NewAtomicBuildImageTool(f.pipelineOperations, f.sessionManager, f.logger)
}

// CreatePushImageTool creates an instance of AtomicPushImageTool
func (f *ToolFactory) CreatePushImageTool() *tools.AtomicPushImageTool {
	return tools.NewAtomicPushImageTool(f.pipelineOperations, f.sessionManager, f.logger)
}

// CreatePullImageTool creates an instance of AtomicPullImageTool
func (f *ToolFactory) CreatePullImageTool() *tools.AtomicPullImageTool {
	return tools.NewAtomicPullImageTool(f.pipelineOperations, f.sessionManager, f.logger)
}

// CreateTagImageTool creates an instance of AtomicTagImageTool
func (f *ToolFactory) CreateTagImageTool() *tools.AtomicTagImageTool {
	return tools.NewAtomicTagImageTool(f.pipelineOperations, f.sessionManager, f.logger)
}

// CreateScanImageSecurityTool creates an instance of AtomicScanImageSecurityTool
func (f *ToolFactory) CreateScanImageSecurityTool() *tools.AtomicScanImageSecurityTool {
	return tools.NewAtomicScanImageSecurityTool(f.pipelineOperations, f.sessionManager, f.logger)
}

// CreateScanSecretsTool creates an instance of AtomicScanSecretsTool
func (f *ToolFactory) CreateScanSecretsTool() *tools.AtomicScanSecretsTool {
	return tools.NewAtomicScanSecretsTool(f.pipelineOperations, f.sessionManager, f.logger)
}

// CreateGenerateManifestsTool creates an instance of AtomicGenerateManifestsTool
func (f *ToolFactory) CreateGenerateManifestsTool() *tools.AtomicGenerateManifestsTool {
	return tools.NewAtomicGenerateManifestsTool(f.pipelineOperations, f.sessionManager, f.logger)
}

// CreateDeployKubernetesTool creates an instance of AtomicDeployKubernetesTool
func (f *ToolFactory) CreateDeployKubernetesTool() *tools.AtomicDeployKubernetesTool {
	return tools.NewAtomicDeployKubernetesTool(f.pipelineOperations, f.sessionManager, f.logger)
}

// CreateCheckHealthTool creates an instance of AtomicCheckHealthTool
func (f *ToolFactory) CreateCheckHealthTool() *tools.AtomicCheckHealthTool {
	return tools.NewAtomicCheckHealthTool(f.pipelineOperations, f.sessionManager, f.logger)
}

// CreateGenerateDockerfileTool creates an instance of GenerateDockerfileTool
func (f *ToolFactory) CreateGenerateDockerfileTool() *tools.GenerateDockerfileTool {
	return tools.NewGenerateDockerfileTool(f.sessionManager, f.logger)
}

// CreateValidateDockerfileTool creates an instance of AtomicValidateDockerfileTool
func (f *ToolFactory) CreateValidateDockerfileTool() *tools.AtomicValidateDockerfileTool {
	return tools.NewAtomicValidateDockerfileTool(f.pipelineOperations, f.sessionManager, f.logger)
}

// CreateTool creates a tool by name
func (f *ToolFactory) CreateTool(toolName string) (interface{}, error) {
	switch toolName {
	case "analyze_repository_atomic":
		return f.CreateAnalyzeRepositoryTool(), nil
	case "build_image_atomic":
		return f.CreateBuildImageTool(), nil
	case "push_image_atomic":
		return f.CreatePushImageTool(), nil
	case "pull_image_atomic":
		return f.CreatePullImageTool(), nil
	case "tag_image_atomic":
		return f.CreateTagImageTool(), nil
	case "scan_image_security_atomic":
		return f.CreateScanImageSecurityTool(), nil
	case "scan_secrets_atomic":
		return f.CreateScanSecretsTool(), nil
	case "generate_manifests_atomic":
		return f.CreateGenerateManifestsTool(), nil
	case "deploy_kubernetes_atomic":
		return f.CreateDeployKubernetesTool(), nil
	case "check_health_atomic":
		return f.CreateCheckHealthTool(), nil
	case "generate_dockerfile":
		return f.CreateGenerateDockerfileTool(), nil
	case "validate_dockerfile_atomic":
		return f.CreateValidateDockerfileTool(), nil
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}
