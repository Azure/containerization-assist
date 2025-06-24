package execution

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/orchestration/workflow"
	"github.com/rs/zerolog"
)

// SequentialExecutor handles sequential execution of workflow stage tools
type SequentialExecutor struct {
	logger zerolog.Logger
}

// NewSequentialExecutor creates a new sequential executor
func NewSequentialExecutor(logger zerolog.Logger) *SequentialExecutor {
	return &SequentialExecutor{
		logger: logger.With().Str("executor", "sequential").Logger(),
	}
}

// Execute runs tools sequentially in the order specified
func (se *SequentialExecutor) Execute(
	ctx context.Context,
	stage *workflow.WorkflowStage,
	session *workflow.WorkflowSession,
	toolNames []string,
	executeToolFunc ExecuteToolFunc,
) (*ExecutionResult, error) {
	se.logger.Debug().
		Str("stage_name", stage.Name).
		Int("tool_count", len(toolNames)).
		Msg("Starting sequential execution")

	result := &ExecutionResult{
		Results:   make(map[string]interface{}),
		Artifacts: []workflow.WorkflowArtifact{},
		Metrics: map[string]interface{}{
			"execution_type": "sequential",
			"tool_count":     len(toolNames),
		},
	}

	startTime := time.Now()

	for i, toolName := range toolNames {
		se.logger.Debug().
			Str("stage_name", stage.Name).
			Str("tool_name", toolName).
			Int("tool_index", i).
			Int("progress", i+1).
			Int("total", len(toolNames)).
			Msg("Executing tool in sequence")

		toolResult, err := executeToolFunc(ctx, toolName, stage, session)
		if err != nil {
			se.logger.Error().
				Err(err).
				Str("stage_name", stage.Name).
				Str("tool_name", toolName).
				Int("failed_at_index", i).
				Msg("Tool execution failed")

			result.Error = &ExecutionError{
				ToolName: toolName,
				Index:    i,
				Error:    err,
				Type:     "sequential_execution_error",
			}
			return result, fmt.Errorf("tool %s failed at index %d: %w", toolName, i, err)
		}

		// Store tool result
		result.Results[toolName] = toolResult

		// Extract artifacts if present
		if artifacts := extractArtifacts(toolResult); artifacts != nil {
			result.Artifacts = append(result.Artifacts, artifacts...)
		}

		se.logger.Debug().
			Str("stage_name", stage.Name).
			Str("tool_name", toolName).
			Int("completed", i+1).
			Int("total", len(toolNames)).
			Msg("Tool execution completed successfully")
	}

	result.Duration = time.Since(startTime)
	result.Metrics["execution_time"] = result.Duration.String()
	result.Success = true

	se.logger.Info().
		Str("stage_name", stage.Name).
		Int("tools_executed", len(toolNames)).
		Dur("duration", result.Duration).
		Msg("Sequential execution completed successfully")

	return result, nil
}
