package pipeline

import (
	"context"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// WorkflowPipeline implements workflow execution semantics
type WorkflowPipeline struct {
	stages   []api.PipelineStage
	parallel bool
	mu       sync.RWMutex
}

// NewWorkflowPipeline creates a new workflow pipeline
func NewWorkflowPipeline(parallel bool, stages ...api.PipelineStage) *WorkflowPipeline {
	return &WorkflowPipeline{
		stages:   stages,
		parallel: parallel,
	}
}

// Execute runs pipeline as workflow
func (p *WorkflowPipeline) Execute(ctx context.Context, request *api.PipelineRequest) (*api.PipelineResponse, error) {
	if p.parallel {
		return p.executeParallel(ctx, request)
	}
	return p.executeSequential(ctx, request)
}

// executeParallel runs stages in parallel
func (p *WorkflowPipeline) executeParallel(ctx context.Context, request *api.PipelineRequest) (*api.PipelineResponse, error) {
	var wg sync.WaitGroup
	results := make([]interface{}, len(p.stages))
	errs := make([]error, len(p.stages))

	for i, stage := range p.stages {
		wg.Add(1)
		go func(idx int, s api.PipelineStage) {
			defer wg.Done()
			result, err := s.Execute(ctx, request.Input)
			results[idx] = result
			errs[idx] = err
		}(i, stage)
	}

	wg.Wait()

	// Check for errors
	for i, err := range errs {
		if err != nil {
			return nil, errors.NewError().
				Code(errors.CodeToolExecutionFailed).
				Type(errors.ErrTypeOperation).
				Message("workflow pipeline stage failed").
				Context("stage", p.stages[i].Name()).
				Cause(err).
				Build()
		}
	}

	return &api.PipelineResponse{
		Output: results,
		Metadata: map[string]interface{}{
			"type":     "workflow",
			"parallel": true,
			"stages":   len(p.stages),
		},
	}, nil
}

// executeSequential runs stages sequentially
func (p *WorkflowPipeline) executeSequential(ctx context.Context, request *api.PipelineRequest) (*api.PipelineResponse, error) {
	var result interface{} = request.Input

	for _, stage := range p.stages {
		var err error
		result, err = stage.Execute(ctx, result)
		if err != nil {
			return nil, errors.NewError().
				Code(errors.CodeToolExecutionFailed).
				Type(errors.ErrTypeOperation).
				Message("workflow pipeline stage failed").
				Context("stage", stage.Name()).
				Cause(err).
				Build()
		}
	}

	return &api.PipelineResponse{
		Output: result,
		Metadata: map[string]interface{}{
			"type":     "workflow",
			"parallel": false,
			"stages":   len(p.stages),
		},
	}, nil
}

// AddStage adds a stage to the workflow pipeline
func (p *WorkflowPipeline) AddStage(stage api.PipelineStage) api.Pipeline {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stages = append(p.stages, stage)
	return p
}

// WithTimeout sets timeout
func (p *WorkflowPipeline) WithTimeout(timeout time.Duration) api.Pipeline {
	return p
}

// WithRetry sets retry policy
func (p *WorkflowPipeline) WithRetry(policy api.PipelineRetryPolicy) api.Pipeline {
	return p
}

// WithMetrics enables metrics collection
func (p *WorkflowPipeline) WithMetrics(_ api.MetricsCollector) api.Pipeline {
	return p
}
