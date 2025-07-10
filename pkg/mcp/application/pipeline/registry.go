package pipeline

import (
	"context"
	"sync"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// StageRegistry manages pipeline stages
type StageRegistry struct {
	stages map[string]api.PipelineStage
	mu     sync.RWMutex
}

// NewStageRegistry creates a new stage registry
func NewStageRegistry() *StageRegistry {
	return &StageRegistry{
		stages: make(map[string]api.PipelineStage),
	}
}

// Register registers a pipeline stage
func (r *StageRegistry) Register(name string, stage api.PipelineStage) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.stages[name]; exists {
		return errors.NewError().
			Code(errors.CodeResourceAlreadyExists).
			Type(errors.ErrTypeValidation).
			Message("stage already registered").
			Context("stage", name).
			Build()
	}

	r.stages[name] = stage
	return nil
}

// Get retrieves a pipeline stage
func (r *StageRegistry) Get(name string) (api.PipelineStage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stage, exists := r.stages[name]
	if !exists {
		return nil, errors.NewError().
			Code(errors.CodeResourceNotFound).
			Type(errors.ErrTypeValidation).
			Message("stage not found").
			Context("stage", name).
			Build()
	}

	return stage, nil
}

// List returns all registered stages
func (r *StageRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stages := make([]string, 0, len(r.stages))
	for name := range r.stages {
		stages = append(stages, name)
	}
	return stages
}

// Common stage implementations

// ValidateStage implements validation stage
type ValidateStage struct {
	validator func(interface{}) error
}

// NewValidateStage creates a new validation stage
func NewValidateStage(validator func(interface{}) error) *ValidateStage {
	return &ValidateStage{
		validator: validator,
	}
}

// Name returns the stage name
func (s *ValidateStage) Name() string {
	return "validate"
}

// Execute executes the validation stage
func (s *ValidateStage) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	if err := s.validator(input); err != nil {
		return nil, err
	}
	return input, nil
}

// Validate validates the input
func (s *ValidateStage) Validate(input interface{}) error {
	return s.validator(input)
}

// TransformStage implements transformation stage
type TransformStage struct {
	transformer func(interface{}) (interface{}, error)
}

// NewTransformStage creates a new transformation stage
func NewTransformStage(transformer func(interface{}) (interface{}, error)) *TransformStage {
	return &TransformStage{
		transformer: transformer,
	}
}

// Name returns the stage name
func (s *TransformStage) Name() string {
	return "transform"
}

// Execute executes the transformation stage
func (s *TransformStage) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	return s.transformer(input)
}

// Validate validates the input for transformation
func (s *TransformStage) Validate(_ interface{}) error {
	// Basic validation - could be extended
	return nil
}
