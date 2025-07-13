package utilities

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestWithAIRetry tests basic retry functionality
func TestWithAIRetry(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("success on first attempt", func(t *testing.T) {
		callCount := 0
		fn := func() error {
			callCount++
			return nil
		}

		err := WithAIRetry(context.Background(), "test_operation", 3, fn, logger)
		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)
	})

	t.Run("success after retries", func(t *testing.T) {
		callCount := 0
		fn := func() error {
			callCount++
			if callCount < 3 {
				return errors.New("temporary failure")
			}
			return nil
		}

		err := WithAIRetry(context.Background(), "test_operation", 5, fn, logger)
		assert.NoError(t, err)
		assert.Equal(t, 3, callCount)
	})

	t.Run("failure after max retries", func(t *testing.T) {
		callCount := 0
		fn := func() error {
			callCount++
			return errors.New("persistent failure")
		}

		err := WithAIRetry(context.Background(), "test_operation", 3, fn, logger)
		assert.Error(t, err)
		assert.Equal(t, 3, callCount)
		assert.Contains(t, err.Error(), "AI ASSISTANT")
	})
}

// TestRetryableOperation tests the RetryableOperation wrapper
func TestRetryableOperation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("retryable operation execution", func(t *testing.T) {
		op := NewRetryableOperation("test_op", 2, logger)
		assert.Equal(t, "test_op", op.Name)
		assert.Equal(t, 2, op.MaxRetries)

		callCount := 0
		fn := func() error {
			callCount++
			return nil
		}

		err := op.Execute(context.Background(), fn)
		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)
	})
}

// TestGenerateFixSuggestions tests the fix suggestion logic
func TestGenerateFixSuggestions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	testCases := []struct {
		name               string
		operation          string
		errorMsg           string
		expectedSubstrings []string
	}{
		{
			name:               "Maven command not found",
			operation:          "build_image",
			errorMsg:           "mvn: command not found",
			expectedSubstrings: []string{"Maven is not installed", "maven:3.9-eclipse-temurin-17"},
		},
		{
			name:               "Gradle command not found",
			operation:          "build_image",
			errorMsg:           "gradle: command not found",
			expectedSubstrings: []string{"Gradle is not installed", "gradle:8-jdk17"},
		},
		{
			name:               "Dockerfile syntax error",
			operation:          "build_image",
			errorMsg:           "Dockerfile syntax error: unknown instruction",
			expectedSubstrings: []string{"Check Dockerfile syntax", "instruction names"},
		},
		{
			name:               "Port connection refused",
			operation:          "deploy_to_k8s",
			errorMsg:           "port 8080 connection refused",
			expectedSubstrings: []string{"Verify application listens", "port bindings"},
		},
		{
			name:               "Kind cluster not found",
			operation:          "deploy_to_k8s",
			errorMsg:           "kind cluster not found",
			expectedSubstrings: []string{"kind cluster 'container-kit'", "kubectl are installed"},
		},
		{
			name:               "Permission denied",
			operation:          "build_image",
			errorMsg:           "permission denied",
			expectedSubstrings: []string{"Check file permissions", "Docker daemon permissions"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			suggestions := generateFixSuggestions(tc.operation, tc.errorMsg, logger)
			assert.NotEmpty(t, suggestions)

			for _, expected := range tc.expectedSubstrings {
				assert.Contains(t, suggestions, expected,
					"Expected suggestion to contain '%s' for error: %s", expected, tc.errorMsg)
			}
		})
	}
}

// TestContainsPattern tests the pattern matching logic
func TestContainsPattern(t *testing.T) {
	testCases := []struct {
		prompt   string
		patterns []string
		expected bool
	}{
		{
			prompt:   "Docker build failed: mvn command not found",
			patterns: []string{"mvn", "maven"},
			expected: true,
		},
		{
			prompt:   "GRADLE BUILD FAILED",
			patterns: []string{"gradle"},
			expected: true,
		},
		{
			prompt:   "Connection refused on port 8080",
			patterns: []string{"port", "connection"},
			expected: true,
		},
		{
			prompt:   "File not found error",
			patterns: []string{"docker", "kubernetes"},
			expected: false,
		},
	}

	for _, tc := range testCases {
		result := containsPattern(tc.prompt, tc.patterns...)
		assert.Equal(t, tc.expected, result,
			"Pattern matching failed for prompt: %s with patterns: %v", tc.prompt, tc.patterns)
	}
}

// TestApplyFixStepsIntegration tests the fix application logic
func TestApplyFixStepsIntegration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("apply single fix step", func(t *testing.T) {
		// Test with a step that shouldn't match any fix patterns
		applied, err := applySingleFixStep(context.Background(), "unknown fix step", logger)
		assert.NoError(t, err)
		assert.False(t, applied) // Should not apply any fixes
	})

	t.Run("permission fix step", func(t *testing.T) {
		// Test permission fix (this should work without actual files)
		applied, err := applySingleFixStep(context.Background(), "fix file permissions with chmod", logger)
		assert.NoError(t, err)
		// Applied will be false because the test files don't exist
		assert.False(t, applied)
	})
}

// TestDockerfileFixes tests Dockerfile manipulation functions
func TestDockerfileFixes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("dockerfile not found", func(t *testing.T) {
		// Test when Dockerfile doesn't exist
		applied, err := applyDockerfileBaseFix("Use maven base image", logger)
		assert.NoError(t, err)
		assert.False(t, applied)
	})

	t.Run("maven dockerfile fix not found", func(t *testing.T) {
		// Test when Dockerfile doesn't exist
		applied, err := applyMavenDockerfileFix("Install Maven", logger)
		assert.NoError(t, err)
		assert.False(t, applied)
	})

	t.Run("gradle dockerfile fix not found", func(t *testing.T) {
		// Test when Dockerfile doesn't exist
		applied, err := applyGradleDockerfileFix("Install Gradle", logger)
		assert.NoError(t, err)
		assert.False(t, applied)
	})

	t.Run("port expose fix not found", func(t *testing.T) {
		// Test when Dockerfile doesn't exist
		applied, err := applyPortExposeFix("Add EXPOSE 8080", logger)
		assert.NoError(t, err)
		assert.False(t, applied)
	})
}

// TestEnvironmentAndPermissionFixes tests environment and permission fixes
func TestEnvironmentAndPermissionFixes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("environment fix", func(t *testing.T) {
		// Environment fixes should log but not apply automatically
		applied, err := applyEnvironmentFix("Set JAVA_HOME environment variable", logger)
		assert.NoError(t, err)
		assert.False(t, applied) // Environment fixes require manual intervention
	})

	t.Run("permission fix with no files", func(t *testing.T) {
		// Permission fixes should not apply when no target files exist
		applied, err := applyPermissionFix("Make scripts executable", logger)
		assert.NoError(t, err)
		assert.False(t, applied) // No script files exist to fix
	})
}

// TestEnhanceErrorForAI tests the error enhancement for AI assistance
func TestEnhanceErrorForAI(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	originalErr := errors.New("build failed: mvn command not found")
	enhancedErr := enhanceErrorForAI("build_image", originalErr, 2, 3, logger)

	assert.Error(t, enhancedErr)
	errorMsg := enhancedErr.Error()

	// Check that the enhanced error contains AI-specific guidance
	assert.Contains(t, errorMsg, "AI ASSISTANT")
	assert.Contains(t, errorMsg, "containerize_and_deploy")
	assert.Contains(t, errorMsg, "TROUBLESHOOTING CHECKLIST")
	assert.Contains(t, errorMsg, "mvn command not found")
	assert.Contains(t, errorMsg, "attempt 2/3")
}

// BenchmarkAIRetry benchmarks the retry logic
func BenchmarkAIRetry(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	b.Run("successful operation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			fn := func() error { return nil }
			_ = WithAIRetry(context.Background(), "benchmark_op", 3, fn, logger)
		}
	})

	b.Run("failing operation", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			fn := func() error { return errors.New("benchmark failure") }
			_ = WithAIRetry(context.Background(), "benchmark_op", 2, fn, logger)
		}
	})
}
