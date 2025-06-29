package orchestration

import (
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp"
	mcptypes "github.com/Azure/container-kit/pkg/mcp"
	"github.com/Azure/container-kit/pkg/mcp/internal/analyze"
	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	"github.com/Azure/container-kit/pkg/mcp/internal/deploy"
	"github.com/Azure/container-kit/pkg/mcp/internal/scan"
	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/rs/zerolog"
)

// ToolFactory creates tool instances with proper dependencies
type ToolFactory struct {
	pipelineOperations mcptypes.PipelineOperations
	sessionManager     *session.SessionManager
	analyzer           mcp.AIAnalyzer
	analyzerHelper     *AnalyzerHelper
	logger             zerolog.Logger
}

// NewToolFactory creates a new tool factory
func NewToolFactory(
	pipelineOperations mcptypes.PipelineOperations,
	sessionManager *session.SessionManager,
	analyzer mcp.AIAnalyzer,
	logger zerolog.Logger,
) *ToolFactory {
	factory := &ToolFactory{
		pipelineOperations: pipelineOperations,
		sessionManager:     sessionManager,
		analyzer:           analyzer,
		logger:             logger,
	}

	// Create analyzer helper with factory support for repository adapter
	factory.analyzerHelper = NewAnalyzerHelperWithFactory(analyzer, factory, sessionManager, logger)

	return factory
}

// CreateAnalyzeRepositoryTool creates an instance of AtomicAnalyzeRepositoryTool
func (f *ToolFactory) CreateAnalyzeRepositoryTool() *analyze.AtomicAnalyzeRepositoryTool {
	return analyze.NewAtomicAnalyzeRepositoryTool(f.pipelineOperations, f.sessionManager, f.logger)
}

// CreateBuildImageTool creates an instance of AtomicBuildImageTool
func (f *ToolFactory) CreateBuildImageTool() *build.AtomicBuildImageTool {
	tool := build.NewAtomicBuildImageTool(f.pipelineOperations, f.sessionManager, f.logger)
	if f.analyzer != nil {
		// Get enhanced analyzer for build integration
		enhancedAnalyzer := f.analyzerHelper.GetEnhancedBuildAnalyzer()
		tool.SetAnalyzer(enhancedAnalyzer)
	}
	return tool
}

// CreatePushImageTool creates an instance of AtomicPushImageTool
func (f *ToolFactory) CreatePushImageTool() *build.AtomicPushImageTool {
	tool := build.NewAtomicPushImageTool(f.pipelineOperations, f.sessionManager, f.logger)
	initializer := NewBuildToolInitializer(f.analyzerHelper)
	initializer.SetupAnalyzer(tool, "push_image")
	return tool
}

// CreatePullImageTool creates an instance of AtomicPullImageTool
func (f *ToolFactory) CreatePullImageTool() *build.AtomicPullImageTool {
	tool := build.NewAtomicPullImageTool(f.pipelineOperations, f.sessionManager, f.logger)
	initializer := NewBuildToolInitializer(f.analyzerHelper)
	initializer.SetupAnalyzer(tool, "pull_image")
	return tool
}

// CreateTagImageTool creates an instance of AtomicTagImageTool
func (f *ToolFactory) CreateTagImageTool() *build.AtomicTagImageTool {
	tool := build.NewAtomicTagImageTool(f.pipelineOperations, f.sessionManager, f.logger)
	initializer := NewBuildToolInitializer(f.analyzerHelper)
	initializer.SetupAnalyzer(tool, "tag_image")
	return tool
}

// CreateScanImageSecurityTool creates an instance of AtomicScanImageSecurityTool
func (f *ToolFactory) CreateScanImageSecurityTool() *scan.AtomicScanImageSecurityTool {
	tool := scan.NewAtomicScanImageSecurityTool(f.pipelineOperations, f.sessionManager, f.logger)
	// Note: Scan tools may need different analyzer interface
	// TODO: Implement proper scan analyzer when scan integration is completed
	return tool
}

// CreateScanSecretsTool creates an instance of AtomicScanSecretsTool
func (f *ToolFactory) CreateScanSecretsTool() *scan.AtomicScanSecretsTool {
	return scan.NewAtomicScanSecretsTool(f.pipelineOperations, f.sessionManager, f.logger)
}

// CreateGenerateManifestsTool creates an instance of AtomicGenerateManifestsTool
func (f *ToolFactory) CreateGenerateManifestsTool() *deploy.AtomicGenerateManifestsTool {
	tool := deploy.NewAtomicGenerateManifestsTool(f.pipelineOperations, f.sessionManager, f.logger)
	// Note: Deploy tools may need different analyzer interface
	// TODO: Implement proper deploy analyzer when deploy integration is completed
	return tool
}

// CreateDeployKubernetesTool creates an instance of AtomicDeployKubernetesTool
func (f *ToolFactory) CreateDeployKubernetesTool() *deploy.AtomicDeployKubernetesTool {
	tool := deploy.NewAtomicDeployKubernetesTool(f.pipelineOperations, f.sessionManager, f.logger)
	// Note: Deploy tools may need different analyzer interface
	// TODO: Implement proper deploy analyzer when deploy integration is completed
	return tool
}

// CreateCheckHealthTool creates an instance of AtomicCheckHealthTool
func (f *ToolFactory) CreateCheckHealthTool() *deploy.AtomicCheckHealthTool {
	tool := deploy.NewAtomicCheckHealthTool(f.pipelineOperations, f.sessionManager, f.logger)
	initializer := NewDeployToolInitializer(f.analyzerHelper)
	initializer.SetupAnalyzer(tool, "check_health")
	return tool
}

// CreateGenerateDockerfileTool creates an instance of AtomicGenerateDockerfileTool
func (f *ToolFactory) CreateGenerateDockerfileTool() *analyze.AtomicGenerateDockerfileTool {
	return analyze.NewAtomicGenerateDockerfileTool(f.sessionManager, f.logger)
}

// CreateValidateDockerfileTool creates an instance of AtomicValidateDockerfileTool
func (f *ToolFactory) CreateValidateDockerfileTool() *analyze.AtomicValidateDockerfileTool {
	tool := analyze.NewAtomicValidateDockerfileTool(f.pipelineOperations, f.sessionManager, f.logger)
	if f.analyzer != nil {
		// Create a default analyzer for analyze tools
		analyzer := f.analyzerHelper.SetupAnalyzeToolAnalyzer("validate_dockerfile")
		if analyzer != nil {
			tool.SetAnalyzer(analyzer)
		}
	}
	return tool
}

// CreateTool creates a tool by name
func (f *ToolFactory) CreateTool(toolName string) (interface{}, error) {
	switch toolName {
	case "analyze_repository":
		return f.CreateAnalyzeRepositoryTool(), nil
	case "build_image":
		return f.CreateBuildImageTool(), nil
	case "push_image":
		return f.CreatePushImageTool(), nil
	case "pull_image":
		return f.CreatePullImageTool(), nil
	case "tag_image":
		return f.CreateTagImageTool(), nil
	case "scan_image_security":
		return f.CreateScanImageSecurityTool(), nil
	case "scan_secrets":
		return f.CreateScanSecretsTool(), nil
	case "generate_manifests":
		return f.CreateGenerateManifestsTool(), nil
	case "deploy_kubernetes":
		return f.CreateDeployKubernetesTool(), nil
	case "check_health":
		return f.CreateCheckHealthTool(), nil
	case "generate_dockerfile":
		return f.CreateGenerateDockerfileTool(), nil
	case "validate_dockerfile":
		return f.CreateValidateDockerfileTool(), nil
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

// GetEnhancedBuildAnalyzer returns the enhanced build analyzer instance
func (f *ToolFactory) GetEnhancedBuildAnalyzer() *build.EnhancedBuildAnalyzer {
	return f.analyzerHelper.GetEnhancedBuildAnalyzer()
}
