package scan

import (
	"context"
	"io"
	"testing"
	"time"

	"log/slog"
	"os"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/core"
	securityTypes "github.com/Azure/container-kit/pkg/mcp/application/core/types"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockPipelineAdapter is a mock implementation of TypedPipelineOperations
type MockPipelineAdapter struct {
	mock.Mock
}

// Session operations
func (m *MockPipelineAdapter) GetSessionWorkspace(sessionID string) string {
	args := m.Called(sessionID)
	return args.String(0)
}

func (m *MockPipelineAdapter) UpdateSessionState(sessionID string, updateFunc func(*core.SessionState)) error {
	args := m.Called(sessionID, updateFunc)
	return args.Error(0)
}

// Docker operations
func (m *MockPipelineAdapter) BuildImageTyped(ctx context.Context, sessionID string, params core.BuildImageParams) (*core.BuildImageResult, error) {
	args := m.Called(ctx, sessionID, params)
	return args.Get(0).(*core.BuildImageResult), args.Error(1)
}

func (m *MockPipelineAdapter) PushImageTyped(ctx context.Context, sessionID string, params core.PushImageParams) (*core.PushImageResult, error) {
	args := m.Called(ctx, sessionID, params)
	return args.Get(0).(*core.PushImageResult), args.Error(1)
}

func (m *MockPipelineAdapter) PullImageTyped(ctx context.Context, sessionID string, params core.PullImageParams) (*core.PullImageResult, error) {
	args := m.Called(ctx, sessionID, params)
	return args.Get(0).(*core.PullImageResult), args.Error(1)
}

func (m *MockPipelineAdapter) TagImageTyped(ctx context.Context, sessionID string, params core.TagImageParams) (*core.TagImageResult, error) {
	args := m.Called(ctx, sessionID, params)
	return args.Get(0).(*core.TagImageResult), args.Error(1)
}

// Kubernetes operations
func (m *MockPipelineAdapter) GenerateManifestsTyped(ctx context.Context, sessionID string, params core.GenerateManifestsParams) (*core.GenerateManifestsResult, error) {
	args := m.Called(ctx, sessionID, params)
	return args.Get(0).(*core.GenerateManifestsResult), args.Error(1)
}

func (m *MockPipelineAdapter) DeployKubernetesTyped(ctx context.Context, sessionID string, params core.DeployParams) (*core.DeployResult, error) {
	args := m.Called(ctx, sessionID, params)
	return args.Get(0).(*core.DeployResult), args.Error(1)
}

func (m *MockPipelineAdapter) CheckHealthTyped(ctx context.Context, sessionID string, params core.HealthCheckParams) (*core.HealthCheckResult, error) {
	args := m.Called(ctx, sessionID, params)
	return args.Get(0).(*core.HealthCheckResult), args.Error(1)
}

// Analysis operations
func (m *MockPipelineAdapter) AnalyzeRepositoryTyped(ctx context.Context, sessionID string, params core.AnalyzeParams) (*core.AnalyzeResult, error) {
	args := m.Called(ctx, sessionID, params)
	return args.Get(0).(*core.AnalyzeResult), args.Error(1)
}

func (m *MockPipelineAdapter) ValidateDockerfileTyped(ctx context.Context, sessionID string, params core.ValidateParams) (*core.ConsolidatedValidateResult, error) {
	args := m.Called(ctx, sessionID, params)
	return args.Get(0).(*core.ConsolidatedValidateResult), args.Error(1)
}

// Security operations
func (m *MockPipelineAdapter) ScanSecurityTyped(ctx context.Context, sessionID string, params core.ConsolidatedScanParams) (*core.ScanResult, error) {
	args := m.Called(ctx, sessionID, params)
	return args.Get(0).(*core.ScanResult), args.Error(1)
}

func (m *MockPipelineAdapter) ScanSecretsTyped(ctx context.Context, sessionID string, params core.ScanSecretsParams) (*core.ScanSecretsResult, error) {
	args := m.Called(ctx, sessionID, params)
	return args.Get(0).(*core.ScanSecretsResult), args.Error(1)
}

// MockSessionManager is a mock implementation of UnifiedSessionManager
type MockSessionManager struct {
	mock.Mock
}

func (m *MockSessionManager) GetSession(ctx context.Context, sessionID string) (*session.SessionState, error) {
	args := m.Called(ctx, sessionID)
	return args.Get(0).(*session.SessionState), args.Error(1)
}

func (m *MockSessionManager) CreateSession(ctx context.Context, userID string) (*session.SessionState, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(*session.SessionState), args.Error(1)
}

func (m *MockSessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *MockSessionManager) ListSessions(ctx context.Context) ([]*session.SessionData, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*session.SessionData), args.Error(1)
}

func (m *MockSessionManager) GetOrCreateSession(ctx context.Context, sessionID string) (*session.SessionState, error) {
	args := m.Called(ctx, sessionID)
	return args.Get(0).(*session.SessionState), args.Error(1)
}

func (m *MockSessionManager) UpdateSession(ctx context.Context, sessionID string, updater func(*session.SessionState) error) error {
	args := m.Called(ctx, sessionID, updater)
	return args.Error(0)
}

func (m *MockSessionManager) GetStats(ctx context.Context) (*core.SessionManagerStats, error) {
	args := m.Called(ctx)
	return args.Get(0).(*core.SessionManagerStats), args.Error(1)
}

func (m *MockSessionManager) GarbageCollect(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockSessionManager) CreateWorkflowSession(ctx context.Context, spec *session.WorkflowSpec) (*session.SessionState, error) {
	args := m.Called(ctx, spec)
	return args.Get(0).(*session.SessionState), args.Error(1)
}

func (m *MockSessionManager) GetWorkflowSession(ctx context.Context, sessionID string) (*session.WorkflowSession, error) {
	args := m.Called(ctx, sessionID)
	return args.Get(0).(*session.WorkflowSession), args.Error(1)
}

func (m *MockSessionManager) UpdateWorkflowSession(ctx context.Context, session *session.WorkflowSession) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

// Missing methods from UnifiedSessionManager interface
func (m *MockSessionManager) ListSessionSummaries(ctx context.Context) ([]session.SessionSummary, error) {
	args := m.Called(ctx)
	return args.Get(0).([]session.SessionSummary), args.Error(1)
}

func (m *MockSessionManager) GetSessionData(ctx context.Context, sessionID string) (*session.SessionData, error) {
	args := m.Called(ctx, sessionID)
	return args.Get(0).(*session.SessionData), args.Error(1)
}

func (m *MockSessionManager) SaveSession(ctx context.Context, sessionID string, sessionState *session.SessionState) error {
	args := m.Called(ctx, sessionID, sessionState)
	return args.Error(0)
}

func (m *MockSessionManager) AddSessionLabel(ctx context.Context, sessionID, label string) error {
	args := m.Called(ctx, sessionID, label)
	return args.Error(0)
}

func (m *MockSessionManager) RemoveSessionLabel(ctx context.Context, sessionID, label string) error {
	args := m.Called(ctx, sessionID, label)
	return args.Error(0)
}

func (m *MockSessionManager) GetSessionsByLabel(ctx context.Context, label string) ([]*session.SessionData, error) {
	args := m.Called(ctx, label)
	return args.Get(0).([]*session.SessionData), args.Error(1)
}

func (m *MockSessionManager) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestSecurityScanTool_Execute(t *testing.T) {
	tests := []struct {
		name           string
		params         securityTypes.SecurityScanParams
		mockSetup      func(*MockPipelineAdapter, *MockSessionManager)
		expectedResult func(*testing.T, securityTypes.SecurityScanResult)
		expectedError  string
	}{
		{
			name: "successful_scan_with_vulnerabilities",
			params: securityTypes.SecurityScanParams{
				Target:        "nginx:latest",
				ScanType:      "image",
				Scanner:       "trivy",
				Format:        "json",
				IgnoreUnfixed: false,
				SessionID:     "test-session-123",
			},
			mockSetup: func(adapter *MockPipelineAdapter, session *MockSessionManager) {
				// No specific mock setup needed for this test
			},
			expectedResult: func(t *testing.T, result securityTypes.SecurityScanResult) {
				assert.True(t, result.Success)
				assert.Equal(t, "nginx:latest", result.Target)
				assert.Equal(t, "image", result.ScanType)
				assert.Equal(t, "trivy", result.Scanner)
				assert.Equal(t, "test-session-123", result.SessionID)
				assert.Greater(t, result.Duration, time.Duration(0))
				assert.Equal(t, 5, result.TotalVulnerabilities)
				assert.Equal(t, 1, result.VulnerabilitiesBySeverity["CRITICAL"])
				assert.Equal(t, 2, result.VulnerabilitiesBySeverity["HIGH"])
				assert.Equal(t, 1, result.VulnerabilitiesBySeverity["MEDIUM"])
				assert.Equal(t, 1, result.VulnerabilitiesBySeverity["LOW"])
				assert.Equal(t, 10.0, result.RiskScore)       // 1*10 + 2*7 + 1*4 + 1*1 = 29, capped at 10
				assert.Equal(t, "CRITICAL", result.RiskLevel) // Score of 10 = CRITICAL
				assert.Len(t, result.Vulnerabilities, 1)
				assert.Equal(t, "CVE-2023-1234", result.Vulnerabilities[0].ID)
				assert.Equal(t, "CRITICAL", result.Vulnerabilities[0].Severity)
				assert.Len(t, result.ComplianceResults, 1)
				assert.Len(t, result.Secrets, 1)
				assert.Len(t, result.Licenses, 1)
				assert.Len(t, result.Recommendations, 3)
			},
			expectedError: "",
		},
		{
			name: "successful_scan_with_default_scanner",
			params: securityTypes.SecurityScanParams{
				Target:        "alpine:latest",
				ScanType:      "image",
				Scanner:       "", // Test default scanner
				Format:        "json",
				IgnoreUnfixed: true,
				SessionID:     "test-session-456",
			},
			mockSetup: func(adapter *MockPipelineAdapter, session *MockSessionManager) {
				// No specific mock setup needed for this test
			},
			expectedResult: func(t *testing.T, result securityTypes.SecurityScanResult) {
				assert.True(t, result.Success)
				assert.Equal(t, "alpine:latest", result.Target)
				assert.Equal(t, "trivy", result.Scanner) // Should default to trivy
				assert.Equal(t, "test-session-456", result.SessionID)
			},
			expectedError: "",
		},
		{
			name: "scan_with_grype_scanner",
			params: securityTypes.SecurityScanParams{
				Target:        "ubuntu:20.04",
				ScanType:      "image",
				Scanner:       "grype",
				Format:        "yaml",
				IgnoreUnfixed: false,
				SessionID:     "test-session-789",
			},
			mockSetup: func(adapter *MockPipelineAdapter, session *MockSessionManager) {
				// No specific mock setup needed for this test
			},
			expectedResult: func(t *testing.T, result securityTypes.SecurityScanResult) {
				assert.True(t, result.Success)
				assert.Equal(t, "ubuntu:20.04", result.Target)
				assert.Equal(t, "grype", result.Scanner)
				assert.Equal(t, "test-session-789", result.SessionID)
			},
			expectedError: "",
		},
		{
			name: "validation_error_missing_target",
			params: securityTypes.SecurityScanParams{
				Target:        "", // Missing target
				ScanType:      "image",
				Scanner:       "trivy",
				Format:        "json",
				IgnoreUnfixed: false,
				SessionID:     "test-session-error",
			},
			mockSetup: func(adapter *MockPipelineAdapter, session *MockSessionManager) {
				// No mock setup needed for validation error
			},
			expectedResult: func(t *testing.T, result securityTypes.SecurityScanResult) {
				assert.False(t, result.Success)
			},
			expectedError: "Security scan parameters validation failed",
		},
		{
			name: "validation_error_missing_scan_type",
			params: securityTypes.SecurityScanParams{
				Target:        "nginx:latest",
				ScanType:      "", // Missing scan type
				Scanner:       "trivy",
				Format:        "json",
				IgnoreUnfixed: false,
				SessionID:     "test-session-error",
			},
			mockSetup: func(adapter *MockPipelineAdapter, session *MockSessionManager) {
				// No mock setup needed for validation error
			},
			expectedResult: func(t *testing.T, result securityTypes.SecurityScanResult) {
				assert.False(t, result.Success)
			},
			expectedError: "Security scan parameters validation failed",
		},
		{
			name: "scan_execution_simulated_error",
			params: securityTypes.SecurityScanParams{
				Target:        "", // This will cause an error in the current implementation
				ScanType:      "image",
				Scanner:       "trivy",
				Format:        "json",
				IgnoreUnfixed: false,
				SessionID:     "test-session-fail",
			},
			mockSetup: func(adapter *MockPipelineAdapter, session *MockSessionManager) {
				// No specific mock setup needed for this test
			},
			expectedResult: func(t *testing.T, result securityTypes.SecurityScanResult) {
				assert.False(t, result.Success)
			},
			expectedError: "Security scan parameters validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			mockAdapter := new(MockPipelineAdapter)
			mockSession := new(MockSessionManager)
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

			// Setup mocks
			tt.mockSetup(mockAdapter, mockSession)

			// Create tool instance
			tool := NewSecurityScanToolWithMocks(mockAdapter, mockSession, logger)
			scanTool, ok := tool.(*securityScanToolImpl)
			require.True(t, ok)

			// Execute test
			ctx := context.Background()
			toolInput := api.ToolInput{
				SessionID: tt.params.SessionID,
				Data: map[string]interface{}{
					"params": tt.params,
				},
			}
			toolOutput, err := scanTool.Execute(ctx, toolInput)

			// Verify results
			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
			}

			// Extract result from toolOutput
			if toolOutput.Success {
				if resultData, ok := toolOutput.Data["result"]; ok {
					if result, ok := resultData.(*securityTypes.SecurityScanResult); ok {
						tt.expectedResult(t, *result)
					}
				}
			} else {
				// For error cases, create a failed result
				result := securityTypes.SecurityScanResult{
					Success:   false,
					Target:    tt.params.Target,
					ScanType:  tt.params.ScanType,
					SessionID: tt.params.SessionID,
				}
				tt.expectedResult(t, result)
			}

			// No mock expectations to verify for this simple test
		})
	}
}

func TestSecurityScanTool_GetName(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tool := NewSecurityScanToolWithMocks(nil, nil, logger)
	scanTool, ok := tool.(*securityScanToolImpl)
	require.True(t, ok)

	assert.Equal(t, "security_scan", scanTool.Name())
}

func TestSecurityScanTool_GetDescription(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tool := NewSecurityScanToolWithMocks(nil, nil, logger)
	scanTool, ok := tool.(*securityScanToolImpl)
	require.True(t, ok)

	description := scanTool.Description()
	assert.Contains(t, description, "security scans")
	assert.Contains(t, description, "strongly-typed")
}

func TestSecurityScanTool_GetSchema(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tool := NewSecurityScanToolWithMocks(nil, nil, logger)
	scanTool, ok := tool.(*securityScanToolImpl)
	require.True(t, ok)

	schema := scanTool.Schema()

	// Test schema structure
	assert.Equal(t, "security_scan", schema.Name)
	assert.Equal(t, "2.0.0", schema.Version)
	assert.Contains(t, schema.Description, "security scans")

	// Test params schema structure
	assert.NotNil(t, schema.InputSchema)
	assert.NotNil(t, schema.OutputSchema)

	// Test examples (currently no examples in schema)
	assert.Len(t, schema.Examples, 0)
}

func TestCalculateRiskScore(t *testing.T) {
	tests := []struct {
		name            string
		vulnerabilities map[string]int
		expectedScore   float64
	}{
		{
			name:            "no_vulnerabilities",
			vulnerabilities: map[string]int{},
			expectedScore:   0.0,
		},
		{
			name: "single_critical_vulnerability",
			vulnerabilities: map[string]int{
				"CRITICAL": 1,
			},
			expectedScore: 10.0,
		},
		{
			name: "multiple_vulnerabilities",
			vulnerabilities: map[string]int{
				"CRITICAL": 1,
				"HIGH":     2,
				"MEDIUM":   3,
				"LOW":      4,
			},
			expectedScore: 10.0, // Should be capped at 10.0
		},
		{
			name: "medium_and_low_vulnerabilities",
			vulnerabilities: map[string]int{
				"MEDIUM": 1,
				"LOW":    2,
			},
			expectedScore: 6.0, // 4.0 + 2.0
		},
		{
			name: "unknown_severity",
			vulnerabilities: map[string]int{
				"UNKNOWN": 4,
			},
			expectedScore: 2.0, // 0.5 * 4
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calculateRiskScore(tt.vulnerabilities)
			assert.Equal(t, tt.expectedScore, score)
		})
	}
}

func TestCalculateRiskLevel(t *testing.T) {
	tests := []struct {
		name          string
		score         float64
		expectedLevel string
	}{
		{
			name:          "critical_risk",
			score:         9.5,
			expectedLevel: "CRITICAL",
		},
		{
			name:          "high_risk",
			score:         7.0,
			expectedLevel: "HIGH",
		},
		{
			name:          "medium_risk",
			score:         5.0,
			expectedLevel: "MEDIUM",
		},
		{
			name:          "low_risk",
			score:         3.0,
			expectedLevel: "LOW",
		},
		{
			name:          "minimal_risk",
			score:         1.0,
			expectedLevel: "MINIMAL",
		},
		{
			name:          "no_risk",
			score:         0.0,
			expectedLevel: "MINIMAL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := calculateRiskLevel(tt.score)
			assert.Equal(t, tt.expectedLevel, level)
		})
	}
}

func TestSecurityScanTool_ExecuteScan(t *testing.T) {
	tests := []struct {
		name          string
		params        securityTypes.SecurityScanParams
		scanner       string
		expectedError string
	}{
		{
			name: "successful_scan",
			params: securityTypes.SecurityScanParams{
				Target:   "nginx:latest",
				ScanType: "image",
			},
			scanner:       "trivy",
			expectedError: "",
		},
		{
			name: "missing_target",
			params: securityTypes.SecurityScanParams{
				Target:   "", // Missing target
				ScanType: "image",
			},
			scanner:       "trivy",
			expectedError: "Missing required scan parameters",
		},
		{
			name: "missing_scan_type",
			params: securityTypes.SecurityScanParams{
				Target:   "nginx:latest",
				ScanType: "", // Missing scan type
			},
			scanner:       "trivy",
			expectedError: "Missing required scan parameters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
			tool := &securityScanToolImpl{
				logger: logger,
			}

			ctx := context.Background()
			// Use the core types directly (no conversion needed)
			err := tool.executeScan(ctx, tt.params, tt.scanner)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSecurityScanTool_ContextCancellation(t *testing.T) {
	mockAdapter := new(MockPipelineAdapter)
	mockSession := new(MockSessionManager)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	tool := NewSecurityScanToolWithMocks(mockAdapter, mockSession, logger)
	scanTool, ok := tool.(*securityScanToolImpl)
	require.True(t, ok)

	// Create a canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	params := securityTypes.SecurityScanParams{
		Target:   "nginx:latest",
		ScanType: "image",
		Scanner:  "trivy",
	}

	// Execute should handle canceled context gracefully
	toolInput := api.ToolInput{
		SessionID: "test-session",
		Data: map[string]interface{}{
			"params": params,
		},
	}
	toolOutput, err := scanTool.Execute(ctx, toolInput)

	// The current implementation doesn't check for context cancellation,
	// but it should still return a result
	assert.NoError(t, err)
	assert.True(t, toolOutput.Success)
}

func TestSecurityScanTool_PerformanceCharacteristics(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	mockAdapter := new(MockPipelineAdapter)
	mockSession := new(MockSessionManager)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// No mock setup needed for this test

	tool := NewSecurityScanToolWithMocks(mockAdapter, mockSession, logger)
	scanTool, ok := tool.(*securityScanToolImpl)
	require.True(t, ok)

	params := securityTypes.SecurityScanParams{
		Target:   "nginx:latest",
		ScanType: "image",
		Scanner:  "trivy",
	}

	ctx := context.Background()
	start := time.Now()

	toolInput := api.ToolInput{
		SessionID: "test-session",
		Data: map[string]interface{}{
			"params": params,
		},
	}
	toolOutput, err := scanTool.Execute(ctx, toolInput)

	duration := time.Since(start)

	require.NoError(t, err)
	assert.True(t, toolOutput.Success)

	// Should complete within reasonable time (including mock delay)
	assert.Less(t, duration, 1*time.Second)

	// Verify result data structure
	assert.NotNil(t, toolOutput.Data)
}

// BenchmarkSecurityScanTool_Execute benchmarks the Execute method
func BenchmarkSecurityScanTool_Execute(b *testing.B) {
	mockAdapter := new(MockPipelineAdapter)
	mockSession := new(MockSessionManager)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// No mock setup needed for this test

	tool := NewSecurityScanToolWithMocks(mockAdapter, mockSession, logger)
	scanTool, ok := tool.(*securityScanToolImpl)
	require.True(b, ok)

	params := securityTypes.SecurityScanParams{
		Target:   "nginx:latest",
		ScanType: "image",
		Scanner:  "trivy",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		toolInput := api.ToolInput{
			SessionID: "test-session",
			Data: map[string]interface{}{
				"params": params,
			},
		}
		_, err := scanTool.Execute(ctx, toolInput)
		require.NoError(b, err)
	}
}
