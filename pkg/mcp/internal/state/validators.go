package state

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/validation/core"
	"github.com/Azure/container-kit/pkg/mcp/validation/validators"
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

// UnifiedStateValidator bridges state validation with the unified validation framework
type UnifiedStateValidator struct {
	*validators.BaseValidatorImpl
	sessionValidator  *session.SessionValidator
	formatValidator   *validators.FormatValidator
	securityValidator *validators.SecurityValidator
}

// NewUnifiedStateValidator creates a new unified state validator
func NewUnifiedStateValidator() *UnifiedStateValidator {
	return &UnifiedStateValidator{
		BaseValidatorImpl: validators.NewBaseValidator("state", "1.0.0", []string{"state", "session", "workflow", "conversation"}),
		sessionValidator:  session.NewSessionValidator(100, 10*1024*1024*1024, 100*1024*1024*1024, 24*time.Hour),
		formatValidator:   validators.NewFormatValidator(),
		securityValidator: validators.NewSecurityValidator(),
	}
}

// ValidateStateUnified validates state using unified validation framework
func (v *UnifiedStateValidator) ValidateStateUnified(ctx context.Context, stateType StateType, state interface{}, options *core.ValidationOptions) *core.ValidationResult {
	startTime := time.Now()
	result := &core.ValidationResult{
		Valid:    true,
		Errors:   make([]*core.ValidationError, 0),
		Warnings: make([]*core.ValidationWarning, 0),
		Metadata: core.ValidationMetadata{
			ValidatedAt:      startTime,
			ValidatorName:    "unified-state-validator",
			ValidatorVersion: "1.0.0",
			Context:          make(map[string]interface{}),
		},
		Suggestions: make([]string, 0),
	}

	// Add state type to metadata
	result.Metadata.Context["state_type"] = string(stateType)

	switch stateType {
	case StateTypeSession:
		v.validateSessionStateUnified(ctx, state, result, options)
	case StateTypeWorkflow:
		v.validateWorkflowStateUnified(ctx, state, result, options)
	case StateTypeConversation:
		v.validateConversationStateUnified(ctx, state, result, options)
	case StateTypeTool:
		v.validateToolStateUnified(ctx, state, result, options)
	case StateTypeGlobal:
		v.validateGlobalStateUnified(ctx, state, result, options)
	default:
		result.AddError(&core.ValidationError{
			Code:     "UNKNOWN_STATE_TYPE",
			Message:  fmt.Sprintf("Unknown state type: %s", stateType),
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
			Field:    "state_type",
		})
	}

	result.Duration = time.Since(startTime)
	v.calculateStateValidationScore(result)

	return result
}

// ValidateState implements the legacy StateValidator interface for backward compatibility
func (v *UnifiedStateValidator) ValidateState(ctx context.Context, stateType StateType, state interface{}) error {
	options := core.NewValidationOptions().WithStrictMode(false)
	result := v.ValidateStateUnified(ctx, stateType, state, options)

	// Convert unified validation result to legacy error
	if !result.Valid {
		var errorMessages []string
		for _, err := range result.Errors {
			errorMessages = append(errorMessages, err.Message)
		}
		if len(errorMessages) > 0 {
			return fmt.Errorf("state validation failed: %s", errorMessages[0])
		}
	}

	return nil
}

// Specific state type validation methods

func (v *UnifiedStateValidator) validateSessionStateUnified(ctx context.Context, state interface{}, result *core.ValidationResult, options *core.ValidationOptions) {
	sessionState, ok := state.(*session.SessionState)
	if !ok {
		result.AddError(&core.ValidationError{
			Code:     "INVALID_SESSION_STATE_TYPE",
			Message:  fmt.Sprintf("Expected *session.SessionState, got %T", state),
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityCritical,
		})
		return
	}

	// Use the session validator for detailed validation
	sessionResult := v.sessionValidator.ValidateSessionState(ctx, sessionState, options)
	result.Merge(sessionResult)
}

func (v *UnifiedStateValidator) validateWorkflowStateUnified(ctx context.Context, state interface{}, result *core.ValidationResult, options *core.ValidationOptions) {
	workflowState, ok := state.(WorkflowSessionInterface)
	if !ok {
		result.AddError(&core.ValidationError{
			Code:     "INVALID_WORKFLOW_STATE_TYPE",
			Message:  fmt.Sprintf("Expected WorkflowSessionInterface, got %T", state),
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityCritical,
		})
		return
	}

	// Validate workflow-specific fields
	if workflowState.GetSessionID() == "" {
		result.AddError(&core.ValidationError{
			Code:     "MISSING_WORKFLOW_SESSION_ID",
			Message:  "Workflow session ID is required",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityCritical,
			Field:    "session_id",
		})
	}

	if workflowState.GetCurrentStage() == "" {
		result.AddError(&core.ValidationError{
			Code:     "MISSING_WORKFLOW_STAGE",
			Message:  "Current workflow stage is required",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
			Field:    "current_stage",
		})
	}

	// Validate progress
	progress := workflowState.GetProgress()
	if progress < 0 || progress > 100 {
		result.AddError(&core.ValidationError{
			Code:     "INVALID_WORKFLOW_PROGRESS",
			Message:  fmt.Sprintf("Workflow progress must be between 0-100, got %.2f", progress),
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
			Field:    "progress",
		})
	}

	// Validate completion consistency
	if workflowState.IsCompleted() && progress < 100 {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "INCONSISTENT_WORKFLOW_COMPLETION",
				Message:  "Workflow marked as completed but progress is not 100%",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityMedium,
				Field:    "completed",
			},
		})
	}

	// Validate timing
	if !workflowState.GetStartTime().IsZero() && workflowState.GetEndTime() != nil {
		if workflowState.GetEndTime().Before(workflowState.GetStartTime()) {
			result.AddError(&core.ValidationError{
				Code:     "INVALID_WORKFLOW_TIMING",
				Message:  "Workflow end time cannot be before start time",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityHigh,
				Field:    "end_time",
			})
		}
	}

	// Check for long-running workflows
	if !workflowState.GetStartTime().IsZero() && workflowState.GetEndTime() == nil {
		duration := time.Since(workflowState.GetStartTime())
		if duration > 4*time.Hour {
			result.AddWarning(&core.ValidationWarning{
				ValidationError: &core.ValidationError{
					Code:     "LONG_RUNNING_WORKFLOW",
					Message:  fmt.Sprintf("Workflow has been running for %v", duration),
					Type:     core.ErrTypeSystem,
					Severity: core.SeverityMedium,
					Field:    "start_time",
				},
			})
		}
	}
}

func (v *UnifiedStateValidator) validateConversationStateUnified(ctx context.Context, state interface{}, result *core.ValidationResult, options *core.ValidationOptions) {
	conversationState, ok := state.(*BasicConversationState)
	if !ok {
		result.AddError(&core.ValidationError{
			Code:     "INVALID_CONVERSATION_STATE_TYPE",
			Message:  fmt.Sprintf("Expected *BasicConversationState, got %T", state),
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityCritical,
		})
		return
	}

	// Validate embedded session state if it's available and of the right type
	if conversationState.SessionState != nil {
		if sessionState, ok := conversationState.SessionState.(*session.SessionState); ok {
			sessionResult := v.sessionValidator.ValidateSessionState(ctx, sessionState, options)
			// Add session validation errors with conversation context
			for _, err := range sessionResult.Errors {
				conversationErr := &core.ValidationError{
					Code:     err.Code,
					Message:  fmt.Sprintf("Embedded session state: %s", err.Message),
					Type:     err.Type,
					Severity: err.Severity,
					Field:    fmt.Sprintf("session_state.%s", err.Field),
					Context:  err.Context,
				}
				result.Errors = append(result.Errors, conversationErr)
			}
		} else {
			result.AddWarning(&core.ValidationWarning{
				ValidationError: &core.ValidationError{
					Code:     "UNKNOWN_SESSION_STATE_TYPE",
					Message:  fmt.Sprintf("Embedded session state has unexpected type: %T", conversationState.SessionState),
					Type:     core.ErrTypeValidation,
					Severity: core.SeverityLow,
					Field:    "session_state",
				},
			})
		}
	}

	// Validate conversation-specific fields
	if conversationState.ConversationID == "" {
		result.AddError(&core.ValidationError{
			Code:     "MISSING_CONVERSATION_ID",
			Message:  "Conversation ID is required",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityCritical,
			Field:    "conversation_id",
		})
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
			result.AddError(&core.ValidationError{
				Code:     "INVALID_CONVERSATION_STAGE",
				Message:  fmt.Sprintf("Invalid conversation stage: %s", conversationState.CurrentStage),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityHigh,
				Field:    "current_stage",
			})
		}
	}
}

func (v *UnifiedStateValidator) validateToolStateUnified(ctx context.Context, state interface{}, result *core.ValidationResult, options *core.ValidationOptions) {
	// Use reflection to validate tool state structure
	stateValue := reflect.ValueOf(state)
	if stateValue.Kind() == reflect.Ptr {
		stateValue = stateValue.Elem()
	}

	if stateValue.Kind() != reflect.Struct {
		result.AddError(&core.ValidationError{
			Code:     "INVALID_TOOL_STATE_TYPE",
			Message:  fmt.Sprintf("Tool state must be a struct, got %v", stateValue.Kind()),
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityHigh,
		})
		return
	}

	// Basic structure validation
	stateType := stateValue.Type()
	fieldCount := stateValue.NumField()

	if fieldCount == 0 {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "EMPTY_TOOL_STATE",
				Message:  "Tool state has no fields",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
			},
		})
	}

	// Validate common tool state patterns
	hasID := false
	hasTimestamp := false

	for i := 0; i < fieldCount; i++ {
		field := stateType.Field(i)
		fieldValue := stateValue.Field(i)

		// Check for common patterns
		if field.Name == "ID" || field.Name == "SessionID" || field.Name == "ToolID" {
			hasID = true
			if field.Type.Kind() == reflect.String && fieldValue.String() == "" {
				result.AddError(&core.ValidationError{
					Code:     "MISSING_TOOL_ID",
					Message:  fmt.Sprintf("Tool state field '%s' cannot be empty", field.Name),
					Type:     core.ErrTypeValidation,
					Severity: core.SeverityHigh,
					Field:    field.Name,
				})
			}
		}

		if field.Name == "CreatedAt" || field.Name == "UpdatedAt" || field.Name == "Timestamp" {
			hasTimestamp = true
			if field.Type == reflect.TypeOf(time.Time{}) {
				timeVal := fieldValue.Interface().(time.Time)
				if timeVal.IsZero() {
					result.AddWarning(&core.ValidationWarning{
						ValidationError: &core.ValidationError{
							Code:     "MISSING_TOOL_TIMESTAMP",
							Message:  fmt.Sprintf("Tool state timestamp field '%s' is not set", field.Name),
							Type:     core.ErrTypeValidation,
							Severity: core.SeverityLow,
							Field:    field.Name,
						},
					})
				}
			}
		}
	}

	if !hasID {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "NO_TOOL_IDENTIFIER",
				Message:  "Tool state lacks identifier fields (ID, SessionID, ToolID)",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityMedium,
			},
		})
	}

	if !hasTimestamp {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "NO_TOOL_TIMESTAMP",
				Message:  "Tool state lacks timestamp fields",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
			},
		})
	}
}

func (v *UnifiedStateValidator) validateGlobalStateUnified(ctx context.Context, state interface{}, result *core.ValidationResult, options *core.ValidationOptions) {
	if state == nil {
		result.AddError(&core.ValidationError{
			Code:     "NULL_GLOBAL_STATE",
			Message:  "Global state cannot be null",
			Type:     core.ErrTypeValidation,
			Severity: core.SeverityCritical,
		})
		return
	}

	// Use reflection to validate global state structure
	stateValue := reflect.ValueOf(state)
	if stateValue.Kind() == reflect.Ptr {
		stateValue = stateValue.Elem()
	}

	// Check for reasonable state structure
	switch stateValue.Kind() {
	case reflect.Map:
		// Validate map-based global state
		v.validateGlobalStateMap(stateValue, result)
	case reflect.Struct:
		// Validate struct-based global state
		v.validateGlobalStateStruct(stateValue, result)
	default:
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "UNUSUAL_GLOBAL_STATE_TYPE",
				Message:  fmt.Sprintf("Global state has unusual type: %v", stateValue.Kind()),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
			},
		})
	}
}

func (v *UnifiedStateValidator) validateGlobalStateMap(stateValue reflect.Value, result *core.ValidationResult) {
	if stateValue.Len() == 0 {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "EMPTY_GLOBAL_STATE",
				Message:  "Global state map is empty",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
			},
		})
		return
	}

	// Check for very large global state
	if stateValue.Len() > 1000 {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "LARGE_GLOBAL_STATE",
				Message:  fmt.Sprintf("Global state map has %d entries, consider optimization", stateValue.Len()),
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityMedium,
			},
		})
	}

	// Check for sensitive keys
	sensitivePatterns := []string{"password", "secret", "key", "token", "credential"}
	for _, key := range stateValue.MapKeys() {
		if key.Kind() == reflect.String {
			keyStr := key.String()
			for _, pattern := range sensitivePatterns {
				if fmt.Sprintf("%v", keyStr) == pattern {
					result.AddWarning(&core.ValidationWarning{
						ValidationError: &core.ValidationError{
							Code:     "SENSITIVE_GLOBAL_STATE_KEY",
							Message:  fmt.Sprintf("Global state contains potentially sensitive key: %s", keyStr),
							Type:     core.ErrTypeSecurity,
							Severity: core.SeverityMedium,
							Field:    keyStr,
						},
					})
				}
			}
		}
	}
}

func (v *UnifiedStateValidator) validateGlobalStateStruct(stateValue reflect.Value, result *core.ValidationResult) {
	stateType := stateValue.Type()
	fieldCount := stateValue.NumField()

	if fieldCount == 0 {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "EMPTY_GLOBAL_STATE_STRUCT",
				Message:  "Global state struct has no fields",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityLow,
			},
		})
		return
	}

	// Check for version field
	hasVersion := false
	for i := 0; i < fieldCount; i++ {
		field := stateType.Field(i)
		if field.Name == "Version" || field.Name == "SchemaVersion" {
			hasVersion = true
			break
		}
	}

	if !hasVersion {
		result.AddWarning(&core.ValidationWarning{
			ValidationError: &core.ValidationError{
				Code:     "NO_GLOBAL_STATE_VERSION",
				Message:  "Global state lacks version information",
				Type:     core.ErrTypeValidation,
				Severity: core.SeverityMedium,
			},
		})
	}
}

func (v *UnifiedStateValidator) calculateStateValidationScore(result *core.ValidationResult) {
	score := 100.0

	// Deduct points for errors
	for _, err := range result.Errors {
		switch err.Severity {
		case core.SeverityCritical:
			score -= 30
		case core.SeverityHigh:
			score -= 20
		case core.SeverityMedium:
			score -= 10
		case core.SeverityLow:
			score -= 5
		}
	}

	// Deduct points for warnings
	for _, warning := range result.Warnings {
		switch warning.Severity {
		case core.SeverityHigh:
			score -= 10
		case core.SeverityMedium:
			score -= 5
		case core.SeverityLow:
			score -= 2
		}
	}

	if score < 0 {
		score = 0
	}

	result.Score = score

	// Set risk level
	hasCritical := false
	for _, err := range result.Errors {
		if err.Severity == core.SeverityCritical {
			hasCritical = true
			break
		}
	}

	if hasCritical || score < 40 {
		result.RiskLevel = "critical"
	} else if score < 60 {
		result.RiskLevel = "high"
	} else if score < 80 {
		result.RiskLevel = "medium"
	} else {
		result.RiskLevel = "low"
	}

	// Update validity
	result.Valid = len(result.Errors) == 0
}

// Implement core.Validator interface

func (v *UnifiedStateValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.ValidationResult {
	// Try to determine state type from data
	var stateType StateType

	switch data.(type) {
	case *session.SessionState, session.SessionState:
		stateType = StateTypeSession
	case WorkflowSessionInterface, *BasicWorkflowSession, BasicWorkflowSession:
		stateType = StateTypeWorkflow
	case *BasicConversationState, BasicConversationState:
		stateType = StateTypeConversation
	default:
		// Default to tool state for unknown types
		stateType = StateTypeTool
	}

	return v.ValidateStateUnified(ctx, stateType, data, options)
}

// Enhanced composite validator with unified validation support
type UnifiedCompositeValidator struct {
	*CompositeValidator
	unifiedValidator *UnifiedStateValidator
}

// NewUnifiedCompositeValidator creates a composite validator with unified validation
func NewUnifiedCompositeValidator(validators ...StateValidator) *UnifiedCompositeValidator {
	return &UnifiedCompositeValidator{
		CompositeValidator: &CompositeValidator{validators: validators},
		unifiedValidator:   NewUnifiedStateValidator(),
	}
}

// ValidateStateUnified validates state using both legacy and unified validators
func (v *UnifiedCompositeValidator) ValidateStateUnified(ctx context.Context, stateType StateType, state interface{}, options *core.ValidationOptions) *core.ValidationResult {
	// Run unified validation
	result := v.unifiedValidator.ValidateStateUnified(ctx, stateType, state, options)

	// Run legacy validators and convert errors to warnings
	for i, validator := range v.validators {
		if err := validator.ValidateState(ctx, stateType, state); err != nil {
			result.AddWarning(&core.ValidationWarning{
				ValidationError: &core.ValidationError{
					Code:     "LEGACY_VALIDATOR_ERROR",
					Message:  fmt.Sprintf("Legacy validator %d: %s", i, err.Error()),
					Type:     core.ErrTypeValidation,
					Severity: core.SeverityMedium,
				},
			})
		}
	}

	return result
}

// Public unified validation functions

// ValidateSessionStateUnified validates session state using unified validation
func ValidateSessionStateUnified(state *session.SessionState) *core.ValidationResult {
	validator := NewUnifiedStateValidator()
	ctx := context.Background()
	options := core.NewValidationOptions()

	return validator.ValidateStateUnified(ctx, StateTypeSession, state, options)
}

// ValidateWorkflowStateUnified validates workflow state using unified validation
func ValidateWorkflowStateUnified(state WorkflowSessionInterface) *core.ValidationResult {
	validator := NewUnifiedStateValidator()
	ctx := context.Background()
	options := core.NewValidationOptions()

	return validator.ValidateStateUnified(ctx, StateTypeWorkflow, state, options)
}

// ValidateConversationStateUnified validates conversation state using unified validation
func ValidateConversationStateUnified(state *BasicConversationState) *core.ValidationResult {
	validator := NewUnifiedStateValidator()
	ctx := context.Background()
	options := core.NewValidationOptions()

	return validator.ValidateStateUnified(ctx, StateTypeConversation, state, options)
}

// ValidateToolStateUnified validates tool state using unified validation
func ValidateToolStateUnified(state interface{}) *core.ValidationResult {
	validator := NewUnifiedStateValidator()
	ctx := context.Background()
	options := core.NewValidationOptions()

	return validator.ValidateStateUnified(ctx, StateTypeTool, state, options)
}

// ValidateGlobalStateUnified validates global state using unified validation
func ValidateGlobalStateUnified(state interface{}) *core.ValidationResult {
	validator := NewUnifiedStateValidator()
	ctx := context.Background()
	options := core.NewValidationOptions()

	return validator.ValidateStateUnified(ctx, StateTypeGlobal, state, options)
}
