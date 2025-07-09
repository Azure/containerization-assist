package utils

import (
	"regexp"
	"strings"
)

// SanitizeRegistryError removes sensitive information from registry error messages
func SanitizeRegistryError(errorMsg, output string) (string, string) {
	// Patterns that might contain sensitive information
	sensitivePatterns := []struct {
		pattern     *regexp.Regexp
		replacement string
	}{
		// Basic auth in URLs
		{
			pattern:     regexp.MustCompile(`https?://[^:]+:[^@]+@`),
			replacement: "https://[REDACTED]:[REDACTED]@",
		},
		// Docker config auth tokens
		{
			pattern:     regexp.MustCompile(`"auth":\s*"[^"]+"`),
			replacement: `"auth": "[REDACTED]"`,
		},
		// Azure/AWS/GCP tokens
		{
			pattern:     regexp.MustCompile(`([Tt]oken|[Kk]ey|[Ss]ecret|[Pp]assword)[\s=:]+[A-Za-z0-9\-\._~\+\/]+=*`),
			replacement: "$1=[REDACTED]",
		},
		// JWT tokens
		{
			pattern:     regexp.MustCompile(`eyJ[A-Za-z0-9\-_]+\.eyJ[A-Za-z0-9\-_]+\.[A-Za-z0-9\-_]+`),
			replacement: "[JWT_REDACTED]",
		},
		// Generic tokens after common keywords
		{
			pattern:     regexp.MustCompile(`([Tt]oken|[Bb]earer)[\s:=]+[A-Za-z0-9\-\._~\+\/]+=*`),
			replacement: "$1=[REDACTED]",
		},
		// Docker registry tokens
		{
			pattern:     regexp.MustCompile(`[Dd]ocker-[Bb]earer\s+[A-Za-z0-9\-\._~\+\/]+=*`),
			replacement: "Docker-Bearer [REDACTED]",
		},
		// Generic base64 encoded credentials after "Authorization:" header
		{
			pattern:     regexp.MustCompile(`[Aa]uthorization:\s*[A-Za-z]+\s+[A-Za-z0-9\+\/]+=*`),
			replacement: "Authorization: [REDACTED]",
		},
	}

	// Apply sanitization to both error message and output
	sanitizedError := errorMsg
	sanitizedOutput := output

	for _, sp := range sensitivePatterns {
		sanitizedError = sp.pattern.ReplaceAllString(sanitizedError, sp.replacement)
		sanitizedOutput = sp.pattern.ReplaceAllString(sanitizedOutput, sp.replacement)
	}

	return sanitizedError, sanitizedOutput
}

// IsAuthenticationError checks if an error is related to authentication
func IsAuthenticationError(err error, output string) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	outputStr := strings.ToLower(output)
	combined := errStr + " " + outputStr

	authIndicators := []string{
		"401",
		"unauthorized",
		"authentication required",
		"authentication failed",
		"access denied",
		"forbidden",
		"invalid credentials",
		"login required",
		"not authenticated",
		"token expired",
		"invalid token",
	}

	for _, indicator := range authIndicators {
		if strings.Contains(combined, indicator) {
			return true
		}
	}

	return false
}

// GetAuthErrorGuidance provides user-friendly guidance for authentication errors
func GetAuthErrorGuidance(registry string) []string {
	guidance := []string{
		"Authentication failed. Please re-authenticate with the registry.",
	}

	// Add registry-specific guidance
	if strings.Contains(registry, "azurecr.io") {
		guidance = append(guidance,
			"For Azure Container Registry:",
			"  az acr login --name <registry-name>",
			"  Or use: docker login <registry>.azurecr.io",
		)
	} else if strings.Contains(registry, "gcr.io") || strings.Contains(registry, "pkg.dev") {
		guidance = append(guidance,
			"For Google Container Registry:",
			"  gcloud auth configure-docker",
			"  Or use: docker login gcr.io",
		)
	} else if strings.Contains(registry, "amazonaws.com") {
		guidance = append(guidance,
			"For Amazon ECR:",
			"  aws ecr get-login-password | docker login --username AWS --password-stdin <registry>",
		)
	} else if registry == "docker.io" || strings.Contains(registry, "docker.com") {
		guidance = append(guidance,
			"For Docker Hub:",
			"  docker login",
			"  Or use: docker login docker.io",
		)
	} else {
		guidance = append(guidance,
			"For private registries:",
			"  docker login <registry-url>",
			"  Ensure your credentials are up to date",
		)
	}

	guidance = append(guidance,
		"",
		"After re-authenticating, retry the push operation.",
	)

	return guidance
}
