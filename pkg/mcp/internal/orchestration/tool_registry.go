package orchestration

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"github.com/Azure/container-kit/pkg/mcp/internal/analyze"
	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	"github.com/Azure/container-kit/pkg/mcp/internal/deploy"
	"github.com/Azure/container-kit/pkg/mcp/internal/scan"
	"github.com/invopop/jsonschema"
	"github.com/rs/zerolog"
)

// MCPToolRegistry implements ToolInstanceRegistry for MCP atomic tools
type MCPToolRegistry struct {
	tools    map[string]ToolInfo
	metadata map[string]*ToolMetadata
	mutex    sync.RWMutex
	logger   zerolog.Logger
}

// ToolInfo contains information about a registered tool
type ToolInfo struct {
	Name         string       `json:"name"`
	Instance     interface{}  `json:"-"`
	Type         reflect.Type `json:"-"`
	Category     string       `json:"category"`
	Description  string       `json:"description"`
	Version      string       `json:"version"`
	Dependencies []string     `json:"dependencies"`
	Capabilities []string     `json:"capabilities"`
}

// NewMCPToolRegistry creates a new tool registry for MCP atomic tools
func NewMCPToolRegistry(logger zerolog.Logger) *MCPToolRegistry {
	registry := &MCPToolRegistry{
		tools:    make(map[string]ToolInfo),
		metadata: make(map[string]*ToolMetadata),
		logger:   logger.With().Str("component", "tool_registry").Logger(),
	}

	// Don't auto-register tools here - they should be registered with proper dependencies
	// by the code that creates them (e.g., gomcp_tools.go)
	// registry.registerAtomicTools()

	return registry
}

// RegisterTool registers a tool in the registry
func (r *MCPToolRegistry) RegisterTool(name string, tool interface{}) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %s is already registered", name)
	}

	toolType := reflect.TypeOf(tool)
	if toolType.Kind() == reflect.Ptr {
		toolType = toolType.Elem()
	}

	// Create tool info
	toolInfo := ToolInfo{
		Name:         name,
		Instance:     tool,
		Type:         toolType,
		Category:     r.inferCategory(name),
		Description:  r.inferDescription(name),
		Version:      "1.0.0",
		Dependencies: r.inferDependencies(name),
		Capabilities: r.inferCapabilities(name),
	}

	// Create metadata
	metadata := &ToolMetadata{
		Name:         name,
		Description:  toolInfo.Description,
		Version:      toolInfo.Version,
		Category:     toolInfo.Category,
		Dependencies: toolInfo.Dependencies,
		Capabilities: toolInfo.Capabilities,
		Requirements: r.inferRequirements(name),
		Parameters:   r.inferParameters(tool),
		OutputSchema: r.inferOutputSchema(tool),
		Examples:     r.generateExamples(name),
	}

	r.tools[name] = toolInfo
	r.metadata[name] = metadata

	r.logger.Info().
		Str("tool_name", name).
		Str("category", toolInfo.Category).
		Str("type", toolType.Name()).
		Msg("Registered tool")

	return nil
}

// GetTool retrieves a tool from the registry
func (r *MCPToolRegistry) GetTool(name string) (interface{}, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	toolInfo, exists := r.tools[name]
	if !exists {
		return nil, fmt.Errorf("tool %s not found", name)
	}

	return toolInfo.Instance, nil
}

// ListTools returns a list of all registered tool names
func (r *MCPToolRegistry) ListTools() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var names []string
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// ValidateTool validates that a tool exists and is properly configured
func (r *MCPToolRegistry) ValidateTool(name string) error {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	toolInfo, exists := r.tools[name]
	if !exists {
		return fmt.Errorf("tool %s is not registered", name)
	}

	// Validate tool instance
	if toolInfo.Instance == nil {
		return fmt.Errorf("tool %s has nil instance", name)
	}

	// Check if tool implements Execute method
	toolValue := reflect.ValueOf(toolInfo.Instance)
	executeMethod := toolValue.MethodByName("Execute")
	if !executeMethod.IsValid() {
		return fmt.Errorf("tool %s does not implement Execute method", name)
	}

	return nil
}

// GetToolMetadata returns metadata for a specific tool
func (r *MCPToolRegistry) GetToolMetadata(name string) (*ToolMetadata, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	metadata, exists := r.metadata[name]
	if !exists {
		return nil, fmt.Errorf("metadata for tool %s not found", name)
	}

	return metadata, nil
}

// GetToolInfo returns information about a tool
func (r *MCPToolRegistry) GetToolInfo(name string) (*ToolInfo, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	toolInfo, exists := r.tools[name]
	if !exists {
		return nil, fmt.Errorf("tool %s not found", name)
	}

	return &toolInfo, nil
}

// GetToolsByCategory returns all tools in a specific category
func (r *MCPToolRegistry) GetToolsByCategory(category string) []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var tools []string
	for name, info := range r.tools {
		if info.Category == category {
			tools = append(tools, name)
		}
	}
	return tools
}

// GetToolCategories returns all available tool categories
func (r *MCPToolRegistry) GetToolCategories() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	categories := make(map[string]bool)
	for _, info := range r.tools {
		categories[info.Category] = true
	}

	var result []string
	for category := range categories {
		result = append(result, category)
	}
	return result
}

// registerAtomicTools registers all atomic tools with their adapters
func (r *MCPToolRegistry) registerAtomicTools() {
	// Repository analysis tools
	r.registerTool("analyze_repository_atomic", &analyze.AtomicAnalyzeRepositoryTool{})

	// Docker tools
	r.registerTool("generate_dockerfile", &analyze.AtomicGenerateDockerfileTool{})
	r.registerTool("validate_dockerfile_atomic", &analyze.AtomicValidateDockerfileTool{})
	r.registerTool("build_image_atomic", &build.AtomicBuildImageTool{})
	r.registerTool("push_image_atomic", &build.AtomicPushImageTool{})
	r.registerTool("pull_image_atomic", &build.AtomicPullImageTool{})
	r.registerTool("tag_image_atomic", &build.AtomicTagImageTool{})

	// Security tools
	r.registerTool("scan_image_security_atomic", &scan.AtomicScanImageSecurityTool{})
	r.registerTool("scan_secrets_atomic", &scan.AtomicScanSecretsTool{})

	// Kubernetes tools
	r.registerTool("generate_manifests_atomic", &deploy.AtomicGenerateManifestsTool{})
	r.registerTool("deploy_kubernetes_atomic", &deploy.AtomicDeployKubernetesTool{})
	r.registerTool("check_health_atomic", &deploy.AtomicCheckHealthTool{})

	r.logger.Info().
		Int("tool_count", len(r.tools)).
		Msg("Registered all atomic tools")
}

// registerTool is a helper method for registering tools
func (r *MCPToolRegistry) registerTool(name string, tool interface{}) {
	if err := r.RegisterTool(name, tool); err != nil {
		r.logger.Error().
			Err(err).
			Str("tool_name", name).
			Msg("Failed to register tool")
	}
}

// Helper methods for inferring tool properties

func (r *MCPToolRegistry) inferCategory(name string) string {
	categoryMap := map[string]string{
		"analyze_repository_atomic":  "analysis",
		"generate_dockerfile":        "generation",
		"validate_dockerfile_atomic": "validation",
		"build_image_atomic":         "build",
		"push_image_atomic":          "registry",
		"pull_image_atomic":          "registry",
		"tag_image_atomic":           "registry",
		"scan_image_security_atomic": "security",
		"scan_secrets_atomic":        "security",
		"generate_manifests_atomic":  "kubernetes",
		"deploy_kubernetes_atomic":   "kubernetes",
		"check_health_atomic":        "monitoring",
	}

	if category, exists := categoryMap[name]; exists {
		return category
	}
	return "general"
}

func (r *MCPToolRegistry) inferDescription(name string) string {
	descMap := map[string]string{
		"analyze_repository_atomic":  "Analyzes repository structure and dependencies",
		"generate_dockerfile":        "Generates optimized Dockerfile from repository analysis",
		"validate_dockerfile_atomic": "Validates Dockerfile syntax and best practices",
		"build_image_atomic":         "Builds Docker image from Dockerfile",
		"push_image_atomic":          "Pushes Docker image to registry",
		"pull_image_atomic":          "Pulls Docker image from registry",
		"tag_image_atomic":           "Tags Docker image with specified tags",
		"scan_image_security_atomic": "Performs security scanning on Docker image",
		"scan_secrets_atomic":        "Scans for secrets and sensitive information",
		"generate_manifests_atomic":  "Generates Kubernetes manifests for deployment",
		"deploy_kubernetes_atomic":   "Deploys application to Kubernetes cluster",
		"check_health_atomic":        "Checks health and readiness of deployed application",
	}

	if desc, exists := descMap[name]; exists {
		return desc
	}
	return "Atomic tool for container operations"
}

func (r *MCPToolRegistry) inferDependencies(name string) []string {
	depMap := map[string][]string{
		"generate_dockerfile":        {"analyze_repository_atomic"},
		"validate_dockerfile_atomic": {"generate_dockerfile"},
		"build_image_atomic":         {"validate_dockerfile_atomic"},
		"push_image_atomic":          {"build_image_atomic"},
		"tag_image_atomic":           {"build_image_atomic"},
		"scan_image_security_atomic": {"build_image_atomic"},
		"generate_manifests_atomic":  {"push_image_atomic"},
		"deploy_kubernetes_atomic":   {"generate_manifests_atomic"},
		"check_health_atomic":        {"deploy_kubernetes_atomic"},
	}

	if deps, exists := depMap[name]; exists {
		return deps
	}
	return []string{}
}

func (r *MCPToolRegistry) inferCapabilities(name string) []string {
	capMap := map[string][]string{
		"analyze_repository_atomic":  {"language_detection", "framework_analysis", "dependency_scanning"},
		"generate_dockerfile":        {"template_selection", "optimization", "best_practices"},
		"validate_dockerfile_atomic": {"syntax_validation", "security_checks", "best_practices"},
		"build_image_atomic":         {"docker_build", "layer_optimization", "caching"},
		"push_image_atomic":          {"registry_auth", "multi_architecture", "retries"},
		"pull_image_atomic":          {"registry_auth", "verification", "caching"},
		"tag_image_atomic":           {"semantic_versioning", "multi_tagging", "metadata"},
		"scan_image_security_atomic": {"vulnerability_scanning", "compliance_checks", "reporting"},
		"scan_secrets_atomic":        {"secret_detection", "pattern_matching", "false_positive_reduction"},
		"generate_manifests_atomic":  {"template_generation", "secret_management", "resource_optimization"},
		"deploy_kubernetes_atomic":   {"rolling_deployment", "health_checks", "rollback"},
		"check_health_atomic":        {"endpoint_monitoring", "kubernetes_probes", "custom_checks"},
	}

	if caps, exists := capMap[name]; exists {
		return caps
	}
	return []string{"basic_execution"}
}

func (r *MCPToolRegistry) inferRequirements(name string) []string {
	reqMap := map[string][]string{
		"analyze_repository_atomic":  {"repository_access"},
		"build_image_atomic":         {"docker_daemon"},
		"push_image_atomic":          {"docker_daemon", "registry_access"},
		"pull_image_atomic":          {"docker_daemon", "registry_access"},
		"tag_image_atomic":           {"docker_daemon"},
		"scan_image_security_atomic": {"docker_daemon", "security_scanner"},
		"generate_manifests_atomic":  {"kubernetes_templates"},
		"deploy_kubernetes_atomic":   {"kubernetes_access"},
		"check_health_atomic":        {"kubernetes_access", "network_access"},
	}

	if reqs, exists := reqMap[name]; exists {
		return reqs
	}
	return []string{}
}

func (r *MCPToolRegistry) inferParameters(tool interface{}) map[string]interface{} {
	// Use reflection to infer parameters from the tool's Execute method
	toolValue := reflect.ValueOf(tool)
	toolType := toolValue.Type()

	// Find Execute method
	var executeMethod reflect.Method
	var found bool
	for i := 0; i < toolType.NumMethod(); i++ {
		method := toolType.Method(i)
		if method.Name == "Execute" {
			executeMethod = method
			found = true
			break
		}
	}

	if !found {
		return map[string]interface{}{}
	}

	// Analyze method parameters
	methodType := executeMethod.Type

	if methodType.NumIn() >= 3 { // receiver, context, args
		argsType := methodType.In(2)

		// Use invopop/jsonschema to generate proper JSON schema
		reflector := &jsonschema.Reflector{
			RequiredFromJSONSchemaTags: true,
			AllowAdditionalProperties:  false,
			DoNotReference:             true,
		}

		schema := reflector.Reflect(argsType)

		// Convert to map
		schemaJSON, err := json.Marshal(schema)
		if err != nil {
			r.logger.Error().Err(err).Str("type", argsType.Name()).Msg("Failed to marshal schema")
			return map[string]interface{}{}
		}

		var schemaMap map[string]interface{}
		if err := json.Unmarshal(schemaJSON, &schemaMap); err != nil {
			r.logger.Error().Err(err).Str("type", argsType.Name()).Msg("Failed to unmarshal schema")
			return map[string]interface{}{}
		}

		// Sanitize the schema to ensure array types have items
		r.sanitizeInvopopSchema(schemaMap)

		return schemaMap
	}

	return map[string]interface{}{}
}

func (r *MCPToolRegistry) inferOutputSchema(tool interface{}) map[string]interface{} {
	// Use reflection to infer output schema from the tool's Execute method
	toolValue := reflect.ValueOf(tool)
	toolType := toolValue.Type()

	// Find Execute method
	var executeMethod reflect.Method
	var found bool
	for i := 0; i < toolType.NumMethod(); i++ {
		method := toolType.Method(i)
		if method.Name == "Execute" {
			executeMethod = method
			found = true
			break
		}
	}

	if !found {
		return map[string]interface{}{}
	}

	// Analyze method return types
	methodType := executeMethod.Type

	if methodType.NumOut() >= 1 {
		returnType := methodType.Out(0)
		if returnType.Kind() == reflect.Ptr {
			returnType = returnType.Elem()
		}

		// Use invopop/jsonschema to generate proper JSON schema
		reflector := &jsonschema.Reflector{
			RequiredFromJSONSchemaTags: true,
			AllowAdditionalProperties:  false,
			DoNotReference:             true,
		}

		schema := reflector.Reflect(returnType)

		// Convert to map
		schemaJSON, err := json.Marshal(schema)
		if err != nil {
			r.logger.Error().Err(err).Str("type", returnType.Name()).Msg("Failed to marshal output schema")
			return map[string]interface{}{}
		}

		var schemaMap map[string]interface{}
		if err := json.Unmarshal(schemaJSON, &schemaMap); err != nil {
			r.logger.Error().Err(err).Str("type", returnType.Name()).Msg("Failed to unmarshal output schema")
			return map[string]interface{}{}
		}

		// Sanitize the schema to ensure array types have items
		r.sanitizeInvopopSchema(schemaMap)

		return schemaMap
	}

	return map[string]interface{}{}
}

func (r *MCPToolRegistry) generateExamples(name string) []ToolExample {
	// Generate basic examples for each tool
	exampleMap := map[string][]ToolExample{
		"analyze_repository_atomic": {
			{
				Name:        "Basic Repository Analysis",
				Description: "Analyze a GitHub repository",
				Input: map[string]interface{}{
					"session_id": "example-session",
					"repo_url":   "https://github.com/example/app",
				},
				Output: map[string]interface{}{
					"language":        "javascript",
					"framework":       "express",
					"package_manager": "npm",
				},
			},
		},
		"build_image_atomic": {
			{
				Name:        "Basic Image Build",
				Description: "Build Docker image from Dockerfile",
				Input: map[string]interface{}{
					"session_id": "example-session",
					"image_name": "myapp",
					"tag":        "latest",
				},
				Output: map[string]interface{}{
					"success":    true,
					"image_id":   "sha256:abc123...",
					"image_size": "150MB",
				},
			},
		},
	}

	if examples, exists := exampleMap[name]; exists {
		return examples
	}

	// Return default example
	return []ToolExample{
		{
			Name:        "Basic Usage",
			Description: fmt.Sprintf("Basic usage of %s tool", name),
			Input:       map[string]interface{}{"session_id": "example-session"},
			Output:      map[string]interface{}{"success": true},
		},
	}
}

// sanitizeInvopopSchema ensures that all array types have an "items" property
func (r *MCPToolRegistry) sanitizeInvopopSchema(schema map[string]interface{}) {
	if schema == nil {
		return
	}

	// Check if this is an array type that needs items
	if schemaType, ok := schema["type"].(string); ok && schemaType == "array" {
		if _, hasItems := schema["items"]; !hasItems {
			// Default to string items if not specified
			schema["items"] = map[string]interface{}{
				"type": "string",
			}
			r.logger.Warn().
				Str("schema_type", "array").
				Msg("Added missing items property to array schema")
		}
	}

	// Recursively check properties
	if properties, ok := schema["properties"].(map[string]interface{}); ok {
		for _, propValue := range properties {
			if propSchema, ok := propValue.(map[string]interface{}); ok {
				r.sanitizeInvopopSchema(propSchema)
			}
		}
	}

	// Check items if this is an array
	if items, ok := schema["items"].(map[string]interface{}); ok {
		r.sanitizeInvopopSchema(items)
	}

	// Check additional properties
	if additionalProps, ok := schema["additionalProperties"].(map[string]interface{}); ok {
		r.sanitizeInvopopSchema(additionalProps)
	}
}
