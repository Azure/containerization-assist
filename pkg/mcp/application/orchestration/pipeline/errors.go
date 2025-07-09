package pipeline

import "errors"

// Common errors for pipeline services
var (
	// ErrWorkerNotFound indicates the requested worker was not found
	ErrWorkerNotFound = errors.New("worker not found")

	// ErrJobNotFound indicates the requested job was not found
	ErrJobNotFound = errors.New("job not found")

	// ErrPipelineNotRunning indicates an operation was attempted on a stopped pipeline
	ErrPipelineNotRunning = errors.New("pipeline is not running")

	// ErrPipelineAlreadyRunning indicates the pipeline is already started
	ErrPipelineAlreadyRunning = errors.New("pipeline is already running")
)
