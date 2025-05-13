package pipeline

import (
	"context"
	"io"
)

// PipelineStage defines a common interface for all pipeline types
type PipelineStage interface {
	// Initialize prepares the pipeline state with initial values
	Initialize(ctx context.Context, state *PipelineState, path string) error

	// Generate creates artifacts (Dockerfile or manifests)
	Generate(ctx context.Context, state *PipelineState, targetDir string) error

	// GetErrors returns pipeline-specific errors as a formatted string
	GetErrors(state *PipelineState) string

	// WriteSuccessfulFiles writes successful pipeline files to disk
	WriteSuccessfulFiles(state *PipelineState) error

	// Run executes the pipeline with iteration and error correction
	Run(ctx context.Context, state *PipelineState, clients interface{}, options RunnerOptions) error

	// Deploy handles the deployment of pipeline artifacts
	Deploy(ctx context.Context, state *PipelineState, clients interface{}) error
}

// RunnerOptions defines configuration options for a pipeline run
type RunnerOptions struct {
	MaxIterations             int //Maximum number of iterations per stage
	CompleteLoopMaxIterations int // Maximum times entire pipeline can be run
	GenerateSnapshot          bool
	TargetDirectory           string
}

// Runner coordinates and executes a set of stages.
type Runner struct {
	stages map[string]PipelineStage
	order  []string
	out    io.Writer
}
