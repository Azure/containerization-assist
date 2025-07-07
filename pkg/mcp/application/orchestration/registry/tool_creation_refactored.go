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

// ToolDependenciesV2 holds dependencies without internal package imports
type ToolDependenciesV2 struct {
	pipelineOperations mcptypes.TypedPipelineOperations
	sessionManager     session.UnifiedSessionManager
	analyzer           core.AIAnalyzer
	analyzerHelper     *execution.AnalyzerHelperV2
	logger             zerolog.Logger
	factory            ToolFactory
}

// NewToolDependenciesV2 creates tool dependencies using factory pattern
func NewToolDependenciesV2(
	pipelineOperations mcptypes.TypedPipelineOperations,
	sessionManager session.UnifiedSessionManager,
	analyzer core.AIAnalyzer,
	factory ToolFactory,
	logger zerolog.Logger,
) *ToolDependenciesV2 {
	deps := &ToolDependenciesV2{
		pipelineOperations: pipelineOperations,
		sessionManager:     sessionManager,
		analyzer:           analyzer,
		logger:             logger,
		factory:            factory,
	}

	deps.analyzerHelper = execution.NewAnalyzerHelperV2WithSessionManager(
		analyzer,
		sessionManager,
		factory.(api.ToolFactory),
		logger,
	)

	return deps
}

// CreateToolV2 creates a tool by name using the factory
func (d *ToolDependenciesV2) CreateToolV2(toolName string) (interface{}, error) {
	if d.factory == nil {
		return nil, errors.NewError().
			Code(errors.CodeInternalError).
			Message("tool factory not initialized").
			Build()
	}

	category, err := getToolCategory(toolName)
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

// getToolCategory maps tool names to categories
func getToolCategory(toolName string) (ToolCategory, error) {
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
		return "", errors.NewError().Messagef("unknown tool: %s", toolName).WithLocation().Build()
	}

	return category, nil
}

// setupToolAnalyzer configures analyzer for tools that support it
func (d *ToolDependenciesV2) setupToolAnalyzer(tool interface{ SetAnalyzer(interface{}) }, toolName string) {
	if d.analyzer == nil || d.analyzerHelper == nil {
		return
	}

	category, _ := getToolCategory(toolName)

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
func (d *ToolDependenciesV2) GetAnalyzerHelper() *execution.AnalyzerHelperV2 {
	return d.analyzerHelper
}

// GetFactory returns the tool factory
func (d *ToolDependenciesV2) GetFactory() ToolFactory {
	return d.factory
}

// NewAnalyzerHelperV2WithSessionManager creates analyzer helper with factory
func NewAnalyzerHelperV2WithSessionManager(
	analyzer core.AIAnalyzer,
	sessionManager session.UnifiedSessionManager,
	factory ToolFactory,
	logger zerolog.Logger,
) *execution.AnalyzerHelperV2 {
	var interfaceFactory api.ToolFactory
	if factory != nil {
		interfaceFactory = factory.(api.ToolFactory)
	}

	return execution.NewAnalyzerHelperV2WithSessionManager(analyzer, sessionManager, interfaceFactory, logger)
}
