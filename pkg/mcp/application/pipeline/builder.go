package pipeline

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// Builder implements PipelineBuilder interface
type Builder struct {
	stages      []api.PipelineStage
	timeout     time.Duration
	retryPolicy *api.PipelineRetryPolicy
	metrics     api.MetricsCollector
}

// New creates a new pipeline builder
func New() api.PipelineBuilder {
	return &Builder{
		stages:  make([]api.PipelineStage, 0),
		timeout: 30 * time.Second,
	}
}

// New implements the PipelineBuilder interface method
func (b *Builder) New() api.Pipeline {
	return &Pipeline{
		stages:  make([]api.PipelineStage, 0),
		timeout: 30 * time.Second,
	}
}

// FromTemplate loads pipeline from template
func (b *Builder) FromTemplate(template string) api.Pipeline {
	// Implementation for template loading
	return b.Build()
}

// WithStages adds stages to pipeline
func (b *Builder) WithStages(stages ...api.PipelineStage) api.PipelineBuilder {
	b.stages = append(b.stages, stages...)
	return b
}

// Build creates the final pipeline
func (b *Builder) Build() api.Pipeline {
	return &Pipeline{
		stages:      b.stages,
		timeout:     b.timeout,
		retryPolicy: b.retryPolicy,
		metrics:     b.metrics,
	}
}

// Pipeline implements the Pipeline interface
type Pipeline struct {
	stages      []api.PipelineStage
	timeout     time.Duration
	retryPolicy *api.PipelineRetryPolicy
	metrics     api.MetricsCollector
}

// Execute runs the pipeline
func (p *Pipeline) Execute(ctx context.Context, request *api.PipelineRequest) (*api.PipelineResponse, error) {
	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// Execute stages sequentially
	var result interface{} = request.Input

	for _, stage := range p.stages {
		start := time.Now()

		var err error
		result, err = stage.Execute(ctx, result)

		// Record metrics if collector is available
		if p.metrics != nil {
			p.metrics.RecordStageExecution(stage.Name(), time.Since(start), err)
		}

		if err != nil {
			// Apply retry policy if configured
			if p.retryPolicy != nil {
				for attempt := 1; attempt <= p.retryPolicy.MaxAttempts; attempt++ {
					time.Sleep(p.retryPolicy.BackoffDuration)
					result, err = stage.Execute(ctx, result)
					if err == nil {
						break
					}
				}
			}

			if err != nil {
				return nil, errors.NewError().
					Code(errors.CodeToolExecutionFailed).
					Type(errors.ErrTypeOperation).
					Message("pipeline stage failed").
					Context("stage", stage.Name()).
					Cause(err).
					Build()
			}
		}
	}

	return &api.PipelineResponse{
		Output: result,
		Metadata: map[string]interface{}{
			"type":    "pipeline",
			"stages":  len(p.stages),
			"timeout": p.timeout.String(),
		},
	}, nil
}

// AddStage adds a stage to the pipeline
func (p *Pipeline) AddStage(stage api.PipelineStage) api.Pipeline {
	p.stages = append(p.stages, stage)
	return p
}

// WithTimeout sets pipeline timeout
func (p *Pipeline) WithTimeout(timeout time.Duration) api.Pipeline {
	p.timeout = timeout
	return p
}

// WithRetry sets retry policy
func (p *Pipeline) WithRetry(policy api.PipelineRetryPolicy) api.Pipeline {
	p.retryPolicy = &policy
	return p
}

// WithMetrics enables metrics collection
func (p *Pipeline) WithMetrics(collector api.MetricsCollector) api.Pipeline {
	p.metrics = collector
	return p
}
