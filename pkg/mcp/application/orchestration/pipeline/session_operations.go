package pipeline

import (
	"context"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	sessionsvc "github.com/Azure/container-kit/pkg/mcp/session"
)

// GetSessionWorkspace retrieves the workspace directory for a given session ID
func (o *Operations) GetSessionWorkspace(sessionID string) string {
	if sessionID == "" {
		return ""
	}

	coreSession, err := o.sessionManager.GetSessionTyped(sessionID)
	if err != nil {
		o.logger.Error("Failed to get session", "session_id", sessionID, "error", err)
		return ""
	}
	return coreSession.WorkspaceDir
}

// UpdateSessionFromDockerResults updates session state with Docker operation results
func (o *Operations) UpdateSessionFromDockerResults(sessionID string, data SessionOperationData) error {
	if sessionID == "" {
		return errors.NewError().Message("session ID is required").Build()
	}

	if data.Timestamp == 0 {
		data.Timestamp = time.Now().Unix()
	}

	return o.sessionManager.UpdateSession(context.Background(), sessionID, func(sess *sessionsvc.SessionState) error {
		sess.LastAccessed = time.Now()
		if sess.Metadata == nil {
			sess.Metadata = make(map[string]interface{})
		}
		sess.Metadata["last_operation"] = data
		return nil
	})
}

// UpdateSessionState updates session state using a callback function
func (o *Operations) UpdateSessionState(sessionID string, updateFunc func(*core.SessionState)) error {
	return o.sessionManager.UpdateSession(context.Background(), sessionID, func(sessionState *sessionsvc.SessionState) error {
		coreState := &core.SessionState{
			SessionID:           sessionState.SessionID,
			UserID:              "",
			CreatedAt:           sessionState.CreatedAt,
			UpdatedAt:           sessionState.LastAccessed,
			ExpiresAt:           sessionState.ExpiresAt,
			WorkspaceDir:        sessionState.WorkspaceDir,
			RepositoryAnalyzed:  false,
			RepoURL:             sessionState.RepoURL,
			DockerfileGenerated: sessionState.Dockerfile.Built,
			DockerfilePath:      sessionState.Dockerfile.Path,
		}
		updateFunc(coreState)
		return nil
	})
}

// startJobTracking starts job tracking for a session operation
func (o *Operations) startJobTracking(sessionID, jobType string) (string, error) {
	jobID, err := o.sessionManager.StartJob(sessionID, jobType)
	if err != nil {
		o.logger.Warn("Failed to start job tracking", "session_id", sessionID, "error", err)
		return "", err
	}
	return jobID, nil
}

// updateJobStatus updates the status of a tracked job
func (o *Operations) updateJobStatus(sessionID, jobID string, status interface{}, result interface{}, err error) error {
	updateErr := o.sessionManager.UpdateJobStatus(sessionID, jobID, status, result, err)
	if updateErr != nil {
		o.logger.Warn("Failed to update job status", "session_id", sessionID, "job_id", jobID, "error", updateErr)
	}
	return updateErr
}

// completeJob marks a job as completed with result data
func (o *Operations) completeJob(sessionID, jobID string, result interface{}) error {
	return o.sessionManager.CompleteJob(sessionID, jobID, result)
}

// trackToolExecution tracks tool execution for session monitoring
func (o *Operations) trackToolExecution(sessionID, toolName string, args interface{}) error {
	return o.sessionManager.TrackToolExecution(sessionID, toolName, args)
}

// completeToolExecution marks tool execution as completed
func (o *Operations) completeToolExecution(sessionID, toolName string, success bool, err error, tokensUsed int) error {
	return o.sessionManager.CompleteToolExecution(sessionID, toolName, success, err, tokensUsed)
}

// trackSessionError tracks errors for session statistics and debugging
func (o *Operations) trackSessionError(sessionID string, err error, context map[string]interface{}) error {
	return o.sessionManager.TrackError(sessionID, err, context)
}

// AcquireResource acquires a resource for a session
func (o *Operations) AcquireResource(sessionID, resourceType string) error {
	o.logger.Debug("Acquiring resource", "session_id", sessionID, "resource_type", resourceType)
	return nil
}

// ReleaseResource releases a resource from a session
func (o *Operations) ReleaseResource(sessionID, resourceType string) error {
	o.logger.Debug("Releasing resource", "session_id", sessionID, "resource_type", resourceType)
	return nil
}

// updateSessionWithOperationStart updates session state when starting an operation
func (o *Operations) updateSessionWithOperationStart(sessionID, operation, imageRef, jobID string) error {
	return o.UpdateSessionFromDockerResults(sessionID, SessionOperationData{
		Operation: operation,
		ImageRef:  imageRef,
		Status:    "starting",
		JobID:     jobID,
		Timestamp: time.Now().Unix(),
	})
}

// updateSessionWithOperationComplete updates session state when completing an operation
func (o *Operations) updateSessionWithOperationComplete(sessionID, operation, imageRef, output, jobID string) error {
	return o.UpdateSessionFromDockerResults(sessionID, SessionOperationData{
		Operation: operation,
		ImageRef:  imageRef,
		Status:    "completed",
		Output:    output,
		JobID:     jobID,
		Timestamp: time.Now().Unix(),
	})
}

// updateSessionWithOperationError updates session state when an operation fails
func (o *Operations) updateSessionWithOperationError(sessionID, operation, imageRef, errorMsg, output, jobID string) error {
	return o.UpdateSessionFromDockerResults(sessionID, SessionOperationData{
		Operation: operation,
		ImageRef:  imageRef,
		Status:    "failed",
		Error:     errorMsg,
		Output:    output,
		JobID:     jobID,
		Timestamp: time.Now().Unix(),
	})
}

// updateSessionWithTagOperationStart updates session state when starting a tag operation
func (o *Operations) updateSessionWithTagOperationStart(sessionID, operation, sourceRef, targetRef, jobID string) error {
	return o.UpdateSessionFromDockerResults(sessionID, SessionOperationData{
		Operation: operation,
		SourceRef: sourceRef,
		TargetRef: targetRef,
		Status:    "starting",
		JobID:     jobID,
		Timestamp: time.Now().Unix(),
	})
}

// updateSessionWithTagOperationComplete updates session state when completing a tag operation
func (o *Operations) updateSessionWithTagOperationComplete(sessionID, operation, sourceRef, targetRef, output string) error {
	return o.UpdateSessionFromDockerResults(sessionID, SessionOperationData{
		Operation: operation,
		SourceRef: sourceRef,
		TargetRef: targetRef,
		Status:    "completed",
		Output:    output,
		Timestamp: time.Now().Unix(),
	})
}

// updateSessionWithTagOperationError updates session state when a tag operation fails
func (o *Operations) updateSessionWithTagOperationError(sessionID, operation, sourceRef, targetRef, errorMsg, output string) error {
	return o.UpdateSessionFromDockerResults(sessionID, SessionOperationData{
		Operation: operation,
		SourceRef: sourceRef,
		TargetRef: targetRef,
		Status:    "failed",
		Error:     errorMsg,
		Output:    output,
		Timestamp: time.Now().Unix(),
	})
}
