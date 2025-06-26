package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// Mock implementations for testing

type mockStateMachine struct {
	transitionError  error
	terminalStates   map[WorkflowStatus]bool
	currentStatus    WorkflowStatus
	transitionCalled bool
}

func (m *mockStateMachine) TransitionState(session *WorkflowSession, status WorkflowStatus) error {
	m.transitionCalled = true
	if m.transitionError != nil {
		return m.transitionError
	}
	session.Status = status
	m.currentStatus = status
	return nil
}

func (m *mockStateMachine) IsTerminalState(status WorkflowStatus) bool {
	if m.terminalStates == nil {
		return status == WorkflowStatusCompleted || status == WorkflowStatusFailed || status == WorkflowStatusCancelled
	}
	return m.terminalStates[status]
}

type mockExecutor struct {
	executeError error
	stageResults []StageResult
}

func (m *mockExecutor) ExecuteStageGroup(ctx context.Context, stages []WorkflowStage, session *WorkflowSession, spec *WorkflowSpec, enableParallel bool) ([]StageResult, error) {
	if m.executeError != nil {
		return nil, m.executeError
	}
	
	if m.stageResults != nil {
		return m.stageResults, nil
	}

	// Default behavior: create successful results for all stages
	results := make([]StageResult, len(stages))
	for i, stage := range stages {
		results[i] = StageResult{
			StageName: stage.Name,
			Success:   true,
			Results:   map[string]interface{}{"completed": true},
			Duration:  time.Millisecond * 100,
			Artifacts: []WorkflowArtifact{},
		}
	}
	return results, nil
}

type mockWorkflowSessionManager struct {
	sessions    map[string]*WorkflowSession
	createError error
	getError    error
	updateError error
}

func (m *mockWorkflowSessionManager) CreateSession(spec *WorkflowSpec) (*WorkflowSession, error) {
	if m.createError != nil {
		return nil, m.createError
	}
	
	session := &WorkflowSession{
		ID:               "test-session-id",
		WorkflowID:       "test-workflow-id",
		WorkflowName:     spec.Metadata.Name,
		Status:           WorkflowStatusPending,
		CurrentStage:     "",
		CompletedStages:  []string{},
		FailedStages:     []string{},
		SkippedStages:    []string{},
		SharedContext:    make(map[string]interface{}),
		ResourceBindings: make(map[string]interface{}),
		StageResults:     make(map[string]interface{}),
		LastActivity:     time.Now(),
		StartTime:        time.Now(),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		Checkpoints:      []WorkflowCheckpoint{},
	}
	
	if m.sessions == nil {
		m.sessions = make(map[string]*WorkflowSession)
	}
	m.sessions[session.ID] = session
	
	return session, nil
}

func (m *mockWorkflowSessionManager) GetSession(sessionID string) (*WorkflowSession, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	
	if m.sessions == nil {
		return nil, errors.New("session not found")
	}
	
	session, exists := m.sessions[sessionID]
	if !exists {
		return nil, errors.New("session not found")
	}
	
	return session, nil
}

func (m *mockWorkflowSessionManager) UpdateSession(session *WorkflowSession) error {
	if m.updateError != nil {
		return m.updateError
	}
	
	if m.sessions == nil {
		m.sessions = make(map[string]*WorkflowSession)
	}
	m.sessions[session.ID] = session
	
	return nil
}

type mockDependencyResolver struct {
	resolveError error
	groups       [][]WorkflowStage
}

func (m *mockDependencyResolver) ResolveDependencies(stages []WorkflowStage) ([][]WorkflowStage, error) {
	if m.resolveError != nil {
		return nil, m.resolveError
	}
	
	if m.groups != nil {
		return m.groups, nil
	}
	
	// Default: each stage in its own group
	groups := make([][]WorkflowStage, len(stages))
	for i, stage := range stages {
		groups[i] = []WorkflowStage{stage}
	}
	return groups, nil
}

type mockCheckpointManager struct {
	createError  error
	restoreError error
	listError    error
	checkpoints  []*WorkflowCheckpoint
	checkpoint   *WorkflowCheckpoint
}

func (m *mockCheckpointManager) CreateCheckpoint(session *WorkflowSession, stageID string, description string, spec *WorkflowSpec) (*WorkflowCheckpoint, error) {
	if m.createError != nil {
		return nil, m.createError
	}
	
	checkpoint := &WorkflowCheckpoint{
		ID:      "checkpoint-" + stageID,
		StageID: stageID,
		Created: time.Now(),
	}
	
	if m.checkpoint != nil {
		return m.checkpoint, nil
	}
	
	return checkpoint, nil
}

func (m *mockCheckpointManager) RestoreFromCheckpoint(sessionID string, checkpointID string) (*WorkflowSession, error) {
	if m.restoreError != nil {
		return nil, m.restoreError
	}
	
	session := &WorkflowSession{
		ID:           sessionID,
		Status:       WorkflowStatusPaused,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		LastActivity: time.Now(),
	}
	
	return session, nil
}

func (m *mockCheckpointManager) ListCheckpoints(sessionID string) ([]*WorkflowCheckpoint, error) {
	if m.listError != nil {
		return nil, m.listError
	}
	
	if m.checkpoints != nil {
		return m.checkpoints, nil
	}
	
	return []*WorkflowCheckpoint{}, nil
}

// Test constants and utilities

func createTestWorkflowSpec() *WorkflowSpec {
	return &WorkflowSpec{
		Metadata: WorkflowMetadata{
			Name:    "test-workflow",
			Version: "1.0.0",
		},
		Spec: WorkflowDefinition{
			Stages: []WorkflowStage{
				{
					Name:      "stage1",
					Type:      "build",
					Tools:     []string{"tool1"},
					DependsOn: []string{},
					Variables: map[string]interface{}{"var1": "value1"},
				},
				{
					Name:      "stage2",
					Type:      "deploy",
					Tools:     []string{"tool2"},
					DependsOn: []string{"stage1"},
					Variables: map[string]interface{}{"var2": "value2"},
				},
			},
			Variables: map[string]interface{}{
				"global_var": "global_value",
			},
			ErrorPolicy: ErrorPolicy{
				Mode: "fail_fast",
			},
		},
	}
}

func createTestExecutionOptions() *ExecutionOptions {
	return &ExecutionOptions{
		SessionID:         "",
		EnableParallel:    true,
		CreateCheckpoints: false,
		Variables: map[string]interface{}{
			"test_var": "test_value",
		},
	}
}

// Unit Tests

func TestWorkflowStatus_Constants(t *testing.T) {
	assert.Equal(t, WorkflowStatus("pending"), WorkflowStatusPending)
	assert.Equal(t, WorkflowStatus("running"), WorkflowStatusRunning)
	assert.Equal(t, WorkflowStatus("completed"), WorkflowStatusCompleted)
	assert.Equal(t, WorkflowStatus("failed"), WorkflowStatusFailed)
	assert.Equal(t, WorkflowStatus("paused"), WorkflowStatusPaused)
	assert.Equal(t, WorkflowStatus("cancelled"), WorkflowStatusCancelled)
}

func TestNewCoordinator(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	stateMachine := &mockStateMachine{}
	executor := &mockExecutor{}
	sessionManager := &mockWorkflowSessionManager{}
	depResolver := &mockDependencyResolver{}
	checkpointManager := &mockCheckpointManager{}

	coordinator := NewCoordinator(
		logger,
		stateMachine,
		executor,
		sessionManager,
		depResolver,
		checkpointManager,
	)

	assert.NotNil(t, coordinator)
	assert.Equal(t, stateMachine, coordinator.stateMachine)
	assert.Equal(t, executor, coordinator.executor)
	assert.Equal(t, sessionManager, coordinator.sessionManager)
	assert.Equal(t, depResolver, coordinator.dependencyResolver)
	assert.Equal(t, checkpointManager, coordinator.checkpointManager)
}

func TestCoordinator_ExecuteWorkflow_Success(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	stateMachine := &mockStateMachine{}
	executor := &mockExecutor{}
	sessionManager := &mockWorkflowSessionManager{}
	depResolver := &mockDependencyResolver{}
	checkpointManager := &mockCheckpointManager{}

	coordinator := NewCoordinator(
		logger,
		stateMachine,
		executor,
		sessionManager,
		depResolver,
		checkpointManager,
	)

	workflowSpec := createTestWorkflowSpec()
	options := createTestExecutionOptions()

	result, err := coordinator.ExecuteWorkflow(context.Background(), workflowSpec, options)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, WorkflowStatusCompleted, result.Status)
	assert.True(t, result.Success)
	assert.Equal(t, "Workflow completed successfully", result.Message)
	assert.Equal(t, 2, result.StagesCompleted) // Two stages in test spec
	assert.Equal(t, 0, result.StagesFailed)
	assert.True(t, stateMachine.transitionCalled)
}

func TestCoordinator_ExecuteWorkflow_SessionCreationError(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	stateMachine := &mockStateMachine{}
	executor := &mockExecutor{}
	sessionManager := &mockWorkflowSessionManager{
		createError: errors.New("session creation failed"),
	}
	depResolver := &mockDependencyResolver{}
	checkpointManager := &mockCheckpointManager{}

	coordinator := NewCoordinator(
		logger,
		stateMachine,
		executor,
		sessionManager,
		depResolver,
		checkpointManager,
	)

	workflowSpec := createTestWorkflowSpec()
	options := createTestExecutionOptions()

	result, err := coordinator.ExecuteWorkflow(context.Background(), workflowSpec, options)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "SESSION_INITIALIZATION_FAILED")
}

func TestCoordinator_ExecuteWorkflow_StateTransitionError(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	stateMachine := &mockStateMachine{
		transitionError: errors.New("state transition failed"),
	}
	executor := &mockExecutor{}
	sessionManager := &mockWorkflowSessionManager{}
	depResolver := &mockDependencyResolver{}
	checkpointManager := &mockCheckpointManager{}

	coordinator := NewCoordinator(
		logger,
		stateMachine,
		executor,
		sessionManager,
		depResolver,
		checkpointManager,
	)

	workflowSpec := createTestWorkflowSpec()
	options := createTestExecutionOptions()

	result, err := coordinator.ExecuteWorkflow(context.Background(), workflowSpec, options)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "WORKFLOW_START_FAILED")
}

func TestCoordinator_ExecuteWorkflow_DependencyResolutionError(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	stateMachine := &mockStateMachine{}
	executor := &mockExecutor{}
	sessionManager := &mockWorkflowSessionManager{}
	depResolver := &mockDependencyResolver{
		resolveError: errors.New("dependency resolution failed"),
	}
	checkpointManager := &mockCheckpointManager{}

	coordinator := NewCoordinator(
		logger,
		stateMachine,
		executor,
		sessionManager,
		depResolver,
		checkpointManager,
	)

	workflowSpec := createTestWorkflowSpec()
	options := createTestExecutionOptions()

	result, err := coordinator.ExecuteWorkflow(context.Background(), workflowSpec, options)

	assert.NoError(t, err) // ExecuteWorkflow doesn't return error for dependency resolution failure
	assert.NotNil(t, result)
	assert.Equal(t, WorkflowStatusFailed, result.Status)
	assert.False(t, result.Success)
	assert.Contains(t, result.Message, "Failed to resolve dependencies")
}

func TestCoordinator_PauseWorkflow_Success(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	stateMachine := &mockStateMachine{}
	executor := &mockExecutor{}
	sessionManager := &mockWorkflowSessionManager{
		sessions: map[string]*WorkflowSession{
			"test-session": {
				ID:     "test-session",
				Status: WorkflowStatusRunning,
			},
		},
	}
	depResolver := &mockDependencyResolver{}
	checkpointManager := &mockCheckpointManager{}

	coordinator := NewCoordinator(
		logger,
		stateMachine,
		executor,
		sessionManager,
		depResolver,
		checkpointManager,
	)

	err := coordinator.PauseWorkflow("test-session")

	assert.NoError(t, err)
	assert.True(t, stateMachine.transitionCalled)
}

func TestCoordinator_PauseWorkflow_SessionNotFound(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	stateMachine := &mockStateMachine{}
	executor := &mockExecutor{}
	sessionManager := &mockWorkflowSessionManager{
		getError: errors.New("session not found"),
	}
	depResolver := &mockDependencyResolver{}
	checkpointManager := &mockCheckpointManager{}

	coordinator := NewCoordinator(
		logger,
		stateMachine,
		executor,
		sessionManager,
		depResolver,
		checkpointManager,
	)

	err := coordinator.PauseWorkflow("test-session")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SESSION_NOT_FOUND")
}

func TestCoordinator_ResumeWorkflow_Success(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	stateMachine := &mockStateMachine{}
	executor := &mockExecutor{}
	sessionManager := &mockWorkflowSessionManager{
		sessions: map[string]*WorkflowSession{
			"test-session": {
				ID:     "test-session",
				Status: WorkflowStatusPaused,
			},
		},
	}
	depResolver := &mockDependencyResolver{}
	checkpointManager := &mockCheckpointManager{}

	coordinator := NewCoordinator(
		logger,
		stateMachine,
		executor,
		sessionManager,
		depResolver,
		checkpointManager,
	)

	workflowSpec := createTestWorkflowSpec()
	result, err := coordinator.ResumeWorkflow(context.Background(), "test-session", workflowSpec)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, stateMachine.transitionCalled)
}

func TestCoordinator_ResumeWorkflow_NotPaused(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	stateMachine := &mockStateMachine{}
	executor := &mockExecutor{}
	sessionManager := &mockWorkflowSessionManager{
		sessions: map[string]*WorkflowSession{
			"test-session": {
				ID:     "test-session",
				Status: WorkflowStatusRunning, // Not paused
			},
		},
	}
	depResolver := &mockDependencyResolver{}
	checkpointManager := &mockCheckpointManager{}

	coordinator := NewCoordinator(
		logger,
		stateMachine,
		executor,
		sessionManager,
		depResolver,
		checkpointManager,
	)

	workflowSpec := createTestWorkflowSpec()
	result, err := coordinator.ResumeWorkflow(context.Background(), "test-session", workflowSpec)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "WORKFLOW_NOT_PAUSED")
}

func TestCoordinator_CancelWorkflow_Success(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	stateMachine := &mockStateMachine{}
	executor := &mockExecutor{}
	sessionManager := &mockWorkflowSessionManager{
		sessions: map[string]*WorkflowSession{
			"test-session": {
				ID:     "test-session",
				Status: WorkflowStatusRunning,
			},
		},
	}
	depResolver := &mockDependencyResolver{}
	checkpointManager := &mockCheckpointManager{}

	coordinator := NewCoordinator(
		logger,
		stateMachine,
		executor,
		sessionManager,
		depResolver,
		checkpointManager,
	)

	err := coordinator.CancelWorkflow("test-session")

	assert.NoError(t, err)
	assert.True(t, stateMachine.transitionCalled)
}

func TestCoordinator_CancelWorkflow_AlreadyTerminal(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	stateMachine := &mockStateMachine{
		terminalStates: map[WorkflowStatus]bool{
			WorkflowStatusCompleted: true,
			WorkflowStatusFailed:    true,
			WorkflowStatusCancelled: true,
		},
	}
	executor := &mockExecutor{}
	sessionManager := &mockWorkflowSessionManager{
		sessions: map[string]*WorkflowSession{
			"test-session": {
				ID:     "test-session",
				Status: WorkflowStatusCompleted, // Terminal state
			},
		},
	}
	depResolver := &mockDependencyResolver{}
	checkpointManager := &mockCheckpointManager{}

	coordinator := NewCoordinator(
		logger,
		stateMachine,
		executor,
		sessionManager,
		depResolver,
		checkpointManager,
	)

	err := coordinator.CancelWorkflow("test-session")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "WORKFLOW_ALREADY_TERMINAL")
}

func TestCoordinator_GetCheckpointHistory(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	stateMachine := &mockStateMachine{}
	executor := &mockExecutor{}
	sessionManager := &mockWorkflowSessionManager{}
	depResolver := &mockDependencyResolver{}
	
	expectedCheckpoints := []*WorkflowCheckpoint{
		{
			ID:      "checkpoint-1",
			StageID: "stage1",
			Created: time.Now(),
		},
		{
			ID:      "checkpoint-2",
			StageID: "stage2",
			Created: time.Now(),
		},
	}
	
	checkpointManager := &mockCheckpointManager{
		checkpoints: expectedCheckpoints,
	}

	coordinator := NewCoordinator(
		logger,
		stateMachine,
		executor,
		sessionManager,
		depResolver,
		checkpointManager,
	)

	checkpoints, err := coordinator.GetCheckpointHistory("test-session")

	assert.NoError(t, err)
	assert.Equal(t, expectedCheckpoints, checkpoints)
}

func TestCoordinator_HelperMethods(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	stateMachine := &mockStateMachine{}
	executor := &mockExecutor{}
	sessionManager := &mockWorkflowSessionManager{}
	depResolver := &mockDependencyResolver{}
	checkpointManager := &mockCheckpointManager{}

	coordinator := NewCoordinator(
		logger,
		stateMachine,
		executor,
		sessionManager,
		depResolver,
		checkpointManager,
	)

	// Test containsString
	slice := []string{"a", "b", "c"}
	assert.True(t, coordinator.containsString(slice, "b"))
	assert.False(t, coordinator.containsString(slice, "d"))

	// Test removeString
	result := coordinator.removeString(slice, "b")
	expected := []string{"a", "c"}
	assert.Equal(t, expected, result)

	// Test removeString with non-existent element
	result2 := coordinator.removeString(slice, "d")
	assert.Equal(t, slice, result2)
}

func TestWorkflowSession_Structure(t *testing.T) {
	session := &WorkflowSession{
		ID:               "test-id",
		WorkflowID:       "workflow-id",
		WorkflowName:     "test-workflow",
		Status:           WorkflowStatusPending,
		CurrentStage:     "stage1",
		CompletedStages:  []string{"stage0"},
		FailedStages:     []string{},
		SkippedStages:    []string{},
		SharedContext:    map[string]interface{}{"key": "value"},
		ResourceBindings: map[string]interface{}{"resource": "binding"},
		StageResults:     map[string]interface{}{"stage": "result"},
		LastActivity:     time.Now(),
		StartTime:        time.Now(),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		Checkpoints:      []WorkflowCheckpoint{},
	}

	assert.Equal(t, "test-id", session.ID)
	assert.Equal(t, "workflow-id", session.WorkflowID)
	assert.Equal(t, "test-workflow", session.WorkflowName)
	assert.Equal(t, WorkflowStatusPending, session.Status)
	assert.Equal(t, "stage1", session.CurrentStage)
	assert.Len(t, session.CompletedStages, 1)
	assert.Len(t, session.FailedStages, 0)
	assert.Contains(t, session.SharedContext, "key")
	assert.Equal(t, "value", session.SharedContext["key"])
}

func TestWorkflowTypes_Structure(t *testing.T) {
	// Test WorkflowSpec
	spec := createTestWorkflowSpec()
	assert.Equal(t, "test-workflow", spec.Metadata.Name)
	assert.Equal(t, "1.0.0", spec.Metadata.Version)
	assert.Len(t, spec.Spec.Stages, 2)
	assert.Equal(t, "fail_fast", spec.Spec.ErrorPolicy.Mode)

	// Test StageResult
	stageResult := StageResult{
		StageName: "test-stage",
		Success:   true,
		Results:   map[string]interface{}{"result": "success"},
		Duration:  time.Second,
		Artifacts: []WorkflowArtifact{
			{Name: "artifact1", Path: "/path/to/artifact1"},
		},
	}
	assert.Equal(t, "test-stage", stageResult.StageName)
	assert.True(t, stageResult.Success)
	assert.Equal(t, time.Second, stageResult.Duration)
	assert.Len(t, stageResult.Artifacts, 1)

	// Test WorkflowResult
	workflowResult := WorkflowResult{
		WorkflowID:      "workflow-123",
		SessionID:       "session-456",
		Status:          WorkflowStatusCompleted,
		Success:         true,
		Message:         "Completed successfully",
		Duration:        time.Minute,
		Results:         map[string]interface{}{"final": "result"},
		StagesExecuted:  3,
		StagesCompleted: 3,
		StagesFailed:    0,
	}
	assert.Equal(t, "workflow-123", workflowResult.WorkflowID)
	assert.True(t, workflowResult.Success)
	assert.Equal(t, 3, workflowResult.StagesCompleted)
	assert.Equal(t, 0, workflowResult.StagesFailed)

	// Test ExecutionOptions
	options := createTestExecutionOptions()
	assert.True(t, options.EnableParallel)
	assert.False(t, options.CreateCheckpoints)
	assert.Contains(t, options.Variables, "test_var")
}

func TestWorkflowErrorTypes(t *testing.T) {
	// Test WorkflowError
	wfError := WorkflowError{
		StageName: "stage1",
		ErrorType: "validation_error",
		Severity:  "critical",
		Retryable: false,
	}
	assert.Equal(t, "stage1", wfError.StageName)
	assert.Equal(t, "validation_error", wfError.ErrorType)
	assert.Equal(t, "critical", wfError.Severity)
	assert.False(t, wfError.Retryable)

	// Test WorkflowErrorContext
	errorContext := &WorkflowErrorContext{
		ErrorHistory: []WorkflowError{wfError},
		RetryCount:   2,
		LastError:    "Last error message",
	}
	assert.Len(t, errorContext.ErrorHistory, 1)
	assert.Equal(t, 2, errorContext.RetryCount)
	assert.Equal(t, "Last error message", errorContext.LastError)

	// Test WorkflowErrorSummary
	errorSummary := &WorkflowErrorSummary{
		TotalErrors:       5,
		CriticalErrors:    2,
		RecoverableErrors: 3,
		ErrorsByType:      map[string]int{"validation": 2, "timeout": 3},
		ErrorsByStage:     map[string]int{"stage1": 3, "stage2": 2},
		RetryAttempts:     4,
		LastError:         "Connection timeout",
		Recommendations:   []string{"Check network", "Increase timeout"},
	}
	assert.Equal(t, 5, errorSummary.TotalErrors)
	assert.Equal(t, 2, errorSummary.CriticalErrors)
	assert.Equal(t, 3, errorSummary.RecoverableErrors)
	assert.Len(t, errorSummary.Recommendations, 2)
}

func TestWorkflowMetrics(t *testing.T) {
	metrics := WorkflowMetrics{
		TotalDuration: time.Minute * 5,
		StageDurations: map[string]time.Duration{
			"stage1": time.Minute * 2,
			"stage2": time.Minute * 3,
		},
		ToolExecutionCounts: map[string]int{
			"tool1": 3,
			"tool2": 2,
		},
	}

	assert.Equal(t, time.Minute*5, metrics.TotalDuration)
	assert.Equal(t, time.Minute*2, metrics.StageDurations["stage1"])
	assert.Equal(t, 3, metrics.ToolExecutionCounts["tool1"])
}

func TestWorkflowCheckpoint(t *testing.T) {
	now := time.Now()
	checkpoint := WorkflowCheckpoint{
		ID:      "checkpoint-123",
		StageID: "stage1",
		Created: now,
	}

	assert.Equal(t, "checkpoint-123", checkpoint.ID)
	assert.Equal(t, "stage1", checkpoint.StageID)
	assert.Equal(t, now, checkpoint.Created)
}

func TestWorkflowArtifact(t *testing.T) {
	artifact := WorkflowArtifact{
		Name: "build-output",
		Path: "/path/to/output.tar.gz",
	}

	assert.Equal(t, "build-output", artifact.Name)
	assert.Equal(t, "/path/to/output.tar.gz", artifact.Path)
}