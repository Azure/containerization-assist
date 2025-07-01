package build

import (
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/rs/zerolog"
)

// This file now serves as a compatibility layer and main entry point.
// The actual implementation has been split into focused modules:
// - fixer_types.go: Type definitions and analysis functions
// - fixer_core.go: Core fixer framework and orchestration
// - recovery_strategies.go: Concrete recovery strategy implementations
// - resource_management.go: System resource management and optimization

// NewAdvancedBuildFixer creates a new advanced build fixer with default strategies
// This is the main entry point that external packages should use
func NewAdvancedBuildFixerWithDefaults(logger zerolog.Logger, analyzer core.AIAnalyzer, sessionManager core.ToolSessionManager) *AdvancedBuildFixer {
	// Create the core fixer
	fixer := NewAdvancedBuildFixer(logger, analyzer, sessionManager)

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

// CreateAdvancedBuildFixer provides backward compatibility for older code
func CreateAdvancedBuildFixer(logger zerolog.Logger, analyzer core.AIAnalyzer, sessionManager core.ToolSessionManager) *AdvancedBuildFixer {
	return NewAdvancedBuildFixerWithDefaults(logger, analyzer, sessionManager)
}

// NewBuildOperation creates a new Docker build operation
func NewBuildOperation(tool *AtomicBuildImageTool, args AtomicBuildImageArgs, session *core.SessionState, workspaceDir, buildContext, dockerfilePath string, logger zerolog.Logger) *AtomicDockerBuildOperation {
	return NewAtomicDockerBuildOperation(tool, args, session, workspaceDir, buildContext, dockerfilePath, logger)
}

// CreateResourceMonitor creates a new resource monitor
func CreateResourceMonitor(logger zerolog.Logger) *ResourceMonitor {
	return NewResourceMonitor(logger)
}

// CreateSpaceOptimizer creates a new space optimizer
func CreateSpaceOptimizer(logger zerolog.Logger) *SpaceOptimizer {
	return NewSpaceOptimizer(logger)
}
