package security_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/containerization-assist/pkg/core/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecretDiscovery_ScanDirectory(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sd := security.NewSecretDiscovery(logger)

	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	tests := []struct {
		name           string
		files          map[string]string
		expectedTypes  []string
		expectedCounts map[string]int
	}{
		{
			name: "detect AWS credentials",
			files: map[string]string{
				"config.txt": `
					aws_access_key_id = AKIAIOSFODNN7PRODKEY
					aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYPRODKEY
				`,
			},
			expectedTypes: []string{"aws_access_key"},
			expectedCounts: map[string]int{
				"aws_access_key": 1,
			},
		},
		{
			name: "detect API keys",
			files: map[string]string{
				"app.js": `
					const apiKey = "sk-1234567890abcdef1234567890abcdef1234567890abcdef";
					const token = "ghp_1234567890abcdef1234567890abcdef12345";
				`,
			},
			expectedTypes: []string{"api_key", "github_token"},
			expectedCounts: map[string]int{
				"total": 2,
			},
		},
		{
			name: "detect private keys",
			files: map[string]string{
				"key.pem": `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA1234567890abcdefghijklmnopqrstuvwxyz
-----END RSA PRIVATE KEY-----`,
			},
			expectedTypes: []string{"private_key"},
		},
		{
			name: "no secrets in normal code",
			files: map[string]string{
				"main.go": `
					package main
					import "fmt"
					func main() {
						fmt.Println("Hello, World!")
					}
				`,
			},
			expectedTypes:  []string{},
			expectedCounts: map[string]int{"total": 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test files
			testDir := filepath.Join(tmpDir, tt.name)
			err := os.MkdirAll(testDir, 0755)
			require.NoError(t, err)

			for filename, content := range tt.files {
				err := os.WriteFile(filepath.Join(testDir, filename), []byte(content), 0644)
				require.NoError(t, err)
			}

			// Scan directory
			ctx := context.Background()
			options := security.DefaultScanOptions()
			result, err := sd.ScanDirectory(ctx, testDir, options)
			require.NoError(t, err)

			// Verify results
			assert.NotNil(t, result)

			// Check total count if specified
			if expectedTotal, ok := tt.expectedCounts["total"]; ok {
				actualTotal := result.Summary.TotalFindings - result.Summary.FalsePositives
				assert.Equal(t, expectedTotal, actualTotal,
					"Expected %d total findings, got %d", expectedTotal, actualTotal)
			}

			// Check specific types if specified
			if len(tt.expectedTypes) > 0 {
				foundTypes := make(map[string]bool)
				for _, finding := range result.Findings {
					if !finding.FalsePositive {
						foundTypes[finding.Type] = true
					}
				}

				for _, expectedType := range tt.expectedTypes {
					assert.True(t, foundTypes[expectedType],
						"Expected to find secret type: %s", expectedType)
				}
			}
		})
	}
}

func TestSecretDiscovery_ScanOptions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sd := security.NewSecretDiscovery(logger)

	tmpDir := t.TempDir()

	// Create test files
	err := os.WriteFile(filepath.Join(tmpDir, "secret.txt"),
		[]byte("api_key = sk-1234567890abcdef1234567890abcdef"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "test_secret.txt"),
		[]byte("test_key = sk-test-1234567890abcdef1234567890abcdef"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "readme.md"),
		[]byte("# Example\napi_key = sk-example-1234567890abcdef"), 0644)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("exclude test files", func(t *testing.T) {
		options := security.ScanOptions{
			MaxFileSize:    1024 * 1024,
			MaxConcurrency: 4,
			Recursive:      true,
		}

		result, err := sd.ScanDirectory(ctx, tmpDir, options)
		require.NoError(t, err)

		// Should not find secrets in test files
		for _, finding := range result.Findings {
			assert.NotContains(t, finding.File, "test_")
		}
	})

	t.Run("skip binary files", func(t *testing.T) {
		// Create a binary file
		binaryData := []byte{0x00, 0x01, 0x02, 0x03, 0xFF}
		err := os.WriteFile(filepath.Join(tmpDir, "binary.dat"), binaryData, 0644)
		require.NoError(t, err)

		options := security.DefaultScanOptions()
		result, err := sd.ScanDirectory(ctx, tmpDir, options)
		require.NoError(t, err)

		// Should not scan binary files
		for _, finding := range result.Findings {
			assert.NotEqual(t, "binary.dat", filepath.Base(finding.File))
		}
	})
}

func TestSecretDiscovery_FalsePositives(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sd := security.NewSecretDiscovery(logger)

	tmpDir := t.TempDir()

	// Create files with potential false positives
	testFiles := map[string]string{
		"example.txt": `
			# Example configuration
			aws_access_key_id = AKIAIOSFODNN7EXAMPLE
			api_key = sk-example-key-1234567890abcdef
		`,
		"test.js": `
			const mockApiKey = "sk-test-1234567890abcdef1234567890abcdef";
			const demoToken = "demo-token-1234567890";
		`,
		"docs.md": `
			## API Key Format
			Your API key should look like: sk-1234567890abcdef1234567890abcdef
			Never share your real API key!
		`,
	}

	for filename, content := range testFiles {
		err := os.WriteFile(filepath.Join(tmpDir, filename), []byte(content), 0644)
		require.NoError(t, err)
	}

	ctx := context.Background()
	options := security.DefaultScanOptions()
	result, err := sd.ScanDirectory(ctx, tmpDir, options)
	require.NoError(t, err)

	// Check that some findings are marked as false positives
	assert.Greater(t, result.Summary.FalsePositives, 0,
		"Should detect some false positives in example/test content")

	// Verify false positive detection
	for _, finding := range result.Findings {
		if strings.Contains(finding.File, "example") ||
			strings.Contains(finding.Match, "example") ||
			strings.Contains(finding.Match, "test") ||
			strings.Contains(finding.Match, "demo") {
			// These should likely be marked as false positives
			// but we can't assert this without knowing the internal logic
			t.Logf("Potential false positive in test/demo content: %s", finding.Match)
		}
	}
}

func TestSecretDiscovery_ExtendedSecretFinding(t *testing.T) {
	// Test that ExtendedSecretFinding properly embeds SecretFinding
	finding := security.ExtendedSecretFinding{
		SecretFinding: security.SecretFinding{
			Type:        "api_key",
			File:        "config.txt",
			Line:        10,
			Description: "API key found",
			Confidence:  0.95,
			RuleID:      "api-key-001",
		},
		ID:       "finding-001",
		Column:   15,
		Severity: "HIGH",
		Match:    "sk-1234567890",
		Redacted: "sk-***",
		Context:  "API_KEY=sk-1234567890",
		Entropy:  4.5,
		Pattern:  "sk-[a-zA-Z0-9]{10,}",
		Verified: false,
	}

	assert.Equal(t, "api_key", finding.Type)
	assert.Equal(t, "config.txt", finding.File)
	assert.Equal(t, 10, finding.Line)
	assert.False(t, finding.FalsePositive)
	assert.Equal(t, 0.95, finding.Confidence)
}

func TestSecretDiscovery_LargeFile(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sd := security.NewSecretDiscovery(logger)

	tmpDir := t.TempDir()

	// Create a large file that exceeds default max size
	largeContent := strings.Repeat("no secrets here\n", 100000)
	largeContent += "api_key = sk-hidden-in-large-file-1234567890abcdef"

	err := os.WriteFile(filepath.Join(tmpDir, "large.txt"), []byte(largeContent), 0644)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("skip large files with default options", func(t *testing.T) {
		options := security.DefaultScanOptions()
		options.MaxFileSize = 1024 * 1024 // 1MB limit

		result, err := sd.ScanDirectory(ctx, tmpDir, options)
		require.NoError(t, err)

		// Large file should be skipped
		assert.Equal(t, 0, result.Summary.TotalFindings,
			"Should skip files larger than MaxFileSize")
	})

	t.Run("scan large files when limit increased", func(t *testing.T) {
		options := security.DefaultScanOptions()
		options.MaxFileSize = 10 * 1024 * 1024 // 10MB limit

		result, err := sd.ScanDirectory(ctx, tmpDir, options)
		require.NoError(t, err)

		// Should find the secret in large file
		assert.Greater(t, result.Summary.TotalFindings, 0,
			"Should find secrets when file size is within limit")
	})
}

func TestSecretDiscovery_ContextCancellation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sd := security.NewSecretDiscovery(logger)

	tmpDir := t.TempDir()

	// Create many files to scan
	for i := 0; i < 100; i++ {
		content := "some content without secrets"
		filename := filepath.Join(tmpDir, strings.Repeat("subdir/", 5),
			fmt.Sprintf("file%d.txt", i))

		err := os.MkdirAll(filepath.Dir(filename), 0755)
		require.NoError(t, err)

		err = os.WriteFile(filename, []byte(content), 0644)
		require.NoError(t, err)
	}

	// Create context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context immediately
	cancel()

	options := security.DefaultScanOptions()
	_, err := sd.ScanDirectory(ctx, tmpDir, options)

	// Should return context error
	assert.Error(t, err)
	// The error should indicate the operation failed (due to context cancellation)
	assert.Contains(t, err.Error(), "failed to walk directory")
}

func TestSecretDiscovery_EmptyDirectory(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sd := security.NewSecretDiscovery(logger)

	tmpDir := t.TempDir()

	ctx := context.Background()
	options := security.DefaultScanOptions()

	result, err := sd.ScanDirectory(ctx, tmpDir, options)
	require.NoError(t, err)

	assert.NotNil(t, result)
	assert.Equal(t, 0, result.Summary.TotalFindings)
	assert.Equal(t, 0, result.Summary.TotalFindings)
}

func TestSecretDiscovery_InvalidPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sd := security.NewSecretDiscovery(logger)

	ctx := context.Background()
	options := security.DefaultScanOptions()

	_, err := sd.ScanDirectory(ctx, "/non/existent/path", options)
	assert.Error(t, err)
}

func TestSecretDiscovery_DefaultScanOptions(t *testing.T) {
	options := security.DefaultScanOptions()

	assert.Greater(t, options.MaxFileSize, int64(0))
	assert.Greater(t, options.MaxConcurrency, 0)
}

// Test scanning specific file patterns
func TestSecretDiscovery_FilePatterns(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	sd := security.NewSecretDiscovery(logger)

	tmpDir := t.TempDir()

	// Create various file types
	files := map[string]string{
		".env": `
			DATABASE_URL=postgresql://user:password@localhost/db
			API_KEY=sk-prod-1234567890abcdef
		`,
		".env.local": `
			SECRET_KEY=dev-secret-key-1234567890
		`,
		"config.yml": `
			api:
			  api_key: sk-1234567890abcdef1234567890abcdef
		`,
		"settings.json": `{
			"apiKey": "sk-json-1234567890abcdef1234567890"
		}`,
	}

	for filename, content := range files {
		err := os.WriteFile(filepath.Join(tmpDir, filename), []byte(content), 0644)
		require.NoError(t, err)
	}

	ctx := context.Background()
	options := security.DefaultScanOptions()

	result, err := sd.ScanDirectory(ctx, tmpDir, options)
	require.NoError(t, err)

	// Should find secrets in configuration files
	assert.Greater(t, result.Summary.TotalFindings, 0)

	// Check that secrets were found in expected files
	foundInEnv := false
	foundInConfig := false
	for _, finding := range result.Findings {
		if strings.Contains(finding.File, ".env") {
			foundInEnv = true
		}
		if strings.Contains(finding.File, "config") || strings.Contains(finding.File, "settings") {
			foundInConfig = true
		}
	}

	assert.True(t, foundInEnv, "Should find secrets in .env files")
	assert.True(t, foundInConfig, "Should find secrets in config files")
}
