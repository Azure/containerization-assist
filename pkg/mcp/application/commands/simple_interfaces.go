package commands

import (
	"context"
	"sync"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
)

// Tool registry functions
var (
	globalToolRegistry = make(map[string]func() api.Tool)
	registryMutex      sync.RWMutex
)

// RegisterTool registers a tool factory function with the global registry
func RegisterTool(name string, factory func() api.Tool) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	globalToolRegistry[name] = factory
}

// GetRegisteredTools returns all registered tools
func GetRegisteredTools() map[string]func() api.Tool {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	result := make(map[string]func() api.Tool)
	for name, factory := range globalToolRegistry {
		result[name] = factory
	}
	return result
}

// Command represents a command in the application layer
type Command interface {
	Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error)
}

// CommandRegistry manages command registration and execution
type CommandRegistry struct {
	commands map[string]Command
	mutex    sync.RWMutex
}

// NewCommandRegistry creates a new command registry
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands: make(map[string]Command),
	}
}

// RegisterCommand registers a command with the registry
func (r *CommandRegistry) RegisterCommand(name string, command Command) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.commands[name] = command
}

// GetCommand retrieves a command by name
func (r *CommandRegistry) GetCommand(name string) (Command, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	command, exists := r.commands[name]
	return command, exists
}

// ListCommands returns all registered command names
func (r *CommandRegistry) ListCommands() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	names := make([]string, 0, len(r.commands))
	for name := range r.commands {
		names = append(names, name)
	}
	return names
}