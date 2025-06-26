package core

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// Tool interface for common tool operations
type Tool interface {
	Execute(ctx context.Context, args interface{}) (interface{}, error)
}

// ToolWithMetadata interface for tools that provide metadata
type ToolWithMetadata interface {
	Tool
	GetMetadata() (*mcptypes.ToolMetadata, error)
}

// ToolWithValidation interface for tools that provide validation
type ToolWithValidation interface {
	Tool
	Validate(args interface{}) error
}

// getToolName safely extracts tool name from interface{} tool
func getToolName(tool interface{}) string {
	if t, ok := tool.(ToolWithMetadata); ok {
		if metadata, err := t.GetMetadata(); err == nil && metadata != nil {
			return metadata.Name
		}
	}
	return "unknown"
}

// getToolMetadata safely extracts tool metadata from interface{} tool
func getToolMetadata(tool interface{}) *mcptypes.ToolMetadata {
	if t, ok := tool.(ToolWithMetadata); ok {
		if metadata, err := t.GetMetadata(); err == nil {
			return metadata
		}
	}
	return &mcptypes.ToolMetadata{Name: "unknown"}
}

// ToolMiddleware provides middleware functionality for atomic tools
type ToolMiddleware struct {
	validationService *build.ValidationService
	errorService      *ErrorService
	telemetryService  *TelemetryService
	logger            zerolog.Logger
	middlewares       []Middleware
}

// NewToolMiddleware creates a new tool middleware
func NewToolMiddleware(
	validationService *build.ValidationService,
	errorService *ErrorService,
	telemetryService *TelemetryService,
	logger zerolog.Logger,
) *ToolMiddleware {
	return &ToolMiddleware{
		validationService: validationService,
		errorService:      errorService,
		telemetryService:  telemetryService,
		logger:            logger.With().Str("service", "middleware").Logger(),
		middlewares:       make([]Middleware, 0),
	}
}

// Use adds a middleware to the chain
func (m *ToolMiddleware) Use(middleware Middleware) {
	m.middlewares = append(m.middlewares, middleware)
}

// ExecuteWithMiddleware executes a tool with all middleware applied
func (m *ToolMiddleware) ExecuteWithMiddleware(ctx context.Context, tool interface{}, args interface{}) (interface{}, error) {
	// Create execution context
	execCtx := &ExecutionContext{
		Context:   ctx,
		Tool:      tool,
		Args:      args,
		StartTime: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	// Build middleware chain
	handler := m.buildChain(execCtx)

	// Execute through middleware chain
	result, err := handler(execCtx)

	// Record execution
	m.recordExecution(execCtx, result, err)

	return result, err
}

// buildChain builds the middleware execution chain
func (m *ToolMiddleware) buildChain(execCtx *ExecutionContext) HandlerFunc {
	// Start with the actual tool execution
	handler := func(ctx *ExecutionContext) (interface{}, error) {
		if tool, ok := ctx.Tool.(Tool); ok {
			return tool.Execute(ctx.Context, ctx.Args)
		}
		return nil, fmt.Errorf("tool does not implement Tool interface")
	}

	// Wrap with middleware in reverse order
	for i := len(m.middlewares) - 1; i >= 0; i-- {
		middleware := m.middlewares[i]
		handler = middleware.Wrap(handler)
	}

	return handler
}

// recordExecution records the tool execution
func (m *ToolMiddleware) recordExecution(execCtx *ExecutionContext, result interface{}, err error) {
	duration := time.Since(execCtx.StartTime)

	execution := ToolExecution{
		Tool:      getToolName(execCtx.Tool),
		Operation: "execute",
		StartTime: execCtx.StartTime,
		EndTime:   time.Now(),
		Duration:  duration,
		Success:   err == nil,
		Metadata:  execCtx.Metadata,
	}

	// Extract session ID if available
	if sessionID, ok := execCtx.Metadata["session_id"].(string); ok {
		execution.SessionID = sessionID
	}

	m.telemetryService.TrackToolExecution(execCtx.Context, execution)
}

// ExecutionContext provides context for tool execution
type ExecutionContext struct {
	Context   context.Context
	Tool      interface{}
	Args      interface{}
	StartTime time.Time
	Metadata  map[string]interface{}
}

// HandlerFunc represents a tool execution handler
type HandlerFunc func(*ExecutionContext) (interface{}, error)

// Middleware defines the interface for middleware
type Middleware interface {
	Wrap(next HandlerFunc) HandlerFunc
}

// ValidationMiddleware provides automatic validation
type ValidationMiddleware struct {
	service *build.ValidationService
	logger  zerolog.Logger
}

// NewValidationMiddleware creates a new validation middleware
func NewValidationMiddleware(service *build.ValidationService, logger zerolog.Logger) *ValidationMiddleware {
	return &ValidationMiddleware{
		service: service,
		logger:  logger.With().Str("middleware", "validation").Logger(),
	}
}

// Wrap wraps the handler with validation
func (m *ValidationMiddleware) Wrap(next HandlerFunc) HandlerFunc {
	return func(ctx *ExecutionContext) (interface{}, error) {
		// Validate arguments using the tool's validation if available
		if tool, ok := ctx.Tool.(ToolWithValidation); ok {
			if err := tool.Validate(ctx.Args); err != nil {
				m.logger.Error().Err(err).Str("tool", getToolName(ctx.Tool)).Msg("Validation failed")
				return nil, err
			}
			m.logger.Debug().Str("tool", getToolName(ctx.Tool)).Msg("Validation passed")
		} else {
			m.logger.Debug().Str("tool", getToolName(ctx.Tool)).Msg("Tool does not implement validation")
		}

		// Continue to next middleware
		return next(ctx)
	}
}

// LoggingMiddleware provides automatic logging
type LoggingMiddleware struct {
	logger zerolog.Logger
}

// NewLoggingMiddleware creates a new logging middleware
func NewLoggingMiddleware(logger zerolog.Logger) *LoggingMiddleware {
	return &LoggingMiddleware{
		logger: logger.With().Str("middleware", "logging").Logger(),
	}
}

// Wrap wraps the handler with logging
func (m *LoggingMiddleware) Wrap(next HandlerFunc) HandlerFunc {
	return func(ctx *ExecutionContext) (interface{}, error) {
		m.logger.Info().
			Str("tool", getToolName(ctx.Tool)).
			Msg("Tool execution started")

		result, err := next(ctx)

		if err != nil {
			m.logger.Error().
				Err(err).
				Str("tool", getToolName(ctx.Tool)).
				Dur("duration", time.Since(ctx.StartTime)).
				Msg("Tool execution failed")
		} else {
			m.logger.Info().
				Str("tool", getToolName(ctx.Tool)).
				Dur("duration", time.Since(ctx.StartTime)).
				Msg("Tool execution completed successfully")
		}

		return result, err
	}
}

// ErrorHandlingMiddleware provides automatic error handling
type ErrorHandlingMiddleware struct {
	service *ErrorService
	logger  zerolog.Logger
}

// NewErrorHandlingMiddleware creates a new error handling middleware
func NewErrorHandlingMiddleware(service *ErrorService, logger zerolog.Logger) *ErrorHandlingMiddleware {
	return &ErrorHandlingMiddleware{
		service: service,
		logger:  logger.With().Str("middleware", "error_handling").Logger(),
	}
}

// Wrap wraps the handler with error handling
func (m *ErrorHandlingMiddleware) Wrap(next HandlerFunc) HandlerFunc {
	return func(ctx *ExecutionContext) (interface{}, error) {
		result, err := next(ctx)

		if err != nil {
			// Create error context
			errorCtx := ErrorContext{
				Tool:      getToolName(ctx.Tool),
				Operation: "execute",
				Fields:    make(map[string]interface{}),
			}

			// Add session ID if available
			if sessionID, ok := ctx.Metadata["session_id"].(string); ok {
				errorCtx.SessionID = sessionID
			}

			// Handle the error through the error service
			handledErr := m.service.HandleError(ctx.Context, err, errorCtx)
			return result, handledErr
		}

		return result, nil
	}
}

// MetricsMiddleware provides automatic metrics collection
type MetricsMiddleware struct {
	service *TelemetryService
	logger  zerolog.Logger
}

// NewMetricsMiddleware creates a new metrics middleware
func NewMetricsMiddleware(service *TelemetryService, logger zerolog.Logger) *MetricsMiddleware {
	return &MetricsMiddleware{
		service: service,
		logger:  logger.With().Str("middleware", "metrics").Logger(),
	}
}

// Wrap wraps the handler with metrics collection
func (m *MetricsMiddleware) Wrap(next HandlerFunc) HandlerFunc {
	return func(ctx *ExecutionContext) (interface{}, error) {
		// Create performance tracker
		tracker := m.service.CreatePerformanceTracker(getToolName(ctx.Tool), "execute")
		tracker.Start()

		result, err := next(ctx)

		// Record duration
		duration := tracker.Finish()

		// Record additional metrics
		if err == nil {
			tracker.Record("success", 1, "count")
		} else {
			tracker.Record("failure", 1, "count")
		}

		m.logger.Debug().
			Str("tool", getToolName(ctx.Tool)).
			Dur("duration", duration).
			Bool("success", err == nil).
			Msg("Metrics recorded")

		return result, err
	}
}

// RecoveryMiddleware provides panic recovery
type RecoveryMiddleware struct {
	logger zerolog.Logger
}

// NewRecoveryMiddleware creates a new recovery middleware
func NewRecoveryMiddleware(logger zerolog.Logger) *RecoveryMiddleware {
	return &RecoveryMiddleware{
		logger: logger.With().Str("middleware", "recovery").Logger(),
	}
}

// Wrap wraps the handler with panic recovery
func (m *RecoveryMiddleware) Wrap(next HandlerFunc) HandlerFunc {
	return func(ctx *ExecutionContext) (result interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				m.logger.Error().
					Interface("panic", r).
					Str("tool", getToolName(ctx.Tool)).
					Msg("Panic recovered during tool execution")

				err = fmt.Errorf("tool execution panicked: %v", r)
			}
		}()

		return next(ctx)
	}
}

// ContextMiddleware adds common context information
type ContextMiddleware struct {
	logger zerolog.Logger
}

// NewContextMiddleware creates a new context middleware
func NewContextMiddleware(logger zerolog.Logger) *ContextMiddleware {
	return &ContextMiddleware{
		logger: logger.With().Str("middleware", "context").Logger(),
	}
}

// Wrap wraps the handler with context enrichment
func (m *ContextMiddleware) Wrap(next HandlerFunc) HandlerFunc {
	return func(ctx *ExecutionContext) (interface{}, error) {
		// Extract common metadata from args
		m.extractMetadata(ctx)

		return next(ctx)
	}
}

// extractMetadata extracts common metadata from arguments
func (m *ContextMiddleware) extractMetadata(ctx *ExecutionContext) {
	// Try to extract session ID using reflection or type assertion
	if baseArgs, ok := ctx.Args.(interface{ GetSessionID() string }); ok {
		ctx.Metadata["session_id"] = baseArgs.GetSessionID()
	}

	// Try to extract dry run flag
	if dryRunArgs, ok := ctx.Args.(interface{ IsDryRun() bool }); ok {
		ctx.Metadata["dry_run"] = dryRunArgs.IsDryRun()
	}

	// Add tool metadata
	metadata := getToolMetadata(ctx.Tool)
	ctx.Metadata["tool_name"] = metadata.Name
	ctx.Metadata["tool_version"] = metadata.Version
}

// TimeoutMiddleware provides execution timeout
type TimeoutMiddleware struct {
	timeout time.Duration
	logger  zerolog.Logger
}

// NewTimeoutMiddleware creates a new timeout middleware
func NewTimeoutMiddleware(timeout time.Duration, logger zerolog.Logger) *TimeoutMiddleware {
	return &TimeoutMiddleware{
		timeout: timeout,
		logger:  logger.With().Str("middleware", "timeout").Logger(),
	}
}

// Wrap wraps the handler with timeout
func (m *TimeoutMiddleware) Wrap(next HandlerFunc) HandlerFunc {
	return func(ctx *ExecutionContext) (interface{}, error) {
		// Create timeout context
		timeoutCtx, cancel := context.WithTimeout(ctx.Context, m.timeout)
		defer cancel()

		// Update execution context
		originalCtx := ctx.Context
		ctx.Context = timeoutCtx

		// Use a channel to get the result
		resultChan := make(chan struct {
			result interface{}
			err    error
		}, 1)

		go func() {
			result, err := next(ctx)
			resultChan <- struct {
				result interface{}
				err    error
			}{result, err}
		}()

		select {
		case res := <-resultChan:
			return res.result, res.err
		case <-timeoutCtx.Done():
			// Restore original context
			ctx.Context = originalCtx

			m.logger.Error().
				Str("tool", getToolName(ctx.Tool)).
				Dur("timeout", m.timeout).
				Msg("Tool execution timed out")

			return nil, fmt.Errorf("tool execution timed out after %v", m.timeout)
		}
	}
}

// StandardMiddlewareChain creates a standard middleware chain
func StandardMiddlewareChain(
	validationService *build.ValidationService,
	errorService *ErrorService,
	telemetryService *TelemetryService,
	logger zerolog.Logger,
) *ToolMiddleware {
	middleware := NewToolMiddleware(validationService, errorService, telemetryService, logger)

	// Add standard middleware in order
	middleware.Use(NewRecoveryMiddleware(logger))
	middleware.Use(NewContextMiddleware(logger))
	middleware.Use(NewTimeoutMiddleware(5*time.Minute, logger))
	middleware.Use(NewLoggingMiddleware(logger))
	middleware.Use(NewValidationMiddleware(validationService, logger))
	middleware.Use(NewErrorHandlingMiddleware(errorService, logger))
	middleware.Use(NewMetricsMiddleware(telemetryService, logger))

	return middleware
}
