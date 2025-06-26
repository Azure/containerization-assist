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
	SnapshotCompletions       bool
	GenerateReport            bool
	TargetDirectory           string
}

// StageConfig defines the configuration for a single stage in the pipeline runner.
type StageConfig struct {
	// Id is a unique arbitrary identifier for the stage.
	Id string

	// Stage represents the pipeline stage to be executed.
	Stage PipelineStage

	// MaxRetries specifies the maximum number of retries allowed for the stage.
	MaxRetries int

	// OnFailGoto specifies the ID of the stage to go to on failure.
	// If empty, the runner will exit after exceeding MaxRetries.
	OnFailGoto string

	// OnSuccessGoto specifies the ID of the stage to go to on success.
	// If empty, the runner will proceed to the next stage.
	OnSuccessGoto string

	// Path specifies the file system path associated with the stage.
	Path string
}

// Runner coordinates and executes a set of stages.
type Runner struct {
	stageConfigs []*StageConfig
	id2Stage     map[string]*StageConfig
	out          io.Writer
}
