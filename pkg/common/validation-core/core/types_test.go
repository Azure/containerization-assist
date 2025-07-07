package core

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorType_Constants(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		expected  string
	}{
		{"validation_error", ErrTypeValidation, "validation"},
		{"not_found_error", ErrTypeNotFound, "not_found"},
		{"system_error", ErrTypeSystem, "system"},
		{"build_error", ErrTypeBuild, "build"},
		{"deployment_error", ErrTypeDeployment, "deployment"},
		{"security_error", ErrTypeSecurity, "security"},
		{"config_error", ErrTypeConfig, "configuration"},
		{"network_error", ErrTypeNetwork, "network"},
		{"permission_error", ErrTypePermission, "permission"},
		{"syntax_error", ErrTypeSyntax, "syntax"},
		{"format_error", ErrTypeFormat, "format"},
		{"compliance_error", ErrTypeCompliance, "compliance"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.errorType))
		})
	}
}

func TestErrorSeverity_Constants(t *testing.T) {
	tests := []struct {
		name     string
		severity ErrorSeverity
		expected string
	}{
		{"critical_severity", SeverityCritical, "critical"},
		{"high_severity", SeverityHigh, "high"},
		{"medium_severity", SeverityMedium, "medium"},
		{"low_severity", SeverityLow, "low"},
		{"info_severity", SeverityInfo, "info"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.severity))
		})
	}
}

func TestNewError(t *testing.T) {
	tests := []struct {
		name      string
		code      string
		message   string
		errorType ErrorType
		severity  ErrorSeverity
	}{
		{
			name:      "validation_error",
			code:      "VALIDATION_001",
			message:   "Invalid field value",
			errorType: ErrTypeValidation,
			severity:  SeverityMedium,
		},
		{
			name:      "security_error",
			code:      "SECURITY_001",
			message:   "Security vulnerability detected",
			errorType: ErrTypeSecurity,
			severity:  SeverityCritical,
		},
		{
			name:      "build_error",
			code:      "BUILD_001",
			message:   "Build failed",
			errorType: ErrTypeBuild,
			severity:  SeverityHigh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			beforeTime := time.Now()
			err := NewError(tt.code, tt.message, tt.errorType, tt.severity)
			afterTime := time.Now()

			// Verify basic fields
			assert.Equal(t, tt.code, err.Code)
			assert.Equal(t, tt.message, err.Message)
			assert.Equal(t, tt.errorType, err.Type)
			assert.Equal(t, tt.severity, err.Severity)

			// Verify initialization
			assert.NotNil(t, err.Context)
			assert.Empty(t, err.Context)
			assert.True(t, err.Timestamp.After(beforeTime) || err.Timestamp.Equal(beforeTime))
			assert.True(t, err.Timestamp.Before(afterTime) || err.Timestamp.Equal(afterTime))

			// Verify optional fields are empty
			assert.Empty(t, err.Field)
			assert.Zero(t, err.Line)
			assert.Zero(t, err.Column)
			assert.Empty(t, err.Rule)
			assert.Empty(t, err.Suggestions)
		})
	}
}

func TestNewFieldError(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		message string
	}{
		{
			name:    "empty_field_name",
			field:   "name",
			message: "is required",
		},
		{
			name:    "invalid_email",
			field:   "email",
			message: "invalid format",
		},
		{
			name:    "number_out_of_range",
			field:   "port",
			message: "must be between 1 and 65535",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			beforeTime := time.Now()
			err := NewFieldError(tt.field, tt.message)
			afterTime := time.Now()

			// Verify basic structure
			assert.Equal(t, "FIELD_VALIDATION_ERROR", err.Code)
			assert.Equal(t, ErrTypeValidation, err.Type)
			assert.Equal(t, SeverityMedium, err.Severity)
			assert.Equal(t, tt.field, err.Field)

			// Verify message format
			expectedMessage := "Field '" + tt.field + "': " + tt.message
			assert.Equal(t, expectedMessage, err.Message)

			// Verify context
			assert.NotNil(t, err.Context)
			assert.Equal(t, tt.field, err.Context["field"])

			// Verify timestamp
			assert.True(t, err.Timestamp.After(beforeTime) || err.Timestamp.Equal(beforeTime))
			assert.True(t, err.Timestamp.Before(afterTime) || err.Timestamp.Equal(afterTime))
		})
	}
}

func TestNewLineError(t *testing.T) {
	tests := []struct {
		name    string
		line    int
		message string
		rule    string
	}{
		{
			name:    "dockerfile_syntax_error",
			line:    5,
			message: "Invalid instruction format",
			rule:    "DOCKERFILE_SYNTAX",
		},
		{
			name:    "kubernetes_yaml_error",
			line:    12,
			message: "Missing required field",
			rule:    "K8S_REQUIRED_FIELD",
		},
		{
			name:    "security_rule_violation",
			line:    3,
			message: "Running as root user",
			rule:    "SECURITY_NO_ROOT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			beforeTime := time.Now()
			err := NewLineError(tt.line, tt.message, tt.rule)
			afterTime := time.Now()

			// Verify basic structure
			assert.Equal(t, "LINE_VALIDATION_ERROR", err.Code)
			assert.Equal(t, tt.message, err.Message)
			assert.Equal(t, ErrTypeValidation, err.Type)
			assert.Equal(t, SeverityMedium, err.Severity)
			assert.Equal(t, tt.line, err.Line)
			assert.Equal(t, tt.rule, err.Rule)

			// Verify context
			assert.NotNil(t, err.Context)
			assert.Equal(t, tt.line, err.Context["line"])
			assert.Equal(t, tt.rule, err.Context["rule"])

			// Verify timestamp
			assert.True(t, err.Timestamp.After(beforeTime) || err.Timestamp.Equal(beforeTime))
			assert.True(t, err.Timestamp.Before(afterTime) || err.Timestamp.Equal(afterTime))
		})
	}
}

func TestError_JSONSerialization(t *testing.T) {
	err := &Error{
		Code:     "TEST_001",
		Message:  "Test error message",
		Type:     ErrTypeValidation,
		Severity: SeverityHigh,
		Field:    "testField",
		Line:     10,
		Column:   5,
		Rule:     "TEST_RULE",
		Context: map[string]interface{}{
			"extra": "data",
			"count": 42,
		},
		Suggestions: []string{"Fix this", "Try that"},
		Timestamp:   time.Date(2023, 12, 1, 12, 0, 0, 0, time.UTC),
	}

	// Test JSON marshaling
	data, marshalErr := json.Marshal(err)
	require.NoError(t, marshalErr)

	// Test JSON unmarshaling
	var unmarshaled Error
	unmarshalErr := json.Unmarshal(data, &unmarshaled)
	require.NoError(t, unmarshalErr)

	// Verify all fields are preserved
	assert.Equal(t, err.Code, unmarshaled.Code)
	assert.Equal(t, err.Message, unmarshaled.Message)
	assert.Equal(t, err.Type, unmarshaled.Type)
	assert.Equal(t, err.Severity, unmarshaled.Severity)
	assert.Equal(t, err.Field, unmarshaled.Field)
	assert.Equal(t, err.Line, unmarshaled.Line)
	assert.Equal(t, err.Column, unmarshaled.Column)
	assert.Equal(t, err.Rule, unmarshaled.Rule)

	// Verify Context separately to handle JSON number type conversion
	require.NotNil(t, unmarshaled.Context)
	assert.Equal(t, "data", unmarshaled.Context["extra"])
	// JSON unmarshals numbers as float64, so convert for comparison
	assert.Equal(t, float64(42), unmarshaled.Context["count"])

	assert.Equal(t, err.Suggestions, unmarshaled.Suggestions)
	assert.True(t, err.Timestamp.Equal(unmarshaled.Timestamp))
}

func TestError_OmitEmptyFields(t *testing.T) {
	// Create error with minimal fields
	err := &Error{
		Code:      "MINIMAL_001",
		Message:   "Minimal error",
		Type:      ErrTypeValidation,
		Severity:  SeverityLow,
		Timestamp: time.Date(2023, 12, 1, 12, 0, 0, 0, time.UTC),
	}

	data, marshalErr := json.Marshal(err)
	require.NoError(t, marshalErr)

	// Parse JSON to check omitempty behavior
	var jsonMap map[string]interface{}
	unmarshalErr := json.Unmarshal(data, &jsonMap)
	require.NoError(t, unmarshalErr)

	// Verify required fields are present
	assert.Contains(t, jsonMap, "code")
	assert.Contains(t, jsonMap, "message")
	assert.Contains(t, jsonMap, "type")
	assert.Contains(t, jsonMap, "severity")
	assert.Contains(t, jsonMap, "timestamp")

	// Verify omitempty fields are not present
	assert.NotContains(t, jsonMap, "field")
	assert.NotContains(t, jsonMap, "line")
	assert.NotContains(t, jsonMap, "column")
	assert.NotContains(t, jsonMap, "rule")
	assert.NotContains(t, jsonMap, "context")
	assert.NotContains(t, jsonMap, "suggestions")
}

func TestWarning_Structure(t *testing.T) {
	baseError := NewError("WARN_001", "This is a warning", ErrTypeValidation, SeverityLow)
	warning := &Warning{Error: baseError}

	// Verify warning embeds error
	assert.Equal(t, baseError.Code, warning.Code)
	assert.Equal(t, baseError.Message, warning.Message)
	assert.Equal(t, baseError.Type, warning.Type)
	assert.Equal(t, baseError.Severity, warning.Severity)
	assert.Equal(t, baseError.Timestamp, warning.Timestamp)

	// Verify warning can be used as error
	var _ *Error = warning.Error
}

func TestError_ContextManipulation(t *testing.T) {
	err := NewError("CTX_001", "Context test", ErrTypeValidation, SeverityMedium)

	// Test context is initialized as empty map
	assert.NotNil(t, err.Context)
	assert.Empty(t, err.Context)

	// Test adding context data
	err.Context["operation"] = "test"
	err.Context["retry_count"] = 3
	err.Context["metadata"] = map[string]string{"key": "value"}

	assert.Equal(t, "test", err.Context["operation"])
	assert.Equal(t, 3, err.Context["retry_count"])
	assert.IsType(t, map[string]string{}, err.Context["metadata"])
}

func TestError_SuggestionManagement(t *testing.T) {
	err := NewError("SUG_001", "Suggestion test", ErrTypeValidation, SeverityMedium)

	// Test suggestions are initially empty
	assert.Empty(t, err.Suggestions)

	// Test adding suggestions
	err.Suggestions = append(err.Suggestions, "Check the configuration file")
	err.Suggestions = append(err.Suggestions, "Verify the input parameters")
	err.Suggestions = append(err.Suggestions, "Consult the documentation")

	assert.Len(t, err.Suggestions, 3)
	assert.Contains(t, err.Suggestions, "Check the configuration file")
	assert.Contains(t, err.Suggestions, "Verify the input parameters")
	assert.Contains(t, err.Suggestions, "Consult the documentation")
}

// Benchmark tests for performance-critical operations
func BenchmarkNewError(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewError("BENCH_001", "Benchmark error", ErrTypeValidation, SeverityMedium)
	}
}

func BenchmarkNewFieldError(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewFieldError("testField", "benchmark message")
	}
}

func BenchmarkNewLineError(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewLineError(42, "benchmark message", "BENCH_RULE")
	}
}

func BenchmarkErrorJSONMarshal(b *testing.B) {
	err := NewError("BENCH_001", "Benchmark error", ErrTypeValidation, SeverityMedium)
	err.Field = "testField"
	err.Context["key"] = "value"
	err.Suggestions = []string{"suggestion1", "suggestion2"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(err)
	}
}

func TestNewWarning(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		message string
	}{
		{
			name:    "basic_warning",
			code:    "WARN_001",
			message: "This is a warning message",
		},
		{
			name:    "security_warning",
			code:    "SEC_WARN_001",
			message: "Potential security issue detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			beforeTime := time.Now()
			warning := NewWarning(tt.code, tt.message)
			afterTime := time.Now()

			// Verify warning structure
			assert.NotNil(t, warning)
			assert.NotNil(t, warning.Error)

			// Verify embedded error fields
			assert.Equal(t, tt.code, warning.Code)
			assert.Equal(t, tt.message, warning.Message)
			assert.Equal(t, ErrTypeValidation, warning.Type)
			assert.Equal(t, SeverityLow, warning.Severity)

			// Verify initialization
			assert.NotNil(t, warning.Context)
			assert.Empty(t, warning.Context)
			assert.True(t, warning.Timestamp.After(beforeTime) || warning.Timestamp.Equal(beforeTime))
			assert.True(t, warning.Timestamp.Before(afterTime) || warning.Timestamp.Equal(afterTime))
		})
	}
}

func TestError_ErrorMethod(t *testing.T) {
	tests := []struct {
		name     string
		error    *Error
		expected string
	}{
		{
			name: "error_with_field",
			error: &Error{
				Severity: SeverityHigh,
				Field:    "username",
				Message:  "is required",
			},
			expected: "[high] Field 'username': is required",
		},
		{
			name: "error_with_line",
			error: &Error{
				Severity: SeverityMedium,
				Line:     42,
				Message:  "syntax error",
			},
			expected: "[medium] Line 42: syntax error",
		},
		{
			name: "basic_error",
			error: &Error{
				Severity: SeverityCritical,
				Message:  "system failure",
			},
			expected: "[critical] system failure",
		},
		{
			name: "error_with_field_and_line_prioritizes_field",
			error: &Error{
				Severity: SeverityLow,
				Field:    "email",
				Line:     10,
				Message:  "invalid format",
			},
			expected: "[low] Field 'email': invalid format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.error.Error()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestError_FluentMethods(t *testing.T) {
	err := NewError("TEST_001", "Test error", ErrTypeValidation, SeverityMedium)

	// Test method chaining
	result := err.
		WithField("testField").
		WithLine(10).
		WithColumn(5).
		WithRule("TEST_RULE").
		WithContext("operation", "test").
		WithContext("retry_count", 3).
		WithSuggestion("Check the input").
		WithSuggestion("Try again")

	// Verify the same instance is returned for chaining
	assert.Same(t, err, result)

	// Verify all values are set
	assert.Equal(t, "testField", err.Field)
	assert.Equal(t, 10, err.Line)
	assert.Equal(t, 5, err.Column)
	assert.Equal(t, "TEST_RULE", err.Rule)

	// Verify context
	assert.Equal(t, "testField", err.Context["field"])
	assert.Equal(t, 10, err.Context["line"])
	assert.Equal(t, 5, err.Context["column"])
	assert.Equal(t, "TEST_RULE", err.Context["rule"])
	assert.Equal(t, "test", err.Context["operation"])
	assert.Equal(t, 3, err.Context["retry_count"])

	// Verify suggestions
	assert.Len(t, err.Suggestions, 2)
	assert.Contains(t, err.Suggestions, "Check the input")
	assert.Contains(t, err.Suggestions, "Try again")
}

func TestError_WithContextInitialization(t *testing.T) {
	// Test with nil context
	err := &Error{Context: nil}

	result := err.WithContext("key", "value")

	assert.Same(t, err, result)
	assert.NotNil(t, err.Context)
	assert.Equal(t, "value", err.Context["key"])
}

func TestError_WithFieldInitialization(t *testing.T) {
	// Test with nil context
	err := &Error{Context: nil}

	result := err.WithField("testField")

	assert.Same(t, err, result)
	assert.NotNil(t, err.Context)
	assert.Equal(t, "testField", err.Field)
	assert.Equal(t, "testField", err.Context["field"])
}

func TestError_WithLineInitialization(t *testing.T) {
	// Test with nil context
	err := &Error{Context: nil}

	result := err.WithLine(42)

	assert.Same(t, err, result)
	assert.NotNil(t, err.Context)
	assert.Equal(t, 42, err.Line)
	assert.Equal(t, 42, err.Context["line"])
}

func TestError_WithColumnInitialization(t *testing.T) {
	// Test with nil context
	err := &Error{Context: nil}

	result := err.WithColumn(10)

	assert.Same(t, err, result)
	assert.NotNil(t, err.Context)
	assert.Equal(t, 10, err.Column)
	assert.Equal(t, 10, err.Context["column"])
}

func TestError_WithRuleInitialization(t *testing.T) {
	// Test with nil context
	err := &Error{Context: nil}

	result := err.WithRule("TEST_RULE")

	assert.Same(t, err, result)
	assert.NotNil(t, err.Context)
	assert.Equal(t, "TEST_RULE", err.Rule)
	assert.Equal(t, "TEST_RULE", err.Context["rule"])
}

func TestError_WithSuggestionAccumulation(t *testing.T) {
	err := NewError("TEST_001", "Test error", ErrTypeValidation, SeverityMedium)

	// Initially empty
	assert.Empty(t, err.Suggestions)

	// Add multiple suggestions
	err.WithSuggestion("First suggestion")
	assert.Len(t, err.Suggestions, 1)
	assert.Equal(t, "First suggestion", err.Suggestions[0])

	err.WithSuggestion("Second suggestion")
	assert.Len(t, err.Suggestions, 2)
	assert.Equal(t, "Second suggestion", err.Suggestions[1])

	err.WithSuggestion("Third suggestion")
	assert.Len(t, err.Suggestions, 3)
	assert.Equal(t, "Third suggestion", err.Suggestions[2])
}

// Test type aliases for domain-specific results
func TestResultTypeAliases(t *testing.T) {
	// Test NonGenericResult type alias
	var nonGeneric NonGenericResult
	var genericInterface Result[interface{}]

	// These should be the same type
	assert.IsType(t, genericInterface, nonGeneric)

	// Test that we can assign interface{} data
	nonGeneric.Data = "test data"
	assert.Equal(t, "test data", nonGeneric.Data)

	nonGeneric.Data = 42
	assert.Equal(t, 42, nonGeneric.Data)

	nonGeneric.Data = map[string]string{"key": "value"}
	assert.Equal(t, map[string]string{"key": "value"}, nonGeneric.Data)
}
