package pipeline

import (
	"context"
	"log/slog"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperations_GenerateManifestsTyped(t *testing.T) {
	tests := []struct {
		name        string
		sessionID   string
		params      core.GenerateManifestsParams
		expectError bool
		errorMsg    string
	}{
		{
			name:      "valid_generate_parameters",
			sessionID: "test-session-123",
			params: core.GenerateManifestsParams{
				AppName:     "test-app",
				ImageRef:    "test-image:latest",
				Namespace:   "default",
				Replicas:    3,
				Port:        8080,
				Labels:      map[string]string{"app": "test-app"},
				Annotations: map[string]string{"env": "test"},
				Resources: core.ResourceLimits{
					Requests: core.ResourceSpec{
						CPU:    "100m",
						Memory: "128Mi",
					},
					Limits: core.ResourceSpec{
						CPU:    "200m",
						Memory: "256Mi",
					},
				},
				HealthCheck: core.HealthCheckConfig{
					Enabled:             true,
					Path:                "/health",
					Port:                8080,
					InitialDelaySeconds: 30,
					PeriodSeconds:       10,
					TimeoutSeconds:      5,
					FailureThreshold:    3,
				},
			},
			expectError: false,
		},
		{
			name:      "missing_app_name",
			sessionID: "test-session-456",
			params: core.GenerateManifestsParams{
				ImageRef:  "test-image:latest",
				Namespace: "default",
				Replicas:  1,
			},
			expectError: true,
			errorMsg:    "app name is required",
		},
		{
			name:      "missing_image_name",
			sessionID: "test-session-789",
			params: core.GenerateManifestsParams{
				AppName:   "test-app",
				Namespace: "default",
				Replicas:  1,
			},
			expectError: true,
			errorMsg:    "image name is required",
		},
		{
			name:      "invalid_replicas",
			sessionID: "test-session-101",
			params: core.GenerateManifestsParams{
				AppName:   "test-app",
				ImageRef:  "test-image:latest",
				Namespace: "default",
				Replicas:  0,
			},
			expectError: true,
			errorMsg:    "replicas must be greater than 0",
		},
		{
			name:      "empty_session_id",
			sessionID: "",
			params: core.GenerateManifestsParams{
				AppName:   "test-app",
				ImageRef:  "test-image:latest",
				Namespace: "default",
				Replicas:  1,
			},
			expectError: true,
			errorMsg:    "session ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create operations instance
			// Create operations instance with proper initialization
			ops := TestOperations(t)

			// Create session if needed for non-error cases
			if tt.sessionID != "" && !tt.expectError {
				_, err := ops.sessionManager.GetOrCreateSession(context.Background(), tt.sessionID)
				require.NoError(t, err, "Failed to create test session")
			}

			// Execute test
			ctx := context.Background()
			result, err := ops.GenerateManifestsTyped(ctx, tt.sessionID, tt.params)

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
				assert.NotEmpty(t, result.ManifestPaths)
				assert.Greater(t, result.ManifestCount, 0)
			}
		})
	}
}

func TestOperations_DeployKubernetesTyped(t *testing.T) {
	tests := []struct {
		name        string
		sessionID   string
		params      core.DeployParams
		expectError bool
		errorMsg    string
	}{
		{
			name:      "valid_deploy_parameters",
			sessionID: "test-session-123",
			params: core.DeployParams{
				SessionID:     "test-session-123",
				ManifestPaths: []string{"/workspace/k8s/manifests.yaml"},
				Namespace:     "default",
				DryRun:        false,
				Wait:          true,
				Timeout:       300,
			},
			expectError: false,
		},
		{
			name:      "missing_manifest_path",
			sessionID: "test-session-456",
			params: core.DeployParams{
				SessionID: "test-session-456",
				Namespace: "default",
			},
			expectError: true,
			errorMsg:    "manifest_paths",
		},
		{
			name:      "missing_namespace",
			sessionID: "test-session-789",
			params: core.DeployParams{
				SessionID:     "test-session-789",
				ManifestPaths: []string{"/workspace/k8s/manifests.yaml"},
			},
			expectError: true,
			errorMsg:    "namespace is required",
		},
		{
			name:      "invalid_timeout",
			sessionID: "test-session-101",
			params: core.DeployParams{
				SessionID:     "test-session-101",
				ManifestPaths: []string{"/workspace/k8s/manifests.yaml"},
				Namespace:     "default",
				Timeout:       -1,
			},
			expectError: true,
			errorMsg:    "timeout must be positive",
		},
		{
			name:      "empty_session_id",
			sessionID: "",
			params: core.DeployParams{
				SessionID:     "",
				ManifestPaths: []string{"/workspace/k8s/manifests.yaml"},
				Namespace:     "default",
			},
			expectError: true,
			errorMsg:    "session ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create operations instance
			logger := slog.Default()
			ops := &Operations{
				logger: logger,
			}

			// Execute test
			ctx := context.Background()
			result, err := ops.DeployKubernetesTyped(ctx, tt.sessionID, tt.params)

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
				assert.NotEmpty(t, result.Status)
				assert.NotEmpty(t, result.Namespace)
			}
		})
	}
}

func TestOperations_CheckHealthTyped(t *testing.T) {
	tests := []struct {
		name        string
		sessionID   string
		params      core.HealthCheckParams
		expectError bool
		errorMsg    string
	}{
		{
			name:      "valid_health_check_parameters",
			sessionID: "test-session-123",
			params: core.HealthCheckParams{
				AppName:     "test-app",
				Namespace:   "default",
				WaitTimeout: 60,
			},
			expectError: false,
		},
		{
			name:      "missing_app_name",
			sessionID: "test-session-456",
			params: core.HealthCheckParams{
				Namespace:   "default",
				WaitTimeout: 30,
			},
			expectError: true,
			errorMsg:    "app name is required",
		},
		{
			name:      "missing_namespace",
			sessionID: "test-session-789",
			params: core.HealthCheckParams{
				AppName:     "test-app",
				WaitTimeout: 30,
			},
			expectError: true,
			errorMsg:    "namespace is required",
		},
		{
			name:      "invalid_timeout",
			sessionID: "test-session-101",
			params: core.HealthCheckParams{
				AppName:     "test-app",
				Namespace:   "default",
				WaitTimeout: 0,
			},
			expectError: true,
			errorMsg:    "timeout must be positive",
		},
		{
			name:      "empty_session_id",
			sessionID: "",
			params: core.HealthCheckParams{
				AppName:     "test-app",
				Namespace:   "default",
				WaitTimeout: 30,
			},
			expectError: true,
			errorMsg:    "session ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create operations instance
			logger := slog.Default()
			ops := &Operations{
				logger: logger,
			}

			// Execute test
			ctx := context.Background()
			result, err := ops.CheckHealthTyped(ctx, tt.sessionID, tt.params)

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
				assert.NotEmpty(t, result.OverallHealth)
				assert.NotNil(t, result.ResourceStatuses)
			}
		})
	}
}

// ResourceRequirements validation is tested in core package

// HealthCheckConfig validation is tested in core package

// BenchmarkGenerateManifestsTyped benchmarks the GenerateManifestsTyped operation
func BenchmarkGenerateManifestsTyped(b *testing.B) {
	logger := slog.Default()
	ops := &Operations{
		logger: logger,
	}

	params := core.GenerateManifestsParams{
		AppName:   "benchmark-app",
		ImageRef:  "benchmark-image:latest",
		Namespace: "default",
		Replicas:  1,
		Port:      8080,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ops.GenerateManifestsTyped(context.Background(), "benchmark-session", params)
	}
}

// BenchmarkDeployKubernetesTyped benchmarks the DeployKubernetesTyped operation
func BenchmarkDeployKubernetesTyped(b *testing.B) {
	logger := slog.Default()
	ops := &Operations{
		logger: logger,
	}

	params := core.DeployParams{
		SessionID:     "benchmark-session",
		ManifestPaths: []string{"/tmp/benchmark/manifests.yaml"},
		Namespace:     "default",
		DryRun:        true, // Use dry run for benchmarking
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ops.DeployKubernetesTyped(context.Background(), "benchmark-session", params)
	}
}
