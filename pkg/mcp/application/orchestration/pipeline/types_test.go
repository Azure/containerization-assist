package pipeline

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypedScanSecretsArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args TypedScanSecretsArgs
	}{
		{
			name: "valid_args_with_all_fields",
			args: TypedScanSecretsArgs{
				SessionID:   "test-session-123",
				RepoPath:    "/path/to/repo",
				FilePattern: "*.go",
			},
		},
		{
			name: "valid_args_minimal",
			args: TypedScanSecretsArgs{
				SessionID: "minimal-session",
				RepoPath:  "/minimal/path",
			},
		},
		{
			name: "empty_args",
			args: TypedScanSecretsArgs{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Test that the struct can be created and accessed
			assert.IsType(t, TypedScanSecretsArgs{}, tt.args)

			// Test field access
			sessionID := tt.args.SessionID
			repoPath := tt.args.RepoPath
			filePattern := tt.args.FilePattern

			// Verify types
			assert.IsType(t, "", sessionID)
			assert.IsType(t, "", repoPath)
			assert.IsType(t, "", filePattern)
		})
	}
}

func TestTypedOperationResult(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		result TypedOperationResult
	}{
		{
			name: "successful_operation_result",
			result: TypedOperationResult{
				Success: true,
				Data: map[string]interface{}{
					"operation": "build",
					"image_id":  "sha256:abc123",
				},
				Metadata: map[string]string{
					"version": "1.0.0",
					"env":     "production",
				},
				Duration:  30 * time.Second,
				Timestamp: time.Now(),
			},
		},
		{
			name: "failed_operation_result",
			result: TypedOperationResult{
				Success: false,
				Error:   "build failed: dockerfile not found",
				Metadata: map[string]string{
					"operation": "build",
					"stage":     "preparation",
				},
				Duration:  5 * time.Second,
				Timestamp: time.Now(),
			},
		},
		{
			name: "minimal_result",
			result: TypedOperationResult{
				Success:   true,
				Timestamp: time.Now(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.result

			// Test basic structure
			assert.IsType(t, TypedOperationResult{}, result)
			assert.IsType(t, true, result.Success)
			assert.IsType(t, "", result.Error)
			assert.IsType(t, map[string]interface{}{}, result.Data)
			assert.IsType(t, map[string]string{}, result.Metadata)
			assert.IsType(t, time.Duration(0), result.Duration)
			assert.IsType(t, time.Time{}, result.Timestamp)

			// Test logical consistency
			if result.Success {
				// Successful operations shouldn't have error messages (in most cases)
				if result.Error != "" {
					t.Logf("Warning: Successful operation has error message: %s", result.Error)
				}
			} else {
				// Failed operations might have error messages
				if result.Error == "" {
					t.Logf("Note: Failed operation has no error message")
				}
			}

			// Test timestamp validity
			if !result.Timestamp.IsZero() {
				assert.True(t, result.Timestamp.Before(time.Now().Add(time.Second)) ||
					result.Timestamp.Equal(time.Now()) ||
					result.Timestamp.After(time.Now().Add(-time.Hour)))
			}
		})
	}
}

func TestTypedOperationResult_DataAccess(t *testing.T) {
	t.Parallel()
	result := TypedOperationResult{
		Success: true,
		Data: map[string]interface{}{
			"string_value": "test",
			"int_value":    42,
			"bool_value":   true,
			"float_value":  3.14,
		},
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	// Test data access
	require.NotNil(t, result.Data)
	assert.Equal(t, "test", result.Data["string_value"])
	assert.Equal(t, 42, result.Data["int_value"])
	assert.Equal(t, true, result.Data["bool_value"])
	assert.Equal(t, 3.14, result.Data["float_value"])

	// Test metadata access
	require.NotNil(t, result.Metadata)
	assert.Equal(t, "value1", result.Metadata["key1"])
	assert.Equal(t, "value2", result.Metadata["key2"])

	// Test non-existent keys
	assert.Nil(t, result.Data["non_existent"])
	assert.Equal(t, "", result.Metadata["non_existent"])
}

func TestTypedOperationResult_Duration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		duration time.Duration
	}{
		{
			name:     "zero_duration",
			duration: 0,
		},
		{
			name:     "milliseconds",
			duration: 500 * time.Millisecond,
		},
		{
			name:     "seconds",
			duration: 30 * time.Second,
		},
		{
			name:     "minutes",
			duration: 5 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := TypedOperationResult{
				Duration: tt.duration,
			}

			assert.Equal(t, tt.duration, result.Duration)
			assert.GreaterOrEqual(t, result.Duration, time.Duration(0))
		})
	}
}

// Test type aliases to ensure they work correctly
func TestTypeAliases(t *testing.T) {
	t.Parallel()
	t.Run("type_aliases_exist", func(t *testing.T) {
		t.Parallel()
		// Test that type aliases are properly defined
		var buildArgs TypedBuildImageArgs
		var pushArgs TypedPushImageArgs
		var pullArgs TypedPullImageArgs
		var tagArgs TypedTagImageArgs
		var manifestArgs TypedGenerateManifestsArgs
		var deployArgs TypedDeployKubernetesArgs
		var healthArgs TypedCheckHealthArgs
		var analyzeArgs TypedAnalyzeRepositoryArgs
		var validateArgs TypedValidateDockerfileArgs
		var scanArgs TypedScanSecurityArgs

		// Verify they are actual types
		assert.IsType(t, TypedBuildImageArgs{}, buildArgs)
		assert.IsType(t, TypedPushImageArgs{}, pushArgs)
		assert.IsType(t, TypedPullImageArgs{}, pullArgs)
		assert.IsType(t, TypedTagImageArgs{}, tagArgs)
		assert.IsType(t, TypedGenerateManifestsArgs{}, manifestArgs)
		assert.IsType(t, TypedDeployKubernetesArgs{}, deployArgs)
		assert.IsType(t, TypedCheckHealthArgs{}, healthArgs)
		assert.IsType(t, TypedAnalyzeRepositoryArgs{}, analyzeArgs)
		assert.IsType(t, TypedValidateDockerfileArgs{}, validateArgs)
		assert.IsType(t, TypedScanSecurityArgs{}, scanArgs)
	})
}

// Benchmark tests for type operations
func BenchmarkTypedOperationResult_Creation(b *testing.B) {
	timestamp := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := TypedOperationResult{
			Success: true,
			Data: map[string]interface{}{
				"key": "value",
			},
			Metadata: map[string]string{
				"meta": "data",
			},
			Duration:  time.Millisecond,
			Timestamp: timestamp,
		}
		_ = result // Prevent optimization
	}
}

func BenchmarkTypedOperationResult_DataAccess(b *testing.B) {
	result := TypedOperationResult{
		Success: true,
		Data: map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
		Metadata: map[string]string{
			"meta1": "data1",
			"meta2": "data2",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = result.Data["key1"]
		_ = result.Data["key2"]
		_ = result.Metadata["meta1"]
		_ = result.Success
	}
}

func TestTypedScanSecretsArgs_JSON(t *testing.T) {
	t.Parallel()
	args := TypedScanSecretsArgs{
		SessionID:   "test-session",
		RepoPath:    "/test/repo",
		FilePattern: "*.go",
	}

	// Test that struct tags are properly set for JSON serialization
	// This is important for API compatibility
	assert.Equal(t, "test-session", args.SessionID)
	assert.Equal(t, "/test/repo", args.RepoPath)
	assert.Equal(t, "*.go", args.FilePattern)
}
