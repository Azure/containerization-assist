package pipeline

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// OrchestrationPipeline implements full orchestration semantics
type OrchestrationPipeline struct {
	stages      []api.PipelineStage
	timeout     time.Duration
	retryPolicy *api.PipelineRetryPolicy
	metrics     api.MetricsCollector
}

// NewOrchestrationPipeline creates a new orchestration pipeline
func NewOrchestrationPipeline(stages ...api.PipelineStage) *OrchestrationPipeline {
	return &OrchestrationPipeline{
		stages:  stages,
		timeout: 30 * time.Second,
	}
}

// Execute runs pipeline with full orchestration
func (p *OrchestrationPipeline) Execute(ctx context.Context, request *api.PipelineRequest) (*api.PipelineResponse, error) {
	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// Execute stages with metrics
	var result interface{} = request.Input

	for _, stage := range p.stages {
		start := time.Now()

		var err error
		result, err = stage.Execute(ctx, result)

		// Record metrics
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
					Message("orchestration pipeline stage failed").
					Context("stage", stage.Name()).
					Cause(err).
					Build()
			}
		}
	}

	return &api.PipelineResponse{
		Output: result,
		Metadata: map[string]interface{}{
			"type":    "orchestration",
			"stages":  len(p.stages),
			"timeout": p.timeout.String(),
		},
	}, nil
}

// AddStage adds a stage to the orchestration pipeline
func (p *OrchestrationPipeline) AddStage(stage api.PipelineStage) api.Pipeline {
	p.stages = append(p.stages, stage)
	return p
}

// WithTimeout sets timeout
func (p *OrchestrationPipeline) WithTimeout(timeout time.Duration) api.Pipeline {
	p.timeout = timeout
	return p
}

// WithRetry sets retry policy
func (p *OrchestrationPipeline) WithRetry(policy api.PipelineRetryPolicy) api.Pipeline {
	p.retryPolicy = &policy
	return p
}

// WithMetrics enables metrics collection
func (p *OrchestrationPipeline) WithMetrics(collector api.MetricsCollector) api.Pipeline {
	p.metrics = collector
	return p
}
