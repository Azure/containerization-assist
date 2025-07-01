package core

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/errors"
	"github.com/Azure/container-kit/pkg/mcp/internal/runtime"
	"github.com/rs/zerolog"
)

type ErrorService struct {
	logger      zerolog.Logger
	aggregators map[string]*ErrorAggregator
	handlers    []ErrorHandler
	mu          sync.RWMutex
	metrics     *ErrorMetrics
}

func NewErrorService(logger zerolog.Logger) *ErrorService {
	return &ErrorService{
		logger:      logger.With().Str("service", "errors").Logger(),
		aggregators: make(map[string]*ErrorAggregator),
		handlers:    make([]ErrorHandler, 0),
		metrics:     NewErrorMetrics(),
	}
}

func (s *ErrorService) RegisterHandler(handler ErrorHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.handlers = append(s.handlers, handler)
	s.logger.Debug().Str("handler", fmt.Sprintf("%T", handler)).Msg("Error handler registered")
}

func (s *ErrorService) CreateAggregator(sessionID string) *ErrorAggregator {
	s.mu.Lock()
	defer s.mu.Unlock()

	aggregator := NewErrorAggregator(sessionID, s.logger)
	s.aggregators[sessionID] = aggregator
	return aggregator
}

func (s *ErrorService) GetAggregator(sessionID string) *ErrorAggregator {
	s.mu.RLock()
	aggregator, exists := s.aggregators[sessionID]
	s.mu.RUnlock()

	if !exists {
		return s.CreateAggregator(sessionID)
	}

	return aggregator
}

func (s *ErrorService) HandleError(ctx context.Context, err error, context ErrorContext) error {
	if err == nil {
		return nil
	}

	s.metrics.RecordError(context.Tool, context.Operation)

	enrichedErr := s.enrichError(err, context)

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

func (s *ErrorService) enrichError(err error, context ErrorContext) error {
	coreErr := errors.Wrap(err, context.Tool, fmt.Sprintf("operation: %s", context.Operation))

	coreErr = coreErr.WithSession(context.SessionID, context.Tool, context.Stage, "")

	for k, v := range context.Fields {
		coreErr = coreErr.WithContext(k, v)
	}

	return coreErr
}

func (s *ErrorService) GetMetrics() *ErrorMetrics {
	return s.metrics
}

func (s *ErrorService) CleanupAggregator(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.aggregators, sessionID)
}

type ErrorContext struct {
	Tool      string
	Operation string
	Stage     string
	SessionID string
	Fields    map[string]interface{}
}

type ErrorHandler interface {
	Handle(ctx context.Context, err error, context ErrorContext) error
}

type ErrorAggregator struct {
	sessionID string
	errors    []ErrorRecord
	mu        sync.RWMutex
	logger    zerolog.Logger
}

func NewErrorAggregator(sessionID string, logger zerolog.Logger) *ErrorAggregator {
	return &ErrorAggregator{
		sessionID: sessionID,
		errors:    make([]ErrorRecord, 0),
		logger:    logger.With().Str("session", sessionID).Logger(),
	}
}

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

func (a *ErrorAggregator) GetErrors() []ErrorRecord {
	a.mu.RLock()
	defer a.mu.RUnlock()

	errors := make([]ErrorRecord, len(a.errors))
	copy(errors, a.errors)
	return errors
}

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
		summary.ByTool[record.Context.Tool]++

		if toolErr, ok := record.Error.(*runtime.ToolError); ok {
			summary.BySeverity[string(toolErr.Severity)]++
			summary.ByType[string(toolErr.Type)]++
		} else {
			summary.BySeverity["unknown"]++
			summary.ByType["unknown"]++
		}
	}

	return summary
}

func (a *ErrorAggregator) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.errors = make([]ErrorRecord, 0)
}

type ErrorRecord struct {
	Error     error
	Context   ErrorContext
	Timestamp time.Time
}

type ErrorSummary struct {
	TotalErrors int
	ByTool      map[string]int
	BySeverity  map[string]int
	ByType      map[string]int
}

type ErrorMetrics struct {
	totalErrors  int64
	errorsByTool map[string]int64
	errorsByOp   map[string]int64
	mu           sync.RWMutex
}

func NewErrorMetrics() *ErrorMetrics {
	return &ErrorMetrics{
		errorsByTool: make(map[string]int64),
		errorsByOp:   make(map[string]int64),
	}
}

func (m *ErrorMetrics) RecordError(tool, operation string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalErrors++
	m.errorsByTool[tool]++
	m.errorsByOp[operation]++
}

func (m *ErrorMetrics) GetTotalErrors() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.totalErrors
}

func (m *ErrorMetrics) GetErrorsByTool() map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]int64)
	for k, v := range m.errorsByTool {
		result[k] = v
	}
	return result
}

func (m *ErrorMetrics) GetErrorsByOperation() map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]int64)
	for k, v := range m.errorsByOp {
		result[k] = v
	}
	return result
}

type LoggingErrorHandler struct {
	logger zerolog.Logger
}

func NewLoggingErrorHandler(logger zerolog.Logger) *LoggingErrorHandler {
	return &LoggingErrorHandler{
		logger: logger.With().Str("handler", "logging").Logger(),
	}
}

func (h *LoggingErrorHandler) Handle(ctx context.Context, err error, context ErrorContext) error {
	if toolErr, ok := err.(*runtime.ToolError); ok {
		event := h.logger.Error()

		switch toolErr.Severity {
		case runtime.SeverityCritical:
			event = h.logger.Error()
		case runtime.SeverityHigh:
			event = h.logger.Error()
		case runtime.SeverityMedium:
			event = h.logger.Warn()
		case runtime.SeverityLow:
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

type RetryableErrorHandler struct {
	logger  zerolog.Logger
	handler *runtime.ErrorHandler
}

func NewRetryableErrorHandler(logger zerolog.Logger) *RetryableErrorHandler {
	return &RetryableErrorHandler{
		logger:  logger.With().Str("handler", "retryable").Logger(),
		handler: runtime.NewErrorHandler(logger),
	}
}

func (h *RetryableErrorHandler) Handle(ctx context.Context, err error, context ErrorContext) error {
	if h.handler.IsRetryable(err) {
		h.logger.Info().
			Err(err).
			Str("tool", context.Tool).
			Str("operation", context.Operation).
			Msg("Error is retryable")

	}

	return nil
}

type ErrorReporter interface {
	ReportError(ctx context.Context, err error, context ErrorContext) error
}

type CompositeErrorHandler struct {
	handlers []ErrorHandler
}

func NewCompositeErrorHandler(handlers ...ErrorHandler) *CompositeErrorHandler {
	return &CompositeErrorHandler{
		handlers: handlers,
	}
}

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
