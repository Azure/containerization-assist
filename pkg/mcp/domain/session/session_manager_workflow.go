package session

import (
	"context"
)

// GetWorkflowSession implements UnifiedSessionManager.GetWorkflowSession
func (sm *SessionManager) GetWorkflowSession(ctx context.Context, sessionID string) (*WorkflowSession, error) {
	session, err := sm.getOrCreateSessionConcrete(sessionID)
	if err != nil {
		return nil, err
	}

	// Convert to WorkflowSession
	workflowSession := &WorkflowSession{
		SessionState: session,
	}

	// Extract workflow metadata
	if session.Metadata != nil {
		if workflowID, ok := session.Metadata["workflow_id"].(string); ok {
			workflowSession.WorkflowID = workflowID
		}
		if workflowName, ok := session.Metadata["workflow_name"].(string); ok {
			workflowSession.WorkflowName = workflowName
		}
	}

	return workflowSession, nil
}

// UpdateWorkflowSession implements UnifiedSessionManager.UpdateWorkflowSession
func (sm *SessionManager) UpdateWorkflowSession(ctx context.Context, workflowSession *WorkflowSession) error {
	return sm.store.Save(ctx, workflowSession.SessionID, workflowSession.SessionState)
}
