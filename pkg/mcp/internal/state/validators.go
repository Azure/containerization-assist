package state

import (
	"context"
	"fmt"
	"reflect"

	"github.com/Azure/container-kit/pkg/mcp/internal/session"
)

// SessionStateValidator validates session state
type SessionStateValidator struct{}

// NewSessionStateValidator creates a new session state validator
func NewSessionStateValidator() StateValidator {
	return &SessionStateValidator{}
}

// ValidateState validates session state
func (v *SessionStateValidator) ValidateState(_ context.Context, stateType StateType, state interface{}) error {
	sessionState, ok := state.(*session.SessionState)
	if !ok {
		return fmt.Errorf("invalid state type: expected *session.SessionState, got %T", state)
	}

	// Validate required fields
	if sessionState.SessionID == "" {
		return fmt.Errorf("session ID is required")
	}

	if sessionState.CreatedAt.IsZero() {
		return fmt.Errorf("session creation time is required")
	}

	// Validate disk usage
	if sessionState.DiskUsage < 0 {
		return fmt.Errorf("disk usage cannot be negative")
	}

	if sessionState.MaxDiskUsage > 0 && sessionState.DiskUsage > sessionState.MaxDiskUsage {
		return fmt.Errorf("disk usage %d exceeds maximum allowed %d", sessionState.DiskUsage, sessionState.MaxDiskUsage)
	}

	return nil
}

// ConversationStateValidator validates conversation state
type ConversationStateValidator struct{}

// NewConversationStateValidator creates a new conversation state validator
func NewConversationStateValidator() StateValidator {
	return &ConversationStateValidator{}
}

// ValidateState validates conversation state
func (v *ConversationStateValidator) ValidateState(ctx context.Context, _ StateType, state interface{}) error {
	conversationState, ok := state.(*BasicConversationState)
	if !ok {
		return fmt.Errorf("invalid state type: expected *BasicConversationState, got %T", state)
	}

	// Validate session state (embedded)
	sessionValidator := NewSessionStateValidator()
	if err := sessionValidator.ValidateState(ctx, StateTypeSession, &conversationState.SessionState); err != nil {
		return fmt.Errorf("embedded session state validation failed: %v", err)
	}

	// Validate conversation-specific fields
	if conversationState.ConversationID == "" {
		return fmt.Errorf("conversation ID is required")
	}

	// Validate current stage
	if conversationState.CurrentStage != "" {
		validStages := map[string]bool{
			"planning":   true,
			"building":   true,
			"deploying":  true,
			"monitoring": true,
			"optimizing": true,
		}
		if !validStages[conversationState.CurrentStage] {
			return fmt.Errorf("invalid current stage: %s", conversationState.CurrentStage)
		}
	}

	// Basic validation completed - no retry count field in BasicConversationState
	return nil
}

// WorkflowStateValidator validates workflow state
type WorkflowStateValidator struct{}

// NewWorkflowStateValidator creates a new workflow state validator
func NewWorkflowStateValidator() StateValidator {
	return &WorkflowStateValidator{}
}

// ValidateState validates workflow state
func (v *WorkflowStateValidator) ValidateState(ctx context.Context, stateType StateType, state interface{}) error {
	workflowSession, ok := state.(WorkflowSessionInterface)
	if !ok {
		return fmt.Errorf("invalid state type: expected WorkflowSessionInterface, got %T", state)
	}

	// Validate required fields
	if workflowSession.GetSessionID() == "" {
		return fmt.Errorf("workflow session ID is required")
	}

	if workflowSession.GetCurrentStage() == "" {
		return fmt.Errorf("current stage is required")
	}

	// Validate progress is within valid range
	progress := workflowSession.GetProgress()
	if progress < 0 || progress > 100 {
		return fmt.Errorf("workflow progress out of range: %f (must be 0-100)", progress)
	}

	return nil
}

// ToolStateValidator validates tool-specific state
type ToolStateValidator struct {
	toolName   string
	validators map[string]func(interface{}) error
}

// NewToolStateValidator creates a new tool state validator
func NewToolStateValidator(toolName string) StateValidator {
	return &ToolStateValidator{
		toolName:   toolName,
		validators: make(map[string]func(interface{}) error),
	}
}

// AddFieldValidator adds a validator for a specific field
func (v *ToolStateValidator) AddFieldValidator(fieldName string, validator func(interface{}) error) {
	v.validators[fieldName] = validator
}

// ValidateState validates tool state
func (v *ToolStateValidator) ValidateState(ctx context.Context, stateType StateType, state interface{}) error {
	// Use reflection to validate fields
	stateValue := reflect.ValueOf(state)
	if stateValue.Kind() == reflect.Ptr {
		stateValue = stateValue.Elem()
	}

	if stateValue.Kind() != reflect.Struct {
		return fmt.Errorf("tool state must be a struct, got %v", stateValue.Kind())
	}

	// reflectType is not used, removing it
	// The actual validation is done on stateValue which is already dereferenced

	// Validate each field with a registered validator
	for fieldName, validator := range v.validators {
		field := stateValue.FieldByName(fieldName)
		if !field.IsValid() {
			return fmt.Errorf("field %s not found in tool state", fieldName)
		}

		if err := validator(field.Interface()); err != nil {
			return fmt.Errorf("validation failed for field %s: %w", fieldName, err)
		}
	}

	return nil
}

// GlobalStateValidator validates global state
type GlobalStateValidator struct {
	schema interface{}
}

// NewGlobalStateValidator creates a new global state validator
func NewGlobalStateValidator(schema interface{}) StateValidator {
	return &GlobalStateValidator{
		schema: schema,
	}
}

// ValidateState validates global state
func (v *GlobalStateValidator) ValidateState(ctx context.Context, stateType StateType, state interface{}) error {
	// For now, just check that state is not nil
	if state == nil {
		return fmt.Errorf("global state cannot be nil")
	}

	// Additional schema validation could be implemented here
	// using a JSON schema validator or similar

	return nil
}

// CompositeValidator combines multiple validators
type CompositeValidator struct {
	validators []StateValidator
}

// NewCompositeValidator creates a new composite validator
func NewCompositeValidator(validators ...StateValidator) StateValidator {
	return &CompositeValidator{
		validators: validators,
	}
}

// ValidateState runs all validators
func (v *CompositeValidator) ValidateState(ctx context.Context, stateType StateType, state interface{}) error {
	for i, validator := range v.validators {
		if err := validator.ValidateState(ctx, stateType, state); err != nil {
			return fmt.Errorf("validator %d failed: %w", i, err)
		}
	}
	return nil
}
