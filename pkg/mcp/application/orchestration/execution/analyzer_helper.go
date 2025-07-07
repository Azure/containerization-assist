package execution

import (
	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/application/internal/common"
	"github.com/Azure/container-kit/pkg/mcp/domain/containerization/analyze"
	"github.com/Azure/container-kit/pkg/mcp/domain/containerization/build"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
)

// AnalyzerHelper provides common analyzer initialization patterns
type AnalyzerHelper struct {
	analyzer              core.Analyzer // Unified analyzer interface
	enhancedBuildAnalyzer *build.EnhancedBuildAnalyzer
	sessionManager        session.UnifiedSessionManager
	logger                *slog.Logger
}

// NewAnalyzerHelper creates a new analyzer helper
func NewAnalyzerHelper(analyzer core.AIAnalyzer, logger *slog.Logger) *AnalyzerHelper {
	if analyzer == nil {
		return &AnalyzerHelper{
			analyzer: nil,
			logger:   logger,
		}
	}

	// Create unified analyzer from AI analyzer and a new repository analyzer
	repoAnalyzer := analyze.NewCoreRepositoryAnalyzer(logger)
	unifiedAnalyzer := analyze.NewUnifiedAnalyzer(repoAnalyzer, analyzer, logger)

	return &AnalyzerHelper{
		analyzer: unifiedAnalyzer,
		logger:   logger,
	}
}

// NewAnalyzerHelperWithSessionManager creates a new analyzer helper with session manager support
func NewAnalyzerHelperWithSessionManager(
	analyzer core.AIAnalyzer,
	sessionManager session.UnifiedSessionManager,
	logger *slog.Logger,
) *AnalyzerHelper {
	if analyzer == nil {
		return &AnalyzerHelper{
			analyzer:       nil,
			sessionManager: sessionManager,
			logger:         logger,
		}
	}

	// Create unified analyzer from AI analyzer and a new repository analyzer
	repoAnalyzer := analyze.NewCoreRepositoryAnalyzer(logger)
	unifiedAnalyzer := analyze.NewUnifiedAnalyzer(repoAnalyzer, analyzer, logger)

	return &AnalyzerHelper{
		analyzer:       unifiedAnalyzer,
		sessionManager: sessionManager,
		logger:         logger,
	}
}

// SetupBuildToolAnalyzer sets up analyzer and fixing mixin for build tools
func (h *AnalyzerHelper) SetupBuildToolAnalyzer(toolName string) (*common.DefaultFailureAnalyzer, *build.AtomicToolFixingMixin) {
	if h.analyzer == nil {
		return nil, nil
	}

	// Ensure enhanced analyzer exists
	h.ensureEnhancedAnalyzer()

	// Create tool analyzer and fixing mixin
	analyzer := common.NewDefaultFailureAnalyzer(toolName, "build", h.logger)
	fixingMixin := build.NewAtomicToolFixingMixin(h.analyzer, "atomic_"+toolName, h.logger)

	return analyzer, fixingMixin
}

// SetupAnalyzeToolAnalyzer sets up analyzer for analyze tools
func (h *AnalyzerHelper) SetupAnalyzeToolAnalyzer(toolName string) *common.DefaultFailureAnalyzer {
	if h.analyzer == nil {
		return nil
	}

	// Create a default analyzer for analyze tools
	return common.NewDefaultFailureAnalyzer(toolName, "analyze", h.logger)
}

// SetupDeployToolAnalyzer sets up analyzer and fixing mixin for deploy tools
func (h *AnalyzerHelper) SetupDeployToolAnalyzer(toolName string) (*common.DefaultFailureAnalyzer, *build.AtomicToolFixingMixin) {
	if h.analyzer == nil {
		return nil, nil
	}

	// Create tool analyzer and fixing mixin
	analyzer := common.NewDefaultFailureAnalyzer(toolName, "deploy", h.logger)
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
		// Use the unified analyzer which provides both AI and repository analysis
		h.enhancedBuildAnalyzer = build.NewEnhancedBuildAnalyzerFromUnified(h.analyzer, h.logger)
		h.logger.Info("Created enhanced build analyzer with unified analyzer")
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
func (h *AnalyzerHelper) SetupScanToolAnalyzer(toolName string) *common.DefaultFailureAnalyzer {
	if h.analyzer == nil {
		return nil
	}

	// Create a default analyzer for scan tools
	return common.NewDefaultFailureAnalyzer(toolName, "scan", h.logger)
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
