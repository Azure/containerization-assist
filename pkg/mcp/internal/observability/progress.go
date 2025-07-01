package observability

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// UnifiedProgressReporter provides a single, unified implementation for progress reporting
// across all atomic tools. This eliminates the need for multiple progress adapter
// implementations while providing direct GoMCP integration.
type UnifiedProgressReporter struct {
	serverCtx *server.Context // GoMCP integration
	stages    map[core.ProgressToken]*progressState
	logger    zerolog.Logger
	mutex     sync.RWMutex
}

// progressState tracks the state of an individual progress stage
type progressState struct {
	stage      *core.ProgressStage
	startTime  time.Time
	lastUpdate time.Time
	progress   int
}

// NewUnifiedProgressReporter creates a new unified progress reporter with GoMCP integration
func NewUnifiedProgressReporter(serverCtx *server.Context) core.ProgressReporter {
	return &UnifiedProgressReporter{
		serverCtx: serverCtx,
		stages:    make(map[core.ProgressToken]*progressState),
		logger:    zerolog.New(os.Stderr).With().Str("component", "progress").Logger(),
	}
}

// StartStage implements core.ProgressReporter
// Creates a new progress stage and returns a token for tracking
func (p *UnifiedProgressReporter) StartStage(stage string) core.ProgressToken {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Generate unique token
	token := core.ProgressToken(fmt.Sprintf("stage_%d_%s", time.Now().UnixNano(), stage))

	// Create progress stage
	progressStage := &core.ProgressStage{
		Name:        stage,
		Description: stage,
		Status:      "running",
		Progress:    0,
		Message:     fmt.Sprintf("Starting %s", stage),
		Weight:      1.0, // Default weight for all stages
	}

	// Store stage state
	p.stages[token] = &progressState{
		stage:     progressStage,
		startTime: time.Now(),
		progress:  0,
	}

	// Direct GoMCP integration - no adapter needed
	if p.serverCtx != nil {
		p.serverCtx.SendProgress(0.0, nil, progressStage.Message)
	}

	p.logger.Info().
		Str("token", string(token)).
		Str("stage", stage).
		Msg("Progress stage started")

	return token
}

// UpdateProgress implements core.ProgressReporter
// Updates the progress of an existing stage
func (p *UnifiedProgressReporter) UpdateProgress(token core.ProgressToken, message string, percent int) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	state, exists := p.stages[token]
	if !exists {
		p.logger.Warn().Str("token", string(token)).Msg("Progress token not found")
		return
	}

	// Validate percentage bounds
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	// Update stage state
	state.stage.Message = message
	state.stage.Progress = percent
	state.lastUpdate = time.Now()
	state.progress = percent

	// GoMCP integration
	if p.serverCtx != nil {
		p.serverCtx.SendProgress(float64(percent)/100.0, nil, message)
	}

	p.logger.Debug().
		Str("token", string(token)).
		Str("message", message).
		Int("percent", percent).
		Msg("Progress updated")
}

// CompleteStage implements core.ProgressReporter
// Marks a stage as completed (success or failure)
func (p *UnifiedProgressReporter) CompleteStage(token core.ProgressToken, success bool, message string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	state, exists := p.stages[token]
	if !exists {
		p.logger.Warn().Str("token", string(token)).Msg("Progress token not found for completion")
		return
	}

	// Update stage completion state
	if success {
		state.stage.Status = "completed"
		state.stage.Progress = 100
	} else {
		state.stage.Status = "failed"
	}
	state.stage.Message = message

	// GoMCP integration
	if p.serverCtx != nil {
		if success {
			p.serverCtx.SendProgress(1.0, nil, message)
			p.serverCtx.CompleteProgress(message)
		} else {
			p.serverCtx.SendProgress(float64(state.stage.Progress)/100.0, nil, message)
			p.serverCtx.CompleteProgress(fmt.Sprintf("Failed: %s", message))
		}
	}

	duration := time.Since(state.startTime)
	p.logger.Info().
		Str("token", string(token)).
		Bool("success", success).
		Dur("duration", duration).
		Str("message", message).
		Msg("Progress stage completed")

	// Clean up completed stages after a delay to allow clients to read final state
	go func() {
		time.Sleep(5 * time.Minute)
		p.mutex.Lock()
		delete(p.stages, token)
		p.mutex.Unlock()
	}()
}

// GetStageInfo provides access to current stage information (for testing/debugging)
func (p *UnifiedProgressReporter) GetStageInfo(token core.ProgressToken) (*core.ProgressStage, bool) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	state, exists := p.stages[token]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent external modifications
	stageCopy := *state.stage
	return &stageCopy, true
}

// GetActiveStages returns the number of currently active stages (for monitoring)
func (p *UnifiedProgressReporter) GetActiveStages() int {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return len(p.stages)
}

// ExecuteWithProgress is a convenience function that wraps tool execution with
// automatic progress tracking. This provides a simple way for atomic tools
// to integrate unified progress reporting.
func ExecuteWithProgress[TArgs any, TResult any](
	progress core.ProgressReporter,
	stageName string,
	executeFn func(ctx context.Context, args TArgs, progress core.ProgressReporter, token core.ProgressToken) (TResult, error),
	ctx context.Context,
	args TArgs,
) (TResult, error) {
	var result TResult

	// Start progress tracking
	token := progress.StartStage(stageName)

	// Execute the function with progress tracking
	result, err := executeFn(ctx, args, progress, token)

	// Complete progress tracking
	if err != nil {
		progress.CompleteStage(token, false, fmt.Sprintf("Operation failed: %v", err))
	} else {
		progress.CompleteStage(token, true, "Operation completed successfully")
	}

	return result, err
}

// MultiStageExecutor helps execute multi-stage operations with consistent progress reporting
type MultiStageExecutor struct {
	progress core.ProgressReporter
	stages   []StageDefinition
	tokens   []core.ProgressToken
}

// StageDefinition defines a stage in a multi-stage operation
type StageDefinition struct {
	Name        string
	Description string
	Weight      float64 // Relative weight (0.0-1.0) for progress calculation
}

// NewMultiStageExecutor creates a new multi-stage executor
func NewMultiStageExecutor(progress core.ProgressReporter, stages []StageDefinition) *MultiStageExecutor {
	return &MultiStageExecutor{
		progress: progress,
		stages:   stages,
		tokens:   make([]core.ProgressToken, len(stages)),
	}
}

// ExecuteStage executes a specific stage in the multi-stage operation
func (mse *MultiStageExecutor) ExecuteStage(stageIndex int, executeFn func() error) error {
	if stageIndex < 0 || stageIndex >= len(mse.stages) {
		return fmt.Errorf("invalid stage index: %d", stageIndex)
	}

	stage := mse.stages[stageIndex]
	token := mse.progress.StartStage(stage.Name)
	mse.tokens[stageIndex] = token

	// Execute the stage
	err := executeFn()

	// Complete the stage
	if err != nil {
		mse.progress.CompleteStage(token, false, fmt.Sprintf("Stage failed: %v", err))
		return err
	}

	mse.progress.CompleteStage(token, true, "Stage completed successfully")
	return nil
}

// UpdateStageProgress updates progress within a specific stage
func (mse *MultiStageExecutor) UpdateStageProgress(stageIndex int, message string, percent int) {
	if stageIndex < 0 || stageIndex >= len(mse.tokens) {
		return
	}

	token := mse.tokens[stageIndex]
	if token != "" {
		mse.progress.UpdateProgress(token, message, percent)
	}
}
