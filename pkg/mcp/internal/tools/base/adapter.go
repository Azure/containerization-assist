package base

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/rs/zerolog"
)

// ModuleAdapter defines the interface for adapting modular components
type ModuleAdapter interface {
	// UseRefactoredModules checks if refactored modules should be used
	UseRefactoredModules() bool

	// ExecuteWithModules executes using refactored modules
	ExecuteWithModules(ctx context.Context, args interface{}) (interface{}, error)

	// ExecuteLegacy executes using legacy code
	ExecuteLegacy(ctx context.Context, args interface{}) (interface{}, error)
}

// BaseAdapter provides common functionality for adapters
type BaseAdapter struct {
	ModuleName   string
	EnvVarName   string
	Logger       zerolog.Logger
	TypeMappings map[string]TypeMapping
}

// TypeMapping defines how to map between types
type TypeMapping struct {
	FromType reflect.Type
	ToType   reflect.Type
	Mapper   func(from interface{}) (interface{}, error)
}

// NewBaseAdapter creates a new base adapter
func NewBaseAdapter(moduleName, envVarName string, logger zerolog.Logger) *BaseAdapter {
	return &BaseAdapter{
		ModuleName:   moduleName,
		EnvVarName:   envVarName,
		Logger:       logger.With().Str("adapter", moduleName).Logger(),
		TypeMappings: make(map[string]TypeMapping),
	}
}

// UseRefactoredModules checks if refactored modules should be used
func (a *BaseAdapter) UseRefactoredModules() bool {
	value := os.Getenv(a.EnvVarName)
	use := value == "true"

	if use {
		a.Logger.Info().
			Str("env_var", a.EnvVarName).
			Str("value", value).
			Msg("Using refactored modules")
	}

	return use
}

// RegisterTypeMapping registers a type mapping
func (a *BaseAdapter) RegisterTypeMapping(name string, mapping TypeMapping) {
	a.TypeMappings[name] = mapping
}

// ConvertType converts between types using registered mappings
func (a *BaseAdapter) ConvertType(name string, from interface{}) (interface{}, error) {
	mapping, exists := a.TypeMappings[name]
	if !exists {
		return nil, fmt.Errorf("no type mapping registered for %s", name)
	}

	// Check type compatibility
	fromType := reflect.TypeOf(from)
	if fromType != mapping.FromType {
		return nil, fmt.Errorf("type mismatch: expected %v, got %v", mapping.FromType, fromType)
	}

	return mapping.Mapper(from)
}

// ResultMerger helps merge results from multiple modules
type ResultMerger struct {
	logger zerolog.Logger
}

// NewResultMerger creates a new result merger
func NewResultMerger(logger zerolog.Logger) *ResultMerger {
	return &ResultMerger{
		logger: logger,
	}
}

// MergeErrors merges error slices
func (m *ResultMerger) MergeErrors(target interface{}, source interface{}) error {
	targetVal := reflect.ValueOf(target)
	sourceVal := reflect.ValueOf(source)

	// Ensure both are slices
	if targetVal.Kind() != reflect.Slice || sourceVal.Kind() != reflect.Slice {
		return fmt.Errorf("both target and source must be slices")
	}

	// Append source elements to target
	for i := 0; i < sourceVal.Len(); i++ {
		targetVal.Set(reflect.Append(targetVal, sourceVal.Index(i)))
	}

	return nil
}

// MergeResults merges two result structures
func (m *ResultMerger) MergeResults(target, source interface{}) error {
	targetVal := reflect.ValueOf(target).Elem()
	sourceVal := reflect.ValueOf(source).Elem()

	if targetVal.Kind() != reflect.Struct || sourceVal.Kind() != reflect.Struct {
		return fmt.Errorf("both target and source must be struct pointers")
	}

	// Merge fields with same name and type
	for i := 0; i < sourceVal.NumField(); i++ {
		sourceField := sourceVal.Type().Field(i)
		sourceFieldVal := sourceVal.Field(i)

		// Find corresponding field in target
		targetField := targetVal.FieldByName(sourceField.Name)
		if !targetField.IsValid() || !targetField.CanSet() {
			continue
		}

		// Check type compatibility
		if targetField.Type() != sourceFieldVal.Type() {
			m.logger.Warn().
				Str("field", sourceField.Name).
				Str("source_type", sourceFieldVal.Type().String()).
				Str("target_type", targetField.Type().String()).
				Msg("Type mismatch, skipping field")
			continue
		}

		// Handle different field types
		switch targetField.Kind() {
		case reflect.Slice:
			// Append slices
			if !sourceFieldVal.IsNil() {
				for j := 0; j < sourceFieldVal.Len(); j++ {
					targetField.Set(reflect.Append(targetField, sourceFieldVal.Index(j)))
				}
			}
		case reflect.Map:
			// Merge maps
			if !sourceFieldVal.IsNil() {
				if targetField.IsNil() {
					targetField.Set(reflect.MakeMap(targetField.Type()))
				}
				for _, key := range sourceFieldVal.MapKeys() {
					targetField.SetMapIndex(key, sourceFieldVal.MapIndex(key))
				}
			}
		case reflect.Int, reflect.Int32, reflect.Int64:
			// Add numeric values
			targetField.SetInt(targetField.Int() + sourceFieldVal.Int())
		case reflect.Bool:
			// OR boolean values
			if sourceFieldVal.Bool() {
				targetField.SetBool(true)
			}
		default:
			// For other types, only set if target is zero value
			if targetField.IsZero() && !sourceFieldVal.IsZero() {
				targetField.Set(sourceFieldVal)
			}
		}
	}

	return nil
}

// ModuleRegistry manages module registration
type ModuleRegistry struct {
	modules map[string]interface{}
	logger  zerolog.Logger
}

// NewModuleRegistry creates a new module registry
func NewModuleRegistry(logger zerolog.Logger) *ModuleRegistry {
	return &ModuleRegistry{
		modules: make(map[string]interface{}),
		logger:  logger,
	}
}

// Register registers a module
func (r *ModuleRegistry) Register(name string, module interface{}) {
	r.modules[name] = module
	r.logger.Debug().
		Str("module", name).
		Str("type", reflect.TypeOf(module).String()).
		Msg("Module registered")
}

// Get retrieves a module
func (r *ModuleRegistry) Get(name string) (interface{}, bool) {
	module, exists := r.modules[name]
	return module, exists
}

// GetTyped retrieves a typed module
func GetTyped[T any](r *ModuleRegistry, name string) (T, error) {
	var zero T

	module, exists := r.modules[name]
	if !exists {
		return zero, fmt.Errorf("module %s not found", name)
	}

	typed, ok := module.(T)
	if !ok {
		return zero, fmt.Errorf("module %s is not of expected type", name)
	}

	return typed, nil
}

// AdapterOptions provides configuration for adapters
type AdapterOptions struct {
	// Enable debug logging
	Debug bool

	// Custom environment variable prefix
	EnvPrefix string

	// Module initialization timeout
	InitTimeout int

	// Custom parameters
	Custom map[string]interface{}
}

// DefaultAdapterOptions returns default adapter options
func DefaultAdapterOptions() AdapterOptions {
	return AdapterOptions{
		Debug:       false,
		EnvPrefix:   "USE_REFACTORED_",
		InitTimeout: 30,
		Custom:      make(map[string]interface{}),
	}
}

// ConversionHelper provides utility functions for type conversion
type ConversionHelper struct{}

// ConvertMap converts map[string]interface{} to a typed struct
func (c *ConversionHelper) ConvertMap(source map[string]interface{}, target interface{}) error {
	targetVal := reflect.ValueOf(target).Elem()
	if targetVal.Kind() != reflect.Struct {
		return fmt.Errorf("target must be a struct pointer")
	}

	for key, value := range source {
		field := targetVal.FieldByNameFunc(func(name string) bool {
			// Case-insensitive field matching
			return strings.EqualFold(name, key)
		})

		if !field.IsValid() || !field.CanSet() {
			continue
		}

		// Convert and set value
		if err := c.setFieldValue(field, value); err != nil {
			return fmt.Errorf("failed to set field %s: %w", key, err)
		}
	}

	return nil
}

// setFieldValue sets a field value with type conversion
func (c *ConversionHelper) setFieldValue(field reflect.Value, value interface{}) error {
	if value == nil {
		return nil
	}

	valueType := reflect.TypeOf(value)
	fieldType := field.Type()

	// Direct assignment if types match
	if valueType == fieldType {
		field.Set(reflect.ValueOf(value))
		return nil
	}

	// Handle common conversions
	switch field.Kind() {
	case reflect.String:
		field.SetString(fmt.Sprintf("%v", value))
	case reflect.Int, reflect.Int32, reflect.Int64:
		switch v := value.(type) {
		case float64:
			field.SetInt(int64(v))
		case int:
			field.SetInt(int64(v))
		default:
			return fmt.Errorf("cannot convert %T to int", value)
		}
	case reflect.Float32, reflect.Float64:
		switch v := value.(type) {
		case float64:
			field.SetFloat(v)
		case int:
			field.SetFloat(float64(v))
		default:
			return fmt.Errorf("cannot convert %T to float", value)
		}
	case reflect.Bool:
		switch v := value.(type) {
		case bool:
			field.SetBool(v)
		case string:
			field.SetBool(v == "true")
		default:
			return fmt.Errorf("cannot convert %T to bool", value)
		}
	case reflect.Slice:
		return c.setSliceValue(field, value)
	case reflect.Map:
		return c.setMapValue(field, value)
	default:
		return fmt.Errorf("unsupported field type: %v", field.Kind())
	}

	return nil
}

// setSliceValue sets a slice field value
func (c *ConversionHelper) setSliceValue(field reflect.Value, value interface{}) error {
	sourceVal := reflect.ValueOf(value)
	if sourceVal.Kind() != reflect.Slice {
		return fmt.Errorf("source value is not a slice")
	}

	// Create new slice
	slice := reflect.MakeSlice(field.Type(), sourceVal.Len(), sourceVal.Len())

	// Copy elements
	for i := 0; i < sourceVal.Len(); i++ {
		if err := c.setFieldValue(slice.Index(i), sourceVal.Index(i).Interface()); err != nil {
			return err
		}
	}

	field.Set(slice)
	return nil
}

// setMapValue sets a map field value
func (c *ConversionHelper) setMapValue(field reflect.Value, value interface{}) error {
	sourceVal := reflect.ValueOf(value)
	if sourceVal.Kind() != reflect.Map {
		return fmt.Errorf("source value is not a map")
	}

	// Create new map
	mapVal := reflect.MakeMap(field.Type())

	// Copy entries
	for _, key := range sourceVal.MapKeys() {
		val := sourceVal.MapIndex(key)
		mapVal.SetMapIndex(key, val)
	}

	field.Set(mapVal)
	return nil
}
