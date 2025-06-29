package build

import (
	"fmt"

	"github.com/rs/zerolog"
)

// StrategyManager manages different build strategies
type StrategyManager struct {
	strategies map[string]BuildStrategy
	logger     zerolog.Logger
}

// NewStrategyManager creates a new strategy manager
func NewStrategyManager(logger zerolog.Logger) *StrategyManager {
	sm := &StrategyManager{
		strategies: make(map[string]BuildStrategy),
		logger:     logger.With().Str("component", "strategy_manager").Logger(),
	}
	// Register default strategies
	// TODO: Fix method calls - strategy constructors not found
	// sm.RegisterStrategy(NewDockerBuildStrategy(logger))
	// sm.RegisterStrategy(NewBuildKitStrategy(logger))
	return sm
}

// RegisterStrategy registers a new build strategy
func (sm *StrategyManager) RegisterStrategy(strategy BuildStrategy) {
	sm.strategies[strategy.Name()] = strategy
}

// SelectStrategy selects the best strategy for the given context
func (sm *StrategyManager) SelectStrategy(ctx BuildContext) (BuildStrategy, error) {
	sm.logger.Info().
		Str("dockerfile", ctx.DockerfilePath).
		Bool("buildkit_available", sm.IsBuildKitAvailable()).
		Msg("Selecting build strategy")
	// Check if BuildKit is requested and available
	if sm.IsBuildKitAvailable() && sm.ShouldUseBuildKit(ctx) {
		if strategy, exists := sm.strategies["buildkit"]; exists {
			if err := strategy.Validate(ctx); err == nil {
				sm.logger.Info().Str("strategy", "buildkit").Msg("Selected BuildKit strategy")
				return strategy, nil
			}
		}
	}
	// Default to standard Docker build
	if strategy, exists := sm.strategies["docker"]; exists {
		if err := strategy.Validate(ctx); err == nil {
			sm.logger.Info().Str("strategy", "docker").Msg("Selected Docker strategy")
			return strategy, nil
		}
	}

	return nil, fmt.Errorf("no suitable build strategy found")
}

// IsBuildKitAvailable checks if BuildKit is available
func (sm *StrategyManager) IsBuildKitAvailable() bool {
	// Simple check - could be enhanced to actually check Docker version
	return false // Default to false for now
}

// ShouldUseBuildKit determines if BuildKit should be used for the given context
func (sm *StrategyManager) ShouldUseBuildKit(ctx BuildContext) bool {
	// Simple logic - could be enhanced based on dockerfile features, etc.
	return false // Default to false for now
}
