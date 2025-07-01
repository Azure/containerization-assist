package orchestration

import (
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/analyze"
	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	"github.com/Azure/container-kit/pkg/mcp/internal/deploy"
	"github.com/Azure/container-kit/pkg/mcp/internal/scan"
	"github.com/rs/zerolog"
)

// AnalyzerHelper provides common analyzer initialization patterns
type AnalyzerHelper struct {
	analyzer              core.AIAnalyzer
	enhancedBuildAnalyzer *build.EnhancedBuildAnalyzer
	repositoryAnalyzer    core.RepositoryAnalyzer // Use core interface directly
	sessionManager        core.ToolSessionManager
	logger                zerolog.Logger
}

// NewAnalyzerHelper creates a new analyzer helper
func NewAnalyzerHelper(analyzer core.AIAnalyzer, logger zerolog.Logger) *AnalyzerHelper {
	return &AnalyzerHelper{
		analyzer: analyzer,
		logger:   logger,
	}
}

// NewAnalyzerHelperWithDependencies creates a new analyzer helper with tool dependencies support
func NewAnalyzerHelperWithDependencies(
	analyzer core.AIAnalyzer,
	toolDeps *ToolDependencies,
	sessionManager core.ToolSessionManager,
	logger zerolog.Logger,
) *AnalyzerHelper {
	helper := &AnalyzerHelper{
		analyzer:       analyzer,
		sessionManager: sessionManager,
		logger:         logger,
	}

	// Create the core repository analyzer directly (no adapter needed)
	if toolDeps != nil && sessionManager != nil {
		helper.repositoryAnalyzer = analyze.NewCoreRepositoryAnalyzer(logger)
	}

	return helper
}

// SetupBuildToolAnalyzer sets up analyzer and fixing mixin for build tools
func (h *AnalyzerHelper) SetupBuildToolAnalyzer(toolName string) (*build.DefaultToolAnalyzer, *build.AtomicToolFixingMixin) {
	if h.analyzer == nil {
		return nil, nil
	}

	// Ensure enhanced analyzer exists
	h.ensureEnhancedAnalyzer()

	// Create tool analyzer and fixing mixin
	analyzer := build.NewDefaultToolAnalyzer(toolName)
	fixingMixin := build.NewAtomicToolFixingMixin(h.analyzer, "atomic_"+toolName, h.logger)

	return analyzer, fixingMixin
}

// SetupAnalyzeToolAnalyzer sets up analyzer for analyze tools
func (h *AnalyzerHelper) SetupAnalyzeToolAnalyzer(toolName string) *analyze.DefaultToolAnalyzer {
	if h.analyzer == nil {
		return nil
	}

	// Create a default analyzer for analyze tools
	return analyze.NewDefaultToolAnalyzer(toolName)
}

// SetupDeployToolAnalyzer sets up analyzer and fixing mixin for deploy tools
func (h *AnalyzerHelper) SetupDeployToolAnalyzer(toolName string) (*deploy.DefaultToolAnalyzer, *build.AtomicToolFixingMixin) {
	if h.analyzer == nil {
		return nil, nil
	}

	// Create tool analyzer and fixing mixin
	analyzer := deploy.NewDefaultToolAnalyzer(toolName)
	fixingMixin := build.NewAtomicToolFixingMixin(h.analyzer, "atomic_"+toolName, h.logger)

	return analyzer, fixingMixin
}

// GetEnhancedBuildAnalyzer returns the enhanced build analyzer for build image tool
func (h *AnalyzerHelper) GetEnhancedBuildAnalyzer() *build.EnhancedBuildAnalyzer {
	h.ensureEnhancedAnalyzer()
	return h.enhancedBuildAnalyzer
}

// ensureEnhancedAnalyzer creates the enhanced build analyzer if it doesn't exist
func (h *AnalyzerHelper) ensureEnhancedAnalyzer() {
	if h.enhancedBuildAnalyzer == nil && h.analyzer != nil {
		// Use the core repository analyzer directly (no adapter needed)
		if h.repositoryAnalyzer != nil {
			h.logger.Info().Msg("Using core RepositoryAnalyzer for enhanced build analyzer")
		} else {
			// Create one if it doesn't exist
			h.repositoryAnalyzer = analyze.NewCoreRepositoryAnalyzer(h.logger)
			h.logger.Info().Msg("Created new core RepositoryAnalyzer for enhanced build analyzer")
		}

		h.enhancedBuildAnalyzer = build.NewEnhancedBuildAnalyzer(h.analyzer, h.repositoryAnalyzer, h.logger)
	}
}

// BuildToolInitializer provides a fluent interface for setting up build tools
type BuildToolInitializer struct {
	helper *AnalyzerHelper
}

// NewBuildToolInitializer creates a new build tool initializer
func NewBuildToolInitializer(helper *AnalyzerHelper) *BuildToolInitializer {
	return &BuildToolInitializer{helper: helper}
}

// SetupAnalyzer sets up analyzer and fixing mixin on a build tool that supports them
func (b *BuildToolInitializer) SetupAnalyzer(tool interface{}, toolName string) {
	analyzer, fixingMixin := b.helper.SetupBuildToolAnalyzer(toolName)

	// Try to set analyzer if tool supports it
	if setter, ok := tool.(interface{ SetAnalyzer(interface{}) }); ok && analyzer != nil {
		setter.SetAnalyzer(analyzer)
	}

	// Try to set fixing mixin if tool supports it
	if setter, ok := tool.(interface{ SetFixingMixin(interface{}) }); ok && fixingMixin != nil {
		setter.SetFixingMixin(fixingMixin)
	}
}

// DeployToolInitializer provides a fluent interface for setting up deploy tools
type DeployToolInitializer struct {
	helper *AnalyzerHelper
}

// NewDeployToolInitializer creates a new deploy tool initializer
func NewDeployToolInitializer(helper *AnalyzerHelper) *DeployToolInitializer {
	return &DeployToolInitializer{helper: helper}
}

// SetupAnalyzer sets up analyzer and fixing mixin on a deploy tool that supports them
func (d *DeployToolInitializer) SetupAnalyzer(tool interface{}, toolName string) {
	// Deploy tools expect the core AI analyzer directly, not a tool-specific analyzer
	if d.helper.analyzer == nil {
		return
	}

	// Try to set analyzer if tool supports it
	if setter, ok := tool.(interface{ SetAnalyzer(interface{}) }); ok {
		setter.SetAnalyzer(d.helper.analyzer)
	}

	// Note: Deploy tools create their own fixing mixin internally when SetAnalyzer is called
	// So we don't need to set it separately
}

// SetupScanToolAnalyzer sets up analyzer for scan tools
func (h *AnalyzerHelper) SetupScanToolAnalyzer(toolName string) *scan.DefaultToolAnalyzer {
	if h.analyzer == nil {
		return nil
	}

	// Create a default analyzer for scan tools
	return scan.NewDefaultToolAnalyzer(toolName)
}

// ScanToolInitializer provides a fluent interface for setting up scan tools
type ScanToolInitializer struct {
	helper *AnalyzerHelper
}

// NewScanToolInitializer creates a new scan tool initializer
func NewScanToolInitializer(helper *AnalyzerHelper) *ScanToolInitializer {
	return &ScanToolInitializer{helper: helper}
}

// SetupAnalyzer sets up analyzer on a scan tool that supports it
func (s *ScanToolInitializer) SetupAnalyzer(tool interface{}, toolName string) {
	analyzer := s.helper.SetupScanToolAnalyzer(toolName)

	// Try to set analyzer if tool supports it
	if setter, ok := tool.(interface{ SetAnalyzer(interface{}) }); ok && analyzer != nil {
		setter.SetAnalyzer(analyzer)
	}
}
