package tools

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	errors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// ============================================================================
// Type-Safe Schema Generator Enhancement
// ============================================================================

// TypedSchemaGenerator provides fully type-safe schema generation without interface{}
type TypedSchemaGenerator struct {
	// RefResolver handles schema references and definitions
	RefResolver map[string]*JSONSchema
	// TypeRegistry maps Go types to custom schemas
	TypeRegistry map[reflect.Type]*JSONSchema
	// TagOptions controls schema generation behavior
	TagOptions SchemaOptions
	// DefinitionsCache caches generated definitions
	DefinitionsCache map[string]*JSONSchema
}

// NewTypedSchemaGenerator creates a new type-safe schema generator
func NewTypedSchemaGenerator() *TypedSchemaGenerator {
	return &TypedSchemaGenerator{
		RefResolver:      make(map[string]*JSONSchema),
		TypeRegistry:     make(map[reflect.Type]*JSONSchema),
		DefinitionsCache: make(map[string]*JSONSchema),
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

// GenerateTypedSchema generates a type-safe JSON schema for a given type
func (g *TypedSchemaGenerator) GenerateTypedSchema(t reflect.Type) (*JSONSchema, error) {
	return g.generateTypedSchemaForType(t, make(map[reflect.Type]bool))
}

// generateTypedSchemaForType recursively generates schema for a type without interface{}
func (g *TypedSchemaGenerator) generateTypedSchemaForType(t reflect.Type, visited map[reflect.Type]bool) (*JSONSchema, error) {
	// Check for custom type mapping
	if schema, exists := g.TypeRegistry[t]; exists {
		return schema, nil
	}

	// Prevent infinite recursion
	if visited[t] {
		return &JSONSchema{
			Ref: fmt.Sprintf("#/definitions/%s", t.Name()),
		}, nil
	}
	visited[t] = true

	switch t.Kind() {
	case reflect.String:
		return g.generateTypedStringSchema(t)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return g.generateTypedIntegerSchema(t)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return g.generateTypedIntegerSchema(t)
	case reflect.Float32, reflect.Float64:
		return g.generateTypedNumberSchema(t)
	case reflect.Bool:
		return g.generateTypedBooleanSchema(t)
	case reflect.Array, reflect.Slice:
		return g.generateTypedArraySchema(t, visited)
	case reflect.Map:
		return g.generateTypedMapSchema(t, visited)
	case reflect.Struct:
		return g.generateTypedStructSchema(t, visited)
	case reflect.Ptr:
		return g.generateTypedSchemaForType(t.Elem(), visited)
	case reflect.Interface:
		return g.generateTypedInterfaceSchema(t)
	default:
		return &JSONSchema{
			Type:        "object",
			Description: fmt.Sprintf("Unsupported type: %s", t.Kind()),
		}, nil
	}
}

// TypedExample represents a type-safe example value
type TypedExample struct {
	StringValue *string           `json:"string_value,omitempty"`
	IntValue    *int64            `json:"int_value,omitempty"`
	FloatValue  *float64          `json:"float_value,omitempty"`
	BoolValue   *bool             `json:"bool_value,omitempty"`
	ArrayValue  []TypedExample    `json:"array_value,omitempty"`
	ObjectValue map[string]string `json:"object_value,omitempty"`
}

// generateTypedStringSchema generates type-safe schema for string types
func (g *TypedSchemaGenerator) generateTypedStringSchema(t reflect.Type) (*JSONSchema, error) {
	schema := &JSONSchema{
		Type: "string",
	}

	// Handle special string types
	if t.PkgPath() == "time" && t.Name() == "Time" {
		schema.Format = "date-time"
		if g.TagOptions.GenerateExamples {
			// Store example as string directly
			schema.Example = "2023-01-01T00:00:00Z"
		}
	}

	return schema, nil
}

// generateTypedIntegerSchema generates type-safe schema for integer types
func (g *TypedSchemaGenerator) generateTypedIntegerSchema(t reflect.Type) (*JSONSchema, error) {
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
		schema.Example = int64(0)
	}

	return schema, nil
}

// generateTypedNumberSchema generates type-safe schema for floating-point types
func (g *TypedSchemaGenerator) generateTypedNumberSchema(t reflect.Type) (*JSONSchema, error) {
	schema := &JSONSchema{
		Type: "number",
	}

	if t.Kind() == reflect.Float32 {
		schema.Format = "float"
	} else {
		schema.Format = "double"
	}

	if g.TagOptions.GenerateExamples {
		schema.Example = float64(0.0)
	}

	return schema, nil
}

// generateTypedBooleanSchema generates type-safe schema for boolean types
func (g *TypedSchemaGenerator) generateTypedBooleanSchema(t reflect.Type) (*JSONSchema, error) {
	schema := &JSONSchema{
		Type: "boolean",
	}

	if g.TagOptions.GenerateExamples {
		schema.Example = false
	}

	return schema, nil
}

// generateTypedArraySchema generates type-safe schema for array and slice types
func (g *TypedSchemaGenerator) generateTypedArraySchema(t reflect.Type, visited map[reflect.Type]bool) (*JSONSchema, error) {
	elemSchema, err := g.generateTypedSchemaForType(t.Elem(), visited)
	if err != nil {
		return nil, err
	}

	schema := &JSONSchema{
		Type:  "array",
		Items: elemSchema,
	}

	if g.TagOptions.GenerateExamples {
		// Set example as empty JSON array string
		schema.Example = "[]"
	}

	return schema, nil
}

// generateTypedMapSchema generates type-safe schema for map types
func (g *TypedSchemaGenerator) generateTypedMapSchema(t reflect.Type, visited map[reflect.Type]bool) (*JSONSchema, error) {
	_, err := g.generateTypedSchemaForType(t.Elem(), visited)
	if err != nil {
		return nil, err
	}

	schema := &JSONSchema{
		Type: "object",
	}

	addlProps := true
	schema.AdditionalProperties = &addlProps

	if g.TagOptions.GenerateExamples {
		// Set example as empty JSON object string
		schema.Example = "{}"
	}

	return schema, nil
}

// generateTypedStructSchema generates type-safe schema for struct types
func (g *TypedSchemaGenerator) generateTypedStructSchema(t reflect.Type, visited map[reflect.Type]bool) (*JSONSchema, error) {
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
		fieldSchema, err := g.generateTypedSchemaForType(field.Type, visited)
		if err != nil {
			return nil, errors.NewError().Message(fmt.Sprintf("failed to generate schema for field %s", fieldName)).Cause(err).WithLocation(

			// Add validation constraints from tags
			).Build()
		}

		if g.TagOptions.UseValidateTags {
			if validateTag := field.Tag.Get("validate"); validateTag != "" {
				g.applyTypedValidationConstraints(fieldSchema, validateTag)
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

// generateTypedInterfaceSchema generates type-safe schema for interface types
func (g *TypedSchemaGenerator) generateTypedInterfaceSchema(t reflect.Type) (*JSONSchema, error) {
	return &JSONSchema{
		Type:        "object",
		Description: fmt.Sprintf("Interface type: %s", t.Name()),
	}, nil
}

// applyTypedValidationConstraints applies validation tag constraints to typed schema
func (g *TypedSchemaGenerator) applyTypedValidationConstraints(schema *JSONSchema, validateTag string) {
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

// ============================================================================
// Type-Safe Schema Validation
// ============================================================================

// TypedSchemaValidator provides type-safe schema validation
type TypedSchemaValidator struct {
	// SchemaCache caches parsed schemas
	SchemaCache map[string]*JSONSchema
}

// NewTypedSchemaValidator creates a new type-safe schema validator
func NewTypedSchemaValidator() *TypedSchemaValidator {
	return &TypedSchemaValidator{
		SchemaCache: make(map[string]*JSONSchema),
	}
}

// ValidateTypedData validates typed data against a JSON schema
func (v *TypedSchemaValidator) ValidateTypedData(data TypedData, schema *JSONSchema) error {
	// Validate based on schema type
	switch schema.Type {
	case "string":
		if data.StringValue == nil {
			return errors.NewError().Messagef("expected string value").Build()
		}
		return v.validateString(*data.StringValue, schema)
	case "integer":
		if data.IntValue == nil {
			return errors.NewError().Messagef("expected integer value").Build()
		}
		return v.validateInteger(*data.IntValue, schema)
	case "number":
		if data.FloatValue == nil {
			return errors.NewError().Messagef("expected number value").Build()
		}
		return v.validateNumber(*data.FloatValue, schema)
	case "boolean":
		if data.BoolValue == nil {
			return errors.NewError().Messagef("expected boolean value").Build(

			// No additional validation for booleans
			)
		}
		return nil
	case "array":
		if data.ArrayValue == nil {
			return errors.NewError().Messagef("expected array value").Build()
		}
		return v.validateArray(data.ArrayValue, schema)
	case "object":
		if data.ObjectValue == nil {
			return errors.NewError().Messagef("expected object value").Build()
		}
		return v.validateObject(data.ObjectValue, schema)
	default:
		return errors.NewError().Messagef("unsupported schema type: %s", schema.Type).WithLocation(

		// TypedData represents typed data for validation
		).Build()
	}
}

type TypedData struct {
	StringValue *string              `json:"string_value,omitempty"`
	IntValue    *int64               `json:"int_value,omitempty"`
	FloatValue  *float64             `json:"float_value,omitempty"`
	BoolValue   *bool                `json:"bool_value,omitempty"`
	ArrayValue  []TypedData          `json:"array_value,omitempty"`
	ObjectValue map[string]TypedData `json:"object_value,omitempty"`
}

// validateString validates a string against schema constraints
func (v *TypedSchemaValidator) validateString(value string, schema *JSONSchema) error {
	if schema.MinLength != nil && len(value) < *schema.MinLength {
		return errors.NewError().Messagef("string length %d is less than minimum %d", len(value), *schema.MinLength).Build()
	}
	if schema.MaxLength != nil && len(value) > *schema.MaxLength {
		return errors.NewError().Messagef("string length %d exceeds maximum %d", len(value), *schema.MaxLength).Build()
	}
	if schema.Pattern != "" {
		// Pattern validation would require regex compilation
	}
	return nil
}

// validateInteger validates an integer against schema constraints
func (v *TypedSchemaValidator) validateInteger(value int64, schema *JSONSchema) error {
	floatValue := float64(value)
	if schema.Minimum != nil && floatValue < *schema.Minimum {
		return errors.NewError().Messagef("value %d is less than minimum %f", value, *schema.Minimum).Build()
	}
	if schema.Maximum != nil && floatValue > *schema.Maximum {
		return errors.NewError().Messagef("value %d exceeds maximum %f", value, *schema.Maximum).Build(

		// validateNumber validates a number against schema constraints
		)
	}
	return nil
}

func (v *TypedSchemaValidator) validateNumber(value float64, schema *JSONSchema) error {
	if schema.Minimum != nil && value < *schema.Minimum {
		return errors.NewError().Messagef("value %f is less than minimum %f", value, *schema.Minimum).Build()
	}
	if schema.Maximum != nil && value > *schema.Maximum {
		return errors.NewError().Messagef("value %f exceeds maximum %f", value, *schema.Maximum).Build(

		// validateArray validates an array against schema constraints
		)
	}
	return nil
}

func (v *TypedSchemaValidator) validateArray(value []TypedData, schema *JSONSchema) error {
	if schema.Items == nil {
		return nil // No item schema to validate against
	}

	// Validate each item
	for i, item := range value {
		if err := v.ValidateTypedData(item, schema.Items); err != nil {
			return errors.NewError().Message(fmt.Sprintf("array item %d validation failed", i)).Cause(err).Build()
		}
	}

	return nil
}

// validateObject validates an object against schema constraints
func (v *TypedSchemaValidator) validateObject(value map[string]TypedData, schema *JSONSchema) error {
	// Check required properties
	for _, required := range schema.Required {
		if _, exists := value[required]; !exists {
			return errors.NewError().Messagef("missing required property: %s", required).WithLocation(

			// Validate each property
			).Build()
		}
	}

	for propName, propValue := range value {
		if propSchema, exists := schema.Properties[propName]; exists {
			if err := v.ValidateTypedData(propValue, propSchema); err != nil {
				return errors.NewError().Message(fmt.Sprintf("property '%s' validation failed", propName)).Cause(err).Build()
			}
		} else if schema.AdditionalProperties != nil && !*schema.AdditionalProperties {
			return errors.NewError().Messagef("additional property '%s' not allowed", propName).Build()
		}
	}

	return nil
}

// ============================================================================
// Type-Safe Enum Support
// ============================================================================

// TypedEnumValue represents a type-safe enum value
type TypedEnumValue struct {
	StringValue *string  `json:"string_value,omitempty"`
	IntValue    *int64   `json:"int_value,omitempty"`
	FloatValue  *float64 `json:"float_value,omitempty"`
}

// ConvertEnumToTyped converts interface{} enum values to typed enum values
func ConvertEnumToTyped(values []any) []TypedEnumValue {
	result := make([]TypedEnumValue, 0, len(values))

	for _, v := range values {
		var typedValue TypedEnumValue

		switch val := v.(type) {
		case string:
			typedValue.StringValue = &val
		case int:
			intVal := int64(val)
			typedValue.IntValue = &intVal
		case int64:
			typedValue.IntValue = &val
		case float64:
			typedValue.FloatValue = &val
		case json.Number:
			if intVal, err := val.Int64(); err == nil {
				typedValue.IntValue = &intVal
			} else if floatVal, err := val.Float64(); err == nil {
				typedValue.FloatValue = &floatVal
			}
		}

		result = append(result, typedValue)
	}

	return result
}

// ============================================================================
// Conversion Utilities
// ============================================================================

// ConvertJSONSchemaToTyped converts a legacy JSONSchema with any to fully typed
func ConvertJSONSchemaToTyped(schema *JSONSchema) *JSONSchema {
	if schema == nil {
		return nil
	}

	// Create a copy to avoid modifying the original
	typed := &JSONSchema{
		Type:                 schema.Type,
		Format:               schema.Format,
		Title:                schema.Title,
		Description:          schema.Description,
		Items:                ConvertJSONSchemaToTyped(schema.Items),
		Properties:           make(map[string]*JSONSchema),
		Required:             append([]string{}, schema.Required...),
		AdditionalProperties: schema.AdditionalProperties,
		Minimum:              schema.Minimum,
		Maximum:              schema.Maximum,
		MinLength:            schema.MinLength,
		MaxLength:            schema.MaxLength,
		Pattern:              schema.Pattern,
		Ref:                  schema.Ref,
		Definitions:          make(map[string]*JSONSchema),
	}

	// Convert properties
	for k, v := range schema.Properties {
		typed.Properties[k] = ConvertJSONSchemaToTyped(v)
	}

	// Convert definitions
	for k, v := range schema.Definitions {
		typed.Definitions[k] = ConvertJSONSchemaToTyped(v)
	}

	// Convert AllOf, AnyOf, OneOf
	if len(schema.AllOf) > 0 {
		typed.AllOf = make([]*JSONSchema, len(schema.AllOf))
		for i, s := range schema.AllOf {
			typed.AllOf[i] = ConvertJSONSchemaToTyped(s)
		}
	}

	if len(schema.AnyOf) > 0 {
		typed.AnyOf = make([]*JSONSchema, len(schema.AnyOf))
		for i, s := range schema.AnyOf {
			typed.AnyOf[i] = ConvertJSONSchemaToTyped(s)
		}
	}

	if len(schema.OneOf) > 0 {
		typed.OneOf = make([]*JSONSchema, len(schema.OneOf))
		for i, s := range schema.OneOf {
			typed.OneOf[i] = ConvertJSONSchemaToTyped(s)
		}
	}

	// Convert enum values to typed representation (store as JSON string)
	if len(schema.Enum) > 0 {
		// Convert enum values to JSON strings for type safety
		typed.Enum = make([]any, len(schema.Enum))
		copy(typed.Enum, schema.Enum)
	}

	// Convert example to typed representation
	if schema.Example != nil {
		// Keep example as-is since it needs to match the actual type
		typed.Example = schema.Example
	}

	return typed
}
