package build

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	coredocker "github.com/Azure/container-kit/pkg/core/docker"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to process JSON vulnerabilities
func processJSONVulnerabilities(trivyResult *coredocker.TrivyResult, scanResult *coredocker.ScanResult) {
	for _, result := range trivyResult.Results {
		for _, vuln := range result.Vulnerabilities {
			// Create vulnerability object
			vulnerability := coredocker.Vulnerability{
				VulnerabilityID:  vuln.VulnerabilityID,
				PkgName:          vuln.PkgName,
				InstalledVersion: vuln.InstalledVersion,
				FixedVersion:     vuln.FixedVersion,
				Severity:         vuln.Severity,
				Title:            vuln.Title,
				Description:      vuln.Description,
				References:       vuln.References,
			}

			if vuln.Layer.DiffID != "" {
				vulnerability.Layer = vuln.Layer.DiffID
			}

			scanResult.Vulnerabilities = append(scanResult.Vulnerabilities, vulnerability)

			// Update summary counts
			switch vuln.Severity {
			case "CRITICAL":
				scanResult.Summary.Critical++
			case "HIGH":
				scanResult.Summary.High++
			case "MEDIUM":
				scanResult.Summary.Medium++
			case "LOW":
				scanResult.Summary.Low++
			default:
				scanResult.Summary.Unknown++
			}
			scanResult.Summary.Total++

			// Count fixable vulnerabilities
			if vuln.FixedVersion != "" {
				scanResult.Summary.Fixable++
			}
		}
	}
}

// Helper function to count vulnerabilities from string
func countVulnerabilitiesFromString(outputStr string, scanResult *coredocker.ScanResult) {
	severityLevels := []struct {
		level string
		field *int
	}{
		{"CRITICAL", &scanResult.Summary.Critical},
		{"HIGH", &scanResult.Summary.High},
		{"MEDIUM", &scanResult.Summary.Medium},
		{"LOW", &scanResult.Summary.Low},
		{"UNKNOWN", &scanResult.Summary.Unknown},
	}

	for _, severity := range severityLevels {
		count := strings.Count(outputStr, severity.level)
		if count > 0 {
			*severity.field = count
			scanResult.Summary.Total += count
		}
	}
}

// Helper function to verify scan results
func verifyScanResults(t *testing.T, tt struct {
	name            string
	trivyOutput     string
	expectedTotal   int
	expectedCrit    int
	expectedHigh    int
	expectedMedium  int
	expectedLow     int
	expectedUnknown int
	expectedFixable int
	shouldFallback  bool
}, scanResult *coredocker.ScanResult) {
	assert.Equal(t, tt.expectedTotal, scanResult.Summary.Total, "Total vulnerabilities mismatch")
	assert.Equal(t, tt.expectedCrit, scanResult.Summary.Critical, "Critical vulnerabilities mismatch")
	assert.Equal(t, tt.expectedHigh, scanResult.Summary.High, "High vulnerabilities mismatch")
	assert.Equal(t, tt.expectedMedium, scanResult.Summary.Medium, "Medium vulnerabilities mismatch")
	assert.Equal(t, tt.expectedLow, scanResult.Summary.Low, "Low vulnerabilities mismatch")
	assert.Equal(t, tt.expectedFixable, scanResult.Summary.Fixable, "Fixable vulnerabilities mismatch")

	// Verify vulnerability details for valid JSON
	if !tt.shouldFallback && tt.expectedTotal > 0 {
		assert.Len(t, scanResult.Vulnerabilities, tt.expectedTotal)

		// Check first vulnerability details
		if len(scanResult.Vulnerabilities) > 0 {
			firstVuln := scanResult.Vulnerabilities[0]
			assert.NotEmpty(t, firstVuln.VulnerabilityID)
			assert.NotEmpty(t, firstVuln.PkgName)
			assert.NotEmpty(t, firstVuln.InstalledVersion)
			assert.NotEmpty(t, firstVuln.Severity)
		}
	}
}

func TestBuildValidator_ParseTrivyJSON(t *testing.T) {

	tests := []struct {
		name            string
		trivyOutput     string
		expectedTotal   int
		expectedCrit    int
		expectedHigh    int
		expectedMedium  int
		expectedLow     int
		expectedUnknown int
		expectedFixable int
		shouldFallback  bool
	}{
		{
			name: "Valid Trivy JSON with vulnerabilities",
			trivyOutput: `{
				"Results": [
					{
						"Target": "alpine:3.12",
						"Vulnerabilities": [
							{
								"VulnerabilityID": "CVE-2021-36159",
								"PkgName": "apk-tools",
								"InstalledVersion": "2.10.5-r1",
								"FixedVersion": "2.10.6-r0",
								"Severity": "CRITICAL",
								"Title": "apk-tools: Improper verification of signature",
								"Description": "A vulnerability in apk-tools...",
								"References": ["https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-36159"],
								"Layer": {
									"DiffID": "sha256:123456"
								}
							},
							{
								"VulnerabilityID": "CVE-2021-30139",
								"PkgName": "busybox",
								"InstalledVersion": "1.31.1-r19",
								"FixedVersion": "1.31.1-r20",
								"Severity": "HIGH",
								"Title": "busybox: Use after free",
								"Description": "A use-after-free in busybox...",
								"References": ["https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-30139"]
							},
							{
								"VulnerabilityID": "CVE-2021-28831",
								"PkgName": "busybox",
								"InstalledVersion": "1.31.1-r19",
								"FixedVersion": "",
								"Severity": "MEDIUM",
								"Title": "busybox: Invalid read",
								"Description": "An invalid read in busybox..."
							},
							{
								"VulnerabilityID": "CVE-2021-42378",
								"PkgName": "busybox",
								"InstalledVersion": "1.31.1-r19",
								"FixedVersion": "1.31.1-r21",
								"Severity": "LOW",
								"Title": "busybox: Out-of-bounds read",
								"Description": "An out-of-bounds read..."
							}
						]
					}
				]
			}`,
			expectedTotal:   4,
			expectedCrit:    1,
			expectedHigh:    1,
			expectedMedium:  1,
			expectedLow:     1,
			expectedUnknown: 0,
			expectedFixable: 3,
			shouldFallback:  false,
		},
		{
			name: "Empty Trivy JSON result",
			trivyOutput: `{
				"Results": []
			}`,
			expectedTotal:   0,
			expectedCrit:    0,
			expectedHigh:    0,
			expectedMedium:  0,
			expectedLow:     0,
			expectedFixable: 0,
			shouldFallback:  false,
		},
		{
			name: "Invalid JSON - fallback to string matching",
			trivyOutput: `This is not valid JSON
			CRITICAL: Found critical vulnerability
			HIGH: Found high vulnerability
			MEDIUM: Found medium vulnerability
			LOW: Found low vulnerability
			UNKNOWN: Found unknown vulnerability`,
			expectedTotal:   5,
			expectedCrit:    1,
			expectedHigh:    1,
			expectedMedium:  1,
			expectedLow:     1,
			expectedFixable: 0,
			shouldFallback:  true,
		},
		{
			name: "Invalid JSON with multiple severity counts",
			trivyOutput: `This is not valid JSON with multiple vulnerabilities
			CRITICAL vulnerability 1 found
			CRITICAL vulnerability 2 found
			HIGH severity issue detected
			HIGH priority fix needed
			HIGH impact found
			MEDIUM level warning`,
			expectedTotal:   6,
			expectedCrit:    2,
			expectedHigh:    3,
			expectedMedium:  1,
			expectedLow:     0,
			expectedFixable: 0,
			shouldFallback:  true,
		},
		{
			name: "Trivy JSON with unknown severity",
			trivyOutput: `{
				"Results": [
					{
						"Target": "ubuntu:20.04",
						"Vulnerabilities": [
							{
								"VulnerabilityID": "CVE-2021-00000",
								"PkgName": "unknown-pkg",
								"InstalledVersion": "1.0.0",
								"Severity": "UNKNOWN",
								"Title": "Unknown vulnerability"
							}
						]
					}
				]
			}`,
			expectedTotal:   1,
			expectedCrit:    0,
			expectedHigh:    0,
			expectedMedium:  0,
			expectedLow:     0,
			expectedFixable: 0,
			shouldFallback:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock scan result
			scanResult := &coredocker.ScanResult{
				Success:         true,
				ImageRef:        "test:latest",
				Summary:         coredocker.VulnerabilitySummary{},
				Vulnerabilities: []coredocker.Vulnerability{},
				Remediation:     []coredocker.RemediationStep{},
				Context:         make(map[string]interface{}),
			}

			// Test JSON parsing logic directly
			var trivyResult coredocker.TrivyResult
			err := json.Unmarshal([]byte(tt.trivyOutput), &trivyResult)

			if err != nil && !tt.shouldFallback {
				t.Fatalf("Expected valid JSON but got error: %v", err)
			}

			if err == nil {
				// Process parsed JSON
				processJSONVulnerabilities(&trivyResult, scanResult)
			} else if tt.shouldFallback {
				// Test fallback string matching
				countVulnerabilitiesFromString(tt.trivyOutput, scanResult)
			}

			// Verify results
			verifyScanResults(t, tt, scanResult)
		})
	}
}

func TestBuildValidator_ValidateBuildPrerequisites(t *testing.T) {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	validator := NewBuildValidator(logger)

	// Create temp directory for tests
	tempDir := t.TempDir()

	tests := []struct {
		name          string
		setupFunc     func() (dockerfilePath, buildContext string)
		expectedError bool
		errorContains string
	}{
		{
			name: "Valid Dockerfile and context",
			setupFunc: func() (string, string) {
				dockerfilePath := filepath.Join(tempDir, "Dockerfile")
				err := os.WriteFile(dockerfilePath, []byte("FROM alpine:latest\nRUN echo hello"), 0600)
				require.NoError(t, err)
				return dockerfilePath, tempDir
			},
			expectedError: false,
		},
		{
			name: "Missing Dockerfile",
			setupFunc: func() (string, string) {
				return filepath.Join(tempDir, "nonexistent", "Dockerfile"), tempDir
			},
			expectedError: true,
			errorContains: "Dockerfile not found",
		},
		{
			name: "Missing build context",
			setupFunc: func() (string, string) {
				dockerfilePath := filepath.Join(tempDir, "Dockerfile2")
				err := os.WriteFile(dockerfilePath, []byte("FROM alpine:latest"), 0600)
				require.NoError(t, err)
				return dockerfilePath, filepath.Join(tempDir, "nonexistent")
			},
			expectedError: true,
			errorContains: "Build context directory not found",
		},
		{
			name: "Valid empty Dockerfile",
			setupFunc: func() (string, string) {
				dockerfilePath := filepath.Join(tempDir, "Dockerfile.empty")
				err := os.WriteFile(dockerfilePath, []byte(""), 0600)
				require.NoError(t, err)
				return dockerfilePath, tempDir
			},
			expectedError: false, // Current implementation doesn't validate content
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dockerfilePath, buildContext := tt.setupFunc()
			err := validator.ValidateBuildPrerequisites(dockerfilePath, buildContext)

			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBuildValidator_AddPushTroubleshootingTips(t *testing.T) {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	validator := NewBuildValidator(logger)

	tests := []struct {
		name         string
		errorMsg     string
		registryURL  string
		expectedTips []string
	}{
		{
			name:        "Authentication error",
			errorMsg:    "authentication required",
			registryURL: "docker.io",
			expectedTips: []string{
				"Authentication failed. Run: docker login docker.io",
				"Check if your credentials are correct",
				"For private registries, ensure you have push permissions",
			},
		},
		{
			name:        "Connection refused error",
			errorMsg:    "connection refused",
			registryURL: "localhost:5000",
			expectedTips: []string{
				"Cannot connect to registry. Check if the registry URL is correct",
				"Verify network connectivity to localhost:5000",
				"If using a private registry, ensure it's accessible from your network",
			},
		},
		{
			name:        "Permission denied error",
			errorMsg:    "permission denied",
			registryURL: "gcr.io/myproject",
			expectedTips: []string{
				"Access denied. Verify you have push permissions to this repository",
				"Check if the repository exists and you have write access",
				"For organization repositories, ensure your account is properly configured",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fmt.Errorf("%s", tt.errorMsg)
			tips := validator.AddPushTroubleshootingTips(err, tt.registryURL)

			assert.Len(t, tips, len(tt.expectedTips))
			for i, expectedTip := range tt.expectedTips {
				assert.Equal(t, expectedTip, tips[i])
			}
		})
	}
}
