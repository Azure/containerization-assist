package scan

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRegexBasedScanner(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewRegexBasedScanner(logger)

	require.NotNil(t, scanner)
	assert.Equal(t, "regex_scanner", scanner.GetName())
	assert.True(t, len(scanner.patterns) > 0)
}

func TestRegexBasedScanner_GetName(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewRegexBasedScanner(logger)

	assert.Equal(t, "regex_scanner", scanner.GetName())
}

func TestRegexBasedScanner_GetScanTypes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewRegexBasedScanner(logger)

	scanTypes := scanner.GetScanTypes()
	expectedTypes := []string{
		string(SecretTypeAPIKey),
		string(SecretTypePassword),
		string(SecretTypeToken),
		string(SecretTypeCredential),
		string(SecretTypeSecret),
		string(SecretTypeEnvironmentVar),
	}

	for _, expectedType := range expectedTypes {
		assert.Contains(t, scanTypes, expectedType)
	}
}

func TestRegexBasedScanner_IsApplicable(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewRegexBasedScanner(logger)

	tests := []struct {
		name        string
		content     string
		contentType ContentType
		expected    bool
	}{
		{
			name:        "source_code_applicable",
			content:     "some code",
			contentType: ContentTypeSourceCode,
			expected:    true,
		},
		{
			name:        "config_applicable",
			content:     "config content",
			contentType: ContentTypeConfig,
			expected:    true,
		},
		{
			name:        "environment_applicable",
			content:     "env content",
			contentType: ContentTypeEnvironment,
			expected:    true,
		},
		{
			name:        "generic_applicable",
			content:     "generic content",
			contentType: ContentTypeGeneric,
			expected:    true,
		},
		{
			name:        "certificate_not_applicable",
			content:     "certificate content",
			contentType: ContentTypeCertificate,
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scanner.IsApplicable(tt.content, tt.contentType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRegexBasedScanner_Scan_APIKeys(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewRegexBasedScanner(logger)

	tests := []struct {
		name            string
		content         string
		expectedSecrets int
		expectedType    SecretType
	}{
		{
			name:            "api_key_detection",
			content:         "API_KEY=sk-test123456789abcdef",
			expectedSecrets: 1,
			expectedType:    SecretTypeAPIKey,
		},
		{
			name:            "apikey_detection",
			content:         "apikey: test-key-123456789",
			expectedSecrets: 1,
			expectedType:    SecretTypeAPIKey,
		},
		{
			name:            "api_key_quoted",
			content:         `api_key = "ak-production-key-456789"`,
			expectedSecrets: 1,
			expectedType:    SecretTypeAPIKey,
		},
		{
			name:            "no_api_key",
			content:         "regular text without any keys",
			expectedSecrets: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ScanConfig{
				Content:  tt.content,
				FilePath: "test.env",
				Options: ScanOptions{
					IncludeHighEntropy: false,
				},
			}

			ctx := context.Background()
			result, err := scanner.Scan(ctx, config)

			require.NoError(t, err)
			assert.True(t, result.Success)
			assert.Equal(t, tt.expectedSecrets, len(result.Secrets))

			if tt.expectedSecrets > 0 {
				assert.Equal(t, tt.expectedType, result.Secrets[0].Type)
			}
		})
	}
}

func TestRegexBasedScanner_Scan_Passwords(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewRegexBasedScanner(logger)

	tests := []struct {
		name            string
		content         string
		expectedSecrets int
		expectedType    SecretType
	}{
		{
			name:            "password_detection",
			content:         "PASSWORD=mypassword123",
			expectedSecrets: 1,
			expectedType:    SecretTypePassword,
		},
		{
			name:            "passwd_detection",
			content:         "passwd: secretpass456",
			expectedSecrets: 1,
			expectedType:    SecretTypePassword,
		},
		{
			name:            "pwd_detection",
			content:         `pwd = "userpass789"`,
			expectedSecrets: 1,
			expectedType:    SecretTypePassword,
		},
		{
			name:            "password_too_short",
			content:         "PASSWORD=short",
			expectedSecrets: 0, // Should be filtered out due to length
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ScanConfig{
				Content:  tt.content,
				FilePath: "test.config",
				Options: ScanOptions{
					IncludeHighEntropy: false,
				},
			}

			ctx := context.Background()
			result, err := scanner.Scan(ctx, config)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedSecrets, len(result.Secrets))

			if tt.expectedSecrets > 0 {
				assert.Equal(t, tt.expectedType, result.Secrets[0].Type)
			}
		})
	}
}

func TestRegexBasedScanner_Scan_Tokens(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewRegexBasedScanner(logger)

	tests := []struct {
		name            string
		content         string
		expectedSecrets int
		expectedType    SecretType
	}{
		{
			name:            "token_detection",
			content:         "TOKEN=tk-abcdef123456789012345",
			expectedSecrets: 1,
			expectedType:    SecretTypeToken,
		},
		{
			name:            "access_token_detection",
			content:         "access_token: at-xyz789012345678901234",
			expectedSecrets: 1,
			expectedType:    SecretTypeToken,
		},
		{
			name:            "access-token_detection",
			content:         `access-token = "bearer-token-123456789012345"`,
			expectedSecrets: 1,
			expectedType:    SecretTypeToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ScanConfig{
				Content:  tt.content,
				FilePath: "test.config",
				Options: ScanOptions{
					IncludeHighEntropy: false,
				},
			}

			ctx := context.Background()
			result, err := scanner.Scan(ctx, config)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedSecrets, len(result.Secrets))

			if tt.expectedSecrets > 0 {
				assert.Equal(t, tt.expectedType, result.Secrets[0].Type)
			}
		})
	}
}

func TestRegexBasedScanner_Scan_Secrets(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewRegexBasedScanner(logger)

	tests := []struct {
		name            string
		content         string
		expectedSecrets int
		expectedType    SecretType
	}{
		{
			name:            "secret_detection",
			content:         "SECRET=my-super-secret-value-123456",
			expectedSecrets: 1,
			expectedType:    SecretTypeSecret,
		},
		{
			name:            "client_secret_detection",
			content:         "client_secret: cs-abcdef123456789012345678",
			expectedSecrets: 1,
			expectedType:    SecretTypeSecret,
		},
		{
			name:            "client-secret_detection",
			content:         `client-secret = "client-secret-xyz789012345"`,
			expectedSecrets: 1,
			expectedType:    SecretTypeSecret,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ScanConfig{
				Content:  tt.content,
				FilePath: "test.config",
				Options: ScanOptions{
					IncludeHighEntropy: false,
				},
			}

			ctx := context.Background()
			result, err := scanner.Scan(ctx, config)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedSecrets, len(result.Secrets))

			if tt.expectedSecrets > 0 {
				assert.Equal(t, tt.expectedType, result.Secrets[0].Type)
			}
		})
	}
}

func TestRegexBasedScanner_Scan_EnvironmentVariables(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewRegexBasedScanner(logger)

	tests := []struct {
		name            string
		content         string
		expectedSecrets int
		expectedType    SecretType
	}{
		{
			name:            "secret_env_var",
			content:         "SECRET_API_KEY=sk-production-key-123456789",
			expectedSecrets: 1,
			expectedType:    SecretTypeEnvironmentVar,
		},
		{
			name:            "key_env_var",
			content:         "KEY_DATABASE=db-key-abcdef123456789",
			expectedSecrets: 1,
			expectedType:    SecretTypeEnvironmentVar,
		},
		{
			name:            "token_env_var",
			content:         "TOKEN_ACCESS=access-token-xyz789012345",
			expectedSecrets: 1,
			expectedType:    SecretTypeEnvironmentVar,
		},
		{
			name:            "password_env_var",
			content:         "PASSWORD_ADMIN=admin-password-secret123",
			expectedSecrets: 1,
			expectedType:    SecretTypeEnvironmentVar,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ScanConfig{
				Content:  tt.content,
				FilePath: "test.env",
				Options: ScanOptions{
					IncludeHighEntropy: false,
				},
			}

			ctx := context.Background()
			result, err := scanner.Scan(ctx, config)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedSecrets, len(result.Secrets))

			if tt.expectedSecrets > 0 {
				assert.Equal(t, tt.expectedType, result.Secrets[0].Type)
			}
		})
	}
}

func TestRegexBasedScanner_Scan_HighEntropy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewRegexBasedScanner(logger)

	tests := []struct {
		name            string
		content         string
		includeEntropy  bool
		expectedSecrets int
		expectedType    SecretType
	}{
		{
			name:            "high_entropy_detected",
			content:         `data_value = "4f8b9c2e1a7d5369bf04e8a2c6d93f718e5b4a7c9d2e6f3a8b1c4d7e9f0a2b5c"`,
			includeEntropy:  true,
			expectedSecrets: 1,
			expectedType:    SecretTypeHighEntropy,
		},
		{
			name:            "high_entropy_disabled",
			content:         `data_value = "aB3dE6fG9hI2jK5lM8nO1pQ4rS7tU0vW3xY6zA9bC2dE5fG8hI1jK4lM7nO0pQ3rS6tU9vW2xY5zA8bC1dE4"`,
			includeEntropy:  false,
			expectedSecrets: 0, // Should not detect when entropy scanning is disabled
		},
		{
			name:            "low_entropy_string",
			content:         `simple_value = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`,
			includeEntropy:  true,
			expectedSecrets: 0, // Low entropy, should not be detected
		},
		{
			name:            "short_high_entropy",
			content:         `short = "aB3dE6fG9h"`,
			includeEntropy:  true,
			expectedSecrets: 0, // Too short, should not be detected
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ScanConfig{
				Content:  tt.content,
				FilePath: "test.config",
				Options: ScanOptions{
					IncludeHighEntropy: tt.includeEntropy,
				},
			}

			ctx := context.Background()
			result, err := scanner.Scan(ctx, config)

			require.NoError(t, err)

			assert.Equal(t, tt.expectedSecrets, len(result.Secrets))

			if tt.expectedSecrets > 0 && len(result.Secrets) > 0 {
				t.Logf("Secret detected: Type=%s, Entropy=%f", result.Secrets[0].Type, result.Secrets[0].Entropy)
				assert.Equal(t, tt.expectedType, result.Secrets[0].Type)
				assert.Greater(t, result.Secrets[0].Entropy, 0.6)
			}
		})
	}
}

func TestRegexBasedScanner_calculateSecretConfidence(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewRegexBasedScanner(logger)

	tests := []struct {
		name        string
		secretType  SecretType
		value       string
		context     string
		expectedMin float64
		expectedMax float64
	}{
		{
			name:        "long_mixed_case_with_numbers",
			secretType:  SecretTypeAPIKey,
			value:       "aB3dE6fG9hI2jK5lM8nO1pQ4rS7tU0vW3x",
			context:     "API_KEY=aB3dE6fG9hI2jK5lM8nO1pQ4rS7tU0vW3x",
			expectedMin: 0.7,
			expectedMax: 1.0,
		},
		{
			name:        "short_lowercase_only",
			secretType:  SecretTypePassword,
			value:       "password",
			context:     "PASSWORD=password",
			expectedMin: 0.0,
			expectedMax: 0.2,
		},
		{
			name:        "example_context",
			secretType:  SecretTypeToken,
			value:       "real-token-value",
			context:     "# Example: TOKEN=real-token-value",
			expectedMin: 0.0,
			expectedMax: 0.3,
		},
		{
			name:        "test_context",
			secretType:  SecretTypeSecret,
			value:       "test-secret",
			context:     "# Test secret: test-secret",
			expectedMin: 0.0,
			expectedMax: 0.3,
		},
		{
			name:        "obvious_non_secret",
			secretType:  SecretTypeAPIKey,
			value:       "your_api_key_here",
			context:     "API_KEY=your_api_key_here",
			expectedMin: 0.0,
			expectedMax: 0.15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := scanner.calculateSecretConfidence(tt.secretType, tt.value, tt.context)
			assert.GreaterOrEqual(t, confidence, tt.expectedMin)
			assert.LessOrEqual(t, confidence, tt.expectedMax)
		})
	}
}

func TestRegexBasedScanner_calculateEntropyConfidence(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewRegexBasedScanner(logger)

	tests := []struct {
		name        string
		entropy     float64
		value       string
		expectedMin float64
		expectedMax float64
	}{
		{
			name:        "high_entropy_good_length",
			entropy:     0.75, // Normalized entropy
			value:       "aB3dE6fG9hI2jK5lM8nO1pQ4rS7tU0vW",
			expectedMin: 0.4,
			expectedMax: 0.7,
		},
		{
			name:        "very_high_entropy",
			entropy:     0.9, // Normalized entropy
			value:       "aB3dE6fG9hI2jK5lM8nO1pQ4rS7tU0vW",
			expectedMin: 0.8,
			expectedMax: 1.0,
		},
		{
			name:        "high_entropy_too_short",
			entropy:     0.75, // Normalized entropy
			value:       "aB3dE6fG9h",
			expectedMin: 0.0,
			expectedMax: 0.5,
		},
		{
			name:        "high_entropy_too_long",
			entropy:     0.75, // Normalized entropy
			value:       "aB3dE6fG9hI2jK5lM8nO1pQ4rS7tU0vW3xY6zA9bC2dE5fG8hI1jK4lM7nO0pQ3rS6tU9vW2xY5zA8bC1dE4fG7hI0jK3lM6nO9pQ2rS5tU8vW1xY4zA7bC0dE3fG6hI9jK2lM5nO8pQ1rS4tU7vW0xY3zA6bC9dE2fG5hI8jK1lM4nO7pQ0rS3tU6vW9xY2zA5bC8dE1fG4hI7jK0lM3nO6pQ9rS2tU5vW8xY1zA4bC7dE0fG3hI6jK9lM2nO5pQ8rS1tU4vW7xY0zA3bC6dE9fG2hI5jK8lM1nO4pQ7rS0tU3vW6xY9zA2bC5dE8fG1hI4jK7lM0nO3pQ6rS9tU2vW5xY8zA1bC4dE7fG0hI3jK6lM9nO2pQ5rS8tU1vW4xY7zA0bC3dE6fG9hI2jK5lM8nO1pQ4rS7tU0vW3xY6zA9bC2dE5fG8hI1jK4lM7nO0pQ3rS6tU9vW2xY5zA8bC1dE4",
			expectedMin: 0.2,
			expectedMax: 0.6,
		},
		{
			name:        "threshold_entropy",
			entropy:     0.55, // Normalized entropy
			value:       "aB3dE6fG9hI2jK5lM8nO",
			expectedMin: 0.0,
			expectedMax: 0.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := scanner.calculateEntropyConfidence(tt.entropy, tt.value)
			assert.GreaterOrEqual(t, confidence, tt.expectedMin)
			assert.LessOrEqual(t, confidence, tt.expectedMax)
		})
	}
}

func TestRegexBasedScanner_extractTokens(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewRegexBasedScanner(logger)

	tests := []struct {
		name     string
		line     string
		expected []string
	}{
		{
			name:     "quoted_strings",
			line:     `api_key = "sk-1234567890abcdef1234567890abcdef12345678"`,
			expected: []string{"sk-1234567890abcdef1234567890abcdef12345678"},
		},
		{
			name:     "key_value_pairs",
			line:     "token: bearer-token-abcdef123456789012345678",
			expected: []string{"bearer-token-abcdef123456789012345678"},
		},
		{
			name:     "base64_like",
			line:     "data: aGVsbG8gd29ybGQgdGhpcyBpcyBhIGxvbmcgYmFzZTY0IHN0cmluZw==",
			expected: []string{"aGVsbG8gd29ybGQgdGhpcyBpcyBhIGxvbmcgYmFzZTY0IHN0cmluZw=="},
		},
		{
			name:     "hex_strings",
			line:     "hash: deadbeef12345678901234567890abcdef123456789012345678901234567890",
			expected: []string{"deadbeef12345678901234567890abcdef123456789012345678901234567890"},
		},
		{
			name:     "multiple_tokens",
			line:     `key1="token1234567890abcdef" key2=token2abcdef123456789012345`,
			expected: []string{"token1234567890abcdef", "token2abcdef123456789012345"},
		},
		{
			name:     "no_tokens",
			line:     "just some regular text without any tokens",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := scanner.extractTokens(tt.line)

			if len(tt.expected) == 0 {
				assert.Empty(t, tokens)
			} else {
				for _, expectedToken := range tt.expected {
					assert.Contains(t, tokens, expectedToken)
				}
			}
		})
	}
}

func TestRegexBasedScanner_MultiLineContent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewRegexBasedScanner(logger)

	content := `# Configuration file
API_KEY=sk-test123456789abcdef
PASSWORD=mypassword123
TOKEN=access-token-xyz789012345
SECRET=client-secret-abc123456789
# End of config`

	config := ScanConfig{
		Content:  content,
		FilePath: "config.env",
		Options: ScanOptions{
			IncludeHighEntropy: false,
		},
	}

	ctx := context.Background()
	result, err := scanner.Scan(ctx, config)

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.GreaterOrEqual(t, len(result.Secrets), 4) // Should find at least 4 secrets

	// Verify each secret has proper location information
	for _, secret := range result.Secrets {
		assert.Greater(t, secret.Location.Line, 0)
		assert.Greater(t, secret.Location.Column, 0)
		assert.Equal(t, "config.env", secret.Location.File)
		assert.NotEmpty(t, secret.Context)
	}
}

func TestRegexBasedScanner_calculateConfidence(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewRegexBasedScanner(logger)

	tests := []struct {
		name     string
		result   *ScanResult
		expected float64
	}{
		{
			name: "no_secrets",
			result: &ScanResult{
				Secrets: []Secret{},
			},
			expected: 0.0,
		},
		{
			name: "single_secret",
			result: &ScanResult{
				Secrets: []Secret{
					{Confidence: 0.8},
				},
			},
			expected: 0.8,
		},
		{
			name: "multiple_secrets",
			result: &ScanResult{
				Secrets: []Secret{
					{Confidence: 0.9},
					{Confidence: 0.7},
					{Confidence: 0.8},
				},
			},
			expected: 0.8, // Average of 0.9, 0.7, 0.8
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := scanner.calculateConfidence(tt.result)
			assert.InDelta(t, tt.expected, confidence, 0.01)
		})
	}
}

func TestRegexBasedScanner_getPatternString(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewRegexBasedScanner(logger)

	// Test with known pattern
	patternString := scanner.getPatternString(SecretTypeAPIKey)
	assert.NotEqual(t, "unknown", patternString)
	assert.Contains(t, patternString, "api")

	// Test with unknown pattern
	unknownPatternString := scanner.getPatternString(SecretType("nonexistent"))
	assert.Equal(t, "unknown", unknownPatternString)
}

func TestRegexBasedScanner_ErrorHandling(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	scanner := NewRegexBasedScanner(logger)

	// Test with empty content
	config := ScanConfig{
		Content:  "",
		FilePath: "empty.txt",
		Options: ScanOptions{
			IncludeHighEntropy: false,
		},
	}

	ctx := context.Background()
	result, err := scanner.Scan(ctx, config)

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 0, len(result.Secrets))
	assert.Equal(t, 0, len(result.Errors))
}

// BenchmarkRegexBasedScanner_Scan benchmarks the scanning performance
func BenchmarkRegexBasedScanner_Scan(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	scanner := NewRegexBasedScanner(logger)

	content := `
API_KEY=sk-test123456789abcdef
PASSWORD=mypassword123
TOKEN=access-token-xyz789012345
SECRET=client-secret-abc123456789
credential=user-credential-def456789
KEY_DATABASE=db-key-ghi789012345
`

	config := ScanConfig{
		Content:  content,
		FilePath: "benchmark.env",
		Options: ScanOptions{
			IncludeHighEntropy: true,
		},
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := scanner.Scan(ctx, config)
		require.NoError(b, err)
	}
}
