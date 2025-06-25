package orchestration

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
)

// Engine is the main workflow engine that provides the public API
type Engine struct {
	logger      zerolog.Logger
	coordinator *Coordinator
}

// NewEngine creates a new workflow engine with all components
func NewEngine(
	logger zerolog.Logger,
	stageExecutor StageExecutor,
	sessionManager WorkflowSessionManager,
	dependencyResolver DependencyResolver,
	errorRouter ErrorRouter,
	checkpointManager CheckpointManager,
) *Engine {
	// Create state machine
	stateMachine := NewStateMachine(logger, sessionManager)

	// Create executor
	executor := NewExecutor(logger, stageExecutor, errorRouter, stateMachine)

	// Create coordinator
	coordinator := NewCoordinator(
		logger,
		stateMachine,
		executor,
		sessionManager,
		dependencyResolver,
		checkpointManager,
	)

	return &Engine{
		logger:      logger.With().Str("component", "workflow_engine").Logger(),
		coordinator: coordinator,
	}
}

// ExecuteWorkflow executes a workflow according to its specification
func (e *Engine) ExecuteWorkflow(
	ctx context.Context,
	workflowSpec *WorkflowSpec,
	options ...ExecutionOption,
) (*WorkflowResult, error) {
	// Apply execution options
	opts := &ExecutionOptions{
		EnableParallel: true, // Enable parallel execution by default
	}
	for _, opt := range options {
		opt(opts)
	}

	// Validate workflow specification
	if err := e.ValidateWorkflow(workflowSpec); err != nil {
		return nil, fmt.Errorf("invalid workflow specification: %w", err)
	}

	// Execute workflow
	return e.coordinator.ExecuteWorkflow(ctx, workflowSpec, opts)
}

// ValidateWorkflow validates a workflow specification without executing it
func (e *Engine) ValidateWorkflow(workflowSpec *WorkflowSpec) error {
	// Validate basic structure
	if workflowSpec.APIVersion == "" {
		return fmt.Errorf("apiVersion is required")
	}
	if workflowSpec.Kind != "Workflow" {
		return fmt.Errorf("kind must be 'Workflow'")
	}
	if workflowSpec.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}
	if len(workflowSpec.Spec.Stages) == 0 {
		return fmt.Errorf("at least one stage is required")
	}

	// Validate stages
	stageNames := make(map[string]bool)
	for _, stage := range workflowSpec.Spec.Stages {
		if stage.Name == "" {
			return fmt.Errorf("stage name is required")
		}
		if stageNames[stage.Name] {
			return fmt.Errorf("duplicate stage name: %s", stage.Name)
		}
		stageNames[stage.Name] = true

		if len(stage.Tools) == 0 {
			return fmt.Errorf("stage %s must specify at least one tool", stage.Name)
		}

		// Validate dependencies reference existing stages
		for _, dep := range stage.DependsOn {
			if !stageNames[dep] {
				// Check if it will be defined later
				found := false
				for _, futureStage := range workflowSpec.Spec.Stages {
					if futureStage.Name == dep {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("stage %s depends on unknown stage: %s", stage.Name, dep)
				}
			}
		}
	}

	return nil
}

// GetWorkflowStatus returns the current status of a workflow session
func (e *Engine) GetWorkflowStatus(sessionID string) (*WorkflowSession, error) {
	// Delegate to coordinator's session manager
	return e.coordinator.sessionManager.GetSession(sessionID)
}

// PauseWorkflow pauses a running workflow
func (e *Engine) PauseWorkflow(sessionID string) error {
	return e.coordinator.PauseWorkflow(sessionID)
}

// ResumeWorkflow resumes a paused workflow
func (e *Engine) ResumeWorkflow(ctx context.Context, sessionID string, workflowSpec *WorkflowSpec) (*WorkflowResult, error) {
	// Validate workflow specification
	if err := e.ValidateWorkflow(workflowSpec); err != nil {
		return nil, fmt.Errorf("invalid workflow specification: %w", err)
	}

	return e.coordinator.ResumeWorkflow(ctx, sessionID, workflowSpec)
}

// CancelWorkflow cancels a workflow execution
func (e *Engine) CancelWorkflow(sessionID string) error {
	return e.coordinator.CancelWorkflow(sessionID)
}

// ResumeFromStage resumes a workflow from a specific stage
func (e *Engine) ResumeFromStage(ctx context.Context, sessionID, stageName string, workflowSpec *WorkflowSpec) (*WorkflowResult, error) {
	// Validate workflow specification
	if err := e.ValidateWorkflow(workflowSpec); err != nil {
		return nil, fmt.Errorf("invalid workflow specification: %w", err)
	}

	return e.coordinator.ResumeFromStage(ctx, sessionID, stageName, workflowSpec)
}

// GetCheckpointHistory returns the checkpoint history for a workflow session
func (e *Engine) GetCheckpointHistory(sessionID string) ([]*WorkflowCheckpoint, error) {
	return e.coordinator.GetCheckpointHistory(sessionID)
}

// CreateCheckpoint manually creates a checkpoint for the current workflow state
func (e *Engine) CreateCheckpoint(sessionID, message string, workflowSpec *WorkflowSpec) (*WorkflowCheckpoint, error) {
	session, err := e.coordinator.sessionManager.GetSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return e.coordinator.checkpointManager.CreateCheckpoint(session, session.CurrentStage, message, workflowSpec)
}

// Execution option functions

// WithSessionID sets the session ID for resuming an existing workflow
func WithSessionID(sessionID string) ExecutionOption {
	return func(opts *ExecutionOptions) {
		opts.SessionID = sessionID
	}
}

// WithResumeFromCheckpoint resumes workflow from a specific checkpoint
func WithResumeFromCheckpoint(sessionID, checkpointID string) ExecutionOption {
	return func(opts *ExecutionOptions) {
		opts.SessionID = sessionID
		opts.ResumeFromCheckpoint = checkpointID
	}
}

// WithVariables sets additional variables for the workflow
func WithVariables(variables map[string]string) ExecutionOption {
	return func(opts *ExecutionOptions) {
		opts.Variables = variables
	}
}

// WithTimeout sets the overall workflow timeout
func WithTimeout(timeout time.Duration) ExecutionOption {
	return func(opts *ExecutionOptions) {
		opts.Timeout = timeout
	}
}

// WithStageTimeout sets the default timeout for stages
func WithStageTimeout(timeout time.Duration) ExecutionOption {
	return func(opts *ExecutionOptions) {
		opts.StageTimeout = timeout
	}
}

// WithDryRun enables dry run mode (validation without execution)
func WithDryRun(dryRun bool) ExecutionOption {
	return func(opts *ExecutionOptions) {
		opts.DryRun = dryRun
	}
}

// WithCreateCheckpoints enables checkpoint creation after each stage group
func WithCreateCheckpoints(createCheckpoints bool) ExecutionOption {
	return func(opts *ExecutionOptions) {
		opts.CreateCheckpoints = createCheckpoints
	}
}

// WithEnableParallel enables or disables parallel execution of workflow stages
func WithEnableParallel(enableParallel bool) ExecutionOption {
	return func(opts *ExecutionOptions) {
		opts.EnableParallel = enableParallel
	}
}

// WithConcurrencyConfig sets the concurrency configuration for workflow execution
func WithConcurrencyConfig(config *ConcurrencyConfig) ExecutionOption {
	return func(opts *ExecutionOptions) {
		opts.ConcurrencyConfig = config
	}
}

// WithMaxParallelStages sets the maximum number of stages that can execute in parallel
func WithMaxParallelStages(maxParallel int) ExecutionOption {
	return func(opts *ExecutionOptions) {
		if opts.ConcurrencyConfig == nil {
			opts.ConcurrencyConfig = &ConcurrencyConfig{}
		}
		opts.ConcurrencyConfig.MaxParallelStages = maxParallel
	}
}
