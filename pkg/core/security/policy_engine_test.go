package security

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPolicyEngine(t *testing.T) {
	logger := zerolog.Nop()
	engine := NewPolicyEngine(logger)

	assert.NotNil(t, engine)
	assert.Empty(t, engine.policies)
}

func TestLoadDefaultPolicies(t *testing.T) {
	logger := zerolog.Nop()
	engine := NewPolicyEngine(logger)

	err := engine.LoadDefaultPolicies()
	require.NoError(t, err)

	policies := engine.GetPolicies()
	assert.Len(t, policies, 5) // Expected default policies

	// Check specific policies exist
	policyMap := make(map[string]SecurityPolicy)
	for _, policy := range policies {
		policyMap[policy.ID] = policy
	}

	assert.Contains(t, policyMap, "critical-vulns-block")
	assert.Contains(t, policyMap, "high-vulns-warn")
	assert.Contains(t, policyMap, "cvss-threshold")
	assert.Contains(t, policyMap, "secrets-block")
	assert.Contains(t, policyMap, "outdated-packages")
}

func TestEvaluateVulnerabilityCountRule(t *testing.T) {
	logger := zerolog.Nop()
	engine := NewPolicyEngine(logger)

	tests := []struct {
		name            string
		rule            PolicyRule
		scanCtx         *SecurityScanContext
		expectError     bool
		expectViolation bool
	}{
		{
			name: "critical vulnerabilities exceed threshold",
			rule: PolicyRule{
				ID:       "test-critical",
				Type:     RuleTypeVulnerabilityCount,
				Field:    "critical",
				Operator: OperatorGreaterThan,
				Value:    float64(0),
			},
			scanCtx: &SecurityScanContext{
				VulnSummary: VulnerabilitySummary{
					Critical: 2,
					High:     1,
					Total:    3,
				},
			},
			expectViolation: true,
		},
		{
			name: "no critical vulnerabilities - within threshold",
			rule: PolicyRule{
				ID:       "test-critical",
				Type:     RuleTypeVulnerabilityCount,
				Field:    "critical",
				Operator: OperatorGreaterThan,
				Value:    float64(0),
			},
			scanCtx: &SecurityScanContext{
				VulnSummary: VulnerabilitySummary{
					Critical: 0,
					High:     2,
					Total:    2,
				},
			},
			expectViolation: false,
		},
		{
			name: "high vulnerabilities within threshold",
			rule: PolicyRule{
				ID:       "test-high",
				Type:     RuleTypeVulnerabilityCount,
				Field:    "high",
				Operator: OperatorGreaterThan,
				Value:    float64(5),
			},
			scanCtx: &SecurityScanContext{
				VulnSummary: VulnerabilitySummary{
					High:  3,
					Total: 3,
				},
			},
			expectViolation: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violation, err := engine.evaluateVulnerabilityCountRule(tt.rule, tt.scanCtx)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			if tt.expectViolation {
				assert.NotNil(t, violation)
				assert.Equal(t, tt.rule.ID, violation.RuleID)
			} else {
				assert.Nil(t, violation)
			}
		})
	}
}

func TestEvaluateCVSSScoreRule(t *testing.T) {
	logger := zerolog.Nop()
	engine := NewPolicyEngine(logger)

	rule := PolicyRule{
		ID:       "test-cvss",
		Type:     RuleTypeCVSSScore,
		Field:    "max_cvss_score",
		Operator: OperatorGreaterThan,
		Value:    float64(7.0),
	}

	tests := []struct {
		name            string
		vulnerabilities []Vulnerability
		expectViolation bool
	}{
		{
			name: "high CVSS score triggers violation",
			vulnerabilities: []Vulnerability{
				{
					VulnerabilityID: "CVE-2023-1234",
					CVSSV3: CVSSV3Info{
						Score: 8.5,
					},
				},
				{
					VulnerabilityID: "CVE-2023-5678",
					CVSS: CVSSInfo{
						Score: 6.0,
					},
				},
			},
			expectViolation: true,
		},
		{
			name: "low CVSS scores no violation",
			vulnerabilities: []Vulnerability{
				{
					VulnerabilityID: "CVE-2023-1234",
					CVSS: CVSSInfo{
						Score: 5.0,
					},
				},
				{
					VulnerabilityID: "CVE-2023-5678",
					CVSS: CVSSInfo{
						Score: 6.5,
					},
				},
			},
			expectViolation: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanCtx := &SecurityScanContext{
				Vulnerabilities: tt.vulnerabilities,
			}

			violation, err := engine.evaluateCVSSScoreRule(rule, scanCtx)
			assert.NoError(t, err)

			if tt.expectViolation {
				assert.NotNil(t, violation)
				assert.Equal(t, rule.ID, violation.RuleID)
			} else {
				assert.Nil(t, violation)
			}
		})
	}
}

func TestEvaluateSecretPresenceRule(t *testing.T) {
	logger := zerolog.Nop()
	engine := NewPolicyEngine(logger)

	rule := PolicyRule{
		ID:       "test-secrets",
		Type:     RuleTypeSecretPresence,
		Field:    "secrets_found",
		Operator: OperatorGreaterThan,
		Value:    float64(0),
	}

	tests := []struct {
		name            string
		secretFindings  []SecretFinding
		secretSummary   *DiscoverySummary
		expectViolation bool
	}{
		{
			name: "secrets found triggers violation",
			secretFindings: []SecretFinding{
				{
					SecretType:    "api_key",
					FalsePositive: false,
				},
				{
					SecretType:    "password",
					FalsePositive: true, // This should be excluded
				},
			},
			secretSummary: &DiscoverySummary{
				TotalFindings:  2,
				FalsePositives: 1,
			},
			expectViolation: true,
		},
		{
			name: "only false positives no violation",
			secretFindings: []SecretFinding{
				{
					SecretType:    "password",
					FalsePositive: true,
				},
			},
			secretSummary: &DiscoverySummary{
				TotalFindings:  1,
				FalsePositives: 1,
			},
			expectViolation: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanCtx := &SecurityScanContext{
				SecretFindings: tt.secretFindings,
				SecretSummary:  tt.secretSummary,
			}

			violation, err := engine.evaluateSecretPresenceRule(rule, scanCtx)
			assert.NoError(t, err)

			if tt.expectViolation {
				assert.NotNil(t, violation)
				assert.Equal(t, rule.ID, violation.RuleID)
			} else {
				assert.Nil(t, violation)
			}
		})
	}
}

func TestEvaluatePolicies(t *testing.T) {
	logger := zerolog.Nop()
	engine := NewPolicyEngine(logger)

	// Load default policies
	err := engine.LoadDefaultPolicies()
	require.NoError(t, err)

	ctx := context.Background()
	scanCtx := &SecurityScanContext{
		ImageRef: "test:latest",
		ScanTime: time.Now(),
		VulnSummary: VulnerabilitySummary{
			Total:    10,
			Critical: 2,
			High:     3,
			Medium:   3,
			Low:      2,
			Fixable:  8,
		},
		Vulnerabilities: []Vulnerability{
			{
				VulnerabilityID: "CVE-2023-1234",
				Severity:        "CRITICAL",
				CVSSV3: CVSSV3Info{
					Score: 9.0,
				},
			},
		},
		SecretSummary: &DiscoverySummary{
			TotalFindings:  1,
			FalsePositives: 0,
		},
		SecretFindings: []SecretFinding{
			{
				SecretType:    "api_key",
				FalsePositive: false,
			},
		},
	}

	results, err := engine.EvaluatePolicies(ctx, scanCtx)
	assert.NoError(t, err)

	// Should have evaluated all enabled policies
	enabledCount := 0
	for _, policy := range engine.GetPolicies() {
		if policy.Enabled {
			enabledCount++
		}
	}
	assert.Len(t, results, enabledCount)

	// Check that critical policies failed
	var criticalPolicy *PolicyEvaluationResult
	for _, result := range results {
		if result.PolicyID == "critical-vulns-block" {
			criticalPolicy = &result
			break
		}
	}

	require.NotNil(t, criticalPolicy)
	assert.False(t, criticalPolicy.Passed)
	assert.NotEmpty(t, criticalPolicy.Violations)

	// Check that blocking action is present
	hasBlockingAction := false
	for _, action := range criticalPolicy.Actions {
		if action.Type == ActionTypeBlock {
			hasBlockingAction = true
			break
		}
	}
	assert.True(t, hasBlockingAction)
}

func TestShouldBlock(t *testing.T) {
	logger := zerolog.Nop()
	engine := NewPolicyEngine(logger)

	tests := []struct {
		name        string
		results     []PolicyEvaluationResult
		shouldBlock bool
	}{
		{
			name: "blocking action should block",
			results: []PolicyEvaluationResult{
				{
					PolicyID: "test-policy",
					Passed:   false,
					Actions: []PolicyAction{
						{Type: ActionTypeBlock},
					},
				},
			},
			shouldBlock: true,
		},
		{
			name: "only warning actions should not block",
			results: []PolicyEvaluationResult{
				{
					PolicyID: "test-policy",
					Passed:   false,
					Actions: []PolicyAction{
						{Type: ActionTypeWarn},
						{Type: ActionTypeLog},
					},
				},
			},
			shouldBlock: false,
		},
		{
			name: "passed policies should not block",
			results: []PolicyEvaluationResult{
				{
					PolicyID: "test-policy",
					Passed:   true,
				},
			},
			shouldBlock: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldBlock := engine.ShouldBlock(tt.results)
			assert.Equal(t, tt.shouldBlock, shouldBlock)
		})
	}
}

func TestGetViolationsSummary(t *testing.T) {
	logger := zerolog.Nop()
	engine := NewPolicyEngine(logger)

	results := []PolicyEvaluationResult{
		{
			PolicyID: "policy1",
			Passed:   true,
		},
		{
			PolicyID: "policy2",
			Passed:   false,
			Violations: []PolicyViolation{
				{Severity: PolicySeverityCritical},
				{Severity: PolicySeverityHigh},
			},
			Actions: []PolicyAction{
				{Type: ActionTypeBlock},
				{Type: ActionTypeNotify},
			},
		},
		{
			PolicyID: "policy3",
			Passed:   false,
			Violations: []PolicyViolation{
				{Severity: PolicySeverityMedium},
			},
			Actions: []PolicyAction{
				{Type: ActionTypeWarn},
			},
		},
	}

	summary := engine.GetViolationsSummary(results)

	assert.Equal(t, 3, summary["total_policies"])
	assert.Equal(t, 1, summary["passed_policies"])
	assert.Equal(t, 2, summary["failed_policies"])
	assert.Equal(t, 3, summary["total_violations"])
	assert.Equal(t, 1, summary["blocking_policies"])

	severityCounts := summary["severity_counts"].(map[string]int)
	assert.Equal(t, 1, severityCounts["critical"])
	assert.Equal(t, 1, severityCounts["high"])
	assert.Equal(t, 1, severityCounts["medium"])
	assert.Equal(t, 0, severityCounts["low"])

	actionCounts := summary["action_counts"].(map[string]int)
	assert.Equal(t, 1, actionCounts["block"])
	assert.Equal(t, 1, actionCounts["warn"])
	assert.Equal(t, 1, actionCounts["notify"])
}

func TestPolicyManagement(t *testing.T) {
	logger := zerolog.Nop()
	engine := NewPolicyEngine(logger)

	// Test adding a policy
	policy := SecurityPolicy{
		ID:          "test-policy",
		Name:        "Test Policy",
		Description: "A test policy",
		Enabled:     true,
		Severity:    PolicySeverityHigh,
		Category:    PolicyCategoryVulnerability,
		Rules: []PolicyRule{
			{
				ID:          "test-rule",
				Type:        RuleTypeVulnerabilityCount,
				Field:       "total",
				Operator:    OperatorGreaterThan,
				Value:       float64(10),
				Description: "Test rule",
			},
		},
		Actions: []PolicyAction{
			{
				Type:        ActionTypeWarn,
				Description: "Test action",
			},
		},
	}

	err := engine.AddPolicy(policy)
	assert.NoError(t, err)

	// Test retrieving the policy
	retrieved, err := engine.GetPolicyByID("test-policy")
	assert.NoError(t, err)
	assert.Equal(t, policy.ID, retrieved.ID)
	assert.Equal(t, policy.Name, retrieved.Name)

	// Test updating the policy
	policy.Name = "Updated Test Policy"
	err = engine.UpdatePolicy(policy)
	assert.NoError(t, err)

	updated, err := engine.GetPolicyByID("test-policy")
	assert.NoError(t, err)
	assert.Equal(t, "Updated Test Policy", updated.Name)

	// Test removing the policy
	err = engine.RemovePolicy("test-policy")
	assert.NoError(t, err)

	_, err = engine.GetPolicyByID("test-policy")
	assert.Error(t, err)
}

func TestCompareValues(t *testing.T) {
	logger := zerolog.Nop()
	engine := NewPolicyEngine(logger)

	tests := []struct {
		name     string
		actual   interface{}
		operator RuleOperator
		expected interface{}
		result   bool
	}{
		{"equals int", 5, OperatorEquals, 5, true},
		{"equals string", "test", OperatorEquals, "test", true},
		{"not equals", 5, OperatorNotEquals, 3, true},
		{"greater than", 10, OperatorGreaterThan, 5, true},
		{"greater than false", 3, OperatorGreaterThan, 5, false},
		{"less than", 3, OperatorLessThan, 5, true},
		{"contains", "hello world", OperatorContains, "world", true},
		{"not contains", "hello world", OperatorNotContains, "xyz", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.compareValues(tt.actual, tt.operator, tt.expected)
			assert.Equal(t, tt.result, result)
		})
	}
}
