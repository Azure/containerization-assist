package runtime

import (
	"context"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/localrivet/gomcp/server"
)

// GoMCPProgressAdapter provides a bridge between the existing ProgressReporter interface
// and GoMCP's native progress tokens. This allows existing tools to use GoMCP progress
// without requiring extensive refactoring.
type GoMCPProgressAdapter struct {
	serverCtx *server.Context
	token     string
	stages    []mcptypes.ProgressStage
	current   int
}

// NewGoMCPProgressAdapter creates a progress adapter using GoMCP native progress tokens
func NewGoMCPProgressAdapter(serverCtx *server.Context, stages []mcptypes.ProgressStage) *GoMCPProgressAdapter {
	token := serverCtx.CreateProgressToken()

	return &GoMCPProgressAdapter{
		serverCtx: serverCtx,
		token:     token,
		stages:    stages,
		current:   0,
	}
}

// ReportStage implements mcptypes.InternalProgressReporter
func (a *GoMCPProgressAdapter) ReportStage(stageProgress float64, message string) {
	if a.token == "" {
		return
	}

	// Calculate overall progress based on current stage and stage progress
	var overallProgress float64
	for i := 0; i < a.current; i++ {
		overallProgress += a.stages[i].Weight
	}
	if a.current < len(a.stages) {
		overallProgress += a.stages[a.current].Weight * stageProgress
	}

	a.serverCtx.SendProgress(overallProgress, nil, message)
}

// NextStage implements mcptypes.InternalProgressReporter
func (a *GoMCPProgressAdapter) NextStage(message string) {
	if a.current < len(a.stages)-1 {
		a.current++
	}
	a.ReportStage(0.0, message)
}

// SetStage implements mcptypes.InternalProgressReporter
func (a *GoMCPProgressAdapter) SetStage(stageIndex int, message string) {
	if stageIndex >= 0 && stageIndex < len(a.stages) {
		a.current = stageIndex
	}
	a.ReportStage(0.0, message)
}

// ReportOverall implements mcptypes.InternalProgressReporter
func (a *GoMCPProgressAdapter) ReportOverall(progress float64, message string) {
	if a.token != "" {
		a.serverCtx.SendProgress(progress, nil, message)
	}
}

// GetCurrentStage implements mcptypes.InternalProgressReporter
func (a *GoMCPProgressAdapter) GetCurrentStage() (int, mcptypes.ProgressStage) {
	if a.current >= 0 && a.current < len(a.stages) {
		return a.current, a.stages[a.current]
	}
	return 0, mcptypes.ProgressStage{}
}

// Complete finalizes the progress tracking
func (a *GoMCPProgressAdapter) Complete(message string) {
	if a.token != "" {
		a.serverCtx.CompleteProgress(message)
	}
}

// ExecuteToolWithGoMCPProgress is a helper function that executes a tool's existing Execute method
// with GoMCP progress tracking by wrapping it with a progress adapter
func ExecuteToolWithGoMCPProgress[TArgs any, TResult any](
	serverCtx *server.Context,
	stages []mcptypes.ProgressStage,
	executeFn func(ctx context.Context, args TArgs, reporter mcptypes.InternalProgressReporter) (TResult, error),
	fallbackFn func(ctx context.Context, args TArgs) (TResult, error),
	args TArgs,
) (TResult, error) {
	ctx := context.Background()
	var result TResult
	var err error

	// Create progress adapter for GoMCP
	adapter := NewGoMCPProgressAdapter(serverCtx, stages)

	// Execute the function with progress tracking
	if executeFn != nil {
		result, err = executeFn(ctx, args, adapter)
	} else if fallbackFn != nil {
		result, err = fallbackFn(ctx, args)
	} else {
		var zero TResult
		return zero, types.NewRichError("INVALID_ARGUMENTS", "no execution function provided", "validation_error")
	}

	// Complete progress tracking
	if err != nil {
		adapter.Complete("Operation failed")
	} else {
		adapter.Complete("Operation completed successfully")
	}

	return result, err
}
