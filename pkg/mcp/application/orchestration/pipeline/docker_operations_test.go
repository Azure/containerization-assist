package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperations_BuildImageTyped(t *testing.T) {
	tests := []struct {
		name        string
		sessionID   string
		params      domain.BuildImageParams
		expectError bool
		errorMsg    string
	}{
		{
			name:      "valid_build_parameters",
			sessionID: "test-session-123",
			params: domain.BuildImageParams{
				SessionID:      "test-session-123",
				ImageName:      "test-image",
				Tags:           []string{"latest"},
				ContextPath:    "/workspace/test",
				DockerfilePath: "Dockerfile",
				BuildArgs:      map[string]string{"ENV": "test"},
				NoCache:        false,
				Pull:           true,
			},
			expectError: false,
		},
		{
			name:      "missing_image_name",
			sessionID: "test-session-456",
			params: domain.BuildImageParams{
				SessionID:      "test-session-456",
				Tags:           []string{"latest"},
				ContextPath:    "/workspace/test",
				DockerfilePath: "Dockerfile",
			},
			expectError: true,
			errorMsg:    "image name is required",
		},
		{
			name:      "missing_context_path",
			sessionID: "test-session-789",
			params: domain.BuildImageParams{
				SessionID:      "test-session-789",
				ImageName:      "test-image",
				Tags:           []string{"latest"},
				DockerfilePath: "Dockerfile",
			},
			expectError: true,
			errorMsg:    "invalid session workspace",
		},
		{
			name:      "empty_session_id",
			sessionID: "",
			params: domain.BuildImageParams{
				SessionID:      "",
				ImageName:      "test-image",
				Tags:           []string{"latest"},
				ContextPath:    "/workspace/test",
				DockerfilePath: "Dockerfile",
			},
			expectError: true,
			errorMsg:    "session ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create operations instance for testing
			ops := TestOperations(t)

			// Create session if needed for non-error cases
			if tt.sessionID != "" && !tt.expectError {
				_, err := ops.sessionManager.GetOrCreateSession(tt.sessionID)
				require.NoError(t, err, "Failed to create test session")
			}

			// Execute test
			ctx := context.Background()
			result, err := ops.BuildImageTyped(ctx, tt.sessionID, tt.params)

			// Verify results
			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
				assert.NotEmpty(t, result.ImageID)
				assert.Greater(t, result.BuildTime, time.Duration(0))
			}
		})
	}
}

func TestOperations_PushImageTyped(t *testing.T) {
	tests := []struct {
		name        string
		sessionID   string
		params      domain.PushImageParams
		expectError bool
		errorMsg    string
	}{
		{
			name:      "valid_push_parameters",
			sessionID: "test-session-123",
			params: domain.PushImageParams{
				ImageName:  "test-image",
				ImageRef:   "test-image:latest",
				Tag:        "latest",
				Registry:   "registry.example.com",
				Repository: "test-repo",
			},
			expectError: false,
		},
		{
			name:      "missing_image_name",
			sessionID: "test-session-456",
			params: domain.PushImageParams{
				ImageRef: "",
				Tag:      "latest",
				Registry: "registry.example.com",
			},
			expectError: true,
			errorMsg:    "image name is required",
		},
		{
			name:      "missing_registry_url",
			sessionID: "test-session-789",
			params: domain.PushImageParams{
				ImageName: "test-image",
				ImageRef:  "test-image:latest",
				Tag:       "latest",
			},
			expectError: true,
			errorMsg:    "registry URL is required",
		},
		{
			name:      "empty_session_id",
			sessionID: "",
			params: domain.PushImageParams{
				ImageName: "test-image",
				ImageRef:  "test-image:latest",
				Tag:       "latest",
				Registry:  "registry.example.com",
			},
			expectError: true,
			errorMsg:    "session ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create operations instance for testing
			ops := TestOperations(t)

			// Create session if needed for non-error cases
			if tt.sessionID != "" && !tt.expectError {
				_, err := ops.sessionManager.GetOrCreateSession(tt.sessionID)
				require.NoError(t, err, "Failed to create test session")
			}

			// Execute test
			ctx := context.Background()
			result, err := ops.PushImageTyped(ctx, tt.sessionID, tt.params)

			// Verify results
			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
				assert.NotEmpty(t, result.ImageName)
				assert.NotEmpty(t, result.Registry)
			}
		})
	}
}

func TestOperations_PullImageTyped(t *testing.T) {
	tests := []struct {
		name        string
		sessionID   string
		params      domain.PullImageParams
		expectError bool
		errorMsg    string
	}{
		{
			name:      "valid_pull_parameters",
			sessionID: "test-session-123",
			params: domain.PullImageParams{
				ImageName: "test-image:latest",
				ImageRef:  "test-image:latest",
				Platform:  "linux/amd64",
			},
			expectError: false,
		},
		{
			name:      "missing_image_name",
			sessionID: "test-session-456",
			params: domain.PullImageParams{
				ImageRef: "",
			},
			expectError: true,
			errorMsg:    "image name is required",
		},
		{
			name:      "empty_session_id",
			sessionID: "",
			params: domain.PullImageParams{
				ImageName: "test-image:latest",
				ImageRef:  "test-image:latest",
			},
			expectError: true,
			errorMsg:    "session ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create operations instance for testing
			ops := TestOperations(t)

			// Create session if needed for non-error cases
			if tt.sessionID != "" && !tt.expectError {
				_, err := ops.sessionManager.GetOrCreateSession(tt.sessionID)
				require.NoError(t, err, "Failed to create test session")
			}

			// Execute test
			ctx := context.Background()
			result, err := ops.PullImageTyped(ctx, tt.sessionID, tt.params)

			// Verify results
			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
				assert.NotEmpty(t, result.ImageID)
				assert.Greater(t, result.PullTime, time.Duration(0))
			}
		})
	}
}

func TestOperations_TagImageTyped(t *testing.T) {
	tests := []struct {
		name        string
		sessionID   string
		params      domain.TagImageParams
		expectError bool
		errorMsg    string
	}{
		{
			name:      "valid_tag_parameters",
			sessionID: "test-session-123",
			params: domain.TagImageParams{
				SourceImage: "test-image:latest",
				TargetImage: "test-image:v1.0.0",
			},
			expectError: false,
		},
		{
			name:      "missing_source_image",
			sessionID: "test-session-456",
			params: domain.TagImageParams{
				TargetImage: "test-image:v1.0.0",
			},
			expectError: true,
			errorMsg:    "source image is required",
		},
		{
			name:      "missing_target_image",
			sessionID: "test-session-789",
			params: domain.TagImageParams{
				SourceImage: "test-image:latest",
			},
			expectError: true,
			errorMsg:    "target image is required",
		},
		{
			name:      "empty_session_id",
			sessionID: "",
			params: domain.TagImageParams{
				SourceImage: "test-image:latest",
				TargetImage: "test-image:v1.0.0",
			},
			expectError: true,
			errorMsg:    "session ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create operations instance for testing
			ops := TestOperations(t)

			// Create session if needed for non-error cases
			if tt.sessionID != "" && !tt.expectError {
				_, err := ops.sessionManager.GetOrCreateSession(tt.sessionID)
				require.NoError(t, err, "Failed to create test session")
			}

			// Execute test
			ctx := context.Background()
			result, err := ops.TagImageTyped(ctx, tt.sessionID, tt.params)

			// Verify results
			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
				assert.NotEmpty(t, result.TargetImage)
				assert.Equal(t, tt.params.TargetImage, result.TargetImage)
			}
		})
	}
}

// BenchmarkBuildImageTyped benchmarks the BuildImageTyped operation
func BenchmarkBuildImageTyped(b *testing.B) {
	// Create operations instance for benchmarking
	ops := createBenchmarkOperations(b)

	// Create session for benchmark
	_, err := ops.sessionManager.GetOrCreateSession("benchmark-session")
	if err != nil {
		b.Fatal("Failed to create benchmark session")
	}

	params := domain.BuildImageParams{
		SessionID:      "benchmark-session",
		ImageName:      "benchmark-image",
		Tags:           []string{"latest"},
		ContextPath:    "/tmp/benchmark",
		DockerfilePath: "Dockerfile",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ops.BuildImageTyped(context.Background(), "benchmark-session", params)
	}
}

// BenchmarkPushImageTyped benchmarks the PushImageTyped operation
func BenchmarkPushImageTyped(b *testing.B) {
	// Create operations instance for benchmarking
	ops := createBenchmarkOperations(b)

	// Create session for benchmark
	_, err := ops.sessionManager.GetOrCreateSession("benchmark-session")
	if err != nil {
		b.Fatal("Failed to create benchmark session")
	}

	params := domain.PushImageParams{
		ImageRef:   "benchmark-image:latest",
		Tag:        "latest",
		Registry:   "registry.example.com",
		Repository: "benchmark-repo",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ops.PushImageTyped(context.Background(), "benchmark-session", params)
	}
}
