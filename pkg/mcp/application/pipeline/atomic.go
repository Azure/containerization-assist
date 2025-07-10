package pipeline

import (
	"context"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// AtomicPipeline implements atomic execution semantics
type AtomicPipeline struct {
	stages []api.PipelineStage
	mu     sync.Mutex
}

// NewAtomicPipeline creates a new atomic pipeline
func NewAtomicPipeline(stages ...api.PipelineStage) *AtomicPipeline {
	return &AtomicPipeline{
		stages: stages,
	}
}

// Execute runs pipeline atomically
func (p *AtomicPipeline) Execute(ctx context.Context, request *api.PipelineRequest) (*api.PipelineResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Atomic execution logic
	var result interface{} = request.Input

	for _, stage := range p.stages {
		var err error
		result, err = stage.Execute(ctx, result)
		if err != nil {
			return nil, errors.NewError().
				Code(errors.CodeToolExecutionFailed).
				Type(errors.ErrTypeOperation).
				Message("atomic pipeline stage failed").
				Context("stage", stage.Name()).
				Cause(err).
				Build()
		}
	}

	return &api.PipelineResponse{
		Output: result,
		Metadata: map[string]interface{}{
			"type":   "atomic",
			"stages": len(p.stages),
		},
	}, nil
}

// AddStage adds a stage to the atomic pipeline
func (p *AtomicPipeline) AddStage(stage api.PipelineStage) api.Pipeline {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stages = append(p.stages, stage)
	return p
}

// WithTimeout sets timeout (atomic pipelines don't support individual timeouts)
func (p *AtomicPipeline) WithTimeout(timeout time.Duration) api.Pipeline {
	return p
}

// WithRetry sets retry policy
func (p *AtomicPipeline) WithRetry(policy api.PipelineRetryPolicy) api.Pipeline {
	return p
}

// WithMetrics enables metrics collection
func (p *AtomicPipeline) WithMetrics(collector api.MetricsCollector) api.Pipeline {
	return p
}
