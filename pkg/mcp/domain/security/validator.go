package security

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
	"github.com/Azure/container-kit/pkg/common/validation-core/validators"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/rs/zerolog"
)

// SecurityValidator provides comprehensive security validation for sandboxed execution using unified validation
type SecurityValidator struct {
	logger            zerolog.Logger
	vulnDatabase      map[string]VulnerabilityInfo
	policyEngine      *SecurityPolicyEngine
	threatModel       *ThreatModel
	securityValidator core.Validator
	secretValidator   *validators.SecurityScanValidator
}

// UnifiedSecurityValidator provides a unified validation interface
type UnifiedSecurityValidator struct {
	impl *SecurityValidator
}

// NewUnifiedSecurityValidator creates a new unified security validator
func NewUnifiedSecurityValidator(logger zerolog.Logger) *UnifiedSecurityValidator {
	return &UnifiedSecurityValidator{
		impl: NewSecurityValidator(logger),
	}
}

type SecurityPolicyEngine struct {
	logger   zerolog.Logger
	policies map[string]SecurityPolicy
}

// NewSecurityValidator creates a new security validator with unified validation support
func NewSecurityValidator(logger zerolog.Logger) *SecurityValidator {
	return &SecurityValidator{
		logger:            logger.With().Str("component", "unified_security_validator").Logger(),
		vulnDatabase:      make(map[string]VulnerabilityInfo),
		policyEngine:      NewSecurityPolicyEngine(logger),
		threatModel:       initializeDefaultThreatModel(),
		securityValidator: validators.NewSecurityValidator(),
		secretValidator:   validators.NewSecurityScanValidator(),
	}
}

// ValidateSecurityUnified performs comprehensive security validation using unified framework
func (sv *SecurityValidator) ValidateSecurityUnified(ctx context.Context, operation string, params map[string]interface{}) (*core.SecurityResult, error) {
	sv.logger.Info().
		Str("operation", operation).
		Msg("Starting unified security validation")

	// Create security validation data
	securityData := map[string]interface{}{
		"operation": operation,
		"params":    params,
		"scan_type": "comprehensive",
		"timestamp": time.Now(),
	}

	// Use unified security validator
	options := core.NewValidationOptions().WithStrictMode(true)
	nonGenericResult := sv.securityValidator.Validate(ctx, securityData, options)

	// Convert to SecurityResult
	result := core.NewSecurityResult("unified_security_validator", "1.0.0")
	result.Valid = nonGenericResult.Valid
	result.Errors = nonGenericResult.Errors
	result.Warnings = nonGenericResult.Warnings
	result.Suggestions = nonGenericResult.Suggestions
	result.Duration = nonGenericResult.Duration
	result.Metadata = nonGenericResult.Metadata

	// Add security-specific validations
	threats := sv.assessThreats(operation, params)
	if len(threats) > 0 {
		for _, threat := range threats {
			result.AddError(core.NewSecurityError("SECURITY_THREAT_DETECTED",
				fmt.Sprintf("Security threat detected: %s - %s", threat.Category, threat.Description),
				"threat_assessment"))
		}
	}

	// Validate against security policy
	if err := sv.policyEngine.ValidateOperation(operation, params); err != nil {
		result.AddError(core.NewSecurityError("POLICY_VIOLATION",
			fmt.Sprintf("Security policy violation: %v", err),
			"policy_validation"))
	}

	// Add security-specific metadata
	result.Data = core.SecurityValidationData{
		ScanType:         "comprehensive",
		PolicyViolations: convertPolicyViolations(result.Errors),
		ComplianceChecks: []core.ComplianceCheck{
			{
				Standard: "internal",
				Control:  "threat_assessment",
				Status:   getComplianceStatus(len(threats) == 0),
			},
		},
	}

	sv.logger.Info().
		Bool("valid", result.Valid).
		Int("errors", len(result.Errors)).
		Int("warnings", len(result.Warnings)).
		Str("operation", operation).
		Msg("Unified security validation completed")

	return result, nil
}

// ValidateSecurityContext validates the security context for an operation (legacy compatibility)
func (sv *SecurityValidator) ValidateSecurityContext(ctx context.Context, operation string, params map[string]interface{}) error {
	sv.logger.Debug().
		Str("operation", operation).
		Interface("params", params).
		Msg("Validating security context")

	// Check for known threats
	if threats := sv.assessThreats(operation, params); len(threats) > 0 {
		for _, threat := range threats {
			if threat.Impact == "HIGH" && threat.Probability == "HIGH" {
				return errors.NewError().
					Code(errors.CodePermissionDenied).
					Type(errors.ErrTypeSecurity).
					Severity(errors.SeverityHigh).
					Messagef("High-risk security threat detected: %s", threat.Name).
					Context("threat_id", threat.ID).
					Context("operation", operation).
					Context("threat_category", threat.Category).
					Suggestion("Review operation parameters and apply security mitigations").
					WithLocation().
					Build()
			}
		}
	}

	// Validate against security policies
	if err := sv.policyEngine.ValidateOperation(operation, params); err != nil {
		return err
	}

	return nil
}

// ScanForSecrets performs comprehensive secret scanning
func (sv *SecurityValidator) ScanForSecrets(ctx context.Context, targetPath string) (*SecretScannerResult, error) {
	startTime := time.Now()

	result := &SecretScannerResult{
		Found:    false,
		Secrets:  make([]DetectedSecret, 0),
		Files:    make(map[string][]FileSecret),
		Metadata: make(map[string]interface{}),
		ScanSummary: SecretScanSummary{
			PatternMatches: make(map[string]int),
			FileTypes:      make(map[string]int),
			Metadata:       make(map[string]interface{}),
		},
	}

	// Define secret patterns
	secretPatterns := map[string]*regexp.Regexp{
		"aws_access_key":  regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		"aws_secret_key":  regexp.MustCompile(`[0-9a-zA-Z/+]{40}`),
		"github_token":    regexp.MustCompile(`ghp_[0-9a-zA-Z]{36}`),
		"slack_token":     regexp.MustCompile(`xox[baprs]-[0-9a-zA-Z-]+`),
		"jwt_token":       regexp.MustCompile(`eyJ[0-9a-zA-Z_-]*\.eyJ[0-9a-zA-Z_-]*\.[0-9a-zA-Z_-]*`),
		"private_key":     regexp.MustCompile(`-----BEGIN [A-Z ]+PRIVATE KEY-----`),
		"ssh_private_key": regexp.MustCompile(`-----BEGIN OPENSSH PRIVATE KEY-----`),
		"api_key":         regexp.MustCompile(`(?i)api[_-]?key['"\s]*[:=]['"\s]*[0-9a-zA-Z]{20,}`),
		"password":        regexp.MustCompile(`(?i)password['"\s]*[:=]['"\s]*[^\s'"]{8,}`),
		"database_url":    regexp.MustCompile(`(?i)(mysql|postgres|mongodb)://[^\s'"]+`),
		"docker_auth":     regexp.MustCompile(`"auth":\s*"[A-Za-z0-9+/=]+"`),
	}

	// Walk through files
	err := filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and binary files
		if info.IsDir() || sv.isBinaryFile(path) {
			return nil
		}

		// Track file types
		ext := filepath.Ext(path)
		result.ScanSummary.FileTypes[ext]++
		result.ScanSummary.FilesScanned++

		// Scan file content
		content, err := os.ReadFile(path)
		if err != nil {
			sv.logger.Warn().Err(err).Str("file", path).Msg("Failed to read file for secret scanning")
			return nil
		}

		fileSecrets := sv.scanFileContent(string(content), path, secretPatterns)
		if len(fileSecrets) > 0 {
			result.Files[path] = fileSecrets
			// Convert FileSecret to DetectedSecret
			for _, fs := range fileSecrets {
				ds := DetectedSecret{
					Type:        fs.Type,
					File:        path,
					Line:        fs.Line,
					Column:      fs.Column,
					Confidence:  fs.Confidence,
					Description: fs.Description,
					Severity:    sv.calculateSeverity(fs.Type),
				}
				result.Secrets = append(result.Secrets, ds)
			}
			result.Found = true
		}

		return nil
	})

	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeIOError).
			Type(errors.ErrTypeIO).
			Severity(errors.SeverityMedium).
			Message("Failed to scan directory for secrets").
			Context("target_path", targetPath).
			Cause(err).
			Suggestion("Check directory permissions and accessibility").
			WithLocation().
			Build()
	}

	// Calculate summary statistics
	result.ScanSummary.SecretsFound = len(result.Secrets)
	result.ScanSummary.ScanDuration = time.Since(startTime)

	for _, secret := range result.Secrets {
		switch secret.Severity {
		case "HIGH":
			result.ScanSummary.HighSeverity++
		case "MEDIUM":
			result.ScanSummary.MediumSeverity++
		case "LOW":
			result.ScanSummary.LowSeverity++
		}
	}

	return result, nil
}

// SanitizeErrorMessage removes potentially sensitive information from error messages
func (sv *SecurityValidator) SanitizeErrorMessage(message string) string {
	// Patterns to sanitize
	sanitizePatterns := map[string]string{
		// Paths - replace with generic path
		`/[a-zA-Z0-9/_.-]*/(users?|home)/[a-zA-Z0-9_.-]+`: "/path/to/user",
		`C:\\Users\\[a-zA-Z0-9_.-]+`:                      "C:\\Users\\user",

		// API Keys and tokens
		`[Aa]pi[_-]?[Kk]ey['":\s]*[=:]['":\s]*[a-zA-Z0-9]{20,}`: "api_key=***",
		`[Tt]oken['":\s]*[=:]['":\s]*[a-zA-Z0-9]{20,}`:          "token=***",

		// URLs with credentials
		`https?://[^:]+:[^@]+@`:   "https://***:***@",
		`mysql://[^:]+:[^@]+@`:    "mysql://***:***@",
		`postgres://[^:]+:[^@]+@`: "postgres://***:***@",

		// IP addresses (partial sanitization)
		`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.(\d{1,3})\b`: "${1}.${2}.${3}.***",

		// File paths with sensitive info
		`/etc/(passwd|shadow|hosts)`: "/etc/***",
		`C:\\Windows\\System32`:      "C:\\Windows\\***",
	}

	sanitized := message
	for pattern, replacement := range sanitizePatterns {
		re := regexp.MustCompile(pattern)
		sanitized = re.ReplaceAllString(sanitized, replacement)
	}

	return sanitized
}

// NewSecurityPolicyEngine creates a new security policy engine
func NewSecurityPolicyEngine(logger zerolog.Logger) *SecurityPolicyEngine {
	return &SecurityPolicyEngine{
		logger:   logger.With().Str("component", "security_policy_engine").Logger(),
		policies: initializeDefaultPolicies(),
	}
}

// ValidateOperation validates an operation against security policies
func (spe *SecurityPolicyEngine) ValidateOperation(operation string, params map[string]interface{}) error {
	for _, policy := range spe.policies {
		if !policy.Enabled {
			continue
		}

		for _, rule := range policy.Rules {
			if violation := spe.checkRule(rule, operation, params); violation != nil {
				return violation
			}
		}
	}
	return nil
}

// assessThreats evaluates threats for a given operation
func (sv *SecurityValidator) assessThreats(operation string, params map[string]interface{}) []ThreatInfo {
	threats := make([]ThreatInfo, 0)

	// Check operation-specific threats
	for _, threat := range sv.threatModel.Threats {
		if sv.operationMatchesThreat(operation, params, threat) {
			threats = append(threats, threat)
		}
	}

	return threats
}

// operationMatchesThreat checks if an operation matches threat criteria
func (sv *SecurityValidator) operationMatchesThreat(operation string, params map[string]interface{}, threat ThreatInfo) bool {
	// Simple pattern matching - in production this would be more sophisticated
	switch threat.Category {
	case "CONTAINER_ESCAPE":
		return strings.Contains(operation, "docker") || strings.Contains(operation, "container")
	case "CODE_INJECTION":
		if cmd, ok := params["command"].(string); ok {
			dangerousPatterns := []string{";", "&&", "||", "|", "`", "$", "(", ")"}
			for _, pattern := range dangerousPatterns {
				if strings.Contains(cmd, pattern) {
					return true
				}
			}
		}
	case "PATH_TRAVERSAL":
		if path, ok := params["path"].(string); ok {
			return strings.Contains(path, "..") || strings.Contains(path, "/etc/") || strings.Contains(path, "C:\\Windows\\")
		}
	}

	return false
}

// scanFileContent scans file content for secrets using patterns
func (sv *SecurityValidator) scanFileContent(content, filePath string, patterns map[string]*regexp.Regexp) []FileSecret {
	secrets := make([]FileSecret, 0)
	lines := strings.Split(content, "\n")

	for lineNum, line := range lines {
		for secretType, pattern := range patterns {
			matches := pattern.FindAllStringIndex(line, -1)
			for _, match := range matches {
				secret := FileSecret{
					Type:        secretType,
					Line:        lineNum + 1,
					Column:      match[0] + 1,
					Confidence:  sv.calculateConfidence(secretType, line[match[0]:match[1]]),
					Description: fmt.Sprintf("Potential %s detected", secretType),
				}
				secrets = append(secrets, secret)
			}
		}
	}

	return secrets
}

// isBinaryFile checks if a file is binary
func (sv *SecurityValidator) isBinaryFile(path string) bool {
	binaryExts := []string{".exe", ".dll", ".so", ".dylib", ".bin", ".obj", ".o", ".a", ".lib", ".zip", ".tar", ".gz", ".jpg", ".png", ".gif", ".pdf"}
	ext := strings.ToLower(filepath.Ext(path))

	for _, binaryExt := range binaryExts {
		if ext == binaryExt {
			return true
		}
	}

	return false
}

// calculateConfidence calculates confidence level for detected secret
func (sv *SecurityValidator) calculateConfidence(secretType, value string) string {
	// Simple heuristics - in production this would be more sophisticated
	switch secretType {
	case "aws_access_key", "github_token":
		return "HIGH"
	case "private_key", "ssh_private_key":
		return "HIGH"
	case "jwt_token":
		if len(value) > 100 {
			return "HIGH"
		}
		return "MEDIUM"
	default:
		return "MEDIUM"
	}
}

// calculateSeverity calculates severity level for detected secret
func (sv *SecurityValidator) calculateSeverity(secretType string) string {
	highSeverityTypes := []string{"aws_secret_key", "private_key", "ssh_private_key", "database_url"}

	for _, highType := range highSeverityTypes {
		if secretType == highType {
			return "HIGH"
		}
	}

	return "MEDIUM"
}

// checkRule checks if a rule is violated
func (spe *SecurityPolicyEngine) checkRule(rule SecurityRule, operation string, params map[string]interface{}) error {
	switch rule.RuleType {
	case "REGEX":
		if matched, _ := regexp.MatchString(rule.Pattern, operation); matched {
			if rule.Action == "BLOCK" {
				return errors.NewError().
					Code(errors.CodePermissionDenied).
					Type(errors.ErrTypeSecurity).
					Severity(errors.SeverityHigh).
					Messagef("Security policy violation: %s", rule.Description).
					Context("rule_id", rule.ID).
					Context("operation", operation).
					Context("policy_action", rule.Action).
					Suggestion("Review operation against security policies").
					WithLocation().
					Build()
			}
		}
	case "PATH":
		if path, ok := params["path"].(string); ok {
			if matched, _ := regexp.MatchString(rule.Pattern, path); matched {
				if rule.Action == "BLOCK" {
					return errors.NewError().
						Code(errors.CodePermissionDenied).
						Type(errors.ErrTypeSecurity).
						Severity(errors.SeverityHigh).
						Messagef("Path access denied by security policy: %s", rule.Description).
						Context("rule_id", rule.ID).
						Context("path", path).
						Context("policy_action", rule.Action).
						Suggestion("Use an allowed path or request policy exception").
						WithLocation().
						Build()
				}
			}
		}
	}

	return nil
}

// initializeDefaultThreatModel creates a default threat model
func initializeDefaultThreatModel() *ThreatModel {
	return &ThreatModel{
		Threats: map[string]ThreatInfo{
			"container_escape": {
				ID:          "THREAT_001",
				Name:        "Container Escape",
				Description: "Attempt to escape container sandbox",
				Impact:      "HIGH",
				Probability: "MEDIUM",
				Category:    "CONTAINER_ESCAPE",
				Mitigations: []string{"AppArmor", "SELinux", "Seccomp"},
			},
			"code_injection": {
				ID:          "THREAT_002",
				Name:        "Code Injection",
				Description: "Injection of malicious code through user input",
				Impact:      "HIGH",
				Probability: "HIGH",
				Category:    "CODE_INJECTION",
				Mitigations: []string{"Input validation", "Sandboxing", "Privilege separation"},
			},
			"path_traversal": {
				ID:          "THREAT_003",
				Name:        "Path Traversal",
				Description: "Unauthorized access to files outside allowed paths",
				Impact:      "MEDIUM",
				Probability: "MEDIUM",
				Category:    "PATH_TRAVERSAL",
				Mitigations: []string{"Path validation", "Chroot", "Containerization"},
			},
		},
		Controls: map[string]ControlInfo{
			"input_validation": {
				ID:            "CTRL_001",
				Name:          "Input Validation",
				Description:   "Validate all user inputs",
				Type:          "PREVENTIVE",
				Effectiveness: "HIGH",
				Threats:       []string{"THREAT_002"},
			},
		},
		RiskMatrix: map[string][]RiskFactor{
			"default": {
				{Factor: "impact", Weight: 0.6, Impact: "HIGH", Description: "Impact weight"},
				{Factor: "probability", Weight: 0.4, Impact: "HIGH", Description: "Probability weight"},
			},
		},
	}
}

// initializeDefaultPolicies creates default security policies
func initializeDefaultPolicies() map[string]SecurityPolicy {
	return map[string]SecurityPolicy{
		"path_security": {
			ID:          "POL_001",
			Name:        "Path Security Policy",
			Description: "Prevents access to sensitive system paths",
			Severity:    "HIGH",
			Enabled:     true,
			Rules: []SecurityRule{
				{
					ID:          "RULE_001",
					Description: "Block access to /etc directory",
					Pattern:     "/etc/.*",
					RuleType:    "PATH",
					Action:      "BLOCK",
				},
				{
					ID:          "RULE_002",
					Description: "Block access to Windows system directory",
					Pattern:     "C:\\\\Windows\\\\System32.*",
					RuleType:    "PATH",
					Action:      "BLOCK",
				},
			},
		},
		"command_injection": {
			ID:          "POL_002",
			Name:        "Command Injection Prevention",
			Description: "Prevents command injection attacks",
			Severity:    "HIGH",
			Enabled:     true,
			Rules: []SecurityRule{
				{
					ID:          "RULE_003",
					Description: "Block operations with command injection patterns",
					Pattern:     ".*(;|&&|\\|\\||`|\\$\\(|\\$\\{).*",
					RuleType:    "REGEX",
					Action:      "BLOCK",
				},
			},
		},
	}
}

// SecretScanner detects sensitive values in environment variables and content
type SecretScanner struct {
	sensitivePatterns []*regexp.Regexp
	secretManagers    []SecretManager
	logger            zerolog.Logger
}

// NewSecretScanner creates a new secret scanner with default patterns
func NewSecretScanner() *SecretScanner {
	patterns := []*regexp.Regexp{
		// Password patterns
		regexp.MustCompile(`(?i)(password|passwd|pwd).*=.*`),
		// Token patterns
		regexp.MustCompile(`(?i)(token|api_key|secret|key).*=.*`),
		// Authentication patterns
		regexp.MustCompile(`(?i)(auth|credential|access_key).*=.*`),
		// Database patterns
		regexp.MustCompile(`(?i)(db_|database_|connection_string).*=.*`),
		// Certificate patterns
		regexp.MustCompile(`(?i)(cert|certificate|private_key).*=.*`),
		// Cloud provider patterns
		regexp.MustCompile(`(?i)(aws_|azure_|gcp_|google_).*=.*`),
	}

	managers := []SecretManager{
		{
			Name:        "kubernetes-secrets",
			Description: "Native Kubernetes Secrets",
			Example:     "kubectl create secret generic my-secret --from-literal=key=value",
		},
		{
			Name:        "sealed-secrets",
			Description: "Bitnami Sealed Secrets for GitOps",
			Example:     "kubeseal -o yaml < secret.yaml > sealed-secret.yaml",
		},
		{
			Name:        "external-secrets",
			Description: "External Secrets Operator",
			Example:     "SecretStore + ExternalSecret resources",
		},
		{
			Name:        "vault",
			Description: "HashiCorp Vault integration",
			Example:     "Vault Agent or CSI driver integration",
		},
	}

	return &SecretScanner{
		sensitivePatterns: patterns,
		secretManagers:    managers,
		logger:            zerolog.New(os.Stderr).With().Str("component", "secret_scanner").Logger(),
	}
}

// ScanEnvironment scans environment variables for sensitive data
func (s *SecretScanner) ScanEnvironment(envVars map[string]string) []SensitiveEnvVar {
	var detected []SensitiveEnvVar

	for name, value := range envVars {
		if s.isSensitive(name, value) {
			detected = append(detected, SensitiveEnvVar{
				Name:          name,
				Value:         value,
				Pattern:       s.getMatchingPattern(name, value),
				Redacted:      s.redactValue(value),
				SuggestedName: s.generateSecretName(name),
			})
		}
	}

	return detected
}

// ScanContent scans content for sensitive data patterns
func (s *SecretScanner) ScanContent(content string) []SensitiveEnvVar {
	var detected []SensitiveEnvVar
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		for _, pattern := range s.sensitivePatterns {
			if pattern.MatchString(line) {
				// Extract key=value pairs
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					name := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])

					detected = append(detected, SensitiveEnvVar{
						Name:          name,
						Value:         value,
						Pattern:       pattern.String(),
						Redacted:      s.redactValue(value),
						SuggestedName: s.generateSecretName(name),
					})
				}
				break
			}
		}
	}

	return detected
}

// CreateExternalizationPlan creates a plan for externalizing detected secrets
func (s *SecretScanner) CreateExternalizationPlan(envVars map[string]string, preferredManager string) *SecretExternalizationPlan {
	detected := s.ScanEnvironment(envVars)

	plan := &SecretExternalizationPlan{
		DetectedSecrets:  detected,
		PreferredManager: preferredManager,
		SecretReferences: make(map[string]SecretReference),
		ConfigMapEntries: make(map[string]string),
	}

	// Create secret references for sensitive vars
	for _, secret := range detected {
		plan.SecretReferences[secret.Name] = SecretReference{
			SecretName: secret.SuggestedName,
			SecretKey:  strings.ToLower(secret.Name),
			EnvVarName: secret.Name,
		}
	}

	// Add non-sensitive vars to ConfigMap
	for name, value := range envVars {
		if !s.isSensitiveVar(name, detected) {
			plan.ConfigMapEntries[name] = value
		}
	}

	return plan
}

// GetSecretManagers returns supported secret management solutions
func (s *SecretScanner) GetSecretManagers() []SecretManager {
	return s.secretManagers
}

// GetRecommendedManager recommends a secret manager based on context
func (s *SecretScanner) GetRecommendedManager(hasGitOps bool, cloudProvider string) string {
	if hasGitOps {
		return "sealed-secrets"
	}
	if cloudProvider != "" {
		return "external-secrets"
	}
	return "kubernetes-secrets"
}

// GenerateSecretManifest generates a Kubernetes Secret manifest
func (s *SecretScanner) GenerateSecretManifest(secretName string, secrets map[string]string, namespace string) string {
	manifest := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: %s
type: Opaque
data:`, secretName, namespace)

	for key := range secrets {
		// In practice, you'd base64 encode the values
		manifest += fmt.Sprintf("\n  %s: <base64-encoded-value>", key)
	}

	return manifest
}

// GenerateExternalSecretManifest generates an ExternalSecret manifest
func (s *SecretScanner) GenerateExternalSecretManifest(secretName, namespace, secretStore string, mappings map[string]string) string {
	manifest := fmt.Sprintf(`apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: %s
  namespace: %s
spec:
  secretStoreRef:
    name: %s
    kind: SecretStore
  target:
    name: %s
  data:`, secretName, namespace, secretStore, secretName)

	for envVar, secretKey := range mappings {
		manifest += fmt.Sprintf("\n  - secretKey: %s\n    remoteRef:\n      key: %s", envVar, secretKey)
	}

	return manifest
}

// Private helper methods

func (s *SecretScanner) isSensitive(name, value string) bool {
	nameValue := fmt.Sprintf("%s=%s", name, value)
	for _, pattern := range s.sensitivePatterns {
		if pattern.MatchString(nameValue) {
			return true
		}
	}
	return false
}

func (s *SecretScanner) getMatchingPattern(name, value string) string {
	nameValue := fmt.Sprintf("%s=%s", name, value)
	for _, pattern := range s.sensitivePatterns {
		if pattern.MatchString(nameValue) {
			return pattern.String()
		}
	}
	return ""
}

func (s *SecretScanner) redactValue(value string) string {
	if len(value) <= 4 {
		return "***"
	}
	return value[:2] + strings.Repeat("*", len(value)-4) + value[len(value)-2:]
}

func (s *SecretScanner) generateSecretName(envVarName string) string {
	// Convert to lowercase and replace underscores
	name := strings.ToLower(envVarName)
	name = strings.ReplaceAll(name, "_", "-")
	// Ensure it's a valid Kubernetes name
	if !strings.HasSuffix(name, "-secret") {
		name += "-secret"
	}
	return name
}

func (s *SecretScanner) isSensitiveVar(name string, detected []SensitiveEnvVar) bool {
	for _, secret := range detected {
		if secret.Name == name {
			return true
		}
	}
	return false
}

// Validate implements the GenericValidator interface
func (usv *UnifiedSecurityValidator) Validate(ctx context.Context, data core.SecurityValidationData, options *core.ValidationOptions) *core.SecurityResult {
	// Convert SecurityValidationData to the format expected by ValidateSecurityUnified
	params := map[string]interface{}{
		"scan_type":         data.ScanType,
		"vulnerabilities":   data.Vulnerabilities,
		"policy_violations": data.PolicyViolations,
		"compliance_checks": data.ComplianceChecks,
	}

	result, err := usv.impl.ValidateSecurityUnified(ctx, "unified_validation", params)
	if err != nil {
		if result == nil {
			result = core.NewSecurityResult("unified_security_validator", "1.0.0")
		}
		result.AddError(core.NewSecurityError("VALIDATION_ERROR", err.Error(), "validation"))
	}
	return result
}

// GetName returns the validator name
func (usv *UnifiedSecurityValidator) GetName() string {
	return "unified_security_validator"
}

// GetVersion returns the validator version
func (usv *UnifiedSecurityValidator) GetVersion() string {
	return "1.0.0"
}

// GetSupportedTypes returns the data types this validator can handle
func (usv *UnifiedSecurityValidator) GetSupportedTypes() []string {
	return []string{"SecurityValidationData", "map[string]interface{}", "string"}
}

// ValidateWithThreatModel performs validation with threat modeling
func (usv *UnifiedSecurityValidator) ValidateWithThreatModel(ctx context.Context, operation string, params map[string]interface{}) (*core.SecurityResult, []ThreatInfo) {
	result, err := usv.impl.ValidateSecurityUnified(ctx, operation, params)
	if err != nil && result == nil {
		result = core.NewSecurityResult("unified_security_validator", "1.0.0")
		result.AddError(core.NewSecurityError("VALIDATION_ERROR", err.Error(), "validation"))
	}

	threats := usv.impl.assessThreats(operation, params)
	return result, threats
}

// ValidateSecrets performs unified secret scanning validation
func (usv *UnifiedSecurityValidator) ValidateSecrets(ctx context.Context, targetPath string) (*core.SecurityResult, error) {
	// Use the existing ScanForSecrets method but return unified result
	secretResult, err := usv.impl.ScanForSecrets(ctx, targetPath)
	if err != nil {
		result := core.NewSecurityResult("unified_security_validator", "1.0.0")
		result.AddError(core.NewSecurityError("SECRET_SCAN_ERROR", err.Error(), "secret_scanning"))
		return result, err
	}

	// Convert secret scan result to unified security result
	result := core.NewSecurityResult("unified_security_validator", "1.0.0")
	if len(secretResult.Secrets) > 0 {
		result.Valid = false
		for _, secret := range secretResult.Secrets {
			result.AddError(core.NewSecurityError("SECRET_DETECTED",
				fmt.Sprintf("Secret detected in %s: %s", secret.File, secret.Type),
				"secret_detection"))
		}
	}

	result.Data = core.SecurityValidationData{
		ScanType:        "secrets",
		Vulnerabilities: convertSecretsToVulnerabilities(secretResult.Secrets),
	}

	return result, nil
}

// Helper functions for unified validation

func convertPolicyViolations(errors []*core.Error) []core.PolicyViolation {
	violations := make([]core.PolicyViolation, 0)
	for _, err := range errors {
		if err.Type == core.ErrTypeSecurity {
			violations = append(violations, core.PolicyViolation{
				PolicyName: "security_policy",
				Rule:       err.Code,
				Message:    err.Message,
			})
		}
	}
	return violations
}

func getComplianceStatus(compliant bool) string {
	if compliant {
		return "PASS"
	}
	return "FAIL"
}

func convertSecretsToVulnerabilities(secrets []DetectedSecret) []core.Vulnerability {
	vulnerabilities := make([]core.Vulnerability, 0)
	for _, secret := range secrets {
		vulnerabilities = append(vulnerabilities, core.Vulnerability{
			ID:          fmt.Sprintf("SECRET-%s", secret.Type),
			Severity:    secret.Severity,
			Description: fmt.Sprintf("Secret detected: %s in %s", secret.Type, secret.File),
			Package:     secret.File,
		})
	}
	return vulnerabilities
}

// Migration helpers for backward compatibility

// MigrateSecurityValidatorToUnified provides a drop-in replacement for legacy SecurityValidator
func MigrateSecurityValidatorToUnified(logger zerolog.Logger) *UnifiedSecurityValidator {
	return NewUnifiedSecurityValidator(logger)
}
