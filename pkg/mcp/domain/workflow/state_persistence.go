// Package workflow provides state persistence for workflow execution
package workflow

import (
	"time"

	"log/slog"
)

// WorkflowCheckpoint represents a saved state of workflow execution
type WorkflowCheckpoint struct {
	WorkflowID     string                    `json:"workflow_id"`
	Timestamp      time.Time                 `json:"timestamp"`
	CurrentStep    int                       `json:"current_step"`
	TotalSteps     int                       `json:"total_steps"`
	Args           ContainerizeAndDeployArgs `json:"args"`
	State          map[string]interface{}    `json:"state"`
	Errors         []string                  `json:"errors"`
	Warnings       []string                  `json:"warnings"`
	CompletedSteps []string                  `json:"completed_steps"`
}

// StatePersistence is now just a wrapper around the StateStore interface
type StatePersistence struct {
	store  StateStore
	logger *slog.Logger
}

// NewStatePersistence creates a new state persistence handler using the provided StateStore
func NewStatePersistence(store StateStore, logger *slog.Logger) *StatePersistence {
	return &StatePersistence{
		store:  store,
		logger: logger.With("component", "state-persistence"),
	}
}

// SaveCheckpoint saves the current workflow state using the underlying StateStore
func (sp *StatePersistence) SaveCheckpoint(checkpoint *WorkflowCheckpoint) error {
	return sp.store.SaveCheckpoint(checkpoint)
}

// LoadLatestCheckpoint loads the most recent checkpoint for a workflow using the underlying StateStore
func (sp *StatePersistence) LoadLatestCheckpoint(workflowID string) (*WorkflowCheckpoint, error) {
	return sp.store.LoadLatestCheckpoint(workflowID)
}

// CleanupOldCheckpoints removes checkpoints older than the specified duration using the underlying StateStore
func (sp *StatePersistence) CleanupOldCheckpoints(maxAge time.Duration) error {
	return sp.store.CleanupOldCheckpoints(maxAge)
}

// WorkflowStateManager manages workflow state during execution
type WorkflowStateManager struct {
	persistence *StatePersistence
	workflowID  string
	args        ContainerizeAndDeployArgs
	state       map[string]interface{}
	errors      []string
	warnings    []string
	completed   []string
	currentStep int
	totalSteps  int
	logger      *slog.Logger
}

// NewWorkflowStateManager creates a new workflow state manager
func NewWorkflowStateManager(workflowID string, args ContainerizeAndDeployArgs, totalSteps int, persistence *StatePersistence, logger *slog.Logger) *WorkflowStateManager {
	return &WorkflowStateManager{
		persistence: persistence,
		workflowID:  workflowID,
		args:        args,
		state:       make(map[string]interface{}),
		errors:      []string{},
		warnings:    []string{},
		completed:   []string{},
		currentStep: 0,
		totalSteps:  totalSteps,
		logger:      logger.With("component", "state-manager", "workflow_id", workflowID),
	}
}

// SaveState saves the current workflow state
func (sm *WorkflowStateManager) SaveState(stepName string) error {
	checkpoint := &WorkflowCheckpoint{
		WorkflowID:     sm.workflowID,
		Timestamp:      time.Now(),
		CurrentStep:    sm.currentStep,
		TotalSteps:     sm.totalSteps,
		Args:           sm.args,
		State:          sm.state,
		Errors:         sm.errors,
		Warnings:       sm.warnings,
		CompletedSteps: sm.completed,
	}

	return sm.persistence.SaveCheckpoint(checkpoint)
}

// SetStepCompleted marks a step as completed
func (sm *WorkflowStateManager) SetStepCompleted(stepName string) {
	sm.completed = append(sm.completed, stepName)
	sm.currentStep++
}

// AddError adds an error to the state
func (sm *WorkflowStateManager) AddError(err string) {
	sm.errors = append(sm.errors, err)
}

// AddWarning adds a warning to the state
func (sm *WorkflowStateManager) AddWarning(warning string) {
	sm.warnings = append(sm.warnings, warning)
}

// SetState sets a state value
func (sm *WorkflowStateManager) SetState(key string, value interface{}) {
	sm.state[key] = value
}

// GetState retrieves a state value
func (sm *WorkflowStateManager) GetState(key string) (interface{}, bool) {
	val, ok := sm.state[key]
	return val, ok
}

// CanResumeFrom checks if workflow can be resumed from a checkpoint
func (sm *WorkflowStateManager) CanResumeFrom(checkpoint *WorkflowCheckpoint) bool {
	// Check if args match (excluding test mode flag)
	if checkpoint.Args.RepoURL != sm.args.RepoURL ||
		checkpoint.Args.Branch != sm.args.Branch {
		return false
	}

	// Check if workflow is incomplete
	return checkpoint.CurrentStep < checkpoint.TotalSteps
}
