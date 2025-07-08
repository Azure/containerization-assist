//go:build integration
// +build integration

package scan

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/core"
	securityTypes "github.com/Azure/container-kit/pkg/mcp/core/types"
	"github.com/Azure/container-kit/pkg/mcp/session"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TrivyIntegrationTest represents an integration test with Trivy scanner
type TrivyIntegrationTest struct {
	name        string
	image       string
	scanType    string
	scanner     string
	expectError bool
	timeout     time.Duration
}

// TestTrivyIntegration_ScanImages tests integration with actual Trivy scanner
func TestTrivyIntegration_ScanImages(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Skip if SKIP_INTEGRATION is set
	if os.Getenv("SKIP_INTEGRATION") == "true" {
		t.Skip("Skipping integration tests due to SKIP_INTEGRATION environment variable")
	}

	tests := []TrivyIntegrationTest{
		{
			name:        "scan_alpine_latest",
			image:       "alpine:latest",
			scanType:    "image",
			scanner:     "trivy",
			expectError: false,
			timeout:     60 * time.Second,
		},
		{
			name:        "scan_nginx_latest",
			image:       "nginx:latest",
			scanType:    "image",
			scanner:     "trivy",
			expectError: false,
			timeout:     120 * time.Second,
		},
		{
			name:        "scan_nonexistent_image",
			image:       "nonexistent/image:notfound",
			scanType:    "image",
			scanner:     "trivy",
			expectError: true,
			timeout:     30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create integration test setup
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			// Create mock dependencies for integration test
			mockAdapter := &IntegrationPipelineAdapter{}
			mockSession := &IntegrationSessionManager{}
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

			// Create tool instance
			tool := NewSecurityScanTool(mockAdapter, mockSession, logger)
			scanTool, ok := tool.(*securityScanToolImpl)
			require.True(t, ok)

			// Prepare scan parameters
			params := securityTypes.SecurityScanParams{
				Target:        tt.image,
				ScanType:      tt.scanType,
				Scanner:       tt.scanner,
				Format:        "json",
				IgnoreUnfixed: false,
				SessionID:     "integration-test-session",
			}

			// Execute the scan
			result, err := scanTool.Execute(ctx, params)

			// Verify results
			if tt.expectError {
				// For error cases, we might still get a result but with Success=false
				assert.False(t, result.Success)
			} else {
				require.NoError(t, err)
				assert.True(t, result.Success)
				assert.Equal(t, tt.image, result.Target)
				assert.Equal(t, tt.scanner, result.Scanner)
				assert.Greater(t, result.Duration, time.Duration(0))

				// Verify scan results structure
				assert.GreaterOrEqual(t, result.TotalVulnerabilities, 0)
				assert.NotNil(t, result.VulnerabilitiesBySeverity)
				assert.GreaterOrEqual(t, result.RiskScore, 0.0)
				assert.LessOrEqual(t, result.RiskScore, 10.0)
				assert.Contains(t, []string{"MINIMAL", "LOW", "MEDIUM", "HIGH", "CRITICAL"}, result.RiskLevel)
			}

			t.Logf("Scan completed for %s: Success=%v, Vulnerabilities=%d, RiskScore=%.2f",
				tt.image, result.Success, result.TotalVulnerabilities, result.RiskScore)
		})
	}
}

// TestTrivyIntegration_ScanPerformance tests the performance characteristics of Trivy integration
func TestTrivyIntegration_ScanPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance tests in short mode")
	}

	if os.Getenv("SKIP_INTEGRATION") == "true" {
		t.Skip("Skipping integration tests due to SKIP_INTEGRATION environment variable")
	}

	// Create integration test setup
	ctx := context.Background()
	mockAdapter := &IntegrationPipelineAdapter{}
	mockSession := &IntegrationSessionManager{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	tool := NewSecurityScanTool(mockAdapter, mockSession, logger)
	scanTool, ok := tool.(*securityScanToolImpl)
	require.True(t, ok)

	params := securityTypes.SecurityScanParams{
		Target:        "alpine:latest",
		ScanType:      "image",
		Scanner:       "trivy",
		Format:        "json",
		IgnoreUnfixed: true, // Skip unfixed for faster scanning
		SessionID:     "performance-test-session",
	}

	// Warm-up run
	_, err := scanTool.Execute(ctx, params)
	require.NoError(t, err)

	// Performance measurement
	const numRuns = 3
	var totalDuration time.Duration

	for i := 0; i < numRuns; i++ {
		start := time.Now()
		result, err := scanTool.Execute(ctx, params)
		duration := time.Since(start)

		require.NoError(t, err)
		assert.True(t, result.Success)

		totalDuration += duration
		t.Logf("Run %d: %v", i+1, duration)
	}

	avgDuration := totalDuration / numRuns
	t.Logf("Average duration over %d runs: %v", numRuns, avgDuration)

	// Performance assertions (adjust thresholds based on environment)
	assert.Less(t, avgDuration, 30*time.Second, "Average scan time should be under 30 seconds")
}

// TestTrivyIntegration_ErrorHandling tests error handling in Trivy integration
func TestTrivyIntegration_ErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	if os.Getenv("SKIP_INTEGRATION") == "true" {
		t.Skip("Skipping integration tests due to SKIP_INTEGRATION environment variable")
	}

	tests := []struct {
		name   string
		params securityTypes.SecurityScanParams
	}{
		{
			name: "invalid_image_format",
			params: securityTypes.SecurityScanParams{
				Target:    "invalid-image-format",
				ScanType:  "image",
				Scanner:   "trivy",
				SessionID: "error-test-1",
			},
		},
		{
			name: "unsupported_scan_type",
			params: securityTypes.SecurityScanParams{
				Target:    "alpine:latest",
				ScanType:  "unsupported",
				Scanner:   "trivy",
				SessionID: "error-test-2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			mockAdapter := &IntegrationPipelineAdapter{}
			mockSession := &IntegrationSessionManager{}
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

			tool := NewSecurityScanTool(mockAdapter, mockSession, logger)
			scanTool, ok := tool.(*securityScanToolImpl)
			require.True(t, ok)

			result, err := scanTool.Execute(ctx, tt.params)

			// Should handle errors gracefully
			// Result may be returned even with errors
			if err != nil {
				t.Logf("Expected error for %s: %v", tt.name, err)
			}

			// Result should indicate failure for invalid inputs
			if tt.name == "unsupported_scan_type" {
				assert.False(t, result.Success)
			}
		})
	}
}

// TestTrivyIntegration_OutputFormats tests different output formats
func TestTrivyIntegration_OutputFormats(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	if os.Getenv("SKIP_INTEGRATION") == "true" {
		t.Skip("Skipping integration tests due to SKIP_INTEGRATION environment variable")
	}

	formats := []string{"json", "yaml", "table"}

	for _, format := range formats {
		t.Run("format_"+format, func(t *testing.T) {
			ctx := context.Background()
			mockAdapter := &IntegrationPipelineAdapter{}
			mockSession := &IntegrationSessionManager{}
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

			tool := NewSecurityScanTool(mockAdapter, mockSession, logger)
			scanTool, ok := tool.(*securityScanToolImpl)
			require.True(t, ok)

			params := securityTypes.SecurityScanParams{
				Target:        "alpine:latest",
				ScanType:      "image",
				Scanner:       "trivy",
				Format:        format,
				IgnoreUnfixed: true,
				SessionID:     "format-test-" + format,
			}

			result, err := scanTool.Execute(ctx, params)
			require.NoError(t, err)
			assert.True(t, result.Success)
			assert.Equal(t, "alpine:latest", result.Target)
		})
	}
}

// Integration test helper structs

// IntegrationPipelineAdapter provides minimal implementation for integration tests
type IntegrationPipelineAdapter struct{}

func (a *IntegrationPipelineAdapter) GetSessionWorkspace(sessionID string) string {
	return "/tmp/test-workspace/" + sessionID
}

func (a *IntegrationPipelineAdapter) UpdateSessionState(sessionID string, updateFunc func(*core.SessionState)) error {
	// Mock implementation for integration tests
	return nil
}

// Implement all required methods with minimal functionality for integration tests
func (a *IntegrationPipelineAdapter) BuildImageTyped(ctx context.Context, sessionID string, params core.BuildImageParams) (*core.BuildImageResult, error) {
	return &core.BuildImageResult{}, nil
}

func (a *IntegrationPipelineAdapter) PushImageTyped(ctx context.Context, sessionID string, params core.PushImageParams) (*core.PushImageResult, error) {
	return &core.PushImageResult{}, nil
}

func (a *IntegrationPipelineAdapter) PullImageTyped(ctx context.Context, sessionID string, params core.PullImageParams) (*core.PullImageResult, error) {
	return &core.PullImageResult{}, nil
}

func (a *IntegrationPipelineAdapter) TagImageTyped(ctx context.Context, sessionID string, params core.TagImageParams) (*core.TagImageResult, error) {
	return &core.TagImageResult{}, nil
}

func (a *IntegrationPipelineAdapter) GenerateManifestsTyped(ctx context.Context, sessionID string, params core.GenerateManifestsParams) (*core.GenerateManifestsResult, error) {
	return &core.GenerateManifestsResult{}, nil
}

func (a *IntegrationPipelineAdapter) DeployKubernetesTyped(ctx context.Context, sessionID string, params core.DeployParams) (*core.DeployResult, error) {
	return &core.DeployResult{}, nil
}

func (a *IntegrationPipelineAdapter) CheckHealthTyped(ctx context.Context, sessionID string, params core.HealthCheckParams) (*core.HealthCheckResult, error) {
	return &core.HealthCheckResult{}, nil
}

func (a *IntegrationPipelineAdapter) AnalyzeRepositoryTyped(ctx context.Context, sessionID string, params core.AnalyzeParams) (*core.AnalyzeResult, error) {
	return &core.AnalyzeResult{}, nil
}

func (a *IntegrationPipelineAdapter) ValidateDockerfileTyped(ctx context.Context, sessionID string, params core.ValidateParams) (*core.ConsolidatedValidateResult, error) {
	return &core.ConsolidatedValidateResult{}, nil
}

func (a *IntegrationPipelineAdapter) ScanSecurityTyped(ctx context.Context, sessionID string, params core.ConsolidatedScanParams) (*core.ScanResult, error) {
	return &core.ScanResult{}, nil
}

func (a *IntegrationPipelineAdapter) ScanSecretsTyped(ctx context.Context, sessionID string, params core.ScanSecretsParams) (*core.ScanSecretsResult, error) {
	return &core.ScanSecretsResult{}, nil
}

// IntegrationSessionManager provides minimal implementation for integration tests
type IntegrationSessionManager struct{}

func (m *IntegrationSessionManager) GetSession(ctx context.Context, sessionID string) (*session.SessionState, error) {
	return &session.SessionState{
		ID:        sessionID,
		UserID:    "integration-test-user",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (m *IntegrationSessionManager) CreateSession(ctx context.Context, userID string) (*session.SessionState, error) {
	return &session.SessionState{
		ID:        "integration-test-session",
		UserID:    userID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (m *IntegrationSessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	return nil
}

func (m *IntegrationSessionManager) ListSessions(ctx context.Context, filter core.SessionFilter) ([]*session.SessionState, error) {
	return []*session.SessionState{}, nil
}

func (m *IntegrationSessionManager) GetOrCreateSession(ctx context.Context, sessionID string) (*session.SessionState, error) {
	return m.GetSession(ctx, sessionID)
}

func (m *IntegrationSessionManager) UpdateSession(ctx context.Context, sessionID string, updater func(*session.SessionState)) error {
	return nil
}

func (m *IntegrationSessionManager) GetStats(ctx context.Context) (*core.SessionManagerStats, error) {
	return &core.SessionManagerStats{}, nil
}

func (m *IntegrationSessionManager) GarbageCollect(ctx context.Context) error {
	return nil
}

func (m *IntegrationSessionManager) CreateWorkflowSession(ctx context.Context, spec *session.WorkflowSpec) (*session.SessionState, error) {
	return &session.SessionState{}, nil
}

func (m *IntegrationSessionManager) GetWorkflowSession(ctx context.Context, sessionID string) (*session.WorkflowSession, error) {
	return &session.WorkflowSession{}, nil
}

func (m *IntegrationSessionManager) UpdateWorkflowSession(ctx context.Context, session *session.WorkflowSession) error {
	return nil
}

// Helper function to load and parse Trivy JSON output for testing
func loadTrivyTestData(filename string) (map[string]interface{}, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	return result, err
}

// BenchmarkTrivyIntegration_ScanImage benchmarks actual Trivy scanning
func BenchmarkTrivyIntegration_ScanImage(b *testing.B) {
	if os.Getenv("SKIP_INTEGRATION") == "true" {
		b.Skip("Skipping integration benchmarks due to SKIP_INTEGRATION environment variable")
	}

	ctx := context.Background()
	mockAdapter := &IntegrationPipelineAdapter{}
	mockSession := &IntegrationSessionManager{}
	logger := zerolog.New(io.Discard) // Suppress logging during benchmarks

	tool := NewSecurityScanTool(mockAdapter, mockSession, logger)
	scanTool, ok := tool.(*securityScanToolImpl)
	require.True(b, ok)

	params := securityTypes.SecurityScanParams{
		Target:        "alpine:latest",
		ScanType:      "image",
		Scanner:       "trivy",
		Format:        "json",
		IgnoreUnfixed: true,
		SessionID:     "benchmark-session",
	}

	// Warm-up
	_, err := scanTool.Execute(ctx, params)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := scanTool.Execute(ctx, params)
		require.NoError(b, err)
	}
}
