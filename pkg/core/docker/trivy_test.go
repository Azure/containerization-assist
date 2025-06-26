package docker

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestTrivyScanner_FormatScanSummary(t *testing.T) {
	tests := []struct {
		name          string
		result        *ScanResult
		expectedParts []string
		notExpected   []string
	}{
		{
			name: "successful scan with no vulnerabilities",
			result: &ScanResult{
				Success:  true,
				ImageRef: "myapp:latest",
				Duration: 5 * time.Second,
				Summary: VulnerabilitySummary{
					Total:    0,
					Critical: 0,
					High:     0,
					Medium:   0,
					Low:      0,
					Fixable:  0,
				},
				Remediation: []RemediationStep{
					{
						Priority:    1,
						Action:      "No action required",
						Description: "No vulnerabilities found in the image",
					},
				},
			},
			expectedParts: []string{
				"Security Scan Results for myapp:latest:",
				"Scan completed in 5s",
				"CRITICAL: 0",
				"HIGH:     0",
				"MEDIUM:   0",
				"LOW:      0",
				"TOTAL:    0 (Fixable: 0)",
				"✅ Image passed security requirements",
				"No action required",
			},
			notExpected: []string{
				"❌",
			},
		},
		{
			name: "failed scan with critical vulnerabilities",
			result: &ScanResult{
				Success:  false,
				ImageRef: "vulnerable:v1.0",
				Duration: 3500 * time.Millisecond,
				Summary: VulnerabilitySummary{
					Total:    15,
					Critical: 3,
					High:     5,
					Medium:   4,
					Low:      3,
					Fixable:  12,
				},
				Remediation: []RemediationStep{
					{
						Priority:    1,
						Action:      "Fix critical vulnerabilities",
						Description: "Found 3 CRITICAL and 5 HIGH severity vulnerabilities that must be fixed",
					},
					{
						Priority:    2,
						Action:      "Update packages",
						Description: "12 vulnerabilities have fixes available. Update packages in your Dockerfile",
						Command:     "RUN apt-get update && apt-get upgrade -y && rm -rf /var/lib/apt/lists/*",
					},
				},
			},
			expectedParts: []string{
				"Security Scan Results for vulnerable:v1.0:",
				"Scan completed in 3.5s",
				"CRITICAL: 3",
				"HIGH:     5",
				"MEDIUM:   4",
				"LOW:      3",
				"TOTAL:    15 (Fixable: 12)",
				"❌ Image has 3 CRITICAL and 5 HIGH severity vulnerabilities",
				"Fix critical vulnerabilities",
				"Update packages",
				"Command: RUN apt-get update",
			},
			notExpected: []string{
				"✅",
			},
		},
		{
			name: "scan with only medium and low vulnerabilities",
			result: &ScanResult{
				Success:  true,
				ImageRef: "myapp:v2.0",
				Duration: 2 * time.Second,
				Summary: VulnerabilitySummary{
					Total:    5,
					Critical: 0,
					High:     0,
					Medium:   3,
					Low:      2,
					Fixable:  4,
				},
				Remediation: []RemediationStep{
					{
						Priority:    1,
						Action:      "Update packages",
						Description: "4 vulnerabilities have fixes available. Update packages in your Dockerfile",
						Command:     "RUN apt-get update && apt-get upgrade -y && rm -rf /var/lib/apt/lists/*",
					},
					{
						Priority:    2,
						Action:      "Regular scanning",
						Description: "Scan images regularly as new vulnerabilities are discovered daily",
					},
				},
			},
			expectedParts: []string{
				"CRITICAL: 0",
				"HIGH:     0",
				"MEDIUM:   3",
				"LOW:      2",
				"TOTAL:    5 (Fixable: 4)",
				"✅ Image passed security requirements",
				"Update packages",
				"Regular scanning",
			},
		},
		{
			name: "scan with unknown severity",
			result: &ScanResult{
				Success:  true,
				ImageRef: "test:unknown",
				Duration: 1 * time.Second,
				Summary: VulnerabilitySummary{
					Total:   3,
					Unknown: 3,
					Fixable: 0,
				},
			},
			expectedParts: []string{
				"TOTAL:    3 (Fixable: 0)",
				"✅ Image passed security requirements",
			},
		},
	}

	ts := &TrivyScanner{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := ts.FormatScanSummary(tt.result)

			// Check expected parts
			for _, expected := range tt.expectedParts {
				assert.Contains(t, output, expected, "Expected string not found: %s", expected)
			}

			// Check not expected parts
			for _, notExpected := range tt.notExpected {
				assert.NotContains(t, output, notExpected, "Unexpected string found: %s", notExpected)
			}
		})
	}
}

func TestTrivyScanner_generateRemediationSteps(t *testing.T) {
	tests := []struct {
		name               string
		result             *ScanResult
		expectedStepCount  int
		expectedActions    []string
		expectedPriorities []int
	}{
		{
			name: "no vulnerabilities",
			result: &ScanResult{
				Summary: VulnerabilitySummary{
					Total: 0,
				},
			},
			expectedStepCount:  1,
			expectedActions:    []string{"No action required"},
			expectedPriorities: []int{1},
		},
		{
			name: "critical and high vulnerabilities",
			result: &ScanResult{
				Summary: VulnerabilitySummary{
					Total:    10,
					Critical: 2,
					High:     3,
					Fixable:  8,
				},
				Vulnerabilities: []Vulnerability{
					{Severity: "CRITICAL", PkgName: "openssl"},
					{Severity: "HIGH", PkgName: "base-files"},
				},
			},
			expectedStepCount: 3,
			expectedActions: []string{
				"Fix critical vulnerabilities",
				"Update packages",
				"Regular scanning",
			},
		},
		{
			name: "base image vulnerabilities",
			result: &ScanResult{
				Summary: VulnerabilitySummary{
					Total:    5,
					Critical: 1,
					High:     2,
					Fixable:  3,
				},
				Vulnerabilities: []Vulnerability{
					{Severity: "CRITICAL", PkgName: "base-image"},
					{Severity: "HIGH", PkgName: "base-libs"},
				},
			},
			expectedStepCount: 4,
			expectedActions: []string{
				"Fix critical vulnerabilities",
				"Update base image",
				"Update packages",
				"Regular scanning",
			},
		},
		{
			name: "only fixable vulnerabilities",
			result: &ScanResult{
				Summary: VulnerabilitySummary{
					Total:   6,
					Medium:  4,
					Low:     2,
					Fixable: 6,
				},
			},
			expectedStepCount: 2,
			expectedActions: []string{
				"Update packages",
				"Regular scanning",
			},
		},
		{
			name: "no fixable vulnerabilities",
			result: &ScanResult{
				Summary: VulnerabilitySummary{
					Total:   3,
					Low:     3,
					Fixable: 0,
				},
			},
			expectedStepCount: 1,
			expectedActions: []string{
				"Regular scanning",
			},
		},
	}

	ts := &TrivyScanner{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts.generateRemediationSteps(tt.result)

			assert.Len(t, tt.result.Remediation, tt.expectedStepCount)

			for i, expectedAction := range tt.expectedActions {
				if i < len(tt.result.Remediation) {
					assert.Equal(t, expectedAction, tt.result.Remediation[i].Action)
				}
			}

			// Check priorities are sequential
			for i, step := range tt.result.Remediation {
				assert.Equal(t, i+1, step.Priority)
			}
		})
	}
}

// TestNewTrivyScanner tests the constructor
func TestNewTrivyScanner(t *testing.T) {
	logger := zerolog.Nop()

	scanner := NewTrivyScanner(logger)

	assert.NotNil(t, scanner)
	assert.NotNil(t, scanner.logger)
}

// TestTrivyResult_Unmarshal tests JSON unmarshaling of Trivy results
func TestTrivyResult_Unmarshal(t *testing.T) {
	jsonData := `{
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
						"Title": "apk-tools: Improper verification",
						"Description": "A vulnerability in apk-tools",
						"References": ["https://cve.mitre.org"],
						"Layer": {
							"DiffID": "sha256:123456"
						}
					}
				]
			}
		]
	}`

	var result TrivyResult
	err := json.Unmarshal([]byte(jsonData), &result)

	assert.NoError(t, err)
	assert.Len(t, result.Results, 1)
	assert.Equal(t, "alpine:3.12", result.Results[0].Target)
	assert.Len(t, result.Results[0].Vulnerabilities, 1)

	vuln := result.Results[0].Vulnerabilities[0]
	assert.Equal(t, "CVE-2021-36159", vuln.VulnerabilityID)
	assert.Equal(t, "apk-tools", vuln.PkgName)
	assert.Equal(t, "CRITICAL", vuln.Severity)
	assert.Equal(t, "2.10.6-r0", vuln.FixedVersion)
}
