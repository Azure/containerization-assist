package security_test

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/containerization-assist/pkg/core/security"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPolicyEngine(t *testing.T) {
	logger := zerolog.Nop()
	engine := security.NewPolicyEngine(logger)

	assert.NotNil(t, engine)
	// Cannot access unexported fields - just verify creation works
}

func TestLoadDefaultPolicies(t *testing.T) {
	logger := zerolog.Nop()
	engine := security.NewPolicyEngine(logger)

	err := engine.LoadDefaultPolicies()
	require.NoError(t, err)

	policies := engine.GetPolicies()
	assert.Len(t, policies, 8) // Expected default policies

	// Check specific policies exist
	policyMap := make(map[string]security.Policy)
	for _, policy := range policies {
		policyMap[policy.ID] = policy
	}

	assert.Contains(t, policyMap, "critical-vulns-block")
	assert.Contains(t, policyMap, "high-vulns-warn")
	assert.Contains(t, policyMap, "cvss-score-limit")
	assert.Contains(t, policyMap, "no-secrets")
	assert.Contains(t, policyMap, "image-age-limit")
	assert.Contains(t, policyMap, "image-size-limit")
	assert.Contains(t, policyMap, "approved-licenses")
	assert.Contains(t, policyMap, "package-version-ban")
}

func TestEvaluatePolicies(t *testing.T) {
	logger := zerolog.Nop()
	engine := security.NewPolicyEngine(logger)

	// Load default policies
	err := engine.LoadDefaultPolicies()
	require.NoError(t, err)

	ctx := context.Background()
	scanCtx := &security.ScanContext{
		ImageRef: "test:latest",
		ScanTime: time.Now(),
		VulnSummary: security.VulnerabilitySummary{
			Total:    10,
			Critical: 2,
			High:     3,
			Medium:   3,
			Low:      2,
			Fixable:  8,
		},
		Vulnerabilities: []security.Vulnerability{
			{
				VulnerabilityID: "CVE-2023-1234",
				Severity:        "CRITICAL",
				CVSSV3: security.CVSSV3Info{
					Score: 9.0,
				},
			},
		},
		SecretSummary: &security.DiscoverySummary{
			TotalFindings:  1,
			FalsePositives: 0,
		},
		SecretFindings: []security.ExtendedSecretFinding{
			{
				SecretFinding: security.SecretFinding{
					Type:        "api_key",
					File:        "config.txt",
					Line:        10,
					Description: "API key found",
					Confidence:  0.95,
					RuleID:      "api-key-001",
				},
				ID:       "finding-001",
				Severity: "HIGH",
				Match:    "sk-1234567890",
				Verified: false,
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
	var criticalPolicy *security.PolicyEvaluationResult
	for i, result := range results {
		if result.PolicyID == "critical-vulns-block" {
			criticalPolicy = &results[i]
			break
		}
	}

	require.NotNil(t, criticalPolicy)
	assert.False(t, criticalPolicy.Passed)
	assert.NotEmpty(t, criticalPolicy.Violations)

	// Check that blocking action is present
	hasBlockingAction := false
	for _, action := range criticalPolicy.Actions {
		if action.Type == security.ActionTypeBlock {
			hasBlockingAction = true
			break
		}
	}
	assert.True(t, hasBlockingAction)
}

func TestShouldBlock(t *testing.T) {
	logger := zerolog.Nop()
	engine := security.NewPolicyEngine(logger)

	tests := []struct {
		name        string
		results     []security.PolicyEvaluationResult
		shouldBlock bool
	}{
		{
			name: "blocking action should block",
			results: []security.PolicyEvaluationResult{
				{
					PolicyID: "test-policy",
					Passed:   false,
					Actions: []security.PolicyAction{
						{Type: security.ActionTypeBlock},
					},
				},
			},
			shouldBlock: true,
		},
		{
			name: "only warning actions should not block",
			results: []security.PolicyEvaluationResult{
				{
					PolicyID: "test-policy",
					Passed:   false,
					Actions: []security.PolicyAction{
						{Type: security.ActionTypeWarn},
						{Type: security.ActionTypeLog},
					},
				},
			},
			shouldBlock: false,
		},
		{
			name: "passed policies should not block",
			results: []security.PolicyEvaluationResult{
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
	engine := security.NewPolicyEngine(logger)

	results := []security.PolicyEvaluationResult{
		{
			PolicyID: "policy1",
			Passed:   true,
		},
		{
			PolicyID: "policy2",
			Passed:   false,
			Violations: []security.PolicyViolation{
				{Severity: security.PolicySeverityCritical},
				{Severity: security.PolicySeverityHigh},
			},
			Actions: []security.PolicyAction{
				{Type: security.ActionTypeBlock},
				{Type: security.ActionTypeNotify},
			},
		},
		{
			PolicyID: "policy3",
			Passed:   false,
			Violations: []security.PolicyViolation{
				{Severity: security.PolicySeverityMedium},
			},
			Actions: []security.PolicyAction{
				{Type: security.ActionTypeWarn},
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
	engine := security.NewPolicyEngine(logger)

	// Test adding a policy
	policy := security.Policy{
		ID:          "test-policy",
		Name:        "Test Policy",
		Description: "A test policy",
		Enabled:     true,
		Severity:    security.PolicySeverityHigh,
		Category:    security.PolicyCategoryVulnerability,
		Rules: []security.PolicyRule{
			{
				ID:          "test-rule",
				Type:        security.RuleTypeVulnerabilityCount,
				Field:       "total",
				Operator:    security.OperatorGreaterThan,
				Value:       float64(10),
				Description: "Test rule",
			},
		},
		Actions: []security.PolicyAction{
			{
				Type:        security.ActionTypeWarn,
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

// Test that default policies can detect violations
func TestDefaultPoliciesDetectViolations(t *testing.T) {
	logger := zerolog.Nop()
	engine := security.NewPolicyEngine(logger)

	err := engine.LoadDefaultPolicies()
	require.NoError(t, err)

	ctx := context.Background()

	// Create a scan context with critical vulnerabilities
	scanCtx := &security.ScanContext{
		ImageRef: "vulnerable:latest",
		ScanTime: time.Now(),
		VulnSummary: security.VulnerabilitySummary{
			Total:    5,
			Critical: 3, // This should trigger the critical-vulns-block policy
			High:     2,
		},
	}

	results, err := engine.EvaluatePolicies(ctx, scanCtx)
	assert.NoError(t, err)

	// Find the critical vulnerabilities policy result
	var criticalResult *security.PolicyEvaluationResult
	for i, result := range results {
		if result.PolicyID == "critical-vulns-block" {
			criticalResult = &results[i]
			break
		}
	}

	require.NotNil(t, criticalResult)
	assert.False(t, criticalResult.Passed, "Critical vulnerabilities policy should fail")
	assert.NotEmpty(t, criticalResult.Violations, "Should have violations")

	// Check that it recommends blocking
	shouldBlock := engine.ShouldBlock(results)
	assert.True(t, shouldBlock, "Should recommend blocking due to critical vulnerabilities")
}

// Test policy evaluation with secrets
func TestPolicyEvaluationWithSecrets(t *testing.T) {
	logger := zerolog.Nop()
	engine := security.NewPolicyEngine(logger)

	err := engine.LoadDefaultPolicies()
	require.NoError(t, err)

	ctx := context.Background()

	// Create a scan context with secrets
	scanCtx := &security.ScanContext{
		ImageRef: "test:latest",
		ScanTime: time.Now(),
		VulnSummary: security.VulnerabilitySummary{
			Total: 0, // No vulnerabilities
		},
		SecretSummary: &security.DiscoverySummary{
			TotalFindings:  2,
			FalsePositives: 0,
		},
		SecretFindings: []security.ExtendedSecretFinding{
			{
				SecretFinding: security.SecretFinding{
					Type:        "aws_access_key",
					File:        "config.txt",
					Line:        20,
					Description: "AWS access key found",
					Confidence:  0.95,
					RuleID:      "aws-key-001",
				},
				ID:       "finding-002",
				Severity: "CRITICAL",
				Match:    "AKIAIOSFODNN7EXAMPLE",
				Verified: false,
			},
			{
				SecretFinding: security.SecretFinding{
					Type:        "private_key",
					File:        "id_rsa",
					Line:        1,
					Description: "Private key found",
					Confidence:  0.95,
					RuleID:      "private-key-001",
				},
				ID:       "finding-003",
				Severity: "CRITICAL",
				Match:    "-----BEGIN RSA PRIVATE KEY-----",
				Verified: false,
			},
		},
	}

	results, err := engine.EvaluatePolicies(ctx, scanCtx)
	assert.NoError(t, err)

	// Find the no-secrets policy result
	var secretsResult *security.PolicyEvaluationResult
	for i, result := range results {
		if result.PolicyID == "no-secrets" {
			secretsResult = &results[i]
			break
		}
	}

	require.NotNil(t, secretsResult)
	assert.False(t, secretsResult.Passed, "No-secrets policy should fail")
	assert.NotEmpty(t, secretsResult.Violations, "Should have violations for found secrets")
}
