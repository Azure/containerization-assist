package errors

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/rs/zerolog"
)

// ErrorReporterImpl implements ErrorReporter interface using BETA's unified error system
type ErrorReporterImpl struct {
	logger zerolog.Logger
}

// NewErrorReporter creates a new error reporter
func NewErrorReporter(logger zerolog.Logger) *ErrorReporterImpl {
	return &ErrorReporterImpl{
		logger: logger,
	}
}

// Report implements ErrorReporter.Report
func (r *ErrorReporterImpl) Report(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	event := r.logger.Error()

	if richErr, ok := err.(*errors.RichError); ok {
		event = event.
			Str("error_code", string(richErr.Code)).
			Str("error_type", string(richErr.Type)).
			Str("error_severity", string(richErr.Severity)).
			Time("error_timestamp", richErr.Timestamp)

		if richErr.Context != nil {
			for key, value := range richErr.Context {
				event = event.Interface(fmt.Sprintf("ctx_%s", key), value)
			}
		}

		if richErr.Location != nil {
			event = event.
				Str("source_file", richErr.Location.File).
				Int("source_line", richErr.Location.Line).
				Str("source_function", richErr.Location.Function)
		}

		if len(richErr.Suggestions) > 0 {
			event = event.Strs("suggestions", richErr.Suggestions)
		}

		event.Err(err).Msg(richErr.Message)
	} else {
		event.Err(err).Msg("Unstructured error reported")
	}

	return nil
}

// Wrap implements ErrorReporter.Wrap
func (r *ErrorReporterImpl) Wrap(err error, message string) error {
	if err == nil {
		return errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeInternal).
			Message(message).Build()
	}

	if richErr, ok := err.(*errors.RichError); ok {
		return errors.NewError().
			Code(richErr.Code).
			Type(richErr.Type).
			Severity(richErr.Severity).
			Message(message).
			Cause(err).Build()
	}

	return errors.NewError().
		Code(errors.CodeInternalError).
		Type(errors.ErrTypeInternal).
		Message(message).
		Cause(err).Build()
}

// New implements ErrorReporter.New
func (r *ErrorReporterImpl) New(message string) error {
	return errors.NewError().
		Code(errors.CodeInternalError).
		Type(errors.ErrTypeInternal).
		Message(message).Build()
}

// BuildSessionError creates a session-specific error
func (r *ErrorReporterImpl) BuildSessionError(code errors.ErrorCode, message string, sessionID string, cause error) error {
	builder := errors.NewError().
		Code(code).
		Type(errors.ErrTypeSession).
		Message(message).
		Context("session_id", sessionID)

	if cause != nil {
		builder = builder.Cause(cause)
	}

	return builder.Build()
}

// BuildBuildError creates a build-specific error
func (r *ErrorReporterImpl) BuildBuildError(code errors.ErrorCode, message string, buildID string, cause error) error {
	builder := errors.NewError().
		Code(code).
		Type(errors.ErrTypeContainer).
		Message(message).
		Context("build_id", buildID)

	if cause != nil {
		builder = builder.Cause(cause)
	}

	return builder.Build()
}

// BuildToolError creates a tool-specific error
func (r *ErrorReporterImpl) BuildToolError(code errors.ErrorCode, message string, toolName string, cause error) error {
	builder := errors.NewError().
		Code(code).
		Type(errors.ErrTypeTool).
		Message(message).
		Context("tool_name", toolName)

	if cause != nil {
		builder = builder.Cause(cause)
	}

	return builder.Build()
}

// BuildWorkflowError creates a workflow-specific error
func (r *ErrorReporterImpl) BuildWorkflowError(code errors.ErrorCode, message string, workflowID string, cause error) error {
	builder := errors.NewError().
		Code(code).
		Type(errors.ErrTypeBusiness).
		Message(message).
		Context("workflow_id", workflowID)

	if cause != nil {
		builder = builder.Cause(cause)
	}

	return builder.Build()
}

// BuildValidationError creates a validation-specific error
func (r *ErrorReporterImpl) BuildValidationError(code errors.ErrorCode, message string, field string, value interface{}, cause error) error {
	builder := errors.NewError().
		Code(code).
		Type(errors.ErrTypeValidation).
		Message(message).
		Context("field", field).
		Context("value", value)

	if cause != nil {
		builder = builder.Cause(cause)
	}

	return builder.Build()
}

// ReportWithContext reports an error with additional context
func (r *ErrorReporterImpl) ReportWithContext(ctx context.Context, err error, contextFields map[string]interface{}) error {
	if err == nil {
		return nil
	}

	if richErr, ok := err.(*errors.RichError); ok {
		for key, value := range contextFields {
			if richErr.Context == nil {
				richErr.Context = make(errors.ConsolidatedErrorContext)
			}
			richErr.Context[key] = value
		}
	}

	return r.Report(ctx, err)
}

// GetErrorStatistics returns error reporting statistics
func (r *ErrorReporterImpl) GetErrorStatistics() map[string]interface{} {
	return map[string]interface{}{
		"reporter_type": "unified_error_reporter",
		"features": []string{
			"structured_logging",
			"rich_error_support",
			"context_preservation",
			"source_location_tracking",
		},
		"timestamp": time.Now(),
	}
}
