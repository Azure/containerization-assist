package errors

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/tools/base"
	"github.com/rs/zerolog"
)

// ErrorService provides centralized error handling and reporting
type ErrorService struct {
	logger      zerolog.Logger
	aggregators map[string]*ErrorAggregator
	handlers    []ErrorHandler
	mu          sync.RWMutex
	metrics     *ErrorMetrics
}

// NewErrorService creates a new error service
func NewErrorService(logger zerolog.Logger) *ErrorService {
	return &ErrorService{
		logger:      logger.With().Str("service", "errors").Logger(),
		aggregators: make(map[string]*ErrorAggregator),
		handlers:    make([]ErrorHandler, 0),
		metrics:     NewErrorMetrics(),
	}
}

// RegisterHandler registers an error handler
func (s *ErrorService) RegisterHandler(handler ErrorHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.handlers = append(s.handlers, handler)
	s.logger.Debug().Str("handler", fmt.Sprintf("%T", handler)).Msg("Error handler registered")
}

// CreateAggregator creates a new error aggregator for a session
func (s *ErrorService) CreateAggregator(sessionID string) *ErrorAggregator {
	s.mu.Lock()
	defer s.mu.Unlock()

	aggregator := NewErrorAggregator(sessionID, s.logger)
	s.aggregators[sessionID] = aggregator
	return aggregator
}

// GetAggregator gets an existing aggregator or creates a new one
func (s *ErrorService) GetAggregator(sessionID string) *ErrorAggregator {
	s.mu.RLock()
	aggregator, exists := s.aggregators[sessionID]
	s.mu.RUnlock()

	if !exists {
		return s.CreateAggregator(sessionID)
	}

	return aggregator
}

// HandleError handles an error through all registered handlers
func (s *ErrorService) HandleError(ctx context.Context, err error, context ErrorContext) error {
	if err == nil {
		return nil
	}

	// Update metrics
	s.metrics.RecordError(context.Tool, context.Operation)

	// Enrich error with context
	enrichedErr := s.enrichError(err, context)

	// Process through handlers
	s.mu.RLock()
	handlers := make([]ErrorHandler, len(s.handlers))
	copy(handlers, s.handlers)
	s.mu.RUnlock()

	for _, handler := range handlers {
		if handlerErr := handler.Handle(ctx, enrichedErr, context); handlerErr != nil {
			s.logger.Error().Err(handlerErr).Msg("Error handler failed")
		}
	}

	return enrichedErr
}

// enrichError enriches an error with additional context
func (s *ErrorService) enrichError(err error, context ErrorContext) error {
	// If it's already a ToolError, add context
	if toolErr, ok := err.(*base.ToolError); ok {
		toolErr.Context.Tool = context.Tool
		toolErr.Context.Operation = context.Operation
		toolErr.Context.Stage = context.Stage
		toolErr.Context.SessionID = context.SessionID

		// Merge fields
		for k, v := range context.Fields {
			toolErr.WithContext(k, v)
		}

		return toolErr
	}

	// Wrap as ToolError
	return base.NewErrorBuilder("WRAPPED_ERROR", err.Error()).
		WithCause(err).
		WithTool(context.Tool).
		WithOperation(context.Operation).
		WithStage(context.Stage).
		WithSessionID(context.SessionID).
		Build()
}

// GetMetrics returns error metrics
func (s *ErrorService) GetMetrics() *ErrorMetrics {
	return s.metrics
}

// CleanupAggregator removes an aggregator for a session
func (s *ErrorService) CleanupAggregator(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.aggregators, sessionID)
}

// ErrorContext provides context for error handling
type ErrorContext struct {
	Tool      string
	Operation string
	Stage     string
	SessionID string
	Fields    map[string]interface{}
}

// ErrorHandler defines the interface for error handlers
type ErrorHandler interface {
	Handle(ctx context.Context, err error, context ErrorContext) error
}

// ErrorAggregator collects and aggregates errors for a session
type ErrorAggregator struct {
	sessionID string
	errors    []ErrorRecord
	mu        sync.RWMutex
	logger    zerolog.Logger
}

// NewErrorAggregator creates a new error aggregator
func NewErrorAggregator(sessionID string, logger zerolog.Logger) *ErrorAggregator {
	return &ErrorAggregator{
		sessionID: sessionID,
		errors:    make([]ErrorRecord, 0),
		logger:    logger.With().Str("session", sessionID).Logger(),
	}
}

// AddError adds an error to the aggregator
func (a *ErrorAggregator) AddError(err error, context ErrorContext) {
	a.mu.Lock()
	defer a.mu.Unlock()

	record := ErrorRecord{
		Error:     err,
		Context:   context,
		Timestamp: time.Now(),
	}

	a.errors = append(a.errors, record)

	a.logger.Debug().
		Err(err).
		Str("tool", context.Tool).
		Str("operation", context.Operation).
		Msg("Error added to aggregator")
}

// GetErrors returns all errors in the aggregator
func (a *ErrorAggregator) GetErrors() []ErrorRecord {
	a.mu.RLock()
	defer a.mu.RUnlock()

	errors := make([]ErrorRecord, len(a.errors))
	copy(errors, a.errors)
	return errors
}

// GetErrorsByTool returns errors for a specific tool
func (a *ErrorAggregator) GetErrorsByTool(tool string) []ErrorRecord {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var toolErrors []ErrorRecord
	for _, record := range a.errors {
		if record.Context.Tool == tool {
			toolErrors = append(toolErrors, record)
		}
	}

	return toolErrors
}

// GetSummary returns a summary of errors
func (a *ErrorAggregator) GetSummary() ErrorSummary {
	a.mu.RLock()
	defer a.mu.RUnlock()

	summary := ErrorSummary{
		TotalErrors: len(a.errors),
		ByTool:      make(map[string]int),
		BySeverity:  make(map[string]int),
		ByType:      make(map[string]int),
	}

	for _, record := range a.errors {
		// Count by tool
		summary.ByTool[record.Context.Tool]++

		// Count by severity and type if it's a ToolError
		if toolErr, ok := record.Error.(*base.ToolError); ok {
			summary.BySeverity[string(toolErr.Severity)]++
			summary.ByType[string(toolErr.Type)]++
		} else {
			summary.BySeverity["unknown"]++
			summary.ByType["unknown"]++
		}
	}

	return summary
}

// Clear clears all errors from the aggregator
func (a *ErrorAggregator) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.errors = make([]ErrorRecord, 0)
}

// ErrorRecord represents a recorded error with context
type ErrorRecord struct {
	Error     error
	Context   ErrorContext
	Timestamp time.Time
}

// ErrorSummary provides a summary of errors
type ErrorSummary struct {
	TotalErrors int
	ByTool      map[string]int
	BySeverity  map[string]int
	ByType      map[string]int
}

// ErrorMetrics tracks error metrics across the system
type ErrorMetrics struct {
	totalErrors  int64
	errorsByTool map[string]int64
	errorsByOp   map[string]int64
	mu           sync.RWMutex
}

// NewErrorMetrics creates new error metrics
func NewErrorMetrics() *ErrorMetrics {
	return &ErrorMetrics{
		errorsByTool: make(map[string]int64),
		errorsByOp:   make(map[string]int64),
	}
}

// RecordError records an error occurrence
func (m *ErrorMetrics) RecordError(tool, operation string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalErrors++
	m.errorsByTool[tool]++
	m.errorsByOp[operation]++
}

// GetTotalErrors returns the total number of errors
func (m *ErrorMetrics) GetTotalErrors() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.totalErrors
}

// GetErrorsByTool returns error counts by tool
func (m *ErrorMetrics) GetErrorsByTool() map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]int64)
	for k, v := range m.errorsByTool {
		result[k] = v
	}
	return result
}

// GetErrorsByOperation returns error counts by operation
func (m *ErrorMetrics) GetErrorsByOperation() map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]int64)
	for k, v := range m.errorsByOp {
		result[k] = v
	}
	return result
}

// Standard Error Handlers

// LoggingErrorHandler logs errors to the configured logger
type LoggingErrorHandler struct {
	logger zerolog.Logger
}

// NewLoggingErrorHandler creates a new logging error handler
func NewLoggingErrorHandler(logger zerolog.Logger) *LoggingErrorHandler {
	return &LoggingErrorHandler{
		logger: logger.With().Str("handler", "logging").Logger(),
	}
}

// Handle logs the error
func (h *LoggingErrorHandler) Handle(ctx context.Context, err error, context ErrorContext) error {
	if toolErr, ok := err.(*base.ToolError); ok {
		// Log with appropriate level based on severity
		event := h.logger.Error()

		switch toolErr.Severity {
		case base.SeverityCritical:
			event = h.logger.Error()
		case base.SeverityHigh:
			event = h.logger.Error()
		case base.SeverityMedium:
			event = h.logger.Warn()
		case base.SeverityLow:
			event = h.logger.Info()
		}

		event.
			Err(err).
			Str("code", toolErr.Code).
			Str("type", string(toolErr.Type)).
			Str("severity", string(toolErr.Severity)).
			Str("tool", context.Tool).
			Str("operation", context.Operation).
			Str("session", context.SessionID).
			Msg("Tool error occurred")
	} else {
		h.logger.Error().
			Err(err).
			Str("tool", context.Tool).
			Str("operation", context.Operation).
			Str("session", context.SessionID).
			Msg("Unhandled error occurred")
	}

	return nil
}

// RetryableErrorHandler determines if errors are retryable
type RetryableErrorHandler struct {
	logger  zerolog.Logger
	handler *base.ErrorHandler
}

// NewRetryableErrorHandler creates a new retryable error handler
func NewRetryableErrorHandler(logger zerolog.Logger) *RetryableErrorHandler {
	return &RetryableErrorHandler{
		logger:  logger.With().Str("handler", "retryable").Logger(),
		handler: base.NewErrorHandler(logger),
	}
}

// Handle determines if the error is retryable
func (h *RetryableErrorHandler) Handle(ctx context.Context, err error, context ErrorContext) error {
	if h.handler.IsRetryable(err) {
		h.logger.Info().
			Err(err).
			Str("tool", context.Tool).
			Str("operation", context.Operation).
			Msg("Error is retryable")

		// Could trigger retry logic here
	}

	return nil
}

// ErrorReporter interface for reporting errors to external systems
type ErrorReporter interface {
	ReportError(ctx context.Context, err error, context ErrorContext) error
}

// CompositeErrorHandler combines multiple error handlers
type CompositeErrorHandler struct {
	handlers []ErrorHandler
}

// NewCompositeErrorHandler creates a new composite error handler
func NewCompositeErrorHandler(handlers ...ErrorHandler) *CompositeErrorHandler {
	return &CompositeErrorHandler{
		handlers: handlers,
	}
}

// Handle runs the error through all handlers
func (h *CompositeErrorHandler) Handle(ctx context.Context, err error, context ErrorContext) error {
	var errs []string

	for _, handler := range h.handlers {
		if handlerErr := handler.Handle(ctx, err, context); handlerErr != nil {
			errs = append(errs, handlerErr.Error())
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("handler errors: %s", strings.Join(errs, "; "))
	}

	return nil
}
