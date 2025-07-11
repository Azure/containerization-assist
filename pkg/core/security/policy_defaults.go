package security

import "time"

// LoadDefaultPolicies loads a set of default security policies
func (pe *PolicyEngine) LoadDefaultPolicies() error {
	defaultPolicies := []Policy{
		{
			ID:          "critical-vulns-block",
			Name:        "Block Critical Vulnerabilities",
			Description: "Block images with critical vulnerabilities",
			Enabled:     true,
			Severity:    PolicySeverityCritical,
			Category:    PolicyCategoryVulnerability,
			Rules: []PolicyRule{
				{
					ID:          "critical-count",
					Type:        RuleTypeVulnerabilityCount,
					Field:       "critical",
					Operator:    OperatorGreaterThan,
					Value:       0,
					Description: "No critical vulnerabilities allowed",
				},
			},
			Actions: []PolicyAction{
				{
					Type:        ActionTypeBlock,
					Description: "Block deployment",
				},
				{
					Type:        ActionTypeNotify,
					Description: "Notify security team",
					Parameters: map[string]string{
						"channel": "security-alerts",
					},
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "high-vulns-warn",
			Name:        "Warn on High Vulnerabilities",
			Description: "Warn when images have high severity vulnerabilities",
			Enabled:     true,
			Severity:    PolicySeverityHigh,
			Category:    PolicyCategoryVulnerability,
			Rules: []PolicyRule{
				{
					ID:          "high-count",
					Type:        RuleTypeVulnerabilityCount,
					Field:       "high",
					Operator:    OperatorGreaterThan,
					Value:       5,
					Description: "More than 5 high vulnerabilities",
				},
			},
			Actions: []PolicyAction{
				{
					Type:        ActionTypeWarn,
					Description: "Issue warning",
				},
				{
					Type:        ActionTypeLog,
					Description: "Log to security audit",
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "cvss-score-limit",
			Name:        "CVSS Score Limit",
			Description: "Block images with vulnerabilities above CVSS 9.0",
			Enabled:     true,
			Severity:    PolicySeverityCritical,
			Category:    PolicyCategoryVulnerability,
			Rules: []PolicyRule{
				{
					ID:          "cvss-max",
					Type:        RuleTypeCVSSScore,
					Field:       "max_cvss",
					Operator:    OperatorGreaterThan,
					Value:       9.0,
					Description: "CVSS score above 9.0",
				},
			},
			Actions: []PolicyAction{
				{
					Type:        ActionTypeBlock,
					Description: "Block deployment",
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "no-secrets",
			Name:        "No Secrets Allowed",
			Description: "Block images containing exposed secrets",
			Enabled:     true,
			Severity:    PolicySeverityCritical,
			Category:    PolicyCategorySecret,
			Rules: []PolicyRule{
				{
					ID:          "secret-presence",
					Type:        RuleTypeSecretPresence,
					Field:       "secrets",
					Operator:    OperatorEquals,
					Value:       false,
					Description: "No secrets should be present",
				},
			},
			Actions: []PolicyAction{
				{
					Type:        ActionTypeBlock,
					Description: "Block deployment",
				},
				{
					Type:        ActionTypeNotify,
					Description: "Notify security team immediately",
					Parameters: map[string]string{
						"channel":  "security-critical",
						"priority": "high",
					},
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "image-age-limit",
			Name:        "Image Age Limit",
			Description: "Warn on images older than 30 days",
			Enabled:     true,
			Severity:    PolicySeverityMedium,
			Category:    PolicyCategoryImage,
			Rules: []PolicyRule{
				{
					ID:          "max-age-days",
					Type:        RuleTypeImageAge,
					Field:       "age_days",
					Operator:    OperatorGreaterThan,
					Value:       30,
					Description: "Image older than 30 days",
				},
			},
			Actions: []PolicyAction{
				{
					Type:        ActionTypeWarn,
					Description: "Warn about old image",
				},
				{
					Type:        ActionTypeLog,
					Description: "Log to compliance audit",
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "image-size-limit",
			Name:        "Image Size Limit",
			Description: "Warn on images larger than 1GB",
			Enabled:     true,
			Severity:    PolicySeverityLow,
			Category:    PolicyCategoryImage,
			Rules: []PolicyRule{
				{
					ID:          "max-size-mb",
					Type:        RuleTypeImageSize,
					Field:       "size_mb",
					Operator:    OperatorGreaterThan,
					Value:       1024, // 1GB in MB
					Description: "Image larger than 1GB",
				},
			},
			Actions: []PolicyAction{
				{
					Type:        ActionTypeWarn,
					Description: "Warn about large image size",
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "approved-licenses",
			Name:        "Approved Licenses Only",
			Description: "Ensure only approved licenses are used",
			Enabled:     true,
			Severity:    PolicySeverityHigh,
			Category:    PolicyCategoryCompliance,
			Rules: []PolicyRule{
				{
					ID:       "license-whitelist",
					Type:     RuleTypeLicense,
					Field:    "licenses",
					Operator: OperatorIn,
					Value: []string{
						"MIT",
						"Apache-2.0",
						"BSD-3-Clause",
						"BSD-2-Clause",
						"ISC",
						"MPL-2.0",
					},
					Description: "Only approved open source licenses",
				},
			},
			Actions: []PolicyAction{
				{
					Type:        ActionTypeWarn,
					Description: "Warn about unapproved licenses",
				},
				{
					Type:        ActionTypeLog,
					Description: "Log to compliance audit",
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "package-version-ban",
			Name:        "Banned Package Versions",
			Description: "Block specific vulnerable package versions",
			Enabled:     true,
			Severity:    PolicySeverityCritical,
			Category:    PolicyCategoryVulnerability,
			Rules: []PolicyRule{
				{
					ID:          "log4j-vulnerable",
					Type:        RuleTypePackageVersion,
					Field:       "log4j",
					Operator:    OperatorEquals,
					Value:       "2.14.1", // Known vulnerable version
					Description: "Vulnerable log4j version",
				},
			},
			Actions: []PolicyAction{
				{
					Type:        ActionTypeBlock,
					Description: "Block deployment due to vulnerable package",
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	return pe.LoadPolicies(defaultPolicies)
}
