package scan

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFileSecretScanner(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewFileSecretScanner(logger)

	require.NotNil(t, scanner)
	assert.NotNil(t, scanner.logger)
}

func TestFileSecretScanner_GetDefaultFilePatterns(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewFileSecretScanner(logger)

	tests := []struct {
		name     string
		args     AtomicScanSecretsArgs
		expected []string
	}{
		{
			name: "all_scan_types_enabled",
			args: AtomicScanSecretsArgs{
				ScanDockerfiles: true,
				ScanManifests:   true,
				ScanEnvFiles:    true,
				ScanSourceCode:  true,
			},
			expected: []string{
				"Dockerfile*", "*.dockerfile",
				"*.yaml", "*.yml", "*.json",
				".env*", "*.env",
				"*.py", "*.js", "*.ts", "*.go", "*.java", "*.cs", "*.php", "*.rb",
			},
		},
		{
			name: "only_dockerfiles",
			args: AtomicScanSecretsArgs{
				ScanDockerfiles: true,
			},
			expected: []string{"Dockerfile*", "*.dockerfile"},
		},
		{
			name: "only_manifests",
			args: AtomicScanSecretsArgs{
				ScanManifests: true,
			},
			expected: []string{"*.yaml", "*.yml", "*.json"},
		},
		{
			name: "only_env_files",
			args: AtomicScanSecretsArgs{
				ScanEnvFiles: true,
			},
			expected: []string{".env*", "*.env"},
		},
		{
			name: "only_source_code",
			args: AtomicScanSecretsArgs{
				ScanSourceCode: true,
			},
			expected: []string{"*.py", "*.js", "*.ts", "*.go", "*.java", "*.cs", "*.php", "*.rb"},
		},
		{
			name:     "no_options_default",
			args:     AtomicScanSecretsArgs{},
			expected: []string{"*.yaml", "*.yml", "*.json", ".env*", "*.env", "Dockerfile*"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns := scanner.GetDefaultFilePatterns(tt.args)

			// Check that all expected patterns are present
			for _, expected := range tt.expected {
				assert.Contains(t, patterns, expected)
			}
		})
	}
}

func TestFileSecretScanner_shouldScanFile(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewFileSecretScanner(logger)

	tests := []struct {
		name            string
		filepath        string
		includePatterns []string
		excludePatterns []string
		expected        bool
	}{
		{
			name:            "dockerfile_match",
			filepath:        "/path/to/Dockerfile",
			includePatterns: []string{"Dockerfile*"},
			excludePatterns: []string{},
			expected:        true,
		},
		{
			name:            "python_file_match",
			filepath:        "/path/to/script.py",
			includePatterns: []string{"*.py"},
			excludePatterns: []string{},
			expected:        true,
		},
		{
			name:            "excluded_file",
			filepath:        "/path/to/test.py",
			includePatterns: []string{"*.py"},
			excludePatterns: []string{"test*"},
			expected:        false,
		},
		{
			name:            "no_match",
			filepath:        "/path/to/random.txt",
			includePatterns: []string{"*.py", "*.js"},
			excludePatterns: []string{},
			expected:        false,
		},
		{
			name:            "env_file_match",
			filepath:        "/path/to/.env",
			includePatterns: []string{".env*"},
			excludePatterns: []string{},
			expected:        true,
		},
		{
			name:            "env_suffix_match",
			filepath:        "/path/to/config.env",
			includePatterns: []string{"*.env"},
			excludePatterns: []string{},
			expected:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scanner.shouldScanFile(tt.filepath, tt.includePatterns, tt.excludePatterns)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFileSecretScanner_getFileType(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewFileSecretScanner(logger)

	tests := []struct {
		name     string
		filepath string
		expected string
	}{
		{
			name:     "dockerfile",
			filepath: "/path/to/Dockerfile",
			expected: "dockerfile",
		},
		{
			name:     "dockerfile_with_suffix",
			filepath: "/path/to/Dockerfile.dev",
			expected: "dockerfile",
		},
		{
			name:     "env_file",
			filepath: "/path/to/.env",
			expected: "environment",
		},
		{
			name:     "env_file_with_suffix",
			filepath: "/path/to/.env.local",
			expected: "environment",
		},
		{
			name:     "python_file",
			filepath: "/path/to/script.py",
			expected: "python",
		},
		{
			name:     "javascript_file",
			filepath: "/path/to/script.js",
			expected: "javascript",
		},
		{
			name:     "typescript_file",
			filepath: "/path/to/script.ts",
			expected: "javascript",
		},
		{
			name:     "go_file",
			filepath: "/path/to/main.go",
			expected: "go",
		},
		{
			name:     "yaml_file",
			filepath: "/path/to/config.yaml",
			expected: "kubernetes",
		},
		{
			name:     "yml_file",
			filepath: "/path/to/config.yml",
			expected: "kubernetes",
		},
		{
			name:     "json_file",
			filepath: "/path/to/config.json",
			expected: "json",
		},
		{
			name:     "java_file",
			filepath: "/path/to/Main.java",
			expected: "java",
		},
		{
			name:     "csharp_file",
			filepath: "/path/to/Program.cs",
			expected: "csharp",
		},
		{
			name:     "php_file",
			filepath: "/path/to/index.php",
			expected: "php",
		},
		{
			name:     "ruby_file",
			filepath: "/path/to/script.rb",
			expected: "ruby",
		},
		{
			name:     "unknown_file",
			filepath: "/path/to/readme.txt",
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scanner.getFileType(tt.filepath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFileSecretScanner_determineCleanStatus(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewFileSecretScanner(logger)

	tests := []struct {
		name     string
		secrets  []ScannedSecret
		expected string
	}{
		{
			name:     "no_secrets",
			secrets:  []ScannedSecret{},
			expected: "clean",
		},
		{
			name: "critical_secrets",
			secrets: []ScannedSecret{
				{Severity: "critical"},
			},
			expected: "critical",
		},
		{
			name: "high_secrets",
			secrets: []ScannedSecret{
				{Severity: "high"},
			},
			expected: "critical",
		},
		{
			name: "medium_secrets",
			secrets: []ScannedSecret{
				{Severity: "medium"},
			},
			expected: "warning",
		},
		{
			name: "low_secrets",
			secrets: []ScannedSecret{
				{Severity: "low"},
			},
			expected: "minor",
		},
		{
			name: "mixed_secrets_with_high",
			secrets: []ScannedSecret{
				{Severity: "high"},
				{Severity: "medium"},
				{Severity: "low"},
			},
			expected: "critical",
		},
		{
			name: "mixed_secrets_no_high",
			secrets: []ScannedSecret{
				{Severity: "medium"},
				{Severity: "low"},
			},
			expected: "warning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scanner.determineCleanStatus(tt.secrets)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFileSecretScanner_classifySecretType(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewFileSecretScanner(logger)

	tests := []struct {
		name     string
		pattern  string
		expected string
	}{
		{
			name:     "api_key",
			pattern:  "api_key",
			expected: "api_key",
		},
		{
			name:     "api_key_uppercase",
			pattern:  "API_KEY",
			expected: "api_key",
		},
		{
			name:     "password",
			pattern:  "password",
			expected: "password",
		},
		{
			name:     "token",
			pattern:  "token",
			expected: "token",
		},
		{
			name:     "secret",
			pattern:  "secret",
			expected: "secret",
		},
		{
			name:     "certificate",
			pattern:  "certificate",
			expected: "certificate",
		},
		{
			name:     "cert",
			pattern:  "cert",
			expected: "certificate",
		},
		{
			name:     "private_key",
			pattern:  "private_key",
			expected: "private_key",
		},
		{
			name:     "database_credential",
			pattern:  "database_url",
			expected: "database_credential",
		},
		{
			name:     "db_credential",
			pattern:  "db_password",
			expected: "database_credential",
		},
		{
			name:     "unknown",
			pattern:  "random_pattern",
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scanner.classifySecretType(tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFileSecretScanner_determineSeverity(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewFileSecretScanner(logger)

	tests := []struct {
		name     string
		pattern  string
		value    string
		expected string
	}{
		{
			name:     "production_critical",
			pattern:  "prod_api_key",
			value:    "sk-123456",
			expected: "critical",
		},
		{
			name:     "production_critical_uppercase",
			pattern:  "PRODUCTION_TOKEN",
			value:    "token-456",
			expected: "critical",
		},
		{
			name:     "private_key_critical",
			pattern:  "private_key",
			value:    "-----BEGIN PRIVATE KEY-----",
			expected: "critical",
		},
		{
			name:     "root_credential_critical",
			pattern:  "root_password",
			value:    "password123",
			expected: "critical",
		},
		{
			name:     "admin_credential_critical",
			pattern:  "admin_token",
			value:    "admin-token-123",
			expected: "critical",
		},
		{
			name:     "api_key_high",
			pattern:  "api_key",
			value:    "ak-123456",
			expected: "high",
		},
		{
			name:     "token_high",
			pattern:  "access_token",
			value:    "at-654321",
			expected: "high",
		},
		{
			name:     "database_high",
			pattern:  "database_url",
			value:    "postgres://user:pass@host/db",
			expected: "high",
		},
		{
			name:     "db_high",
			pattern:  "db_password",
			value:    "dbpass123",
			expected: "high",
		},
		{
			name:     "password_medium",
			pattern:  "user_password",
			value:    "userpass123",
			expected: "medium",
		},
		{
			name:     "secret_medium",
			pattern:  "client_secret",
			value:    "secret123",
			expected: "medium",
		},
		{
			name:     "unknown_low",
			pattern:  "some_value",
			value:    "value123",
			expected: "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scanner.determineSeverity(tt.pattern, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFileSecretScanner_calculateConfidence(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewFileSecretScanner(logger)

	tests := []struct {
		name     string
		pattern  string
		expected int
	}{
		{
			name:     "api_key_high_confidence",
			pattern:  "api_key",
			expected: 95,
		},
		{
			name:     "apikey_high_confidence",
			pattern:  "apikey",
			expected: 95,
		},
		{
			name:     "private_key_high_confidence",
			pattern:  "private_key",
			expected: 95,
		},
		{
			name:     "privatekey_high_confidence",
			pattern:  "privatekey",
			expected: 95,
		},
		{
			name:     "password_high_confidence",
			pattern:  "password",
			expected: 90,
		},
		{
			name:     "token_high_confidence",
			pattern:  "token",
			expected: 90,
		},
		{
			name:     "secret_medium_confidence",
			pattern:  "secret",
			expected: 80,
		},
		{
			name:     "key_medium_confidence",
			pattern:  "key",
			expected: 75,
		},
		{
			name:     "unknown_pattern_low_confidence",
			pattern:  "unknown_pattern",
			expected: 60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scanner.calculateConfidence(tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFileSecretScanner_PerformSecretScan_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewFileSecretScanner(logger)

	// Create temporary directory with test files
	tempDir, err := os.MkdirTemp("", "secret-scan-integration-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create test files with various secret patterns
	testFiles := map[string]string{
		"config.env": `
# Configuration file
API_KEY=sk-test123456789abcdef
DATABASE_URL=postgres://user:password@localhost/db
DEBUG=true
PORT=8080
`,
		"docker-compose.yml": `
version: '3.8'
services:
  app:
    environment:
      - SECRET_KEY=super-secret-key-123
      - DB_PASSWORD=database-password
`,
		"app.py": `
import os

api_key = "ak-production-key-456"
db_url = os.getenv("DATABASE_URL", "postgres://default:password@localhost/app")

def main():
    print("Application started")
`,
		"clean.py": `
import os

app_name = os.getenv("APP_NAME", "default")
port = int(os.getenv("PORT", "8080"))

def main():
    print(f"Starting {app_name} on port {port}")
`,
	}

	for filename, content := range testFiles {
		filePath := filepath.Join(tempDir, filename)
		err := os.WriteFile(filePath, []byte(content), 0644)
		require.NoError(t, err)
	}

	// Perform the scan
	filePatterns := []string{"*.env", "*.yml", "*.py"}
	excludePatterns := []string{}

	secrets, fileResults, filesScanned, err := scanner.PerformSecretScan(
		tempDir,
		filePatterns,
		excludePatterns,
		nil, // No progress reporter for test
	)

	require.NoError(t, err)
	assert.Greater(t, filesScanned, 0)
	assert.Greater(t, len(secrets), 0)
	assert.Equal(t, len(testFiles), len(fileResults))

	// Verify file results
	fileTypeMap := make(map[string]FileSecretScanResult)
	for _, result := range fileResults {
		fileTypeMap[filepath.Base(result.FilePath)] = result
	}

	// Check config.env - should have secrets
	configResult, exists := fileTypeMap["config.env"]
	require.True(t, exists)
	assert.Equal(t, "environment", configResult.FileType)
	assert.Greater(t, configResult.SecretsFound, 0)
	assert.NotEqual(t, "clean", configResult.CleanStatus)

	// Check clean.py - should be clean
	cleanResult, exists := fileTypeMap["clean.py"]
	require.True(t, exists)
	assert.Equal(t, "python", cleanResult.FileType)
	assert.Equal(t, 0, cleanResult.SecretsFound)
	assert.Equal(t, "clean", cleanResult.CleanStatus)

	t.Logf("Scan completed: %d files scanned, %d secrets found", filesScanned, len(secrets))
}

func TestFileSecretScanner_PerformSecretScan_EmptyDirectory(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewFileSecretScanner(logger)

	// Create empty temporary directory
	tempDir, err := os.MkdirTemp("", "empty-scan-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	filePatterns := []string{"*.env", "*.py"}
	excludePatterns := []string{}

	secrets, fileResults, filesScanned, err := scanner.PerformSecretScan(
		tempDir,
		filePatterns,
		excludePatterns,
		nil,
	)

	require.NoError(t, err)
	assert.Equal(t, 0, filesScanned)
	assert.Equal(t, 0, len(secrets))
	assert.Equal(t, 0, len(fileResults))
}

func TestFileSecretScanner_PerformSecretScan_NonexistentDirectory(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewFileSecretScanner(logger)

	nonexistentDir := "/path/that/does/not/exist"
	filePatterns := []string{"*.env"}
	excludePatterns := []string{}

	secrets, fileResults, filesScanned, err := scanner.PerformSecretScan(
		nonexistentDir,
		filePatterns,
		excludePatterns,
		nil,
	)

	assert.Error(t, err)
	assert.Equal(t, 0, filesScanned)
	assert.Equal(t, 0, len(secrets))
	assert.Equal(t, 0, len(fileResults))
}

// BenchmarkFileSecretScanner_PerformSecretScan benchmarks the scan performance
func BenchmarkFileSecretScanner_PerformSecretScan(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	scanner := NewFileSecretScanner(logger)

	// Create temporary directory with test files for benchmarking
	tempDir, err := os.MkdirTemp("", "benchmark-scan-test")
	require.NoError(b, err)
	defer os.RemoveAll(tempDir)

	// Create multiple test files
	for i := 0; i < 10; i++ {
		content := `
API_KEY=sk-test123456789abcdef
DATABASE_URL=postgres://user:password@localhost/db
SECRET_TOKEN=secret-token-123456
`
		filePath := filepath.Join(tempDir, fmt.Sprintf("config%d.env", i))
		err := os.WriteFile(filePath, []byte(content), 0644)
		require.NoError(b, err)
	}

	filePatterns := []string{"*.env"}
	excludePatterns := []string{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, err := scanner.PerformSecretScan(tempDir, filePatterns, excludePatterns, nil)
		require.NoError(b, err)
	}
}
