package core

// ToolServices provides access to all tool-related services
type ToolServices interface {
	// Registry returns the tool registry service
	Registry() CoreToolRegistry

	// Executor returns the tool executor service
	Executor() ToolExecutor

	// SchemaProvider returns the tool schema service
	SchemaProvider() ToolSchemaProvider
}

// toolServices implements ToolServices
type toolServices struct {
	registry       CoreToolRegistry
	executor       ToolExecutor
	schemaProvider ToolSchemaProvider
}

// NewToolServices creates a new ToolServices container with all services
func NewToolServices(server *UnifiedMCPServer) ToolServices {
	// Create the tool service
	service := NewToolService(server)

	// Create focused services wrapping the service
	return &toolServices{
		registry:       NewCoreToolRegistry(service),
		executor:       NewToolExecutor(service),
		schemaProvider: NewToolSchemaProvider(service),
	}
}

// NewToolServicesFromService creates services from an existing service
// This is useful for gradual migration
func NewToolServicesFromService(service *ToolService) ToolServices {
	return &toolServices{
		registry:       NewCoreToolRegistry(service),
		executor:       NewToolExecutor(service),
		schemaProvider: NewToolSchemaProvider(service),
	}
}

func (t *toolServices) Registry() CoreToolRegistry {
	return t.registry
}

func (t *toolServices) Executor() ToolExecutor {
	return t.executor
}

func (t *toolServices) SchemaProvider() ToolSchemaProvider {
	return t.schemaProvider
}
