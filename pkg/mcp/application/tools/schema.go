package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// Schema represents a typed schema structure for tools
type Schema[TParams any, TResult any] struct {
	Name         string                      `json:"name"`
	Description  string                      `json:"description"`
	Version      string                      `json:"version"`
	ParamsSchema *JSONSchema                 `json:"params_schema"`
	ResultSchema *JSONSchema                 `json:"result_schema"`
	Examples     []Example[TParams, TResult] `json:"examples"`
}

// Example represents a typed example for a tool
type Example[TParams any, TResult any] struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Input       TParams `json:"input"`
	Output      TResult `json:"output"`
}

// JSONSchema represents a JSON Schema structure without reflection
type JSONSchema struct {
	Type                 string                 `json:"type,omitempty"`
	Format               string                 `json:"format,omitempty"`
	Title                string                 `json:"title,omitempty"`
	Description          string                 `json:"description,omitempty"`
	Items                *JSONSchema            `json:"items,omitempty"`
	Properties           map[string]*JSONSchema `json:"properties,omitempty"`
	Required             []string               `json:"required,omitempty"`
	AdditionalProperties *bool                  `json:"additionalProperties,omitempty"`
	Minimum              *float64               `json:"minimum,omitempty"`
	Maximum              *float64               `json:"maximum,omitempty"`
	MinLength            *int                   `json:"minLength,omitempty"`
	MaxLength            *int                   `json:"maxLength,omitempty"`
	Pattern              string                 `json:"pattern,omitempty"`
	Enum                 []any                  `json:"enum,omitempty"`
	Example              any                    `json:"example,omitempty"`
	Ref                  string                 `json:"$ref,omitempty"`
	Definitions          map[string]*JSONSchema `json:"definitions,omitempty"`
	AllOf                []*JSONSchema          `json:"allOf,omitempty"`
	AnyOf                []*JSONSchema          `json:"anyOf,omitempty"`
	OneOf                []*JSONSchema          `json:"oneOf,omitempty"`
}

// ToMap converts JSONSchema to map[string]any for compatibility
func (s *JSONSchema) ToMap() map[string]any {
	data, _ := json.Marshal(s)
	var result map[string]any
	json.Unmarshal(data, &result)
	return result
}

// FromMap creates JSONSchema from map[string]any
func FromMap(m map[string]any) *JSONSchema {
	data, _ := json.Marshal(m)
	var schema JSONSchema
	json.Unmarshal(data, &schema)
	return &schema
}

// StaticSchemaBuilder provides a fluent API for building schemas without reflection
type StaticSchemaBuilder struct {
	schema *JSONSchema
}

// NewStaticSchemaBuilder creates a new schema builder
func NewStaticSchemaBuilder() *StaticSchemaBuilder {
	return &StaticSchemaBuilder{
		schema: &JSONSchema{},
	}
}

// Type sets the schema type
func (b *StaticSchemaBuilder) Type(t string) *StaticSchemaBuilder {
	b.schema.Type = t
	return b
}

// Description sets the schema description
func (b *StaticSchemaBuilder) Description(desc string) *StaticSchemaBuilder {
	b.schema.Description = desc
	return b
}

// Properties sets the schema properties
func (b *StaticSchemaBuilder) Properties(props map[string]*JSONSchema) *StaticSchemaBuilder {
	b.schema.Properties = props
	return b
}

// Required sets the required fields
func (b *StaticSchemaBuilder) Required(fields ...string) *StaticSchemaBuilder {
	b.schema.Required = fields
	return b
}

// MinLength sets the minimum length
func (b *StaticSchemaBuilder) MinLength(length int) *StaticSchemaBuilder {
	b.schema.MinLength = &length
	return b
}

// MaxLength sets the maximum length
func (b *StaticSchemaBuilder) MaxLength(length int) *StaticSchemaBuilder {
	b.schema.MaxLength = &length
	return b
}

// Pattern sets the pattern
func (b *StaticSchemaBuilder) Pattern(pattern string) *StaticSchemaBuilder {
	b.schema.Pattern = pattern
	return b
}

// Enum sets the enum values
func (b *StaticSchemaBuilder) Enum(values ...any) *StaticSchemaBuilder {
	b.schema.Enum = values
	return b
}

// Build returns the built schema
func (b *StaticSchemaBuilder) Build() *JSONSchema {
	return b.schema
}

// Predefined schema builders for common types

// StringSchemaBuilder creates a string schema builder
func StringSchemaBuilder() *StaticSchemaBuilder {
	return NewStaticSchemaBuilder().Type("string")
}

// IntegerSchemaBuilder creates an integer schema builder
func IntegerSchemaBuilder() *StaticSchemaBuilder {
	return NewStaticSchemaBuilder().Type("integer")
}

// NumberSchemaBuilder creates a number schema builder
func NumberSchemaBuilder() *StaticSchemaBuilder {
	return NewStaticSchemaBuilder().Type("number")
}

// BooleanSchemaBuilder creates a boolean schema builder
func BooleanSchemaBuilder() *StaticSchemaBuilder {
	return NewStaticSchemaBuilder().Type("boolean")
}

// ArraySchemaBuilder creates an array schema builder
func ArraySchemaBuilder(itemSchema *JSONSchema) *StaticSchemaBuilder {
	builder := NewStaticSchemaBuilder().Type("array")
	builder.schema.Items = itemSchema
	return builder
}

// ObjectSchemaBuilder creates an object schema builder
func ObjectSchemaBuilder() *StaticSchemaBuilder {
	return NewStaticSchemaBuilder().Type("object")
}

// Common schema templates

// CreateStringSchema creates a string schema with constraints
func CreateStringSchema(minLen, maxLen int, pattern string) *JSONSchema {
	builder := StringSchemaBuilder()

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

// CreateEnumSchema creates an enum schema
func CreateEnumSchema(values []string) *JSONSchema {
	enumVals := make([]any, len(values))
	for i, v := range values {
		enumVals[i] = v
	}

	return StringSchemaBuilder().Enum(enumVals...).Build()
}

// CreateObjectSchema creates an object schema with properties
func CreateObjectSchema(properties map[string]*JSONSchema, required []string) *JSONSchema {
	builder := ObjectSchemaBuilder().Properties(properties)

	if len(required) > 0 {
		builder.Required(required...)
	}

	return builder.Build()
}

// CreateArraySchema creates an array schema
func CreateArraySchema(itemSchema *JSONSchema) *JSONSchema {
	return &JSONSchema{
		Type:  "array",
		Items: itemSchema,
	}
}

// Validation functions

// ValidateAgainstSchema validates data against a JSON schema
func ValidateAgainstSchema(data any, schema *JSONSchema) error {
	if schema.Type == "" {
		return nil // No type constraint
	}

	dataType := getJSONType(data)
	if dataType != schema.Type {
		return errors.NewError().
			Message(fmt.Sprintf("expected type %s, got %s", schema.Type, dataType)).
			WithLocation().
			Build()
	}

	return nil
}

// getJSONType determines the JSON type of a value
func getJSONType(v any) string {
	if v == nil {
		return "null"
	}

	switch v.(type) {
	case bool:
		return "boolean"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "integer"
	case float32, float64:
		return "number"
	case string:
		return "string"
	case []any, []int, []string:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "object"
	}
}

// Tool schema creation functions

// CreateToolSchema creates a tool schema without reflection
func CreateToolSchema(name, description string, inputSchema, outputSchema *JSONSchema) api.ToolSchema {
	return api.ToolSchema{
		Name:         name,
		Description:  description,
		InputSchema:  inputSchema.ToMap(),
		OutputSchema: outputSchema.ToMap(),
		Version:      "1.0.0",
	}
}

// GenerateToolSchema generates a tool schema using static registration
func GenerateToolSchema[TParams any, TResult any](
	name, description string,
	inputBuilder func() *JSONSchema,
	outputBuilder func() *JSONSchema,
) (Schema[TParams, TResult], error) {
	inputSchema := inputBuilder()
	outputSchema := outputBuilder()

	return Schema[TParams, TResult]{
		Name:         name,
		Description:  description,
		Version:      "1.0.0",
		ParamsSchema: inputSchema,
		ResultSchema: outputSchema,
		Examples:     []Example[TParams, TResult]{},
	}, nil
}

// Containerization tool schema builders

// CreateAnalyzeToolSchema creates schema for analyze tool
func CreateAnalyzeToolSchema() *JSONSchema {
	return CreateObjectSchema(map[string]*JSONSchema{
		"repository_path": CreateStringSchema(1, 500, ""),
		"output_format":   CreateEnumSchema([]string{"json", "yaml", "text"}),
		"deep_scan":       BooleanSchemaBuilder().Build(),
	}, []string{"repository_path"})
}

// CreateBuildToolSchema creates schema for build tool
func CreateBuildToolSchema() *JSONSchema {
	return CreateObjectSchema(map[string]*JSONSchema{
		"dockerfile_path": CreateStringSchema(1, 500, ""),
		"image_name":      CreateStringSchema(1, 200, ""),
		"build_context":   CreateStringSchema(1, 500, ""),
		"build_args":      ObjectSchemaBuilder().Build(),
		"no_cache":        BooleanSchemaBuilder().Build(),
	}, []string{"dockerfile_path", "image_name"})
}

// CreateDeployToolSchema creates schema for deploy tool
func CreateDeployToolSchema() *JSONSchema {
	return CreateObjectSchema(map[string]*JSONSchema{
		"image_name":  CreateStringSchema(1, 200, ""),
		"namespace":   CreateStringSchema(1, 100, ""),
		"replicas":    IntegerSchemaBuilder().Build(),
		"port":        IntegerSchemaBuilder().Build(),
		"environment": ObjectSchemaBuilder().Build(),
	}, []string{"image_name"})
}

// CreateScanToolSchema creates schema for scan tool
func CreateScanToolSchema() *JSONSchema {
	return CreateObjectSchema(map[string]*JSONSchema{
		"image_name": CreateStringSchema(1, 200, ""),
		"scanner":    CreateEnumSchema([]string{"trivy", "grype"}),
		"format":     CreateEnumSchema([]string{"json", "table", "sarif"}),
		"severity":   CreateEnumSchema([]string{"LOW", "MEDIUM", "HIGH", "CRITICAL"}),
	}, []string{"image_name"})
}

// Session tool schema builders

// CreateSessionCreateToolSchema creates schema for session create tool
func CreateSessionCreateToolSchema() *JSONSchema {
	return CreateObjectSchema(map[string]*JSONSchema{
		"session_name":   CreateStringSchema(1, 100, ""),
		"workspace_path": CreateStringSchema(1, 500, ""),
		"labels":         ObjectSchemaBuilder().Build(),
	}, []string{"session_name"})
}

// CreateSessionManageToolSchema creates schema for session manage tool
func CreateSessionManageToolSchema() *JSONSchema {
	return CreateObjectSchema(map[string]*JSONSchema{
		"session_id": CreateStringSchema(1, 100, ""),
		"action":     CreateEnumSchema([]string{"get", "update", "delete", "list"}),
		"metadata":   ObjectSchemaBuilder().Build(),
	}, []string{"action"})
}

// Schema registry for static schema management

// SchemaRegistry manages static schemas
type SchemaRegistry struct {
	schemas map[string]*JSONSchema
}

// NewSchemaRegistry creates a new schema registry
func NewSchemaRegistry() *SchemaRegistry {
	registry := &SchemaRegistry{
		schemas: make(map[string]*JSONSchema),
	}

	// Register built-in schemas
	registry.registerBuiltinSchemas()

	return registry
}

// registerBuiltinSchemas registers common tool schemas
func (r *SchemaRegistry) registerBuiltinSchemas() {
	r.schemas["containerization_analyze"] = CreateAnalyzeToolSchema()
	r.schemas["containerization_build"] = CreateBuildToolSchema()
	r.schemas["containerization_deploy"] = CreateDeployToolSchema()
	r.schemas["containerization_scan"] = CreateScanToolSchema()
	r.schemas["session_create"] = CreateSessionCreateToolSchema()
	r.schemas["session_manage"] = CreateSessionManageToolSchema()
}

// GetSchema retrieves a schema by name
func (r *SchemaRegistry) GetSchema(name string) (*JSONSchema, error) {
	if schema, exists := r.schemas[name]; exists {
		return schema, nil
	}

	return nil, errors.NewError().
		Message(fmt.Sprintf("schema not found: %s", name)).
		WithLocation().
		Build()
}

// RegisterSchema registers a custom schema
func (r *SchemaRegistry) RegisterSchema(name string, schema *JSONSchema) {
	r.schemas[name] = schema
}

// ListSchemas returns all registered schema names
func (r *SchemaRegistry) ListSchemas() []string {
	names := make([]string, 0, len(r.schemas))
	for name := range r.schemas {
		names = append(names, name)
	}
	return names
}

// Global schema registry instance
var globalSchemaRegistry = NewSchemaRegistry()

// GetGlobalSchemaRegistry returns the global schema registry
func GetGlobalSchemaRegistry() *SchemaRegistry {
	return globalSchemaRegistry
}

// Utility functions

// GetSchemaAsJSON returns the schema as JSON bytes
func GetSchemaAsJSON(schema *JSONSchema) ([]byte, error) {
	return json.MarshalIndent(schema, "", "  ")
}

// ParseValidationRules parses validation rules from struct tags
func ParseValidationRules(tag string) []string {
	if tag == "" {
		return nil
	}
	return strings.Split(tag, ",")
}

// ApplyValidationRule applies a validation rule to a value
func ApplyValidationRule(value any, fieldName string, rule string) error {
	parts := strings.SplitN(rule, "=", 2)
	ruleName := parts[0]

	switch ruleName {
	case "required":
		if isZeroValue(value) {
			return errors.NewError().
				Message(fmt.Sprintf("field %s is required", fieldName)).
				WithLocation().
				Build()
		}
	case "min":
		if len(parts) > 1 {
			return validateMinConstraint(value, fieldName, parts[1])
		}
	case "max":
		if len(parts) > 1 {
			return validateMaxConstraint(value, fieldName, parts[1])
		}
	}

	return nil
}

// isZeroValue checks if a value is zero
func isZeroValue(value any) bool {
	if value == nil {
		return true
	}

	switch v := value.(type) {
	case string:
		return v == ""
	case bool:
		return !v
	case int, int8, int16, int32, int64:
		return v == 0
	case uint, uint8, uint16, uint32, uint64:
		return v == 0
	case float32, float64:
		return v == 0
	default:
		return false
	}
}

// validateMinConstraint validates minimum constraint
func validateMinConstraint(value any, fieldName string, param string) error {
	switch v := value.(type) {
	case string:
		if len(v) < parseIntParam(param) {
			return errors.NewError().
				Message(fmt.Sprintf("field %s must be at least %s characters", fieldName, param)).
				WithLocation().
				Build()
		}
	case int:
		if v < parseIntParam(param) {
			return errors.NewError().
				Message(fmt.Sprintf("field %s must be at least %s", fieldName, param)).
				WithLocation().
				Build()
		}
	}
	return nil
}

// validateMaxConstraint validates maximum constraint
func validateMaxConstraint(value any, fieldName string, param string) error {
	switch v := value.(type) {
	case string:
		if len(v) > parseIntParam(param) {
			return errors.NewError().
				Message(fmt.Sprintf("field %s must be at most %s characters", fieldName, param)).
				WithLocation().
				Build()
		}
	case int:
		if v > parseIntParam(param) {
			return errors.NewError().
				Message(fmt.Sprintf("field %s must be at most %s", fieldName, param)).
				WithLocation().
				Build()
		}
	}
	return nil
}

// parseIntParam parses integer parameter
func parseIntParam(param string) int {
	var val int
	for _, r := range param {
		if r >= '0' && r <= '9' {
			val = val*10 + int(r-'0')
		} else {
			break
		}
	}
	return val
}

// Schema validation context

// ValidationContext provides context for schema validation
type ValidationContext struct {
	FieldPath   string
	SchemaPath  string
	Errors      []error
	Warnings    []string
	ValidatedAt time.Time
}

// NewValidationContext creates a new validation context
func NewValidationContext() *ValidationContext {
	return &ValidationContext{
		Errors:      []error{},
		Warnings:    []string{},
		ValidatedAt: time.Now(),
	}
}

// AddError adds an error to the context
func (c *ValidationContext) AddError(err error) {
	c.Errors = append(c.Errors, err)
}

// AddWarning adds a warning to the context
func (c *ValidationContext) AddWarning(msg string) {
	c.Warnings = append(c.Warnings, msg)
}

// HasErrors returns true if there are errors
func (c *ValidationContext) HasErrors() bool {
	return len(c.Errors) > 0
}

// CombineErrors combines all errors into a single error
func (c *ValidationContext) CombineErrors() error {
	if len(c.Errors) == 0 {
		return nil
	}

	if len(c.Errors) == 1 {
		return c.Errors[0]
	}

	var messages []string
	for _, err := range c.Errors {
		messages = append(messages, err.Error())
	}

	return errors.NewError().
		Message(fmt.Sprintf("validation errors: %s", strings.Join(messages, "; "))).
		WithLocation().
		Build()
}

// ValidateWithContext validates data against schema with context
func ValidateWithContext(_ context.Context, data any, schema *JSONSchema, validationCtx *ValidationContext) error {
	if schema.Type == "" {
		return nil
	}

	dataType := getJSONType(data)
	if dataType != schema.Type {
		err := errors.NewError().
			Message(fmt.Sprintf("expected type %s, got %s", schema.Type, dataType)).
			WithLocation().
			Build()
		validationCtx.AddError(err)
		return err
	}

	return validationCtx.CombineErrors()
}
