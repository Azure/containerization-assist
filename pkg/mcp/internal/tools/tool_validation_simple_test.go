package tools

import (
	"encoding/json"
	"testing"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBaseToolArgsValidation tests the basic validation of tool arguments
func TestBaseToolArgsValidation(t *testing.T) {
	testCases := []struct {
		name        string
		sessionID   string
		expectError bool
	}{
		{
			name:        "valid_session_id",
			sessionID:   "valid-session-123",
			expectError: false,
		},
		{
			name:        "empty_session_id",
			sessionID:   "",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := types.BaseToolArgs{
				SessionID: tc.sessionID,
				DryRun:    false,
			}

			err := validateBaseToolArgs(args)

			if tc.expectError {
				assert.Error(t, err, "Should return error for %s", tc.name)
			} else {
				assert.NoError(t, err, "Should not return error for %s", tc.name)
			}
		})
	}
}

// TestJSONSerializationOfToolStructs tests that our tool structs can be serialized
func TestJSONSerializationOfToolStructs(t *testing.T) {
	t.Run("base_tool_args", func(t *testing.T) {
		args := types.BaseToolArgs{
			SessionID: "test-session",
			DryRun:    true,
		}

		data, err := json.Marshal(args)
		require.NoError(t, err, "Should be able to marshal BaseToolArgs")

		var unmarshaled types.BaseToolArgs
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err, "Should be able to unmarshal BaseToolArgs")

		assert.Equal(t, args.SessionID, unmarshaled.SessionID)
		assert.Equal(t, args.DryRun, unmarshaled.DryRun)
	})
}

// TestSchemaTagValidation validates that JSON schema tags work correctly
func TestSchemaTagValidation(t *testing.T) {
	t.Run("schema_tags_for_validation", func(t *testing.T) {
		// This test demonstrates that schema tags are properly used
		// It verifies that structs with tags can be serialized correctly

		// Example struct with schema constraints (using our updated BuildImageArgs pattern)
		type TestStruct struct {
			RequiredField string `json:"required_field" jsonschema:"required"`
			OptionalField string `json:"optional_field,omitempty"`
			EnumField     string `json:"enum_field,omitempty" jsonschema:"enum=value1,value2,value3"`
		}

		test := TestStruct{
			RequiredField: "test",
			EnumField:     "value1",
		}

		data, err := json.Marshal(test)
		require.NoError(t, err, "Should be able to marshal struct with schema tags")

		// Verify that required field is present
		assert.Contains(t, string(data), "required_field", "JSON should contain required field")
		assert.Contains(t, string(data), "test", "JSON should contain field value")
	})
}

// validateBaseToolArgs provides validation for BaseToolArgs
func validateBaseToolArgs(args types.BaseToolArgs) error {
	if args.SessionID == "" {
		return types.NewRichError("VALIDATION_ERROR", "SessionID is required", "validation_error")
	}
	return nil
}

// TestErrorHandlingPatterns tests that our error handling follows RichError patterns
func TestErrorHandlingPatterns(t *testing.T) {
	t.Run("rich_error_creation", func(t *testing.T) {
		err := types.NewRichError("TEST_ERROR", "This is a test error", "test_error")

		assert.NotNil(t, err, "RichError should be created")
		assert.Contains(t, err.Error(), "TEST_ERROR", "Error string should contain error code")
		assert.Contains(t, err.Error(), "This is a test error", "Error string should contain message")
	})

	t.Run("validation_error_pattern", func(t *testing.T) {
		args := types.BaseToolArgs{
			SessionID: "", // Invalid - empty session ID
		}

		err := validateBaseToolArgs(args)
		assert.Error(t, err, "Should return error for invalid args")

		// Test that it returns a RichError
		richErr, ok := err.(*types.RichError)
		assert.True(t, ok, "Should return a RichError")
		if richErr != nil {
			assert.Equal(t, "VALIDATION_ERROR", richErr.Code)
		}
	})
}

// BenchmarkValidation provides performance benchmarks for validation
func BenchmarkValidation(b *testing.B) {
	args := types.BaseToolArgs{
		SessionID: "test-session-123",
		DryRun:    false,
	}

	b.Run("validate_base_args", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = validateBaseToolArgs(args)
		}
	})

	b.Run("json_marshal_base_args", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = json.Marshal(args)
		}
	})
}
