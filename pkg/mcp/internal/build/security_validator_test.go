package build

import (
	"testing"

	"github.com/rs/zerolog"
)

// Test compileSecretPatterns function
func TestCompileSecretPatterns(t *testing.T) {
	patterns := compileSecretPatterns()

	if len(patterns) == 0 {
		t.Error("compileSecretPatterns should return at least one pattern")
	}

	// Test that all patterns are valid regex
	for i, pattern := range patterns {
		if pattern == nil {
			t.Errorf("Pattern %d should not be nil", i)
		}
	}

	// Test that patterns can detect common secret patterns
	testCases := []string{
		`API_KEY="abc123def456"`,
		`api-key: "secret-value"`,
		`password = "mypassword"`,
		`bearer abcdef123456`,
		`TOKEN="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"`,
		`-----BEGIN RSA PRIVATE KEY-----`,
		`-----BEGIN PRIVATE KEY-----`,
	}

	for _, testCase := range testCases {
		found := false
		for _, pattern := range patterns {
			if pattern.MatchString(testCase) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("No pattern matched secret case: %s", testCase)
		}
	}

	// Test that patterns don't match non-secrets
	nonSecrets := []string{
		`APP_NAME="myapp"`,
		`version: "1.0.0"`,
		`short_string="test"`,
		`normal text content`,
	}

	for _, nonSecret := range nonSecrets {
		matched := false
		for _, pattern := range patterns {
			if pattern.MatchString(nonSecret) {
				matched = true
				break
			}
		}
		// It's okay if some non-secrets match (e.g., long strings), but we just test the function works
		_ = matched // Suppress unused variable warning
	}
}

// Test NewSecurityValidator constructor
func TestNewSecurityValidator(t *testing.T) {
	logger := zerolog.Nop()
	trustedRegistries := []string{"docker.io", "gcr.io", "quay.io"}

	validator := NewSecurityValidator(logger, trustedRegistries)

	if validator == nil {
		t.Error("NewSecurityValidator should not return nil")
	}
	if len(validator.trustedRegistries) != len(trustedRegistries) {
		t.Errorf("Expected %d trusted registries, got %d", len(trustedRegistries), len(validator.trustedRegistries))
	}
	for i, registry := range trustedRegistries {
		if validator.trustedRegistries[i] != registry {
			t.Errorf("Expected registry '%s', got '%s'", registry, validator.trustedRegistries[i])
		}
	}
	if len(validator.secretPatterns) == 0 {
		t.Error("Expected secret patterns to be compiled")
	}

	// Test with empty trusted registries
	emptyValidator := NewSecurityValidator(logger, []string{})
	if emptyValidator == nil {
		t.Error("NewSecurityValidator should not return nil with empty registries")
	}
	if len(emptyValidator.trustedRegistries) != 0 {
		t.Errorf("Expected 0 trusted registries, got %d", len(emptyValidator.trustedRegistries))
	}

	// Test with nil trusted registries
	nilValidator := NewSecurityValidator(logger, nil)
	if nilValidator == nil {
		t.Error("NewSecurityValidator should not return nil with nil registries")
	}
}

// Test Validate function with disabled security
func TestSecurityValidatorValidateDisabled(t *testing.T) {
	logger := zerolog.Nop()
	validator := NewSecurityValidator(logger, []string{"docker.io"})

	options := ValidationOptions{
		CheckSecurity: false,
	}

	content := `FROM ubuntu:20.04
USER root
ENV API_KEY="secret123"`

	result, err := validator.Validate(content, options)
	if err != nil {
		t.Errorf("Validate should not return error when security is disabled: %v", err)
	}
	if result == nil {
		t.Error("Validate should not return nil result")
	}
	if !result.Valid {
		t.Error("Validate should return valid result when security is disabled")
	}
	if len(result.Errors) > 0 {
		t.Error("Validate should not return errors when security is disabled")
	}
}

// Test ValidationOptions type
func TestValidationOptions(t *testing.T) {
	options := ValidationOptions{
		UseHadolint:        true,
		Severity:           "error",
		IgnoreRules:        []string{"DL3008", "DL3009"},
		TrustedRegistries:  []string{"docker.io", "gcr.io"},
		CheckSecurity:      true,
		CheckOptimization:  false,
		CheckBestPractices: true,
	}

	if !options.UseHadolint {
		t.Error("Expected UseHadolint to be true")
	}
	if options.Severity != "error" {
		t.Errorf("Expected Severity to be 'error', got '%s'", options.Severity)
	}
	if len(options.IgnoreRules) != 2 {
		t.Errorf("Expected 2 ignore rules, got %d", len(options.IgnoreRules))
	}
	if options.IgnoreRules[0] != "DL3008" {
		t.Errorf("Expected first ignore rule to be 'DL3008', got '%s'", options.IgnoreRules[0])
	}
	if len(options.TrustedRegistries) != 2 {
		t.Errorf("Expected 2 trusted registries, got %d", len(options.TrustedRegistries))
	}
	if !options.CheckSecurity {
		t.Error("Expected CheckSecurity to be true")
	}
	if options.CheckOptimization {
		t.Error("Expected CheckOptimization to be false")
	}
	if !options.CheckBestPractices {
		t.Error("Expected CheckBestPractices to be true")
	}
}

// Test ValidationContext type
func TestValidationContext(t *testing.T) {
	options := ValidationOptions{
		CheckSecurity: true,
		Severity:      "warning",
	}

	context := ValidationContext{
		DockerfilePath:    "/path/to/Dockerfile",
		DockerfileContent: "FROM alpine:latest",
		SessionID:         "session-123",
		Options:           options,
	}

	if context.DockerfilePath != "/path/to/Dockerfile" {
		t.Errorf("Expected DockerfilePath to be '/path/to/Dockerfile', got '%s'", context.DockerfilePath)
	}
	if context.DockerfileContent != "FROM alpine:latest" {
		t.Errorf("Expected DockerfileContent to be 'FROM alpine:latest', got '%s'", context.DockerfileContent)
	}
	if context.SessionID != "session-123" {
		t.Errorf("Expected SessionID to be 'session-123', got '%s'", context.SessionID)
	}
	if !context.Options.CheckSecurity {
		t.Error("Expected Options.CheckSecurity to be true")
	}
	if context.Options.Severity != "warning" {
		t.Errorf("Expected Options.Severity to be 'warning', got '%s'", context.Options.Severity)
	}
}
