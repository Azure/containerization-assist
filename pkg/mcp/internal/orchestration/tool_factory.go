package orchestration

import (
	"fmt"

	"github.com/Azure/container-copilot/pkg/mcp/internal/analyze"
	"github.com/Azure/container-copilot/pkg/mcp/internal/build"
	"github.com/Azure/container-copilot/pkg/mcp/internal/deploy"
	"github.com/Azure/container-copilot/pkg/mcp/internal/scan"
	"github.com/Azure/container-copilot/pkg/mcp/internal/session/session"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// ToolFactory creates tool instances with proper dependencies
type ToolFactory struct {
	pipelineOperations mcptypes.PipelineOperations
	sessionManager     *session.SessionManager
	analyzer           mcptypes.AIAnalyzer
	logger             zerolog.Logger
}

// NewToolFactory creates a new tool factory
func NewToolFactory(
	pipelineOperations mcptypes.PipelineOperations,
	sessionManager *session.SessionManager,
	analyzer mcptypes.AIAnalyzer,
	logger zerolog.Logger,
) *ToolFactory {
	return &ToolFactory{
		pipelineOperations: pipelineOperations,
		sessionManager:     sessionManager,
		analyzer:           analyzer,
		logger:             logger,
	}
}

// CreateAnalyzeRepositoryTool creates an instance of AtomicAnalyzeRepositoryTool
func (f *ToolFactory) CreateAnalyzeRepositoryTool() *analyze.AtomicAnalyzeRepositoryTool {
	return analyze.NewAtomicAnalyzeRepositoryTool(f.pipelineOperations, f.sessionManager, f.logger)
}

// CreateBuildImageTool creates an instance of AtomicBuildImageTool
func (f *ToolFactory) CreateBuildImageTool() *build.AtomicBuildImageTool {
	tool := build.NewAtomicBuildImageTool(f.pipelineOperations, f.sessionManager, f.logger)
	if f.analyzer != nil {
		tool.SetAnalyzer(f.analyzer)
	}
	return tool
}

// CreatePushImageTool creates an instance of AtomicPushImageTool
func (f *ToolFactory) CreatePushImageTool() *build.AtomicPushImageTool {
	return build.NewAtomicPushImageTool(f.pipelineOperations, f.sessionManager, f.logger)
}

// CreatePullImageTool creates an instance of AtomicPullImageTool
func (f *ToolFactory) CreatePullImageTool() *build.AtomicPullImageTool {
	return build.NewAtomicPullImageTool(f.pipelineOperations, f.sessionManager, f.logger)
}

// CreateTagImageTool creates an instance of AtomicTagImageTool
func (f *ToolFactory) CreateTagImageTool() *build.AtomicTagImageTool {
	return build.NewAtomicTagImageTool(f.pipelineOperations, f.sessionManager, f.logger)
}

// CreateScanImageSecurityTool creates an instance of AtomicScanImageSecurityTool
func (f *ToolFactory) CreateScanImageSecurityTool() *scan.AtomicScanImageSecurityTool {
	tool := scan.NewAtomicScanImageSecurityTool(f.pipelineOperations, f.sessionManager, f.logger)
	if f.analyzer != nil {
		tool.SetAnalyzer(f.analyzer)
	}
	return tool
}

// CreateScanSecretsTool creates an instance of AtomicScanSecretsTool
func (f *ToolFactory) CreateScanSecretsTool() *scan.AtomicScanSecretsTool {
	return scan.NewAtomicScanSecretsTool(f.pipelineOperations, f.sessionManager, f.logger)
}

// CreateGenerateManifestsTool creates an instance of AtomicGenerateManifestsTool
func (f *ToolFactory) CreateGenerateManifestsTool() *deploy.AtomicGenerateManifestsTool {
	tool := deploy.NewAtomicGenerateManifestsTool(f.pipelineOperations, f.sessionManager, f.logger)
	if f.analyzer != nil {
		tool.SetAnalyzer(f.analyzer)
	}
	return tool
}

// CreateDeployKubernetesTool creates an instance of AtomicDeployKubernetesTool
func (f *ToolFactory) CreateDeployKubernetesTool() *deploy.AtomicDeployKubernetesTool {
	tool := deploy.NewAtomicDeployKubernetesTool(f.pipelineOperations, f.sessionManager, f.logger)
	if f.analyzer != nil {
		tool.SetAnalyzer(f.analyzer)
	}
	return tool
}

// CreateCheckHealthTool creates an instance of AtomicCheckHealthTool
func (f *ToolFactory) CreateCheckHealthTool() *deploy.AtomicCheckHealthTool {
	return deploy.NewAtomicCheckHealthTool(f.pipelineOperations, f.sessionManager, f.logger)
}

// CreateGenerateDockerfileTool creates an instance of GenerateDockerfileTool
func (f *ToolFactory) CreateGenerateDockerfileTool() *analyze.GenerateDockerfileTool {
	return analyze.NewGenerateDockerfileTool(f.sessionManager, f.logger)
}

// CreateValidateDockerfileTool creates an instance of AtomicValidateDockerfileTool
func (f *ToolFactory) CreateValidateDockerfileTool() *analyze.AtomicValidateDockerfileTool {
	tool := analyze.NewAtomicValidateDockerfileTool(f.pipelineOperations, f.sessionManager, f.logger)
	if f.analyzer != nil {
		tool.SetAnalyzer(f.analyzer)
	}
	return tool
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
