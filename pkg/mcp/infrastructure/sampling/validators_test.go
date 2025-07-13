package sampling

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultValidator_ValidateManifestContent(t *testing.T) {
	validator := NewDefaultValidator()

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
    image: nginx:1.21
    resources:
      limits:
        memory: "128Mi"
        cpu: "500m"
    livenessProbe:
      httpGet:
        path: /health
        port: 80`,
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
			name: "invalid YAML",
			content: `apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  invalid yaml structure: [[[`,
			expectValid:  false,
			expectSyntax: false,
			expectBest:   false,
			expectErrors: []string{"invalid YAML syntax"},
		},
		{
			name: "missing apiVersion",
			content: `kind: Pod
metadata:
  name: test-pod`,
			expectValid:  false,
			expectSyntax: true,
			expectBest:   false,
			expectErrors: []string{"missing required field: apiVersion"},
		},
		{
			name: "missing kind",
			content: `apiVersion: v1
metadata:
  name: test-pod`,
			expectValid:  false,
			expectSyntax: true,
			expectBest:   false,
			expectErrors: []string{"missing required field: kind"},
		},
		{
			name: "missing metadata",
			content: `apiVersion: v1
kind: Pod
spec:
  containers:
  - name: app`,
			expectValid:  false,
			expectSyntax: true,
			expectBest:   false,
			expectErrors: []string{"missing required field: metadata"},
		},
		{
			name: "missing resources (best practice warning)",
			content: `apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: app
    image: nginx:1.21`,
			expectValid:    true,
			expectSyntax:   true,
			expectBest:     false,
			expectWarnings: []string{"no resource limits specified (best practice)", "no health checks configured (best practice)"},
		},
		{
			name: "privileged container (security violation)",
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
			expectBest:   false,
			expectErrors: []string{"SECURITY: Privileged containers pose significant security risks"},
		},
		{
			name: "hostNetwork security issue",
			content: `apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  hostNetwork: true
  containers:
  - name: app
    image: nginx:1.21`,
			expectValid:  false,
			expectSyntax: true,
			expectBest:   false,
			expectErrors: []string{"SECURITY: hostNetwork: true can expose pod to host networking"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateManifestContent(tt.content)

			assert.Equal(t, tt.expectValid, result.IsValid, "IsValid mismatch")
			assert.Equal(t, tt.expectSyntax, result.SyntaxValid, "SyntaxValid mismatch")
			assert.Equal(t, tt.expectBest, result.BestPractices, "BestPractices mismatch")

			for _, expectedError := range tt.expectErrors {
				found := false
				for _, actualError := range result.Errors {
					if assert.Contains(t, actualError, expectedError) {
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
				assert.True(t, found, "Expected warning not found: %s. Actual warnings: %v", expectedWarning, result.Warnings)
			}
		})
	}
}

func TestDefaultValidator_ValidateDockerfileContent(t *testing.T) {
	validator := NewDefaultValidator()

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
			name: "valid dockerfile",
			content: `FROM node:16-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
USER node
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:3000/health || exit 1
CMD ["npm", "start"]`,
			expectValid:    true,
			expectSyntax:   true,
			expectBest:     false,
			expectWarnings: []string{"consider using multi-stage build"},
		},
		{
			name:         "empty content",
			content:      "",
			expectValid:  false,
			expectSyntax: false,
			expectBest:   false,
			expectErrors: []string{"dockerfile content is empty"},
		},
		{
			name: "missing FROM instruction",
			content: `WORKDIR /app
COPY . .
RUN npm install`,
			expectValid:  false,
			expectSyntax: false,
			expectBest:   false,
			expectErrors: []string{"dockerfile must start with FROM instruction"},
		},
		{
			name: "using latest tag (warning)",
			content: `FROM node:latest
WORKDIR /app
COPY . .
RUN npm install`,
			expectValid:    true,
			expectSyntax:   true,
			expectBest:     false,
			expectWarnings: []string{"SECURITY: Using 'latest' tag is discouraged", "no non-root USER specified", "no HEALTHCHECK specified"},
		},
		{
			name: "using sudo (security violation)",
			content: `FROM ubuntu:20.04
WORKDIR /app
RUN sudo apt-get update && sudo apt-get install -y curl
COPY . .`,
			expectValid:    false,
			expectSyntax:   true,
			expectBest:     false,
			expectErrors:   []string{"SECURITY: Using sudo in Docker containers is potentially unsafe"},
			expectWarnings: []string{"consider using multi-stage build"},
		},
		{
			name: "running as root (security violation)",
			content: `FROM ubuntu:20.04
WORKDIR /app
USER root
COPY . .`,
			expectValid:    false,
			expectSyntax:   true,
			expectBest:     false,
			expectErrors:   []string{"SECURITY: Running as root user is a security risk"},
			expectWarnings: []string{"consider using multi-stage build"},
		},
		{
			name: "curl pipe sh (critical security violation)",
			content: `FROM ubuntu:20.04
WORKDIR /app
RUN curl -fsSL https://example.com/install.sh | sh
COPY . .`,
			expectValid:    false,
			expectSyntax:   true,
			expectBest:     false,
			expectErrors:   []string{"SECURITY: Downloading and executing scripts via curl|sh is extremely risky"},
			expectWarnings: []string{"consider using multi-stage build"},
		},
		{
			name: "chmod 777 (security warning)",
			content: `FROM ubuntu:20.04
WORKDIR /app
COPY . .
RUN chmod 777 /app`,
			expectValid:    false,
			expectSyntax:   true,
			expectBest:     false,
			expectErrors:   []string{"SECURITY: Setting permissions to 777 is insecure"},
			expectWarnings: []string{"consider using multi-stage build"},
		},
		{
			name: "best practice warnings",
			content: `FROM node:16-alpine
COPY . .
RUN npm install`,
			expectValid:    true,
			expectSyntax:   true,
			expectBest:     false,
			expectWarnings: []string{"no WORKDIR specified", "no non-root USER specified", "no HEALTHCHECK specified", "consider using multi-stage build"},
		},
		{
			name: "multi-stage build (no warning)",
			content: `FROM node:16-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production

FROM node:16-alpine
WORKDIR /app
COPY --from=builder /app/node_modules ./node_modules
COPY . .
USER node
CMD ["npm", "start"]`,
			expectValid:    true,
			expectSyntax:   true,
			expectBest:     false, // Still missing HEALTHCHECK
			expectWarnings: []string{"no HEALTHCHECK specified"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateDockerfileContent(tt.content)

			assert.Equal(t, tt.expectValid, result.IsValid, "IsValid mismatch")
			assert.Equal(t, tt.expectSyntax, result.SyntaxValid, "SyntaxValid mismatch")
			assert.Equal(t, tt.expectBest, result.BestPractices, "BestPractices mismatch")

			for _, expectedError := range tt.expectErrors {
				found := false
				for _, actualError := range result.Errors {
					if assert.Contains(t, actualError, expectedError) {
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
				assert.True(t, found, "Expected warning not found: %s. Actual warnings: %v", expectedWarning, result.Warnings)
			}
		})
	}
}

func TestDefaultValidator_ValidateSecurityContent(t *testing.T) {
	validator := NewDefaultValidator()

	tests := []struct {
		name           string
		content        string
		expectValid    bool
		expectBest     bool
		expectErrors   []string
		expectWarnings []string
	}{
		{
			name: "valid security analysis",
			content: `Security Analysis Results:
			
Critical Vulnerabilities:
- CVE-2023-1234: SQL injection vulnerability in package X

Risk Assessment: HIGH

Remediation Steps:
- Update package X to version 2.1.0 or later
- Implement input validation
- Fix the configuration issue

This fix should resolve the security concerns.`,
			expectValid: true,
			expectBest:  true,
		},
		{
			name:         "empty content",
			content:      "",
			expectValid:  false,
			expectBest:   true,
			expectErrors: []string{"security analysis content is empty"},
		},
		{
			name: "missing security information",
			content: `This is a general analysis that doesn't contain any security information.
			It talks about performance and features but no security issues.`,
			expectValid:    false,
			expectBest:     false,
			expectErrors:   []string{"security analysis should contain vulnerability, risk, or remediation information"},
			expectWarnings: []string{"security analysis should provide actionable remediation steps"},
		},
		{
			name: "no actionable steps",
			content: `Security vulnerabilities found:
- CVE-2023-1234: Critical issue
- CVE-2023-5678: High severity

The risk level is critical but no specific remediation provided.`,
			expectValid:    true,
			expectBest:     false,
			expectWarnings: []string{"security analysis should provide actionable remediation steps"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateSecurityContent(tt.content)

			assert.Equal(t, tt.expectValid, result.IsValid, "IsValid mismatch")
			assert.Equal(t, tt.expectBest, result.BestPractices, "BestPractices mismatch")

			for _, expectedError := range tt.expectErrors {
				found := false
				for _, actualError := range result.Errors {
					if assert.Contains(t, actualError, expectedError) {
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
				assert.True(t, found, "Expected warning not found: %s. Actual warnings: %v", expectedWarning, result.Warnings)
			}
		})
	}
}

func TestDefaultValidator_ValidateRepositoryContent(t *testing.T) {
	validator := NewDefaultValidator()

	tests := []struct {
		name           string
		content        string
		expectValid    bool
		expectBest     bool
		expectErrors   []string
		expectWarnings []string
	}{
		{
			name: "valid repository analysis",
			content: `Repository Analysis:
Language: Java
Framework: Spring Boot
Build Tool: Maven
Dependencies: spring-web, spring-data-jpa
Port: 8080
Environment Variables: DB_URL, API_KEY`,
			expectValid: true,
			expectBest:  true,
		},
		{
			name:         "empty content",
			content:      "",
			expectValid:  false,
			expectBest:   true,
			expectErrors: []string{"repository analysis content is empty"},
		},
		{
			name: "missing language",
			content: `Repository analysis shows a web application with dependencies.
Build tool appears to be npm with several packages.`,
			expectValid:  false,
			expectBest:   true,
			expectErrors: []string{"repository analysis must identify programming language"},
		},
		{
			name: "minimal analysis components",
			content: `Language: Python
This is a simple Python script.`,
			expectValid:    true,
			expectBest:     false,
			expectWarnings: []string{"repository analysis should identify framework, dependencies, build tools, or ports"},
		},
		{
			name: "comprehensive analysis",
			content: `Language: JavaScript
Framework: Express.js
Build tool: npm
Dependencies: express, mongoose, jsonwebtoken
Port: 3000`,
			expectValid: true,
			expectBest:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateRepositoryContent(tt.content)

			assert.Equal(t, tt.expectValid, result.IsValid, "IsValid mismatch")
			assert.Equal(t, tt.expectBest, result.BestPractices, "BestPractices mismatch")

			for _, expectedError := range tt.expectErrors {
				found := false
				for _, actualError := range result.Errors {
					if assert.Contains(t, actualError, expectedError) {
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
				assert.True(t, found, "Expected warning not found: %s. Actual warnings: %v", expectedWarning, result.Warnings)
			}
		})
	}
}

func TestEnhancedValidator(t *testing.T) {
	config := NewEnhancedValidationConfig()
	config.MaxContentLength = 1000 // Increased to accommodate test content
	config.RequiredLabels = []string{"app", "version"}
	config.AllowedImageSources = []string{"docker.io", "gcr.io"}
	config.BlockedInstructions = []string{"--privileged"}

	validator := NewEnhancedValidator(config)

	t.Run("content length validation", func(t *testing.T) {
		// Create a validator with a smaller limit for this test
		lengthConfig := NewEnhancedValidationConfig()
		lengthConfig.MaxContentLength = 100
		lengthValidator := NewEnhancedValidator(lengthConfig)

		longContent := "apiVersion: v1\nkind: Pod\nmetadata:\n  name: test\n" +
			"# This is a very long comment that will exceed the limit of 100 characters to trigger the length validation"

		result := lengthValidator.ValidateManifestContent(longContent)
		assert.False(t, result.IsValid)
		assert.Contains(t, result.Errors[0], "content exceeds maximum length")
	})

	t.Run("required labels validation", func(t *testing.T) {
		content := `apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  labels:
    app: myapp
spec:
  containers:
  - name: app
    image: nginx:1.21`

		result := validator.ValidateManifestContent(content)
		assert.True(t, result.IsValid)
		assert.False(t, result.BestPractices)

		found := false
		for _, warning := range result.Warnings {
			if strings.Contains(warning, "missing recommended label: version") {
				found = true
				break
			}
		}
		if !found {
			t.Logf("Warnings: %v", result.Warnings)
		}
		assert.True(t, found, "Expected warning about missing version label")
	})

	t.Run("dockerfile blocked instructions", func(t *testing.T) {
		content := `FROM ubuntu:20.04
WORKDIR /app
RUN docker run --privileged -v /:/host alpine
COPY . .`

		result := validator.ValidateDockerfileContent(content)
		assert.False(t, result.IsValid)

		found := false
		for _, err := range result.Errors {
			if assert.Contains(t, err, "blocked instruction detected: --privileged") {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected error about blocked instruction")
	})

	t.Run("dockerfile image source validation", func(t *testing.T) {
		content := `FROM malicious-registry.com/ubuntu:20.04
WORKDIR /app
COPY . .`

		result := validator.ValidateDockerfileContent(content)
		assert.True(t, result.IsValid) // Warning, not error

		found := false
		for _, warning := range result.Warnings {
			if strings.Contains(warning, "base image not from allowed registry sources") {
				found = true
				break
			}
		}
		if !found {
			t.Logf("Warnings: %v", result.Warnings)
		}
		assert.True(t, found, "Expected warning about image source")
	})
}

func TestContentSanitizer(t *testing.T) {
	validator := NewDefaultValidator()
	sanitizer := NewContentSanitizer(validator)

	t.Run("basic sanitization", func(t *testing.T) {
		content := "FROM ubuntu:20.04\r\n\x00WORKDIR /app  \t\r\nCOPY . .\r\n"
		expected := "FROM ubuntu:20.04\nWORKDIR /app\nCOPY . ."

		sanitized, result := sanitizer.SanitizeAndValidate(content, "dockerfile")

		assert.Equal(t, expected, sanitized)
		assert.True(t, result.IsValid)
	})

	t.Run("unknown content type", func(t *testing.T) {
		content := "some content"

		_, result := sanitizer.SanitizeAndValidate(content, "unknown")

		assert.False(t, result.IsValid)
		assert.Contains(t, result.Errors[0], "unknown content type")
	})
}

func TestValidationMetrics(t *testing.T) {
	metrics := NewValidationMetrics()

	// Record some validations
	metrics.RecordValidation(ValidationResult{
		IsValid:       true,
		SyntaxValid:   true,
		BestPractices: true,
	})

	metrics.RecordValidation(ValidationResult{
		IsValid:  false,
		Errors:   []string{"SECURITY: some security issue"},
		Warnings: []string{"best practice warning"},
	})

	metrics.RecordValidation(ValidationResult{
		IsValid:  false,
		Errors:   []string{"syntax error"},
		Warnings: []string{"SECURITY: security warning"},
	})

	assert.Equal(t, int64(3), metrics.TotalValidations)
	assert.Equal(t, int64(1), metrics.SuccessfulValidations)
	assert.Equal(t, int64(2), metrics.FailedValidations)
	assert.Equal(t, int64(1), metrics.SecurityIssuesFound)
	assert.Equal(t, int64(2), metrics.BestPracticeWarnings)
	assert.InDelta(t, 0.333, metrics.GetSuccessRate(), 0.01)

	metricsMap := metrics.GetMetrics()
	assert.Equal(t, int64(3), metricsMap["total_validations"])
	assert.Equal(t, int64(1), metricsMap["successful_validations"])
	assert.Equal(t, int64(2), metricsMap["failed_validations"])
	assert.InDelta(t, 0.333, metricsMap["success_rate"], 0.01)
}

func TestSecurityPatterns(t *testing.T) {
	validator := NewDefaultValidator()

	tests := []struct {
		name           string
		content        string
		expectSecurity bool
		pattern        string
	}{
		{
			name:           "AWS credentials exposure",
			content:        "ENV AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE",
			expectSecurity: true,
			pattern:        "AWS credentials exposure",
		},
		{
			name:           "Generic secret exposure",
			content:        "ENV SECRET_KEY=abcd1234567890efgh",
			expectSecurity: true,
			pattern:        "Potential secret or credential exposure",
		},
		{
			name:           "Safe environment variable",
			content:        "ENV NODE_ENV=production",
			expectSecurity: false,
		},
		{
			name:           "ADD with URL",
			content:        "ADD https://example.com/file.tar.gz /tmp/",
			expectSecurity: true,
			pattern:        "Using ADD with URLs can be a security risk",
		},
		{
			name:           "Safe COPY instruction",
			content:        "COPY ./file.tar.gz /tmp/",
			expectSecurity: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateDockerfileContent("FROM ubuntu:20.04\n" + tt.content)

			if tt.expectSecurity {
				// Should have security-related errors or warnings
				hasSecurityIssue := false
				for _, err := range result.Errors {
					if strings.Contains(err, tt.pattern) {
						hasSecurityIssue = true
						break
					}
				}
				for _, warning := range result.Warnings {
					if strings.Contains(warning, tt.pattern) {
						hasSecurityIssue = true
						break
					}
				}
				assert.True(t, hasSecurityIssue, "Expected security issue not found")
			} else {
				// Should not have security issues matching the pattern
				for _, err := range result.Errors {
					if strings.Contains(err, "SECURITY:") && tt.pattern != "" {
						assert.NotContains(t, err, tt.pattern)
					}
				}
				for _, warning := range result.Warnings {
					if strings.Contains(warning, "SECURITY:") && tt.pattern != "" {
						assert.NotContains(t, warning, tt.pattern)
					}
				}
			}
		})
	}
}

func TestComplexValidationScenarios(t *testing.T) {
	validator := NewDefaultValidator()

	t.Run("complex Kubernetes manifest with multiple issues", func(t *testing.T) {
		content := `apiVersion: v1
kind: Pod
metadata:
  name: insecure-pod
spec:
  hostNetwork: true
  containers:
  - name: app
    image: nginx:latest
    securityContext:
      privileged: true
      allowPrivilegeEscalation: true
    env:
    - name: SECRET_KEY
      value: "supersecretpassword123"`

		result := validator.ValidateManifestContent(content)

		assert.False(t, result.IsValid)
		assert.True(t, result.SyntaxValid)
		assert.False(t, result.BestPractices)

		// Should detect multiple security issues
		securityErrors := 0
		for _, err := range result.Errors {
			if assert.Contains(t, err, "SECURITY:") {
				securityErrors++
			}
		}
		assert.Greater(t, securityErrors, 2, "Should detect multiple security issues")

		// Should detect best practice violations
		bestPracticeWarnings := 0
		for _, warning := range result.Warnings {
			if assert.Contains(t, warning, "best practice") {
				bestPracticeWarnings++
			}
		}
		assert.Greater(t, bestPracticeWarnings, 0, "Should detect best practice violations")
	})

	t.Run("complex Dockerfile with security and best practice issues", func(t *testing.T) {
		content := `FROM ubuntu:latest
USER root
WORKDIR /
RUN curl -fsSL https://get.docker.com | sh
RUN chmod 777 /app
ADD https://example.com/script.sh /tmp/
COPY . .
EXPOSE 80`

		result := validator.ValidateDockerfileContent(content)

		assert.False(t, result.IsValid)
		assert.True(t, result.SyntaxValid)
		assert.False(t, result.BestPractices)

		// Should detect multiple critical security issues
		criticalErrors := 0
		for _, err := range result.Errors {
			if assert.Contains(t, err, "SECURITY:") {
				criticalErrors++
			}
		}
		assert.GreaterOrEqual(t, criticalErrors, 3, "Should detect multiple critical security issues")
	})
}
