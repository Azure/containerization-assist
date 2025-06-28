package workflow

import (
	"context"
	"fmt"
	"time"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// Interface definitions for workflow components

// StateMachine manages workflow state transitions
type StateMachine interface {
	TransitionState(session *WorkflowSession, status WorkflowStatus) error
	IsTerminalState(status WorkflowStatus) bool
}

// Executor executes workflow stages
type Executor interface {
	ExecuteStageGroup(ctx context.Context, stages []WorkflowStage, session *WorkflowSession, spec *WorkflowSpec, enableParallel bool) ([]StageResult, error)
}

// WorkflowSessionManager manages workflow sessions
type WorkflowSessionManager interface {
	CreateSession(spec *WorkflowSpec) (*WorkflowSession, error)
	GetSession(sessionID string) (*WorkflowSession, error)
	UpdateSession(session *WorkflowSession) error
}

// DependencyResolver resolves stage dependencies
type DependencyResolver interface {
	ResolveDependencies(stages []WorkflowStage) ([][]WorkflowStage, error)
}

// CheckpointManager manages workflow checkpoints
type CheckpointManager interface {
	CreateCheckpoint(session *WorkflowSession, stageID string, description string, spec *WorkflowSpec) (*WorkflowCheckpoint, error)
	RestoreFromCheckpoint(sessionID string, checkpointID string) (*WorkflowSession, error)
	ListCheckpoints(sessionID string) ([]*WorkflowCheckpoint, error)
}

// Workflow type aliases (referencing orchestration types)
type WorkflowSession struct {
	ID               string                 `json:"id"`
	WorkflowID       string                 `json:"workflow_id"`
	WorkflowName     string                 `json:"workflow_name"`
	Status           WorkflowStatus         `json:"status"`
	CurrentStage     string                 `json:"current_stage"`
	CompletedStages  []string               `json:"completed_stages"`
	FailedStages     []string               `json:"failed_stages"`
	SkippedStages    []string               `json:"skipped_stages"`
	SharedContext    map[string]interface{} `json:"shared_context"`
	ResourceBindings map[string]interface{} `json:"resource_bindings"`
	StageResults     map[string]interface{} `json:"stage_results"`
	LastActivity     time.Time              `json:"last_activity"`
	StartTime        time.Time              `json:"start_time"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
	Checkpoints      []WorkflowCheckpoint   `json:"checkpoints"`
	ErrorContext     *WorkflowErrorContext  `json:"error_context,omitempty"`
}

type WorkflowStatus string

const (
	WorkflowStatusPending   WorkflowStatus = "pending"
	WorkflowStatusRunning   WorkflowStatus = "running"
	WorkflowStatusCompleted WorkflowStatus = "completed"
	WorkflowStatusFailed    WorkflowStatus = "failed"
	WorkflowStatusPaused    WorkflowStatus = "paused"
	WorkflowStatusCancelled WorkflowStatus = "cancelled"
)

type WorkflowStage struct {
	Name      string                 `json:"name"`
	Type      string                 `json:"type"`
	Tools     []string               `json:"tools"`
	DependsOn []string               `json:"depends_on"`
	Variables map[string]interface{} `json:"variables"`
}

type WorkflowSpec struct {
	Metadata WorkflowMetadata   `json:"metadata"`
	Spec     WorkflowDefinition `json:"spec"`
}

type WorkflowMetadata struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type WorkflowDefinition struct {
	Stages      []WorkflowStage        `json:"stages"`
	Variables   map[string]interface{} `json:"variables"`
	ErrorPolicy ErrorPolicy            `json:"error_policy"`
}

type ErrorPolicy struct {
	Mode string `json:"mode"`
}

type WorkflowCheckpoint struct {
	ID      string    `json:"id"`
	StageID string    `json:"stage_id"`
	Created time.Time `json:"created"`
}

type StageResult struct {
	StageName string                 `json:"stage_name"`
	Success   bool                   `json:"success"`
	Results   map[string]interface{} `json:"results"`
	Duration  time.Duration          `json:"duration"`
	Artifacts []WorkflowArtifact     `json:"artifacts"`
}

type WorkflowArtifact struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type WorkflowResult struct {
	WorkflowID      string                 `json:"workflow_id"`
	SessionID       string                 `json:"session_id"`
	Status          WorkflowStatus         `json:"status"`
	Success         bool                   `json:"success"`
	Message         string                 `json:"message"`
	Duration        time.Duration          `json:"duration"`
	Results         map[string]interface{} `json:"results"`
	Artifacts       []WorkflowArtifact     `json:"artifacts"`
	StagesExecuted  int                    `json:"stages_executed"`
	StagesCompleted int                    `json:"stages_completed"`
	StagesFailed    int                    `json:"stages_failed"`
	Metrics         WorkflowMetrics        `json:"metrics"`
	ErrorSummary    *WorkflowErrorSummary  `json:"error_summary,omitempty"`
}

type WorkflowMetrics struct {
	TotalDuration       time.Duration            `json:"total_duration"`
	StageDurations      map[string]time.Duration `json:"stage_durations"`
	ToolExecutionCounts map[string]int           `json:"tool_execution_counts"`
}

type WorkflowErrorContext struct {
	ErrorHistory []WorkflowError `json:"error_history"`
	RetryCount   int             `json:"retry_count"`
	LastError    string          `json:"last_error"`
}

type WorkflowError struct {
	StageName string `json:"stage_name"`
	ErrorType string `json:"error_type"`
	Severity  string `json:"severity"`
	Retryable bool   `json:"retryable"`
}

type WorkflowErrorSummary struct {
	TotalErrors       int            `json:"total_errors"`
	CriticalErrors    int            `json:"critical_errors"`
	RecoverableErrors int            `json:"recoverable_errors"`
	ErrorsByType      map[string]int `json:"errors_by_type"`
	ErrorsByStage     map[string]int `json:"errors_by_stage"`
	RetryAttempts     int            `json:"retry_attempts"`
	LastError         string         `json:"last_error"`
	Recommendations   []string       `json:"recommendations"`
}

type ExecutionOptions struct {
	SessionID            string                 `json:"session_id"`
	ResumeFromCheckpoint string                 `json:"resume_from_checkpoint"`
	EnableParallel       bool                   `json:"enable_parallel"`
	CreateCheckpoints    bool                   `json:"create_checkpoints"`
	Variables            map[string]interface{} `json:"variables"`
}

// Coordinator orchestrates workflow execution by coordinating between state machine and executor
type Coordinator struct {
	logger             zerolog.Logger
	stateMachine       StateMachine
	executor           Executor
	sessionManager     WorkflowSessionManager
	dependencyResolver DependencyResolver
	checkpointManager  CheckpointManager
}

// NewCoordinator creates a new workflow coordinator
func NewCoordinator(
	logger zerolog.Logger,
	stateMachine StateMachine,
	executor Executor,
	sessionManager WorkflowSessionManager,
	dependencyResolver DependencyResolver,
	checkpointManager CheckpointManager,
) *Coordinator {
	return &Coordinator{
		logger:             logger.With().Str("component", "workflow_coordinator").Logger(),
		stateMachine:       stateMachine,
		executor:           executor,
		sessionManager:     sessionManager,
		dependencyResolver: dependencyResolver,
		checkpointManager:  checkpointManager,
	}
}

// ExecuteWorkflow executes a complete workflow
func (c *Coordinator) ExecuteWorkflow(
	ctx context.Context,
	workflowSpec *WorkflowSpec,
	options *ExecutionOptions,
) (*WorkflowResult, error) {
	// Initialize or restore session
	session, err := c.initializeSession(workflowSpec, options)
	if err != nil {
		return nil, mcptypes.NewRichError("SESSION_INITIALIZATION_FAILED", fmt.Sprintf("failed to initialize session: %v", err), "workflow_error")
	}

	c.logger.Info().
		Str("session_id", session.ID).
		Str("workflow_name", workflowSpec.Metadata.Name).
		Msg("Starting workflow execution")

	// Transition to running state
	if err := c.stateMachine.TransitionState(session, WorkflowStatusRunning); err != nil {
		return nil, mcptypes.NewRichError("WORKFLOW_START_FAILED", fmt.Sprintf("failed to start workflow: %v", err), "workflow_error")
	}

	// Execute workflow
	result := c.executeWorkflowSession(ctx, workflowSpec, session, options)

	// Finalize workflow
	c.finalizeWorkflow(session, result)

	return result, nil
}

// PauseWorkflow pauses a running workflow
func (c *Coordinator) PauseWorkflow(sessionID string) error {
	session, err := c.sessionManager.GetSession(sessionID)
	if err != nil {
		return mcptypes.NewRichError("SESSION_NOT_FOUND", fmt.Sprintf("failed to get session: %v", err), "session_error")
	}

	if err := c.stateMachine.TransitionState(session, WorkflowStatusPaused); err != nil {
		return mcptypes.NewRichError("WORKFLOW_PAUSE_FAILED", fmt.Sprintf("failed to pause workflow: %v", err), "workflow_error")
	}

	c.logger.Info().
		Str("session_id", sessionID).
		Msg("Workflow paused")

	return nil
}

// ResumeWorkflow resumes a paused workflow
func (c *Coordinator) ResumeWorkflow(ctx context.Context, sessionID string, workflowSpec *WorkflowSpec) (*WorkflowResult, error) {
	session, err := c.sessionManager.GetSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	if session.Status != WorkflowStatusPaused {
		return nil, mcptypes.NewRichError("WORKFLOW_NOT_PAUSED", fmt.Sprintf("workflow is not paused (current status: %s)", session.Status), "workflow_error")
	}

	if err := c.stateMachine.TransitionState(session, WorkflowStatusRunning); err != nil {
		return nil, mcptypes.NewRichError("WORKFLOW_RESUME_FAILED", fmt.Sprintf("failed to resume workflow: %v", err), "workflow_error")
	}

	c.logger.Info().
		Str("session_id", sessionID).
		Msg("Resuming workflow")

	// Continue execution from where it left off
	options := &ExecutionOptions{
		SessionID:      sessionID,
		EnableParallel: true,
	}

	result := c.executeWorkflowSession(ctx, workflowSpec, session, options)
	c.finalizeWorkflow(session, result)

	return result, nil
}

// CancelWorkflow cancels a workflow execution
func (c *Coordinator) CancelWorkflow(sessionID string) error {
	session, err := c.sessionManager.GetSession(sessionID)
	if err != nil {
		return mcptypes.NewRichError("SESSION_NOT_FOUND", fmt.Sprintf("failed to get session: %v", err), "session_error")
	}

	if c.stateMachine.IsTerminalState(session.Status) {
		return mcptypes.NewRichError("WORKFLOW_ALREADY_TERMINAL", fmt.Sprintf("cannot cancel workflow in terminal state: %s", session.Status), "workflow_error")
	}

	if err := c.stateMachine.TransitionState(session, WorkflowStatusCancelled); err != nil {
		return mcptypes.NewRichError("WORKFLOW_CANCEL_FAILED", fmt.Sprintf("failed to cancel workflow: %v", err), "workflow_error")
	}

	c.logger.Info().
		Str("session_id", sessionID).
		Msg("Workflow cancelled")

	return nil
}

// Private helper methods

func (c *Coordinator) initializeSession(workflowSpec *WorkflowSpec, options *ExecutionOptions) (*WorkflowSession, error) {
	// Resume from checkpoint if specified
	if options.ResumeFromCheckpoint != "" {
		session, err := c.checkpointManager.RestoreFromCheckpoint(options.SessionID, options.ResumeFromCheckpoint)
		if err != nil {
			return nil, mcptypes.NewRichError("CHECKPOINT_RESTORE_FAILED", fmt.Sprintf("failed to restore from checkpoint: %v", err), "workflow_error")
		}
		c.logger.Info().
			Str("session_id", session.ID).
			Str("checkpoint_id", options.ResumeFromCheckpoint).
			Msg("Restored workflow from checkpoint")
		return session, nil
	}

	// Resume existing session if specified
	if options.SessionID != "" {
		session, err := c.sessionManager.GetSession(options.SessionID)
		if err != nil {
			return nil, mcptypes.NewRichError("SESSION_NOT_FOUND", fmt.Sprintf("failed to get existing session: %v", err), "session_error")
		}
		return session, nil
	}

	// Create new session
	session, err := c.sessionManager.CreateSession(workflowSpec)
	if err != nil {
		return nil, mcptypes.NewRichError("SESSION_CREATION_FAILED", fmt.Sprintf("failed to create session: %v", err), "session_error")
	}

	// Store workflow variables for enhanced variable expansion
	if workflowSpec.Spec.Variables != nil {
		session.SharedContext["_workflow_variables"] = workflowSpec.Spec.Variables
	}

	// Apply initial variables
	if options.Variables != nil {
		for k, v := range options.Variables {
			session.SharedContext[k] = v
		}
	}

	return session, nil
}

func (c *Coordinator) executeWorkflowSession(
	ctx context.Context,
	workflowSpec *WorkflowSpec,
	session *WorkflowSession,
	options *ExecutionOptions,
) *WorkflowResult {
	startTime := time.Now()
	result := &WorkflowResult{
		WorkflowID: session.WorkflowID,
		SessionID:  session.ID,
		Status:     WorkflowStatusRunning,
		Results:    make(map[string]interface{}),
		Artifacts:  []WorkflowArtifact{},
		Metrics: WorkflowMetrics{
			StageDurations:      make(map[string]time.Duration),
			ToolExecutionCounts: make(map[string]int),
		},
	}

	// Resolve execution order
	executionGroups, err := c.dependencyResolver.ResolveDependencies(workflowSpec.Spec.Stages)
	if err != nil {
		result.Status = WorkflowStatusFailed
		result.Message = fmt.Sprintf("Failed to resolve dependencies: %v", err)
		return result
	}

	// Execute stage groups
	for groupIndex, stageGroup := range executionGroups {
		// Skip already completed stages
		if c.isGroupCompleted(stageGroup, session) {
			c.logger.Debug().
				Int("group_index", groupIndex).
				Msg("Skipping completed stage group")
			continue
		}

		c.logger.Info().
			Int("group_index", groupIndex).
			Int("stage_count", len(stageGroup)).
			Msg("Executing stage group")

		// Execute the group
		groupResults, err := c.executor.ExecuteStageGroup(
			ctx,
			stageGroup,
			session,
			workflowSpec,
			options.EnableParallel,
		)

		// Process results
		for _, stageResult := range groupResults {
			result.StagesExecuted++
			if stageResult.Success {
				result.StagesCompleted++
			} else {
				result.StagesFailed++
			}

			// Store stage results
			if session.StageResults == nil {
				session.StageResults = make(map[string]interface{})
			}
			session.StageResults[stageResult.StageName] = stageResult.Results

			// Collect artifacts
			result.Artifacts = append(result.Artifacts, stageResult.Artifacts...)

			// Record metrics
			result.Metrics.StageDurations[stageResult.StageName] = stageResult.Duration
		}

		// Handle group execution error
		if err != nil {
			c.logger.Error().
				Err(err).
				Int("group_index", groupIndex).
				Msg("Stage group execution failed")

			result.Status = WorkflowStatusFailed
			result.Message = fmt.Sprintf("Stage group %d failed: %v", groupIndex, err)

			// Check error policy
			if workflowSpec.Spec.ErrorPolicy.Mode == "fail_fast" {
				break
			}
		}

		// Create checkpoint if enabled
		if options.CreateCheckpoints {
			c.createGroupCheckpoint(session, groupIndex, workflowSpec)
		}

		// Handle partial stage completion for resume capability
		c.updateStageCompletionState(session, stageGroup, groupResults)

		// Check for cancellation
		select {
		case <-ctx.Done():
			result.Status = WorkflowStatusCancelled
			result.Message = "Workflow cancelled by context"
			return result
		default:
		}
	}

	// Calculate final metrics
	result.Duration = time.Since(startTime)
	result.Metrics.TotalDuration = result.Duration

	// Determine final status
	if result.Status == WorkflowStatusRunning {
		if result.StagesFailed > 0 {
			result.Status = WorkflowStatusFailed
			result.Success = false
			result.Message = fmt.Sprintf("Workflow completed with %d failed stages", result.StagesFailed)
		} else {
			result.Status = WorkflowStatusCompleted
			result.Success = true
			result.Message = "Workflow completed successfully"
		}
	}

	return result
}

func (c *Coordinator) finalizeWorkflow(session *WorkflowSession, result *WorkflowResult) {
	// Update session with final state
	finalStatus := result.Status
	if err := c.stateMachine.TransitionState(session, finalStatus); err != nil {
		c.logger.Error().
			Err(err).
			Str("session_id", session.ID).
			Str("status", string(finalStatus)).
			Msg("Failed to transition to final state")
	}

	// Generate error summary if there were failures
	if result.StagesFailed > 0 && session.ErrorContext != nil {
		result.ErrorSummary = c.generateErrorSummary(session.ErrorContext)
	}

	c.logger.Info().
		Str("session_id", session.ID).
		Str("status", string(result.Status)).
		Dur("duration", result.Duration).
		Int("stages_completed", result.StagesCompleted).
		Int("stages_failed", result.StagesFailed).
		Msg("Workflow execution completed")
}

func (c *Coordinator) isGroupCompleted(stages []WorkflowStage, session *WorkflowSession) bool {
	for _, stage := range stages {
		completed := false
		for _, completedStage := range session.CompletedStages {
			if completedStage == stage.Name {
				completed = true
				break
			}
		}
		if !completed {
			return false
		}
	}
	return true
}

func (c *Coordinator) createGroupCheckpoint(session *WorkflowSession, groupIndex int, workflowSpec *WorkflowSpec) {
	checkpoint, err := c.checkpointManager.CreateCheckpoint(
		session,
		fmt.Sprintf("group_%d", groupIndex),
		fmt.Sprintf("Completed stage group %d", groupIndex),
		workflowSpec,
	)
	if err != nil {
		c.logger.Warn().
			Err(err).
			Int("group_index", groupIndex).
			Msg("Failed to create checkpoint")
	} else {
		session.Checkpoints = append(session.Checkpoints, *checkpoint)
	}
}

func (c *Coordinator) generateErrorSummary(errorContext *WorkflowErrorContext) *WorkflowErrorSummary {
	summary := &WorkflowErrorSummary{
		TotalErrors:     len(errorContext.ErrorHistory),
		ErrorsByType:    make(map[string]int),
		ErrorsByStage:   make(map[string]int),
		RetryAttempts:   errorContext.RetryCount,
		LastError:       errorContext.LastError,
		Recommendations: []string{},
	}

	// Analyze errors
	for _, err := range errorContext.ErrorHistory {
		summary.ErrorsByType[err.ErrorType]++
		summary.ErrorsByStage[err.StageName]++

		if err.Severity == "critical" {
			summary.CriticalErrors++
		}
		if err.Retryable {
			summary.RecoverableErrors++
		}
	}

	// Generate recommendations
	if summary.RecoverableErrors > 0 {
		summary.Recommendations = append(summary.Recommendations,
			"Consider increasing retry attempts for recoverable errors")
	}
	if summary.CriticalErrors > 0 {
		summary.Recommendations = append(summary.Recommendations,
			"Review critical errors and ensure prerequisites are met")
	}

	return summary
}

func (c *Coordinator) updateStageCompletionState(session *WorkflowSession, stageGroup []WorkflowStage, results []StageResult) {
	// Update completion tracking for resume capability
	for i, result := range results {
		if i < len(stageGroup) {
			stageName := stageGroup[i].Name

			if result.Success {
				// Add to completed stages if not already there
				if !c.containsString(session.CompletedStages, stageName) {
					session.CompletedStages = append(session.CompletedStages, stageName)
				}
				// Remove from failed stages if it was there
				session.FailedStages = c.removeString(session.FailedStages, stageName)
			} else {
				// Add to failed stages if not already there
				if !c.containsString(session.FailedStages, stageName) {
					session.FailedStages = append(session.FailedStages, stageName)
				}
				// Remove from completed stages if it was there
				session.CompletedStages = c.removeString(session.CompletedStages, stageName)
			}
		}
	}

	// Update session
	session.LastActivity = time.Now()
	session.UpdatedAt = time.Now()

	// Persist the updated state
	if err := c.sessionManager.UpdateSession(session); err != nil {
		c.logger.Warn().Err(err).Msg("Failed to update stage completion state")
	}
}

func (c *Coordinator) containsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

func (c *Coordinator) removeString(slice []string, str string) []string {
	var result []string
	for _, s := range slice {
		if s != str {
			result = append(result, s)
		}
	}
	return result
}

// ResumeFromStage allows resuming a workflow from a specific stage
func (c *Coordinator) ResumeFromStage(ctx context.Context, sessionID, stageName string, workflowSpec *WorkflowSpec) (*WorkflowResult, error) {
	session, err := c.sessionManager.GetSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Validate stage exists in workflow
	var stageExists bool
	for _, stage := range workflowSpec.Spec.Stages {
		if stage.Name == stageName {
			stageExists = true
			break
		}
	}

	if !stageExists {
		return nil, mcptypes.NewRichError("STAGE_NOT_FOUND", fmt.Sprintf("stage '%s' not found in workflow", stageName), "workflow_error")
	}

	// Update session state for resume
	session.CurrentStage = stageName
	session.Status = WorkflowStatusPaused
	session.LastActivity = time.Now()
	session.UpdatedAt = time.Now()

	// Remove stages after the resume point from completed list
	var newCompleted []string
	for _, completed := range session.CompletedStages {
		if completed != stageName {
			newCompleted = append(newCompleted, completed)
		} else {
			break
		}
	}
	session.CompletedStages = newCompleted

	// Create checkpoint for this resume point
	checkpoint, err := c.checkpointManager.CreateCheckpoint(session, stageName, fmt.Sprintf("Resume from stage: %s", stageName), workflowSpec)
	if err != nil {
		return nil, mcptypes.NewRichError("CHECKPOINT_CREATION_FAILED", fmt.Sprintf("failed to create resume checkpoint: %v", err), "workflow_error")
	}

	c.logger.Info().
		Str("session_id", sessionID).
		Str("stage_name", stageName).
		Str("checkpoint_id", checkpoint.ID).
		Msg("Created resume checkpoint for specific stage")

	// Resume workflow execution
	options := &ExecutionOptions{
		SessionID:            sessionID,
		ResumeFromCheckpoint: checkpoint.ID,
		EnableParallel:       true,
		CreateCheckpoints:    true,
	}

	result := c.executeWorkflowSession(ctx, workflowSpec, session, options)
	c.finalizeWorkflow(session, result)

	return result, nil
}

// GetCheckpointHistory returns checkpoint history for a session
func (c *Coordinator) GetCheckpointHistory(sessionID string) ([]*WorkflowCheckpoint, error) {
	return c.checkpointManager.ListCheckpoints(sessionID)
}
