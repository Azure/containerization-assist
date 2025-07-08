package build

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/core/config"
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
func (c *DefaultSecurityChecks) CheckForRootUser(lines []string, result *BuildValidationResult) {
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
					error := core.NewError(
						"ROOT_USER",
						"Container explicitly set to run as root user. Use a non-root user for better security",
						core.ErrTypeSecurity,
						core.SeverityHigh,
					).WithLine(i + 1).WithRule("root_user")
					result.AddError(&error.Error)
				} else {
					lastUserIsRoot = false
				}
			}
		}
	}

	if !hasUser || lastUserIsRoot {
		error := core.NewError(
			"DEFAULT_ROOT_USER",
			"Container runs as root user by default. Add 'USER <non-root-user>' instruction to run as non-root",
			core.ErrTypeSecurity,
			core.SeverityHigh,
		).WithRule("root_user")
		result.AddError(&error.Error)
	}
}

// CheckForSecrets checks for hardcoded secrets
func (c *DefaultSecurityChecks) CheckForSecrets(lines []string, result *BuildValidationResult, patterns []*regexp.Regexp) {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip comments
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check for secret patterns
		for _, pattern := range patterns {
			if pattern.MatchString(line) {
				error := core.NewError(
					"EXPOSED_SECRET",
					"Possible secret or sensitive data detected. Use build arguments or environment variables at runtime instead of hardcoding secrets",
					core.ErrTypeSecurity,
					core.SeverityCritical,
				).WithLine(i + 1).WithRule("exposed_secret")
				result.AddError(&error.Error)
				break
			}
		}

		// Check for common secret keywords
		upper := strings.ToUpper(line)
		if strings.Contains(upper, "PASSWORD=") ||
			strings.Contains(upper, "API_KEY=") ||
			strings.Contains(upper, "SECRET=") ||
			strings.Contains(upper, "TOKEN=") {
			error := core.NewError(
				"HARDCODED_SECRET",
				"Sensitive environment variable detected. Use secrets management solution instead of hardcoding",
				core.ErrTypeSecurity,
				core.SeverityCritical,
			).WithLine(i + 1).WithRule("exposed_secret")
			result.AddError(&error.Error)
		}
	}
}

// CheckForSensitivePorts checks for commonly attacked ports
func (c *DefaultSecurityChecks) CheckForSensitivePorts(lines []string, result *BuildValidationResult) {
	sensitivePorts := map[int]string{
		22:                  "SSH",
		23:                  "Telnet",
		3389:                "RDP",
		5900:                "VNC",
		config.PostgresPort: "PostgreSQL",
		config.MySQLPort:    "MySQL",
		config.RedisPort:    "Redis",
		config.MongoDBPort:  "MongoDB",
		1433:                "SQL Server",
		5984:                "CouchDB",
		9200:                "Elasticsearch",
		11211:               "Memcached",
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		if strings.HasPrefix(upper, "EXPOSE") {
			ports := extractPorts(trimmed)
			for _, port := range ports {
				if service, exists := sensitivePorts[port]; exists {
					error := core.NewError(
						"SENSITIVE_PORT",
						fmt.Sprintf("Exposed sensitive port %d (%s). Ensure this port exposure is necessary and properly secured", port, service),
						core.ErrTypeSecurity,
						core.SeverityHigh,
					).WithLine(i + 1).WithRule("sensitive_port")
					result.AddError(&error.Error)
				}
			}
		}
	}
}

// CheckPackagePinning checks if packages are version-pinned
func (c *DefaultSecurityChecks) CheckPackagePinning(lines []string, result *BuildValidationResult) {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check apt-get install without version pinning
		if strings.Contains(trimmed, "apt-get install") &&
			!strings.Contains(trimmed, "apt-get update") {
			// Check if any package has version specified
			hasVersionPin := strings.Contains(trimmed, "=")
			if !hasVersionPin && !strings.Contains(trimmed, "-y") {
				warning := core.NewWarning(
					"UNPINNED_PACKAGES",
					"Package installation without version pinning. Pin package versions for reproducible builds (e.g., package=1.2.3)",
				)
				warning.Error.WithLine(i + 1).WithRule("unpinned_packages")
				wrapAddWarning(result, warning)
			}
		}

		// Check yum/dnf install without version pinning
		if (strings.Contains(trimmed, "yum install") || strings.Contains(trimmed, "dnf install")) &&
			!strings.Contains(trimmed, "-") {
			warning := core.NewWarning(
				"UNPINNED_PACKAGES",
				"Package installation without version pinning. Pin package versions for reproducible builds (e.g., package-1.2.3)",
			)
			warning.Error.WithLine(i + 1).WithRule("unpinned_packages")
			wrapAddWarning(result, warning)
		}

		// Check pip install without version pinning
		if strings.Contains(trimmed, "pip install") &&
			!strings.Contains(trimmed, "requirements") {
			if !strings.Contains(trimmed, "==") && !strings.Contains(trimmed, ">=") && !strings.Contains(trimmed, "~=") {
				warning := core.NewWarning(
					"UNPINNED_PACKAGES",
					"Python package installation without version pinning. Pin package versions (e.g., package==1.2.3)",
				)
				warning.Error.WithLine(i + 1).WithRule("unpinned_packages")
				wrapAddWarning(result, warning)
			}
		}

		// Check npm install without version pinning
		if strings.Contains(trimmed, "npm install") && !strings.Contains(trimmed, "--package-lock") {
			if !strings.Contains(trimmed, "@") {
				warning := core.NewWarning(
					"UNPINNED_PACKAGES",
					"Node.js package installation without version pinning. Pin package versions (e.g., package@1.2.3)",
				)
				warning.Error.WithLine(i + 1).WithRule("unpinned_packages")
				wrapAddWarning(result, warning)
			}
		}
	}
}

// CheckForSUIDBindaries checks for SUID/SGID binary creation
func (c *DefaultSecurityChecks) CheckForSUIDBindaries(lines []string, result *BuildValidationResult) {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for chmod with SUID/SGID bits
		if strings.Contains(trimmed, "chmod") {
			if strings.Contains(trimmed, "+s") ||
				strings.Contains(trimmed, "4755") ||
				strings.Contains(trimmed, "4777") ||
				strings.Contains(trimmed, "2755") ||
				strings.Contains(trimmed, "6755") {
				error := core.NewError(
					"SUID_BINARY",
					"Setting SUID/SGID bits on files. Avoid using SUID/SGID binaries unless absolutely necessary",
					core.ErrTypeSecurity,
					core.SeverityHigh,
				).WithLine(i + 1).WithRule("suid_binary")
				result.AddError(&error.Error)
			}
		}

		// Check for common SUID binaries installation
		if strings.Contains(trimmed, "sudo") && strings.Contains(trimmed, "install") {
			warning := core.NewWarning(
				"SUDO_INSTALLATION",
				"Installing sudo in container. Consider if elevated privileges are really necessary",
			)
			warning.Error.WithLine(i + 1).WithRule("sudo_installation")
			wrapAddWarning(result, warning)
		}
	}
}

// CheckBaseImageSecurity checks base image security
func (c *DefaultSecurityChecks) CheckBaseImageSecurity(lines []string, result *BuildValidationResult, trustedRegistries []string) {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		if strings.HasPrefix(upper, "FROM") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				image := parts[1]

				// Check for latest tag
				if strings.Contains(image, ":latest") || !strings.Contains(image, ":") {
					error := core.NewError(
						"UNPINNED_BASE_IMAGE",
						"Using 'latest' tag or untagged base image. Use specific version tags for base images",
						core.ErrTypeBuild,
						core.SeverityHigh,
					).WithLine(i + 1).WithRule("unpinned_base_image")
					result.AddError(&error.Error)
				}

				// Check for trusted registries
				if len(trustedRegistries) > 0 && !isFromTrustedRegistry(image, trustedRegistries) {
					warning := core.NewWarning(
						"UNTRUSTED_BASE_IMAGE",
						"Base image from untrusted registry. Consider using base images from trusted registries",
					)
					warning.Error.WithLine(i + 1).WithRule("untrusted_base_image")
					wrapAddWarning(result, warning)
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
func (c *DefaultSecurityChecks) CheckForInsecureDownloads(lines []string, result *BuildValidationResult) {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for wget/curl with http://
		if (strings.Contains(trimmed, "wget") || strings.Contains(trimmed, "curl")) &&
			strings.Contains(trimmed, "http://") &&
			!strings.Contains(trimmed, "localhost") &&
			!strings.Contains(trimmed, "127.0.0.1") {
			error := core.NewError(
				"INSECURE_DOWNLOAD",
				"Downloading files over insecure HTTP. Use HTTPS for all external downloads",
				core.ErrTypeSecurity,
				core.SeverityHigh,
			).WithLine(i + 1).WithRule("insecure_download")
			result.AddError(&error.Error)
		}

		// Check for ADD with remote URL
		upper := strings.ToUpper(trimmed)
		if strings.HasPrefix(upper, "ADD") && strings.Contains(trimmed, "http") {
			warning := core.NewWarning(
				"ADD_REMOTE_FILE",
				"Using ADD for remote file download. Use RUN with curl/wget for better control and verification",
			)
			warning.Error.WithLine(i + 1).WithRule("add_remote_file")
			wrapAddWarning(result, warning)
		}

		// Check for downloads without signature verification
		if (strings.Contains(trimmed, "curl") || strings.Contains(trimmed, "wget")) &&
			strings.Contains(trimmed, "https://") &&
			!strings.Contains(trimmed, "gpg") &&
			!strings.Contains(trimmed, "sha256") &&
			!strings.Contains(trimmed, "signature") {
			warning := core.NewWarning(
				"UNVERIFIED_DOWNLOAD",
				"Downloading files without signature or checksum verification. Verify file integrity",
			)
			warning.Error.WithLine(i + 1).WithRule("unverified_download")
			wrapAddWarning(result, warning)
		}
	}
}

// Helper methods for base image checking

// checkVulnerableBaseImages checks for known vulnerable base images
func (c *DefaultSecurityChecks) checkVulnerableBaseImages(image string, line int, result *BuildValidationResult) {
	vulnerableImages := map[string]string{
		"ubuntu:14.04": "Ubuntu 14.04 is end-of-life and no longer receives security updates",
		"centos:6":     "CentOS 6 is end-of-life and no longer receives security updates",
		"debian:7":     "Debian 7 (Wheezy) is end-of-life and no longer receives security updates",
		"alpine:3.3":   "Alpine 3.3 is end-of-life and no longer receives security updates",
	}

	for vulnImage, message := range vulnerableImages {
		if strings.Contains(image, vulnImage) {
			error := core.NewError(
				"VULNERABLE_BASE_IMAGE",
				fmt.Sprintf("Using vulnerable base image: %s", message),
				core.ErrTypeSecurity,
				core.SeverityCritical,
			).WithLine(line).WithRule("vulnerable_base_image")
			result.AddError(&error.Error)
		}
	}
}

// checkOverlyPermissiveBaseImages checks for base images that are overly permissive
func (c *DefaultSecurityChecks) checkOverlyPermissiveBaseImages(image string, line int, result *BuildValidationResult) {
	permissiveImages := map[string]string{
		"ubuntu":      "Consider using ubuntu:20.04-slim or ubuntu:22.04-slim for smaller attack surface",
		"debian":      "Consider using debian:11-slim or debian:12-slim for smaller attack surface",
		"centos":      "Consider using a minimal CentOS variant or switch to Rocky Linux/AlmaLinux",
		"fedora":      "Consider using a minimal Fedora variant",
		"amazonlinux": "Consider using amazonlinux:2-minimal",
	}

	for permissiveImage, suggestion := range permissiveImages {
		if strings.HasPrefix(image, permissiveImage+":") && !strings.Contains(image, "slim") && !strings.Contains(image, "minimal") {
			warning := core.NewWarning(
				"OVERLY_PERMISSIVE_BASE_IMAGE",
				fmt.Sprintf("Using full base image. %s", suggestion),
			)
			warning.Error.WithLine(line).WithRule("overly_permissive_base_image")
			wrapAddWarning(result, warning)
		}
	}
}

// Helper function to check if image is from trusted registry
func isFromTrustedRegistry(image string, trustedRegistries []string) bool {
	for _, registry := range trustedRegistries {
		if strings.HasPrefix(image, registry+"/") || strings.HasPrefix(image, registry+":") {
			return true
		}
	}
	// Check default registries
	if !strings.Contains(image, "/") {
		// No registry specified, it's from Docker Hub
		for _, registry := range trustedRegistries {
			if registry == "docker.io" || registry == "hub.docker.com" {
				return true
			}
		}
	}
	return false
}

// Additional security check methods

// CheckForShellInjection checks for potential shell injection vulnerabilities
func (c *DefaultSecurityChecks) CheckForShellInjection(lines []string, result *BuildValidationResult) {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		if strings.HasPrefix(upper, "RUN") {
			// Check for shell injection patterns
			if strings.Contains(trimmed, "$") && (strings.Contains(trimmed, ";") || strings.Contains(trimmed, "&&") || strings.Contains(trimmed, "||")) {
				warning := core.NewWarning(
					"SHELL_INJECTION_RISK",
					"Potential shell injection vulnerability. Be careful with variable substitution in shell commands",
				)
				warning.Error.WithLine(i + 1).WithRule("shell_injection_risk")
				wrapAddWarning(result, warning)
			}
		}
	}
}

// CheckForTempFileHandling checks for insecure temporary file handling
func (c *DefaultSecurityChecks) CheckForTempFileHandling(lines []string, result *BuildValidationResult) {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for /tmp usage without proper cleanup
		if strings.Contains(trimmed, "/tmp/") && !strings.Contains(trimmed, "rm") {
			warning := core.NewWarning(
				"TEMP_FILE_CLEANUP",
				"Using /tmp directory without cleanup. Ensure temporary files are cleaned up to reduce attack surface",
			)
			warning.Error.WithLine(i + 1).WithRule("temp_file_cleanup")
			wrapAddWarning(result, warning)
		}

		// Check for world-writable directories
		if strings.Contains(trimmed, "chmod") && (strings.Contains(trimmed, "777") || strings.Contains(trimmed, "o+w")) {
			error := core.NewError(
				"WORLD_WRITABLE",
				"Setting world-writable permissions. This creates security risks",
				core.ErrTypeSecurity,
				core.SeverityHigh,
			).WithLine(i + 1).WithRule("world_writable")
			result.AddError(&error.Error)
		}
	}
}

// CheckForCopySecrets checks for accidentally copying secret files
func (c *DefaultSecurityChecks) CheckForCopySecrets(lines []string, result *BuildValidationResult) {
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
					error := core.NewError(
						"COPY_SECRET_FILE",
						fmt.Sprintf("Potentially copying secret file '%s'. Avoid copying sensitive files into container images", secretFile),
						core.ErrTypeSecurity,
						core.SeverityCritical,
					).WithLine(i + 1).WithRule("copy_secret_file")
					result.AddError(&error.Error)
				}
			}
		}
	}
}
