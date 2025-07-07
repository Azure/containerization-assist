package processing

import (
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/common"
)

// TypedJSONData represents strongly typed JSON data structure
type TypedJSONData struct {
	StringFields  map[string]string        `json:"string_fields,omitempty"`
	NumberFields  map[string]float64       `json:"number_fields,omitempty"`
	BooleanFields map[string]bool          `json:"boolean_fields,omitempty"`
	ArrayFields   map[string][]string      `json:"array_fields,omitempty"`
	ObjectFields  map[string]TypedJSONData `json:"object_fields,omitempty"`
	RawData       map[string]interface{}   `json:"raw_data,omitempty"` // Fallback for unknown structures
}

// ToMap converts TypedJSONData to map[string]interface{} for backward compatibility
func (t *TypedJSONData) ToMap() map[string]interface{} {
	result := make(map[string]interface{})

	for k, v := range t.StringFields {
		result[k] = v
	}
	for k, v := range t.NumberFields {
		result[k] = v
	}
	for k, v := range t.BooleanFields {
		result[k] = v
	}
	for k, v := range t.ArrayFields {
		result[k] = v
	}
	for k, v := range t.ObjectFields {
		result[k] = v.ToMap()
	}
	for k, v := range t.RawData {
		result[k] = v
	}

	return result
}

// FromMap creates TypedJSONData from map[string]interface{}
func FromMap(data map[string]interface{}) *TypedJSONData {
	tjd := &TypedJSONData{
		StringFields:  make(map[string]string),
		NumberFields:  make(map[string]float64),
		BooleanFields: make(map[string]bool),
		ArrayFields:   make(map[string][]string),
		ObjectFields:  make(map[string]TypedJSONData),
		RawData:       make(map[string]interface{}),
	}

	for k, v := range data {
		switch val := v.(type) {
		case string:
			tjd.StringFields[k] = val
		case float64:
			tjd.NumberFields[k] = val
		case bool:
			tjd.BooleanFields[k] = val
		case []string:
			tjd.ArrayFields[k] = val
		case map[string]interface{}:
			tjd.ObjectFields[k] = *FromMap(val)
		default:
			tjd.RawData[k] = v
		}
	}

	return tjd
}

// DataProcessingValidationData represents domain-specific data for data processing validation
type DataProcessingValidationData struct {
	Sanitized      bool           `json:"sanitized"`
	Transformed    bool           `json:"transformed"`
	OriginalSize   int            `json:"original_size"`
	FinalSize      int            `json:"final_size"`
	ProcessingTime time.Duration  `json:"processing_time"`
	Metadata       *TypedJSONData `json:"metadata,omitempty"`
}

// DataProcessingValidationResult is an alias to the unified validation framework
type DataProcessingValidationResult = common.ValidationResult[DataProcessingValidationData]

// DataProcessingValidationError represents a validation error
type DataProcessingValidationError struct {
	Field       string   `json:"field"`
	Message     string   `json:"message"`
	Code        string   `json:"code"`
	Value       string   `json:"value,omitempty"` // Simplified to string for most validation cases
	Suggestions []string `json:"suggestions,omitempty"`
}

// ValidationWarning represents a validation warning
type DataProcessingValidationWarning struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"` // Simplified to string for most validation cases
}

// LegacyValidationResult struct for backward compatibility
type LegacyValidationResult struct {
	Valid      bool                              `json:"valid"`
	Errors     []DataProcessingValidationError   `json:"errors,omitempty"`
	Warnings   []DataProcessingValidationWarning `json:"warnings,omitempty"`
	Data       interface{}                       `json:"data,omitempty"`
	Message    string                            `json:"message,omitempty"`
	Score      int                               `json:"score"`
	MaxScore   int                               `json:"max_score"`
	Percentage float64                           `json:"percentage"`
	RiskLevel  string                            `json:"risk_level"`
	Duration   time.Duration                     `json:"duration"`
	Timestamp  time.Time                         `json:"timestamp"`
}

// LegacyValidationError struct for backward compatibility
type LegacyValidationError struct {
	Field    string      `json:"field"`
	Message  string      `json:"message"`
	Value    interface{} `json:"value,omitempty"`
	Expected interface{} `json:"expected,omitempty"`
	Code     string      `json:"code,omitempty"`
}

// LegacyValidationWarning struct for backward compatibility
type LegacyValidationWarning struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

// UserPreferences represents user-specific preferences
type UserPreferences struct {
	UserID      string         `json:"user_id"`
	SessionID   string         `json:"session_id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Preferences *TypedJSONData `json:"preferences"`
	Metadata    *TypedJSONData `json:"metadata"`
	Version     int            `json:"version"`
}

// LegacyUserPreferences represents legacy user preferences for backward compatibility
type LegacyUserPreferences struct {
	UserID      string                 `json:"user_id"`
	SessionID   string                 `json:"session_id"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Preferences map[string]interface{} `json:"preferences"`
	Metadata    map[string]interface{} `json:"metadata"`
	Version     int                    `json:"version"`
}

// PreferenceConfig defines preference store configuration
type PreferenceConfig struct {
	StoragePath       string        `json:"storage_path"`
	AutoSave          bool          `json:"auto_save"`
	SaveInterval      time.Duration `json:"save_interval"`
	EnableEncryption  bool          `json:"enable_encryption"`
	EncryptionKey     string        `json:"encryption_key,omitempty"`
	EnableCompression bool          `json:"enable_compression"`
	MaxPreferenceSize int           `json:"max_preference_size"`
	EnableValidation  bool          `json:"enable_validation"`
	BackupEnabled     bool          `json:"backup_enabled"`
}

// PreferenceStore provides user preferences storage and management
type PreferenceStore struct {
	logger      *slog.Logger
	config      PreferenceConfig
	preferences map[string]UserPreferences
	filePath    string
}

// SchemaConfig defines schema processing configuration
type SchemaConfig struct {
	EnableValidation     bool           `json:"enable_validation"`
	EnableTransformation bool           `json:"enable_transformation"`
	EnableCaching        bool           `json:"enable_caching"`
	CacheSize            int            `json:"cache_size"`
	DefaultSchema        *TypedJSONData `json:"default_schema"`
	StrictMode           bool           `json:"strict_mode"`
	AllowAdditional      bool           `json:"allow_additional"`
}

// SchemaValidator validates data against JSON schemas
type SchemaValidator struct {
	Schema   *TypedJSONData `json:"schema"`
	Compiled bool           `json:"compiled"`
	LastUsed time.Time      `json:"last_used"`
	UseCount int            `json:"use_count"`
}

// LegacySchemaValidator provides backward compatibility with interface{} schemas
type LegacySchemaValidator struct {
	Schema   map[string]interface{} `json:"schema"`
	Compiled bool                   `json:"compiled"`
	LastUsed time.Time              `json:"last_used"`
	UseCount int                    `json:"use_count"`
}
