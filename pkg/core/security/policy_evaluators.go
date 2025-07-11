package security

import (
	"fmt"
	"strconv"
	"time"
)

// evaluateRule evaluates a single policy rule against the scan context
func (pe *PolicyEngine) evaluateRule(rule PolicyRule, scanCtx *ScanContext) (*PolicyViolation, error) {
	switch rule.Type {
	case RuleTypeVulnerabilityCount:
		return pe.evaluateVulnerabilityCountRule(rule, scanCtx)
	case RuleTypeVulnerabilitySeverity:
		return pe.evaluateVulnerabilitySeverityRule(rule, scanCtx)
	case RuleTypeCVSSScore:
		return pe.evaluateCVSSScoreRule(rule, scanCtx)
	case RuleTypeSecretPresence:
		return pe.evaluateSecretPresenceRule(rule, scanCtx)
	case RuleTypePackageVersion:
		return pe.evaluatePackageVersionRule(rule, scanCtx)
	case RuleTypeImageAge:
		return pe.evaluateImageAgeRule(rule, scanCtx)
	case RuleTypeImageSize:
		return pe.evaluateImageSizeRule(rule, scanCtx)
	case RuleTypeLicense:
		return pe.evaluateLicenseRule(rule, scanCtx)
	default:
		return nil, fmt.Errorf("unsupported rule type: %s", rule.Type)
	}
}

// evaluateVulnerabilityCountRule evaluates vulnerability count rules
func (pe *PolicyEngine) evaluateVulnerabilityCountRule(rule PolicyRule, scanCtx *ScanContext) (*PolicyViolation, error) {
	var actualCount int
	var expectedCount int

	// Determine which severity to count based on rule field
	switch rule.Field {
	case "critical":
		actualCount = scanCtx.VulnSummary.Critical
	case "high":
		actualCount = scanCtx.VulnSummary.High
	case "medium":
		actualCount = scanCtx.VulnSummary.Medium
	case "low":
		actualCount = scanCtx.VulnSummary.Low
	case "total":
		actualCount = scanCtx.VulnSummary.Total
	default:
		return nil, fmt.Errorf("invalid vulnerability count field: %s", rule.Field)
	}

	expectedCount = int(pe.toFloat64(rule.Value))

	// For vulnerability count rules, the rule defines a violation condition
	// (e.g., "if count > threshold, it's a violation")
	// rather than a desired state like other rules
	var violation bool
	switch rule.Operator {
	case OperatorGreaterThan:
		violation = actualCount > expectedCount
	case OperatorGreaterThanOrEqual:
		violation = actualCount >= expectedCount
	case OperatorLessThan:
		violation = actualCount < expectedCount
	case OperatorLessThanOrEqual:
		violation = actualCount <= expectedCount
	case OperatorEquals:
		violation = actualCount != expectedCount // Violation when NOT equal (e.g., should be 0 but isn't)
	case OperatorNotEquals:
		violation = actualCount == expectedCount // Violation when equal
	default:
		violation = !pe.compareValues(actualCount, rule.Operator, expectedCount)
	}

	if violation {
		return &PolicyViolation{
			RuleID:        rule.ID,
			Description:   fmt.Sprintf("Vulnerability count check failed: %s %s vulnerabilities found", rule.Field, strconv.Itoa(actualCount)),
			Field:         rule.Field,
			ActualValue:   actualCount,
			ExpectedValue: expectedCount,
			Context: map[string]interface{}{
				"vulnerability_summary": scanCtx.VulnSummary,
			},
		}, nil
	}

	return nil, nil
}

// evaluateVulnerabilitySeverityRule evaluates vulnerability severity rules
func (pe *PolicyEngine) evaluateVulnerabilitySeverityRule(rule PolicyRule, scanCtx *ScanContext) (*PolicyViolation, error) {
	severityOrder := map[string]int{
		"low":      1,
		"medium":   2,
		"high":     3,
		"critical": 4,
	}

	// Find the highest severity vulnerability
	var highestSeverity string
	highestSeverityOrder := 0

	for _, vuln := range scanCtx.Vulnerabilities {
		if order, exists := severityOrder[vuln.Severity]; exists && order > highestSeverityOrder {
			highestSeverity = vuln.Severity
			highestSeverityOrder = order
		}
	}

	expectedSeverity, ok := rule.Value.(string)
	if !ok {
		return nil, fmt.Errorf("invalid severity value type")
	}

	expectedOrder := severityOrder[expectedSeverity]

	if !pe.compareValues(highestSeverityOrder, rule.Operator, expectedOrder) {
		return &PolicyViolation{
			RuleID:        rule.ID,
			Description:   fmt.Sprintf("Vulnerability severity check failed: %s severity vulnerabilities found", highestSeverity),
			Field:         "severity",
			ActualValue:   highestSeverity,
			ExpectedValue: expectedSeverity,
		}, nil
	}

	return nil, nil
}

// evaluateCVSSScoreRule evaluates CVSS score rules
func (pe *PolicyEngine) evaluateCVSSScoreRule(rule PolicyRule, scanCtx *ScanContext) (*PolicyViolation, error) {
	var maxCVSS float64

	// Find the maximum CVSS score (check both CVSS and CVSSV3)
	for _, vuln := range scanCtx.Vulnerabilities {
		if vuln.CVSS.Score > maxCVSS {
			maxCVSS = vuln.CVSS.Score
		}
		// Also check CVSSV3 score
		if vuln.CVSSV3.Score > maxCVSS {
			maxCVSS = vuln.CVSSV3.Score
		}
	}

	expectedCVSS := pe.toFloat64(rule.Value)

	if pe.compareValues(maxCVSS, rule.Operator, expectedCVSS) {
		return &PolicyViolation{
			RuleID:        rule.ID,
			Description:   fmt.Sprintf("CVSS score check failed: maximum score %.1f found", maxCVSS),
			Field:         "cvss_score",
			ActualValue:   maxCVSS,
			ExpectedValue: expectedCVSS,
			Context: map[string]interface{}{
				"vulnerabilities_with_high_cvss": pe.getHighCVSSVulnerabilities(scanCtx.Vulnerabilities, expectedCVSS),
			},
		}, nil
	}

	return nil, nil
}

// evaluateSecretPresenceRule evaluates secret presence rules
func (pe *PolicyEngine) evaluateSecretPresenceRule(rule PolicyRule, scanCtx *ScanContext) (*PolicyViolation, error) {
	// Count only non-false-positive secrets
	realSecretCount := 0
	for _, finding := range scanCtx.SecretFindings {
		if !finding.FalsePositive {
			realSecretCount++
		}
	}

	hasSecrets := realSecretCount > 0
	expectedValue, ok := rule.Value.(bool)
	if !ok {
		// Default to checking if secrets should not be present
		expectedValue = false
	}

	if hasSecrets && !expectedValue {
		return &PolicyViolation{
			RuleID:        rule.ID,
			Description:   fmt.Sprintf("Secret presence check failed: %d secrets found", realSecretCount),
			Field:         "secrets",
			ActualValue:   true,
			ExpectedValue: false,
			Context: map[string]interface{}{
				"secret_count": realSecretCount,
				"secret_types": pe.getSecretTypes(scanCtx.SecretFindings),
			},
		}, nil
	}

	if !hasSecrets && expectedValue {
		return &PolicyViolation{
			RuleID:        rule.ID,
			Description:   "Secret presence check failed: no secrets found when expected",
			Field:         "secrets",
			ActualValue:   false,
			ExpectedValue: true,
		}, nil
	}

	return nil, nil
}

// evaluatePackageVersionRule evaluates package version rules
func (pe *PolicyEngine) evaluatePackageVersionRule(rule PolicyRule, scanCtx *ScanContext) (*PolicyViolation, error) {
	packageName, ok := rule.Field, true
	if !ok || packageName == "" {
		return nil, fmt.Errorf("package name not specified in rule field")
	}

	var foundPackage *PackageInfo
	for _, pkg := range scanCtx.Packages {
		if pkg.Name == packageName {
			foundPackage = &pkg
			break
		}
	}

	if foundPackage == nil {
		// Package not found - this might be a violation depending on the operator
		if rule.Operator == OperatorNotEquals || rule.Operator == OperatorNotContains {
			return nil, nil // Not a violation if we're checking for absence
		}
		return &PolicyViolation{
			RuleID:        rule.ID,
			Description:   fmt.Sprintf("Package %s not found", packageName),
			Field:         "package",
			ActualValue:   nil,
			ExpectedValue: rule.Value,
		}, nil
	}

	expectedVersion, ok := rule.Value.(string)
	if !ok {
		return nil, fmt.Errorf("invalid package version value type")
	}

	if !pe.compareValues(foundPackage.Version, rule.Operator, expectedVersion) {
		return &PolicyViolation{
			RuleID:        rule.ID,
			Description:   fmt.Sprintf("Package version check failed: %s version %s", packageName, foundPackage.Version),
			Field:         "package_version",
			ActualValue:   foundPackage.Version,
			ExpectedValue: expectedVersion,
			Context: map[string]interface{}{
				"package": foundPackage,
			},
		}, nil
	}

	return nil, nil
}

// evaluateImageAgeRule evaluates image age rules
func (pe *PolicyEngine) evaluateImageAgeRule(rule PolicyRule, scanCtx *ScanContext) (*PolicyViolation, error) {
	// Extract image creation time from metadata
	createdAt, ok := scanCtx.ImageMetadata["created_at"].(time.Time)
	if !ok {
		// Try to parse string representation
		if createdStr, ok := scanCtx.ImageMetadata["created_at"].(string); ok {
			var err error
			createdAt, err = time.Parse(time.RFC3339, createdStr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse image creation time: %w", err)
			}
		} else {
			return nil, fmt.Errorf("image creation time not found in metadata")
		}
	}

	age := time.Since(createdAt)
	maxAgeDays := int(pe.toFloat64(rule.Value))
	// maxAge := time.Duration(maxAgeDays) * 24 * time.Hour // Not used in comparison

	if !pe.compareValues(age.Hours()/24, rule.Operator, float64(maxAgeDays)) {
		return &PolicyViolation{
			RuleID:        rule.ID,
			Description:   fmt.Sprintf("Image age check failed: image is %.0f days old", age.Hours()/24),
			Field:         "image_age",
			ActualValue:   int(age.Hours() / 24),
			ExpectedValue: maxAgeDays,
			Context: map[string]interface{}{
				"created_at": createdAt,
				"age_hours":  age.Hours(),
			},
		}, nil
	}

	return nil, nil
}

// evaluateImageSizeRule evaluates image size rules
func (pe *PolicyEngine) evaluateImageSizeRule(rule PolicyRule, scanCtx *ScanContext) (*PolicyViolation, error) {
	// Extract image size from metadata
	sizeBytes, ok := scanCtx.ImageMetadata["size_bytes"].(int64)
	if !ok {
		// Try float64 (JSON numbers are often parsed as float64)
		if sizeFloat, ok := scanCtx.ImageMetadata["size_bytes"].(float64); ok {
			sizeBytes = int64(sizeFloat)
		} else {
			return nil, fmt.Errorf("image size not found in metadata")
		}
	}

	maxSizeMB := pe.toFloat64(rule.Value)
	// maxSizeBytes := int64(maxSizeMB * 1024 * 1024) // Not used in comparison
	sizeMB := float64(sizeBytes) / (1024 * 1024)

	if !pe.compareValues(sizeMB, rule.Operator, maxSizeMB) {
		return &PolicyViolation{
			RuleID:        rule.ID,
			Description:   fmt.Sprintf("Image size check failed: image is %.1f MB", sizeMB),
			Field:         "image_size",
			ActualValue:   sizeMB,
			ExpectedValue: maxSizeMB,
			Context: map[string]interface{}{
				"size_bytes": sizeBytes,
			},
		}, nil
	}

	return nil, nil
}

// evaluateLicenseRule evaluates license rules
func (pe *PolicyEngine) evaluateLicenseRule(rule PolicyRule, scanCtx *ScanContext) (*PolicyViolation, error) {
	// Collect all licenses from packages
	foundLicenses := make(map[string]bool)
	for _, pkg := range scanCtx.Packages {
		for _, license := range pkg.Licenses {
			foundLicenses[license] = true
		}
	}

	// Convert expected value to string slice
	var expectedLicenses []string
	switch v := rule.Value.(type) {
	case string:
		expectedLicenses = []string{v}
	case []string:
		expectedLicenses = v
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				expectedLicenses = append(expectedLicenses, s)
			}
		}
	default:
		return nil, fmt.Errorf("invalid license value type")
	}

	// Check based on operator
	violation := pe.evaluateLicenseCompliance(foundLicenses, expectedLicenses, rule.Operator)
	if violation {
		var foundList []string
		for license := range foundLicenses {
			foundList = append(foundList, license)
		}

		return &PolicyViolation{
			RuleID:        rule.ID,
			Description:   fmt.Sprintf("License check failed: found licenses %v", foundList),
			Field:         "licenses",
			ActualValue:   foundList,
			ExpectedValue: expectedLicenses,
		}, nil
	}

	return nil, nil
}

// Helper methods

func (pe *PolicyEngine) getHighCVSSVulnerabilities(vulns []Vulnerability, threshold float64) []map[string]interface{} {
	var highCVSS []map[string]interface{}
	for _, vuln := range vulns {
		// Check both CVSS and CVSSV3 scores
		maxScore := vuln.CVSS.Score
		if vuln.CVSSV3.Score > maxScore {
			maxScore = vuln.CVSSV3.Score
		}

		if maxScore >= threshold {
			highCVSS = append(highCVSS, map[string]interface{}{
				"id":         vuln.VulnerabilityID,
				"cvss_score": maxScore,
				"severity":   vuln.Severity,
			})
		}
	}
	return highCVSS
}

func (pe *PolicyEngine) getSecretTypes(findings []ExtendedSecretFinding) []string {
	typeMap := make(map[string]bool)
	for _, finding := range findings {
		typeMap[finding.Type] = true
	}

	var types []string
	for t := range typeMap {
		types = append(types, t)
	}
	return types
}

func (pe *PolicyEngine) evaluateLicenseCompliance(foundLicenses map[string]bool, expectedLicenses []string, operator RuleOperator) bool {
	switch operator {
	case OperatorContains:
		// All expected licenses should be found
		for _, expected := range expectedLicenses {
			if !foundLicenses[expected] {
				return true // violation
			}
		}
		return false
	case OperatorNotContains:
		// None of the expected licenses should be found
		for _, expected := range expectedLicenses {
			if foundLicenses[expected] {
				return true // violation
			}
		}
		return false
	case OperatorIn:
		// All found licenses should be in the expected list
		expectedMap := make(map[string]bool)
		for _, exp := range expectedLicenses {
			expectedMap[exp] = true
		}
		for found := range foundLicenses {
			if !expectedMap[found] {
				return true // violation
			}
		}
		return false
	case OperatorNotIn:
		// No found licenses should be in the expected list
		for _, expected := range expectedLicenses {
			if foundLicenses[expected] {
				return true // violation
			}
		}
		return false
	default:
		// For other operators, just check if any license matches
		for _, expected := range expectedLicenses {
			if foundLicenses[expected] {
				return operator != OperatorEquals
			}
		}
		return operator == OperatorEquals
	}
}
