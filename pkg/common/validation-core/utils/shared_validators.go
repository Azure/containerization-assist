package utils

import (
	"regexp"
	"strings"
)

// SharedValidationPatterns contains common validation patterns used across validators
// This eliminates duplication between docker.go, core/docker/validator.go, and other validators

// SecretDetectionPatterns contains regex patterns for detecting hardcoded secrets
var SecretDetectionPatterns = []string{
	`(?i)(password|pwd|passwd)\s*=\s*['"][^'"]+['"]`,
	`(?i)(api[_-]?key|apikey)\s*=\s*['"][^'"]+['"]`,
	`(?i)(secret|token)\s*=\s*['"][^'"]+['"]`,
	`(?i)(private[_-]?key)\s*=\s*['"][^'"]+['"]`,
	`(?i)(access[_-]?token)\s*=\s*['"][^'"]+['"]`,
	`(?i)(auth[_-]?token)\s*=\s*['"][^'"]+['"]`,
}

// SpecificSecretPatterns contains regex patterns for detecting specific cloud provider secrets
// These patterns are consolidated from multiple validator implementations to eliminate duplication
var SpecificSecretPatterns = map[string]string{
	"aws_access_key": `AKIA[0-9A-Z]{16}`,
	"aws_secret_key": `[0-9a-zA-Z/+]{40}`,
	"github_token":   `ghp_[0-9a-zA-Z]{36}`,
	"slack_token":    `xox[baprs]-[0-9]{12}-[0-9]{12}-[0-9a-zA-Z]{24}`,
	"google_api_key": `AIza[0-9A-Za-z_-]{35}`,
	"stripe_api_key": `sk_live_[0-9a-zA-Z]{24}`,
	"mongodb_conn":   `mongodb(\+srv)?://[^:]+:[^@]+@`,
}

// KubernetesNamePattern is the regex pattern for validating Kubernetes resource names
var KubernetesNamePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)

// ImageReferencePattern is the regex pattern for validating Docker image references
var ImageReferencePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]*(:([a-zA-Z0-9][a-zA-Z0-9._-]*|latest))?$`)

// PortPattern is the regex pattern for validating port numbers
var PortPattern = regexp.MustCompile(`^\d+(/tcp|/udp)?$`)

// GetCompiledSecretPatterns returns compiled regex patterns for specific secret detection
// This consolidates pattern compilation across multiple validators
func GetCompiledSecretPatterns() map[string]*regexp.Regexp {
	compiled := make(map[string]*regexp.Regexp)
	for name, pattern := range SpecificSecretPatterns {
		compiled[name] = regexp.MustCompile(pattern)
	}
	return compiled
}

// ContainsSpecificSecrets checks if content contains any specific cloud provider secrets
func ContainsSpecificSecrets(content string) bool {
	patterns := GetCompiledSecretPatterns()
	for _, pattern := range patterns {
		if pattern.MatchString(content) {
			return true
		}
	}
	return false
}

// ContainsHardcodedSecrets checks if a line contains potential hardcoded secrets
// This function consolidates duplicate secret detection logic from multiple validators
func ContainsHardcodedSecrets(line string) bool {
	for _, pattern := range SecretDetectionPatterns {
		if matched, _ := regexp.MatchString(pattern, line); matched {
			// Exclude if it's using environment variables or build args
			if !strings.Contains(line, "${") && !strings.Contains(line, "$(") {
				return true
			}
		}
	}
	return false
}

// ContainsInsecureDownload checks if a line contains insecure HTTP downloads
// This function consolidates duplicate HTTP detection logic
func ContainsInsecureDownload(line string) bool {
	return strings.Contains(strings.ToLower(line), "http://")
}

// InstallsSSH checks if a line installs SSH server
// This function consolidates duplicate SSH detection logic
func InstallsSSH(line string) bool {
	lowerLine := strings.ToLower(line)
	sshPatterns := []string{
		"openssh-server",
		"ssh-server",
		"sshd",
		"install ssh",
		"apt-get install.*ssh",
		"yum install.*ssh",
		"apk add.*ssh",
	}

	for _, pattern := range sshPatterns {
		if matched, _ := regexp.MatchString(pattern, lowerLine); matched {
			return true
		}
	}
	return false
}

// InstallsSudo checks if a line installs sudo
// This function consolidates duplicate sudo detection logic
func InstallsSudo(line string) bool {
	lowerLine := strings.ToLower(line)
	return strings.Contains(lowerLine, "sudo") &&
		(strings.Contains(lowerLine, "install") ||
			strings.Contains(lowerLine, "apt-get") ||
			strings.Contains(lowerLine, "yum") ||
			strings.Contains(lowerLine, "apk add"))
}

// IsRootUser checks if a user specification refers to root user
// This function consolidates duplicate root user detection logic
func IsRootUser(user string) bool {
	return user == "root" || user == "0" || user == ""
}

// ValidateKubernetesName validates a Kubernetes resource name
// This function consolidates duplicate name validation logic
func ValidateKubernetesName(name string) bool {
	if name == "" || len(name) > 253 {
		return false
	}
	return KubernetesNamePattern.MatchString(name)
}

// ValidateImageReference validates a Docker image reference format
// This function consolidates duplicate image reference validation logic
func ValidateImageReference(imageRef string) bool {
	if imageRef == "" {
		return false
	}
	return ImageReferencePattern.MatchString(imageRef)
}

// ValidatePort validates a port specification
// This function consolidates duplicate port validation logic
func ValidatePort(port string) bool {
	if port == "" {
		return false
	}
	return PortPattern.MatchString(port)
}

// HasLatestTag checks if an image reference uses latest tag or no tag
// This function consolidates duplicate latest tag detection logic
func HasLatestTag(imageName string) bool {
	return strings.HasSuffix(imageName, ":latest") || !strings.Contains(imageName, ":")
}

// SecurityCheckResult represents the result of a security check
type SecurityCheckResult struct {
	HasIssue   bool
	Message    string
	Suggestion string
	Code       string
	LineNumber int
}

// PerformDockerfileSecurityChecks performs comprehensive security checks on a Dockerfile line
// This function consolidates duplicate security validation logic
func PerformDockerfileSecurityChecks(line string, lineNum int) []SecurityCheckResult {
	var results []SecurityCheckResult

	// Check for hardcoded secrets
	if ContainsHardcodedSecrets(line) {
		results = append(results, SecurityCheckResult{
			HasIssue:   true,
			Message:    "Potential hardcoded secret detected",
			Suggestion: "Use build arguments or environment variables for secrets",
			Code:       "security-hardcoded-secret",
			LineNumber: lineNum,
		})
	}

	// Check for insecure downloads
	if ContainsInsecureDownload(line) {
		results = append(results, SecurityCheckResult{
			HasIssue:   true,
			Message:    "Insecure download detected (HTTP instead of HTTPS)",
			Suggestion: "Use HTTPS URLs for downloads",
			Code:       "security-insecure-download",
			LineNumber: lineNum,
		})
	}

	// Check for SSH installation
	if InstallsSSH(line) {
		results = append(results, SecurityCheckResult{
			HasIssue:   true,
			Message:    "Installing SSH server in container is not recommended",
			Suggestion: "Use docker exec for debugging instead of SSH",
			Code:       "security-ssh-installation",
			LineNumber: lineNum,
		})
	}

	// Check for sudo installation
	if InstallsSudo(line) {
		results = append(results, SecurityCheckResult{
			HasIssue:   true,
			Message:    "Installing sudo in container is not recommended",
			Suggestion: "Run container with appropriate user privileges",
			Code:       "security-sudo-installation",
			LineNumber: lineNum,
		})
	}

	return results
}
