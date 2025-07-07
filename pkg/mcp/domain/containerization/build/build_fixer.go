package build

import (
	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/domain/types"
	"github.com/rs/zerolog"
)

// This file now serves as a compatibility layer and main entry point.
// The actual implementation has been split into focused modules:
// - fixer_types.go: Type definitions and analysis functions
// - fixer_core.go: Core fixer framework and orchestration
// - recovery_strategies.go: Concrete recovery strategy implementations
// - resource_management.go: System resource management and optimization

// NewAdvancedBuildFixerWithDefaults creates a new advanced build fixer with default strategies
// This is the main entry point that external packages should use
func NewAdvancedBuildFixerWithDefaults(logger zerolog.Logger, analyzer core.AIAnalyzer, sessionManager session.UnifiedSessionManager) *AdvancedBuildFixer {
	return NewAdvancedBuildFixerWithDefaultsUnified(logger, analyzer, sessionManager)
}

// NewAdvancedBuildFixerWithDefaultsUnified creates a new advanced build fixer with unified session manager
func NewAdvancedBuildFixerWithDefaultsUnified(logger zerolog.Logger, analyzer core.AIAnalyzer, sessionManager session.UnifiedSessionManager) *AdvancedBuildFixer {
	// Create the core fixer
	fixer := NewAdvancedBuildFixerUnified(logger, analyzer, sessionManager)

	return setupDefaultStrategies(fixer, logger)
}

// Legacy function removed - use NewAdvancedBuildFixerWithDefaultsUnified with session.UnifiedSessionManager

// setupDefaultStrategies registers default recovery strategies
func setupDefaultStrategies(fixer *AdvancedBuildFixer, logger zerolog.Logger) *AdvancedBuildFixer {

	// Register all default recovery strategies
	fixer.RegisterStrategy("network", NewNetworkErrorRecoveryStrategy(logger))
	fixer.RegisterStrategy("permission", NewPermissionErrorRecoveryStrategy(logger))
	fixer.RegisterStrategy("dockerfile", NewDockerfileErrorRecoveryStrategy(logger))
	fixer.RegisterStrategy("dependency", NewDependencyErrorRecoveryStrategy(logger))
	fixer.RegisterStrategy("disk_space", NewDiskSpaceRecoveryStrategy(logger))

	logger.Info().Msg("Advanced build fixer initialized with default strategies")

	return fixer
}

// Legacy compatibility functions - these delegate to the main implementations

// Legacy function removed - use NewAdvancedBuildFixerWithDefaultsUnified with session.UnifiedSessionManager

// NewBuildOperation creates a new Docker build operation
func NewBuildOperation(config mcptypes.BuildOperationConfig) (*AtomicDockerBuildOperation, error) {
	return NewAtomicDockerBuildOperation(config)
}

// CreateResourceMonitor creates a new resource monitor
func CreateResourceMonitor(logger zerolog.Logger) *ResourceMonitor {
	return NewResourceMonitor(logger)
}

// CreateSpaceOptimizer creates a new space optimizer
func CreateSpaceOptimizer(logger zerolog.Logger) *SpaceOptimizer {
	return NewSpaceOptimizer(logger)
}
