package pipeline

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// ExamplePipelinePipeline implements ExamplePipeline pipeline
type ExamplePipelinePipeline struct {
	stages []api.PipelineStage
}

// NewExamplePipelinePipeline creates a new ExamplePipeline pipeline
func NewExamplePipelinePipeline(stages ...api.PipelineStage) *ExamplePipelinePipeline {
	return &ExamplePipelinePipeline{
		stages: stages,
	}
}

// Execute runs the ExamplePipeline pipeline
func (p *ExamplePipelinePipeline) Execute(ctx context.Context, request *api.PipelineRequest) (*api.PipelineResponse, error) {
	var result interface{} = request.Input

	for _, stage := range p.stages {
		var err error
		result, err = stage.Execute(ctx, result)
		if err != nil {
			return nil, errors.NewError().
				Code(errors.CodeToolExecutionFailed).
				Type(errors.ErrTypeOperation).
				Message("ExamplePipeline pipeline stage failed").
				Context("stage", stage.Name()).
				Cause(err).
				Build()
		}
	}

	return &api.PipelineResponse{
		Output: result,
		Metadata: map[string]interface{}{
			"type":   "ExamplePipeline",
			"stages": len(p.stages),
		},
	}, nil
}

// AddStage adds a stage to the pipeline
func (p *ExamplePipelinePipeline) AddStage(stage api.PipelineStage) api.Pipeline {
	p.stages = append(p.stages, stage)
	return p
}

// WithTimeout sets timeout
func (p *ExamplePipelinePipeline) WithTimeout(timeout time.Duration) api.Pipeline {
	return p
}

// WithRetry sets retry policy
func (p *ExamplePipelinePipeline) WithRetry(policy api.PipelineRetryPolicy) api.Pipeline {
	return p
}

// WithMetrics enables metrics collection
func (p *ExamplePipelinePipeline) WithMetrics(collector api.MetricsCollector) api.Pipeline {
	return p
}
