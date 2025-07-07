package scan

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/mcp/domain/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAtomicScanSecretsArgs_Validate(t *testing.T) {
	tests := []struct {
		name  string
		args  AtomicScanSecretsArgs
		valid bool
	}{
		{
			name: "valid_minimal_args",
			args: AtomicScanSecretsArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "test-session",
				},
			},
			valid: true,
		},
		{
			name: "valid_full_args",
			args: AtomicScanSecretsArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "test-session",
					DryRun:    true,
				},
				ScanPath:           "/custom/path",
				FilePatterns:       []string{"*.py", "*.js", "*.yaml"},
				ExcludePatterns:    []string{"node_modules", ".git"},
				ScanDockerfiles:    true,
				ScanManifests:      true,
				ScanSourceCode:     true,
				ScanEnvFiles:       true,
				SuggestRemediation: true,
				GenerateSecrets:    true,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.Validate()

			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestAtomicScanSecretsArgs_GetSessionID(t *testing.T) {
	args := AtomicScanSecretsArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "test-session-123",
		},
	}

	sessionID := args.GetSessionID()
	assert.Equal(t, "test-session-123", sessionID)
}

func TestAtomicScanSecretsResult_IsSuccess(t *testing.T) {
	tests := []struct {
		name         string
		filesScanned int
		expected     bool
	}{
		{
			name:         "success_with_scanned_files",
			filesScanned: 5,
			expected:     true,
		},
		{
			name:         "success_with_one_file",
			filesScanned: 1,
			expected:     true,
		},
		{
			name:         "failure_no_files_scanned",
			filesScanned: 0,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AtomicScanSecretsResult{
				FilesScanned: tt.filesScanned,
			}

			success := result.IsSuccess()
			assert.Equal(t, tt.expected, success)
		})
	}
}

func TestAtomicScanSecretsTool_BasicFunctionality(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Test tool creation
	tool := newAtomicScanSecretsToolImpl(nil, nil, logger)
	require.NotNil(t, tool)

	// Create a temp directory for testing
	tempDir, err := os.MkdirTemp("", "basic-scan-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Test with minimal args that should not fail validation
	args := AtomicScanSecretsArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "test-session",
			DryRun:    false,
		},
		ScanPath: tempDir, // Use temp directory that we control
	}

	ctx := context.Background()
	result, err := tool.ExecuteScanSecrets(ctx, args)

	// We expect this to complete successfully, even if no secrets are found
	require.NoError(t, err)
	require.NotNil(t, result)

	// Basic result validation
	assert.Equal(t, "test-session", result.SessionID)
	assert.Equal(t, tempDir, result.ScanPath)
	assert.GreaterOrEqual(t, result.FilesScanned, 0)
	assert.GreaterOrEqual(t, result.SecretsFound, 0)
	assert.Greater(t, result.Duration, time.Duration(0))
	assert.GreaterOrEqual(t, result.SecurityScore, 0)
	assert.LessOrEqual(t, result.SecurityScore, 100)
	assert.Contains(t, []string{"low", "medium", "high", "critical"}, result.RiskLevel)
	assert.NotNil(t, result.ScanContext)
}

// TestSecretScanTool_WithTestFiles tests the tool with actual test files
func TestSecretScanTool_WithTestFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping file-based tests in short mode")
	}

	// Create temporary directory with test files
	tempDir, err := os.MkdirTemp("", "secret-scan-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test files with potential secrets
	secretFile := filepath.Join(tempDir, "config.env")
	err = os.WriteFile(secretFile, []byte(`
# Configuration file with secrets
API_KEY=sk-test123456789
DATABASE_URL=postgres://user:password@localhost/db
GITHUB_TOKEN=ghp_abcdef123456789

# Safe values
APP_NAME=test-app
PORT=8080
DEBUG=true
`), 0644)
	require.NoError(t, err)

	cleanFile := filepath.Join(tempDir, "clean.py")
	err = os.WriteFile(cleanFile, []byte(`
# Clean Python file with no secrets
import os

app_name = os.getenv("APP_NAME", "default")
port = int(os.getenv("PORT", "8080"))

def main():
    print(f"Starting {app_name} on port {port}")

if __name__ == "__main__":
    main()
`), 0644)
	require.NoError(t, err)

	// Create tool
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	tool := newAtomicScanSecretsToolImpl(nil, nil, logger)

	args := AtomicScanSecretsArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "file-test-session",
			DryRun:    false,
		},
		ScanPath:       tempDir,
		FilePatterns:   []string{"*.env", "*.py"},
		ScanSourceCode: true,
		ScanEnvFiles:   true,
	}

	ctx := context.Background()
	result, err := tool.ExecuteScanSecrets(ctx, args)

	// Verify results
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.IsSuccess())
	assert.Equal(t, "file-test-session", result.SessionID)
	assert.Equal(t, tempDir, result.ScanPath)
	assert.Greater(t, result.FilesScanned, 0)
	assert.Greater(t, result.Duration, time.Duration(0))

	// Should scan both files
	assert.GreaterOrEqual(t, result.FilesScanned, 2)

	// Results should be consistent
	assert.GreaterOrEqual(t, result.SecurityScore, 0)
	assert.LessOrEqual(t, result.SecurityScore, 100)

	t.Logf("Test completed: Files=%d, Secrets=%d, Score=%d, Risk=%s",
		result.FilesScanned, result.SecretsFound, result.SecurityScore, result.RiskLevel)
}

func TestScannedSecret_Structure(t *testing.T) {
	secret := ScannedSecret{
		File:       "/test/file.py",
		Line:       10,
		Type:       "api_key",
		Pattern:    "API_KEY",
		Value:      "sk-test123",
		Severity:   "HIGH",
		Context:    "API_KEY = 'sk-test123'",
		Confidence: 95,
	}

	assert.Equal(t, "/test/file.py", secret.File)
	assert.Equal(t, 10, secret.Line)
	assert.Equal(t, "api_key", secret.Type)
	assert.Equal(t, "API_KEY", secret.Pattern)
	assert.Equal(t, "sk-test123", secret.Value)
	assert.Equal(t, "HIGH", secret.Severity)
	assert.Equal(t, "API_KEY = 'sk-test123'", secret.Context)
	assert.Equal(t, 95, secret.Confidence)
}

func TestFileSecretScanResult_Structure(t *testing.T) {
	result := FileSecretScanResult{
		FilePath:     "/test/config.py",
		FileType:     "python",
		SecretsFound: 2,
		Secrets: []ScannedSecret{
			{
				File:     "/test/config.py",
				Line:     5,
				Type:     "api_key",
				Severity: "HIGH",
			},
			{
				File:     "/test/config.py",
				Line:     10,
				Type:     "password",
				Severity: "MEDIUM",
			},
		},
		CleanStatus: "secrets_detected",
	}

	assert.Equal(t, "/test/config.py", result.FilePath)
	assert.Equal(t, "python", result.FileType)
	assert.Equal(t, 2, result.SecretsFound)
	assert.Len(t, result.Secrets, 2)
	assert.Equal(t, "secrets_detected", result.CleanStatus)
	assert.Equal(t, "HIGH", result.Secrets[0].Severity)
	assert.Equal(t, "MEDIUM", result.Secrets[1].Severity)
}

// BenchmarkSecretScanTool_BasicOperation benchmarks basic secret scanning
func BenchmarkSecretScanTool_BasicOperation(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	tool := newAtomicScanSecretsToolImpl(nil, nil, logger)

	// Create a temp directory for benchmarking
	tempDir, err := os.MkdirTemp("", "benchmark-scan-test")
	require.NoError(b, err)
	defer os.RemoveAll(tempDir)

	args := AtomicScanSecretsArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "benchmark-session",
			DryRun:    false,
		},
		ScanPath: tempDir, // Use temp directory for benchmarking
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := tool.ExecuteScanSecrets(ctx, args)
		require.NoError(b, err)
	}
}
