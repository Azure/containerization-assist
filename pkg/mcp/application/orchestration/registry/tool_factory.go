package registry

import (
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/core"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// ToolFactory extends the canonical api.ToolFactory with registry-specific methods
type ToolFactory interface {
	api.ToolFactory

	// CreateToolWithCategory creates a tool by category and name (registry-specific version)
	CreateToolWithCategory(category ToolCategory, name string) (api.Tool, error)

	// CreateCoreAnalyzer creates an analyzer (special case due to interfaces)
	CreateCoreAnalyzer(aiAnalyzer core.AIAnalyzer) core.Analyzer

	// RegisterToolCreatorWithCategory registers a tool creator function for a category and name
	RegisterToolCreatorWithCategory(category ToolCategory, name string, creator ToolCreator)
}

// ToolCategory represents different categories of tools
type ToolCategory string

const (
	ToolCategoryAnalyze ToolCategory = "analyze"
	ToolCategoryBuild   ToolCategory = "build"
	ToolCategoryDeploy  ToolCategory = "deploy"
	ToolCategoryScan    ToolCategory = "scan"
	ToolCategorySession ToolCategory = "session"
)

// ToolCreator is a function that creates a tool (registry-specific version)
type ToolCreator func() (api.Tool, error)

// DefaultToolFactory provides a default implementation that can be extended
type DefaultToolFactory struct {
	creators map[string]ToolCreator
}

// NewDefaultToolFactory creates a new default tool factory
func NewDefaultToolFactory() *DefaultToolFactory {
	return &DefaultToolFactory{
		creators: make(map[string]ToolCreator),
	}
}

// CreateToolWithCategory implements ToolFactory (registry-specific version)
func (f *DefaultToolFactory) CreateToolWithCategory(category ToolCategory, name string) (api.Tool, error) {
	key := fmt.Sprintf("%s:%s", category, name)
	creator, exists := f.creators[key]
	if !exists {
		return nil, errors.NewError().Messagef("no tool creator registered for %s:%s", category, name).WithLocation().Build()
	}
	return creator()
}

// RegisterToolCreatorWithCategory implements ToolFactory (registry-specific version)
func (f *DefaultToolFactory) RegisterToolCreatorWithCategory(category ToolCategory, name string, creator ToolCreator) {
	key := fmt.Sprintf("%s:%s", category, name)
	f.creators[key] = creator
}

// RegisterToolCreator implements api.ToolFactory (canonical version)
func (f *DefaultToolFactory) RegisterToolCreator(category string, name string, creator api.ToolCreator) {
	key := fmt.Sprintf("%s:%s", category, name)
	f.creators[key] = func() (api.Tool, error) {
		tool, err := creator()
		if err != nil {
			return nil, err
		}
		return tool, nil
	}
}

// CreateTool implements api.ToolFactory (canonical version)
func (f *DefaultToolFactory) CreateTool(category string, name string) (api.Tool, error) {
	key := fmt.Sprintf("%s:%s", category, name)
	creator, exists := f.creators[key]
	if !exists {
		return nil, errors.NewError().Messagef("no tool creator registered for %s:%s", category, name).WithLocation().Build()
	}
	apiTool, err := creator()
	if err != nil {
		return nil, err
	}
	return apiTool, nil
}

// CreateCoreAnalyzer implements ToolFactory (registry-specific version)
func (f *DefaultToolFactory) CreateCoreAnalyzer(_ core.AIAnalyzer) core.Analyzer {
	return nil
}

// CreateAnalyzer implements api.ToolFactory (canonical version)
func (f *DefaultToolFactory) CreateAnalyzer(aiAnalyzer interface{}) interface{} {
	if coreAnalyzer, ok := aiAnalyzer.(core.AIAnalyzer); ok {
		return f.CreateCoreAnalyzer(coreAnalyzer)
	}
	return nil
}

// CreateEnhancedBuildAnalyzer implements ToolFactory
func (f *DefaultToolFactory) CreateEnhancedBuildAnalyzer() interface{} {
	return nil
}

// CreateSessionStateManager implements ToolFactory
func (f *DefaultToolFactory) CreateSessionStateManager(sessionID string) interface{} {
	return nil
}
