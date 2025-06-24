package dockerfile

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
)

// SecurityValidator handles Dockerfile security validation
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
		return &ValidationResult{IsValid: true}, nil
	}

	v.logger.Info().Msg("Starting Dockerfile security validation")

	result := &ValidationResult{
		IsValid:        true,
		SecurityIssues: make([]SecurityIssue, 0),
		Context:        make(map[string]interface{}),
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
	if len(result.SecurityIssues) > 0 {
		result.IsValid = false
		result.TotalIssues = len(result.SecurityIssues)
		for _, issue := range result.SecurityIssues {
			if issue.Severity == "critical" || issue.Severity == "high" {
				result.CriticalIssues++
			}
		}
	}

	return result, nil
}

// Analyze provides security-specific analysis
func (v *SecurityValidator) Analyze(lines []string, context ValidationContext) interface{} {
	return v.performSecurityAnalysis(lines)
}

// performSecurityAnalysis performs comprehensive security analysis
func (v *SecurityValidator) performSecurityAnalysis(lines []string) SecurityAnalysis {
	analysis := SecurityAnalysis{
		ExposedPorts:    make([]int, 0),
		Recommendations: make([]string, 0),
		RunsAsRoot:      true, // Assume root until proven otherwise
		UsesPackagePin:  true, // Assume true until proven otherwise
		SecurityScore:   100,
	}

	hasUser := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		// Check for USER instruction
		if strings.HasPrefix(upper, "USER") && !strings.Contains(trimmed, "root") {
			hasUser = true
			analysis.RunsAsRoot = false
		}

		// Check for exposed ports
		if strings.HasPrefix(upper, "EXPOSE") {
			ports := extractPorts(trimmed)
			analysis.ExposedPorts = append(analysis.ExposedPorts, ports...)
		}

		// Check for secrets
		if v.containsSecret(trimmed) {
			analysis.HasSecrets = true
			analysis.SecurityScore -= 30
		}

		// Check for package pinning
		if strings.Contains(trimmed, "apt-get install") && !strings.Contains(trimmed, "=") {
			analysis.UsesPackagePin = false
			analysis.SecurityScore -= 10
		}
	}

	if !hasUser {
		analysis.Recommendations = append(analysis.Recommendations,
			"Add a non-root user for better security")
		analysis.SecurityScore -= 20
	}

	if !analysis.UsesPackagePin {
		analysis.Recommendations = append(analysis.Recommendations,
			"Pin package versions to ensure reproducible builds")
	}

	if len(analysis.ExposedPorts) > 5 {
		analysis.Recommendations = append(analysis.Recommendations,
			"Consider reducing the number of exposed ports")
		analysis.SecurityScore -= 5
	}

	// Ensure score doesn't go below 0
	if analysis.SecurityScore < 0 {
		analysis.SecurityScore = 0
	}

	return analysis
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
					result.SecurityIssues = append(result.SecurityIssues, SecurityIssue{
						Type:        "root_user",
						Line:        i + 1,
						Severity:    "high",
						Description: "Container explicitly set to run as root user",
						Remediation: "Use a non-root user for better security",
					})
				} else {
					lastUserIsRoot = false
				}
			}
		}
	}

	if !hasUser || lastUserIsRoot {
		result.SecurityIssues = append(result.SecurityIssues, SecurityIssue{
			Type:        "root_user",
			Line:        0,
			Severity:    "high",
			Description: "Container runs as root user by default",
			Remediation: "Add 'USER <non-root-user>' instruction to run as non-root",
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
				result.SecurityIssues = append(result.SecurityIssues, SecurityIssue{
					Type:        "exposed_secret",
					Line:        i + 1,
					Severity:    "critical",
					Description: "Possible secret or sensitive data detected",
					Remediation: "Use build arguments or environment variables at runtime instead of hardcoding secrets",
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
			result.SecurityIssues = append(result.SecurityIssues, SecurityIssue{
				Type:        "exposed_secret",
				Line:        i + 1,
				Severity:    "critical",
				Description: "Sensitive environment variable detected",
				Remediation: "Use secrets management solution instead of hardcoding",
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
					result.SecurityIssues = append(result.SecurityIssues, SecurityIssue{
						Type:        "sensitive_port",
						Line:        i + 1,
						Severity:    "medium",
						Description: fmt.Sprintf("Exposed sensitive port %d (%s)", port, service),
						Remediation: "Ensure this port exposure is necessary and properly secured",
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
				result.SecurityIssues = append(result.SecurityIssues, SecurityIssue{
					Type:        "unpinned_packages",
					Line:        i + 1,
					Severity:    "medium",
					Description: "Package installation without version pinning",
					Remediation: "Pin package versions for reproducible builds (e.g., package=1.2.3)",
				})
			}
		}

		// Check pip install without version pinning
		if strings.Contains(trimmed, "pip install") &&
			!strings.Contains(trimmed, "requirements") {
			if !strings.Contains(trimmed, "==") && !strings.Contains(trimmed, ">=") {
				result.SecurityIssues = append(result.SecurityIssues, SecurityIssue{
					Type:        "unpinned_packages",
					Line:        i + 1,
					Severity:    "medium",
					Description: "Python package installation without version pinning",
					Remediation: "Pin package versions (e.g., package==1.2.3)",
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
				result.SecurityIssues = append(result.SecurityIssues, SecurityIssue{
					Type:        "suid_binary",
					Line:        i + 1,
					Severity:    "high",
					Description: "Setting SUID/SGID bits on files",
					Remediation: "Avoid using SUID/SGID binaries unless absolutely necessary",
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
					result.SecurityIssues = append(result.SecurityIssues, SecurityIssue{
						Type:        "unpinned_base_image",
						Line:        i + 1,
						Severity:    "medium",
						Description: "Using 'latest' tag or untagged base image",
						Remediation: "Use specific version tags for base images",
					})
				}

				// Check trusted registries
				if len(v.trustedRegistries) > 0 && !v.isFromTrustedRegistry(image) {
					result.SecurityIssues = append(result.SecurityIssues, SecurityIssue{
						Type:        "untrusted_base_image",
						Line:        i + 1,
						Severity:    "medium",
						Description: "Base image from untrusted registry",
						Remediation: "Use base images from trusted registries only",
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
			result.SecurityIssues = append(result.SecurityIssues, SecurityIssue{
				Type:        "insecure_download",
				Line:        i + 1,
				Severity:    "high",
				Description: "Downloading files over insecure HTTP",
				Remediation: "Use HTTPS for all external downloads",
			})
		}

		// Check for ADD with remote URL
		upper := strings.ToUpper(trimmed)
		if strings.HasPrefix(upper, "ADD") && strings.Contains(trimmed, "http") {
			result.SecurityIssues = append(result.SecurityIssues, SecurityIssue{
				Type:        "add_remote_file",
				Line:        i + 1,
				Severity:    "medium",
				Description: "Using ADD for remote file download",
				Remediation: "Use RUN with curl/wget for better control and verification",
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
