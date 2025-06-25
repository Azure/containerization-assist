package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
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

// Execute implements the unified Tool interface
func (t *AddSessionLabelTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	// Type assertion to get proper args
	addArgs, ok := args.(AddSessionLabelArgs)
	if !ok {
		return nil, types.NewRichError("INVALID_ARGUMENTS_TYPE", fmt.Sprintf("invalid arguments type: expected AddSessionLabelArgs, got %T", args), "validation_error")
	}

	return t.ExecuteTyped(ctx, addArgs)
}

// ExecuteTyped provides typed execution for backward compatibility
func (t *AddSessionLabelTool) ExecuteTyped(ctx context.Context, args AddSessionLabelArgs) (*AddSessionLabelResult, error) {
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

// Execute implements the unified Tool interface
func (t *RemoveSessionLabelTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	// Type assertion to get proper args
	removeArgs, ok := args.(RemoveSessionLabelArgs)
	if !ok {
		return nil, types.NewRichError("INVALID_ARGUMENTS_TYPE", fmt.Sprintf("invalid arguments type: expected RemoveSessionLabelArgs, got %T", args), "validation_error")
	}

	return t.ExecuteTyped(ctx, removeArgs)
}

// ExecuteTyped provides typed execution for backward compatibility
func (t *RemoveSessionLabelTool) ExecuteTyped(ctx context.Context, args RemoveSessionLabelArgs) (*RemoveSessionLabelResult, error) {
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

// Execute implements the unified Tool interface
func (t *UpdateSessionLabelsTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	// Type assertion to get proper args
	updateArgs, ok := args.(UpdateSessionLabelsArgs)
	if !ok {
		return nil, types.NewRichError("INVALID_ARGUMENTS_TYPE", fmt.Sprintf("invalid arguments type: expected UpdateSessionLabelsArgs, got %T", args), "validation_error")
	}

	return t.ExecuteTyped(ctx, updateArgs)
}

// ExecuteTyped provides typed execution for backward compatibility
func (t *UpdateSessionLabelsTool) ExecuteTyped(ctx context.Context, args UpdateSessionLabelsArgs) (*UpdateSessionLabelsResult, error) {
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

// Execute implements the unified Tool interface
func (t *ListSessionLabelsTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	// Type assertion to get proper args
	listArgs, ok := args.(ListSessionLabelsArgs)
	if !ok {
		return nil, types.NewRichError("INVALID_ARGUMENTS_TYPE", fmt.Sprintf("invalid arguments type: expected ListSessionLabelsArgs, got %T", args), "validation_error")
	}

	return t.ExecuteTyped(ctx, listArgs)
}

// ExecuteTyped provides typed execution for backward compatibility
func (t *ListSessionLabelsTool) ExecuteTyped(ctx context.Context, args ListSessionLabelsArgs) (*ListSessionLabelsResult, error) {
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

// GetMetadata returns comprehensive metadata about the add session label tool
func (t *AddSessionLabelTool) GetMetadata() mcptypes.ToolMetadata {
	return mcptypes.ToolMetadata{
		Name:        "add_session_label",
		Description: "Add a label to a session for categorization and filtering",
		Version:     "1.0.0",
		Category:    "Session Management",
		Dependencies: []string{
			"Session Manager",
			"Label Manager",
		},
		Capabilities: []string{
			"Label addition",
			"Session targeting",
			"Label validation",
			"Duplicate prevention",
		},
		Requirements: []string{
			"Valid session ID",
			"Non-empty label",
			"Session manager access",
		},
		Parameters: map[string]string{
			"target_session_id": "Optional: Target session ID (default: current session)",
			"label":             "Required: Label to add to the session",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "Add development label",
				Description: "Add a development label to current session",
				Input: map[string]interface{}{
					"label": "development",
				},
				Output: map[string]interface{}{
					"success":           true,
					"target_session_id": "session-123",
					"label":             "development",
					"all_labels":        []string{"development"},
					"message":           "Successfully added label 'development' to session session-123",
				},
			},
		},
	}
}

// Validate checks if the provided arguments are valid for the add session label tool
func (t *AddSessionLabelTool) Validate(ctx context.Context, args interface{}) error {
	addArgs, ok := args.(AddSessionLabelArgs)
	if !ok {
		return types.NewRichError("INVALID_ARGUMENTS_TYPE", fmt.Sprintf("invalid arguments type: expected AddSessionLabelArgs, got %T", args), "validation_error")
	}

	// Validate label
	if strings.TrimSpace(addArgs.Label) == "" {
		return types.NewRichError("LABEL_REQUIRED", "label is required and cannot be empty", "validation_error")
	}

	if len(addArgs.Label) > 100 {
		return types.NewRichError("LABEL_TOO_LONG", "label is too long (max 100 characters)", "validation_error")
	}

	// Validate session manager is available
	if t.sessionManager == nil {
		return types.NewRichError("SESSION_MANAGER_NOT_CONFIGURED", "session manager is not configured", "configuration_error")
	}

	return nil
}

// GetMetadata returns comprehensive metadata about the remove session label tool
func (t *RemoveSessionLabelTool) GetMetadata() mcptypes.ToolMetadata {
	return mcptypes.ToolMetadata{
		Name:        "remove_session_label",
		Description: "Remove a label from a session",
		Version:     "1.0.0",
		Category:    "Session Management",
		Dependencies: []string{
			"Session Manager",
			"Label Manager",
		},
		Capabilities: []string{
			"Label removal",
			"Session targeting",
			"Label validation",
		},
		Requirements: []string{
			"Valid session ID",
			"Existing label",
			"Session manager access",
		},
		Parameters: map[string]string{
			"target_session_id": "Optional: Target session ID (default: current session)",
			"label":             "Required: Label to remove from the session",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "Remove development label",
				Description: "Remove a development label from current session",
				Input: map[string]interface{}{
					"label": "development",
				},
				Output: map[string]interface{}{
					"success":           true,
					"target_session_id": "session-123",
					"label":             "development",
					"all_labels":        []string{},
					"message":           "Successfully removed label 'development' from session session-123",
				},
			},
		},
	}
}

// Validate checks if the provided arguments are valid for the remove session label tool
func (t *RemoveSessionLabelTool) Validate(ctx context.Context, args interface{}) error {
	removeArgs, ok := args.(RemoveSessionLabelArgs)
	if !ok {
		return types.NewRichError("INVALID_ARGUMENTS_TYPE", fmt.Sprintf("invalid arguments type: expected RemoveSessionLabelArgs, got %T", args), "validation_error")
	}

	// Validate label
	if strings.TrimSpace(removeArgs.Label) == "" {
		return types.NewRichError("LABEL_REQUIRED", "label is required and cannot be empty", "validation_error")
	}

	// Validate session manager is available
	if t.sessionManager == nil {
		return types.NewRichError("SESSION_MANAGER_NOT_CONFIGURED", "session manager is not configured", "configuration_error")
	}

	return nil
}

// GetMetadata returns comprehensive metadata about the update session labels tool
func (t *UpdateSessionLabelsTool) GetMetadata() mcptypes.ToolMetadata {
	return mcptypes.ToolMetadata{
		Name:        "update_session_labels",
		Description: "Update all labels on a session with a complete new set",
		Version:     "1.0.0",
		Category:    "Session Management",
		Dependencies: []string{
			"Session Manager",
			"Label Manager",
		},
		Capabilities: []string{
			"Bulk label update",
			"Label replacement",
			"Session targeting",
			"Label validation",
		},
		Requirements: []string{
			"Valid session ID",
			"Session manager access",
		},
		Parameters: map[string]string{
			"target_session_id": "Optional: Target session ID (default: current session)",
			"labels":            "Required: Complete set of labels to apply",
			"replace":           "Optional: Replace all existing labels (default: true)",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "Set production labels",
				Description: "Replace all labels with production environment labels",
				Input: map[string]interface{}{
					"labels":  []string{"production", "backend", "api"},
					"replace": true,
				},
				Output: map[string]interface{}{
					"success":           true,
					"target_session_id": "session-123",
					"previous_labels":   []string{"development"},
					"new_labels":        []string{"production", "backend", "api"},
					"message":           "Successfully updated labels for session session-123",
				},
			},
		},
	}
}

// Validate checks if the provided arguments are valid for the update session labels tool
func (t *UpdateSessionLabelsTool) Validate(ctx context.Context, args interface{}) error {
	updateArgs, ok := args.(UpdateSessionLabelsArgs)
	if !ok {
		return types.NewRichError("INVALID_ARGUMENTS_TYPE", fmt.Sprintf("invalid arguments type: expected UpdateSessionLabelsArgs, got %T", args), "validation_error")
	}

	// Validate labels array
	if len(updateArgs.Labels) > 50 {
		return types.NewRichError("TOO_MANY_LABELS", "too many labels (max 50)", "validation_error")
	}

	for _, label := range updateArgs.Labels {
		if strings.TrimSpace(label) == "" {
			return types.NewRichError("EMPTY_LABEL_IN_LIST", "labels cannot contain empty strings", "validation_error")
		}
		if len(label) > 100 {
			return types.NewRichError("LABEL_TOO_LONG", fmt.Sprintf("label '%s' is too long (max 100 characters)", label), "validation_error")
		}
	}

	// Validate session manager is available
	if t.sessionManager == nil {
		return types.NewRichError("SESSION_MANAGER_NOT_CONFIGURED", "session manager is not configured", "configuration_error")
	}

	return nil
}

// GetMetadata returns comprehensive metadata about the list session labels tool
func (t *ListSessionLabelsTool) GetMetadata() mcptypes.ToolMetadata {
	return mcptypes.ToolMetadata{
		Name:        "list_session_labels",
		Description: "List all labels across sessions with usage statistics",
		Version:     "1.0.0",
		Category:    "Session Management",
		Dependencies: []string{
			"Session Manager",
			"Label Manager",
		},
		Capabilities: []string{
			"Label enumeration",
			"Usage statistics",
			"Label counting",
			"Summary reporting",
		},
		Requirements: []string{
			"Session manager access",
		},
		Parameters: map[string]string{
			"include_count": "Optional: Include usage count for each label",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "List all labels with counts",
				Description: "Get all labels across sessions with usage statistics",
				Input: map[string]interface{}{
					"include_count": true,
				},
				Output: map[string]interface{}{
					"all_labels": []string{"development", "production", "backend", "frontend"},
					"label_counts": map[string]int{
						"development": 5,
						"production":  3,
						"backend":     4,
						"frontend":    2,
					},
					"summary": map[string]interface{}{
						"total_labels":               4,
						"total_sessions":             8,
						"labeled_sessions":           6,
						"unlabeled_sessions":         2,
						"average_labels_per_session": 1,
					},
				},
			},
		},
	}
}

// Validate checks if the provided arguments are valid for the list session labels tool
func (t *ListSessionLabelsTool) Validate(ctx context.Context, args interface{}) error {
	_, ok := args.(ListSessionLabelsArgs)
	if !ok {
		return types.NewRichError("INVALID_ARGUMENTS_TYPE", fmt.Sprintf("invalid arguments type: expected ListSessionLabelsArgs, got %T", args), "validation_error")
	}

	// Validate session manager is available
	if t.sessionManager == nil {
		return types.NewRichError("SESSION_MANAGER_NOT_CONFIGURED", "session manager is not configured", "configuration_error")
	}

	return nil
}
