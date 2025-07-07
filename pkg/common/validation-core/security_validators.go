package validation

import (
	"regexp"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// SecurityValidators consolidates all Security/Scan validation logic
// Replaces: scan/validators.go, build/security_validator.go, domain/security/validator.go
type SecurityValidators struct{}

// NewSecurityValidators creates a new Security validator
func NewSecurityValidators() *SecurityValidators {
	return &SecurityValidators{}
}

// ValidateSecretPattern validates potential secrets in code/configs
func (sv *SecurityValidators) ValidateSecretPattern(content string) error {
	// Common secret patterns
	secretPatterns := map[string]*regexp.Regexp{
		"AWS Access Key":   regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		"Generic API Key":  regexp.MustCompile(`(?i)api[_-]?key[_-]?[:=]\s*['"]*[a-zA-Z0-9]{16,}['"]*`),
		"Generic Password": regexp.MustCompile(`(?i)password[_-]?[:=]\s*['"]*[^\s'"]{8,}['"]*`),
		"JWT Token":        regexp.MustCompile(`ey[A-Za-z0-9-_=]+\.[A-Za-z0-9-_=]+\.?[A-Za-z0-9-_+/=]*`),
		"Private Key":      regexp.MustCompile(`-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----`),
		"Database URL":     regexp.MustCompile(`(?i)(postgres|mysql|mongodb)://[^\s'"]+`),
	}

	for secretType, pattern := range secretPatterns {
		if pattern.MatchString(content) {
			return errors.NewError().
				Code(errors.CodeSecurity).
				Type(errors.ErrTypeSecurity).
				Messagef("potential %s detected in content", secretType).
				Build()
		}
	}

	return nil
}

// ValidateVulnerabilityLevel validates vulnerability severity levels
func (sv *SecurityValidators) ValidateVulnerabilityLevel(level string) error {
	if level == "" {
		return errors.NewError().
			Code(errors.CodeValidation).
			Type(errors.ErrTypeValidation).
			Messagef("vulnerability level cannot be empty").
			Build()
	}

	validLevels := []string{"UNKNOWN", "LOW", "MEDIUM", "HIGH", "CRITICAL"}
	level = strings.ToUpper(level)

	for _, validLevel := range validLevels {
		if level == validLevel {
			return nil
		}
	}

	return errors.NewError().
		Code(errors.CodeValidation).
		Type(errors.ErrTypeValidation).
		Messagef("invalid vulnerability level: %s (must be one of: %s)",
			level, strings.Join(validLevels, ", ")).
		Build()
}

// ValidateCVEID validates CVE (Common Vulnerabilities and Exposures) identifiers
func (sv *SecurityValidators) ValidateCVEID(cveID string) error {
	if cveID == "" {
		return errors.NewError().
			Code(errors.CodeValidation).
			Type(errors.ErrTypeValidation).
			Messagef("CVE ID cannot be empty").
			Build()
	}

	// CVE ID format: CVE-YYYY-NNNN (where YYYY is year and NNNN is sequence number)
	cvePattern := regexp.MustCompile(`^CVE-\d{4}-\d{4,}$`)
	if !cvePattern.MatchString(cveID) {
		return errors.NewError().
			Code(errors.CodeValidation).
			Type(errors.ErrTypeValidation).
			Messagef("invalid CVE ID format: %s (expected format: CVE-YYYY-NNNN)", cveID).
			Build()
	}

	return nil
}

// ValidateDockerImageSecurity validates Docker images for security issues
func (sv *SecurityValidators) ValidateDockerImageSecurity(imageName string) error {
	// Check for latest tag usage (security anti-pattern)
	if strings.HasSuffix(imageName, ":latest") || !strings.Contains(imageName, ":") {
		return errors.NewError().
			Code(errors.CodeSecurity).
			Type(errors.ErrTypeSecurity).
			Messagef("using 'latest' tag or no tag is a security risk: %s", imageName).
			Build()
	}

	// Check for privileged base images
	privilegedImages := []string{
		"ubuntu:latest",
		"centos:latest",
		"debian:latest",
		"alpine:latest",
	}

	for _, privileged := range privilegedImages {
		if strings.Contains(imageName, privileged) {
			return errors.NewError().
				Code(errors.CodeSecurity).
				Type(errors.ErrTypeSecurity).
				Messagef("potentially unsafe base image detected: %s", imageName).
				Build()
		}
	}

	return nil
}

// ValidateContainerSecurityContext validates Kubernetes security contexts
func (sv *SecurityValidators) ValidateContainerSecurityContext(securityContext map[string]interface{}) error {
	// Check for privileged containers
	if privileged, exists := securityContext["privileged"]; exists {
		if p, ok := privileged.(bool); ok && p {
			return errors.NewError().
				Code(errors.CodeSecurity).
				Type(errors.ErrTypeSecurity).
				Messagef("privileged containers are not recommended").
				Build()
		}
	}

	// Check for root user
	if runAsUser, exists := securityContext["runAsUser"]; exists {
		if uid, ok := runAsUser.(float64); ok && uid == 0 {
			return errors.NewError().
				Code(errors.CodeSecurity).
				Type(errors.ErrTypeSecurity).
				Messagef("running as root user (UID 0) is not recommended").
				Build()
		}
	}

	// Check for capability additions
	if capabilities, exists := securityContext["capabilities"]; exists {
		if capMap, ok := capabilities.(map[string]interface{}); ok {
			if add, exists := capMap["add"]; exists {
				if addList, ok := add.([]interface{}); ok && len(addList) > 0 {
					return errors.NewError().
						Code(errors.CodeSecurity).
						Type(errors.ErrTypeSecurity).
						Messagef("adding capabilities increases security risk").
						Build()
				}
			}
		}
	}

	return nil
}

// ValidateNetworkPolicy validates Kubernetes network policies for security
func (sv *SecurityValidators) ValidateNetworkPolicy(policy map[string]interface{}) error {
	spec, ok := policy["spec"].(map[string]interface{})
	if !ok {
		return errors.NewError().
			Code(errors.CodeValidation).
			Type(errors.ErrTypeValidation).
			Messagef("network policy spec is required").
			Build()
	}

	// Check for default deny policy
	policyTypes, exists := spec["policyTypes"]
	if !exists {
		return errors.NewError().
			Code(errors.CodeSecurity).
			Type(errors.ErrTypeSecurity).
			Messagef("network policy should specify policyTypes for better security").
			Build()
	}

	// Validate policy types
	if typeList, ok := policyTypes.([]interface{}); ok {
		validTypes := map[string]bool{"Ingress": true, "Egress": true}
		for _, pType := range typeList {
			if typeStr, ok := pType.(string); ok {
				if !validTypes[typeStr] {
					return errors.NewError().
						Code(errors.CodeValidation).
						Type(errors.ErrTypeValidation).
						Messagef("invalid network policy type: %s", typeStr).
						Build()
				}
			}
		}
	}

	return nil
}

// ValidateScanResult validates security scan results format
func (sv *SecurityValidators) ValidateScanResult(result map[string]interface{}) error {
	// Check required fields
	requiredFields := []string{"vulnerabilities", "summary"}
	for _, field := range requiredFields {
		if _, exists := result[field]; !exists {
			return errors.NewError().
				Code(errors.CodeValidation).
				Type(errors.ErrTypeValidation).
				Messagef("scan result missing required field: %s", field).
				Build()
		}
	}

	// Validate vulnerability structure
	if vulns, exists := result["vulnerabilities"]; exists {
		if vulnList, ok := vulns.([]interface{}); ok {
			for i, vuln := range vulnList {
				if vulnMap, ok := vuln.(map[string]interface{}); ok {
					if err := sv.validateVulnerabilityEntry(vulnMap, i); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

// validateVulnerabilityEntry validates individual vulnerability entries
func (sv *SecurityValidators) validateVulnerabilityEntry(vuln map[string]interface{}, index int) error {
	// Check required vulnerability fields
	requiredFields := []string{"id", "severity", "package"}
	for _, field := range requiredFields {
		if _, exists := vuln[field]; !exists {
			return errors.NewError().
				Code(errors.CodeValidation).
				Type(errors.ErrTypeValidation).
				Messagef("vulnerability entry %d missing required field: %s", index, field).
				Build()
		}
	}

	// Validate severity level
	if severity, exists := vuln["severity"]; exists {
		if severityStr, ok := severity.(string); ok {
			if err := sv.ValidateVulnerabilityLevel(severityStr); err != nil {
				return err
			}
		}
	}

	return nil
}
