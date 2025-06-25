package types

import (
	"time"
)

// ErrorBuilder provides a fluent interface for building RichError instances
type ErrorBuilder struct {
	err *RichError
}

// NewErrorBuilder creates a new error builder
func NewErrorBuilder(code, message, errorType string) *ErrorBuilder {
	return &ErrorBuilder{
		err: NewRichError(code, message, errorType),
	}
}

// WithSeverity sets the error severity
func (b *ErrorBuilder) WithSeverity(severity string) *ErrorBuilder {
	b.err.Severity = severity
	return b
}

// WithOperation sets the operation context
func (b *ErrorBuilder) WithOperation(operation string) *ErrorBuilder {
	b.err.Context.Operation = operation
	return b
}

// WithStage sets the stage context
func (b *ErrorBuilder) WithStage(stage string) *ErrorBuilder {
	b.err.Context.Stage = stage
	return b
}

// WithComponent sets the component context
func (b *ErrorBuilder) WithComponent(component string) *ErrorBuilder {
	b.err.Context.Component = component
	return b
}

// WithInput sets the input that caused the error
func (b *ErrorBuilder) WithInput(input map[string]interface{}) *ErrorBuilder {
	b.err.Context.Input = input
	return b
}

// WithField adds a single input field
func (b *ErrorBuilder) WithField(key string, value interface{}) *ErrorBuilder {
	if b.err.Context.Input == nil {
		b.err.Context.Input = make(map[string]interface{})
	}
	b.err.Context.Input[key] = value
	return b
}

// WithPartialOutput sets any partial output
func (b *ErrorBuilder) WithPartialOutput(output map[string]interface{}) *ErrorBuilder {
	b.err.Context.PartialOutput = output
	return b
}

// WithRelatedFiles sets files involved in the error
func (b *ErrorBuilder) WithRelatedFiles(files ...string) *ErrorBuilder {
	b.err.Context.RelatedFiles = files
	return b
}

// WithRootCause sets the identified root cause
func (b *ErrorBuilder) WithRootCause(cause string) *ErrorBuilder {
	b.err.Diagnostics.RootCause = cause
	return b
}

// WithCategory sets the error category
func (b *ErrorBuilder) WithCategory(category string) *ErrorBuilder {
	b.err.Diagnostics.Category = category
	return b
}

// WithSymptoms adds observed symptoms
func (b *ErrorBuilder) WithSymptoms(symptoms ...string) *ErrorBuilder {
	b.err.Diagnostics.Symptoms = append(b.err.Diagnostics.Symptoms, symptoms...)
	return b
}

// WithDiagnosticCheck adds a diagnostic check result
func (b *ErrorBuilder) WithDiagnosticCheck(name string, passed bool, message string) *ErrorBuilder {
	check := DiagnosticCheck{
		Name:    name,
		Passed:  passed,
		Message: message,
	}
	b.err.Diagnostics.Checks = append(b.err.Diagnostics.Checks, check)
	return b
}

// WithImmediateStep adds an immediate resolution step
func (b *ErrorBuilder) WithImmediateStep(order int, action, description string) *ErrorBuilder {
	step := ResolutionStep{
		Order:       order,
		Action:      action,
		Description: description,
	}
	b.err.Resolution.ImmediateSteps = append(b.err.Resolution.ImmediateSteps, step)
	return b
}

// WithCommand adds a resolution step with a command
func (b *ErrorBuilder) WithCommand(order int, action, description, command, expected string) *ErrorBuilder {
	step := ResolutionStep{
		Order:       order,
		Action:      action,
		Description: description,
		Command:     command,
		Expected:    expected,
	}
	b.err.Resolution.ImmediateSteps = append(b.err.Resolution.ImmediateSteps, step)
	return b
}

// WithToolCall adds a resolution step with a tool call
func (b *ErrorBuilder) WithToolCall(order int, action, description, toolCall, expected string) *ErrorBuilder {
	step := ResolutionStep{
		Order:       order,
		Action:      action,
		Description: description,
		ToolCall:    toolCall,
		Expected:    expected,
	}
	b.err.Resolution.ImmediateSteps = append(b.err.Resolution.ImmediateSteps, step)
	return b
}

// WithAlternative adds an alternative approach
func (b *ErrorBuilder) WithAlternative(name, description string, steps []string) *ErrorBuilder {
	alt := Alternative{
		Name:        name,
		Description: description,
		Steps:       steps,
		Confidence:  0.7, // Default confidence
	}
	b.err.Resolution.Alternatives = append(b.err.Resolution.Alternatives, alt)
	return b
}

// WithPrevention adds prevention suggestions
func (b *ErrorBuilder) WithPrevention(prevention ...string) *ErrorBuilder {
	b.err.Resolution.Prevention = append(b.err.Resolution.Prevention, prevention...)
	return b
}

// WithRetryStrategy sets the retry strategy
func (b *ErrorBuilder) WithRetryStrategy(recommended bool, waitTime time.Duration, maxAttempts int) *ErrorBuilder {
	b.err.Resolution.RetryStrategy = RetryStrategy{
		Recommended:     recommended,
		WaitTime:        waitTime,
		MaxAttempts:     maxAttempts,
		BackoffStrategy: "exponential",
	}
	return b
}

// WithManualSteps adds manual intervention steps
func (b *ErrorBuilder) WithManualSteps(steps ...string) *ErrorBuilder {
	b.err.Resolution.ManualSteps = append(b.err.Resolution.ManualSteps, steps...)
	return b
}

// WithSessionState captures session state at error time
func (b *ErrorBuilder) WithSessionState(sessionID, currentStage string, completedStages []string) *ErrorBuilder {
	b.err.SessionState = &SessionStateSnapshot{
		ID:              sessionID,
		CurrentStage:    currentStage,
		CompletedStages: completedStages,
		Metadata:        make(map[string]interface{}),
	}
	return b
}

// WithTool sets the tool that generated the error
func (b *ErrorBuilder) WithTool(tool string) *ErrorBuilder {
	b.err.Tool = tool
	return b
}

// WithAttemptNumber sets the attempt number
func (b *ErrorBuilder) WithAttemptNumber(attempt int) *ErrorBuilder {
	b.err.AttemptNumber = attempt
	return b
}

// WithPreviousErrors adds previous error messages
func (b *ErrorBuilder) WithPreviousErrors(errors ...string) *ErrorBuilder {
	b.err.PreviousErrors = append(b.err.PreviousErrors, errors...)
	return b
}

// WithSystemState sets the system state
func (b *ErrorBuilder) WithSystemState(dockerAvailable, k8sConnected bool, diskSpaceMB int64) *ErrorBuilder {
	b.err.Context.SystemState = SystemState{
		DockerAvailable: dockerAvailable,
		K8sConnected:    k8sConnected,
		DiskSpaceMB:     diskSpaceMB,
		NetworkStatus:   "connected",
	}
	return b
}

// WithMetadata sets structured metadata
func (b *ErrorBuilder) WithMetadata(sessionID, toolName, operation string) *ErrorBuilder {
	b.err.Context.SetMetadata(sessionID, toolName, operation)
	return b
}

// Build returns the constructed RichError
func (b *ErrorBuilder) Build() *RichError {
	return b.err
}

// Common error builders for frequent patterns

// NewSessionError creates a session-related error
func NewSessionError(sessionID string, operation string) *ErrorBuilder {
	return NewErrorBuilder(
		ErrCodeSessionNotFound,
		"Session not found or invalid",
		ErrTypeSession,
	).WithField("session_id", sessionID).
		WithOperation(operation).
		WithComponent("session_manager").
		WithSeverity("high").
		WithCommand(1, "Verify session exists", "Check if the session ID is valid", "list_sessions", "Session should be listed in active sessions")
}

// NewBuildError creates a build-related error
func NewBuildError(message string, sessionID, imageName string) *ErrorBuilder {
	return NewErrorBuilder(
		ErrCodeBuildFailed,
		message,
		ErrTypeBuild,
	).WithField("session_id", sessionID).
		WithField("image_name", imageName).
		WithOperation("build_image").
		WithComponent("docker").
		WithSeverity("high")
}

// NewDeploymentError creates a deployment-related error
func NewDeploymentError(message string, sessionID, namespace, appName string) *ErrorBuilder {
	return NewErrorBuilder(
		ErrCodeDeployFailed,
		message,
		ErrTypeDeployment,
	).WithField("session_id", sessionID).
		WithField("namespace", namespace).
		WithField("app_name", appName).
		WithOperation("deploy_kubernetes").
		WithComponent("kubernetes").
		WithSeverity("high")
}

// NewAnalysisError creates an analysis-related error
func NewAnalysisError(message string, sessionID, repoPath string) *ErrorBuilder {
	return NewErrorBuilder(
		ErrCodeAnalysisFailed,
		message,
		ErrTypeAnalysis,
	).WithField("session_id", sessionID).
		WithField("repo_path", repoPath).
		WithOperation("analyze_repository").
		WithComponent("analyzer").
		WithSeverity("medium")
}

// NewValidationError creates a validation error
func NewValidationErrorBuilder(message string, field string, value interface{}) *ErrorBuilder {
	return NewErrorBuilder(
		"invalid_arguments",
		message,
		ErrTypeValidation,
	).WithField("field", field).
		WithField("value", value).
		WithOperation("validation").
		WithSeverity("low")
}
