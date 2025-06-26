package scan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock types and interfaces
type mockPipelineAdapter struct {
	sessionWorkspace string
}

func (m *mockPipelineAdapter) GetSessionWorkspace(sessionID string) string {
	if m.sessionWorkspace != "" {
		return m.sessionWorkspace
	}
	return "/tmp/test-workspace"
}

func (m *mockPipelineAdapter) UpdateSessionFromDockerResults(sessionID string, result interface{}) error {
	return nil
}

func (m *mockPipelineAdapter) BuildDockerImage(sessionID, imageRef, dockerfilePath string) (*mcptypes.BuildResult, error) {
	return nil, nil
}

func (m *mockPipelineAdapter) PullDockerImage(sessionID, imageRef string) error {
	return nil
}

func (m *mockPipelineAdapter) PushDockerImage(sessionID, imageRef string) error {
	return nil
}

func (m *mockPipelineAdapter) TagDockerImage(sessionID, sourceRef, targetRef string) error {
	return nil
}

func (m *mockPipelineAdapter) ConvertToDockerState(sessionID string) (*mcptypes.DockerState, error) {
	return nil, nil
}

func (m *mockPipelineAdapter) GenerateKubernetesManifests(sessionID, imageRef, appName string, port int, cpuRequest, memoryRequest, cpuLimit, memoryLimit string) (*mcptypes.KubernetesManifestResult, error) {
	return nil, nil
}

func (m *mockPipelineAdapter) DeployToKubernetes(sessionID string, manifests []string) (*mcptypes.KubernetesDeploymentResult, error) {
	return nil, nil
}

func (m *mockPipelineAdapter) CheckApplicationHealth(sessionID, namespace, deploymentName string, timeout time.Duration) (*mcptypes.HealthCheckResult, error) {
	return nil, nil
}

func (m *mockPipelineAdapter) AcquireResource(sessionID, resourceType string) error {
	return nil
}

func (m *mockPipelineAdapter) ReleaseResource(sessionID, resourceType string) error {
	return nil
}

type mockSessionManager struct {
	sessions map[string]*session.SessionState
	err      error
}

func (m *mockSessionManager) GetSession(sessionID string) (interface{}, error) {
	if m.err != nil {
		return nil, m.err
	}
	if sess, ok := m.sessions[sessionID]; ok {
		return sess, nil
	}
	return &session.SessionState{SessionID: sessionID}, nil
}

func (m *mockSessionManager) GetSessionInterface(sessionID string) (interface{}, error) {
	return m.GetSession(sessionID)
}

func (m *mockSessionManager) GetOrCreateSession(sessionID string) (interface{}, error) {
	return m.GetSession(sessionID)
}

func (m *mockSessionManager) GetOrCreateSessionFromRepo(repoURL string) (interface{}, error) {
	return &session.SessionState{SessionID: "test-session"}, nil
}

func (m *mockSessionManager) UpdateSession(sessionID string, updateFunc func(interface{})) error {
	sess, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}
	updateFunc(sess)
	return nil
}

func (m *mockSessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	delete(m.sessions, sessionID)
	return nil
}

func (m *mockSessionManager) ListSessions(ctx context.Context, filter map[string]interface{}) ([]interface{}, error) {
	var sessions []interface{}
	for _, sess := range m.sessions {
		sessions = append(sessions, sess)
	}
	return sessions, nil
}

func (m *mockSessionManager) FindSessionByRepo(ctx context.Context, repoURL string) (interface{}, error) {
	return &session.SessionState{SessionID: "test-session"}, nil
}

func TestNewAtomicScanSecretsTool(t *testing.T) {
	adapter := &mockPipelineAdapter{}
	sessionManager := &mockSessionManager{sessions: make(map[string]*session.SessionState)}
	logger := zerolog.New(nil)

	tool := NewAtomicScanSecretsTool(adapter, sessionManager, logger)

	assert.NotNil(t, tool)
	assert.Equal(t, adapter, tool.pipelineAdapter)
	assert.Equal(t, sessionManager, tool.sessionManager)
}

func TestAtomicScanSecretsTool_GetMetadata(t *testing.T) {
	tool := &AtomicScanSecretsTool{logger: zerolog.New(nil)}
	metadata := tool.GetMetadata()

	assert.Equal(t, "atomic_scan_secrets", metadata.Name)
	assert.Contains(t, metadata.Description, "secret")
	assert.Equal(t, "1.0.0", metadata.Version)
	assert.Equal(t, "security", metadata.Category)
	
	assert.Contains(t, metadata.Capabilities, "secret_detection")
	assert.Contains(t, metadata.Capabilities, "pattern_matching")
	assert.Contains(t, metadata.Capabilities, "kubernetes_secret_generation")
	
	assert.Contains(t, metadata.Requirements, "valid_session_id")
	assert.Contains(t, metadata.Requirements, "file_system_access")
	
	assert.Len(t, metadata.Examples, 3)
}

func TestAtomicScanSecretsTool_GetName(t *testing.T) {
	tool := &AtomicScanSecretsTool{}
	assert.Equal(t, "atomic_scan_secrets", tool.GetName())
}

func TestAtomicScanSecretsTool_GetDescription(t *testing.T) {
	tool := &AtomicScanSecretsTool{}
	description := tool.GetDescription()
	assert.Contains(t, description, "secret")
	assert.Contains(t, description, "credentials")
}

func TestAtomicScanSecretsTool_GetVersion(t *testing.T) {
	tool := &AtomicScanSecretsTool{}
	assert.Equal(t, "1.0.0", tool.GetVersion())
}

func TestAtomicScanSecretsTool_GetCapabilities(t *testing.T) {
	tool := &AtomicScanSecretsTool{}
	capabilities := tool.GetCapabilities()
	
	assert.True(t, capabilities.SupportsDryRun)
	assert.True(t, capabilities.SupportsStreaming)
	assert.True(t, capabilities.IsLongRunning)
	assert.False(t, capabilities.RequiresAuth)
}

func TestAtomicScanSecretsTool_Validate(t *testing.T) {
	tool := &AtomicScanSecretsTool{logger: zerolog.New(nil)}

	tests := []struct {
		name        string
		args        interface{}
		expectedErr string
	}{
		{
			name: "valid args",
			args: AtomicScanSecretsArgs{
				BaseToolArgs: types.BaseToolArgs{SessionID: "test-session"},
				ScanPath:     "/tmp/test",
			},
		},
		{
			name:        "invalid args type",
			args:        "invalid",
			expectedErr: "Invalid argument type",
		},
		{
			name: "missing session ID",
			args: AtomicScanSecretsArgs{
				BaseToolArgs: types.BaseToolArgs{SessionID: ""},
			},
			expectedErr: "SessionID is required",
		},
		{
			name: "invalid file pattern",
			args: AtomicScanSecretsArgs{
				BaseToolArgs: types.BaseToolArgs{SessionID: "test-session"},
				FilePatterns: []string{"[invalid"},
			},
			expectedErr: "Invalid file pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tool.Validate(context.Background(), tt.args)
			
			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAtomicScanSecretsTool_Execute(t *testing.T) {
	tests := []struct {
		name        string
		args        interface{}
		setupTool   func() *AtomicScanSecretsTool
		expectedErr string
	}{
		{
			name: "valid args",
			args: AtomicScanSecretsArgs{
				BaseToolArgs: types.BaseToolArgs{SessionID: "test-session"},
			},
			setupTool: func() *AtomicScanSecretsTool {
				// Create a temp directory that exists
				tempDir, err := os.MkdirTemp("", "scan-test")
				require.NoError(t, err)
				adapter := &mockPipelineAdapter{sessionWorkspace: tempDir}
				sessionManager := &mockSessionManager{
					sessions: map[string]*session.SessionState{
						"test-session": {SessionID: "test-session"},
					},
				}
				return NewAtomicScanSecretsTool(adapter, sessionManager, zerolog.New(nil))
			},
			expectedErr: "", // Should succeed with empty scan
		},
		{
			name:        "invalid args type",
			args:        "invalid",
			setupTool:   func() *AtomicScanSecretsTool { return &AtomicScanSecretsTool{logger: zerolog.New(nil)} },
			expectedErr: "Invalid argument type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := tt.setupTool()
			result, err := tool.Execute(context.Background(), tt.args)
			
			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestAtomicScanSecretsTool_executeWithoutProgress(t *testing.T) {
	// Create a temporary directory with test files
	tempDir, err := os.MkdirTemp("", "secret-scan-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test files with secrets
	testFiles := map[string]string{
		"config.yaml": `
database:
  password: "super-secret-password"
  api_key: "sk-1234567890abcdef"
`,
		"app.py": `
import os
API_KEY = "test-api-key-123456"
PASSWORD = "hardcoded-password"
`,
		".env": `
DB_PASSWORD=secret123
JWT_SECRET=my-jwt-secret-key
`,
	}

	for filename, content := range testFiles {
		filePath := filepath.Join(tempDir, filename)
		err := os.WriteFile(filePath, []byte(content), 0644)
		require.NoError(t, err)
	}

	// Setup test tool
	adapter := &mockPipelineAdapter{sessionWorkspace: tempDir}
	sessionManager := &mockSessionManager{
		sessions: map[string]*session.SessionState{
			"test-session": {SessionID: "test-session"},
		},
	}
	tool := NewAtomicScanSecretsTool(adapter, sessionManager, zerolog.New(nil))

	args := AtomicScanSecretsArgs{
		BaseToolArgs:       types.BaseToolArgs{SessionID: "test-session"},
		ScanSourceCode:     true,
		ScanEnvFiles:       true,
		ScanManifests:      true,
		SuggestRemediation: true,
	}

	result, err := tool.executeWithoutProgress(context.Background(), args, time.Now())
	
	assert.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "test-session", result.SessionID)
	assert.Equal(t, tempDir, result.ScanPath)
	assert.True(t, result.FilesScanned > 0)
	assert.True(t, result.SecretsFound > 0)
	assert.NotEmpty(t, result.RiskLevel)
	assert.True(t, result.SecurityScore >= 0 && result.SecurityScore <= 100)
	assert.NotEmpty(t, result.Recommendations)
	assert.NotNil(t, result.RemediationPlan)
}

func TestStandardSecretScanStages(t *testing.T) {
	stages := standardSecretScanStages()
	
	assert.Len(t, stages, 5)
	assert.Equal(t, "Initialize", stages[0].Name)
	assert.Equal(t, "Analyze", stages[1].Name)
	assert.Equal(t, "Scan", stages[2].Name)
	assert.Equal(t, "Process", stages[3].Name)
	assert.Equal(t, "Finalize", stages[4].Name)
	
	// Check weights sum to 1.0
	totalWeight := 0.0
	for _, stage := range stages {
		totalWeight += stage.Weight
	}
	assert.InDelta(t, 1.0, totalWeight, 0.01)
}

func TestAtomicScanSecretsTool_getDefaultFilePatterns(t *testing.T) {
	tool := &AtomicScanSecretsTool{logger: zerolog.New(nil)}

	tests := []struct {
		name     string
		args     AtomicScanSecretsArgs
		expected []string
	}{
		{
			name: "scan dockerfiles",
			args: AtomicScanSecretsArgs{ScanDockerfiles: true},
			expected: []string{"Dockerfile*", "*.dockerfile"},
		},
		{
			name: "scan manifests",
			args: AtomicScanSecretsArgs{ScanManifests: true},
			expected: []string{"*.yaml", "*.yml", "*.json"},
		},
		{
			name: "scan env files",
			args: AtomicScanSecretsArgs{ScanEnvFiles: true},
			expected: []string{".env*", "*.env"},
		},
		{
			name: "scan source code",
			args: AtomicScanSecretsArgs{ScanSourceCode: true},
			expected: []string{"*.py", "*.js", "*.ts", "*.go", "*.java", "*.cs", "*.php", "*.rb"},
		},
		{
			name: "no specific options - defaults",
			args: AtomicScanSecretsArgs{},
			expected: []string{"*.yaml", "*.yml", "*.json", ".env*", "*.env", "Dockerfile*"},
		},
		{
			name: "multiple options",
			args: AtomicScanSecretsArgs{
				ScanDockerfiles: true,
				ScanEnvFiles:   true,
			},
			expected: []string{"Dockerfile*", "*.dockerfile", ".env*", "*.env"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns := tool.getDefaultFilePatterns(tt.args)
			assert.Equal(t, tt.expected, patterns)
		})
	}
}

func TestAtomicScanSecretsTool_shouldScanFile(t *testing.T) {
	tool := &AtomicScanSecretsTool{logger: zerolog.New(nil)}

	tests := []struct {
		name            string
		filePath        string
		includePatterns []string
		excludePatterns []string
		expected        bool
	}{
		{
			name:            "include yaml file",
			filePath:        "/path/to/config.yaml",
			includePatterns: []string{"*.yaml", "*.yml"},
			excludePatterns: []string{},
			expected:        true,
		},
		{
			name:            "exclude log file",
			filePath:        "/path/to/app.log",
			includePatterns: []string{"*"},
			excludePatterns: []string{"*.log"},
			expected:        false,
		},
		{
			name:            "include python file",
			filePath:        "/path/to/script.py",
			includePatterns: []string{"*.py"},
			excludePatterns: []string{},
			expected:        true,
		},
		{
			name:            "exclude takes precedence",
			filePath:        "/path/to/test.py",
			includePatterns: []string{"*.py"},
			excludePatterns: []string{"test.*"},
			expected:        false,
		},
		{
			name:            "no match in include patterns",
			filePath:        "/path/to/binary.exe",
			includePatterns: []string{"*.py", "*.js"},
			excludePatterns: []string{},
			expected:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tool.shouldScanFile(tt.filePath, tt.includePatterns, tt.excludePatterns)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAtomicScanSecretsTool_getFileType(t *testing.T) {
	tool := &AtomicScanSecretsTool{logger: zerolog.New(nil)}

	tests := []struct {
		path     string
		expected string
	}{
		{"/path/to/Dockerfile", "dockerfile"},
		{"/path/to/Dockerfile.dev", "dockerfile"},
		{"/path/to/config.yaml", "yaml"},
		{"/path/to/config.yml", "yaml"},
		{"/path/to/data.json", types.LanguageJSON},
		{"/path/to/.env", "env"},
		{"/path/to/script.py", types.LanguagePython},
		{"/path/to/app.js", types.LanguageJavaScript},
		{"/path/to/app.ts", types.LanguageJavaScript},
		{"/path/to/main.go", "go"},
		{"/path/to/App.java", types.LanguageJava},
		{"/path/to/binary.exe", "other"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := tool.getFileType(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAtomicScanSecretsTool_determineCleanStatus(t *testing.T) {
	tool := &AtomicScanSecretsTool{logger: zerolog.New(nil)}

	tests := []struct {
		name     string
		secrets  []ScannedSecret
		expected string
	}{
		{
			name:     "no secrets",
			secrets:  []ScannedSecret{},
			expected: "clean",
		},
		{
			name: "critical severity",
			secrets: []ScannedSecret{
				{Severity: "critical"},
			},
			expected: "critical",
		},
		{
			name: "high severity",
			secrets: []ScannedSecret{
				{Severity: "high"},
			},
			expected: "critical",
		},
		{
			name: "medium severity only",
			secrets: []ScannedSecret{
				{Severity: "medium"},
			},
			expected: "issues",
		},
		{
			name: "low severity only",
			secrets: []ScannedSecret{
				{Severity: "low"},
			},
			expected: "issues",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tool.determineCleanStatus(tt.secrets)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAtomicScanSecretsTool_classifySecretType(t *testing.T) {
	tool := &AtomicScanSecretsTool{logger: zerolog.New(nil)}

	tests := []struct {
		pattern  string
		expected string
	}{
		{"PASSWORD", "password"},
		{"API_KEY", "api_key"},
		{"ACCESS_TOKEN", "token"},
		{"SECRET_VALUE", "secret"},
		{"UNKNOWN_PATTERN", "sensitive"},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			result := tool.classifySecretType(tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAtomicScanSecretsTool_determineSeverity(t *testing.T) {
	tool := &AtomicScanSecretsTool{logger: zerolog.New(nil)}

	tests := []struct {
		pattern  string
		value    string
		expected string
	}{
		{"API_KEY", "sk-1234567890abcdef1234567890", "critical"}, // Long key-like value
		{"TOKEN", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9", "critical"}, // Long token-like value
		{"PASSWORD", "short", "high"},
		{"SECRET", "value", "high"},
		{"OTHER", "value", "medium"},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"/"+tt.value, func(t *testing.T) {
			result := tool.determineSeverity(tt.pattern, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAtomicScanSecretsTool_calculateConfidence(t *testing.T) {
	tool := &AtomicScanSecretsTool{logger: zerolog.New(nil)}

	tests := []struct {
		pattern  string
		expected int
	}{
		{"PASSWORD", 90},
		{"API_KEY", 85},
		{"TOKEN", 85},
		{"OTHER", 70},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			result := tool.calculateConfidence(tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAtomicScanSecretsTool_calculateSeverityBreakdown(t *testing.T) {
	tool := &AtomicScanSecretsTool{logger: zerolog.New(nil)}

	secrets := []ScannedSecret{
		{Severity: "critical"},
		{Severity: "critical"},
		{Severity: "high"},
		{Severity: "medium"},
		{Severity: "medium"},
		{Severity: "medium"},
		{Severity: "low"},
	}

	breakdown := tool.calculateSeverityBreakdown(secrets)

	assert.Equal(t, 2, breakdown["critical"])
	assert.Equal(t, 1, breakdown["high"])
	assert.Equal(t, 3, breakdown["medium"])
	assert.Equal(t, 1, breakdown["low"])
}

func TestAtomicScanSecretsTool_calculateSecurityScore(t *testing.T) {
	tool := &AtomicScanSecretsTool{logger: zerolog.New(nil)}

	tests := []struct {
		name     string
		secrets  []ScannedSecret
		expected int
	}{
		{
			name:     "no secrets",
			secrets:  []ScannedSecret{},
			expected: 100,
		},
		{
			name: "one critical",
			secrets: []ScannedSecret{
				{Severity: "critical"},
			},
			expected: 75, // 100 - 25
		},
		{
			name: "one high",
			secrets: []ScannedSecret{
				{Severity: "high"},
			},
			expected: 85, // 100 - 15
		},
		{
			name: "mixed severities",
			secrets: []ScannedSecret{
				{Severity: "critical"}, // -25
				{Severity: "high"},     // -15
				{Severity: "medium"},   // -8
				{Severity: "low"},      // -3
			},
			expected: 49, // 100 - 25 - 15 - 8 - 3 = 49
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tool.calculateSecurityScore(tt.secrets)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAtomicScanSecretsTool_determineRiskLevel(t *testing.T) {
	tool := &AtomicScanSecretsTool{logger: zerolog.New(nil)}

	tests := []struct {
		score    int
		expected string
	}{
		{90, "low"},
		{80, "low"},
		{75, "medium"},
		{60, "medium"},
		{45, "high"},
		{30, "high"},
		{15, "critical"},
		{0, "critical"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("score_%d", tt.score), func(t *testing.T) {
			result := tool.determineRiskLevel(tt.score, []ScannedSecret{})
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAtomicScanSecretsTool_generateRecommendations(t *testing.T) {
	tool := &AtomicScanSecretsTool{logger: zerolog.New(nil)}

	tests := []struct {
		name     string
		secrets  []ScannedSecret
		args     AtomicScanSecretsArgs
		contains []string
	}{
		{
			name:    "no secrets",
			secrets: []ScannedSecret{},
			args:    AtomicScanSecretsArgs{},
			contains: []string{
				"No secrets detected",
			},
		},
		{
			name: "has critical secrets",
			secrets: []ScannedSecret{
				{Severity: "critical", Type: "api_key"},
			},
			args: AtomicScanSecretsArgs{},
			contains: []string{
				"Remove hardcoded secrets",
				"URGENT: Critical secrets detected",
			},
		},
		{
			name: "has passwords",
			secrets: []ScannedSecret{
				{Severity: "high", Type: "password"},
			},
			args: AtomicScanSecretsArgs{},
			contains: []string{
				"Replace hardcoded passwords",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recommendations := tool.generateRecommendations(tt.secrets, tt.args)
			
			for _, expected := range tt.contains {
				found := false
				for _, rec := range recommendations {
					if strings.Contains(rec, expected) {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected to find recommendation containing: %s", expected)
			}
		})
	}
}

func TestAtomicScanSecretsTool_generateRemediationPlan(t *testing.T) {
	tool := &AtomicScanSecretsTool{logger: zerolog.New(nil)}

	secrets := []ScannedSecret{
		{Type: "api_key", File: "config.py", Line: 1},
		{Type: "password", File: "config.py", Line: 2},
	}

	plan := tool.generateRemediationPlan(secrets)

	assert.NotNil(t, plan)
	assert.Equal(t, "kubernetes-secrets", plan.PreferredManager)
	assert.NotEmpty(t, plan.ImmediateActions)
	assert.NotEmpty(t, plan.MigrationSteps)
	assert.NotEmpty(t, plan.SecretReferences)
	
	// Check that immediate actions include critical steps
	found := false
	for _, action := range plan.ImmediateActions {
		if assert.Contains(t, action, "Stop committing") {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestSecretScanningTypes(t *testing.T) {
	// Test the various types work as expected
	secret := ScannedSecret{
		File:       "test.py",
		Line:       10,
		Type:       "api_key",
		Pattern:    "API_KEY",
		Value:      "redacted",
		Severity:   "critical",
		Context:    "API_KEY = 'secret'",
		Confidence: 85,
	}

	assert.Equal(t, "test.py", secret.File)
	assert.Equal(t, 10, secret.Line)
	assert.Equal(t, "api_key", secret.Type)

	fileResult := FileSecretScanResult{
		FilePath:     "test.py",
		FileType:     "python",
		SecretsFound: 1,
		Secrets:      []ScannedSecret{secret},
		CleanStatus:  "critical",
	}

	assert.Equal(t, 1, fileResult.SecretsFound)
	assert.Len(t, fileResult.Secrets, 1)

	ref := SecretReference{
		SecretName:     "app-secrets",
		SecretKey:      "api-key",
		OriginalEnvVar: "API_KEY",
		KubernetesRef:  "secretKeyRef: {name: app-secrets, key: api-key}",
	}

	assert.Equal(t, "app-secrets", ref.SecretName)
	assert.Equal(t, "api-key", ref.SecretKey)
}