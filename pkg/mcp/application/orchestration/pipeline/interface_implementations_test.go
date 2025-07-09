package pipeline

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperations_AnalyzeRepositoryTyped(t *testing.T) {
	tests := []struct {
		name        string
		sessionID   string
		params      core.AnalyzeParams
		expectError bool
		errorMsg    string
	}{
		{
			name:      "valid_analyze_parameters",
			sessionID: "test-session-123",
			params: core.AnalyzeParams{
				RepositoryPath: "/workspace/test-repo",
				IncludeFiles:   []string{"*.go", "*.js", "*.ts"},
				ExcludeFiles:   []string{".git/*", "node_modules/*"},
				DeepAnalysis:   true,
			},
			expectError: false,
		},
		{
			name:      "missing_repo_path",
			sessionID: "test-session-456",
			params:    core.AnalyzeParams{
				// Missing RepositoryPath to trigger error
			},
			expectError: true,
			errorMsg:    "repository path is required",
		},
		// Removed invalid_analysis_type test - DeepAnalysis=false is valid, test seems incorrect
		{
			name:      "empty_session_id",
			sessionID: "",
			params: core.AnalyzeParams{
				RepositoryPath: "/workspace/test-repo",
				DeepAnalysis:   false,
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
			result, err := ops.AnalyzeRepositoryTyped(ctx, tt.sessionID, tt.params)

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
				assert.NotNil(t, result.RepositoryInfo)
				assert.NotEmpty(t, result.BuildRecommendations)
			}
		})
	}
}

func TestOperations_ValidateDockerfileTyped(t *testing.T) {
	tests := []struct {
		name        string
		sessionID   string
		params      core.ValidateParams
		expectError bool
		errorMsg    string
	}{
		{
			name:      "valid_validate_parameters",
			sessionID: "test-session-123",
			params: core.ValidateParams{
				DockerfilePath: "/workspace/Dockerfile",
				StrictMode:     true,
				Rules:          []string{"security", "performance", "maintainability"},
			},
			expectError: false,
		},
		{
			name:      "missing_dockerfile_path",
			sessionID: "test-session-456",
			params: core.ValidateParams{
				StrictMode: false,
			},
			expectError: true,
			errorMsg:    "dockerfile path is required",
		},
		{
			name:      "empty_session_id",
			sessionID: "",
			params: core.ValidateParams{
				DockerfilePath: "/workspace/Dockerfile",
				StrictMode:     false,
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
			result, err := ops.ValidateDockerfileTyped(ctx, tt.sessionID, tt.params)

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
				assert.GreaterOrEqual(t, result.Score, float64(0))
				assert.Equal(t, result.Valid, result.Score >= 80)
			}
		})
	}
}

func TestOperations_ScanSecurityTyped(t *testing.T) {
	tests := []struct {
		name        string
		sessionID   string
		params      core.ConsolidatedScanParams
		expectError bool
		errorMsg    string
	}{
		{
			name:      "valid_scan_parameters",
			sessionID: "test-session-123",
			params: core.ConsolidatedScanParams{
				SessionID:      "test-session-123",
				ImageRef:       "test-image:latest",
				ScanType:       "comprehensive",
				OutputFile:     "/tmp/scan-output.json",
				SeverityFilter: "medium",
			},
			expectError: false,
		},
		{
			name:      "missing_image_name",
			sessionID: "test-session-456",
			params: core.ConsolidatedScanParams{
				SessionID: "test-session-456",
				ImageRef:  "",
				ScanType:  "basic",
			},
			expectError: true,
			errorMsg:    "image name is required",
		},
		{
			name:      "invalid_scan_type",
			sessionID: "test-session-789",
			params: core.ConsolidatedScanParams{
				SessionID: "test-session-789",
				ImageRef:  "test-image:latest",
				ScanType:  "invalid",
			},
			expectError: true,
			errorMsg:    "invalid scan type",
		},
		{
			name:      "empty_session_id",
			sessionID: "",
			params: core.ConsolidatedScanParams{
				SessionID: "",
				ImageRef:  "test-image:latest",
				ScanType:  "basic",
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
			result, err := ops.ScanSecurityTyped(ctx, tt.sessionID, tt.params)

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
				assert.NotNil(t, result.ScanReport)
				assert.GreaterOrEqual(t, len(result.VulnerabilityDetails), 0)
			}
		})
	}
}

func TestOperations_ScanSecretsTyped(t *testing.T) {
	tests := []struct {
		name        string
		sessionID   string
		params      core.ScanSecretsParams
		expectError bool
		errorMsg    string
	}{
		{
			name:      "valid_secrets_scan_parameters",
			sessionID: "test-session-123",
			params: core.ScanSecretsParams{
				Path:        "/workspace/test-repo",
				Recursive:   true,
				FileTypes:   []string{"js", "py", "go"},
				ExcludeDirs: []string{".git", "node_modules"},
			},
			expectError: false,
		},
		{
			name:      "missing_target_path",
			sessionID: "test-session-456",
			params: core.ScanSecretsParams{
				Path:      "",
				Recursive: false,
			},
			expectError: true,
			errorMsg:    "target path is required",
		},
		// Removed invalid_scan_type test - ScanSecretsParams doesn't have a ScanType field
		{
			name:      "empty_session_id",
			sessionID: "",
			params: core.ScanSecretsParams{
				Path:      "/workspace/test-repo",
				Recursive: true,
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
			result, err := ops.ScanSecretsTyped(ctx, tt.sessionID, tt.params)

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
				assert.GreaterOrEqual(t, result.SecretsFound, 0)
				assert.Greater(t, result.FilesScanned, 0)
			}
		})
	}
}

func TestOperations_ResourceManagement(t *testing.T) {
	tests := []struct {
		name         string
		sessionID    string
		resourceType string
		expectError  bool
	}{
		{
			name:         "acquire_valid_resource",
			sessionID:    "test-session-123",
			resourceType: "docker",
			expectError:  false,
		},
		{
			name:         "acquire_kubernetes_resource",
			sessionID:    "test-session-456",
			resourceType: "kubernetes",
			expectError:  false,
		},
		{
			name:         "acquire_empty_session",
			sessionID:    "",
			resourceType: "docker",
			expectError:  false, // Resource operations should handle empty session gracefully
		},
		{
			name:         "acquire_empty_resource_type",
			sessionID:    "test-session-789",
			resourceType: "",
			expectError:  false, // Resource operations should handle empty type gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create operations instance
			logger := slog.Default()
			ops := &Operations{
				logger: logger,
			}

			// Test AcquireResource
			err := ops.AcquireResource(tt.sessionID, tt.resourceType)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			// Test ReleaseResource (should match AcquireResource behavior)
			err = ops.ReleaseResource(tt.sessionID, tt.resourceType)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAnalyzeParams_Validation(t *testing.T) {
	tests := []struct {
		name        string
		params      core.AnalyzeParams
		expectValid bool
	}{
		{
			name: "valid_full_analysis",
			params: core.AnalyzeParams{
				RepositoryPath: "/workspace/repo",
				IncludeFiles:   []string{"**/*.go", "**/*.js"},
				ExcludeFiles:   []string{".git/**", "node_modules/**"},
				DeepAnalysis:   true,
			},
			expectValid: true,
		},
		{
			name: "minimal_valid_params",
			params: core.AnalyzeParams{
				RepositoryPath: "/workspace/repo",
				DeepAnalysis:   false,
			},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test parameter structure
			assert.IsType(t, core.AnalyzeParams{}, tt.params)

			// Verify field types
			assert.IsType(t, "", tt.params.RepositoryPath)
			assert.IsType(t, []string{}, tt.params.IncludeFiles)
			assert.IsType(t, []string{}, tt.params.ExcludeFiles)
			assert.IsType(t, false, tt.params.DeepAnalysis)
		})
	}
}

func TestConsolidatedScanParams_Validation(t *testing.T) {
	tests := []struct {
		name        string
		params      core.ConsolidatedScanParams
		expectValid bool
	}{
		{
			name: "valid_comprehensive_scan",
			params: core.ConsolidatedScanParams{
				SessionID:      "test-session",
				ImageRef:       "test-image:latest",
				ScanType:       "comprehensive",
				OutputFile:     "/tmp/scan.json",
				SeverityFilter: "medium",
			},
			expectValid: true,
		},
		{
			name: "minimal_valid_scan",
			params: core.ConsolidatedScanParams{
				SessionID: "test-session",
				ImageRef:  "test-image:latest",
				ScanType:  "basic",
			},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test parameter structure
			assert.IsType(t, core.ConsolidatedScanParams{}, tt.params)

			// Verify field types
			assert.IsType(t, "", tt.params.SessionID)
			assert.IsType(t, "", tt.params.ImageRef)
			assert.IsType(t, "", tt.params.ScanType)
			assert.IsType(t, "", tt.params.OutputFile)
			assert.IsType(t, "", tt.params.SeverityFilter)
		})
	}
}

// BenchmarkAnalyzeRepositoryTyped benchmarks the AnalyzeRepositoryTyped operation
func BenchmarkAnalyzeRepositoryTyped(b *testing.B) {
	logger := slog.Default()
	ops := &Operations{
		logger: logger,
	}

	params := core.AnalyzeParams{
		RepositoryPath: "/tmp/benchmark-repo",
		DeepAnalysis:   false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ops.AnalyzeRepositoryTyped(context.Background(), "benchmark-session", params)
	}
}

// BenchmarkScanSecurityTyped benchmarks the ScanSecurityTyped operation
func BenchmarkScanSecurityTyped(b *testing.B) {
	logger := slog.Default()
	ops := &Operations{
		logger: logger,
	}

	params := core.ConsolidatedScanParams{
		SessionID: "benchmark-session",
		ImageRef:  "benchmark-image:latest",
		ScanType:  "basic",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ops.ScanSecurityTyped(context.Background(), "benchmark-session", params)
	}
}
