package scan

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResultProcessor_CalculateSecurityScore(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	processor := NewResultProcessor(logger)

	tests := []struct {
		name          string
		secrets       []ScannedSecret
		expectedScore int
	}{
		{
			name:          "no_secrets",
			secrets:       []ScannedSecret{},
			expectedScore: 100,
		},
		{
			name: "critical_secrets",
			secrets: []ScannedSecret{
				{Severity: "critical"},
				{Severity: "critical"},
			},
			expectedScore: 50, // 100 - 25 - 25
		},
		{
			name: "mixed_severity_secrets",
			secrets: []ScannedSecret{
				{Severity: "critical"},
				{Severity: "high"},
				{Severity: "medium"},
				{Severity: "low"},
			},
			expectedScore: 49, // 100 - 25 - 15 - 8 - 3
		},
		{
			name: "score_floor",
			secrets: []ScannedSecret{
				{Severity: "critical"},
				{Severity: "critical"},
				{Severity: "critical"},
				{Severity: "critical"},
				{Severity: "critical"},
			},
			expectedScore: 0, // Should not go below 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := processor.CalculateSecurityScore(tt.secrets)
			assert.Equal(t, tt.expectedScore, score)
		})
	}
}

func TestResultProcessor_DetermineRiskLevel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	processor := NewResultProcessor(logger)

	tests := []struct {
		name          string
		score         int
		secrets       []ScannedSecret
		expectedLevel string
	}{
		{
			name:          "low_risk",
			score:         90,
			secrets:       []ScannedSecret{},
			expectedLevel: "low",
		},
		{
			name:          "medium_risk",
			score:         70,
			secrets:       []ScannedSecret{},
			expectedLevel: "medium",
		},
		{
			name:          "high_risk",
			score:         40,
			secrets:       []ScannedSecret{},
			expectedLevel: "high",
		},
		{
			name:          "critical_risk",
			score:         10,
			secrets:       []ScannedSecret{},
			expectedLevel: "critical",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := processor.DetermineRiskLevel(tt.score, tt.secrets)
			assert.Equal(t, tt.expectedLevel, level)
		})
	}
}

func TestResultProcessor_CalculateSeverityBreakdown(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	processor := NewResultProcessor(logger)

	secrets := []ScannedSecret{
		{Severity: "critical"},
		{Severity: "critical"},
		{Severity: "high"},
		{Severity: "medium"},
		{Severity: "low"},
		{Severity: "low"},
		{Severity: "low"},
	}

	breakdown := processor.CalculateSeverityBreakdown(secrets)

	assert.Equal(t, 2, breakdown["critical"])
	assert.Equal(t, 1, breakdown["high"])
	assert.Equal(t, 1, breakdown["medium"])
	assert.Equal(t, 3, breakdown["low"])
}

func TestResultProcessor_GenerateRecommendations(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	processor := NewResultProcessor(logger)

	tests := []struct {
		name           string
		secrets        []ScannedSecret
		args           AtomicScanSecretsArgs
		expectCount    int
		expectCritical bool
	}{
		{
			name:           "no_secrets",
			secrets:        []ScannedSecret{},
			args:           AtomicScanSecretsArgs{},
			expectCount:    2, // Should get default recommendations
			expectCritical: false,
		},
		{
			name: "critical_secrets",
			secrets: []ScannedSecret{
				{Severity: "critical", File: "config.py"},
			},
			args:           AtomicScanSecretsArgs{},
			expectCount:    5, // Should get critical + general recommendations
			expectCritical: true,
		},
		{
			name: "dockerfile_secrets",
			secrets: []ScannedSecret{
				{Severity: "high", File: "Dockerfile"},
			},
			args:           AtomicScanSecretsArgs{},
			expectCount:    5, // Should get Docker-specific recommendations
			expectCritical: false,
		},
		{
			name: "kubernetes_secrets",
			secrets: []ScannedSecret{
				{Severity: "medium", File: "deployment.yaml"},
			},
			args:           AtomicScanSecretsArgs{},
			expectCount:    5, // Should get K8s-specific recommendations
			expectCritical: false,
		},
		{
			name: "env_file_secrets",
			secrets: []ScannedSecret{
				{Severity: "low", File: ".env"},
			},
			args:           AtomicScanSecretsArgs{},
			expectCount:    5, // Should get env file recommendations
			expectCritical: false,
		},
		{
			name: "generate_secrets_enabled",
			secrets: []ScannedSecret{
				{Severity: "medium", File: "config.py"},
			},
			args:           AtomicScanSecretsArgs{GenerateSecrets: true},
			expectCount:    6, // Should get extra recommendations
			expectCritical: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recommendations := processor.GenerateRecommendations(tt.secrets, tt.args)

			assert.GreaterOrEqual(t, len(recommendations), tt.expectCount)

			if tt.expectCritical {
				found := false
				for _, rec := range recommendations {
					if assert.Contains(t, rec, "CRITICAL") {
						found = true
						break
					}
				}
				assert.True(t, found, "Should contain critical recommendations")
			}
		})
	}
}

func TestResultProcessor_GenerateScanContext(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	processor := NewResultProcessor(logger)

	secrets := []ScannedSecret{
		{Type: "api_key", File: "config.py"},
		{Type: "password", File: "settings.yaml"},
	}

	fileResults := []FileSecretScanResult{
		{FilePath: "config.py", FileType: "python", SecretsFound: 1},
		{FilePath: "settings.yaml", FileType: "yaml", SecretsFound: 1},
		{FilePath: "clean.py", FileType: "python", SecretsFound: 0},
	}

	args := AtomicScanSecretsArgs{
		ScanDockerfiles: true,
		ScanManifests:   true,
		ScanSourceCode:  true,
		ScanEnvFiles:    false,
	}

	context := processor.GenerateScanContext(secrets, fileResults, args)

	// Verify context structure
	assert.Contains(t, context, "file_types_scanned")
	assert.Contains(t, context, "secret_types_found")
	assert.Contains(t, context, "scan_configuration")
	assert.Contains(t, context, "files_with_secrets")
	assert.Contains(t, context, "risk_factors")

	// Verify file types
	fileTypes := context["file_types_scanned"].(map[string]int)
	assert.Equal(t, 2, fileTypes["python"])
	assert.Equal(t, 1, fileTypes["yaml"])

	// Verify secret types
	secretTypes := context["secret_types_found"].(map[string]int)
	assert.Equal(t, 1, secretTypes["api_key"])
	assert.Equal(t, 1, secretTypes["password"])

	// Verify scan configuration
	scanConfig := context["scan_configuration"].(map[string]interface{})
	assert.True(t, scanConfig["scan_dockerfiles"].(bool))
	assert.True(t, scanConfig["scan_source_code"].(bool))
	assert.False(t, scanConfig["scan_env_files"].(bool))

	// Verify files with secrets
	filesWithSecrets := context["files_with_secrets"].([]string)
	assert.Len(t, filesWithSecrets, 2)
	assert.Contains(t, filesWithSecrets, "config.py")
	assert.Contains(t, filesWithSecrets, "settings.yaml")
}

func TestResultProcessor_IdentifyRiskFactors(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	processor := NewResultProcessor(logger)

	// Use reflection to test the private method indirectly through GenerateScanContext
	secrets := []ScannedSecret{
		{Type: "api_key", File: "config.py", Pattern: "PROD_API_KEY"},
		{Type: "password", File: "config.py", Pattern: "DB_PASSWORD"},
		{Type: "api_key", File: "config.py", Pattern: "GITHUB_TOKEN"},
		{Type: "api_key", File: "config.py", Pattern: "STRIPE_KEY"},
		{Type: "database_url", File: "settings.py", Pattern: "DATABASE_URL"},
	}

	fileResults := []FileSecretScanResult{
		{FilePath: "config.py", FileType: "python", SecretsFound: 4},
		{FilePath: "settings.py", FileType: "python", SecretsFound: 1},
	}

	context := processor.GenerateScanContext(secrets, fileResults, AtomicScanSecretsArgs{})
	riskFactors := context["risk_factors"].([]string)

	assert.GreaterOrEqual(t, len(riskFactors), 1)

	// Should identify multiple secrets in same file
	hasMultipleSecretsRisk := false
	for _, factor := range riskFactors {
		if assert.Contains(t, factor, "Multiple secrets") {
			hasMultipleSecretsRisk = true
			break
		}
	}
	assert.True(t, hasMultipleSecretsRisk)
}

func TestResultProcessor_Creation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	processor := NewResultProcessor(logger)

	require.NotNil(t, processor)
	assert.Equal(t, logger, processor.logger)
}

func TestResultProcessor_EdgeCases(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	processor := NewResultProcessor(logger)

	// Test with nil/empty inputs
	t.Run("empty_secrets", func(t *testing.T) {
		score := processor.CalculateSecurityScore(nil)
		assert.Equal(t, 100, score)

		breakdown := processor.CalculateSeverityBreakdown(nil)
		assert.Empty(t, breakdown)
	})

	t.Run("unknown_severity", func(t *testing.T) {
		secrets := []ScannedSecret{
			{Severity: "unknown"},
			{Severity: ""},
		}

		score := processor.CalculateSecurityScore(secrets)
		assert.Equal(t, 100, score) // Unknown severities shouldn't affect score
	})

	t.Run("empty_file_results", func(t *testing.T) {
		context := processor.GenerateScanContext([]ScannedSecret{}, []FileSecretScanResult{}, AtomicScanSecretsArgs{})

		assert.Contains(t, context, "file_types_scanned")
		assert.Contains(t, context, "secret_types_found")

		fileTypes := context["file_types_scanned"].(map[string]int)
		assert.Empty(t, fileTypes)

		secretTypes := context["secret_types_found"].(map[string]int)
		assert.Empty(t, secretTypes)
	})
}

// BenchmarkResultProcessor_CalculateSecurityScore benchmarks security score calculation
func BenchmarkResultProcessor_CalculateSecurityScore(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	processor := NewResultProcessor(logger)

	// Create a large set of secrets for benchmarking
	secrets := make([]ScannedSecret, 1000)
	for i := 0; i < 1000; i++ {
		severities := []string{"critical", "high", "medium", "low"}
		secrets[i] = ScannedSecret{
			Severity: severities[i%4],
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processor.CalculateSecurityScore(secrets)
	}
}

// BenchmarkResultProcessor_GenerateRecommendations benchmarks recommendation generation
func BenchmarkResultProcessor_GenerateRecommendations(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	processor := NewResultProcessor(logger)

	secrets := []ScannedSecret{
		{Severity: "critical", File: "Dockerfile", Type: "api_key"},
		{Severity: "high", File: "deployment.yaml", Type: "password"},
		{Severity: "medium", File: ".env", Type: "database_url"},
		{Severity: "low", File: "config.py", Type: "token"},
	}

	args := AtomicScanSecretsArgs{
		ScanDockerfiles: true,
		ScanManifests:   true,
		ScanSourceCode:  true,
		ScanEnvFiles:    true,
		GenerateSecrets: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processor.GenerateRecommendations(secrets, args)
	}
}
