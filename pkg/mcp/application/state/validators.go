package state

import (
	"context"
	"fmt"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors/codes"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
)

// SessionStateValidator validates session state
type SessionStateValidator struct{}

// NewSessionStateValidator creates a new session state validator
func NewSessionStateValidator() StateValidator {
	return &SessionStateValidator{}
}

// Validate validates session state
func (v *SessionStateValidator) Validate(state interface{}) error {
	return v.ValidateState(context.Background(), StateTypeSession, state)
}

// ValidateState validates session state
func (v *SessionStateValidator) ValidateState(_ context.Context, stateType StateType, state interface{}) error {
	sessionState, ok := state.(*session.SessionState)
	if !ok {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Message(fmt.Sprintf("Invalid state type: expected *session.SessionState, got %T", state)).
			Context("expected_type", "*session.SessionState").
			Context("actual_type", fmt.Sprintf("%T", state)).
			Context("component", "session_state_validator").
			Build()
	}

	if sessionState.SessionID == "" {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Message("Session ID is required").
			Context("field", "session_id").
			Context("component", "session_state_validator").
			Suggestion("Provide a valid session ID").
			Build()
	}

	if sessionState.CreatedAt.IsZero() {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Message("Session creation time is required").
			Context("field", "created_at").
			Context("component", "session_state_validator").
			Suggestion("Set session creation timestamp").
			Build()
	}

	if sessionState.DiskUsage < 0 {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Message("Disk usage cannot be negative").
			Context("field", "disk_usage").
			Context("value", sessionState.DiskUsage).
			Context("component", "session_state_validator").
			Suggestion("Ensure disk usage is a positive value").
			Build()
	}

	if sessionState.MaxDiskUsage > 0 && sessionState.DiskUsage > sessionState.MaxDiskUsage {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Message(fmt.Sprintf("Disk usage %d exceeds maximum allowed %d", sessionState.DiskUsage, sessionState.MaxDiskUsage)).
			Context("field", "disk_usage").
			Context("current_usage", sessionState.DiskUsage).
			Context("max_usage", sessionState.MaxDiskUsage).
			Context("component", "session_state_validator").
			Suggestion("Reduce disk usage or increase maximum limit").
			Build()
	}

	return nil
}

// GetRules returns validation rules
func (v *SessionStateValidator) GetRules() []ValidationRule {
	return []ValidationRule{
		{
			Name:     "session_id_required",
			Message:  "Session ID must be provided and non-empty",
			Severity: "error",
		},
		{
			Name:     "disk_usage_limit",
			Message:  "Disk usage must not exceed maximum limit",
			Severity: "error",
		},
	}
}

// ConversationStateValidator validates conversation state
type ConversationStateValidator struct{}

// NewConversationStateValidator creates a new conversation state validator
func NewConversationStateValidator() StateValidator {
	return &ConversationStateValidator{}
}

// Validate validates conversation state
func (v *ConversationStateValidator) Validate(state interface{}) error {
	return v.ValidateState(context.Background(), StateTypeConversation, state)
}

// ValidateState validates conversation state
func (v *ConversationStateValidator) ValidateState(ctx context.Context, _ StateType, state interface{}) error {
	conversationState, ok := state.(*BasicConversationState)
	if !ok {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Message(fmt.Sprintf("Invalid state type: expected *BasicConversationState, got %T", state)).
			Context("expected_type", "*BasicConversationState").
			Context("actual_type", fmt.Sprintf("%T", state)).
			Context("component", "conversation_state_validator").
			Build()
	}

	sessionValidator := NewSessionStateValidator()
	if err := sessionValidator.Validate(&conversationState.SessionState); err != nil {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Message("Embedded session state validation failed").
			Cause(err).
			Context("field", "session_state").
			Context("component", "conversation_state_validator").
			Suggestion("Fix embedded session state validation errors").
			Build()
	}

	if conversationState.ConversationID == "" {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Message("Conversation ID is required").
			Context("field", "conversation_id").
			Context("component", "conversation_state_validator").
			Suggestion("Provide a valid conversation ID").
			Build()
	}

	return nil
}

// GetRules returns validation rules
func (v *ConversationStateValidator) GetRules() []ValidationRule {
	return []ValidationRule{
		{
			Name:     "conversation_id_required",
			Message:  "Conversation ID must be provided and non-empty",
			Severity: "error",
		},
		{
			Name:     "session_state_valid",
			Message:  "Embedded session state must be valid",
			Severity: "error",
		},
	}
}

// WorkflowStateValidator validates workflow state
type WorkflowStateValidator struct{}

// NewWorkflowStateValidator creates a new workflow state validator
func NewWorkflowStateValidator() StateValidator {
	return &WorkflowStateValidator{}
}

// Validate validates workflow state
func (v *WorkflowStateValidator) Validate(state interface{}) error {
	return v.ValidateState(context.Background(), StateTypeWorkflow, state)
}

// ValidateState validates workflow state
func (v *WorkflowStateValidator) ValidateState(ctx context.Context, stateType StateType, state interface{}) error {
	if state == nil {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Message("State cannot be nil").
			Context("state_type", string(stateType)).
			Context("component", "workflow_state_validator").
			Build()
	}

	return nil
}

// GetRules returns validation rules
func (v *WorkflowStateValidator) GetRules() []ValidationRule {
	return []ValidationRule{
		{
			Name:     "state_not_nil",
			Message:  "State must not be nil",
			Severity: "error",
		},
	}
}

// StateValidationData represents data for state validation
type StateValidationData struct {
	StateType  string                 `json:"state_type"`
	StateValue interface{}            `json:"state_value"`
	Context    map[string]interface{} `json:"context,omitempty"`
}

// UnifiedSessionStateValidator implements unified validation for session state
type UnifiedSessionStateValidator struct {
	core.Validator
	sessionValidator *SessionStateValidator
}

// NewUnifiedSessionStateValidator creates a new unified session state validator
func NewUnifiedSessionStateValidator() core.Validator {
	return &UnifiedSessionStateValidator{
		sessionValidator: &SessionStateValidator{},
	}
}

// Validate implements the core.Validator interface for session state
func (v *UnifiedSessionStateValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.NonGenericResult {
	result := core.NewNonGenericResult("unified_session_state_validator", "1.0.0")

	var stateData *StateValidationData
	if mapped, ok := data.(map[string]interface{}); ok {
		stateValue, exists := mapped["state_value"]
		if !exists {
			result.AddError(&core.Error{
				Code:     "SESSION_STATE_VALIDATOR_001",
				Message:  "state_value field is required",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityHigh,
				Field:    "state_value",
			})
			return result
		}
		stateData = &StateValidationData{
			StateValue: stateValue,
			StateType:  "session",
		}
		if stType, ok := mapped["state_type"].(string); ok {
			stateData.StateType = stType
		}
		if ctx, ok := mapped["context"].(map[string]interface{}); ok {
			stateData.Context = ctx
		}
	} else if typed, ok := data.(*StateValidationData); ok {
		stateData = typed
	} else {
		stateData = &StateValidationData{
			StateValue: data,
			StateType:  "session",
		}
	}

	err := v.sessionValidator.ValidateState(ctx, StateTypeSession, stateData.StateValue)

	if err != nil {
		result.AddError(&core.Error{
			Code:     "SESSION_STATE_VALIDATOR_002",
			Message:  err.Error(),
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
			Context: map[string]interface{}{
				"original_error": err.Error(),
				"state_type":     stateData.StateType,
			},
		})
	} else {
		result.AddSuggestion("Session state validation passed successfully")
	}

	return result
}

// GetName returns the validator name
func (v *UnifiedSessionStateValidator) GetName() string {
	return "unified_session_state_validator"
}

// GetVersion returns the validator version
func (v *UnifiedSessionStateValidator) GetVersion() string {
	return "1.0.0"
}

// GetSupportedTypes returns the data types this validator can handle
func (v *UnifiedSessionStateValidator) GetSupportedTypes() []string {
	return []string{"session_state", "map[string]interface{}", "*StateValidationData"}
}

// ValidateSessionStateUnified provides a convenience method for unified session state validation
func ValidateSessionStateUnified(ctx context.Context, sessionState interface{}, options *core.ValidationOptions) *core.NonGenericResult {
	validator := NewUnifiedSessionStateValidator()
	return validator.Validate(ctx, sessionState, options)
}
