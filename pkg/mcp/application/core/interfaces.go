package core

import (
	"sync"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
)

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

// GetRegisteredToolNames returns all registered tool names
func GetRegisteredToolNames() []string {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	names := make([]string, 0, len(globalToolRegistry))
	for name := range globalToolRegistry {
		names = append(names, name)
	}
	return names
}
