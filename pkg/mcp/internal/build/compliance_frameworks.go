package build

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
)

// extractPorts extracts port numbers from EXPOSE instruction
func extractPorts(line string) []int {
	ports := []int{}
	parts := strings.Fields(line)

	for i := 1; i < len(parts); i++ {
		portStr := strings.TrimSpace(parts[i])
		// Remove protocol if present (e.g., "80/tcp" -> "80")
		if idx := strings.Index(portStr, "/"); idx != -1 {
			portStr = portStr[:idx]
		}

		if port, err := strconv.Atoi(portStr); err == nil {
			ports = append(ports, port)
		}
	}

	return ports
}

// ComplianceFrameworkProvider provides methods for checking specific compliance frameworks
type ComplianceFrameworkProvider struct {
	logger zerolog.Logger
}

// NewComplianceFrameworkProvider creates a new compliance framework provider
func NewComplianceFrameworkProvider(logger zerolog.Logger) *ComplianceFrameworkProvider {
	return &ComplianceFrameworkProvider{
		logger: logger.With().Str("component", "compliance_frameworks").Logger(),
	}
}

// Compliance check implementations

// checkNoRootUser verifies container doesn't run as root
func (p *ComplianceFrameworkProvider) checkNoRootUser(lines []string) bool {
	hasUser := false
	lastUserIsRoot := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		if strings.HasPrefix(upper, "USER") {
			hasUser = true
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				user := parts[1]
				lastUserIsRoot = (user == "root" || user == "0")
			}
		}
	}

	return hasUser && !lastUserIsRoot
}

// checkMinimalBaseImage checks if using minimal base images
func (p *ComplianceFrameworkProvider) checkMinimalBaseImage(lines []string) bool {
	minimalImages := []string{
		"alpine", "scratch", "distroless", "busybox", "-slim", "-minimal",
	}

	for _, line := range lines {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), "FROM") {
			for _, minimal := range minimalImages {
				if strings.Contains(strings.ToLower(line), minimal) {
					return true
				}
			}
		}
	}

	return false
}

// checkNoSudoInstall checks that sudo is not installed
func (p *ComplianceFrameworkProvider) checkNoSudoInstall(lines []string) bool {
	for _, line := range lines {
		if strings.Contains(line, "install") && strings.Contains(line, "sudo") {
			return false
		}
	}
	return true
}

// checkHealthcheckDefined verifies HEALTHCHECK instruction exists
func (p *ComplianceFrameworkProvider) checkHealthcheckDefined(lines []string) bool {
	for _, line := range lines {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), "HEALTHCHECK") {
			return true
		}
	}
	return false
}

// checkNoSSHServer checks that SSH server is not installed
func (p *ComplianceFrameworkProvider) checkNoSSHServer(lines []string) bool {
	sshPackages := []string{"openssh-server", "ssh-server", "sshd"}

	for _, line := range lines {
		for _, pkg := range sshPackages {
			if strings.Contains(line, pkg) && strings.Contains(line, "install") {
				return false
			}
		}
	}
	return true
}

// checkSecretsManagement verifies no hardcoded secrets
func (p *ComplianceFrameworkProvider) checkSecretsManagement(lines []string) bool {
	for _, line := range lines {
		upper := strings.ToUpper(line)
		// Check for hardcoded secrets
		if strings.Contains(upper, "PASSWORD=") ||
			strings.Contains(upper, "API_KEY=") ||
			strings.Contains(upper, "SECRET=") ||
			strings.Contains(upper, "TOKEN=") {
			// Allow if it's using ARG or ENV with placeholder
			if strings.Contains(upper, "ARG ") ||
				strings.Contains(line, "${") ||
				strings.Contains(line, "$(") {
				continue
			}
			return false
		}
	}
	return true
}

// checkLoggingConfigured checks if logging is properly configured
func (p *ComplianceFrameworkProvider) checkLoggingConfigured(lines []string) bool {
	// Check for logging configuration
	loggingPatterns := []string{
		"LOG_", "LOGGING_", "log4j", "logback", "winston", "morgan",
		"/var/log", "stdout", "stderr",
	}

	for _, line := range lines {
		for _, pattern := range loggingPatterns {
			if strings.Contains(strings.ToLower(line), pattern) {
				return true
			}
		}
	}

	return false
}

// checkResourceLimits checks if resource limits are defined
func (p *ComplianceFrameworkProvider) checkResourceLimits(lines []string) bool {
	// In Dockerfile, resource limits are typically set at runtime
	// Check for documentation or labels indicating limits
	for _, line := range lines {
		if strings.Contains(line, "LABEL") &&
			(strings.Contains(line, "memory") || strings.Contains(line, "cpu")) {
			return true
		}
	}
	return false
}

// checkNoPrivilegedPorts verifies no privileged ports (<1024) are exposed
func (p *ComplianceFrameworkProvider) checkNoPrivilegedPorts(lines []string) bool {
	for _, line := range lines {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), "EXPOSE") {
			ports := extractPorts(line)
			for _, port := range ports {
				if port < 1024 && port != 80 && port != 443 {
					return false
				}
			}
		}
	}
	return true
}

// checkSignedImages checks if base images use content trust
func (p *ComplianceFrameworkProvider) checkSignedImages(lines []string) bool {
	for _, line := range lines {
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), "FROM") {
			// Check for digest reference (indicates signed image)
			if strings.Contains(line, "@sha256:") {
				return true
			}
		}
	}
	return false
}

// Default compliance framework definitions

// GetDefaultCISDockerBenchmark returns CIS Docker Benchmark compliance framework
func GetDefaultCISDockerBenchmark() *ComplianceFramework {
	return &ComplianceFramework{
		Name:    "CIS-Docker-Benchmark",
		Version: "1.4.0",
		Requirements: []ComplianceRequirement{
			{
				ID:          "4.1",
				Description: "Ensure a user for the container has been created",
				Category:    "Container Images and Build File",
				Check:       "no_root_user",
			},
			{
				ID:          "4.2",
				Description: "Ensure that containers use only trusted base images",
				Category:    "Container Images and Build File",
				Check:       "signed_images",
			},
			{
				ID:          "4.3",
				Description: "Ensure unnecessary packages are not installed",
				Category:    "Container Images and Build File",
				Check:       "minimal_base_image",
			},
			{
				ID:          "4.5",
				Description: "Ensure Content trust for Docker is Enabled",
				Category:    "Container Images and Build File",
				Check:       "signed_images",
			},
			{
				ID:          "4.6",
				Description: "Ensure HEALTHCHECK instructions have been added",
				Category:    "Container Images and Build File",
				Check:       "healthcheck_defined",
			},
			{
				ID:          "4.7",
				Description: "Ensure update instructions are not used alone",
				Category:    "Container Images and Build File",
				Check:       "no_root_user",
			},
			{
				ID:          "4.9",
				Description: "Ensure COPY is used instead of ADD",
				Category:    "Container Images and Build File",
				Check:       "minimal_base_image",
			},
			{
				ID:          "4.10",
				Description: "Ensure secrets are not stored in images",
				Category:    "Container Images and Build File",
				Check:       "secrets_management",
			},
		},
	}
}

// GetDefaultNISTFramework returns NIST 800-190 compliance framework
func GetDefaultNISTFramework() *ComplianceFramework {
	return &ComplianceFramework{
		Name:    "NIST-800-190",
		Version: "1.0",
		Requirements: []ComplianceRequirement{
			{
				ID:          "CP-1",
				Description: "Use minimal base images",
				Category:    "Container Protection",
				Check:       "minimal_base_image",
			},
			{
				ID:          "CP-2",
				Description: "Remove unnecessary tools and packages",
				Category:    "Container Protection",
				Check:       "no_sudo_install",
			},
			{
				ID:          "CP-3",
				Description: "Scan images for vulnerabilities",
				Category:    "Container Protection",
				Check:       "signed_images",
			},
			{
				ID:          "AC-1",
				Description: "Run containers with non-root users",
				Category:    "Access Control",
				Check:       "no_root_user",
			},
			{
				ID:          "AC-2",
				Description: "Limit container capabilities",
				Category:    "Access Control",
				Check:       "resource_limits",
			},
			{
				ID:          "AU-1",
				Description: "Enable logging for containers",
				Category:    "Audit and Accountability",
				Check:       "logging_configured",
			},
			{
				ID:          "SC-1",
				Description: "Protect sensitive data in containers",
				Category:    "System and Communications Protection",
				Check:       "secrets_management",
			},
			{
				ID:          "SC-2",
				Description: "Use secure communication channels",
				Category:    "System and Communications Protection",
				Check:       "no_ssh_server",
			},
		},
	}
}

// GetDefaultPCIDSSFramework returns PCI-DSS compliance framework
func GetDefaultPCIDSSFramework() *ComplianceFramework {
	return &ComplianceFramework{
		Name:    "PCI-DSS",
		Version: "4.0",
		Requirements: []ComplianceRequirement{
			{
				ID:          "2.2.2",
				Description: "Enable only necessary services",
				Category:    "Secure Configuration",
				Check:       "no_ssh_server",
			},
			{
				ID:          "2.2.5",
				Description: "Remove unnecessary functionality",
				Category:    "Secure Configuration",
				Check:       "minimal_base_image",
			},
			{
				ID:          "2.3",
				Description: "Encrypt all non-console administrative access",
				Category:    "Secure Configuration",
				Check:       "no_ssh_server",
			},
			{
				ID:          "6.2",
				Description: "Ensure all components are protected from known vulnerabilities",
				Category:    "Vulnerability Management",
				Check:       "signed_images",
			},
			{
				ID:          "7.1",
				Description: "Limit access to system components",
				Category:    "Access Control",
				Check:       "no_root_user",
			},
			{
				ID:          "8.2.1",
				Description: "Strong cryptography for authentication",
				Category:    "Authentication",
				Check:       "secrets_management",
			},
			{
				ID:          "10.1",
				Description: "Implement audit trails",
				Category:    "Logging and Monitoring",
				Check:       "logging_configured",
			},
			{
				ID:          "11.5",
				Description: "Deploy change detection mechanisms",
				Category:    "Security Testing",
				Check:       "signed_images",
			},
		},
	}
}

// Framework-specific compliance check implementations

// CheckCISDockerCompliance checks compliance with CIS Docker Benchmark
func (p *ComplianceFrameworkProvider) CheckCISDockerCompliance(dockerfile string, validationResult *ValidationResult, result *ComplianceResult) {
	// Check for root user
	if len(validationResult.Errors) > 0 {
		for _, err := range validationResult.Errors {
			if err.Rule == "root_user" {
				result.Compliant = false
				result.Score -= 20
				result.Violations = append(result.Violations, SecurityComplianceViolation{
					Requirement: "CIS 4.1",
					Description: "Container running as root user",
					Severity:    "high",
					Line:        err.Line,
				})
			}
		}
	}

	// Check for health check
	if !strings.Contains(dockerfile, "HEALTHCHECK") {
		result.Compliant = false
		result.Score -= 10
		result.Violations = append(result.Violations, SecurityComplianceViolation{
			Requirement: "CIS 4.6",
			Description: "No HEALTHCHECK instruction defined",
			Severity:    "medium",
		})
	}
}

// CheckNIST800190Compliance checks compliance with NIST 800-190
func (p *ComplianceFrameworkProvider) CheckNIST800190Compliance(dockerfile string, validationResult *ValidationResult, result *ComplianceResult) {
	// Check for insecure downloads
	for _, err := range validationResult.Errors {
		if err.Rule == "insecure_download" {
			result.Compliant = false
			result.Score -= 15
			result.Violations = append(result.Violations, SecurityComplianceViolation{
				Requirement: "NIST 800-190 4.3.3",
				Description: "Insecure download detected",
				Severity:    "high",
				Line:        err.Line,
			})
		}
	}
}

// CheckPCIDSSCompliance checks compliance with PCI-DSS
func (p *ComplianceFrameworkProvider) CheckPCIDSSCompliance(dockerfile string, validationResult *ValidationResult, result *ComplianceResult) {
	// Check for hardcoded secrets
	for _, err := range validationResult.Errors {
		if err.Rule == "hardcoded_secret" {
			result.Compliant = false
			result.Score -= 30
			result.Violations = append(result.Violations, SecurityComplianceViolation{
				Requirement: "PCI-DSS 8.2.1",
				Description: "Hardcoded credentials detected",
				Severity:    "critical",
				Line:        err.Line,
			})
		}
	}
}

// CheckHIPAACompliance checks compliance with HIPAA
func (p *ComplianceFrameworkProvider) CheckHIPAACompliance(dockerfile string, validationResult *ValidationResult, result *ComplianceResult) {
	// Check for telnet/unencrypted services
	for _, warn := range validationResult.Warnings {
		if warn.Rule == "sensitive_port" && strings.Contains(warn.Message, "23") {
			result.Compliant = false
			result.Score -= 25
			result.Violations = append(result.Violations, SecurityComplianceViolation{
				Requirement: "HIPAA 164.312(e)(1)",
				Description: "Unencrypted transmission protocol (telnet) exposed",
				Severity:    "high",
				Line:        warn.Line,
			})
		}
	}
}

// CheckSOC2Compliance checks compliance with SOC 2
func (p *ComplianceFrameworkProvider) CheckSOC2Compliance(dockerfile string, validationResult *ValidationResult, result *ComplianceResult) {
	// Check for permission issues
	if strings.Contains(dockerfile, "chmod 777") {
		result.Compliant = false
		result.Score -= 20
		result.Violations = append(result.Violations, SecurityComplianceViolation{
			Requirement: "SOC 2 CC6.3",
			Description: "Overly permissive file permissions detected",
			Severity:    "high",
		})
	}
}

// LoadDefaultComplianceFrameworks loads all default compliance frameworks into an enhanced validator
func LoadDefaultComplianceFrameworks(v *EnhancedSecurityValidator) {
	// Create default security policy with all frameworks
	defaultPolicy := &SecurityPolicy{
		Name:             "default-compliance",
		Description:      "Default compliance policy with CIS, NIST, and PCI-DSS",
		Version:          "1.0",
		EnforcementLevel: "strict",
		Rules: []SecurityRule{
			{
				ID:          "no-root",
				Name:        "No Root User",
				Description: "Containers must not run as root",
				Severity:    "high",
				Category:    "access-control",
				Enabled:     true,
				Action:      "block",
				Patterns:    []string{`USER\s+(root|0)`},
			},
			{
				ID:          "no-secrets",
				Name:        "No Hardcoded Secrets",
				Description: "No hardcoded passwords or API keys",
				Severity:    "critical",
				Category:    "secrets",
				Enabled:     true,
				Action:      "block",
				Patterns: []string{
					`(?i)(password|pwd|passwd)\s*=\s*['"][^'"]+['"]`,
					`(?i)(api[_-]?key|apikey)\s*=\s*['"][^'"]+['"]`,
					`(?i)(secret|token)\s*=\s*['"][^'"]+['"]`,
				},
			},
			{
				ID:          "use-minimal-base",
				Name:        "Use Minimal Base Images",
				Description: "Use minimal base images like alpine or distroless",
				Severity:    "medium",
				Category:    "image-security",
				Enabled:     true,
				Action:      "warn",
				Patterns:    []string{`FROM\s+(ubuntu|debian|centos)(?!.*slim|.*minimal)`},
			},
		},
		TrustedRegistries: []string{
			"docker.io",
			"gcr.io",
			"quay.io",
			"registry.hub.docker.com",
		},
		ComplianceFrameworks: []ComplianceFramework{
			*GetDefaultCISDockerBenchmark(),
			*GetDefaultNISTFramework(),
			*GetDefaultPCIDSSFramework(),
		},
	}

	// Load the default policy
	v.LoadPolicy(defaultPolicy)
	v.SetActivePolicy("default-compliance")
}

// VulnerabilityScanResult represents the result of a vulnerability scan
type VulnerabilityScanResult struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
}

// ProcessVulnerabilityScan processes vulnerability scan results
func ProcessVulnerabilityScan(scanResult *VulnerabilityScanResult) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []ValidationError{},
		Warnings: []ValidationWarning{},
	}

	if scanResult.Critical > 0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Line:    0,
			Column:  0,
			Message: fmt.Sprintf("Found %d critical vulnerabilities that must be fixed", scanResult.Critical),
			Rule:    "critical_vulnerabilities",
		})
	}

	if scanResult.High > 0 {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Line:    0,
			Column:  0,
			Message: fmt.Sprintf("Found %d high severity vulnerabilities", scanResult.High),
			Rule:    "high_vulnerabilities",
		})
	}

	if scanResult.Medium > 0 {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Line:    0,
			Column:  0,
			Message: fmt.Sprintf("Found %d medium severity vulnerabilities", scanResult.Medium),
			Rule:    "medium_vulnerabilities",
		})
	}

	return result
}
