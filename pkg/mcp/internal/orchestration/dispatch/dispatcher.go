package dispatch

import (
	"fmt"
	"sync"
)

// ToolDispatcher handles type-safe tool dispatch without reflection
type ToolDispatcher struct {
	tools      map[string]ToolFactory
	converters map[string]ArgConverter
	metadata   map[string]ToolMetadata
	mu         sync.RWMutex
}

// NewToolDispatcher creates a new tool dispatcher
func NewToolDispatcher() *ToolDispatcher {
	return &ToolDispatcher{
		tools:      make(map[string]ToolFactory),
		converters: make(map[string]ArgConverter),
		metadata:   make(map[string]ToolMetadata),
	}
}

// RegisterTool registers a tool with its factory and argument converter
func (d *ToolDispatcher) RegisterTool(name string, factory ToolFactory, converter ArgConverter) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.tools[name]; exists {
		return fmt.Errorf("tool %s is already registered", name)
	}

	// Create a tool instance to get metadata
	tool := factory()
	metadata := tool.GetMetadata()

	d.tools[name] = factory
	d.converters[name] = converter
	d.metadata[name] = metadata

	return nil
}

// GetToolFactory returns the factory for a specific tool
func (d *ToolDispatcher) GetToolFactory(name string) (ToolFactory, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	factory, exists := d.tools[name]
	return factory, exists
}

// ConvertArgs converts generic arguments to tool-specific types
func (d *ToolDispatcher) ConvertArgs(toolName string, args interface{}) (ToolArgs, error) {
	d.mu.RLock()
	converter, exists := d.converters[toolName]
	d.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no argument converter found for tool %s", toolName)
	}

	// Convert args to map if necessary
	argsMap, ok := args.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("arguments must be a map[string]interface{}")
	}

	// Use the converter to create tool-specific args
	toolArgs, err := converter(argsMap)
	if err != nil {
		return nil, fmt.Errorf("failed to convert arguments for tool %s: %w", toolName, err)
	}

	// Validate the arguments
	if err := toolArgs.Validate(); err != nil {
		return nil, fmt.Errorf("argument validation failed for tool %s: %w", toolName, err)
	}

	return toolArgs, nil
}

// GetToolMetadata returns metadata for a specific tool
func (d *ToolDispatcher) GetToolMetadata(name string) (ToolMetadata, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	metadata, exists := d.metadata[name]
	return metadata, exists
}

// ListTools returns a list of all registered tool names
func (d *ToolDispatcher) ListTools() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	tools := make([]string, 0, len(d.tools))
	for name := range d.tools {
		tools = append(tools, name)
	}
	return tools
}

// GetToolsByCategory returns all tools in a specific category
func (d *ToolDispatcher) GetToolsByCategory(category string) []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var tools []string
	for name, metadata := range d.metadata {
		if metadata.Category == category {
			tools = append(tools, name)
		}
	}
	return tools
}

// GetToolsByCapability returns tools that have a specific capability
func (d *ToolDispatcher) GetToolsByCapability(capability string) []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var tools []string
	for name, metadata := range d.metadata {
		for _, cap := range metadata.Capabilities {
			if cap == capability {
				tools = append(tools, name)
				break
			}
		}
	}
	return tools
}

// ValidateTool checks if a tool is properly registered
func (d *ToolDispatcher) ValidateTool(name string) error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if _, exists := d.tools[name]; !exists {
		return fmt.Errorf("tool %s is not registered", name)
	}

	if _, exists := d.converters[name]; !exists {
		return fmt.Errorf("tool %s has no argument converter", name)
	}

	if _, exists := d.metadata[name]; !exists {
		return fmt.Errorf("tool %s has no metadata", name)
	}

	return nil
}
