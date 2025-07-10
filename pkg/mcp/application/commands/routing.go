package commands

import (
	"context"
	"sync"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// Router implements CommandRouter interface
type Router struct {
	handlers map[string]api.CommandHandler
	mu       sync.RWMutex
}

// NewRouter creates a new command router
func NewRouter() api.CommandRouter {
	return &Router{
		handlers: make(map[string]api.CommandHandler),
	}
}

// Register registers a command handler
func (r *Router) Register(command string, handler api.CommandHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[command]; exists {
		return errors.NewError().
			Code(errors.CodeResourceAlreadyExists).
			Type(errors.ErrTypeValidation).
			Message("command already registered").
			Context("command", command).
			Build()
	}

	r.handlers[command] = handler
	return nil
}

// Route routes a command to its handler
func (r *Router) Route(ctx context.Context, command string, args interface{}) (interface{}, error) {
	r.mu.RLock()
	handler, exists := r.handlers[command]
	r.mu.RUnlock()

	if !exists {
		return nil, errors.NewError().
			Code(errors.CodeResourceNotFound).
			Type(errors.ErrTypeValidation).
			Message("command not found").
			Context("command", command).
			Build()
	}

	return handler.Execute(ctx, args)
}

// ListCommands returns all registered commands
func (r *Router) ListCommands() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	commands := make([]string, 0, len(r.handlers))
	for command := range r.handlers {
		commands = append(commands, command)
	}
	return commands
}

// GetHandler retrieves a handler without executing it
func (r *Router) GetHandler(command string) (api.CommandHandler, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, exists := r.handlers[command]
	if !exists {
		return nil, errors.NewError().
			Code(errors.CodeResourceNotFound).
			Type(errors.ErrTypeValidation).
			Message("command handler not found").
			Context("command", command).
			Build()
	}

	return handler, nil
}

// Unregister removes a command handler
func (r *Router) Unregister(command string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[command]; !exists {
		return errors.NewError().
			Code(errors.CodeResourceNotFound).
			Type(errors.ErrTypeValidation).
			Message("command handler not found").
			Context("command", command).
			Build()
	}

	delete(r.handlers, command)
	return nil
}

// RegisterFunc registers a function as a command handler
func (r *Router) RegisterFunc(command string, handler func(ctx context.Context, args interface{}) (interface{}, error)) error {
	return r.Register(command, HandlerFunc(handler))
}

// HandlerFunc is a function adapter that implements CommandHandler
type HandlerFunc func(ctx context.Context, args interface{}) (interface{}, error)

// Execute implements the CommandHandler interface
func (f HandlerFunc) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	return f(ctx, args)
}
