package build

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
)

// SecurityValidator handles Dockerfile security validation
// Implements DockerfileValidator interface
type SecurityValidator struct {
	logger            zerolog.Logger
	secretPatterns    []*regexp.Regexp
	trustedRegistries []string
}

// NewSecurityValidator creates a new security validator
func NewSecurityValidator(logger zerolog.Logger, trustedRegistries []string) *SecurityValidator {
	return &SecurityValidator{
		logger:            logger.With().Str("component", "security_validator").Logger(),
		trustedRegistries: trustedRegistries,
		secretPatterns:    compileSecretPatterns(),
	}
}

// Validate performs security validation on Dockerfile
func (v *SecurityValidator) Validate(content string, options ValidationOptions) (*ValidationResult, error) {
	if !options.CheckSecurity {
		v.logger.Debug().Msg("Security validation disabled")
		return &ValidationResult{Valid: true}, nil
	}
	v.logger.Info().Msg("Starting Dockerfile security validation")
	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]ValidationError, 0),
		Warnings: make([]ValidationWarning, 0),
		Info:     make([]string, 0),
	}
	lines := strings.Split(content, "\n")
	// Perform various security checks
	v.checkForRootUser(lines, result)
	v.checkForSecrets(lines, result)
	v.checkForSensitivePorts(lines, result)
	v.checkPackagePinning(lines, result)
	v.checkForSUIDBindaries(lines, result)
	v.checkBaseImageSecurity(lines, result)
	v.checkForInsecureDownloads(lines, result)
	// Update validation state
	if len(result.Errors) > 0 {
		result.Valid = false
	}
	return result, nil
}

// checkForRootUser checks if the container runs as root
func (v *SecurityValidator) checkForRootUser(lines []string, result *ValidationResult) {
	hasUser := false
	lastUserIsRoot := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)
		if strings.HasPrefix(upper, "USER") {
			hasUser = true
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				user := parts[1]
				if user == "root" || user == "0" {
					lastUserIsRoot = true
					result.Errors = append(result.Errors, ValidationError{
						Line:    i + 1,
						Column:  0,
						Message: "Container explicitly set to run as root user. Use a non-root user for better security",
						Rule:    "root_user",
					})
				} else {
					lastUserIsRoot = false
				}
			}
		}
	}
	if !hasUser || lastUserIsRoot {
		result.Errors = append(result.Errors, ValidationError{
			Line:    0,
			Column:  0,
			Message: "Container runs as root user by default. Add 'USER <non-root-user>' instruction to run as non-root",
			Rule:    "root_user",
		})
	}
}

// checkForSecrets checks for hardcoded secrets
func (v *SecurityValidator) checkForSecrets(lines []string, result *ValidationResult) {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip comments
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Check for secret patterns
		for _, pattern := range v.secretPatterns {
			if pattern.MatchString(line) {
				result.Errors = append(result.Errors, ValidationError{
					Line:    i + 1,
					Column:  0,
					Message: "Possible secret or sensitive data detected. Use build arguments or environment variables at runtime instead of hardcoding secrets",
					Rule:    "exposed_secret",
				})
				break
			}
		}
		// Check for common secret keywords
		upper := strings.ToUpper(line)
		if strings.Contains(upper, "PASSWORD=") ||
			strings.Contains(upper, "API_KEY=") ||
			strings.Contains(upper, "SECRET=") ||
			strings.Contains(upper, "TOKEN=") {
			result.Errors = append(result.Errors, ValidationError{
				Line:    i + 1,
				Column:  0,
				Message: "Sensitive environment variable detected. Use secrets management solution instead of hardcoding",
				Rule:    "exposed_secret",
			})
		}
	}
}

// checkForSensitivePorts checks for commonly attacked ports
func (v *SecurityValidator) checkForSensitivePorts(lines []string, result *ValidationResult) {
	sensitivePorts := map[int]string{
		22:    "SSH",
		23:    "Telnet",
		3389:  "RDP",
		5900:  "VNC",
		5432:  "PostgreSQL",
		3306:  "MySQL",
		6379:  "Redis",
		27017: "MongoDB",
	}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)
		if strings.HasPrefix(upper, "EXPOSE") {
			ports := extractPorts(trimmed)
			for _, port := range ports {
				if service, exists := sensitivePorts[port]; exists {
					result.Errors = append(result.Errors, ValidationError{
						Line:    i + 1,
						Column:  0,
						Message: fmt.Sprintf("Exposed sensitive port %d (%s). Ensure this port exposure is necessary and properly secured", port, service),
						Rule:    "sensitive_port",
					})
				}
			}
		}
	}
}

// checkPackagePinning checks if packages are version-pinned
func (v *SecurityValidator) checkPackagePinning(lines []string, result *ValidationResult) {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Check apt-get install without version pinning
		if strings.Contains(trimmed, "apt-get install") &&
			!strings.Contains(trimmed, "apt-get update") {
			// Check if any package has version specified
			hasVersionPin := false
			if strings.Contains(trimmed, "=") {
				// Simple check for version pinning
				hasVersionPin = true
			}
			if !hasVersionPin && !strings.Contains(trimmed, "-y") {
				result.Errors = append(result.Errors, ValidationError{
					Line:    i + 1,
					Column:  0,
					Message: "Package installation without version pinning. Pin package versions for reproducible builds (e.g., package=1.2.3)",
					Rule:    "unpinned_packages",
				})
			}
		}
		// Check pip install without version pinning
		if strings.Contains(trimmed, "pip install") &&
			!strings.Contains(trimmed, "requirements") {
			if !strings.Contains(trimmed, "==") && !strings.Contains(trimmed, ">=") {
				result.Errors = append(result.Errors, ValidationError{
					Line:    i + 1,
					Column:  0,
					Message: "Python package installation without version pinning. Pin package versions (e.g., package==1.2.3)",
					Rule:    "unpinned_packages",
				})
			}
		}
	}
}

// checkForSUIDBindaries checks for SUID/SGID binary creation
func (v *SecurityValidator) checkForSUIDBindaries(lines []string, result *ValidationResult) {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Check for chmod with SUID/SGID bits
		if strings.Contains(trimmed, "chmod") {
			if strings.Contains(trimmed, "+s") ||
				strings.Contains(trimmed, "4755") ||
				strings.Contains(trimmed, "4777") ||
				strings.Contains(trimmed, "2755") {
				result.Errors = append(result.Errors, ValidationError{
					Line:    i + 1,
					Column:  0,
					Message: "Setting SUID/SGID bits on files. Avoid using SUID/SGID binaries unless absolutely necessary",
					Rule:    "suid_binary",
				})
			}
		}
	}
}

// checkBaseImageSecurity checks base image security
func (v *SecurityValidator) checkBaseImageSecurity(lines []string, result *ValidationResult) {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)
		if strings.HasPrefix(upper, "FROM") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				image := parts[1]
				// Check for latest tag
				if strings.Contains(image, ":latest") || !strings.Contains(image, ":") {
					result.Errors = append(result.Errors, ValidationError{
						Line:    i + 1,
						Column:  0,
						Message: "Using 'latest' tag or untagged base image. Use specific version tags for base images",
						Rule:    "unpinned_base_image",
					})
				}
				// Check trusted registries
				if len(v.trustedRegistries) > 0 && !v.isFromTrustedRegistry(image) {
					result.Errors = append(result.Errors, ValidationError{
						Line:    i + 1,
						Column:  0,
						Message: "Base image from untrusted registry. Use base images from trusted registries only",
						Rule:    "untrusted_base_image",
					})
				}
			}
		}
	}
}

// checkForInsecureDownloads checks for insecure file downloads
func (v *SecurityValidator) checkForInsecureDownloads(lines []string, result *ValidationResult) {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Check for wget/curl with http://
		if (strings.Contains(trimmed, "wget") || strings.Contains(trimmed, "curl")) &&
			strings.Contains(trimmed, "http://") &&
			!strings.Contains(trimmed, "localhost") &&
			!strings.Contains(trimmed, "127.0.0.1") {
			result.Errors = append(result.Errors, ValidationError{
				Line:    i + 1,
				Column:  0,
				Message: "Downloading files over insecure HTTP. Use HTTPS for all external downloads",
				Rule:    "insecure_download",
			})
		}
		// Check for ADD with remote URL
		upper := strings.ToUpper(trimmed)
		if strings.HasPrefix(upper, "ADD") && strings.Contains(trimmed, "http") {
			result.Errors = append(result.Errors, ValidationError{
				Line:    i + 1,
				Column:  0,
				Message: "Using ADD for remote file download. Use RUN with curl/wget for better control and verification",
				Rule:    "add_remote_file",
			})
		}
	}
}

// Helper functions
func (v *SecurityValidator) containsSecret(line string) bool {
	for _, pattern := range v.secretPatterns {
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
}
func (v *SecurityValidator) isFromTrustedRegistry(image string) bool {
	for _, trusted := range v.trustedRegistries {
		if strings.HasPrefix(image, trusted) {
			return true
		}
	}
	// Check if it's an official image (no registry prefix)
	if !strings.Contains(image, "/") || strings.Count(image, "/") == 1 {
		return true
	}
	return false
}
func extractPorts(exposeLine string) []int {
	ports := make([]int, 0)
	parts := strings.Fields(exposeLine)
	for i := 1; i < len(parts); i++ {
		portStr := strings.TrimSuffix(parts[i], "/tcp")
		portStr = strings.TrimSuffix(portStr, "/udp")
		if port, err := strconv.Atoi(portStr); err == nil {
			ports = append(ports, port)
		}
	}
	return ports
}
func compileSecretPatterns() []*regexp.Regexp {
	patterns := []string{
		`(?i)(api[_-]?key|apikey)\s*[:=]\s*['"]\S+['"]`,
		`(?i)(secret|token)\s*[:=]\s*['"]\S+['"]`,
		`(?i)password\s*[:=]\s*['"]\S+['"]`,
		`(?i)bearer\s+[a-zA-Z0-9\-_]+`,
		`[a-zA-Z0-9]{32,}`, // Long random strings
		`-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----`,
	}
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		if re, err := regexp.Compile(pattern); err == nil {
			compiled = append(compiled, re)
		}
	}
	return compiled
}
