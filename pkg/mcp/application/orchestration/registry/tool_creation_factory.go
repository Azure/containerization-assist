package registry

import (
	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/core"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/application/orchestration/execution"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/rs/zerolog"
)

// ToolDependenciesFactory holds dependencies without internal package imports
type ToolDependenciesFactory struct {
	pipelineOperations mcptypes.TypedPipelineOperations
	sessionManager     session.UnifiedSessionManager
	analyzer           core.AIAnalyzer
	analyzerHelper     *execution.AnalyzerHelperFactory
	logger             zerolog.Logger
	factory            ToolFactory
}

// NewToolDependenciesFactory creates tool dependencies using factory pattern
func NewToolDependenciesFactory(
	pipelineOperations mcptypes.TypedPipelineOperations,
	sessionManager session.UnifiedSessionManager,
	analyzer core.AIAnalyzer,
	factory ToolFactory,
	logger zerolog.Logger,
) *ToolDependenciesFactory {
	deps := &ToolDependenciesFactory{
		pipelineOperations: pipelineOperations,
		sessionManager:     sessionManager,
		analyzer:           analyzer,
		logger:             logger,
		factory:            factory,
	}

	deps.analyzerHelper = execution.NewAnalyzerHelperFactoryWithSessionManager(
		analyzer,
		sessionManager,
		factory.(api.ToolFactory),
		logger,
	)

	return deps
}

// CreateToolFactory creates a tool by name using the factory
func (d *ToolDependenciesFactory) CreateToolFactory(toolName string) (interface{}, error) {
	if d.factory == nil {
		return nil, errors.NewError().
			Code(errors.CodeInternalError).
			Message("tool factory not initialized").
			Build()
	}

	category, err := getToolCategoryFromName(toolName)
	if err != nil {
		return nil, err
	}

	tool, err := d.factory.CreateTool(string(category), toolName)
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeToolNotFound).
			Messagef("failed to create tool %s: %v", toolName, err).
			Build()
	}

	if analyzerAware, ok := tool.(interface{ SetAnalyzer(interface{}) }); ok {
		d.setupToolAnalyzer(analyzerAware, toolName)
	}

	return tool, nil
}

// getToolCategoryFromName maps tool names to categories
func getToolCategoryFromName(toolName string) (ToolCategory, error) {
	categoryMap := map[string]ToolCategory{
		"analyze_repository":  ToolCategoryAnalyze,
		"generate_dockerfile": ToolCategoryAnalyze,
		"validate_dockerfile": ToolCategoryAnalyze,
		"build_image":         ToolCategoryBuild,
		"push_image":          ToolCategoryBuild,
		"pull_image":          ToolCategoryBuild,
		"tag_image":           ToolCategoryBuild,
		"generate_manifests":  ToolCategoryDeploy,
		"deploy_kubernetes":   ToolCategoryDeploy,
		"check_health":        ToolCategoryDeploy,
		"validate_deployment": ToolCategoryDeploy,
		"scan_image_security": ToolCategoryScan,
		"scan_secrets":        ToolCategoryScan,
	}

	category, exists := categoryMap[toolName]
	if !exists {
		return "", errors.NewError().
			Code(errors.CodeToolNotFound).
			Messagef("unknown tool: %s", toolName).
			Build()
	}

	return category, nil
}

// setupToolAnalyzer configures analyzer for tools that support it
func (d *ToolDependenciesFactory) setupToolAnalyzer(tool interface{ SetAnalyzer(interface{}) }, toolName string) {
	if d.analyzer == nil || d.analyzerHelper == nil {
		return
	}

	category, _ := getToolCategoryFromName(toolName)

	switch category {
	case ToolCategoryBuild:
		if enhancedAnalyzer := d.analyzerHelper.GetEnhancedBuildAnalyzer(); enhancedAnalyzer != nil {
			tool.SetAnalyzer(enhancedAnalyzer)
		}
	case ToolCategoryAnalyze:
		if analyzer := d.analyzerHelper.GetAnalyzer(); analyzer != nil {
			tool.SetAnalyzer(analyzer)
		}
	default:
		if analyzer := d.analyzerHelper.GetAnalyzer(); analyzer != nil {
			tool.SetAnalyzer(analyzer)
		}
	}
}

// GetAnalyzerHelper returns the analyzer helper
func (d *ToolDependenciesFactory) GetAnalyzerHelper() *execution.AnalyzerHelperFactory {
	return d.analyzerHelper
}

// GetFactory returns the tool factory
func (d *ToolDependenciesFactory) GetFactory() ToolFactory {
	return d.factory
}

// GetSessionManager returns the session manager
func (d *ToolDependenciesFactory) GetSessionManager() session.UnifiedSessionManager {
	return d.sessionManager
}

// GetPipelineOperations returns the pipeline operations
func (d *ToolDependenciesFactory) GetPipelineOperations() mcptypes.TypedPipelineOperations {
	return d.pipelineOperations
}

// GetLogger returns the logger
func (d *ToolDependenciesFactory) GetLogger() zerolog.Logger {
	return d.logger
}
