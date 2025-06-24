package tools

import (
	"context"
	"strings"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
)

// AddSessionLabelArgs represents arguments for adding a label to a session
type AddSessionLabelArgs struct {
	types.BaseToolArgs
	TargetSessionID string `json:"target_session_id,omitempty" description:"Target session ID (default: current session)"`
	Label           string `json:"label" description:"Label to add to the session"`
}

// AddSessionLabelResult represents the result of adding a label
type AddSessionLabelResult struct {
	types.BaseToolResponse
	Success         bool     `json:"success"`
	TargetSessionID string   `json:"target_session_id"`
	Label           string   `json:"label"`
	AllLabels       []string `json:"all_labels"`
	Message         string   `json:"message"`
}

// RemoveSessionLabelArgs represents arguments for removing a label from a session
type RemoveSessionLabelArgs struct {
	types.BaseToolArgs
	TargetSessionID string `json:"target_session_id,omitempty" description:"Target session ID (default: current session)"`
	Label           string `json:"label" description:"Label to remove from the session"`
}

// RemoveSessionLabelResult represents the result of removing a label
type RemoveSessionLabelResult struct {
	types.BaseToolResponse
	Success         bool     `json:"success"`
	TargetSessionID string   `json:"target_session_id"`
	Label           string   `json:"label"`
	AllLabels       []string `json:"all_labels"`
	Message         string   `json:"message"`
}

// UpdateSessionLabelsArgs represents arguments for updating all labels on a session
type UpdateSessionLabelsArgs struct {
	types.BaseToolArgs
	TargetSessionID string   `json:"target_session_id,omitempty" description:"Target session ID (default: current session)"`
	Labels          []string `json:"labels" description:"Complete set of labels to apply to the session"`
	Replace         bool     `json:"replace,omitempty" description:"Replace all existing labels (default: true)"`
}

// UpdateSessionLabelsResult represents the result of updating labels
type UpdateSessionLabelsResult struct {
	types.BaseToolResponse
	Success         bool     `json:"success"`
	TargetSessionID string   `json:"target_session_id"`
	PreviousLabels  []string `json:"previous_labels"`
	NewLabels       []string `json:"new_labels"`
	Message         string   `json:"message"`
}

// ListSessionLabelsArgs represents arguments for listing all labels across sessions
type ListSessionLabelsArgs struct {
	types.BaseToolArgs
	IncludeCount bool `json:"include_count,omitempty" description:"Include usage count for each label"`
}

// ListSessionLabelsResult represents the result of listing labels
type ListSessionLabelsResult struct {
	types.BaseToolResponse
	AllLabels   []string               `json:"all_labels"`
	LabelCounts map[string]int         `json:"label_counts,omitempty"`
	Summary     SessionLabelingSummary `json:"summary"`
}

// SessionLabelingSummary provides statistics about label usage
type SessionLabelingSummary struct {
	TotalLabels       int `json:"total_labels"`
	TotalSessions     int `json:"total_sessions"`
	LabeledSessions   int `json:"labeled_sessions"`
	UnlabeledSessions int `json:"unlabeled_sessions"`
	AverageLabels     int `json:"average_labels_per_session"`
}

// SessionLabelManager interface for managing session labels
type SessionLabelManager interface {
	AddSessionLabel(sessionID, label string) error
	RemoveSessionLabel(sessionID, label string) error
	SetSessionLabels(sessionID string, labels []string) error
	GetSession(sessionID string) (SessionLabelData, error)
	GetAllLabels() []string
	ListSessions() []SessionLabelData
}

// SessionLabelData represents minimal session data needed for label management
type SessionLabelData struct {
	SessionID string
	Labels    []string
}

// AddSessionLabelTool implements adding labels to sessions
type AddSessionLabelTool struct {
	logger         zerolog.Logger
	sessionManager SessionLabelManager
}

// NewAddSessionLabelTool creates a new add session label tool
func NewAddSessionLabelTool(logger zerolog.Logger, sessionManager SessionLabelManager) *AddSessionLabelTool {
	return &AddSessionLabelTool{
		logger:         logger,
		sessionManager: sessionManager,
	}
}

// Execute adds a label to a session
func (t *AddSessionLabelTool) Execute(ctx context.Context, args AddSessionLabelArgs) (*AddSessionLabelResult, error) {
	targetSessionID := args.TargetSessionID
	if targetSessionID == "" {
		targetSessionID = args.SessionID
	}

	if targetSessionID == "" {
		return nil, types.NewRichError("SESSION_ID_REQUIRED", "session ID is required", types.ErrTypeValidation)
	}

	if strings.TrimSpace(args.Label) == "" {
		return nil, types.NewRichError("LABEL_EMPTY", "label cannot be empty", types.ErrTypeValidation)
	}

	label := strings.TrimSpace(args.Label)

	t.logger.Info().
		Str("target_session_id", targetSessionID).
		Str("label", label).
		Msg("Adding label to session")

	// Add the label
	err := t.sessionManager.AddSessionLabel(targetSessionID, label)
	if err != nil {
		return &AddSessionLabelResult{
			BaseToolResponse: types.NewBaseResponse("add_session_label", args.SessionID, args.DryRun),
			Success:          false,
			TargetSessionID:  targetSessionID,
			Label:            label,
			Message:          "Failed to add label: " + err.Error(),
		}, err
	}

	// Get updated session data
	session, err := t.sessionManager.GetSession(targetSessionID)
	if err != nil {
		return &AddSessionLabelResult{
			BaseToolResponse: types.NewBaseResponse("add_session_label", args.SessionID, args.DryRun),
			Success:          false,
			TargetSessionID:  targetSessionID,
			Label:            label,
			Message:          "Label added but failed to retrieve updated session: " + err.Error(),
		}, nil
	}

	result := &AddSessionLabelResult{
		BaseToolResponse: types.NewBaseResponse("add_session_label", args.SessionID, args.DryRun),
		Success:          true,
		TargetSessionID:  targetSessionID,
		Label:            label,
		AllLabels:        session.Labels,
		Message:          "Successfully added label '" + label + "' to session " + targetSessionID,
	}

	t.logger.Info().
		Str("target_session_id", targetSessionID).
		Str("label", label).
		Strs("all_labels", session.Labels).
		Msg("Label added successfully")

	return result, nil
}

// RemoveSessionLabelTool implements removing labels from sessions
type RemoveSessionLabelTool struct {
	logger         zerolog.Logger
	sessionManager SessionLabelManager
}

// NewRemoveSessionLabelTool creates a new remove session label tool
func NewRemoveSessionLabelTool(logger zerolog.Logger, sessionManager SessionLabelManager) *RemoveSessionLabelTool {
	return &RemoveSessionLabelTool{
		logger:         logger,
		sessionManager: sessionManager,
	}
}

// Execute removes a label from a session
func (t *RemoveSessionLabelTool) Execute(ctx context.Context, args RemoveSessionLabelArgs) (*RemoveSessionLabelResult, error) {
	targetSessionID := args.TargetSessionID
	if targetSessionID == "" {
		targetSessionID = args.SessionID
	}

	if targetSessionID == "" {
		return nil, types.NewRichError("SESSION_ID_REQUIRED", "session ID is required", types.ErrTypeValidation)
	}

	if strings.TrimSpace(args.Label) == "" {
		return nil, types.NewRichError("LABEL_EMPTY", "label cannot be empty", types.ErrTypeValidation)
	}

	label := strings.TrimSpace(args.Label)

	t.logger.Info().
		Str("target_session_id", targetSessionID).
		Str("label", label).
		Msg("Removing label from session")

	// Remove the label
	err := t.sessionManager.RemoveSessionLabel(targetSessionID, label)
	if err != nil {
		return &RemoveSessionLabelResult{
			BaseToolResponse: types.NewBaseResponse("remove_session_label", args.SessionID, args.DryRun),
			Success:          false,
			TargetSessionID:  targetSessionID,
			Label:            label,
			Message:          "Failed to remove label: " + err.Error(),
		}, err
	}

	// Get updated session data
	session, err := t.sessionManager.GetSession(targetSessionID)
	if err != nil {
		return &RemoveSessionLabelResult{
			BaseToolResponse: types.NewBaseResponse("remove_session_label", args.SessionID, args.DryRun),
			Success:          false,
			TargetSessionID:  targetSessionID,
			Label:            label,
			Message:          "Label removed but failed to retrieve updated session: " + err.Error(),
		}, nil
	}

	result := &RemoveSessionLabelResult{
		BaseToolResponse: types.NewBaseResponse("remove_session_label", args.SessionID, args.DryRun),
		Success:          true,
		TargetSessionID:  targetSessionID,
		Label:            label,
		AllLabels:        session.Labels,
		Message:          "Successfully removed label '" + label + "' from session " + targetSessionID,
	}

	t.logger.Info().
		Str("target_session_id", targetSessionID).
		Str("label", label).
		Strs("all_labels", session.Labels).
		Msg("Label removed successfully")

	return result, nil
}

// UpdateSessionLabelsTool implements updating all labels on a session
type UpdateSessionLabelsTool struct {
	logger         zerolog.Logger
	sessionManager SessionLabelManager
}

// NewUpdateSessionLabelsTool creates a new update session labels tool
func NewUpdateSessionLabelsTool(logger zerolog.Logger, sessionManager SessionLabelManager) *UpdateSessionLabelsTool {
	return &UpdateSessionLabelsTool{
		logger:         logger,
		sessionManager: sessionManager,
	}
}

// Execute updates all labels on a session
func (t *UpdateSessionLabelsTool) Execute(ctx context.Context, args UpdateSessionLabelsArgs) (*UpdateSessionLabelsResult, error) {
	targetSessionID := args.TargetSessionID
	if targetSessionID == "" {
		targetSessionID = args.SessionID
	}

	if targetSessionID == "" {
		return nil, types.NewRichError("SESSION_ID_REQUIRED", "session ID is required", types.ErrTypeValidation)
	}

	// Clean up labels (trim whitespace and remove empty strings)
	cleanLabels := make([]string, 0, len(args.Labels))
	for _, label := range args.Labels {
		if cleaned := strings.TrimSpace(label); cleaned != "" {
			cleanLabels = append(cleanLabels, cleaned)
		}
	}

	t.logger.Info().
		Str("target_session_id", targetSessionID).
		Strs("new_labels", cleanLabels).
		Bool("replace", args.Replace).
		Msg("Updating session labels")

	// Get current session data
	currentSession, err := t.sessionManager.GetSession(targetSessionID)
	if err != nil {
		return &UpdateSessionLabelsResult{
			BaseToolResponse: types.NewBaseResponse("update_session_labels", args.SessionID, args.DryRun),
			Success:          false,
			TargetSessionID:  targetSessionID,
			Message:          "Failed to get current session: " + err.Error(),
		}, err
	}

	previousLabels := make([]string, len(currentSession.Labels))
	copy(previousLabels, currentSession.Labels)

	// Update the labels
	err = t.sessionManager.SetSessionLabels(targetSessionID, cleanLabels)
	if err != nil {
		return &UpdateSessionLabelsResult{
			BaseToolResponse: types.NewBaseResponse("update_session_labels", args.SessionID, args.DryRun),
			Success:          false,
			TargetSessionID:  targetSessionID,
			PreviousLabels:   previousLabels,
			Message:          "Failed to update labels: " + err.Error(),
		}, err
	}

	result := &UpdateSessionLabelsResult{
		BaseToolResponse: types.NewBaseResponse("update_session_labels", args.SessionID, args.DryRun),
		Success:          true,
		TargetSessionID:  targetSessionID,
		PreviousLabels:   previousLabels,
		NewLabels:        cleanLabels,
		Message:          "Successfully updated labels for session " + targetSessionID,
	}

	t.logger.Info().
		Str("target_session_id", targetSessionID).
		Strs("previous_labels", previousLabels).
		Strs("new_labels", cleanLabels).
		Msg("Labels updated successfully")

	return result, nil
}

// ListSessionLabelsTool implements listing all labels across sessions
type ListSessionLabelsTool struct {
	logger         zerolog.Logger
	sessionManager SessionLabelManager
}

// NewListSessionLabelsTool creates a new list session labels tool
func NewListSessionLabelsTool(logger zerolog.Logger, sessionManager SessionLabelManager) *ListSessionLabelsTool {
	return &ListSessionLabelsTool{
		logger:         logger,
		sessionManager: sessionManager,
	}
}

// Execute lists all labels across sessions with optional usage statistics
func (t *ListSessionLabelsTool) Execute(ctx context.Context, args ListSessionLabelsArgs) (*ListSessionLabelsResult, error) {
	t.logger.Info().
		Bool("include_count", args.IncludeCount).
		Msg("Listing session labels")

	// Get all labels
	allLabels := t.sessionManager.GetAllLabels()

	result := &ListSessionLabelsResult{
		BaseToolResponse: types.NewBaseResponse("list_session_labels", args.SessionID, args.DryRun),
		AllLabels:        allLabels,
	}

	// Calculate label counts and summary if requested
	if args.IncludeCount {
		sessions := t.sessionManager.ListSessions()
		labelCounts := make(map[string]int)
		labeledSessions := 0
		totalLabels := 0

		for _, session := range sessions {
			if len(session.Labels) > 0 {
				labeledSessions++
			}
			totalLabels += len(session.Labels)

			for _, label := range session.Labels {
				labelCounts[label]++
			}
		}

		result.LabelCounts = labelCounts
		result.Summary = SessionLabelingSummary{
			TotalLabels:       len(allLabels),
			TotalSessions:     len(sessions),
			LabeledSessions:   labeledSessions,
			UnlabeledSessions: len(sessions) - labeledSessions,
			AverageLabels:     0,
		}

		if len(sessions) > 0 {
			result.Summary.AverageLabels = totalLabels / len(sessions)
		}
	}

	t.logger.Info().
		Int("total_labels", len(allLabels)).
		Int("labeled_sessions", result.Summary.LabeledSessions).
		Msg("Labels listed successfully")

	return result, nil
}
