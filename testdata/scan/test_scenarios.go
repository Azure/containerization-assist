package testdata

import (
	"path/filepath"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/containerization/scan"
)

// TestScenario represents a test scenario for security scanning
type TestScenario struct {
	Name                string                 `json:"name"`
	Description         string                 `json:"description"`
	ImageName           string                 `json:"image_name"`
	ScanType            string                 `json:"scan_type"`
	Scanner             string                 `json:"scanner"`
	ExpectedVulnCount   int                    `json:"expected_vuln_count"`
	ExpectedSecretCount int                    `json:"expected_secret_count"`
	ExpectedError       bool                   `json:"expected_error"`
	ExpectedSeverities  []string               `json:"expected_severities"`
	TestDataFile        string                 `json:"test_data_file"`
	MockError           string                 `json:"mock_error,omitempty"`
	Tags                []string               `json:"tags"`
	Timeout             time.Duration          `json:"timeout"`
	Context             map[string]interface{} `json:"context"`
}

// SecurityScanTestScenarios defines all security scanning test scenarios
var SecurityScanTestScenarios = []TestScenario{
	{
		Name:                "successful_scan_with_high_severity_vulnerabilities",
		Description:         "Scan returns multiple vulnerabilities including high severity ones",
		ImageName:           "nginx:latest",
		ScanType:            "comprehensive",
		Scanner:             "trivy",
		ExpectedVulnCount:   2,
		ExpectedSecretCount: 0,
		ExpectedError:       false,
		ExpectedSeverities:  []string{"HIGH", "MEDIUM"},
		TestDataFile:        "testdata/scan/trivy/response_with_vulns.json",
		Tags:                []string{"security", "vulnerabilities", "trivy"},
		Timeout:             30 * time.Second,
		Context: map[string]interface{}{
			"test_type":         "unit",
			"expected_cves":     []string{"CVE-2023-1234", "CVE-2023-5678"},
			"expected_packages": []string{"libssl1.1", "nginx"},
		},
	},
	{
		Name:                "successful_scan_clean_image",
		Description:         "Scan returns no vulnerabilities for clean image",
		ImageName:           "alpine:latest",
		ScanType:            "basic",
		Scanner:             "trivy",
		ExpectedVulnCount:   0,
		ExpectedSecretCount: 0,
		ExpectedError:       false,
		ExpectedSeverities:  []string{},
		TestDataFile:        "testdata/scan/trivy/response_clean.json",
		Tags:                []string{"security", "clean", "trivy"},
		Timeout:             30 * time.Second,
		Context: map[string]interface{}{
			"test_type":        "unit",
			"expected_message": "No vulnerabilities found",
		},
	},
	{
		Name:                "scanner_timeout_error",
		Description:         "Scanner times out during scan operation",
		ImageName:           "timeout-image:latest",
		ScanType:            "comprehensive",
		Scanner:             "trivy",
		ExpectedVulnCount:   0,
		ExpectedSecretCount: 0,
		ExpectedError:       true,
		ExpectedSeverities:  []string{},
		TestDataFile:        "",
		MockError:           "scanner timeout: operation took too long",
		Tags:                []string{"error", "timeout", "trivy"},
		Timeout:             5 * time.Second,
		Context: map[string]interface{}{
			"test_type":  "error",
			"error_type": "timeout",
		},
	},
	{
		Name:                "scanner_not_found_error",
		Description:         "Scanner executable not found on system",
		ImageName:           "test-image:latest",
		ScanType:            "comprehensive",
		Scanner:             "trivy",
		ExpectedVulnCount:   0,
		ExpectedSecretCount: 0,
		ExpectedError:       true,
		ExpectedSeverities:  []string{},
		TestDataFile:        "",
		MockError:           "trivy executable not found",
		Tags:                []string{"error", "not_found", "trivy"},
		Timeout:             30 * time.Second,
		Context: map[string]interface{}{
			"test_type":  "error",
			"error_type": "not_found",
		},
	},
	{
		Name:                "invalid_image_name",
		Description:         "Scan with invalid image name format",
		ImageName:           "invalid-image-name",
		ScanType:            "basic",
		Scanner:             "trivy",
		ExpectedVulnCount:   0,
		ExpectedSecretCount: 0,
		ExpectedError:       true,
		ExpectedSeverities:  []string{},
		TestDataFile:        "",
		MockError:           "invalid image name format",
		Tags:                []string{"error", "validation", "trivy"},
		Timeout:             30 * time.Second,
		Context: map[string]interface{}{
			"test_type":  "validation",
			"error_type": "invalid_input",
		},
	},
	{
		Name:                "grype_scanner_with_vulnerabilities",
		Description:         "Grype scanner finds vulnerabilities in image",
		ImageName:           "ubuntu:18.04",
		ScanType:            "comprehensive",
		Scanner:             "grype",
		ExpectedVulnCount:   1,
		ExpectedSecretCount: 0,
		ExpectedError:       false,
		ExpectedSeverities:  []string{"CRITICAL"},
		TestDataFile:        "testdata/scan/grype/response_with_vulns.json",
		Tags:                []string{"security", "vulnerabilities", "grype"},
		Timeout:             30 * time.Second,
		Context: map[string]interface{}{
			"test_type":    "unit",
			"scanner_type": "grype",
		},
	},
	{
		Name:                "large_image_scan_performance",
		Description:         "Performance test for scanning large images",
		ImageName:           "large-image:latest",
		ScanType:            "comprehensive",
		Scanner:             "trivy",
		ExpectedVulnCount:   100,
		ExpectedSecretCount: 0,
		ExpectedError:       false,
		ExpectedSeverities:  []string{"HIGH", "MEDIUM", "LOW"},
		TestDataFile:        "testdata/scan/trivy/response_large.json",
		Tags:                []string{"performance", "large", "trivy"},
		Timeout:             60 * time.Second,
		Context: map[string]interface{}{
			"test_type":             "performance",
			"expected_max_duration": "45s",
		},
	},
}

// SecretScanTestScenarios defines all secret scanning test scenarios
var SecretScanTestScenarios = []TestScenario{
	{
		Name:                "file_with_multiple_secrets",
		Description:         "File contains multiple types of secrets",
		ImageName:           "testdata/scan/secrets/test_secrets.txt",
		ScanType:            "secrets",
		Scanner:             "internal",
		ExpectedVulnCount:   0,
		ExpectedSecretCount: 6,
		ExpectedError:       false,
		ExpectedSeverities:  []string{"HIGH", "MEDIUM"},
		TestDataFile:        "testdata/scan/secrets/test_secrets.txt",
		Tags:                []string{"secrets", "multiple", "internal"},
		Timeout:             10 * time.Second,
		Context: map[string]interface{}{
			"test_type":      "unit",
			"expected_types": []string{"aws_access_key", "github_token", "api_key", "private_key"},
		},
	},
	{
		Name:                "clean_file_no_secrets",
		Description:         "File contains no secrets",
		ImageName:           "testdata/scan/secrets/clean_file.txt",
		ScanType:            "secrets",
		Scanner:             "internal",
		ExpectedVulnCount:   0,
		ExpectedSecretCount: 0,
		ExpectedError:       false,
		ExpectedSeverities:  []string{},
		TestDataFile:        "testdata/scan/secrets/clean_file.txt",
		Tags:                []string{"secrets", "clean", "internal"},
		Timeout:             10 * time.Second,
		Context: map[string]interface{}{
			"test_type":        "unit",
			"expected_message": "No secrets found",
		},
	},
	{
		Name:                "file_not_found_error",
		Description:         "Secret scan on non-existent file",
		ImageName:           "testdata/scan/secrets/nonexistent.txt",
		ScanType:            "secrets",
		Scanner:             "internal",
		ExpectedVulnCount:   0,
		ExpectedSecretCount: 0,
		ExpectedError:       true,
		ExpectedSeverities:  []string{},
		TestDataFile:        "",
		MockError:           "file not found",
		Tags:                []string{"error", "file_not_found", "internal"},
		Timeout:             10 * time.Second,
		Context: map[string]interface{}{
			"test_type":  "error",
			"error_type": "file_not_found",
		},
	},
}

// GetTestScenarioByName returns a test scenario by name
func GetTestScenarioByName(name string) *TestScenario {
	for _, scenario := range SecurityScanTestScenarios {
		if scenario.Name == name {
			return &scenario
		}
	}
	for _, scenario := range SecretScanTestScenarios {
		if scenario.Name == name {
			return &scenario
		}
	}
	return nil
}

// GetTestScenariosByTag returns test scenarios filtered by tag
func GetTestScenariosByTag(tag string) []TestScenario {
	var scenarios []TestScenario

	for _, scenario := range SecurityScanTestScenarios {
		for _, t := range scenario.Tags {
			if t == tag {
				scenarios = append(scenarios, scenario)
				break
			}
		}
	}

	for _, scenario := range SecretScanTestScenarios {
		for _, t := range scenario.Tags {
			if t == tag {
				scenarios = append(scenarios, scenario)
				break
			}
		}
	}

	return scenarios
}

// TestDataLoader provides utilities for loading test data
type TestDataLoader struct {
	BaseDir string
}

// NewTestDataLoader creates a new test data loader
func NewTestDataLoader(baseDir string) *TestDataLoader {
	return &TestDataLoader{
		BaseDir: baseDir,
	}
}

// LoadTestData loads test data from file
func (l *TestDataLoader) LoadTestData(filename string) ([]byte, error) {
	fullPath := filepath.Join(l.BaseDir, filename)
	// Implementation would read file and return bytes
	return nil, nil
}

// LoadSecretMatches loads expected secret matches for a test scenario
func (l *TestDataLoader) LoadSecretMatches(scenario *TestScenario) ([]scan.SecretMatch, error) {
	// Default expected secret matches for test scenarios
	switch scenario.Name {
	case "file_with_multiple_secrets":
		return []scan.SecretMatch{
			{
				Type:        "aws_access_key",
				Value:       "AKIAIOSFODNN7EXAMPLE",
				Line:        4,
				Column:      20,
				File:        scenario.TestDataFile,
				Severity:    "HIGH",
				Description: "AWS Access Key ID detected",
			},
			{
				Type:        "aws_secret_key",
				Value:       "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				Line:        5,
				Column:      25,
				File:        scenario.TestDataFile,
				Severity:    "HIGH",
				Description: "AWS Secret Access Key detected",
			},
			{
				Type:        "github_token",
				Value:       "ghp_1234567890abcdef1234567890abcdef12345678",
				Line:        8,
				Column:      16,
				File:        scenario.TestDataFile,
				Severity:    "HIGH",
				Description: "GitHub Token detected",
			},
			{
				Type:        "api_key",
				Value:       "sk-1234567890abcdef1234567890abcdef12345678",
				Line:        14,
				Column:      11,
				File:        scenario.TestDataFile,
				Severity:    "MEDIUM",
				Description: "API Key detected",
			},
			{
				Type:        "stripe_key",
				Value:       "sk_test_1234567890abcdef1234567890abcdef12345678",
				Line:        15,
				Column:      14,
				File:        scenario.TestDataFile,
				Severity:    "MEDIUM",
				Description: "Stripe API Key detected",
			},
			{
				Type:        "private_key",
				Value:       "-----BEGIN RSA PRIVATE KEY-----",
				Line:        18,
				Column:      1,
				File:        scenario.TestDataFile,
				Severity:    "HIGH",
				Description: "Private Key detected",
			},
		}, nil
	case "clean_file_no_secrets":
		return []scan.SecretMatch{}, nil
	default:
		return []scan.SecretMatch{}, nil
	}
}
