package tools

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// MigrationHelper provides utilities for migrating from legacy interfaces to generic types
type MigrationHelper struct {
	TypeRegistry map[string]reflect.Type
	Converters   map[string]TypeConverter
}

// TypeConverter converts between interface{} and specific types
type TypeConverter func(interface{}) (interface{}, error)

// NewMigrationHelper creates a new migration helper
func NewMigrationHelper() *MigrationHelper {
	helper := &MigrationHelper{
		TypeRegistry: make(map[string]reflect.Type),
		Converters:   make(map[string]TypeConverter),
	}

	// Register common type conversions
	helper.registerCommonConverters()

	return helper
}

// RegisterType registers a type for migration
func (m *MigrationHelper) RegisterType(name string, t reflect.Type, converter TypeConverter) {
	m.TypeRegistry[name] = t
	m.Converters[name] = converter
}

// ConvertFromInterface converts interface{} to a specific type
func (m *MigrationHelper) ConvertFromInterface(data interface{}, targetTypeName string) (interface{}, error) {
	converter, exists := m.Converters[targetTypeName]
	if !exists {
		return nil, fmt.Errorf("no converter registered for type: %s", targetTypeName)
	}

	return converter(data)
}

// ConvertToGenericParams converts legacy parameters to generic parameters
func ConvertToGenericParams[TParams ToolParams](data interface{}) (TParams, error) {
	var zero TParams

	// Try direct type assertion first
	if typed, ok := data.(TParams); ok {
		return typed, nil
	}

	// Try conversion from map[string]interface{}
	if mapData, ok := data.(map[string]interface{}); ok {
		return convertMapToStruct[TParams](mapData)
	}

	return zero, fmt.Errorf("cannot convert %T to %T", data, zero)
}

// ConvertToGenericResult converts legacy results to generic results
func ConvertToGenericResult[TResult ToolResult](data interface{}) (TResult, error) {
	var zero TResult

	// Try direct type assertion first
	if typed, ok := data.(TResult); ok {
		return typed, nil
	}

	// Try conversion from map[string]interface{}
	if mapData, ok := data.(map[string]interface{}); ok {
		return convertMapToStruct[TResult](mapData)
	}

	return zero, fmt.Errorf("cannot convert %T to %T", data, zero)
}

// convertMapToStruct converts a map to a struct using reflection
func convertMapToStruct[T any](data map[string]interface{}) (T, error) {
	var result T
	resultValue := reflect.ValueOf(&result).Elem()
	resultType := resultValue.Type()

	for i := 0; i < resultType.NumField(); i++ {
		field := resultType.Field(i)
		fieldValue := resultValue.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		// Get field name from JSON tag or field name
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" && parts[0] != "-" {
				fieldName = parts[0]
			}
		}

		// Get value from map
		mapValue, exists := data[fieldName]
		if !exists {
			continue
		}

		// Convert and set the value
		if err := setFieldValue(fieldValue, mapValue); err != nil {
			return result, fmt.Errorf("failed to set field %s: %w", fieldName, err)
		}
	}

	return result, nil
}

// setFieldValue sets a reflect.Value from an interface{} value
func setFieldValue(fieldValue reflect.Value, value interface{}) error {
	if value == nil {
		return nil
	}

	valueReflect := reflect.ValueOf(value)
	fieldType := fieldValue.Type()

	// Handle direct assignment if types match
	if valueReflect.Type().AssignableTo(fieldType) {
		fieldValue.Set(valueReflect)
		return nil
	}

	// Handle conversions
	switch fieldType.Kind() {
	case reflect.String:
		str, err := convertToString(value)
		if err != nil {
			return err
		}
		fieldValue.SetString(str)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := convertToInt64(value)
		if err != nil {
			return err
		}
		fieldValue.SetInt(i)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := convertToUint64(value)
		if err != nil {
			return err
		}
		fieldValue.SetUint(u)

	case reflect.Float32, reflect.Float64:
		f, err := convertToFloat64(value)
		if err != nil {
			return err
		}
		fieldValue.SetFloat(f)

	case reflect.Bool:
		b, err := convertToBool(value)
		if err != nil {
			return err
		}
		fieldValue.SetBool(b)

	case reflect.Slice:
		slice, err := convertToSlice(value, fieldType.Elem())
		if err != nil {
			return err
		}
		fieldValue.Set(slice)

	case reflect.Map:
		mapVal, err := convertToMap(value, fieldType.Key(), fieldType.Elem())
		if err != nil {
			return err
		}
		fieldValue.Set(mapVal)

	case reflect.Ptr:
		if valueReflect.Kind() == reflect.Ptr {
			fieldValue.Set(valueReflect)
		} else {
			// Create new pointer and set the value
			newPtr := reflect.New(fieldType.Elem())
			if err := setFieldValue(newPtr.Elem(), value); err != nil {
				return err
			}
			fieldValue.Set(newPtr)
		}

	case reflect.Struct:
		if mapData, ok := value.(map[string]interface{}); ok {
			structVal, err := convertMapToStructReflect(mapData, fieldType)
			if err != nil {
				return err
			}
			fieldValue.Set(structVal)
		} else {
			return fmt.Errorf("cannot convert %T to struct", value)
		}

	default:
		return fmt.Errorf("unsupported field type: %s", fieldType.Kind())
	}

	return nil
}

// convertMapToStructReflect converts a map to a struct using reflection
func convertMapToStructReflect(data map[string]interface{}, structType reflect.Type) (reflect.Value, error) {
	structValue := reflect.New(structType).Elem()

	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		fieldValue := structValue.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		// Get field name from JSON tag or field name
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" && parts[0] != "-" {
				fieldName = parts[0]
			}
		}

		// Get value from map
		mapValue, exists := data[fieldName]
		if !exists {
			continue
		}

		// Convert and set the value
		if err := setFieldValue(fieldValue, mapValue); err != nil {
			return structValue, fmt.Errorf("failed to set field %s: %w", fieldName, err)
		}
	}

	return structValue, nil
}

// Type conversion utility functions

func convertToString(value interface{}) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v), nil
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v), nil
	case float32, float64:
		return fmt.Sprintf("%g", v), nil
	case bool:
		return strconv.FormatBool(v), nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

func convertToInt64(value interface{}) (int64, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case uint:
		return int64(v), nil
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint64:
		return int64(v), nil
	case float32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", value)
	}
}

func convertToUint64(value interface{}) (uint64, error) {
	switch v := value.(type) {
	case int:
		return uint64(v), nil
	case int8:
		return uint64(v), nil
	case int16:
		return uint64(v), nil
	case int32:
		return uint64(v), nil
	case int64:
		return uint64(v), nil
	case uint:
		return uint64(v), nil
	case uint8:
		return uint64(v), nil
	case uint16:
		return uint64(v), nil
	case uint32:
		return uint64(v), nil
	case uint64:
		return v, nil
	case float32:
		return uint64(v), nil
	case float64:
		return uint64(v), nil
	case string:
		return strconv.ParseUint(v, 10, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to uint64", value)
	}
}

func convertToFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", value)
	}
}

func convertToBool(value interface{}) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(v)
	case int, int8, int16, int32, int64:
		return v != 0, nil
	case uint, uint8, uint16, uint32, uint64:
		return v != 0, nil
	default:
		return false, fmt.Errorf("cannot convert %T to bool", value)
	}
}

func convertToSlice(value interface{}, elemType reflect.Type) (reflect.Value, error) {
	valueReflect := reflect.ValueOf(value)
	if valueReflect.Kind() != reflect.Slice && valueReflect.Kind() != reflect.Array {
		return reflect.Value{}, fmt.Errorf("expected slice or array, got %T", value)
	}

	sliceType := reflect.SliceOf(elemType)
	result := reflect.MakeSlice(sliceType, valueReflect.Len(), valueReflect.Len())

	for i := 0; i < valueReflect.Len(); i++ {
		elem := valueReflect.Index(i)
		resultElem := result.Index(i)

		if err := setFieldValue(resultElem, elem.Interface()); err != nil {
			return reflect.Value{}, fmt.Errorf("failed to convert slice element %d: %w", i, err)
		}
	}

	return result, nil
}

func convertToMap(value interface{}, keyType, valueType reflect.Type) (reflect.Value, error) {
	valueReflect := reflect.ValueOf(value)
	if valueReflect.Kind() != reflect.Map {
		return reflect.Value{}, fmt.Errorf("expected map, got %T", value)
	}

	mapType := reflect.MapOf(keyType, valueType)
	result := reflect.MakeMap(mapType)

	for _, key := range valueReflect.MapKeys() {
		mapValue := valueReflect.MapIndex(key)

		// Convert key
		newKey := reflect.New(keyType).Elem()
		if err := setFieldValue(newKey, key.Interface()); err != nil {
			return reflect.Value{}, fmt.Errorf("failed to convert map key: %w", err)
		}

		// Convert value
		newValue := reflect.New(valueType).Elem()
		if err := setFieldValue(newValue, mapValue.Interface()); err != nil {
			return reflect.Value{}, fmt.Errorf("failed to convert map value: %w", err)
		}

		result.SetMapIndex(newKey, newValue)
	}

	return result, nil
}

// registerCommonConverters registers converters for common types
func (m *MigrationHelper) registerCommonConverters() {
	// String converter
	m.RegisterType("string", reflect.TypeOf(""), func(data interface{}) (interface{}, error) {
		return convertToString(data)
	})

	// Int converter
	m.RegisterType("int", reflect.TypeOf(0), func(data interface{}) (interface{}, error) {
		i64, err := convertToInt64(data)
		return int(i64), err
	})

	// Float64 converter
	m.RegisterType("float64", reflect.TypeOf(0.0), func(data interface{}) (interface{}, error) {
		return convertToFloat64(data)
	})

	// Bool converter
	m.RegisterType("bool", reflect.TypeOf(false), func(data interface{}) (interface{}, error) {
		return convertToBool(data)
	})

	// Map[string]interface{} converter (pass-through)
	m.RegisterType("map[string]interface{}", reflect.TypeOf(map[string]interface{}{}), func(data interface{}) (interface{}, error) {
		if mapData, ok := data.(map[string]interface{}); ok {
			return mapData, nil
		}
		return nil, fmt.Errorf("expected map[string]interface{}, got %T", data)
	})
}

// Legacy error conversion utilities

// ConvertStandardErrorToRich converts a standard error to a RichError
func ConvertStandardErrorToRich(err error) interface{} {
	if err == nil {
		return nil
	}

	// This would import from the rich package in a real implementation
	// For now, return a map structure that could be converted
	return map[string]interface{}{
		"code":      "LEGACY_ERROR",
		"message":   err.Error(),
		"type":      "INTERNAL",
		"severity":  "MEDIUM",
		"timestamp": "now", // Would use actual timestamp
	}
}

// BackwardCompatibilityLayer provides compatibility for old interfaces
type BackwardCompatibilityLayer struct {
	LegacyRegistry map[string]interface{}
	NewRegistry    interface{} // Would be typed as Registry[T, TParams, TResult] in real use
}

// NewBackwardCompatibilityLayer creates a new compatibility layer
func NewBackwardCompatibilityLayer() *BackwardCompatibilityLayer {
	return &BackwardCompatibilityLayer{
		LegacyRegistry: make(map[string]interface{}),
	}
}

// RegisterLegacyTool registers a legacy tool
func (b *BackwardCompatibilityLayer) RegisterLegacyTool(name string, tool interface{}) {
	b.LegacyRegistry[name] = tool
}

// ExecuteLegacyTool executes a legacy tool with interface{} parameters
func (b *BackwardCompatibilityLayer) ExecuteLegacyTool(name string, params interface{}) (interface{}, error) {
	tool, exists := b.LegacyRegistry[name]
	if !exists {
		return nil, fmt.Errorf("legacy tool not found: %s", name)
	}

	// This would need to be implemented based on the legacy tool interface
	_ = tool
	return nil, fmt.Errorf("legacy tool execution not implemented")
}

// MigrateLegacyRegistry migrates a legacy registry to a new generic registry
func MigrateLegacyRegistry(legacyRegistry map[string]interface{}) error {
	// This would implement the actual migration logic
	// For each tool in the legacy registry:
	// 1. Determine the appropriate generic types
	// 2. Create a wrapper that converts between interface{} and generic types
	// 3. Register the wrapper in the new registry

	for name, tool := range legacyRegistry {
		_ = name
		_ = tool
		// Implementation would go here
	}

	return nil
}
