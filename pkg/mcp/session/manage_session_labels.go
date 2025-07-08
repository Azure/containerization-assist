package session

import (
	"context"
	"strings"

	errors "github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// SessionLabelManager handles all session label operations
type SessionLabelManager struct {
	logger         zerolog.Logger
	sessionManager *SessionManager
}

// NewSessionLabelManager creates a new session label manager
func NewSessionLabelManager(logger zerolog.Logger, sessionManager *SessionManager) *SessionLabelManager {
	return &SessionLabelManager{
		logger:         logger,
		sessionManager: sessionManager,
	}
}

// LabelOperationArgs represents arguments for label operations
type LabelOperationArgs struct {
	types.BaseToolArgs
	Operation       string   `json:"operation" description:"Operation: add, remove, list, update"`
	TargetSessionID string   `json:"target_session_id,omitempty" description:"Target session ID (default: current)"`
	Label           string   `json:"label,omitempty" description:"Label for add/remove operations"`
	Labels          []string `json:"labels,omitempty" description:"Labels for update operation"`
}

// LabelOperationResult represents the result of label operations
type LabelOperationResult struct {
	types.BaseToolResponse
	Success         bool     `json:"success"`
	TargetSessionID string   `json:"target_session_id"`
	AllLabels       []string `json:"all_labels"`
	Message         string   `json:"message"`
}

// Execute handles all label operations
func (m *SessionLabelManager) Execute(ctx context.Context, args *LabelOperationArgs) (*LabelOperationResult, error) {
	targetSessionID := args.TargetSessionID
	if targetSessionID == "" {
		targetSessionID = args.SessionID
	}

	if targetSessionID == "" {
		return nil, errors.NewError().Messagef("session ID is required").Build()
	}

	session, err := m.sessionManager.GetSession(context.Background(), targetSessionID)
	if err != nil {
		return nil, err
	}

	switch args.Operation {
	case "add":
		return m.addLabel(session, args.Label)
	case "remove":
		return m.removeLabel(session, args.Label)
	case "list":
		return m.listLabels(session)
	case "update":
		return m.updateLabels(session, args.Labels)
	default:
		return nil, errors.NewError().Messagef("invalid operation: %s", args.Operation).Build()
	}
}

// addLabel adds a label to the session
func (m *SessionLabelManager) addLabel(session *SessionState, label string) (*LabelOperationResult, error) {
	if strings.TrimSpace(label) == "" {
		return nil, errors.NewError().Messagef("label cannot be empty").Build()
	}

	label = strings.TrimSpace(label)

	// Check if label already exists
	for _, existingLabel := range session.Labels {
		if existingLabel == label {
			return &LabelOperationResult{
				Success:         true,
				TargetSessionID: session.ID,
				AllLabels:       session.Labels,
				Message:         "Label already exists",
			}, nil
		}
	}

	// Add the label
	session.Labels = append(session.Labels, label)

	if err := m.sessionManager.SaveSession(context.Background(), session.SessionID, session); err != nil {
		return nil, err
	}

	return &LabelOperationResult{
		Success:         true,
		TargetSessionID: session.ID,
		AllLabels:       session.Labels,
		Message:         "Label added successfully",
	}, nil
}

// removeLabel removes a label from the session
func (m *SessionLabelManager) removeLabel(session *SessionState, label string) (*LabelOperationResult, error) {
	if strings.TrimSpace(label) == "" {
		return nil, errors.NewError().Messagef("label cannot be empty").Build()
	}

	label = strings.TrimSpace(label)
	found := false
	newLabels := make([]string, 0, len(session.Labels))

	for _, existingLabel := range session.Labels {
		if existingLabel != label {
			newLabels = append(newLabels, existingLabel)
		} else {
			found = true
		}
	}

	if !found {
		return &LabelOperationResult{
			Success:         true,
			TargetSessionID: session.ID,
			AllLabels:       session.Labels,
			Message:         "Label not found",
		}, nil
	}

	session.Labels = newLabels

	if err := m.sessionManager.SaveSession(context.Background(), session.SessionID, session); err != nil {
		return nil, err
	}

	return &LabelOperationResult{
		Success:         true,
		TargetSessionID: session.ID,
		AllLabels:       session.Labels,
		Message:         "Label removed successfully",
	}, nil
}

// listLabels returns all labels for the session
func (m *SessionLabelManager) listLabels(session *SessionState) (*LabelOperationResult, error) {
	return &LabelOperationResult{
		Success:         true,
		TargetSessionID: session.ID,
		AllLabels:       session.Labels,
		Message:         "Labels retrieved successfully",
	}, nil
}

// updateLabels replaces all labels for the session
func (m *SessionLabelManager) updateLabels(session *SessionState, labels []string) (*LabelOperationResult, error) {
	// Clean and validate labels
	cleanLabels := make([]string, 0, len(labels))
	for _, label := range labels {
		if cleaned := strings.TrimSpace(label); cleaned != "" {
			cleanLabels = append(cleanLabels, cleaned)
		}
	}

	session.Labels = cleanLabels

	if err := m.sessionManager.SaveSession(context.Background(), session.SessionID, session); err != nil {
		return nil, err
	}

	return &LabelOperationResult{
		Success:         true,
		TargetSessionID: session.ID,
		AllLabels:       session.Labels,
		Message:         "Labels updated successfully",
	}, nil
}
