package utils

import (
	"strings"
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/errors"
)

func TestSanitizeRegistryError(t *testing.T) {
	tests := []struct {
		name         string
		errorMsg     string
		output       string
		wantError    string
		wantOutput   string
		shouldRedact bool
	}{
		{
			name:         "Bearer token in error",
			errorMsg:     "401 Unauthorized: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			output:       "",
			wantError:    "401 Unauthorized: Bearer [JWT_REDACTED]",
			wantOutput:   "",
			shouldRedact: true,
		},
		{
			name:         "Basic auth in URL",
			errorMsg:     "Failed to push to https://user:password123@myregistry.azurecr.io/v2/",
			output:       "",
			wantError:    "Failed to push to https://[REDACTED]:[REDACTED]@myregistry.azurecr.io/v2/",
			wantOutput:   "",
			shouldRedact: true,
		},
		{
			name:         "Docker config auth token",
			errorMsg:     `Error reading config: {"auths":{"registry.example.com":{"auth":"dXNlcjpwYXNzd29yZA=="}}}`,
			output:       "",
			wantError:    `Error reading config: {"auths":{"registry.example.com":{"auth": "[REDACTED]"}}}`,
			wantOutput:   "",
			shouldRedact: true,
		},
		{
			name:         "JWT token in output",
			errorMsg:     "Push failed",
			output:       "Authorization failed with token: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			wantError:    "Push failed",
			wantOutput:   "Authorization failed with token=[REDACTED]",
			shouldRedact: true,
		},
		{
			name:         "Multiple sensitive values",
			errorMsg:     "Failed with Bearer abc123def456 and password=secretpass123",
			output:       "Docker-Bearer xyz789ghi012 failed",
			wantError:    "Failed with Bearer=[REDACTED] and password=[REDACTED]",
			wantOutput:   "Docker-Bearer=[REDACTED] failed",
			shouldRedact: true,
		},
		{
			name:         "No sensitive information",
			errorMsg:     "Connection timeout: unable to reach registry",
			output:       "Network error occurred",
			wantError:    "Connection timeout: unable to reach registry",
			wantOutput:   "Network error occurred",
			shouldRedact: false,
		},
		{
			name:         "Authorization header",
			errorMsg:     "Request failed: Authorization: Bearer dGVzdDp0ZXN0",
			output:       "",
			wantError:    "Request failed: Authorization: Bearer=[REDACTED]",
			wantOutput:   "",
			shouldRedact: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotError, gotOutput := SanitizeRegistryError(tt.errorMsg, tt.output)

			if gotError != tt.wantError {
				t.Errorf("SanitizeRegistryError() error = %v, want %v", gotError, tt.wantError)
			}

			if gotOutput != tt.wantOutput {
				t.Errorf("SanitizeRegistryError() output = %v, want %v", gotOutput, tt.wantOutput)
			}

			// Verify sensitive data was redacted when expected
			if tt.shouldRedact {
				if strings.Contains(gotError, "eyJ") || strings.Contains(gotOutput, "eyJ") {
					t.Error("JWT token not properly redacted")
				}
				if strings.Contains(gotError, "password123") || strings.Contains(gotOutput, "password123") {
					t.Error("Password not properly redacted")
				}
			}
		})
	}
}

func TestIsAuthenticationError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		output   string
		wantAuth bool
	}{
		{
			name:     "401 error",
			err:      errors.Validation("test", "401 Unauthorized"),
			output:   "",
			wantAuth: true,
		},
		{
			name:     "Unauthorized in output",
			err:      errors.Validation("test", "push failed"),
			output:   "Error: unauthorized: authentication required",
			wantAuth: true,
		},
		{
			name:     "Access denied",
			err:      errors.Validation("test", "access denied: insufficient permissions"),
			output:   "",
			wantAuth: true,
		},
		{
			name:     "Token expired",
			err:      errors.Internal("test", "token expired, please re-authenticate"),
			output:   "",
			wantAuth: true,
		},
		{
			name:     "Network error",
			err:      errors.Validation("test", "connection timeout"),
			output:   "unable to reach server",
			wantAuth: false,
		},
		{
			name:     "Generic error",
			err:      errors.Validation("test", "unknown error occurred"),
			output:   "",
			wantAuth: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			output:   "some output",
			wantAuth: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAuthenticationError(tt.err, tt.output); got != tt.wantAuth {
				t.Errorf("IsAuthenticationError() = %v, want %v", got, tt.wantAuth)
			}
		})
	}
}

func TestGetAuthErrorGuidance(t *testing.T) {
	tests := []struct {
		name         string
		registry     string
		wantContains []string
	}{
		{
			name:         "Azure Container Registry",
			registry:     "myregistry.azurecr.io",
			wantContains: []string{"Azure Container Registry", "az acr login"},
		},
		{
			name:         "Google Container Registry",
			registry:     "gcr.io",
			wantContains: []string{"Google Container Registry", "gcloud auth configure-docker"},
		},
		{
			name:         "Amazon ECR",
			registry:     "123456789.dkr.ecr.us-east-1.amazonaws.com",
			wantContains: []string{"Amazon ECR", "aws ecr get-login-password"},
		},
		{
			name:         "Docker Hub",
			registry:     "docker.io",
			wantContains: []string{"Docker Hub", "docker login"},
		},
		{
			name:         "Private Registry",
			registry:     "registry.company.com",
			wantContains: []string{"private registries", "docker login"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guidance := GetAuthErrorGuidance(tt.registry)

			// Check that guidance is not empty
			if len(guidance) == 0 {
				t.Error("GetAuthErrorGuidance() returned empty guidance")
			}

			// Join all guidance for easier searching
			allGuidance := strings.Join(guidance, " ")

			// Check for expected content
			for _, expected := range tt.wantContains {
				if !strings.Contains(allGuidance, expected) {
					t.Errorf("GetAuthErrorGuidance() missing expected content: %s", expected)
				}
			}
		})
	}
}
