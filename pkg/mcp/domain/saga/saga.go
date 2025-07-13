// Package saga provides distributed transaction coordination for Container Kit workflows.
package saga

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/common/errors"
)

// SagaState represents the current state of a saga transaction
type SagaState string

const (
	SagaStateStarted     SagaState = "started"
	SagaStateInProgress  SagaState = "in_progress"
	SagaStateCompleted   SagaState = "completed"
	SagaStateFailed      SagaState = "failed"
	SagaStateCompensated SagaState = "compensated"
	SagaStateAborted     SagaState = "aborted"
)

// SagaStep represents a step in a saga transaction
type SagaStep interface {
	// Name returns the step name
	Name() string

	// Execute performs the forward action
	Execute(ctx context.Context, data map[string]interface{}) error

	// Compensate performs the rollback action
	Compensate(ctx context.Context, data map[string]interface{}) error

	// CanCompensate returns true if this step can be compensated
	CanCompensate() bool
}

// SagaStepResult represents the result of executing a saga step
type SagaStepResult struct {
	StepName  string                 `json:"step_name"`
	Success   bool                   `json:"success"`
	Error     string                 `json:"error,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Duration  time.Duration          `json:"duration"`
	Timestamp time.Time              `json:"timestamp"`
}

// SagaExecution represents a running saga transaction
type SagaExecution struct {
	ID               string                 `json:"id"`
	WorkflowID       string                 `json:"workflow_id"`
	State            SagaState              `json:"state"`
	Steps            []SagaStep             `json:"-"` // Not serialized
	ExecutedSteps    []SagaStepResult       `json:"executed_steps"`
	CompensatedSteps []SagaStepResult       `json:"compensated_steps"`
	Data             map[string]interface{} `json:"data"`
	StartTime        time.Time              `json:"start_time"`
	EndTime          *time.Time             `json:"end_time,omitempty"`
	Error            string                 `json:"error,omitempty"`
	logger           *slog.Logger
}

// NewSagaExecution creates a new saga execution
func NewSagaExecution(id, workflowID string, steps []SagaStep, logger *slog.Logger) *SagaExecution {
	return &SagaExecution{
		ID:               id,
		WorkflowID:       workflowID,
		State:            SagaStateStarted,
		Steps:            steps,
		ExecutedSteps:    make([]SagaStepResult, 0),
		CompensatedSteps: make([]SagaStepResult, 0),
		Data:             make(map[string]interface{}),
		StartTime:        time.Now(),
		logger:           logger.With("saga_id", id, "workflow_id", workflowID),
	}
}

// Execute runs the saga transaction
func (s *SagaExecution) Execute(ctx context.Context) error {
	s.State = SagaStateInProgress
	s.logger.Info("Starting saga execution", "steps_count", len(s.Steps))

	for i, step := range s.Steps {
		stepStartTime := time.Now()
		s.logger.Info("Executing saga step",
			"step", i+1,
			"step_name", step.Name(),
			"total_steps", len(s.Steps))

		err := step.Execute(ctx, s.Data)
		duration := time.Since(stepStartTime)

		stepResult := SagaStepResult{
			StepName:  step.Name(),
			Success:   err == nil,
			Duration:  duration,
			Timestamp: stepStartTime,
		}

		if err != nil {
			stepResult.Error = err.Error()
			s.ExecutedSteps = append(s.ExecutedSteps, stepResult)

			s.logger.Error("Saga step failed",
				"step_name", step.Name(),
				"error", err,
				"duration", duration)

			// Start compensation for all executed steps
			compensationErr := s.compensate(ctx)
			if compensationErr != nil {
				s.State = SagaStateAborted
				s.Error = fmt.Sprintf("step failed: %v, compensation failed: %v", err, compensationErr)
				return errors.New(errors.CodeOperationFailed, "saga", "saga transaction failed and compensation failed", compensationErr).
					With("step", step.Name()).
					With("step_error", err.Error())
			}

			s.State = SagaStateCompensated
			s.Error = err.Error()
			endTime := time.Now()
			s.EndTime = &endTime

			return errors.New(errors.CodeOperationFailed, "saga", "saga transaction failed but was successfully compensated", err).
				With("step", step.Name())
		}

		s.ExecutedSteps = append(s.ExecutedSteps, stepResult)
		s.logger.Info("Saga step completed successfully",
			"step_name", step.Name(),
			"duration", duration)
	}

	s.State = SagaStateCompleted
	endTime := time.Now()
	s.EndTime = &endTime

	s.logger.Info("Saga execution completed successfully",
		"duration", time.Since(s.StartTime),
		"steps_executed", len(s.ExecutedSteps))

	return nil
}

// compensate executes compensation for all executed steps in reverse order
func (s *SagaExecution) compensate(ctx context.Context) error {
	s.logger.Info("Starting saga compensation", "steps_to_compensate", len(s.ExecutedSteps))

	// Compensate in reverse order
	for i := len(s.ExecutedSteps) - 1; i >= 0; i-- {
		stepResult := s.ExecutedSteps[i]
		step := s.findStepByName(stepResult.StepName)

		if step == nil {
			s.logger.Error("Cannot find step for compensation", "step_name", stepResult.StepName)
			continue
		}

		if !step.CanCompensate() {
			s.logger.Warn("Step cannot be compensated", "step_name", step.Name())
			continue
		}

		compensationStartTime := time.Now()
		s.logger.Info("Compensating saga step", "step_name", step.Name())

		err := step.Compensate(ctx, s.Data)
		duration := time.Since(compensationStartTime)

		compensationResult := SagaStepResult{
			StepName:  step.Name(),
			Success:   err == nil,
			Duration:  duration,
			Timestamp: compensationStartTime,
		}

		if err != nil {
			compensationResult.Error = err.Error()
			s.logger.Error("Saga step compensation failed",
				"step_name", step.Name(),
				"error", err,
				"duration", duration)

			s.CompensatedSteps = append(s.CompensatedSteps, compensationResult)
			return fmt.Errorf("compensation failed for step %s: %w", step.Name(), err)
		}

		s.CompensatedSteps = append(s.CompensatedSteps, compensationResult)
		s.logger.Info("Saga step compensated successfully",
			"step_name", step.Name(),
			"duration", duration)
	}

	s.logger.Info("Saga compensation completed", "steps_compensated", len(s.CompensatedSteps))
	return nil
}

// findStepByName finds a step by its name
func (s *SagaExecution) findStepByName(name string) SagaStep {
	for _, step := range s.Steps {
		if step.Name() == name {
			return step
		}
	}
	return nil
}

// GetState returns the current saga state
func (s *SagaExecution) GetState() SagaState {
	return s.State
}

// GetExecutedSteps returns the list of executed steps
func (s *SagaExecution) GetExecutedSteps() []SagaStepResult {
	return s.ExecutedSteps
}

// GetCompensatedSteps returns the list of compensated steps
func (s *SagaExecution) GetCompensatedSteps() []SagaStepResult {
	return s.CompensatedSteps
}

// IsCompleted returns true if the saga completed successfully
func (s *SagaExecution) IsCompleted() bool {
	return s.State == SagaStateCompleted
}

// IsFailed returns true if the saga failed
func (s *SagaExecution) IsFailed() bool {
	return s.State == SagaStateFailed || s.State == SagaStateAborted
}

// IsCompensated returns true if the saga was compensated
func (s *SagaExecution) IsCompensated() bool {
	return s.State == SagaStateCompensated
}

// GetDuration returns the total execution duration
func (s *SagaExecution) GetDuration() time.Duration {
	if s.EndTime != nil {
		return s.EndTime.Sub(s.StartTime)
	}
	return time.Since(s.StartTime)
}
