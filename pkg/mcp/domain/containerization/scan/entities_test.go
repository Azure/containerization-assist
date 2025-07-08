package scan

import (
	"testing"
	"time"
)

func TestScanRequest_Validate(t *testing.T) {
	validRequest := &ScanRequest{
		SessionID: "test-session",
		Target: ScanTarget{
			Type:       TargetTypeImage,
			Identifier: "nginx:latest",
		},
		ScanType: ScanTypeVulnerability,
		Options: ScanOptions{
			Timeout: time.Hour,
		},
	}

	errors := validRequest.Validate()
	if len(errors) != 0 {
		t.Errorf("expected no validation errors, got %d: %v", len(errors), errors)
	}

	// Test invalid request
	invalidRequest := &ScanRequest{
		SessionID: "",
		Target: ScanTarget{
			Type:       "invalid",
			Identifier: "",
		},
		ScanType: "invalid",
		Options: ScanOptions{
			Timeout: -time.Hour,
		},
	}

	errors = invalidRequest.Validate()
	if len(errors) == 0 {
		t.Error("expected validation errors for invalid request")
	}
}

func TestScanResult_HasCriticalIssues(t *testing.T) {
	// Result with critical issues
	criticalResult := &ScanResult{
		Summary: ScanSummary{
			CriticalCount: 2,
		},
	}

	if !criticalResult.HasCriticalIssues() {
		t.Error("expected result to have critical issues")
	}

	// Result without critical issues
	safeResult := &ScanResult{
		Summary: ScanSummary{
			CriticalCount: 0,
		},
	}

	if safeResult.HasCriticalIssues() {
		t.Error("expected result to not have critical issues")
	}
}

func TestScanResult_GetCriticalVulnerabilities(t *testing.T) {
	result := &ScanResult{
		Vulnerabilities: []Vulnerability{
			{ID: "CVE-1", Severity: SeverityCritical},
			{ID: "CVE-2", Severity: SeverityHigh},
			{ID: "CVE-3", Severity: SeverityCritical},
		},
	}

	critical := result.GetCriticalVulnerabilities()
	if len(critical) != 2 {
		t.Errorf("expected 2 critical vulnerabilities, got %d", len(critical))
	}
}

func TestScanResult_CalculateSecurityGrade(t *testing.T) {
	tests := []struct {
		score    float64
		expected SecurityGrade
	}{
		{95, GradeA},
		{85, GradeB},
		{75, GradeC},
		{65, GradeD},
		{45, GradeF},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := &ScanResult{
				Summary: ScanSummary{
					Score: tt.score,
				},
			}
			if result.CalculateSecurityGrade() != tt.expected {
				t.Errorf("expected grade %s for score %f, got %s", 
					tt.expected, tt.score, result.CalculateSecurityGrade())
			}
		})
	}
}

func TestScanResult_ShouldBlockDeployment(t *testing.T) {
	// Failed scan should block
	failedResult := &ScanResult{
		Status: ScanStatusFailed,
	}
	if !failedResult.ShouldBlockDeployment() {
		t.Error("expected failed scan to block deployment")
	}

	// Critical issues should block
	criticalResult := &ScanResult{
		Status: ScanStatusCompleted,
		Summary: ScanSummary{
			CriticalCount: 1,
			Score:         90,
		},
	}
	if !criticalResult.ShouldBlockDeployment() {
		t.Error("expected critical issues to block deployment")
	}

	// F grade should block
	fGradeResult := &ScanResult{
		Status: ScanStatusCompleted,
		Summary: ScanSummary{
			Score: 30,
		},
	}
	if !fGradeResult.ShouldBlockDeployment() {
		t.Error("expected F grade to block deployment")
	}

	// Active high severity secrets should block
	secretResult := &ScanResult{
		Status: ScanStatusCompleted,
		Summary: ScanSummary{
			Score: 90,
		},
		Secrets: []Secret{
			{ID: "secret-1", Severity: SeverityHigh, IsActive: true},
		},
	}
	if !secretResult.ShouldBlockDeployment() {
		t.Error("expected active high severity secrets to block deployment")
	}

	// Clean scan should not block
	cleanResult := &ScanResult{
		Status: ScanStatusCompleted,
		Summary: ScanSummary{
			Score: 95,
		},
	}
	if cleanResult.ShouldBlockDeployment() {
		t.Error("expected clean scan to not block deployment")
	}
}

func TestSelectOptimalScanner(t *testing.T) {
	tests := []struct {
		scanType ScanType
		expected Scanner
	}{
		{ScanTypeVulnerability, ScannerTrivy},
		{ScanTypeSecret, ScannerTrivy},
		{ScanTypeMalware, ScannerClair},
		{ScanTypeCompliance, ScannerAquaSec},
		{ScanTypeLicense, ScannerSnyk},
	}

	for _, tt := range tests {
		t.Run(string(tt.scanType), func(t *testing.T) {
			if SelectOptimalScanner(tt.scanType) != tt.expected {
				t.Errorf("expected %s for %s", tt.expected, tt.scanType)
			}
		})
	}
}

func TestEstimateScanTime(t *testing.T) {
	req := &ScanRequest{
		ScanType: ScanTypeVulnerability,
		Target: ScanTarget{
			Type: TargetTypeImage,
		},
	}

	duration := EstimateScanTime(req)
	if duration <= 0 {
		t.Error("expected positive scan time estimate")
	}

	// Repository scan should take longer than image scan
	repoReq := &ScanRequest{
		ScanType: ScanTypeVulnerability,
		Target: ScanTarget{
			Type: TargetTypeRepository,
		},
	}

	repoDuration := EstimateScanTime(repoReq)
	if repoDuration <= duration {
		t.Error("expected repository scan to take longer than image scan")
	}
}