// Package errors provides unified error handling for orchestration
package errors

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// ErrorHandler consolidates all error handling logic for orchestration
type ErrorHandler struct {
	classifiers []ErrorClassifier
	routers     []ErrorRouter
	recovery    RecoveryStrategy
}

// ErrorClassifier classifies errors by type
type ErrorClassifier interface {
	Classify(err error) ErrorType
}

// ErrorRouter routes errors to appropriate handlers
type ErrorRouter interface {
	Route(ctx context.Context, err error, errType ErrorType) error
}

// RecoveryStrategy defines how to recover from errors
type RecoveryStrategy interface {
	Recover(ctx context.Context, err error) error
}

// ErrorType represents the classification of an error
type ErrorType string

const (
	ErrorTypeTransient      ErrorType = "transient"
	ErrorTypePermanent      ErrorType = "permanent"
	ErrorTypeConfiguration  ErrorType = "configuration"
	ErrorTypeValidation     ErrorType = "validation"
	ErrorTypeTimeout        ErrorType = "timeout"
	ErrorTypeCircuitBreaker ErrorType = "circuit_breaker"
)

// NewErrorHandler creates a new consolidated error handler
func NewErrorHandler() *ErrorHandler {
	return &ErrorHandler{
		classifiers: make([]ErrorClassifier, 0),
		routers:     make([]ErrorRouter, 0),
	}
}

// HandleError processes an error through classification, routing, and recovery
func (h *ErrorHandler) HandleError(ctx context.Context, err error) error {
	// Classify the error
	errType := h.classifyError(err)

	// Route to appropriate handler
	if routeErr := h.routeError(ctx, err, errType); routeErr != nil {
		return errors.NewError().Message("error routing failed").Cause(routeErr).WithLocation().Build()
	}

	if h.recovery != nil && isRecoverable(errType) {
		return h.recovery.Recover(ctx, err)
	}

	return err
}

func (h *ErrorHandler) classifyError(err error) ErrorType {
	for _, classifier := range h.classifiers {
		if errType := classifier.Classify(err); errType != "" {
			return errType
		}
	}
	return ErrorTypePermanent // Default to permanent error
}

func (h *ErrorHandler) routeError(ctx context.Context, err error, errType ErrorType) error {
	for _, router := range h.routers {
		if routeErr := router.Route(ctx, err, errType); routeErr != nil {
			return routeErr
		}
	}
	return nil
}

func isRecoverable(errType ErrorType) bool {
	switch errType {
	case ErrorTypeTransient, ErrorTypeTimeout:
		return true
	default:
		return false
	}
}

// OrchestrationError represents an error during orchestration
type OrchestrationError struct {
	Type      ErrorType
	Message   string
	Cause     error
	Timestamp time.Time
	Context   map[string]interface{}
}

func (e *OrchestrationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// Unwrap returns the underlying error
func (e *OrchestrationError) Unwrap() error {
	return e.Cause
}
