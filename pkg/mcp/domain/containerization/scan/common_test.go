package scan

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScannerRegistry_Basic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	registry := NewScannerRegistry(logger)

	require.NotNil(t, registry)
	assert.Empty(t, registry.GetScannerNames())
}

func TestScannerRegistry_RegisterAndRetrieve(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	registry := NewScannerRegistry(logger)

	// Create mock scanners
	apiScanner := NewAPIKeyScanner(logger)
	regexScanner := NewRegexBasedScanner(logger)

	// Register scanners
	registry.Register(apiScanner)
	registry.Register(regexScanner)

	// Test retrieval
	names := registry.GetScannerNames()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "api_key_scanner")
	assert.Contains(t, names, "regex_scanner")

	// Test getting specific scanner
	retrieved := registry.GetScanner("api_key_scanner")
	require.NotNil(t, retrieved)
	assert.Equal(t, "api_key_scanner", retrieved.GetName())

	// Test non-existent scanner
	notFound := registry.GetScanner("nonexistent")
	assert.Nil(t, notFound)
}

func TestScannerRegistry_GetApplicableScanners(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	registry := NewScannerRegistry(logger)

	// Register scanners
	apiScanner := NewAPIKeyScanner(logger)
	regexScanner := NewRegexBasedScanner(logger)
	registry.Register(apiScanner)
	registry.Register(regexScanner)

	tests := []struct {
		name        string
		content     string
		contentType ContentType
		expected    int
	}{
		{
			name:        "source_code_applicable",
			content:     "const apiKey = 'test123';",
			contentType: ContentTypeSourceCode,
			expected:    2, // Both scanners should be applicable
		},
		{
			name:        "config_applicable",
			content:     "api_key=test123",
			contentType: ContentTypeConfig,
			expected:    2,
		},
		{
			name:        "certificate_content",
			content:     "-----BEGIN CERTIFICATE-----",
			contentType: ContentTypeCertificate,
			expected:    1, // Only API scanner should be applicable
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applicable := registry.GetApplicableScanners(tt.content, tt.contentType)
			assert.Len(t, applicable, tt.expected)
		})
	}
}

func TestScannerRegistry_ScanWithAllApplicable(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	registry := NewScannerRegistry(logger)

	// Register scanners
	apiScanner := NewAPIKeyScanner(logger)
	regexScanner := NewRegexBasedScanner(logger)
	registry.Register(apiScanner)
	registry.Register(regexScanner)

	config := ScanConfig{
		Content:     "api_key=test123456789",
		ContentType: ContentTypeConfig,
		FilePath:    "test.config",
		Options: ScanOptions{
			IncludeHighEntropy: false,
		},
		Logger: logger,
	}

	ctx := context.Background()
	result, err := registry.ScanWithAllApplicable(ctx, config)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify result structure
	assert.Greater(t, result.Duration, time.Duration(0))
	assert.NotNil(t, result.ScannerResults)
	assert.NotNil(t, result.AllSecrets)
	assert.NotNil(t, result.Summary)

	// Verify summary contains expected fields
	summary := result.Summary
	assert.Contains(t, summary, "total_scanners")
	assert.Contains(t, summary, "total_secrets")
	assert.Contains(t, summary, "by_type")
	assert.Contains(t, summary, "by_severity")
	assert.Contains(t, summary, "confidence_avg")
}

func TestScanOptions_Defaults(t *testing.T) {
	options := ScanOptions{}

	// Test default values
	assert.False(t, options.IncludeHighEntropy)
	assert.False(t, options.IncludeKeywords)
	assert.False(t, options.IncludePatterns)
	assert.False(t, options.IncludeBase64)
	assert.Equal(t, int64(0), options.MaxFileSize)
	assert.Equal(t, SensitivityLevel(""), options.Sensitivity)
	assert.False(t, options.SkipBinary)
	assert.False(t, options.SkipArchives)
}

func TestScanOptions_Configuration(t *testing.T) {
	options := ScanOptions{
		IncludeHighEntropy: true,
		IncludeKeywords:    true,
		IncludePatterns:    true,
		IncludeBase64:      true,
		MaxFileSize:        1024 * 1024, // 1MB
		Sensitivity:        SensitivityHigh,
		SkipBinary:         true,
		SkipArchives:       true,
	}

	assert.True(t, options.IncludeHighEntropy)
	assert.True(t, options.IncludeKeywords)
	assert.True(t, options.IncludePatterns)
	assert.True(t, options.IncludeBase64)
	assert.Equal(t, int64(1024*1024), options.MaxFileSize)
	assert.Equal(t, SensitivityHigh, options.Sensitivity)
	assert.True(t, options.SkipBinary)
	assert.True(t, options.SkipArchives)
}

func TestContentType_Constants(t *testing.T) {
	// Test all content type constants
	assert.Equal(t, ContentType("source_code"), ContentTypeSourceCode)
	assert.Equal(t, ContentType("config"), ContentTypeConfig)
	assert.Equal(t, ContentType("dockerfile"), ContentTypeDockerfile)
	assert.Equal(t, ContentType("kubernetes"), ContentTypeKubernetes)
	assert.Equal(t, ContentType("compose"), ContentTypeCompose)
	assert.Equal(t, ContentType("database"), ContentTypeDatabase)
	assert.Equal(t, ContentType("environment"), ContentTypeEnvironment)
	assert.Equal(t, ContentType("certificate"), ContentTypeCertificate)
	assert.Equal(t, ContentType("generic"), ContentTypeGeneric)
}

func TestSecretType_Constants(t *testing.T) {
	// Test all secret type constants
	assert.Equal(t, SecretType("api_key"), SecretTypeAPIKey)
	assert.Equal(t, SecretType("password"), SecretTypePassword)
	assert.Equal(t, SecretType("private_key"), SecretTypePrivateKey)
	assert.Equal(t, SecretType("certificate"), SecretTypeCertificate)
	assert.Equal(t, SecretType("token"), SecretTypeToken)
	assert.Equal(t, SecretType("connection_string"), SecretTypeConnectionString)
	assert.Equal(t, SecretType("credential"), SecretTypeCredential)
	assert.Equal(t, SecretType("secret"), SecretTypeSecret)
	assert.Equal(t, SecretType("environment_variable"), SecretTypeEnvironmentVar)
	assert.Equal(t, SecretType("high_entropy"), SecretTypeHighEntropy)
	assert.Equal(t, SecretType("generic"), SecretTypeGeneric)
}

func TestSeverity_Constants(t *testing.T) {
	// Test all severity constants
	assert.Equal(t, Severity("info"), SeverityInfo)
	assert.Equal(t, Severity("low"), SeverityLow)
	assert.Equal(t, Severity("medium"), SeverityMedium)
	assert.Equal(t, Severity("high"), SeverityHigh)
	assert.Equal(t, Severity("critical"), SeverityCritical)
}

func TestSensitivityLevel_Constants(t *testing.T) {
	// Test all sensitivity level constants
	assert.Equal(t, SensitivityLevel("low"), SensitivityLow)
	assert.Equal(t, SensitivityLevel("medium"), SensitivityMedium)
	assert.Equal(t, SensitivityLevel("high"), SensitivityHigh)
}

func TestSecret_Structure(t *testing.T) {
	secret := Secret{
		Type:        SecretTypeAPIKey,
		Value:       "test-key-123",
		MaskedValue: "test***123",
		Location: &Location{
			File:   "test.env",
			Line:   5,
			Column: 10,
		},
		Confidence: 0.85,
		Severity:   SeverityHigh,
		Context:    "API_KEY=test-key-123",
		Pattern:    "api_key_pattern",
		Entropy:    4.2,
		Metadata: map[string]interface{}{
			"scanner": "test_scanner",
		},
		Evidence: []Evidence{
			{
				Type:        "pattern_match",
				Description: "Matched API key pattern",
				Value:       "test-key-123",
				Pattern:     "api.*key",
				Context:     "API_KEY=test-key-123",
			},
		},
	}

	// Verify all fields are set correctly
	assert.Equal(t, SecretTypeAPIKey, secret.Type)
	assert.Equal(t, "test-key-123", secret.Value)
	assert.Equal(t, "test***123", secret.MaskedValue)
	assert.Equal(t, "test.env", secret.Location.File)
	assert.Equal(t, 5, secret.Location.Line)
	assert.Equal(t, 10, secret.Location.Column)
	assert.Equal(t, 0.85, secret.Confidence)
	assert.Equal(t, SeverityHigh, secret.Severity)
	assert.Equal(t, "API_KEY=test-key-123", secret.Context)
	assert.Equal(t, "api_key_pattern", secret.Pattern)
	assert.Equal(t, 4.2, secret.Entropy)
	assert.Contains(t, secret.Metadata, "scanner")
	assert.Len(t, secret.Evidence, 1)
	assert.Equal(t, "pattern_match", secret.Evidence[0].Type)
}

func TestLocation_Structure(t *testing.T) {
	location := Location{
		File:       "/path/to/file.env",
		Line:       42,
		Column:     15,
		StartIndex: 100,
		EndIndex:   120,
	}

	assert.Equal(t, "/path/to/file.env", location.File)
	assert.Equal(t, 42, location.Line)
	assert.Equal(t, 15, location.Column)
	assert.Equal(t, 100, location.StartIndex)
	assert.Equal(t, 120, location.EndIndex)
}

func TestEvidence_Structure(t *testing.T) {
	evidence := Evidence{
		Type:        "regex_match",
		Description: "Matched password pattern",
		Value:       "secret123",
		Pattern:     "password.*=.*",
		Context:     "password=secret123",
	}

	assert.Equal(t, "regex_match", evidence.Type)
	assert.Equal(t, "Matched password pattern", evidence.Description)
	assert.Equal(t, "secret123", evidence.Value)
	assert.Equal(t, "password.*=.*", evidence.Pattern)
	assert.Equal(t, "password=secret123", evidence.Context)
}

func TestScanResult_Structure(t *testing.T) {
	result := ScanResult{
		Scanner:    "test_scanner",
		Success:    true,
		Secrets:    []Secret{},
		Metadata:   map[string]interface{}{"lines_scanned": 10},
		Confidence: 0.9,
		Errors:     []error{},
	}

	assert.Equal(t, "test_scanner", result.Scanner)
	assert.True(t, result.Success)
	assert.Empty(t, result.Secrets)
	assert.Contains(t, result.Metadata, "lines_scanned")
	assert.Equal(t, 0.9, result.Confidence)
	assert.Empty(t, result.Errors)
}

func TestCombinedScanResult_Structure(t *testing.T) {
	result := CombinedScanResult{
		ScannerResults: map[string]*ScanResult{
			"scanner1": {
				Scanner: "scanner1",
				Success: true,
			},
		},
		AllSecrets: []Secret{},
		Summary: map[string]interface{}{
			"total_scanners": 1,
			"total_secrets":  0,
		},
	}

	assert.Len(t, result.ScannerResults, 1)
	assert.Contains(t, result.ScannerResults, "scanner1")
	assert.Empty(t, result.AllSecrets)
	assert.Contains(t, result.Summary, "total_scanners")
	assert.Contains(t, result.Summary, "total_secrets")
}

// BenchmarkScannerRegistry benchmarks registry operations
func BenchmarkScannerRegistry(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	registry := NewScannerRegistry(logger)

	// Register scanners
	apiScanner := NewAPIKeyScanner(logger)
	regexScanner := NewRegexBasedScanner(logger)
	registry.Register(apiScanner)
	registry.Register(regexScanner)

	content := "api_key=test123456789"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		applicable := registry.GetApplicableScanners(content, ContentTypeConfig)
		_ = applicable
	}
}
