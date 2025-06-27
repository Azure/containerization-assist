package orchestration

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	"github.com/Azure/container-kit/pkg/mcp/internal/state"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// StateAwareToolWrapper wraps tools with state management capabilities
type StateAwareToolWrapper struct {
	tool         interface{} // server.Tool interface not available
	stateManager *state.UnifiedStateManager
	toolName     string
	logger       zerolog.Logger
}

// NewStateAwareToolWrapper creates a new state-aware tool wrapper
func NewStateAwareToolWrapper(
	tool interface{}, // server.Tool interface not available
	stateManager *state.UnifiedStateManager,
	toolName string,
	logger zerolog.Logger,
) interface{} {
	return &StateAwareToolWrapper{
		tool:         tool,
		stateManager: stateManager,
		toolName:     toolName,
		logger:       logger.With().Str("component", "state_aware_tool").Str("tool", toolName).Logger(),
	}
}

// Run executes the tool with state management
func (w *StateAwareToolWrapper) Run(arguments interface{}) (interface{}, error) {
	ctx := context.Background()

	// Create state transaction
	transaction := w.stateManager.CreateStateTransaction(ctx)

	// Record tool invocation
	invocationState := map[string]interface{}{
		"tool_name": w.toolName,
		"arguments": arguments,
		"timestamp": time.Now(),
		"status":    "running",
	}

	invocationID := fmt.Sprintf("%s_%d", w.toolName, time.Now().UnixNano())
	transaction.Set(state.StateTypeTool, invocationID, invocationState)

	// Commit pre-execution state
	if err := transaction.Commit(); err != nil {
		w.logger.Error().Err(err).Msg("Failed to record tool invocation")
	}

	// Execute the tool
	startTime := time.Now()
	result, err := interface{}(nil), fmt.Errorf("tool execution not implemented")
	duration := time.Since(startTime)

	// Update state with result
	resultTransaction := w.stateManager.CreateStateTransaction(ctx)

	// Update invocation state
	invocationState["status"] = "completed"
	invocationState["duration_ms"] = duration.Milliseconds()
	invocationState["has_error"] = err != nil
	if err != nil {
		invocationState["error"] = err.Error()
	}

	resultTransaction.Set(state.StateTypeTool, invocationID, invocationState)

	// Store tool-specific state if available
	if stateProvider, ok := result.(StateProvider); ok {
		toolState := stateProvider.GetToolState()
		if toolState != nil {
			stateID := fmt.Sprintf("%s_state_%d", w.toolName, time.Now().UnixNano())
			resultTransaction.Set(state.StateTypeTool, stateID, toolState)
		}
	}

	// Commit result state
	if commitErr := resultTransaction.Commit(); commitErr != nil {
		w.logger.Error().Err(commitErr).Msg("Failed to record tool result")
	}

	// Log execution
	logEvent := w.logger.Info().
		Str("invocation_id", invocationID).
		Dur("duration", duration).
		Bool("success", err == nil)

	if err != nil {
		logEvent.Err(err)
	}

	logEvent.Msg("Tool execution completed")

	return result, err
}

// StateProvider interface for tools that provide state
type StateProvider interface {
	GetToolState() interface{}
}

// EnhancedToolFactory creates tools with state awareness and enhanced analyzers
type EnhancedToolFactory struct {
	*ToolFactory
	stateManager *state.UnifiedStateManager
}

// NewEnhancedToolFactory creates a new enhanced tool factory
func NewEnhancedToolFactory(
	baseFactory *ToolFactory,
	stateManager *state.UnifiedStateManager,
) *EnhancedToolFactory {
	return &EnhancedToolFactory{
		ToolFactory:  baseFactory,
		stateManager: stateManager,
	}
}

// CreateBuildTool creates a state-aware build tool
func (f *EnhancedToolFactory) CreateBuildTool(analyzer mcptypes.AIAnalyzer) (interface{}, error) {
	// Create base tool with enhanced analyzer
	baseTool := f.ToolFactory.CreateBuildImageTool()

	// Wrap with state awareness
	return NewStateAwareToolWrapper(baseTool, f.stateManager, "docker_build", f.logger), nil
}

// CreatePushTool creates a state-aware push tool
func (f *EnhancedToolFactory) CreatePushTool(analyzer mcptypes.AIAnalyzer) (interface{}, error) {
	baseTool := f.ToolFactory.CreatePushImageTool()

	return NewStateAwareToolWrapper(baseTool, f.stateManager, "docker_push", f.logger), nil
}

// CreateDeployTool creates a state-aware deploy tool
func (f *EnhancedToolFactory) CreateDeployTool(analyzer mcptypes.AIAnalyzer) (interface{}, error) {
	baseTool := f.ToolFactory.CreateDeployKubernetesTool()

	return NewStateAwareToolWrapper(baseTool, f.stateManager, "k8s_deploy", f.logger), nil
}

// CrossToolStateCoordinator coordinates state between different tools
type CrossToolStateCoordinator struct {
	stateManager *state.UnifiedStateManager
	logger       zerolog.Logger
}

// NewCrossToolStateCoordinator creates a new cross-tool state coordinator
func NewCrossToolStateCoordinator(
	stateManager *state.UnifiedStateManager,
	logger zerolog.Logger,
) *CrossToolStateCoordinator {
	return &CrossToolStateCoordinator{
		stateManager: stateManager,
		logger:       logger.With().Str("component", "cross_tool_coordinator").Logger(),
	}
}

// ShareAnalyzerInsights shares analyzer insights across tools
func (c *CrossToolStateCoordinator) ShareAnalyzerInsights(
	ctx context.Context,
	sourceTool string,
	insights *build.ToolInsights,
) error {
	// Store insights in global state
	insightID := fmt.Sprintf("insights_%s_%d", sourceTool, time.Now().UnixNano())

	globalInsights := map[string]interface{}{
		"source_tool": sourceTool,
		"timestamp":   time.Now(),
		"insights":    insights,
		"shared":      true,
	}

	return c.stateManager.SetState(ctx, state.StateTypeGlobal, insightID, globalInsights)
}

// GetSharedInsights retrieves shared insights for a tool
func (c *CrossToolStateCoordinator) GetSharedInsights(
	ctx context.Context,
	targetTool string,
	since time.Time,
) ([]*build.ToolInsights, error) {
	// This would query global state for relevant insights
	// For now, return empty list
	return []*build.ToolInsights{}, nil
}

// SynchronizeToolStates synchronizes states between related tools
func (c *CrossToolStateCoordinator) SynchronizeToolStates(
	ctx context.Context,
	toolPairs []ToolStatePair,
) error {
	for _, pair := range toolPairs {
		// Create mapping between tools
		mapping := state.NewToolStateMapping(pair.SourceTool, pair.TargetTool)
		for source, target := range pair.FieldMappings {
			mapping.AddFieldMapping(source, target)
		}

		// Sync states
		if err := c.stateManager.SyncStates(ctx, state.StateTypeTool, state.StateTypeTool, mapping); err != nil {
			c.logger.Error().
				Err(err).
				Str("source_tool", pair.SourceTool).
				Str("target_tool", pair.TargetTool).
				Msg("Tool state synchronization failed")
			return err
		}
	}

	return nil
}

// ToolStatePair defines state synchronization between tools
type ToolStatePair struct {
	SourceTool    string
	TargetTool    string
	FieldMappings map[string]string
}
