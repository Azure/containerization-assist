package pipeline

import (
	"context"
)

// Lifecycle handles the start/stop lifecycle of the pipeline
type Lifecycle interface {
	// Start initializes and starts the pipeline
	Start(ctx context.Context) error

	// Stop gracefully shuts down the pipeline
	Stop(ctx context.Context) error

	// IsRunning returns whether the pipeline is currently running
	IsRunning() bool
}

// pipelineLifecycle implements Lifecycle
type pipelineLifecycle struct {
	service Service
}

// NewPipelineLifecycle creates a new Lifecycle service
func NewPipelineLifecycle(service Service) Lifecycle {
	return &pipelineLifecycle{
		service: service,
	}
}

func (p *pipelineLifecycle) Start(_ context.Context) error {
	return p.service.Start()
}

func (p *pipelineLifecycle) Stop(_ context.Context) error {
	return p.service.Stop()
}

func (p *pipelineLifecycle) IsRunning() bool {
	return p.service.IsRunning()
}
