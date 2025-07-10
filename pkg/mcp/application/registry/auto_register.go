package registry

import (
	"context"
	"fmt"
	"sync"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// Global auto-registration variables for init() pattern
var (
	globalAutoRegistry = make(map[string]api.ToolCreator)
	globalAutoMutex    sync.RWMutex
)

// RegisterTool allows tools to register themselves during init()
// This is the global function called by tool init() functions
func RegisterTool(name string, creator api.ToolCreator) {
	globalAutoMutex.Lock()
	defer globalAutoMutex.Unlock()
	globalAutoRegistry[name] = creator
}

// GetAutoRegisteredTools returns all auto-registered tools from init() functions
func GetAutoRegisteredTools() map[string]api.ToolCreator {
	globalAutoMutex.RLock()
	defer globalAutoMutex.RUnlock()

	tools := make(map[string]api.ToolCreator)
	for k, v := range globalAutoRegistry {
		tools[k] = v
	}
	return tools
}

// LoadAutoRegisteredTools loads all auto-registered tools into a registry
func LoadAutoRegisteredTools(registry api.Registry) error {
	tools := GetAutoRegisteredTools()
	for name, creator := range tools {
		tool, err := creator()
		if err != nil {
			return errors.NewError().
				Message(fmt.Sprintf("failed to create tool %s", name)).
				Cause(err).
				WithLocation().
				Build()
		}
		if err := registry.Register(tool); err != nil {
			return errors.NewError().
				Message(fmt.Sprintf("failed to register tool %s", name)).
				Cause(err).
				WithLocation().
				Build()
		}
	}
	return nil
}

// AutoRegistrar handles automatic tool registration
type AutoRegistrar struct {
	registry  api.Registry
	factories map[string]api.ToolCreator
	schemas   map[string]api.ToolSchema
	mutex     sync.RWMutex
}

// NewAutoRegistrar creates a new auto registrar
func NewAutoRegistrar(registry api.Registry) *AutoRegistrar {
	return &AutoRegistrar{
		registry:  registry,
		factories: make(map[string]api.ToolCreator),
		schemas:   make(map[string]api.ToolSchema),
	}
}

// RegisterCategory registers all tools in a category
func (a *AutoRegistrar) RegisterCategory(category string, tools map[string]api.ToolCreator) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	for name, creator := range tools {
		toolName := fmt.Sprintf("%s_%s", category, name)
		a.factories[toolName] = creator

		// Create schema for the tool
		schema := a.createToolSchema(category, name, toolName)
		a.schemas[toolName] = schema
	}

	return nil
}

// RegisterTool registers a single tool with its creator and schema
func (a *AutoRegistrar) RegisterTool(name string, creator api.ToolCreator, schema api.ToolSchema) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	a.factories[name] = creator
	a.schemas[name] = schema

	return nil
}

// MigrateAllTools migrates all registered tools to the registry
func (a *AutoRegistrar) MigrateAllTools(ctx context.Context) error {
	a.mutex.RLock()
	factories := make(map[string]api.ToolCreator)
	schemas := make(map[string]api.ToolSchema)

	for name, creator := range a.factories {
		factories[name] = creator
	}
	for name, schema := range a.schemas {
		schemas[name] = schema
	}
	a.mutex.RUnlock()

	var migrationErrors []error

	for name, creator := range factories {
		// Check context for cancellation
		select {
		case <-ctx.Done():
			return errors.NewError().
				Message("tool migration cancelled").
				WithLocation().
				Build()
		default:
		}

		// Create tool instance
		tool, err := creator()
		if err != nil {
			migrationError := errors.NewError().
				Message(fmt.Sprintf("failed to create tool %s", name)).
				Cause(err).
				WithLocation().
				Build()
			migrationErrors = append(migrationErrors, migrationError)
			continue
		}

		// Register tool with the registry
		if err := a.registry.Register(tool); err != nil {
			migrationError := errors.NewError().
				Message(fmt.Sprintf("failed to register tool %s", name)).
				Cause(err).
				WithLocation().
				Build()
			migrationErrors = append(migrationErrors, migrationError)
			continue
		}
	}

	// Return combined errors if any
	if len(migrationErrors) > 0 {
		return a.combineErrors(migrationErrors)
	}

	return nil
}

// GetRegisteredToolNames returns all registered tool names
func (a *AutoRegistrar) GetRegisteredToolNames() []string {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	names := make([]string, 0, len(a.factories))
	for name := range a.factories {
		names = append(names, name)
	}
	return names
}

// GetToolSchema returns the schema for a tool
func (a *AutoRegistrar) GetToolSchema(name string) (api.ToolSchema, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	if schema, exists := a.schemas[name]; exists {
		return schema, nil
	}

	return api.ToolSchema{}, errors.NewError().
		Message(fmt.Sprintf("schema not found for tool: %s", name)).
		WithLocation().
		Build()
}

// CreateTool creates a tool instance by name
func (a *AutoRegistrar) CreateTool(name string) (api.Tool, error) {
	a.mutex.RLock()
	creator, exists := a.factories[name]
	a.mutex.RUnlock()

	if !exists {
		return nil, errors.NewError().
			Message(fmt.Sprintf("tool creator not found: %s", name)).
			WithLocation().
			Build()
	}

	return creator()
}

// createToolSchema creates a schema for a tool based on category and name
func (a *AutoRegistrar) createToolSchema(category, name, fullName string) api.ToolSchema {
	switch category {
	case "containerization":
		return a.createContainerizationSchema(name, fullName)
	case "session":
		return a.createSessionSchema(name, fullName)
	default:
		return a.createGenericSchema(fullName)
	}
}

// createContainerizationSchema creates schema for containerization tools
func (a *AutoRegistrar) createContainerizationSchema(name, fullName string) api.ToolSchema {
	switch name {
	case "analyze":
		return api.ToolSchema{
			Name:        fullName,
			Description: "Analyze repository for containerization opportunities",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"repository_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the repository to analyze",
					},
					"output_format": map[string]interface{}{
						"type":        "string",
						"description": "Output format (json, yaml, text)",
						"enum":        []string{"json", "yaml", "text"},
						"default":     "json",
					},
					"deep_scan": map[string]interface{}{
						"type":        "boolean",
						"description": "Perform deep analysis including dependencies",
						"default":     false,
					},
				},
				"required": []string{"repository_path"},
			},
			OutputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"dockerfile_generated": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether a Dockerfile was generated",
					},
					"docker_compose_generated": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether a docker-compose.yml was generated",
					},
					"recommendations": map[string]interface{}{
						"type":        "array",
						"description": "List of containerization recommendations",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
			Category: "containerization",
			Version:  "1.0.0",
		}
	case "build":
		return api.ToolSchema{
			Name:        fullName,
			Description: "Build Docker images with AI-powered optimization",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"dockerfile_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the Dockerfile",
					},
					"image_name": map[string]interface{}{
						"type":        "string",
						"description": "Name for the built image",
					},
					"build_context": map[string]interface{}{
						"type":        "string",
						"description": "Build context directory",
						"default":     ".",
					},
					"build_args": map[string]interface{}{
						"type":        "object",
						"description": "Build arguments",
					},
					"no_cache": map[string]interface{}{
						"type":        "boolean",
						"description": "Don't use cache when building",
						"default":     false,
					},
				},
				"required": []string{"dockerfile_path", "image_name"},
			},
			OutputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"image_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the built image",
					},
					"image_size": map[string]interface{}{
						"type":        "integer",
						"description": "Size of the built image in bytes",
					},
					"build_time": map[string]interface{}{
						"type":        "string",
						"description": "Time taken to build the image",
					},
				},
			},
			Category: "containerization",
			Version:  "1.0.0",
		}
	case "deploy":
		return api.ToolSchema{
			Name:        fullName,
			Description: "Deploy containers to Kubernetes with manifest generation",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"image_name": map[string]interface{}{
						"type":        "string",
						"description": "Docker image to deploy",
					},
					"namespace": map[string]interface{}{
						"type":        "string",
						"description": "Kubernetes namespace",
						"default":     "default",
					},
					"replicas": map[string]interface{}{
						"type":        "integer",
						"description": "Number of replicas",
						"default":     1,
						"minimum":     1,
					},
					"port": map[string]interface{}{
						"type":        "integer",
						"description": "Container port",
						"minimum":     1,
						"maximum":     65535,
					},
					"environment": map[string]interface{}{
						"type":        "object",
						"description": "Environment variables",
					},
				},
				"required": []string{"image_name"},
			},
			OutputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"deployment_name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the created deployment",
					},
					"service_name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the created service",
					},
					"status": map[string]interface{}{
						"type":        "string",
						"description": "Deployment status",
					},
				},
			},
			Category: "containerization",
			Version:  "1.0.0",
		}
	case "scan":
		return api.ToolSchema{
			Name:        fullName,
			Description: "Scan container images for security vulnerabilities",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"image_name": map[string]interface{}{
						"type":        "string",
						"description": "Docker image to scan",
					},
					"scanner": map[string]interface{}{
						"type":        "string",
						"description": "Scanner to use (trivy, grype)",
						"enum":        []string{"trivy", "grype"},
						"default":     "trivy",
					},
					"format": map[string]interface{}{
						"type":        "string",
						"description": "Output format",
						"enum":        []string{"json", "table", "sarif"},
						"default":     "json",
					},
					"severity": map[string]interface{}{
						"type":        "string",
						"description": "Minimum severity level",
						"enum":        []string{"LOW", "MEDIUM", "HIGH", "CRITICAL"},
						"default":     "MEDIUM",
					},
				},
				"required": []string{"image_name"},
			},
			OutputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"vulnerabilities": map[string]interface{}{
						"type":        "array",
						"description": "List of found vulnerabilities",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"id": map[string]interface{}{
									"type": "string",
								},
								"severity": map[string]interface{}{
									"type": "string",
								},
								"description": map[string]interface{}{
									"type": "string",
								},
							},
						},
					},
					"total_vulnerabilities": map[string]interface{}{
						"type":        "integer",
						"description": "Total number of vulnerabilities found",
					},
				},
			},
			Category: "containerization",
			Version:  "1.0.0",
		}
	default:
		return a.createGenericSchema(fullName)
	}
}

// createSessionSchema creates schema for session management tools
func (a *AutoRegistrar) createSessionSchema(name, fullName string) api.ToolSchema {
	switch name {
	case "create":
		return api.ToolSchema{
			Name:        fullName,
			Description: "Create a new session with workspace",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_name": map[string]interface{}{
						"type":        "string",
						"description": "Name for the session",
					},
					"workspace_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to workspace directory",
					},
					"labels": map[string]interface{}{
						"type":        "object",
						"description": "Session labels for organization",
					},
				},
				"required": []string{"session_name"},
			},
			OutputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Unique session identifier",
					},
					"workspace_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to created workspace",
					},
					"created_at": map[string]interface{}{
						"type":        "string",
						"format":      "date-time",
						"description": "Session creation timestamp",
					},
				},
			},
			Category: "session",
			Version:  "1.0.0",
		}
	case "manage":
		return api.ToolSchema{
			Name:        fullName,
			Description: "Manage session lifecycle and metadata",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Session to manage",
					},
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Action to perform",
						"enum":        []string{"get", "update", "delete", "list"},
					},
					"metadata": map[string]interface{}{
						"type":        "object",
						"description": "Updated metadata (for update action)",
					},
				},
				"required": []string{"action"},
			},
			OutputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"session_info": map[string]interface{}{
						"type":        "object",
						"description": "Session information",
					},
					"sessions": map[string]interface{}{
						"type":        "array",
						"description": "List of sessions (for list action)",
						"items": map[string]interface{}{
							"type": "object",
						},
					},
				},
			},
			Category: "session",
			Version:  "1.0.0",
		}
	default:
		return a.createGenericSchema(fullName)
	}
}

// createGenericSchema creates a generic schema for unknown tools
func (a *AutoRegistrar) createGenericSchema(name string) api.ToolSchema {
	return api.ToolSchema{
		Name:        name,
		Description: fmt.Sprintf("Generic tool: %s", name),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for this operation",
				},
			},
			"required": []string{"session_id"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether the operation was successful",
				},
				"data": map[string]interface{}{
					"type":        "object",
					"description": "Operation result data",
				},
			},
		},
		Category: "generic",
		Version:  "1.0.0",
	}
}

// combineErrors combines multiple errors into a single error
func (a *AutoRegistrar) combineErrors(errs []error) error {
	if len(errs) == 1 {
		return errs[0]
	}

	var messages []string
	for _, err := range errs {
		messages = append(messages, err.Error())
	}

	return errors.NewError().
		Message(fmt.Sprintf("multiple migration errors: %v", messages)).
		WithLocation().
		Build()
}

// ListCategories returns all registered categories
func (a *AutoRegistrar) ListCategories() []string {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	categories := make(map[string]bool)

	for name := range a.factories {
		// Extract category from tool name (category_tool format)
		if parts := splitToolName(name); len(parts) == 2 {
			categories[parts[0]] = true
		}
	}

	result := make([]string, 0, len(categories))
	for category := range categories {
		result = append(result, category)
	}

	return result
}

// GetToolsInCategory returns all tools in a specific category
func (a *AutoRegistrar) GetToolsInCategory(category string) []string {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	var tools []string

	for name := range a.factories {
		if parts := splitToolName(name); len(parts) == 2 && parts[0] == category {
			tools = append(tools, parts[1])
		}
	}

	return tools
}

// splitToolName splits a tool name into category and tool parts
func splitToolName(name string) []string {
	// Look for the first underscore to split category_tool
	for i, r := range name {
		if r == '_' {
			return []string{name[:i], name[i+1:]}
		}
	}
	return []string{name} // No underscore found
}

// ClearRegistrations clears all registrations
func (a *AutoRegistrar) ClearRegistrations() {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	a.factories = make(map[string]api.ToolCreator)
	a.schemas = make(map[string]api.ToolSchema)
}

// GetRegistrationCount returns the number of registered tools
func (a *AutoRegistrar) GetRegistrationCount() int {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	return len(a.factories)
}
