package processing

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/common"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/rs/zerolog"
)

// SchemaProcessor provides JSON schema processing and validation utilities
type SchemaProcessor struct {
	logger     zerolog.Logger
	config     SchemaConfig
	validators map[string]SchemaValidator
	mutex      sync.RWMutex
}

// NewSchemaProcessor creates a new schema processor
func NewSchemaProcessor(config SchemaConfig, logger zerolog.Logger) *SchemaProcessor {
	return &SchemaProcessor{
		logger:     logger.With().Str("component", "schema_processor").Logger(),
		config:     config,
		validators: make(map[string]SchemaValidator),
	}
}

// NewDataProcessingDataProcessingValidationResult creates a new data processing validation result
func NewDataProcessingDataProcessingValidationResult() *DataProcessingValidationResult {
	return &DataProcessingValidationResult{
		Valid:    true,
		Errors:   []common.ValidationError{},
		Warnings: []common.ValidationWarning{},
		Data: DataProcessingValidationData{
			Sanitized:   false,
			Transformed: false,
		},
		Context:  make(map[string]string),
		Duration: 0,
	}
}

// ValidateAgainstSchema validates data against a JSON schema
func (sp *SchemaProcessor) ValidateAgainstSchema(data interface{}, schema map[string]interface{}) (*DataProcessingValidationResult, error) {
	startTime := time.Now()

	result := NewDataProcessingDataProcessingValidationResult()
	result.Data.Metadata = &TypedJSONData{StringFields: make(map[string]string)}

	// Convert data to JSON for processing
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeTypeConversionFailed).
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Message("Failed to marshal data for schema validation").
			Cause(err).
			Suggestion("Ensure data is JSON-serializable").
			WithLocation().
			Build()
	}

	result.Data.OriginalSize = len(jsonData)

	// Parse as generic interface
	var parsedData interface{}
	if err := json.Unmarshal(jsonData, &parsedData); err != nil {
		return nil, errors.NewError().
			Code(errors.CodeTypeConversionFailed).
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Message("Failed to parse JSON data").
			Cause(err).
			Suggestion("Ensure data is valid JSON").
			WithLocation().
			Build()
	}

	// Perform basic validation
	if err := sp.validateBasicTypes(parsedData, schema, result); err != nil {
		return nil, err
	}

	result.Data.ProcessingTime = time.Since(startTime)
	result.Data.FinalSize = len(jsonData)
	result.Duration = time.Since(startTime)

	return result, nil
}

// ProcessTypedSchema processes and optimizes a typed JSON schema
func (sp *SchemaProcessor) ProcessTypedSchema(schema *TypedJSONData) (*TypedJSONData, error) {
	processed, err := sp.ProcessSchema(schema.ToMap())
	if err != nil {
		return nil, err
	}
	return FromMap(processed), nil
}

// ProcessSchema processes and optimizes a JSON schema (legacy interface{})
func (sp *SchemaProcessor) ProcessSchema(schema map[string]interface{}) (map[string]interface{}, error) {
	processed := make(map[string]interface{})

	// Deep copy the schema
	for key, value := range schema {
		processed[key] = value
	}

	// Add default values if not present
	if processed["type"] == nil {
		processed["type"] = "object"
	}

	if processed["additionalProperties"] == nil {
		processed["additionalProperties"] = sp.config.AllowAdditional
	}

	// Validate schema structure
	if err := sp.validateSchemaStructure(processed); err != nil {
		return nil, err
	}

	return processed, nil
}

// RemoveCopilotIncompatible removes schema keywords that GitHub Copilot's AJV validator cannot handle
func RemoveCopilotIncompatible(schema map[string]interface{}) {
	// Remove unsupported keywords that cause AJV Draft-7 validation failures
	delete(schema, "version")
	delete(schema, "$schema")
	delete(schema, "id")
	delete(schema, "$id")

	// Recursively clean nested properties
	if properties, ok := schema["properties"].(map[string]interface{}); ok {
		for _, prop := range properties {
			if propMap, ok := prop.(map[string]interface{}); ok {
				RemoveCopilotIncompatible(propMap)
			}
		}
	}

	// Clean items schema for arrays
	if items, ok := schema["items"].(map[string]interface{}); ok {
		RemoveCopilotIncompatible(items)
	}

	// Clean additionalProperties if it's an object
	if additionalProps, ok := schema["additionalProperties"].(map[string]interface{}); ok {
		RemoveCopilotIncompatible(additionalProps)
	}
}

// CacheSchema caches a schema for reuse
func (sp *SchemaProcessor) CacheSchema(id string, schema map[string]interface{}) error {
	if !sp.config.EnableCaching {
		return nil
	}

	sp.mutex.Lock()
	defer sp.mutex.Unlock()

	// Check cache size limit
	if len(sp.validators) >= sp.config.CacheSize {
		// Remove least recently used
		sp.evictLRU()
	}

	sp.validators[id] = SchemaValidator{
		Schema:   FromMap(schema),
		Compiled: true,
		LastUsed: time.Now(),
		UseCount: 0,
	}

	return nil
}

// validateBasicTypes performs basic type validation
func (sp *SchemaProcessor) validateBasicTypes(data interface{}, schema map[string]interface{}, result *DataProcessingValidationResult) error {
	// Simple type checking
	expectedType, hasType := schema["type"]
	if !hasType {
		return nil
	}

	dataType := getJSONType(data)
	expectedTypeStr, ok := expectedType.(string)
	if !ok {
		return nil
	}

	if dataType != expectedTypeStr {
		result.AddError(
			"TYPE_MISMATCH",
			fmt.Sprintf("Expected type %s, got %s", expectedTypeStr, dataType),
			map[string]string{
				"expected_type": expectedTypeStr,
				"actual_type":   dataType,
			},
		)
	}

	return nil
}

// validateSchemaStructure validates the schema structure itself
func (sp *SchemaProcessor) validateSchemaStructure(schema map[string]interface{}) error {
	// Check for required schema fields
	if _, hasType := schema["type"]; !hasType && sp.config.StrictMode {
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Message("Schema missing required 'type' field").
			Suggestion("Add a 'type' field to the schema").
			WithLocation().
			Build()
	}

	return nil
}

// evictLRU removes the least recently used schema from cache
func (sp *SchemaProcessor) evictLRU() {
	var oldestID string
	var oldestTime time.Time
	firstEntry := true

	for id, validator := range sp.validators {
		if firstEntry || validator.LastUsed.Before(oldestTime) {
			oldestTime = validator.LastUsed
			oldestID = id
			firstEntry = false
		}
	}

	if oldestID != "" {
		delete(sp.validators, oldestID)
	}
}

// getJSONType returns the JSON type of a value
func getJSONType(value interface{}) string {
	switch value.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64, int, int32, int64:
		return "number"
	case string:
		return "string"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		return "unknown"
	}
}
