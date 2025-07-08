package core

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	errors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// SchemaValue represents a type that can be used in JSON schemas
type SchemaValue interface {
	~string | ~int | ~int64 | ~float64 | ~bool
}

// JSONSchema represents a typed JSON Schema structure with generic support
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
	Enum                 []any                  `json:"enum,omitempty"`    // Schema-compatible enum values
	Example              any                    `json:"example,omitempty"` // Schema-compatible example value
	Ref                  string                 `json:"$ref,omitempty"`
	Definitions          map[string]*JSONSchema `json:"definitions,omitempty"`
	AllOf                []*JSONSchema          `json:"allOf,omitempty"`
	AnyOf                []*JSONSchema          `json:"anyOf,omitempty"`
	OneOf                []*JSONSchema          `json:"oneOf,omitempty"`
}

// TypedJSONSchema provides type-safe schema with generic constraints
type TypedJSONSchema[T SchemaValue] struct {
	*JSONSchema
	TypedEnum    []T `json:"-"` // Type-safe enum values
	TypedExample T   `json:"-"` // Type-safe example value
}

// NewTypedSchema creates a type-safe schema
func NewTypedSchema[T SchemaValue]() *TypedJSONSchema[T] {
	return &TypedJSONSchema[T]{
		JSONSchema: &JSONSchema{},
	}
}

// SetEnum sets type-safe enum values
func (ts *TypedJSONSchema[T]) SetEnum(values []T) {
	ts.TypedEnum = values
	// Convert to any for JSON serialization
	ts.Enum = make([]any, len(values))
	for i, v := range values {
		ts.Enum[i] = v
	}
}

// SetExample sets a type-safe example value
func (ts *TypedJSONSchema[T]) SetExample(example T) {
	ts.TypedExample = example
	ts.Example = example
}

// ToMap converts JSONSchema to map[string]any for backward compatibility
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

// ToMapLegacy provides legacy interface{} support for backward compatibility
// Deprecated: Use ToMap instead
func (s *JSONSchema) ToMapLegacy() map[string]any {
	return s.ToMap()
}

// FromMapLegacy creates JSONSchema from map[string]interface{} for backward compatibility
// Deprecated: Use FromMap instead
func FromMapLegacy(m map[string]any) *JSONSchema {
	return FromMap(m)
}

// SchemaGenerator generates JSON schemas from Go types
type SchemaGenerator struct {
	// RefResolver handles schema references and definitions (bounded)
	RefResolver map[string]*JSONSchema
	// TypeRegistry maps Go types to custom schemas (bounded)
	TypeRegistry map[reflect.Type]*JSONSchema
	// TagOptions controls schema generation behavior
	TagOptions SchemaOptions
	// Cache limits to prevent memory leaks
	maxRefResolverSize  int
	maxTypeRegistrySize int
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
		RefResolver:         make(map[string]*JSONSchema),
		TypeRegistry:        make(map[reflect.Type]*JSONSchema),
		maxRefResolverSize:  1000, // Prevent unbounded growth
		maxTypeRegistrySize: 500,  // Prevent unbounded growth
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
func (g *SchemaGenerator) GenerateSchema(t reflect.Type) (*JSONSchema, error) {
	return g.generateSchemaForType(t, make(map[reflect.Type]bool))
}

// GenerateSchemaAsMap generates a JSON schema as map[string]any for backward compatibility
func (g *SchemaGenerator) GenerateSchemaAsMap(t reflect.Type) (map[string]any, error) {
	schema, err := g.GenerateSchema(t)
	if err != nil {
		return nil, err
	}
	return schema.ToMap(), nil
}

// GenerateSchemaAsMapLegacy generates a JSON schema as map[string]interface{} for legacy compatibility
// Deprecated: Use GenerateSchemaAsMap instead
func (g *SchemaGenerator) GenerateSchemaAsMapLegacy(t reflect.Type) (map[string]any, error) {
	schema, err := g.GenerateSchema(t)
	if err != nil {
		return nil, err
	}
	return schema.ToMap(), nil
}

// generateSchemaForType recursively generates schema for a type
func (g *SchemaGenerator) generateSchemaForType(t reflect.Type, visited map[reflect.Type]bool) (*JSONSchema, error) {
	// Check for custom type mapping
	if schema, exists := g.TypeRegistry[t]; exists {
		return schema, nil
	}

	// Prevent infinite recursion
	if visited[t] {
		refKey := fmt.Sprintf("#/definitions/%s", t.Name())
		// Check if we already have this in RefResolver, otherwise add it
		if _, exists := g.RefResolver[refKey]; !exists {
			// Create a placeholder schema
			g.addToRefResolver(refKey, &JSONSchema{
				Type:        "object",
				Description: fmt.Sprintf("Reference to %s", t.Name()),
			})
		}
		return &JSONSchema{
			Ref: refKey,
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
		return &JSONSchema{
			Type:        "object",
			Description: fmt.Sprintf("Unsupported type: %s", t.Kind()),
		}, nil
	}
}

// generateStringSchema generates schema for string types
func (g *SchemaGenerator) generateStringSchema(t reflect.Type) (*JSONSchema, error) {
	schema := &JSONSchema{
		Type: "string",
	}

	// Handle special string types
	if t == reflect.TypeOf(time.Time{}) {
		schema.Format = "date-time"
		if g.TagOptions.GenerateExamples {
			schema.Example = "2023-01-01T00:00:00Z"
		}
	}

	return schema, nil
}

// generateIntegerSchema generates schema for integer types
func (g *SchemaGenerator) generateIntegerSchema(t reflect.Type) (*JSONSchema, error) {
	schema := &JSONSchema{
		Type: "integer",
	}

	// Add format based on type size
	switch t.Kind() {
	case reflect.Int32, reflect.Uint32:
		schema.Format = "int32"
	case reflect.Int64, reflect.Uint64:
		schema.Format = "int64"
	}

	// Add minimum for unsigned types
	if strings.HasPrefix(t.Kind().String(), "uint") {
		min := float64(0)
		schema.Minimum = &min
	}

	if g.TagOptions.GenerateExamples {
		schema.Example = 0
	}

	return schema, nil
}

// generateNumberSchema generates schema for floating-point types
func (g *SchemaGenerator) generateNumberSchema(t reflect.Type) (*JSONSchema, error) {
	schema := &JSONSchema{
		Type: "number",
	}

	if t.Kind() == reflect.Float32 {
		schema.Format = "float"
	} else {
		schema.Format = "double"
	}

	if g.TagOptions.GenerateExamples {
		schema.Example = 0.0
	}

	return schema, nil
}

// generateBooleanSchema generates schema for boolean types
func (g *SchemaGenerator) generateBooleanSchema(t reflect.Type) (*JSONSchema, error) {
	schema := &JSONSchema{
		Type: "boolean",
	}

	if g.TagOptions.GenerateExamples {
		schema.Example = false
	}

	return schema, nil
}

// generateArraySchema generates schema for array and slice types
func (g *SchemaGenerator) generateArraySchema(t reflect.Type, visited map[reflect.Type]bool) (*JSONSchema, error) {
	elemSchema, err := g.generateSchemaForType(t.Elem(), visited)
	if err != nil {
		return nil, err
	}

	schema := &JSONSchema{
		Type:  "array",
		Items: elemSchema,
	}

	if g.TagOptions.GenerateExamples {
		schema.Example = []string{} // Type-safe empty array example
	}

	return schema, nil
}

// generateMapSchema generates schema for map types
func (g *SchemaGenerator) generateMapSchema(t reflect.Type, visited map[reflect.Type]bool) (*JSONSchema, error) {
	_, err := g.generateSchemaForType(t.Elem(), visited)
	if err != nil {
		return nil, err
	}

	// For maps, we use a special representation where additionalProperties holds the value schema
	// Since JSONSchema struct doesn't have a field for this, we'll store as a schema that allows any additional properties
	schema := &JSONSchema{
		Type: "object",
	}

	// Create a copy of the value schema to use as additional properties
	addlProps := true
	schema.AdditionalProperties = &addlProps

	if g.TagOptions.GenerateExamples {
		schema.Example = map[string]any{} // Schema-compatible empty map example
	}

	return schema, nil
}

// generateStructSchema generates schema for struct types
func (g *SchemaGenerator) generateStructSchema(t reflect.Type, visited map[reflect.Type]bool) (*JSONSchema, error) {
	schema := &JSONSchema{
		Type:       "object",
		Properties: make(map[string]*JSONSchema),
	}

	var required []string

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
			return nil, errors.NewError().Message(fmt.Sprintf("failed to generate schema for field %s", fieldName)).Cause(err).WithLocation(

			// Add validation constraints from tags
			).Build()
		}

		if g.TagOptions.UseValidateTags {
			if validateTag := field.Tag.Get("validate"); validateTag != "" {
				g.applyValidationConstraintsTyped(fieldSchema, validateTag)
			}
		}

		// Add description from comment or tag
		if desc := field.Tag.Get("description"); desc != "" {
			fieldSchema.Description = desc
		}

		schema.Properties[fieldName] = fieldSchema
	}

	if len(required) > 0 {
		schema.Required = required
	}

	schema.AdditionalProperties = &g.TagOptions.AllowAdditional

	return schema, nil
}

// generateInterfaceSchema generates schema for interface types
func (g *SchemaGenerator) generateInterfaceSchema(t reflect.Type) (*JSONSchema, error) {
	return &JSONSchema{
		Type:        "object",
		Description: fmt.Sprintf("Interface type: %s", t.Name()),
	}, nil
}

// applyValidationConstraints applies validation tag constraints to schema (legacy)
// Deprecated: Use applyValidationConstraintsTyped instead
func (g *SchemaGenerator) applyValidationConstraints(schema map[string]any, validateTag string) {
	g.applyValidationConstraintsToMap(schema, validateTag)
}

// applyValidationConstraintsTyped applies validation tag constraints to typed schema
func (g *SchemaGenerator) applyValidationConstraintsTyped(schema *JSONSchema, validateTag string) {
	constraints := strings.Split(validateTag, ",")
	for _, constraint := range constraints {
		constraint = strings.TrimSpace(constraint)

		switch {
		case constraint == "required":
			// Handled at struct level
		case strings.HasPrefix(constraint, "min="):
			if value := strings.TrimPrefix(constraint, "min="); value != "" {
				if num, err := parseNumberFloat(value); err == nil {
					schema.Minimum = &num
				}
			}
		case strings.HasPrefix(constraint, "max="):
			if value := strings.TrimPrefix(constraint, "max="); value != "" {
				if num, err := parseNumberFloat(value); err == nil {
					schema.Maximum = &num
				}
			}
		case strings.HasPrefix(constraint, "len="):
			if value := strings.TrimPrefix(constraint, "len="); value != "" {
				if num64, err := parseNumberInt(value); err == nil {
					num := int(num64)
					if schema.Type == "string" {
						schema.MinLength = &num
						schema.MaxLength = &num
					}
				}
			}
		case strings.HasPrefix(constraint, "email"):
			schema.Format = "email"
		case strings.HasPrefix(constraint, "url"):
			schema.Format = "uri"
		case strings.HasPrefix(constraint, "uuid"):
			schema.Pattern = "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"
		}
	}
}

// applyValidationConstraintsToMap applies validation constraints to a map (for backward compatibility)
func (g *SchemaGenerator) applyValidationConstraintsToMap(schemaMap map[string]any, validateTag string) {

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
					intVal := int(num)
					if schemaMap["type"] == "string" {
						schemaMap["minLength"] = intVal
						schemaMap["maxLength"] = intVal
					} else if schemaMap["type"] == "array" {
						schemaMap["minItems"] = intVal
						schemaMap["maxItems"] = intVal
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
func parseNumber(s string) (float64, error) {
	if f, err := json.Number(s).Float64(); err == nil {
		return f, nil
	}
	return 0, errors.NewError().Messagef("not a number: %s", s).WithLocation().Build()
}

// parseNumberFloat parses a string as float64
func parseNumberFloat(s string) (float64, error) {
	return json.Number(s).Float64()
}

// parseNumberInt parses a string as int64
func parseNumberInt(s string) (int64, error) {
	return json.Number(s).Int64()
}

func GenerateToolSchema[TParams any, TResult any](
	name, description string,
	paramsType reflect.Type,
	resultType reflect.Type,
) (Schema[TParams, TResult], error) {
	generator := NewSchemaGenerator()

	paramsSchema, err := generator.GenerateSchema(paramsType)
	if err != nil {
		return Schema[TParams, TResult]{}, errors.NewError().Message("failed to generate params schema").Cause(err).Build()
	}

	resultSchema, err := generator.GenerateSchema(resultType)
	if err != nil {
		return Schema[TParams, TResult]{}, errors.NewError().Message("failed to generate result schema").Cause(err).Build()
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
func (g *SchemaGenerator) RegisterCustomType(t reflect.Type, schema *JSONSchema) {
	// Evict oldest entries if cache is full
	if len(g.TypeRegistry) >= g.maxTypeRegistrySize {
		g.evictOldestTypeRegistryEntries()
	}
	g.TypeRegistry[t] = schema
}

// evictOldestTypeRegistryEntries removes some entries when the cache is full
func (g *SchemaGenerator) evictOldestTypeRegistryEntries() {
	// Remove 20% of entries to make room for new ones
	evictCount := g.maxTypeRegistrySize / 5
	count := 0
	for key := range g.TypeRegistry {
		delete(g.TypeRegistry, key)
		count++
		if count >= evictCount {
			break
		}
	}
}

// evictOldestRefResolverEntries removes some entries when the cache is full
func (g *SchemaGenerator) evictOldestRefResolverEntries() {
	// Remove 20% of entries to make room for new ones
	evictCount := g.maxRefResolverSize / 5
	count := 0
	for key := range g.RefResolver {
		delete(g.RefResolver, key)
		count++
		if count >= evictCount {
			break
		}
	}
}

// addToRefResolver safely adds an entry to RefResolver with cache eviction
func (g *SchemaGenerator) addToRefResolver(key string, schema *JSONSchema) {
	// Evict oldest entries if cache is full
	if len(g.RefResolver) >= g.maxRefResolverSize {
		g.evictOldestRefResolverEntries()
	}
	g.RefResolver[key] = schema
}

// ClearCaches clears all internal caches to free memory
func (g *SchemaGenerator) ClearCaches() {
	g.RefResolver = make(map[string]*JSONSchema)
	g.TypeRegistry = make(map[reflect.Type]*JSONSchema)
}

// GetCacheStats returns cache statistics
func (g *SchemaGenerator) GetCacheStats() map[string]int {
	return map[string]int{
		"ref_resolver_size":  len(g.RefResolver),
		"ref_resolver_max":   g.maxRefResolverSize,
		"type_registry_size": len(g.TypeRegistry),
		"type_registry_max":  g.maxTypeRegistrySize,
	}
}

// GetSchemaAsJSON returns the schema as JSON bytes
func GetSchemaAsJSON(schema *JSONSchema) ([]byte, error) {
	return json.MarshalIndent(schema, "", "  ")
}

// ValidateAgainstSchema validates data against a JSON schema (simplified)
func ValidateAgainstSchema(data any, schema *JSONSchema) error {
	// This is a simplified validation - in production, use a proper JSON Schema validator
	if schema.Type == "" {
		return nil // No type constraint
	}

	dataType := getJSONType(data)
	if dataType != schema.Type {
		return errors.NewError().Messagef("expected type %s, got %s", schema.Type, dataType).Build()
	}

	return nil
}

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

// getJSONTypeLegacy provides backward compatibility for interface{} parameters
// Deprecated: Use getJSONType instead
func getJSONTypeLegacy(v any) string {
	return getJSONType(v)
}

// Common schema templates for frequently used types

// StringSchema creates a string schema with common constraints
func StringSchema(minLen, maxLen int, pattern string) map[string]any {
	schema := map[string]any{
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

// StringSchemaLegacy provides backward compatibility for interface{} returns
// Deprecated: Use StringSchema instead
func StringSchemaLegacy(minLen, maxLen int, pattern string) map[string]any {
	return StringSchema(minLen, maxLen, pattern)
}

// StringSchemaTyped creates a type-safe string schema with length and pattern constraints
func StringSchemaTyped(minLen, maxLen int, pattern string) *TypedJSONSchema[string] {
	schema := NewTypedSchema[string]()
	schema.Type = "string"
	if minLen > 0 {
		schema.MinLength = &minLen
	}
	if maxLen > 0 {
		schema.MaxLength = &maxLen
	}
	if pattern != "" {
		schema.Pattern = pattern
	}
	return schema
}

// NumberSchema creates a number schema with constraints
func NumberSchema(min, max *float64, multipleOf *float64) map[string]any {
	schema := map[string]any{
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

// NumberSchemaLegacy provides backward compatibility for interface{} returns
// Deprecated: Use NumberSchema instead
func NumberSchemaLegacy(min, max *float64, multipleOf *float64) map[string]any {
	return NumberSchema(min, max, multipleOf)
}

// NumberSchemaTyped creates a type-safe number schema with constraints
func NumberSchemaTyped(min, max *float64, multipleOf *float64) *TypedJSONSchema[float64] {
	schema := NewTypedSchema[float64]()
	schema.Type = "number"
	if min != nil {
		schema.Minimum = min
	}
	if max != nil {
		schema.Maximum = max
	}
	// Note: multipleOf not yet supported in TypedJSONSchema, would need to add field
	return schema
}

// ArraySchema creates an array schema
func ArraySchema(itemSchema any, minItems, maxItems int) map[string]any {
	schema := map[string]any{
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

// ArraySchemaLegacy provides backward compatibility for interface{} parameters and returns
// Deprecated: Use ArraySchema instead
func ArraySchemaLegacy(itemSchema any, minItems, maxItems int) map[string]any {
	return ArraySchema(itemSchema, minItems, maxItems)
}

// ArraySchemaTyped creates a type-safe array schema
func ArraySchemaTyped[T SchemaValue](itemSchema *TypedJSONSchema[T], minItems, maxItems int) *TypedJSONSchema[[]any] {
	schema := NewTypedSchema[[]any]()
	schema.Type = "array"
	schema.Items = itemSchema.JSONSchema
	// Note: minItems and maxItems not yet supported in TypedJSONSchema, would need to add fields
	return schema
}

// EnumSchema creates an enum schema
func EnumSchema(values []any) map[string]any {
	return map[string]any{
		"enum": values,
	}
}

// EnumSchemaLegacy provides backward compatibility for interface{} parameters and returns
// Deprecated: Use EnumSchema instead
func EnumSchemaLegacy(values []any) map[string]any {
	return EnumSchema(values)
}

// EnumSchemaTyped creates a type-safe enum schema for string values
func EnumSchemaTyped(values []string) *TypedJSONSchema[string] {
	schema := NewTypedSchema[string]()
	schema.Type = "string"
	schema.SetEnum(values)
	return schema
}

// ObjectSchema creates an object schema
func ObjectSchema(properties map[string]any, required []string) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

// ObjectSchemaLegacy provides backward compatibility for interface{} parameters and returns
// Deprecated: Use ObjectSchema instead
func ObjectSchemaLegacy(properties map[string]any, required []string) map[string]any {
	return ObjectSchema(properties, required)
}

// TypedSchemaProperty represents a typed schema property
type TypedSchemaProperty = JSONSchema

// ObjectSchemaTyped creates a type-safe object schema
func ObjectSchemaTyped(properties map[string]*TypedSchemaProperty, required []string) *TypedJSONSchema[map[string]any] {
	schema := NewTypedSchema[map[string]any]()
	schema.Type = "object"
	schema.Properties = properties
	schema.Required = required
	return schema
}

// Helper functions for parsing numeric constraints removed - using existing implementations above

// Predefined typed schemas for common tool inputs

// ChatInputSchema provides typed schema for chat input
var ChatInputSchema = ObjectSchemaTyped(map[string]*TypedSchemaProperty{
	"message": {
		Type:        "string",
		Description: "Your message or question",
	},
	"session_id": {
		Type:        "string",
		Description: "Optional session ID for conversation continuity",
	},
	"context": {
		Type:        "object",
		Description: "Additional context for the conversation",
	},
}, []string{"message"})

// ConversationHistoryInputSchema provides typed schema for conversation history input
var ConversationHistoryInputSchema = ObjectSchemaTyped(map[string]*TypedSchemaProperty{
	"session_id": {
		Type:        "string",
		Description: "Session ID to get history for",
	},
	"limit": {
		Type:        "integer",
		Description: "Maximum number of entries to return",
	},
}, []string{})

// WorkflowExecuteInputSchema provides typed schema for workflow execution input
var WorkflowExecuteInputSchema = ObjectSchemaTyped(map[string]*TypedSchemaProperty{
	"workflow_name": {
		Type:        "string",
		Description: "Name of predefined workflow to execute",
	},
	"workflow_spec": {
		Type:        "object",
		Description: "Custom workflow specification",
	},
	"variables": {
		Type:        "object",
		Description: "Variables to pass to the workflow",
	},
	"options": {
		Type:        "object",
		Description: "Execution options (dry_run, checkpoints, etc.)",
	},
}, []string{})

// WorkflowListInputSchema provides typed schema for workflow list input
var WorkflowListInputSchema = ObjectSchemaTyped(map[string]*TypedSchemaProperty{
	"category": {
		Type:        "string",
		Description: "Filter by workflow category",
	},
}, []string{})

// WorkflowStatusInputSchema provides typed schema for workflow status input
var WorkflowStatusInputSchema = ObjectSchemaTyped(map[string]*TypedSchemaProperty{
	"workflow_id": {
		Type:        "string",
		Description: "ID of the workflow to check",
	},
	"session_id": {
		Type:        "string",
		Description: "Optional session ID",
	},
}, []string{"workflow_id"})
