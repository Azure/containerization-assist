package internal

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/localrivet/gomcp/server"
)

// LocalProgressReporter provides progress reporting (local interface to avoid import cycles)
type LocalProgressReporter interface {
	ReportStage(stageProgress float64, message string)
	NextStage(message string)
	SetStage(stageIndex int, message string)
	ReportOverall(progress float64, message string)
	GetCurrentStage() (int, LocalProgressStage)
}

// LocalProgressStage represents a stage in a multi-step operation (local type to avoid import cycles)
type LocalProgressStage struct {
	Name        string  // Human-readable stage name
	Weight      float64 // Relative weight (0.0-1.0) of this stage in overall progress
	Description string  // Optional detailed description
}

// GoMCPProgressAdapter provides a bridge between the existing ProgressReporter interface
// and GoMCP's native progress tokens. This allows existing tools to use GoMCP progress
// without requiring extensive refactoring.
type GoMCPProgressAdapter struct {
	serverCtx *server.Context
	token     string
	stages    []LocalProgressStage
	current   int
}

// NewGoMCPProgressAdapter creates a progress adapter using GoMCP native progress tokens
func NewGoMCPProgressAdapter(serverCtx *server.Context, stages []LocalProgressStage) *GoMCPProgressAdapter {
	token := serverCtx.CreateProgressToken()

	return &GoMCPProgressAdapter{
		serverCtx: serverCtx,
		token:     token,
		stages:    stages,
		current:   0,
	}
}

// ReportStage implements mcptypes.ProgressReporter
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

// NextStage implements mcptypes.ProgressReporter
func (a *GoMCPProgressAdapter) NextStage(message string) {
	if a.current < len(a.stages)-1 {
		a.current++
	}
	a.ReportStage(0.0, message)
}

// SetStage implements mcptypes.ProgressReporter
func (a *GoMCPProgressAdapter) SetStage(stageIndex int, message string) {
	if stageIndex >= 0 && stageIndex < len(a.stages) {
		a.current = stageIndex
	}
	a.ReportStage(0.0, message)
}

// ReportOverall implements mcptypes.ProgressReporter
func (a *GoMCPProgressAdapter) ReportOverall(progress float64, message string) {
	if a.token != "" {
		a.serverCtx.SendProgress(progress, nil, message)
	}
}

// GetCurrentStage implements mcptypes.ProgressReporter
func (a *GoMCPProgressAdapter) GetCurrentStage() (int, LocalProgressStage) {
	if a.current >= 0 && a.current < len(a.stages) {
		return a.current, a.stages[a.current]
	}
	return 0, LocalProgressStage{}
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
	stages []LocalProgressStage,
	executeFn func(ctx context.Context, args TArgs, reporter LocalProgressReporter) (TResult, error),
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
