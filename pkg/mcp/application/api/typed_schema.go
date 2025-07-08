package api

import (
	"encoding/json"
	"reflect"
)

// TypedSchema represents a strongly typed JSON schema
type TypedSchema struct {
	// Type is the JSON schema type (object, string, number, etc.)
	Type string `json:"type"`

	// Title is a human-readable title
	Title string `json:"title,omitempty"`

	// Description explains the schema
	Description string `json:"description,omitempty"`

	// Properties defines object properties
	Properties map[string]PropertySchema `json:"properties,omitempty"`

	// Required lists required property names
	Required []string `json:"required,omitempty"`

	// Items defines array item schema
	Items *TypedSchema `json:"items,omitempty"`

	// AdditionalProperties controls extra properties
	AdditionalProperties bool `json:"additionalProperties,omitempty"`

	// Enum lists allowed values
	Enum []json.RawMessage `json:"enum,omitempty"`

	// Minimum value for numbers
	Minimum *float64 `json:"minimum,omitempty"`

	// Maximum value for numbers
	Maximum *float64 `json:"maximum,omitempty"`

	// MinLength for strings
	MinLength *int `json:"minLength,omitempty"`

	// MaxLength for strings
	MaxLength *int `json:"maxLength,omitempty"`

	// Pattern for string validation
	Pattern string `json:"pattern,omitempty"`

	// Format specifies the data format
	Format string `json:"format,omitempty"`

	// Default value
	Default json.RawMessage `json:"default,omitempty"`
}

// PropertySchema defines a single property in an object schema
type PropertySchema struct {
	TypedSchema

	// ReadOnly indicates if the property is read-only
	ReadOnly bool `json:"readOnly,omitempty"`

	// WriteOnly indicates if the property is write-only
	WriteOnly bool `json:"writeOnly,omitempty"`
}

// SchemaBuilder helps build typed schemas
type SchemaBuilder struct {
	schema TypedSchema
}

// NewSchemaBuilder creates a new schema builder
func NewSchemaBuilder() *SchemaBuilder {
	return &SchemaBuilder{
		schema: TypedSchema{
			Type:                 "object",
			Properties:           make(map[string]PropertySchema),
			AdditionalProperties: false,
		},
	}
}

// WithTitle sets the schema title
func (b *SchemaBuilder) WithTitle(title string) *SchemaBuilder {
	b.schema.Title = title
	return b
}

// WithDescription sets the schema description
func (b *SchemaBuilder) WithDescription(desc string) *SchemaBuilder {
	b.schema.Description = desc
	return b
}

// AddProperty adds a property to the schema
func (b *SchemaBuilder) AddProperty(name string, prop PropertySchema) *SchemaBuilder {
	b.schema.Properties[name] = prop
	return b
}

// AddRequiredProperty adds a required property
func (b *SchemaBuilder) AddRequiredProperty(name string, prop PropertySchema) *SchemaBuilder {
	b.schema.Properties[name] = prop
	b.schema.Required = append(b.schema.Required, name)
	return b
}

// Build returns the built schema
func (b *SchemaBuilder) Build() TypedSchema {
	return b.schema
}

// ============================================================================
// Schema Generation from Go Types
// ============================================================================

// GenerateSchema generates a TypedSchema from a Go type using reflection
func GenerateSchema(t reflect.Type) TypedSchema {
	return generateSchemaRecursive(t, make(map[reflect.Type]bool))
}

func generateSchemaRecursive(t reflect.Type, visited map[reflect.Type]bool) TypedSchema {
	// Handle pointers
	if t.Kind() == reflect.Ptr {
		return generateSchemaRecursive(t.Elem(), visited)
	}

	// Prevent infinite recursion
	if visited[t] {
		return TypedSchema{Type: "object", Description: "Circular reference to " + t.String()}
	}
	visited[t] = true

	switch t.Kind() {
	case reflect.String:
		return TypedSchema{Type: "string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return TypedSchema{Type: "integer"}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return TypedSchema{Type: "integer", Minimum: float64Ptr(0)}
	case reflect.Float32, reflect.Float64:
		return TypedSchema{Type: "number"}
	case reflect.Bool:
		return TypedSchema{Type: "boolean"}
	case reflect.Slice, reflect.Array:
		itemSchema := generateSchemaRecursive(t.Elem(), visited)
		return TypedSchema{
			Type:  "array",
			Items: &itemSchema,
		}
	case reflect.Map:
		return TypedSchema{
			Type:                 "object",
			AdditionalProperties: true,
		}
	case reflect.Struct:
		return generateStructSchema(t, visited)
	default:
		return TypedSchema{Type: "string", Description: "Unknown type: " + t.String()}
	}
}

func generateStructSchema(t reflect.Type, visited map[reflect.Type]bool) TypedSchema {
	schema := TypedSchema{
		Type:                 "object",
		Properties:           make(map[string]PropertySchema),
		AdditionalProperties: false,
	}

	// Handle time.Time specially
	if t.String() == "time.Time" {
		return TypedSchema{Type: "string", Format: "date-time"}
	}

	// Handle time.Duration specially
	if t.String() == "time.Duration" {
		return TypedSchema{Type: "string", Format: "duration"}
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		// Get JSON tag
		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}

		fieldName := field.Name
		omitempty := false
		if jsonTag != "" {
			parts := splitJSONTag(jsonTag)
			if parts[0] != "" {
				fieldName = parts[0]
			}
			for _, part := range parts[1:] {
				if part == "omitempty" {
					omitempty = true
				}
			}
		}

		// Generate schema for field
		fieldSchema := generateSchemaRecursive(field.Type, visited)
		prop := PropertySchema{TypedSchema: fieldSchema}

		// Add to properties
		schema.Properties[fieldName] = prop

		// Add to required if not omitempty
		if !omitempty {
			schema.Required = append(schema.Required, fieldName)
		}
	}

	return schema
}

// Helper functions

func float64Ptr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}

func splitJSONTag(tag string) []string {
	var parts []string
	current := ""
	for _, r := range tag {
		if r == ',' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(r)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// ============================================================================
// Typed Tool Schema
// ============================================================================

// TypedToolSchemaV2 replaces the map[string]interface{} based ToolSchema
type TypedToolSchemaV2 struct {
	// Name is the tool's unique identifier
	Name string `json:"name"`

	// Description explains what the tool does
	Description string `json:"description"`

	// Version indicates the tool's version
	Version string `json:"version"`

	// InputSchema defines the typed schema for input validation
	InputSchema TypedSchema `json:"input_schema"`

	// OutputSchema defines the typed schema for output structure
	OutputSchema TypedSchema `json:"output_schema"`

	// Examples provides usage examples
	Examples []TypedToolExample `json:"examples,omitempty"`

	// Tags categorizes the tool
	Tags []string `json:"tags,omitempty"`

	// Category groups related tools
	Category ToolCategory `json:"category,omitempty"`
}

// TypedToolExample demonstrates tool usage with typed data
type TypedToolExample struct {
	// Name identifies this example
	Name string `json:"name"`

	// Description explains what this example demonstrates
	Description string `json:"description"`

	// Input shows example input data as JSON
	Input json.RawMessage `json:"input"`

	// Output shows expected output as JSON
	Output json.RawMessage `json:"output"`
}

// GenerateToolSchema creates a TypedToolSchemaV2 from input and output types
func GenerateToolSchema[TInput, TOutput any](name, description, version string) TypedToolSchemaV2 {
	var input TInput
	var output TOutput

	return TypedToolSchemaV2{
		Name:         name,
		Description:  description,
		Version:      version,
		InputSchema:  GenerateSchema(reflect.TypeOf(input)),
		OutputSchema: GenerateSchema(reflect.TypeOf(output)),
		Tags:         []string{},
		Category:     CategoryUtility,
	}
}
