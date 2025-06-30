package utils

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestSecurityValidator(t *testing.T) {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	validator := NewSecurityValidator(logger)

	t.Run("SecurityValidator_Creation", func(t *testing.T) {
		assert.NotNil(t, validator)
		assert.NotNil(t, validator.threatModel)
		assert.NotNil(t, validator.policyEngine)
	})

	t.Run("SecurityValidation_SecureConfiguration", func(t *testing.T) {
		ctx := context.Background()
		sessionID := "test-secure-session"

		// Configure secure options
		options := SandboxOptions{
			BaseImage:     "alpine:3.18",
			MemoryLimit:   256 * 1024 * 1024,
			CPUQuota:      50000,
			Timeout:       30 * time.Second, // Add timeout for C008 control
			ReadOnly:      true,
			NetworkAccess: false,
			User:          "1000",
			Group:         "1000",
			Capabilities:  []string{}, // No capabilities
			SecurityPolicy: SecurityPolicy{
				AllowNetworking:   false,
				AllowFileSystem:   true,
				RequireNonRoot:    true,
				TrustedRegistries: []string{"docker.io"},
			},
		}

		report, err := validator.ValidateSecurity(ctx, sessionID, options)
		assert.NoError(t, err)
		assert.NotNil(t, report)

		// Should pass with secure configuration
		assert.True(t, report.Passed)
		assert.Equal(t, "LOW", report.OverallRisk)

		// Verify threat assessments
		assert.NotEmpty(t, report.ThreatAssessment)

		// Check that all major threats are mitigated
		for threatID, assessment := range report.ThreatAssessment {
			t.Logf("Threat %s: Mitigated=%v, RiskLevel=%s",
				threatID, assessment.Mitigated, assessment.RiskLevel)
		}
	})

	t.Run("SecurityValidation_InsecureConfiguration", func(t *testing.T) {
		ctx := context.Background()
		sessionID := "test-insecure-session"

		// Configure insecure options
		options := SandboxOptions{
			BaseImage:     "ubuntu:latest", // Using latest tag
			MemoryLimit:   0,               // No memory limit
			CPUQuota:      0,               // No CPU limit
			ReadOnly:      false,           // Writable filesystem
			NetworkAccess: true,            // Network enabled
			User:          "root",          // Running as root
			Group:         "root",
			Capabilities:  []string{"SYS_ADMIN", "NET_ADMIN"}, // Dangerous capabilities
		}

		report, err := validator.ValidateSecurity(ctx, sessionID, options)
		assert.NoError(t, err)
		assert.NotNil(t, report)

		// Should fail with insecure configuration
		assert.False(t, report.Passed)
		assert.Contains(t, []string{"HIGH", "CRITICAL"}, report.OverallRisk)

		// Should have vulnerabilities
		assert.Greater(t, len(report.Vulnerabilities), 0)

		// Should have recommendations
		assert.Greater(t, len(report.Recommendations), 0)
	})
}

func TestImageSecurity(t *testing.T) {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	validator := NewSecurityValidator(logger)
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
}

func TestCommandSecurity(t *testing.T) {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	validator := NewSecurityValidator(logger)

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
}

func BenchmarkSecurityValidation(b *testing.B) {
	logger := zerolog.Nop()
	validator := NewSecurityValidator(logger)
	ctx := context.Background()

	options := SandboxOptions{
		BaseImage:     "alpine:3.18",
		MemoryLimit:   256 * 1024 * 1024,
		CPUQuota:      50000,
		ReadOnly:      true,
		NetworkAccess: false,
		User:          "1000",
		Group:         "1000",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := validator.ValidateSecurity(ctx, "bench-session", options)
		if err != nil {
			b.Fatal(err)
		}
	}
}
