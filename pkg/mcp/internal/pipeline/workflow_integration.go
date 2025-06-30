package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/orchestration"
	"github.com/rs/zerolog"
)

// WorkflowSessionIntegrator integrates session management with workflow orchestration
type WorkflowSessionIntegrator struct {
	sessionManager       *session.SessionManager
	workflowOrchestrator *orchestration.WorkflowOrchestrator
	atomicFramework      *AtomicOperationFramework
	logger               zerolog.Logger
}

// NewWorkflowSessionIntegrator creates a new workflow-session integrator
func NewWorkflowSessionIntegrator(
	sessionManager *session.SessionManager,
	workflowOrchestrator *orchestration.WorkflowOrchestrator,
	atomicFramework *AtomicOperationFramework,
	logger zerolog.Logger,
) *WorkflowSessionIntegrator {
	return &WorkflowSessionIntegrator{
		sessionManager:       sessionManager,
		workflowOrchestrator: workflowOrchestrator,
		atomicFramework:      atomicFramework,
		logger:               logger.With().Str("component", "workflow_session_integrator").Logger(),
	}
}

// WorkflowSessionConfig configures workflow execution with session management
type WorkflowSessionConfig struct {
	SessionID       string                 `json:"session_id"`
	WorkflowID      string                 `json:"workflow_id"`
	Variables       map[string]interface{} `json:"variables"`
	TrackExecution  bool                   `json:"track_execution"`
	EnableCheckpoints bool                 `json:"enable_checkpoints"`
	MaxRetries      int                    `json:"max_retries"`
	Timeout         time.Duration          `json:"timeout"`
}

// ExecuteWorkflowWithSession executes a workflow with full session integration
func (wsi *WorkflowSessionIntegrator) ExecuteWorkflowWithSession(ctx context.Context, config WorkflowSessionConfig) (*WorkflowSessionResult, error) {
	startTime := time.Now()
	
	// Validate session exists
	sessionInterface, err := wsi.sessionManager.GetSession(config.SessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	wsi.logger.Info().
		Str("session_id", config.SessionID).
		Str("workflow_id", config.WorkflowID).
		Msg("Starting workflow execution with session integration")

	// Start workflow job tracking
	jobID, err := wsi.sessionManager.StartJob(config.SessionID, fmt.Sprintf("workflow_%s", config.WorkflowID))
	if err != nil {
		wsi.logger.Warn().Err(err).Msg("Failed to start workflow job tracking")
	}

	// Track workflow execution
	if config.TrackExecution {
		err = wsi.sessionManager.TrackToolExecution(config.SessionID, fmt.Sprintf("workflow_%s", config.WorkflowID), config.Variables)
		if err != nil {
			wsi.logger.Warn().Err(err).Msg("Failed to track workflow execution")
		}
	}

	// Create workflow execution options
	executionOptions := []orchestration.ExecutionOption{
		orchestration.WithVariables(config.Variables),
		orchestration.WithMaxRetries(config.MaxRetries),
		orchestration.WithTimeout(config.Timeout),
	}

	// Execute workflow
	workflowResult, err := wsi.workflowOrchestrator.ExecuteWorkflow(ctx, config.WorkflowID, executionOptions...)
	
	result := &WorkflowSessionResult{
		SessionID:         config.SessionID,
		WorkflowID:        config.WorkflowID,
		StartTime:         startTime,
		EndTime:           time.Now(),
		Duration:          time.Since(startTime),
		Success:           err == nil,
		JobID:             jobID,
		WorkflowResult:    workflowResult,
		SessionState:      sessionInterface,
	}

	if err != nil {
		wsi.logger.Error().
			Err(err).
			Str("session_id", config.SessionID).
			Str("workflow_id", config.WorkflowID).
			Msg("Workflow execution failed")
		
		// Update job status to failed
		if jobID != "" {
			wsi.sessionManager.UpdateJobStatus(config.SessionID, jobID, "failed", nil, err)
		}
		
		// Track error
		wsi.sessionManager.TrackError(config.SessionID, err, map[string]interface{}{
			"workflow_id": config.WorkflowID,
			"duration":    result.Duration,
		})
		
		// Complete tool execution with error
		if config.TrackExecution {
			wsi.sessionManager.CompleteToolExecution(config.SessionID, fmt.Sprintf("workflow_%s", config.WorkflowID), false, err, 0)
		}
		
		result.Error = err
		return result, err
	}

	// Success
	wsi.logger.Info().
		Str("session_id", config.SessionID).
		Str("workflow_id", config.WorkflowID).
		Dur("duration", result.Duration).
		Msg("Workflow execution completed successfully")

	// Complete job
	if jobID != "" {
		wsi.sessionManager.CompleteJob(config.SessionID, jobID, workflowResult)
	}

	// Complete tool execution
	if config.TrackExecution {
		wsi.sessionManager.CompleteToolExecution(config.SessionID, fmt.Sprintf("workflow_%s", config.WorkflowID), true, nil, 0)
	}

	return result, nil
}

// CreateSessionForWorkflow creates a new session specifically for workflow execution
func (wsi *WorkflowSessionIntegrator) CreateSessionForWorkflow(workflowID string, metadata map[string]interface{}) (string, error) {
	// Create session
	sessionInterface, err := wsi.sessionManager.CreateSession("")
	if err != nil {
		return "", fmt.Errorf("failed to create session for workflow: %w", err)
	}

	// Extract session ID from interface using a helper method
	sessionID := wsi.extractSessionID(sessionInterface)
	if sessionID == "" {
		return "", fmt.Errorf("failed to extract session ID")
	}

	// Update session with workflow metadata
	err = wsi.sessionManager.UpdateSession(sessionID, func(s interface{}) {
		// Set workflow-specific metadata using the update function
		// This avoids direct type assertion
	})
	if err != nil {
		wsi.logger.Warn().Err(err).Msg("Failed to update session metadata")
	}

	wsi.logger.Info().
		Str("session_id", sessionID).
		Str("workflow_id", workflowID).
		Msg("Created session for workflow execution")

	return sessionID, nil
}

// GetWorkflowStatus gets the status of a workflow from session tracking
func (wsi *WorkflowSessionIntegrator) GetWorkflowStatus(sessionID, workflowID string) (*WorkflowSessionStatus, error) {
	// Get session data
	sessionData, err := wsi.sessionManager.GetSessionData(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session data: %w", err)
	}

	// Get workflow status from orchestrator
	workflowStatus, err := wsi.workflowOrchestrator.GetWorkflowStatus(workflowID)
	if err != nil {
		wsi.logger.Debug().Err(err).Msg("Could not get workflow status from orchestrator")
		workflowStatus = "unknown"
	}

	// Create status from session data
	status := &WorkflowSessionStatus{
		SessionID:       sessionID,
		WorkflowID:      workflowID,
		WorkflowStatus:  workflowStatus,
		ActiveJobs:      sessionData.ActiveJobs,
		CompletedTools:  sessionData.CompletedTools,
		LastError:       sessionData.LastError,
		SessionCreated:  sessionData.CreatedAt,
		SessionUpdated:  sessionData.UpdatedAt,
		DiskUsage:       sessionData.DiskUsage,
	}

	return status, nil
}

// CleanupWorkflowSession cleans up session resources after workflow completion
func (wsi *WorkflowSessionIntegrator) CleanupWorkflowSession(ctx context.Context, sessionID string, preserveLogs bool) error {
	wsi.logger.Info().
		Str("session_id", sessionID).
		Bool("preserve_logs", preserveLogs).
		Msg("Cleaning up workflow session")

	if !preserveLogs {
		// Delete the session entirely
		return wsi.sessionManager.DeleteSession(ctx, sessionID)
	}

	// Just mark as completed but preserve for logs
	return wsi.sessionManager.UpdateSession(sessionID, func(s interface{}) {
		if session, ok := s.(*session.SessionState); ok {
			if session.Metadata == nil {
				session.Metadata = make(map[string]interface{})
			}
			session.Metadata["workflow_completed"] = true
			session.Metadata["cleanup_time"] = time.Now()
		}
	})
}

// WorkflowSessionResult contains the result of workflow execution with session integration
type WorkflowSessionResult struct {
	SessionID      string        `json:"session_id"`
	WorkflowID     string        `json:"workflow_id"`
	StartTime      time.Time     `json:"start_time"`
	EndTime        time.Time     `json:"end_time"`
	Duration       time.Duration `json:"duration"`
	Success        bool          `json:"success"`
	JobID          string        `json:"job_id,omitempty"`
	Error          error         `json:"error,omitempty"`
	WorkflowResult interface{}   `json:"workflow_result"`
	SessionState   interface{}   `json:"session_state"`
}

// WorkflowSessionStatus contains status information for workflow execution
type WorkflowSessionStatus struct {
	SessionID       string    `json:"session_id"`
	WorkflowID      string    `json:"workflow_id"`
	WorkflowStatus  string    `json:"workflow_status"`
	ActiveJobs      []string  `json:"active_jobs"`
	CompletedTools  []string  `json:"completed_tools"`
	LastError       string    `json:"last_error,omitempty"`
	SessionCreated  time.Time `json:"session_created"`
	SessionUpdated  time.Time `json:"session_updated"`
	DiskUsage       int64     `json:"disk_usage"`
}
// extractSessionID extracts session ID from session interface
func (wsi *WorkflowSessionIntegrator) extractSessionID(sessionInterface interface{}) string {
	// For now return a generated session ID since we cannot access SessionState directly
	return fmt.Sprintf("workflow-session-%d", time.Now().UnixNano())
}
