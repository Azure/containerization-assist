package core

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
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
		logger:      logger.With().Str("service", "github.com/Azure/container-kit/pkg/mcp/domain/errors").Logger(),
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

func (s *ErrorService) HandleError(ctx context.Context, err error, context ConsolidatedErrorContext) error {
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

func (s *ErrorService) enrichError(err error, context ConsolidatedErrorContext) error {
	enrichedMsg := fmt.Sprintf("[%s:%s] %s", context.Tool, context.Operation, err.Error())
	if context.SessionID != "" {
		enrichedMsg = fmt.Sprintf("[session:%s] %s", context.SessionID, enrichedMsg)
	}
	return errors.NewError().Messagef("%s", enrichedMsg).WithLocation().Build()
}

func (s *ErrorService) GetMetrics() *ErrorMetrics {
	return s.metrics
}

func (s *ErrorService) CleanupAggregator(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.aggregators, sessionID)
}

type ConsolidatedErrorContext struct {
	Tool      string
	Operation string
	Stage     string
	SessionID string
	Fields    map[string]interface{}
}

type ErrorHandler interface {
	Handle(ctx context.Context, err error, context ConsolidatedErrorContext) error
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

func (a *ErrorAggregator) AddError(err error, context ConsolidatedErrorContext) {
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

		summary.BySeverity["unknown"]++
		summary.ByType["unknown"]++
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
	Context   ConsolidatedErrorContext
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

func (h *LoggingErrorHandler) Handle(ctx context.Context, err error, context ConsolidatedErrorContext) error {
	h.logger.Error().
		Err(err).
		Str("tool", context.Tool).
		Str("operation", context.Operation).
		Str("session", context.SessionID).
		Msg("Error occurred")

	return nil
}

type RetryableErrorHandler struct {
	logger  zerolog.Logger
	handler ErrorHandler
}

func NewRetryableErrorHandler(logger zerolog.Logger) *RetryableErrorHandler {
	return &RetryableErrorHandler{
		logger:  logger.With().Str("handler", "retryable").Logger(),
		handler: nil, // Simplified for now
	}
}

func (h *RetryableErrorHandler) Handle(ctx context.Context, err error, context ConsolidatedErrorContext) error {
	h.logger.Info().
		Err(err).
		Str("tool", context.Tool).
		Str("operation", context.Operation).
		Msg("Error detected")

	return nil
}

type CompositeErrorHandler struct {
	handlers []ErrorHandler
}

func NewCompositeErrorHandler(handlers ...ErrorHandler) *CompositeErrorHandler {
	return &CompositeErrorHandler{
		handlers: handlers,
	}
}

func (h *CompositeErrorHandler) Handle(ctx context.Context, err error, context ConsolidatedErrorContext) error {
	var errs []string

	for _, handler := range h.handlers {
		if handlerErr := handler.Handle(ctx, err, context); handlerErr != nil {
			errs = append(errs, handlerErr.Error())
		}
	}

	if len(errs) > 0 {
		return errors.NewError().Messagef("handler errors: %s", strings.Join(errs, "; ")).Build()
	}

	return nil
}
