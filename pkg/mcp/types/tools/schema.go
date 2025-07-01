package tools

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// SchemaGenerator generates JSON schemas from Go types
type SchemaGenerator struct {
	// RefResolver handles schema references and definitions
	RefResolver map[string]interface{}
	// TypeRegistry maps Go types to custom schemas
	TypeRegistry map[reflect.Type]interface{}
	// TagOptions controls schema generation behavior
	TagOptions SchemaOptions
}

// SchemaOptions configures schema generation
type SchemaOptions struct {
	// UseJSONTags uses json struct tags for property names
	UseJSONTags bool
	// UseValidateTags incorporates validate tags as constraints
	UseValidateTags bool
	// RequiredByDefault makes all fields required unless marked optional
	RequiredByDefault bool
	// OmitEmptyFields excludes fields marked with omitempty
	OmitEmptyFields bool
	// GenerateExamples includes example values in schemas
	GenerateExamples bool
	// AllowAdditional allows additional properties in objects
	AllowAdditional bool
}

// NewSchemaGenerator creates a new schema generator with default options
func NewSchemaGenerator() *SchemaGenerator {
	return &SchemaGenerator{
		RefResolver:  make(map[string]interface{}),
		TypeRegistry: make(map[reflect.Type]interface{}),
		TagOptions: SchemaOptions{
			UseJSONTags:       true,
			UseValidateTags:   true,
			RequiredByDefault: false,
			OmitEmptyFields:   true,
			GenerateExamples:  true,
			AllowAdditional:   false,
		},
	}
}

// GenerateSchema generates a JSON schema for a given type
func (g *SchemaGenerator) GenerateSchema(t reflect.Type) (interface{}, error) {
	return g.generateSchemaForType(t, make(map[reflect.Type]bool))
}

// generateSchemaForType recursively generates schema for a type
func (g *SchemaGenerator) generateSchemaForType(t reflect.Type, visited map[reflect.Type]bool) (interface{}, error) {
	// Check for custom type mapping
	if schema, exists := g.TypeRegistry[t]; exists {
		return schema, nil
	}

	// Prevent infinite recursion
	if visited[t] {
		return map[string]interface{}{
			"$ref": fmt.Sprintf("#/definitions/%s", t.Name()),
		}, nil
	}
	visited[t] = true

	switch t.Kind() {
	case reflect.String:
		return g.generateStringSchema(t)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return g.generateIntegerSchema(t)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return g.generateIntegerSchema(t)
	case reflect.Float32, reflect.Float64:
		return g.generateNumberSchema(t)
	case reflect.Bool:
		return g.generateBooleanSchema(t)
	case reflect.Array, reflect.Slice:
		return g.generateArraySchema(t, visited)
	case reflect.Map:
		return g.generateMapSchema(t, visited)
	case reflect.Struct:
		return g.generateStructSchema(t, visited)
	case reflect.Ptr:
		return g.generateSchemaForType(t.Elem(), visited)
	case reflect.Interface:
		return g.generateInterfaceSchema(t)
	default:
		return map[string]interface{}{
			"type":        "object",
			"description": fmt.Sprintf("Unsupported type: %s", t.Kind()),
		}, nil
	}
}

// generateStringSchema generates schema for string types
func (g *SchemaGenerator) generateStringSchema(t reflect.Type) (interface{}, error) {
	schema := map[string]interface{}{
		"type": "string",
	}

	// Handle special string types
	if t == reflect.TypeOf(time.Time{}) {
		schema["format"] = "date-time"
		if g.TagOptions.GenerateExamples {
			schema["example"] = "2023-01-01T00:00:00Z"
		}
	}

	return schema, nil
}

// generateIntegerSchema generates schema for integer types
func (g *SchemaGenerator) generateIntegerSchema(t reflect.Type) (interface{}, error) {
	schema := map[string]interface{}{
		"type": "integer",
	}

	// Add format based on type size
	switch t.Kind() {
	case reflect.Int32, reflect.Uint32:
		schema["format"] = "int32"
	case reflect.Int64, reflect.Uint64:
		schema["format"] = "int64"
	}

	// Add minimum for unsigned types
	if strings.HasPrefix(t.Kind().String(), "uint") {
		schema["minimum"] = 0
	}

	if g.TagOptions.GenerateExamples {
		schema["example"] = 0
	}

	return schema, nil
}

// generateNumberSchema generates schema for floating-point types
func (g *SchemaGenerator) generateNumberSchema(t reflect.Type) (interface{}, error) {
	schema := map[string]interface{}{
		"type": "number",
	}

	if t.Kind() == reflect.Float32 {
		schema["format"] = "float"
	} else {
		schema["format"] = "double"
	}

	if g.TagOptions.GenerateExamples {
		schema["example"] = 0.0
	}

	return schema, nil
}

// generateBooleanSchema generates schema for boolean types
func (g *SchemaGenerator) generateBooleanSchema(t reflect.Type) (interface{}, error) {
	schema := map[string]interface{}{
		"type": "boolean",
	}

	if g.TagOptions.GenerateExamples {
		schema["example"] = false
	}

	return schema, nil
}

// generateArraySchema generates schema for array and slice types
func (g *SchemaGenerator) generateArraySchema(t reflect.Type, visited map[reflect.Type]bool) (interface{}, error) {
	elemSchema, err := g.generateSchemaForType(t.Elem(), visited)
	if err != nil {
		return nil, err
	}

	schema := map[string]interface{}{
		"type":  "array",
		"items": elemSchema,
	}

	if g.TagOptions.GenerateExamples {
		schema["example"] = []interface{}{}
	}

	return schema, nil
}

// generateMapSchema generates schema for map types
func (g *SchemaGenerator) generateMapSchema(t reflect.Type, visited map[reflect.Type]bool) (interface{}, error) {
	valueSchema, err := g.generateSchemaForType(t.Elem(), visited)
	if err != nil {
		return nil, err
	}

	schema := map[string]interface{}{
		"type":                 "object",
		"additionalProperties": valueSchema,
	}

	if g.TagOptions.GenerateExamples {
		schema["example"] = map[string]interface{}{}
	}

	return schema, nil
}

// generateStructSchema generates schema for struct types
func (g *SchemaGenerator) generateStructSchema(t reflect.Type, visited map[reflect.Type]bool) (interface{}, error) {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": make(map[string]interface{}),
	}

	var required []string
	properties := schema["properties"].(map[string]interface{})

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get field name from JSON tag or field name
		fieldName := field.Name
		if g.TagOptions.UseJSONTags {
			if jsonTag := field.Tag.Get("json"); jsonTag != "" {
				parts := strings.Split(jsonTag, ",")
				if parts[0] != "" && parts[0] != "-" {
					fieldName = parts[0]
				}

				// Skip fields marked with json:"-"
				if parts[0] == "-" {
					continue
				}

				// Handle omitempty
				if g.TagOptions.OmitEmptyFields {
					for _, part := range parts[1:] {
						if part == "omitempty" {
							// Field is optional
							goto generateFieldSchema
						}
					}
				}
			}
		}

		// Check if field is required
		if g.TagOptions.RequiredByDefault {
			required = append(required, fieldName)
		}

		// Parse validation tags
		if g.TagOptions.UseValidateTags {
			if validateTag := field.Tag.Get("validate"); validateTag != "" {
				if strings.Contains(validateTag, "required") {
					required = append(required, fieldName)
				}
			}
		}

	generateFieldSchema:
		// Generate schema for field type
		fieldSchema, err := g.generateSchemaForType(field.Type, visited)
		if err != nil {
			return nil, fmt.Errorf("failed to generate schema for field %s: %w", fieldName, err)
		}

		// Add validation constraints from tags
		if g.TagOptions.UseValidateTags {
			if validateTag := field.Tag.Get("validate"); validateTag != "" {
				g.applyValidationConstraints(fieldSchema, validateTag)
			}
		}

		// Add description from comment or tag
		if desc := field.Tag.Get("description"); desc != "" {
			if schemaMap, ok := fieldSchema.(map[string]interface{}); ok {
				schemaMap["description"] = desc
			}
		}

		properties[fieldName] = fieldSchema
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	schema["additionalProperties"] = g.TagOptions.AllowAdditional

	return schema, nil
}

// generateInterfaceSchema generates schema for interface types
func (g *SchemaGenerator) generateInterfaceSchema(t reflect.Type) (interface{}, error) {
	return map[string]interface{}{
		"type":        "object",
		"description": fmt.Sprintf("Interface type: %s", t.Name()),
	}, nil
}

// applyValidationConstraints applies validation tag constraints to schema
func (g *SchemaGenerator) applyValidationConstraints(schema interface{}, validateTag string) {
	schemaMap, ok := schema.(map[string]interface{})
	if !ok {
		return
	}

	constraints := strings.Split(validateTag, ",")
	for _, constraint := range constraints {
		constraint = strings.TrimSpace(constraint)

		switch {
		case constraint == "required":
			// Handled at struct level
		case strings.HasPrefix(constraint, "min="):
			if value := strings.TrimPrefix(constraint, "min="); value != "" {
				if num, err := parseNumber(value); err == nil {
					schemaMap["minimum"] = num
				}
			}
		case strings.HasPrefix(constraint, "max="):
			if value := strings.TrimPrefix(constraint, "max="); value != "" {
				if num, err := parseNumber(value); err == nil {
					schemaMap["maximum"] = num
				}
			}
		case strings.HasPrefix(constraint, "len="):
			if value := strings.TrimPrefix(constraint, "len="); value != "" {
				if num, err := parseNumber(value); err == nil {
					if schemaMap["type"] == "string" {
						schemaMap["minLength"] = num
						schemaMap["maxLength"] = num
					} else if schemaMap["type"] == "array" {
						schemaMap["minItems"] = num
						schemaMap["maxItems"] = num
					}
				}
			}
		case strings.HasPrefix(constraint, "email"):
			schemaMap["format"] = "email"
		case strings.HasPrefix(constraint, "url"):
			schemaMap["format"] = "uri"
		case strings.HasPrefix(constraint, "uuid"):
			schemaMap["pattern"] = "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"
		}
	}
}

// parseNumber attempts to parse a string as a number
func parseNumber(s string) (interface{}, error) {
	if i, err := json.Number(s).Int64(); err == nil {
		return i, nil
	}
	if f, err := json.Number(s).Float64(); err == nil {
		return f, nil
	}
	return nil, fmt.Errorf("not a number: %s", s)
}

// GenerateToolSchema generates a complete schema for a tool
func GenerateToolSchema[TParams ToolParams, TResult ToolResult](
	name, description string,
	paramsType reflect.Type,
	resultType reflect.Type,
) (Schema[TParams, TResult], error) {
	generator := NewSchemaGenerator()

	paramsSchema, err := generator.GenerateSchema(paramsType)
	if err != nil {
		return Schema[TParams, TResult]{}, fmt.Errorf("failed to generate params schema: %w", err)
	}

	resultSchema, err := generator.GenerateSchema(resultType)
	if err != nil {
		return Schema[TParams, TResult]{}, fmt.Errorf("failed to generate result schema: %w", err)
	}

	return Schema[TParams, TResult]{
		Name:         name,
		Description:  description,
		Version:      "1.0.0",
		ParamsSchema: paramsSchema,
		ResultSchema: resultSchema,
		Examples:     []Example[TParams, TResult]{},
	}, nil
}

// RegisterCustomType registers a custom schema for a specific type
func (g *SchemaGenerator) RegisterCustomType(t reflect.Type, schema interface{}) {
	g.TypeRegistry[t] = schema
}

// GetSchemaAsJSON returns the schema as JSON bytes
func GetSchemaAsJSON(schema interface{}) ([]byte, error) {
	return json.MarshalIndent(schema, "", "  ")
}

// ValidateAgainstSchema validates data against a JSON schema (simplified)
func ValidateAgainstSchema(data interface{}, schema interface{}) error {
	// This is a simplified validation - in production, use a proper JSON Schema validator
	schemaMap, ok := schema.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid schema format")
	}

	expectedType, ok := schemaMap["type"].(string)
	if !ok {
		return nil // No type constraint
	}

	dataType := getJSONType(data)
	if dataType != expectedType {
		return fmt.Errorf("expected type %s, got %s", expectedType, dataType)
	}

	return nil
}

// getJSONType returns the JSON type of a value
func getJSONType(v interface{}) string {
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
	case []interface{}, []int, []string: // etc.
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		return "object"
	}
}

// Common schema templates for frequently used types

// StringSchema creates a string schema with common constraints
func StringSchema(minLen, maxLen int, pattern string) map[string]interface{} {
	schema := map[string]interface{}{
		"type": "string",
	}

	if minLen > 0 {
		schema["minLength"] = minLen
	}
	if maxLen > 0 {
		schema["maxLength"] = maxLen
	}
	if pattern != "" {
		schema["pattern"] = pattern
	}

	return schema
}

// NumberSchema creates a number schema with constraints
func NumberSchema(min, max *float64, multipleOf *float64) map[string]interface{} {
	schema := map[string]interface{}{
		"type": "number",
	}

	if min != nil {
		schema["minimum"] = *min
	}
	if max != nil {
		schema["maximum"] = *max
	}
	if multipleOf != nil {
		schema["multipleOf"] = *multipleOf
	}

	return schema
}

// ArraySchema creates an array schema
func ArraySchema(itemSchema interface{}, minItems, maxItems int) map[string]interface{} {
	schema := map[string]interface{}{
		"type":  "array",
		"items": itemSchema,
	}

	if minItems > 0 {
		schema["minItems"] = minItems
	}
	if maxItems > 0 {
		schema["maxItems"] = maxItems
	}

	return schema
}

// EnumSchema creates an enum schema
func EnumSchema(values []interface{}) map[string]interface{} {
	return map[string]interface{}{
		"enum": values,
	}
}

// ObjectSchema creates an object schema
func ObjectSchema(properties map[string]interface{}, required []string) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}
