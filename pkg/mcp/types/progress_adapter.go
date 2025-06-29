package types

import (
	"github.com/localrivet/gomcp/server"
)

// Local type definitions to avoid import cycles

// LocalProgressStage represents a stage in progress tracking (local copy to avoid import cycle)
type LocalProgressStage struct {
	Name        string
	Weight      float64
	Description string
}

// LocalProgressReporter provides progress reporting (local interface to avoid import cycles)
type LocalProgressReporter interface {
	ReportStage(stageProgress float64, message string)
	NextStage(message string)
	SetStage(stageIndex int, message string)
	ReportOverall(progress float64, message string)
	GetCurrentStage() (int, LocalProgressStage)
}

// NOTE: LocalProgressStage is defined in interfaces.go to avoid duplication

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
	var weightedProgress float64
	for i := 0; i < a.current; i++ {
		weightedProgress += a.stages[i].Weight
	}
	if a.current < len(a.stages) {
		weightedProgress += a.stages[a.current].Weight * stageProgress
	}

	// Report progress to GoMCP
	// TODO: ReportProgress method needs to be implemented on server.Context
	// a.serverCtx.ReportProgress(a.token, int(weightedProgress*100), message)
}

// NextStage implements mcptypes.ProgressReporter
func (a *GoMCPProgressAdapter) NextStage(message string) {
	if a.current < len(a.stages)-1 {
		a.current++
		a.ReportStage(0, message)
	}
}

// SetStage implements mcptypes.ProgressReporter
func (a *GoMCPProgressAdapter) SetStage(stageIndex int, message string) {
	if stageIndex >= 0 && stageIndex < len(a.stages) {
		a.current = stageIndex
		a.ReportStage(0, message)
	}
}

// ReportOverall implements mcptypes.ProgressReporter
func (a *GoMCPProgressAdapter) ReportOverall(progress float64, message string) {
	if a.token != "" {
		// TODO: ReportProgress method needs to be implemented on server.Context
		// a.serverCtx.ReportProgress(a.token, int(progress*100), message)
	}
}

// GetCurrentStage implements mcptypes.ProgressReporter
func (a *GoMCPProgressAdapter) GetCurrentStage() (int, LocalProgressStage) {
	if a.current < len(a.stages) {
		return a.current, a.stages[a.current]
	}
	return -1, LocalProgressStage{}
}
