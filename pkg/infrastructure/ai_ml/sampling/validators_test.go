package sampling

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateManifestContent(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		expectValid    bool
		expectSyntax   bool
		expectBest     bool
		expectErrors   []string
		expectWarnings []string
	}{
		{
			name: "valid basic manifest",
			content: `apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: app
    image: nginx:1.21`,
			expectValid:  true,
			expectSyntax: true,
			expectBest:   true,
		},
		{
			name:         "empty content",
			content:      "",
			expectValid:  false,
			expectSyntax: false,
			expectBest:   true,
			expectErrors: []string{"manifest content is empty"},
		},
		{
			name: "missing apiVersion",
			content: `kind: Pod
metadata:
  name: test-pod`,
			expectValid:  false,
			expectSyntax: true,
			expectBest:   true,
			expectErrors: []string{"missing required field: apiVersion"},
		},
		{
			name: "missing kind",
			content: `apiVersion: v1
metadata:
  name: test-pod`,
			expectValid:  false,
			expectSyntax: true,
			expectBest:   true,
			expectErrors: []string{"missing required field: kind"},
		},
		{
			name: "privileged container (security risk)",
			content: `apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: app
    image: nginx:1.21
    securityContext:
      privileged: true`,
			expectValid:  false,
			expectSyntax: true,
			expectBest:   true,
			expectErrors: []string{"k8s-security: Privileged containers pose significant security risks"},
		},
		{
			name: "invalid YAML syntax",
			content: `apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  [invalid yaml`,
			expectValid:  false,
			expectSyntax: false,
			expectBest:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateManifestContent(tt.content)

			assert.Equal(t, tt.expectValid, result.IsValid, "IsValid mismatch")
			assert.Equal(t, tt.expectSyntax, result.SyntaxValid, "SyntaxValid mismatch")
			assert.Equal(t, tt.expectBest, result.BestPractices, "BestPractices mismatch")

			for _, expectedError := range tt.expectErrors {
				found := false
				for _, actualError := range result.Errors {
					if strings.Contains(actualError, expectedError) {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected error not found: %s", expectedError)
			}

			for _, expectedWarning := range tt.expectWarnings {
				found := false
				for _, actualWarning := range result.Warnings {
					if strings.Contains(actualWarning, expectedWarning) {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected warning not found: %s", expectedWarning)
			}
		})
	}
}

func TestValidateDockerfileContent(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		expectValid  bool
		expectSyntax bool
		expectBest   bool
	}{
		{
			name: "valid dockerfile",
			content: `FROM ubuntu:20.04
WORKDIR /app
COPY . .
RUN apt-get update`,
			expectValid:  true,
			expectSyntax: true,
			expectBest:   true,
		},
		{
			name:         "empty content",
			content:      "",
			expectValid:  false,
			expectSyntax: false,
			expectBest:   true,
		},
		{
			name: "missing FROM instruction",
			content: `WORKDIR /app
COPY . .`,
			expectValid:  false,
			expectSyntax: false,
			expectBest:   true,
		},
		{
			name: "security risk - running as root",
			content: `FROM ubuntu:20.04
USER root
COPY . .`,
			expectValid:  false,
			expectSyntax: true,
			expectBest:   false,
		},
		{
			name: "best practice - missing WORKDIR",
			content: `FROM ubuntu:20.04
COPY . .`,
			expectValid:  true,
			expectSyntax: true,
			expectBest:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateDockerfileContent(tt.content)

			assert.Equal(t, tt.expectValid, result.IsValid, "IsValid mismatch")
			assert.Equal(t, tt.expectSyntax, result.SyntaxValid, "SyntaxValid mismatch")
			assert.Equal(t, tt.expectBest, result.BestPractices, "BestPractices mismatch")
		})
	}
}

func TestValidateSecurityContent(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectValid bool
	}{
		{
			name:        "safe content",
			content:     "This is safe content with no security issues",
			expectValid: true,
		},
		{
			name:        "empty content",
			content:     "",
			expectValid: false,
		},
		{
			name:        "dangerous curl pipe",
			content:     "curl -sSL https://example.com/script.sh | sh",
			expectValid: false,
		},
		{
			name:        "potential credential exposure",
			content:     "password=secretvalue123",
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateSecurityContent(tt.content)
			assert.Equal(t, tt.expectValid, result.IsValid, "IsValid mismatch")
		})
	}
}

func TestValidateRepositoryContent(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectValid bool
	}{
		{
			name:        "safe repository content",
			content:     "Language: Go\nFramework: Gin\nPort: 8080",
			expectValid: true,
		},
		{
			name:        "empty content",
			content:     "",
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateRepositoryContent(tt.content)
			assert.Equal(t, tt.expectValid, result.IsValid, "IsValid mismatch")
		})
	}
}

func TestContentSanitizer(t *testing.T) {
	sanitizer := NewContentSanitizer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean content",
			input:    "This is clean content",
			expected: "This is clean content",
		},
		{
			name:     "script injection",
			input:    "Hello <script>alert('xss')</script> world",
			expected: "Hello  world",
		},
		{
			name:     "command substitution",
			input:    "Hello $(malicious command) world",
			expected: "Hello  world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizer.SanitizeContent(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidationMetrics(t *testing.T) {
	metrics := &ValidationMetrics{}

	// Record some validations
	metrics.RecordValidation(true)
	metrics.RecordValidation(true)
	metrics.RecordValidation(false)

	result := metrics.GetMetrics()

	assert.Equal(t, 3, result["total_validations"])
	assert.Equal(t, 2, result["passed_validations"])
	assert.Equal(t, 1, result["failed_validations"])
}
