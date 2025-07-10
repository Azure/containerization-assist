package api

import (
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// StaticTypedSchema provides type-safe schema generation without reflection
type StaticTypedSchema struct {
	Type        string                 `json:"type"`
	Format      string                 `json:"format,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
	Items       interface{}            `json:"items,omitempty"`
	Required    []string               `json:"required,omitempty"`
	Description string                 `json:"description,omitempty"`
	Example     interface{}            `json:"example,omitempty"`
	Enum        []interface{}          `json:"enum,omitempty"`
	Minimum     *float64               `json:"minimum,omitempty"`
	Maximum     *float64               `json:"maximum,omitempty"`
	MinLength   *int                   `json:"minLength,omitempty"`
	MaxLength   *int                   `json:"maxLength,omitempty"`
	Pattern     string                 `json:"pattern,omitempty"`
}

// StaticSchemaBuilder provides a fluent API for building schemas
type StaticSchemaBuilder struct {
	schema StaticTypedSchema
}

// NewStaticSchemaBuilder creates a new schema builder
func NewStaticSchemaBuilder() *StaticSchemaBuilder {
	return &StaticSchemaBuilder{
		schema: StaticTypedSchema{
			Properties: make(map[string]interface{}),
		},
	}
}

// Type sets the schema type
func (b *StaticSchemaBuilder) Type(t string) *StaticSchemaBuilder {
	b.schema.Type = t
	return b
}

// Format sets the schema format
func (b *StaticSchemaBuilder) Format(f string) *StaticSchemaBuilder {
	b.schema.Format = f
	return b
}

// Description sets the schema description
func (b *StaticSchemaBuilder) Description(desc string) *StaticSchemaBuilder {
	b.schema.Description = desc
	return b
}

// Property adds a property to the schema
func (b *StaticSchemaBuilder) Property(name string, schema interface{}) *StaticSchemaBuilder {
	b.schema.Properties[name] = schema
	return b
}

// Required adds required fields
func (b *StaticSchemaBuilder) Required(fields ...string) *StaticSchemaBuilder {
	b.schema.Required = append(b.schema.Required, fields...)
	return b
}

// Items sets the items schema for arrays
func (b *StaticSchemaBuilder) Items(items interface{}) *StaticSchemaBuilder {
	b.schema.Items = items
	return b
}

// Example sets an example value
func (b *StaticSchemaBuilder) Example(example interface{}) *StaticSchemaBuilder {
	b.schema.Example = example
	return b
}

// Enum sets enum values
func (b *StaticSchemaBuilder) Enum(values ...interface{}) *StaticSchemaBuilder {
	b.schema.Enum = values
	return b
}

// MinLength sets minimum length
func (b *StaticSchemaBuilder) MinLength(length int) *StaticSchemaBuilder {
	b.schema.MinLength = &length
	return b
}

// MaxLength sets maximum length
func (b *StaticSchemaBuilder) MaxLength(length int) *StaticSchemaBuilder {
	b.schema.MaxLength = &length
	return b
}

// Pattern sets a regex pattern
func (b *StaticSchemaBuilder) Pattern(pattern string) *StaticSchemaBuilder {
	b.schema.Pattern = pattern
	return b
}

// Minimum sets minimum value
func (b *StaticSchemaBuilder) Minimum(minValue float64) *StaticSchemaBuilder {
	b.schema.Minimum = &minValue
	return b
}

// Maximum sets maximum value
func (b *StaticSchemaBuilder) Maximum(maxValue float64) *StaticSchemaBuilder {
	b.schema.Maximum = &maxValue
	return b
}

// Build returns the built schema
func (b *StaticSchemaBuilder) Build() StaticTypedSchema {
	return b.schema
}

// Predefined schema builders for common types

// StringSchema creates a string schema
func StringSchema() *StaticSchemaBuilder {
	return NewStaticSchemaBuilder().Type("string")
}

// IntegerSchema creates an integer schema
func IntegerSchema() *StaticSchemaBuilder {
	return NewStaticSchemaBuilder().Type("integer")
}

// NumberSchema creates a number schema
func NumberSchema() *StaticSchemaBuilder {
	return NewStaticSchemaBuilder().Type("number")
}

// BooleanSchema creates a boolean schema
func BooleanSchema() *StaticSchemaBuilder {
	return NewStaticSchemaBuilder().Type("boolean")
}

// ArraySchema creates an array schema
func ArraySchema(itemSchema interface{}) *StaticSchemaBuilder {
	return NewStaticSchemaBuilder().Type("array").Items(itemSchema)
}

// ObjectSchema creates an object schema
func ObjectSchema() *StaticSchemaBuilder {
	return NewStaticSchemaBuilder().Type("object")
}

// Common schema creation functions

// CreateStringFieldSchema creates a string field schema with constraints
func CreateStringFieldSchema(minLen, maxLen int, pattern string, description string) StaticTypedSchema {
	builder := StringSchema().Description(description)

	if minLen > 0 {
		builder.MinLength(minLen)
	}
	if maxLen > 0 {
		builder.MaxLength(maxLen)
	}
	if pattern != "" {
		builder.Pattern(pattern)
	}

	return builder.Build()
}

// CreateIntegerFieldSchema creates an integer field schema with constraints
func CreateIntegerFieldSchema(minValue, maxValue *int, description string) StaticTypedSchema {
	builder := IntegerSchema().Description(description)

	if minValue != nil {
		builder.Minimum(float64(*minValue))
	}
	if maxValue != nil {
		builder.Maximum(float64(*maxValue))
	}

	return builder.Build()
}

// CreateEnumFieldSchema creates an enum field schema
func CreateEnumFieldSchema(values []string, description string) StaticTypedSchema {
	enumValues := make([]interface{}, len(values))
	for i, v := range values {
		enumValues[i] = v
	}

	return StringSchema().
		Description(description).
		Enum(enumValues...).
		Build()
}

// CreateArrayFieldSchema creates an array field schema
func CreateArrayFieldSchema(itemSchema interface{}, minItems, maxItems int, description string) StaticTypedSchema {
	builder := ArraySchema(itemSchema).Description(description)

	if minItems > 0 {
		builder.MinLength(minItems)
	}
	if maxItems > 0 {
		builder.MaxLength(maxItems)
	}

	return builder.Build()
}

// CreateObjectFieldSchema creates an object field schema
func CreateObjectFieldSchema(properties map[string]interface{}, required []string, description string) StaticTypedSchema {
	builder := ObjectSchema().Description(description)

	for name, schema := range properties {
		builder.Property(name, schema)
	}

	if len(required) > 0 {
		builder.Required(required...)
	}

	return builder.Build()
}

// Specific schema creators for common tool types

// CreateToolInputSchema creates a standard tool input schema
func CreateToolInputSchema(properties map[string]interface{}, required []string) StaticTypedSchema {
	// Add standard fields
	properties["session_id"] = CreateStringFieldSchema(1, 100, "", "Session ID for this operation")

	// Ensure session_id is required
	if required == nil {
		required = []string{"session_id"}
	} else {
		// Check if session_id is already in required
		found := false
		for _, field := range required {
			if field == "session_id" {
				found = true
				break
			}
		}
		if !found {
			required = append(required, "session_id")
		}
	}

	return CreateObjectFieldSchema(properties, required, "Tool input parameters")
}

// CreateToolOutputSchema creates a standard tool output schema
func CreateToolOutputSchema(properties map[string]interface{}) StaticTypedSchema {
	// Add standard fields
	standardProps := map[string]interface{}{
		"success": CreateBooleanFieldSchema("Whether the operation was successful"),
		"data":    CreateObjectFieldSchema(nil, nil, "Operation result data"),
		"error":   CreateStringFieldSchema(0, 1000, "", "Error message if operation failed"),
	}

	// Merge with provided properties
	for name, schema := range properties {
		standardProps[name] = schema
	}

	return CreateObjectFieldSchema(standardProps, []string{"success"}, "Tool output")
}

// CreateBooleanFieldSchema creates a boolean field schema
func CreateBooleanFieldSchema(description string) StaticTypedSchema {
	return BooleanSchema().Description(description).Build()
}

// Pre-defined schemas for containerization tools

// ContainerizationAnalyzeInputSchema creates schema for analyze tool input
func ContainerizationAnalyzeInputSchema() StaticTypedSchema {
	properties := map[string]interface{}{
		"repository_path": CreateStringFieldSchema(1, 500, "", "Path to the repository to analyze"),
		"output_format":   CreateEnumFieldSchema([]string{"json", "yaml", "text"}, "Output format"),
		"deep_scan":       CreateBooleanFieldSchema("Perform deep analysis including dependencies"),
	}

	return CreateToolInputSchema(properties, []string{"repository_path"})
}

// ContainerizationAnalyzeOutputSchema creates schema for analyze tool output
func ContainerizationAnalyzeOutputSchema() StaticTypedSchema {
	properties := map[string]interface{}{
		"dockerfile_generated":     CreateBooleanFieldSchema("Whether a Dockerfile was generated"),
		"docker_compose_generated": CreateBooleanFieldSchema("Whether a docker-compose.yml was generated"),
		"recommendations":          CreateArrayFieldSchema(StringSchema().Build(), 0, 0, "List of containerization recommendations"),
	}

	return CreateToolOutputSchema(properties)
}

// ContainerizationBuildInputSchema creates schema for build tool input
func ContainerizationBuildInputSchema() StaticTypedSchema {
	properties := map[string]interface{}{
		"dockerfile_path": CreateStringFieldSchema(1, 500, "", "Path to the Dockerfile"),
		"image_name":      CreateStringFieldSchema(1, 200, "", "Name for the built image"),
		"build_context":   CreateStringFieldSchema(1, 500, "", "Build context directory"),
		"build_args":      CreateObjectFieldSchema(nil, nil, "Build arguments"),
		"no_cache":        CreateBooleanFieldSchema("Don't use cache when building"),
	}

	return CreateToolInputSchema(properties, []string{"dockerfile_path", "image_name"})
}

// ContainerizationBuildOutputSchema creates schema for build tool output
func ContainerizationBuildOutputSchema() StaticTypedSchema {
	properties := map[string]interface{}{
		"image_id":   CreateStringFieldSchema(1, 100, "", "ID of the built image"),
		"image_size": CreateIntegerFieldSchema(intPtr(0), nil, "Size of the built image in bytes"),
		"build_time": CreateStringFieldSchema(1, 50, "", "Time taken to build the image"),
	}

	return CreateToolOutputSchema(properties)
}

// ContainerizationDeployInputSchema creates schema for deploy tool input
func ContainerizationDeployInputSchema() StaticTypedSchema {
	properties := map[string]interface{}{
		"image_name":  CreateStringFieldSchema(1, 200, "", "Docker image to deploy"),
		"namespace":   CreateStringFieldSchema(1, 100, "", "Kubernetes namespace"),
		"replicas":    CreateIntegerFieldSchema(intPtr(1), intPtr(100), "Number of replicas"),
		"port":        CreateIntegerFieldSchema(intPtr(1), intPtr(65535), "Container port"),
		"environment": CreateObjectFieldSchema(nil, nil, "Environment variables"),
	}

	return CreateToolInputSchema(properties, []string{"image_name"})
}

// ContainerizationDeployOutputSchema creates schema for deploy tool output
func ContainerizationDeployOutputSchema() StaticTypedSchema {
	properties := map[string]interface{}{
		"deployment_name": CreateStringFieldSchema(1, 100, "", "Name of the created deployment"),
		"service_name":    CreateStringFieldSchema(1, 100, "", "Name of the created service"),
		"status":          CreateStringFieldSchema(1, 50, "", "Deployment status"),
	}

	return CreateToolOutputSchema(properties)
}

// ContainerizationScanInputSchema creates schema for scan tool input
func ContainerizationScanInputSchema() StaticTypedSchema {
	properties := map[string]interface{}{
		"image_name": CreateStringFieldSchema(1, 200, "", "Docker image to scan"),
		"scanner":    CreateEnumFieldSchema([]string{"trivy", "grype"}, "Scanner to use"),
		"format":     CreateEnumFieldSchema([]string{"json", "table", "sarif"}, "Output format"),
		"severity":   CreateEnumFieldSchema([]string{"LOW", "MEDIUM", "HIGH", "CRITICAL"}, "Minimum severity level"),
	}

	return CreateToolInputSchema(properties, []string{"image_name"})
}

// ContainerizationScanOutputSchema creates schema for scan tool output
func ContainerizationScanOutputSchema() StaticTypedSchema {
	vulnerabilitySchema := CreateObjectFieldSchema(map[string]interface{}{
		"id":          CreateStringFieldSchema(1, 100, "", "Vulnerability ID"),
		"severity":    CreateStringFieldSchema(1, 20, "", "Vulnerability severity"),
		"description": CreateStringFieldSchema(1, 1000, "", "Vulnerability description"),
	}, []string{"id", "severity"}, "Vulnerability information")

	properties := map[string]interface{}{
		"vulnerabilities":       CreateArrayFieldSchema(vulnerabilitySchema, 0, 0, "List of found vulnerabilities"),
		"total_vulnerabilities": CreateIntegerFieldSchema(intPtr(0), nil, "Total number of vulnerabilities found"),
	}

	return CreateToolOutputSchema(properties)
}

// Session management schemas

// SessionCreateInputSchema creates schema for session create tool input
func SessionCreateInputSchema() StaticTypedSchema {
	properties := map[string]interface{}{
		"session_name":   CreateStringFieldSchema(1, 100, "", "Name for the session"),
		"workspace_path": CreateStringFieldSchema(1, 500, "", "Path to workspace directory"),
		"labels":         CreateObjectFieldSchema(nil, nil, "Session labels for organization"),
	}

	return CreateToolInputSchema(properties, []string{"session_name"})
}

// SessionCreateOutputSchema creates schema for session create tool output
func SessionCreateOutputSchema() StaticTypedSchema {
	properties := map[string]interface{}{
		"session_id":     CreateStringFieldSchema(1, 100, "", "Unique session identifier"),
		"workspace_path": CreateStringFieldSchema(1, 500, "", "Path to created workspace"),
		"created_at":     CreateDateTimeFieldSchema("Session creation timestamp"),
	}

	return CreateToolOutputSchema(properties)
}

// SessionManageInputSchema creates schema for session manage tool input
func SessionManageInputSchema() StaticTypedSchema {
	properties := map[string]interface{}{
		"session_id": CreateStringFieldSchema(1, 100, "", "Session to manage"),
		"action":     CreateEnumFieldSchema([]string{"get", "update", "delete", "list"}, "Action to perform"),
		"metadata":   CreateObjectFieldSchema(nil, nil, "Updated metadata (for update action)"),
	}

	return CreateToolInputSchema(properties, []string{"action"})
}

// SessionManageOutputSchema creates schema for session manage tool output
func SessionManageOutputSchema() StaticTypedSchema {
	sessionInfoSchema := CreateObjectFieldSchema(map[string]interface{}{
		"id":         CreateStringFieldSchema(1, 100, "", "Session ID"),
		"name":       CreateStringFieldSchema(1, 100, "", "Session name"),
		"created_at": CreateDateTimeFieldSchema("Creation timestamp"),
	}, []string{"id"}, "Session information")

	properties := map[string]interface{}{
		"session_info": sessionInfoSchema,
		"sessions":     CreateArrayFieldSchema(sessionInfoSchema, 0, 0, "List of sessions (for list action)"),
	}

	return CreateToolOutputSchema(properties)
}

// Helper functions

// CreateDateTimeFieldSchema creates a date-time field schema
func CreateDateTimeFieldSchema(description string) StaticTypedSchema {
	return StringSchema().
		Format("date-time").
		Description(description).
		Example(time.Now().Format(time.RFC3339)).
		Build()
}

// intPtr returns a pointer to an int
func intPtr(i int) *int {
	return &i
}

// StaticTypedSchemaRegistry manages static typed schemas
type StaticTypedSchemaRegistry struct {
	schemas map[string]StaticTypedSchema
}

// NewStaticTypedSchemaRegistry creates a new schema registry
func NewStaticTypedSchemaRegistry() *StaticTypedSchemaRegistry {
	registry := &StaticTypedSchemaRegistry{
		schemas: make(map[string]StaticTypedSchema),
	}

	// Register built-in schemas
	registry.registerBuiltinSchemas()

	return registry
}

// registerBuiltinSchemas registers all built-in schemas
func (r *StaticTypedSchemaRegistry) registerBuiltinSchemas() {
	// Containerization tool schemas
	r.schemas["containerization_analyze_input"] = ContainerizationAnalyzeInputSchema()
	r.schemas["containerization_analyze_output"] = ContainerizationAnalyzeOutputSchema()
	r.schemas["containerization_build_input"] = ContainerizationBuildInputSchema()
	r.schemas["containerization_build_output"] = ContainerizationBuildOutputSchema()
	r.schemas["containerization_deploy_input"] = ContainerizationDeployInputSchema()
	r.schemas["containerization_deploy_output"] = ContainerizationDeployOutputSchema()
	r.schemas["containerization_scan_input"] = ContainerizationScanInputSchema()
	r.schemas["containerization_scan_output"] = ContainerizationScanOutputSchema()

	// Session management schemas
	r.schemas["session_create_input"] = SessionCreateInputSchema()
	r.schemas["session_create_output"] = SessionCreateOutputSchema()
	r.schemas["session_manage_input"] = SessionManageInputSchema()
	r.schemas["session_manage_output"] = SessionManageOutputSchema()
}

// GetSchema retrieves a schema by name
func (r *StaticTypedSchemaRegistry) GetSchema(name string) (StaticTypedSchema, error) {
	if schema, exists := r.schemas[name]; exists {
		return schema, nil
	}

	return StaticTypedSchema{}, fmt.Errorf("schema not found: %s", name)
}

// RegisterSchema registers a custom schema
func (r *StaticTypedSchemaRegistry) RegisterSchema(name string, schema StaticTypedSchema) {
	r.schemas[name] = schema
}

// ListSchemas returns all registered schema names
func (r *StaticTypedSchemaRegistry) ListSchemas() []string {
	names := make([]string, 0, len(r.schemas))
	for name := range r.schemas {
		names = append(names, name)
	}
	return names
}

// Global schema registry instance
var globalStaticTypedSchemaRegistry = NewStaticTypedSchemaRegistry()

// GetGlobalStaticTypedSchemaRegistry returns the global schema registry
func GetGlobalStaticTypedSchemaRegistry() *StaticTypedSchemaRegistry {
	return globalStaticTypedSchemaRegistry
}

// CreateTypedToolSchema creates a tool schema from static typed schemas
func CreateTypedToolSchema[Input any, Output any](name, description string, inputSchemaName, outputSchemaName string) (ToolSchema, error) {
	registry := GetGlobalStaticTypedSchemaRegistry()

	inputSchema, err := registry.GetSchema(inputSchemaName)
	if err != nil {
		return ToolSchema{}, errors.NewError().Code(errors.CodeInternalError).Message("failed to get input schema").Cause(err).Build()
	}

	outputSchema, err := registry.GetSchema(outputSchemaName)
	if err != nil {
		return ToolSchema{}, errors.NewError().Code(errors.CodeInternalError).Message("failed to get output schema").Cause(err).Build()
	}

	return ToolSchema{
		Name:         name,
		Description:  description,
		InputSchema:  staticTypedSchemaToMap(inputSchema),
		OutputSchema: staticTypedSchemaToMap(outputSchema),
		Version:      "1.0.0",
	}, nil
}

// staticTypedSchemaToMap converts StaticTypedSchema to map[string]interface{}
func staticTypedSchemaToMap(schema StaticTypedSchema) map[string]interface{} {
	result := map[string]interface{}{
		"type": schema.Type,
	}

	if schema.Format != "" {
		result["format"] = schema.Format
	}
	if schema.Description != "" {
		result["description"] = schema.Description
	}
	if len(schema.Properties) > 0 {
		result["properties"] = schema.Properties
	}
	if schema.Items != nil {
		result["items"] = schema.Items
	}
	if len(schema.Required) > 0 {
		result["required"] = schema.Required
	}
	if schema.Example != nil {
		result["example"] = schema.Example
	}
	if len(schema.Enum) > 0 {
		result["enum"] = schema.Enum
	}
	if schema.Minimum != nil {
		result["minimum"] = *schema.Minimum
	}
	if schema.Maximum != nil {
		result["maximum"] = *schema.Maximum
	}
	if schema.MinLength != nil {
		result["minLength"] = *schema.MinLength
	}
	if schema.MaxLength != nil {
		result["maxLength"] = *schema.MaxLength
	}
	if schema.Pattern != "" {
		result["pattern"] = schema.Pattern
	}

	return result
}
