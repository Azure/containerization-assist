// Package workflow provides core middleware types and utilities for step execution
package workflow

import (
	"context"
)

// StepHandler is a function that executes a workflow step
// This is the core abstraction that allows middleware to wrap step execution
type StepHandler func(ctx context.Context, step Step, state *WorkflowState) error

// StepMiddleware is a function that wraps a StepHandler to add functionality
// Middleware components follow the decorator pattern, allowing for composable behavior
type StepMiddleware func(next StepHandler) StepHandler

// Chain creates a single StepHandler from a list of middlewares
// Middlewares are applied in reverse order (last middleware wraps first)
// This allows for intuitive ordering where the first middleware in the list
// is the outermost wrapper.
//
// Example:
//
//	handler := Chain(RetryMiddleware(), TracingMiddleware())(baseHandler)
//	// Results in: RetryMiddleware(TracingMiddleware(baseHandler))
func Chain(middlewares ...StepMiddleware) StepMiddleware {
	return func(next StepHandler) StepHandler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}
