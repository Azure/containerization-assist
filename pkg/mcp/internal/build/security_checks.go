package build

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/rs/zerolog"
)

// DefaultSecurityChecks provides the default implementation of security checks
type DefaultSecurityChecks struct {
	logger zerolog.Logger
}

// NewDefaultSecurityChecks creates a new default security checks provider
func NewDefaultSecurityChecks(logger zerolog.Logger) *DefaultSecurityChecks {
	return &DefaultSecurityChecks{
		logger: logger.With().Str("component", "security_checks").Logger(),
	}
}

// CheckForRootUser checks if the container runs as root
func (c *DefaultSecurityChecks) CheckForRootUser(lines []string, result *ValidationResult) {
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

// CheckForSecrets checks for hardcoded secrets
func (c *DefaultSecurityChecks) CheckForSecrets(lines []string, result *ValidationResult, patterns []*regexp.Regexp) {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip comments
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check for secret patterns
		for _, pattern := range patterns {
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

// CheckForSensitivePorts checks for commonly attacked ports
func (c *DefaultSecurityChecks) CheckForSensitivePorts(lines []string, result *ValidationResult) {
	sensitivePorts := map[int]string{
		22:    "SSH",
		23:    "Telnet",
		3389:  "RDP",
		5900:  "VNC",
		5432:  "PostgreSQL",
		3306:  "MySQL",
		6379:  "Redis",
		27017: "MongoDB",
		1433:  "SQL Server",
		5984:  "CouchDB",
		9200:  "Elasticsearch",
		11211: "Memcached",
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

// CheckPackagePinning checks if packages are version-pinned
func (c *DefaultSecurityChecks) CheckPackagePinning(lines []string, result *ValidationResult) {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check apt-get install without version pinning
		if strings.Contains(trimmed, "apt-get install") &&
			!strings.Contains(trimmed, "apt-get update") {
			// Check if any package has version specified
			hasVersionPin := strings.Contains(trimmed, "=")
			if !hasVersionPin && !strings.Contains(trimmed, "-y") {
				result.Warnings = append(result.Warnings, ValidationWarning{
					Line:    i + 1,
					Column:  0,
					Message: "Package installation without version pinning. Pin package versions for reproducible builds (e.g., package=1.2.3)",
					Rule:    "unpinned_packages",
				})
			}
		}

		// Check yum/dnf install without version pinning
		if (strings.Contains(trimmed, "yum install") || strings.Contains(trimmed, "dnf install")) &&
			!strings.Contains(trimmed, "-") {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Line:    i + 1,
				Column:  0,
				Message: "Package installation without version pinning. Pin package versions for reproducible builds (e.g., package-1.2.3)",
				Rule:    "unpinned_packages",
			})
		}

		// Check pip install without version pinning
		if strings.Contains(trimmed, "pip install") &&
			!strings.Contains(trimmed, "requirements") {
			if !strings.Contains(trimmed, "==") && !strings.Contains(trimmed, ">=") && !strings.Contains(trimmed, "~=") {
				result.Warnings = append(result.Warnings, ValidationWarning{
					Line:    i + 1,
					Column:  0,
					Message: "Python package installation without version pinning. Pin package versions (e.g., package==1.2.3)",
					Rule:    "unpinned_packages",
				})
			}
		}

		// Check npm install without version pinning
		if strings.Contains(trimmed, "npm install") && !strings.Contains(trimmed, "--package-lock") {
			if !strings.Contains(trimmed, "@") {
				result.Warnings = append(result.Warnings, ValidationWarning{
					Line:    i + 1,
					Column:  0,
					Message: "Node.js package installation without version pinning. Pin package versions (e.g., package@1.2.3)",
					Rule:    "unpinned_packages",
				})
			}
		}
	}
}

// CheckForSUIDBindaries checks for SUID/SGID binary creation
func (c *DefaultSecurityChecks) CheckForSUIDBindaries(lines []string, result *ValidationResult) {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for chmod with SUID/SGID bits
		if strings.Contains(trimmed, "chmod") {
			if strings.Contains(trimmed, "+s") ||
				strings.Contains(trimmed, "4755") ||
				strings.Contains(trimmed, "4777") ||
				strings.Contains(trimmed, "2755") ||
				strings.Contains(trimmed, "6755") {
				result.Errors = append(result.Errors, ValidationError{
					Line:    i + 1,
					Column:  0,
					Message: "Setting SUID/SGID bits on files. Avoid using SUID/SGID binaries unless absolutely necessary",
					Rule:    "suid_binary",
				})
			}
		}

		// Check for common SUID binaries installation
		if strings.Contains(trimmed, "sudo") && strings.Contains(trimmed, "install") {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Line:    i + 1,
				Column:  0,
				Message: "Installing sudo in container. Consider if elevated privileges are really necessary",
				Rule:    "sudo_installation",
			})
		}
	}
}

// CheckBaseImageSecurity checks base image security
func (c *DefaultSecurityChecks) CheckBaseImageSecurity(lines []string, result *ValidationResult, trustedRegistries []string) {
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

				// Check for trusted registries
				if len(trustedRegistries) > 0 && !isFromTrustedRegistry(image, trustedRegistries) {
					result.Warnings = append(result.Warnings, ValidationWarning{
						Line:    i + 1,
						Column:  0,
						Message: "Base image from untrusted registry. Consider using base images from trusted registries",
						Rule:    "untrusted_base_image",
					})
				}

				// Check for vulnerable base images
				c.checkVulnerableBaseImages(image, i+1, result)

				// Check for overly permissive base images
				c.checkOverlyPermissiveBaseImages(image, i+1, result)
			}
		}
	}
}

// CheckForInsecureDownloads checks for insecure file downloads
func (c *DefaultSecurityChecks) CheckForInsecureDownloads(lines []string, result *ValidationResult) {
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
			result.Warnings = append(result.Warnings, ValidationWarning{
				Line:    i + 1,
				Column:  0,
				Message: "Using ADD for remote file download. Use RUN with curl/wget for better control and verification",
				Rule:    "add_remote_file",
			})
		}

		// Check for downloads without signature verification
		if (strings.Contains(trimmed, "curl") || strings.Contains(trimmed, "wget")) &&
			strings.Contains(trimmed, "https://") &&
			!strings.Contains(trimmed, "gpg") &&
			!strings.Contains(trimmed, "sha256") &&
			!strings.Contains(trimmed, "signature") {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Line:    i + 1,
				Column:  0,
				Message: "Downloading files without signature or checksum verification. Verify file integrity",
				Rule:    "unverified_download",
			})
		}
	}
}

// Helper methods for base image checking

// checkVulnerableBaseImages checks for known vulnerable base images
func (c *DefaultSecurityChecks) checkVulnerableBaseImages(image string, line int, result *ValidationResult) {
	vulnerableImages := map[string]string{
		"ubuntu:14.04": "Ubuntu 14.04 is end-of-life and no longer receives security updates",
		"centos:6":     "CentOS 6 is end-of-life and no longer receives security updates",
		"debian:7":     "Debian 7 (Wheezy) is end-of-life and no longer receives security updates",
		"alpine:3.3":   "Alpine 3.3 is end-of-life and no longer receives security updates",
	}

	for vulnImage, message := range vulnerableImages {
		if strings.Contains(image, vulnImage) {
			result.Errors = append(result.Errors, ValidationError{
				Line:    line,
				Column:  0,
				Message: fmt.Sprintf("Using vulnerable base image: %s", message),
				Rule:    "vulnerable_base_image",
			})
		}
	}
}

// checkOverlyPermissiveBaseImages checks for base images that are overly permissive
func (c *DefaultSecurityChecks) checkOverlyPermissiveBaseImages(image string, line int, result *ValidationResult) {
	permissiveImages := map[string]string{
		"ubuntu":      "Consider using ubuntu:20.04-slim or ubuntu:22.04-slim for smaller attack surface",
		"debian":      "Consider using debian:11-slim or debian:12-slim for smaller attack surface",
		"centos":      "Consider using a minimal CentOS variant or switch to Rocky Linux/AlmaLinux",
		"fedora":      "Consider using a minimal Fedora variant",
		"amazonlinux": "Consider using amazonlinux:2-minimal",
	}

	for permissiveImage, suggestion := range permissiveImages {
		if strings.HasPrefix(image, permissiveImage+":") && !strings.Contains(image, "slim") && !strings.Contains(image, "minimal") {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Line:    line,
				Column:  0,
				Message: fmt.Sprintf("Using full base image. %s", suggestion),
				Rule:    "overly_permissive_base_image",
			})
		}
	}
}

// Additional security check methods

// CheckForShellInjection checks for potential shell injection vulnerabilities
func (c *DefaultSecurityChecks) CheckForShellInjection(lines []string, result *ValidationResult) {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		if strings.HasPrefix(upper, "RUN") {
			// Check for shell injection patterns
			if strings.Contains(trimmed, "$") && (strings.Contains(trimmed, ";") || strings.Contains(trimmed, "&&") || strings.Contains(trimmed, "||")) {
				result.Warnings = append(result.Warnings, ValidationWarning{
					Line:    i + 1,
					Column:  0,
					Message: "Potential shell injection vulnerability. Be careful with variable substitution in shell commands",
					Rule:    "shell_injection_risk",
				})
			}
		}
	}
}

// CheckForTempFileHandling checks for insecure temporary file handling
func (c *DefaultSecurityChecks) CheckForTempFileHandling(lines []string, result *ValidationResult) {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for /tmp usage without proper cleanup
		if strings.Contains(trimmed, "/tmp/") && !strings.Contains(trimmed, "rm") {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Line:    i + 1,
				Column:  0,
				Message: "Using /tmp directory without cleanup. Ensure temporary files are cleaned up to reduce attack surface",
				Rule:    "temp_file_cleanup",
			})
		}

		// Check for world-writable directories
		if strings.Contains(trimmed, "chmod") && (strings.Contains(trimmed, "777") || strings.Contains(trimmed, "o+w")) {
			result.Errors = append(result.Errors, ValidationError{
				Line:    i + 1,
				Column:  0,
				Message: "Setting world-writable permissions. This creates security risks",
				Rule:    "world_writable",
			})
		}
	}
}

// CheckForCopySecrets checks for accidentally copying secret files
func (c *DefaultSecurityChecks) CheckForCopySecrets(lines []string, result *ValidationResult) {
	secretFiles := []string{
		".env",
		".env.local",
		".env.production",
		"id_rsa",
		"id_dsa",
		"id_ecdsa",
		"id_ed25519",
		".aws/credentials",
		".ssh/",
		"*.pem",
		"*.key",
		"*.p12",
		"*.pfx",
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		if strings.HasPrefix(upper, "COPY") || strings.HasPrefix(upper, "ADD") {
			for _, secretFile := range secretFiles {
				if strings.Contains(trimmed, secretFile) {
					result.Errors = append(result.Errors, ValidationError{
						Line:    i + 1,
						Column:  0,
						Message: fmt.Sprintf("Potentially copying secret file '%s'. Avoid copying sensitive files into container images", secretFile),
						Rule:    "copy_secret_file",
					})
				}
			}
		}
	}
}
