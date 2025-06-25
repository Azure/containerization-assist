package execution

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/workflow"
	"github.com/rs/zerolog"
)

// ParallelExecutor handles parallel execution of workflow stage tools
type ParallelExecutor struct {
	logger         zerolog.Logger
	maxConcurrency int
}

// NewParallelExecutor creates a new parallel executor
func NewParallelExecutor(logger zerolog.Logger, maxConcurrency int) *ParallelExecutor {
	if maxConcurrency <= 0 {
		maxConcurrency = 10 // Default max concurrency
	}
	return &ParallelExecutor{
		logger:         logger.With().Str("executor", "parallel").Logger(),
		maxConcurrency: maxConcurrency,
	}
}

// Execute runs tools in parallel with optional concurrency limit
func (pe *ParallelExecutor) Execute(
	ctx context.Context,
	stage *workflow.WorkflowStage,
	session *workflow.WorkflowSession,
	toolNames []string,
	executeToolFunc ExecuteToolFunc,
) (*ExecutionResult, error) {
	pe.logger.Debug().
		Str("stage_name", stage.Name).
		Int("tool_count", len(toolNames)).
		Int("max_concurrency", pe.maxConcurrency).
		Msg("Starting parallel execution")

	result := &ExecutionResult{
		Results:   make(map[string]interface{}),
		Artifacts: []workflow.WorkflowArtifact{},
		Metrics: map[string]interface{}{
			"execution_type":  "parallel",
			"tool_count":      len(toolNames),
			"max_concurrency": pe.maxConcurrency,
		},
	}

	startTime := time.Now()

	// Create channels for results and errors
	type toolResult struct {
		toolName string
		result   interface{}
		err      error
		index    int
	}

	resultChan := make(chan toolResult, len(toolNames))
	var wg sync.WaitGroup

	// Create semaphore for concurrency control
	semaphore := make(chan struct{}, pe.maxConcurrency)

	// Launch goroutines for each tool
	for i, toolName := range toolNames {
		wg.Add(1)
		go func(index int, name string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Check context cancellation
			select {
			case <-ctx.Done():
				resultChan <- toolResult{
					toolName: name,
					err:      ctx.Err(),
					index:    index,
				}
				return
			default:
			}

			pe.logger.Debug().
				Str("stage_name", stage.Name).
				Str("tool_name", name).
				Int("tool_index", index).
				Msg("Starting parallel tool execution")

			// Execute tool
			toolRes, err := executeToolFunc(ctx, name, stage, session)

			resultChan <- toolResult{
				toolName: name,
				result:   toolRes,
				err:      err,
				index:    index,
			}

			if err != nil {
				pe.logger.Error().
					Err(err).
					Str("stage_name", stage.Name).
					Str("tool_name", name).
					Msg("Tool execution failed in parallel")
			} else {
				pe.logger.Debug().
					Str("stage_name", stage.Name).
					Str("tool_name", name).
					Msg("Tool execution completed in parallel")
			}
		}(i, toolName)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var firstError error
	successCount := 0
	failureCount := 0
	resultsMutex := sync.Mutex{}

	for res := range resultChan {
		if res.err != nil {
			failureCount++
			if firstError == nil {
				firstError = res.err
				result.Error = &ExecutionError{
					ToolName: res.toolName,
					Index:    res.index,
					Error:    res.err,
					Type:     "parallel_execution_error",
				}
			}
		} else {
			successCount++
			resultsMutex.Lock()
			result.Results[res.toolName] = res.result

			// Extract artifacts if present
			if artifacts := extractArtifacts(res.result); artifacts != nil {
				result.Artifacts = append(result.Artifacts, artifacts...)
			}
			resultsMutex.Unlock()
		}
	}

	result.Duration = time.Since(startTime)
	result.Metrics["execution_time"] = result.Duration.String()
	result.Metrics["successful_tools"] = successCount
	result.Metrics["failed_tools"] = failureCount

	if firstError != nil {
		pe.logger.Error().
			Str("stage_name", stage.Name).
			Int("failed_count", failureCount).
			Int("success_count", successCount).
			Err(firstError).
			Msg("Parallel execution completed with errors")
		return result, fmt.Errorf("parallel execution failed: %d tools failed, first error: %w", failureCount, firstError)
	}

	result.Success = true
	pe.logger.Info().
		Str("stage_name", stage.Name).
		Int("tools_executed", len(toolNames)).
		Dur("duration", result.Duration).
		Msg("Parallel execution completed successfully")

	return result, nil
}
