package build

import (
	"context"
	"testing"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAtomicBuildImageTool_Validate(t *testing.T) {
	logger := zerolog.Nop()
	tool := NewAtomicBuildImageTool(nil, nil, logger)
	ctx := context.Background()

	tests := []struct {
		name      string
		args      interface{}
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid_args_minimal",
			args: AtomicBuildImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "session-123",
				},
				ImageName: "my-app",
			},
			wantError: false,
		},
		{
			name: "valid_args_full",
			args: AtomicBuildImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "session-456",
					DryRun:    false,
				},
				ImageName:      "registry.example.com/my-app",
				ImageTag:       "v1.0.0",
				DockerfilePath: "./Dockerfile",
				BuildContext:   "./src",
				Platform:       "linux/amd64",
				NoCache:        true,
				BuildArgs: map[string]string{
					"VERSION":    "1.0.0",
					"BUILD_DATE": "2023-01-01",
				},
				PushAfterBuild: true,
				RegistryURL:    "registry.example.com",
			},
			wantError: false,
		},
		{
			name: "invalid_args_type",
			args: map[string]interface{}{
				"session_id": "session-123",
				"image_name": "my-app",
			},
			wantError: true,
			errorMsg:  "Invalid argument type for atomic_build_image",
		},
		{
			name:      "invalid_args_nil",
			args:      nil,
			wantError: true,
			errorMsg:  "Invalid argument type for atomic_build_image",
		},
		{
			name: "missing_image_name",
			args: AtomicBuildImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "session-123",
				},
				ImageName: "",
			},
			wantError: true,
			errorMsg:  "ImageName is required",
		},
		{
			name: "missing_session_id",
			args: AtomicBuildImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "",
				},
				ImageName: "my-app",
			},
			wantError: true,
			errorMsg:  "SessionID is required",
		},
		{
			name: "empty_image_name_whitespace",
			args: AtomicBuildImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "session-123",
				},
				ImageName: "   ",
			},
			wantError: false, // Current implementation doesn't trim whitespace
		},
		{
			name: "valid_complex_image_name",
			args: AtomicBuildImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "session-123",
				},
				ImageName: "docker.io/library/my-app_test.service-v2",
			},
			wantError: false,
		},
		{
			name: "valid_with_all_platforms",
			args: AtomicBuildImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "session-123",
				},
				ImageName: "my-app",
				Platform:  "linux/arm64",
			},
			wantError: false,
		},
		{
			name: "valid_with_build_args_empty",
			args: AtomicBuildImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "session-123",
				},
				ImageName: "my-app",
				BuildArgs: map[string]string{},
			},
			wantError: false,
		},
		{
			name: "valid_with_registry_url",
			args: AtomicBuildImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "session-123",
				},
				ImageName:      "my-app",
				PushAfterBuild: true,
				RegistryURL:    "localhost:5000",
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tool.Validate(ctx, tt.args)

			if tt.wantError {
				require.Error(t, err, "Expected error but got none")
				assert.Contains(t, err.Error(), tt.errorMsg, "Error message should contain expected text")

				// Verify it's a RichError (validation errors are built with RichError)
				var richErr *types.RichError
				if assert.ErrorAs(t, err, &richErr, "Should be a RichError") {
					assert.Equal(t, "validation_error", richErr.Type, "Should be validation error type")
				}
			} else {
				assert.NoError(t, err, "Expected no error but got: %v", err)
			}
		})
	}
}

func TestAtomicBuildImageTool_ValidateEdgeCases(t *testing.T) {
	logger := zerolog.Nop()
	tool := NewAtomicBuildImageTool(nil, nil, logger)
	ctx := context.Background()

	tests := []struct {
		name      string
		args      AtomicBuildImageArgs
		wantError bool
		errorMsg  string
	}{
		{
			name: "very_long_image_name",
			args: AtomicBuildImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "session-123",
				},
				ImageName: "registry.example.com/very/long/path/to/image/with/many/components/that/might/cause/issues/my-super-long-application-name-that-exceeds-normal-limits",
			},
			wantError: false, // Should be valid unless there's a length limit
		},
		{
			name: "image_name_with_special_chars",
			args: AtomicBuildImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "session-123",
				},
				ImageName: "my-app_v2.test",
			},
			wantError: false,
		},
		{
			name: "session_id_with_special_chars",
			args: AtomicBuildImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "session-123-abc_def.xyz",
				},
				ImageName: "my-app",
			},
			wantError: false,
		},
		{
			name: "dockerfile_path_absolute",
			args: AtomicBuildImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "session-123",
				},
				ImageName:      "my-app",
				DockerfilePath: "/absolute/path/to/Dockerfile",
			},
			wantError: false,
		},
		{
			name: "build_context_absolute",
			args: AtomicBuildImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "session-123",
				},
				ImageName:    "my-app",
				BuildContext: "/absolute/path/to/context",
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tool.Validate(ctx, tt.args)

			if tt.wantError {
				require.Error(t, err, "Expected error but got none")
				assert.Contains(t, err.Error(), tt.errorMsg, "Error message should contain expected text")
			} else {
				assert.NoError(t, err, "Expected no error but got: %v", err)
			}
		})
	}
}

func TestAtomicBuildImageTool_ValidateContextCancellation(t *testing.T) {
	logger := zerolog.Nop()
	tool := NewAtomicBuildImageTool(nil, nil, logger)

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	args := AtomicBuildImageArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "session-123",
		},
		ImageName: "my-app",
	}

	// Validation should work even with cancelled context since it's not doing async operations
	err := tool.Validate(ctx, args)
	assert.NoError(t, err, "Validation should succeed even with cancelled context")
}

// Benchmark test for validation performance
func BenchmarkAtomicBuildImageTool_Validate(b *testing.B) {
	logger := zerolog.Nop()
	tool := NewAtomicBuildImageTool(nil, nil, logger)
	ctx := context.Background()

	args := AtomicBuildImageArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "session-123",
		},
		ImageName:      "my-app",
		ImageTag:       "v1.0.0",
		DockerfilePath: "./Dockerfile",
		BuildContext:   "./src",
		BuildArgs: map[string]string{
			"VERSION":    "1.0.0",
			"BUILD_DATE": "2023-01-01",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := tool.Validate(ctx, args)
		if err != nil {
			b.Fatalf("Validation failed: %v", err)
		}
	}
}
