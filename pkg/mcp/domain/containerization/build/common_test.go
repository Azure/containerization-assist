package build

import (
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
	"github.com/Azure/container-kit/pkg/core/docker"
)

// Test BuildError Error method
func TestBuildErrorError(t *testing.T) {
	err := &CommonBuildError{
		Code:    "BUILD_FAILED",
		Message: "Docker build failed",
		Stage:   "build",
		Type:    "docker_error",
	}
	if err.Error() != "Docker build failed" {
		t.Errorf("Expected Error() to return 'Docker build failed', got '%s'", err.Error())
	}
}

// Test NewCommonBuildError constructor
func TestNewCommonBuildError(t *testing.T) {
	err := NewCommonBuildError("BUILD_TIMEOUT", "Build operation timed out", "compilation", "timeout_error")
	if err == nil {
		t.Error("NewBuildError should not return nil")
	}
	if err.Code != "BUILD_TIMEOUT" {
		t.Errorf("Expected Code to be 'BUILD_TIMEOUT', got '%s'", err.Code)
	}
	if err.Message != "Build operation timed out" {
		t.Errorf("Expected Message to be 'Build operation timed out', got '%s'", err.Message)
	}
	if err.Stage != "compilation" {
		t.Errorf("Expected Stage to be 'compilation', got '%s'", err.Stage)
	}
	if err.Type != "timeout_error" {
		t.Errorf("Expected Type to be 'timeout_error', got '%s'", err.Type)
	}
}

// Test BuildError struct fields
func TestBuildErrorStruct(t *testing.T) {
	err := CommonBuildError{
		Code:    "VALIDATION_ERROR",
		Message: "Invalid dockerfile syntax",
		Stage:   "validation",
		Type:    "syntax_error",
		Line:    15,
	}
	if err.Code != "VALIDATION_ERROR" {
		t.Errorf("Expected Code to be 'VALIDATION_ERROR', got '%s'", err.Code)
	}
	if err.Message != "Invalid dockerfile syntax" {
		t.Errorf("Expected Message to be 'Invalid dockerfile syntax', got '%s'", err.Message)
	}
	if err.Stage != "validation" {
		t.Errorf("Expected Stage to be 'validation', got '%s'", err.Stage)
	}
	if err.Type != "syntax_error" {
		t.Errorf("Expected Type to be 'syntax_error', got '%s'", err.Type)
	}
	if err.Line != 15 {
		t.Errorf("Expected Line to be 15, got %d", err.Line)
	}
}

// Test determineImpact function
func TestDetermineImpact(t *testing.T) {
	testCases := []struct {
		warningType string
		expected    string
	}{
		{"security", "security"},
		{"best_practice", "maintainability"},
		{"performance", "performance"},
		{"unknown", "performance"},
		{"", "performance"},
	}
	for _, tc := range testCases {
		result := determineImpact(tc.warningType)
		if result != tc.expected {
			t.Errorf("determineImpact(%s): expected '%s', got '%s'", tc.warningType, tc.expected, result)
		}
	}
}

// Test ConvertCoreResult function
func TestConvertCoreResult(t *testing.T) {
	// Create a mock docker.BuildResult
	coreResult := &docker.BuildResult{
		Success: false, // false because we have errors
		Error: &docker.BuildError{
			Message: "Missing FROM instruction",
			Type:    "dockerfile_error",
		},
		// Note: docker.BuildResult doesn't have a Warnings field
		// Warnings would be in the logs or error context
	}
	result := ConvertCoreResult(coreResult)
	if result == nil {
		t.Error("ConvertCoreResult should not return nil")
	}
	// Note: Unified validation framework sets Valid=false when errors are present
	if result.Valid {
		t.Error("Expected result.Valid to be false when errors are present")
	}
	if len(result.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(result.Errors))
	}
	// ConvertCoreResult doesn't preserve line numbers from docker.BuildError
	if result.Errors[0].Message != "Missing FROM instruction" {
		t.Errorf("Expected error message to be 'Missing FROM instruction', got '%s'", result.Errors[0].Message)
	}
	// docker.BuildResult doesn't have warnings, so we can't test them here
}

// Test ValidationResult type
func TestValidationResult(t *testing.T) {
	result := core.NewBuildResult("test-validator", "1.0.0")

	// Add test error
	error := core.NewError(
		"SYNTAX_ERROR",
		"Syntax error",
		core.ErrTypeSyntax,
		core.SeverityHigh,
	).WithLine(5).WithColumn(10).WithRule("DL3000")
	result.AddError(error)

	// Add test warning
	warning := core.NewWarning(
		"BEST_PRACTICE_WARNING",
		"Best practice warning",
	)
	warning.Error.WithLine(15).WithColumn(0).WithRule("DL3008")
	result.AddWarning(warning)

	// Add info
	result.Suggestions = append(result.Suggestions, "Dockerfile validated successfully")
	// Note: Unified validation framework sets Valid=false when errors are present
	if result.Valid {
		t.Error("Expected Valid to be false when errors are present")
	}
	if len(result.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(result.Errors))
	}
	if result.Errors[0].Line != 5 {
		t.Errorf("Expected error line to be 5, got %d", result.Errors[0].Line)
	}
	if result.Errors[0].Message != "Syntax error" {
		t.Errorf("Expected error message to be 'Syntax error', got '%s'", result.Errors[0].Message)
	}
	if len(result.Warnings) != 1 {
		t.Errorf("Expected 1 warning, got %d", len(result.Warnings))
	}
	if len(result.Suggestions) != 1 {
		t.Errorf("Expected 1 suggestion, got %d", len(result.Suggestions))
	}
}

// Test ValidationError type
func TestValidationError(t *testing.T) {
	error := ValidationError{
		Line:    20,
		Column:  5,
		Message: "Missing required instruction",
		Rule:    "DL3001",
	}
	if error.Line != 20 {
		t.Errorf("Expected Line to be 20, got %d", error.Line)
	}
	if error.Column != 5 {
		t.Errorf("Expected Column to be 5, got %d", error.Column)
	}
	if error.Message != "Missing required instruction" {
		t.Errorf("Expected Message to be 'Missing required instruction', got '%s'", error.Message)
	}
	if error.Rule != "DL3001" {
		t.Errorf("Expected Rule to be 'DL3001', got '%s'", error.Rule)
	}
}

// Test ValidationWarning type
func TestValidationWarning(t *testing.T) {
	warning := core.NewWarning(
		"VERSION_WARNING",
		"Consider using specific version",
	)
	warning.Error.WithLine(30).WithColumn(8).WithRule("DL3006")
	if warning.Error.Line != 30 {
		t.Errorf("Expected Line to be 30, got %d", warning.Error.Line)
	}
	if warning.Error.Column != 8 {
		t.Errorf("Expected Column to be 8, got %d", warning.Error.Column)
	}
	if warning.Message != "Consider using specific version" {
		t.Errorf("Expected Message to be 'Consider using specific version', got '%s'", warning.Message)
	}
	if warning.Error.Rule != "DL3006" {
		t.Errorf("Expected Rule to be 'DL3006', got '%s'", warning.Error.Rule)
	}
}

// Test BuildContext type
func TestBuildContext(t *testing.T) {
	context := BuildContext{
		SessionID:      "session-123",
		WorkspaceDir:   "/workspace",
		ImageName:      "myapp",
		ImageTag:       "v1.0.0",
		DockerfilePath: "/workspace/Dockerfile",
		BuildPath:      "/workspace",
		Platform:       "linux/amd64",
		NoCache:        true,
	}
	if context.SessionID != "session-123" {
		t.Errorf("Expected SessionID to be 'session-123', got '%s'", context.SessionID)
	}
	if context.WorkspaceDir != "/workspace" {
		t.Errorf("Expected WorkspaceDir to be '/workspace', got '%s'", context.WorkspaceDir)
	}
	if context.ImageName != "myapp" {
		t.Errorf("Expected ImageName to be 'myapp', got '%s'", context.ImageName)
	}
	if context.ImageTag != "v1.0.0" {
		t.Errorf("Expected ImageTag to be 'v1.0.0', got '%s'", context.ImageTag)
	}
	if context.DockerfilePath != "/workspace/Dockerfile" {
		t.Errorf("Expected DockerfilePath to be '/workspace/Dockerfile', got '%s'", context.DockerfilePath)
	}
	if context.BuildPath != "/workspace" {
		t.Errorf("Expected BuildPath to be '/workspace', got '%s'", context.BuildPath)
	}
	if context.Platform != "linux/amd64" {
		t.Errorf("Expected Platform to be 'linux/amd64', got '%s'", context.Platform)
	}
	if !context.NoCache {
		t.Error("Expected NoCache to be true")
	}
}

// Test BuildResult type
func TestBuildResult(t *testing.T) {
	duration := time.Minute * 5
	result := BuildResult{
		Success:        true,
		ImageID:        "sha256:abc123",
		FullImageRef:   "myapp:v1.0.0",
		Duration:       duration,
		LayerCount:     10,
		ImageSizeBytes: 1024 * 1024 * 100, // 100MB
		BuildLogs:      []string{"Step 1/5", "Step 2/5"},
		CacheHits:      3,
	}
	if !result.Success {
		t.Error("Expected Success to be true")
	}
	if result.ImageID != "sha256:abc123" {
		t.Errorf("Expected ImageID to be 'sha256:abc123', got '%s'", result.ImageID)
	}
	if result.FullImageRef != "myapp:v1.0.0" {
		t.Errorf("Expected FullImageRef to be 'myapp:v1.0.0', got '%s'", result.FullImageRef)
	}
	if result.Duration != duration {
		t.Errorf("Expected Duration to be %v, got %v", duration, result.Duration)
	}
	if result.LayerCount != 10 {
		t.Errorf("Expected LayerCount to be 10, got %d", result.LayerCount)
	}
	expectedSize := int64(1024 * 1024 * 100)
	if result.ImageSizeBytes != expectedSize {
		t.Errorf("Expected ImageSizeBytes to be %d, got %d", expectedSize, result.ImageSizeBytes)
	}
	if len(result.BuildLogs) != 2 {
		t.Errorf("Expected 2 build logs, got %d", len(result.BuildLogs))
	}
	if result.CacheHits != 3 {
		t.Errorf("Expected CacheHits to be 3, got %d", result.CacheHits)
	}
}

// Test SecurityIssue type
func TestSecurityIssue(t *testing.T) {
	issue := SecurityIssue{
		Severity:    "high",
		Type:        "vulnerability",
		Message:     "Outdated package detected",
		Line:        25,
		Remediation: "Update to latest version",
	}
	if issue.Severity != "high" {
		t.Errorf("Expected Severity to be 'high', got '%s'", issue.Severity)
	}
	if issue.Type != "vulnerability" {
		t.Errorf("Expected Type to be 'vulnerability', got '%s'", issue.Type)
	}
	if issue.Message != "Outdated package detected" {
		t.Errorf("Expected Message to be 'Outdated package detected', got '%s'", issue.Message)
	}
	if issue.Line != 25 {
		t.Errorf("Expected Line to be 25, got %d", issue.Line)
	}
	if issue.Remediation != "Update to latest version" {
		t.Errorf("Expected Remediation to be 'Update to latest version', got '%s'", issue.Remediation)
	}
}

// Test ComplianceViolation type
func TestComplianceViolation(t *testing.T) {
	violation := ComplianceViolation{
		Standard: "CIS Docker Benchmark",
		Rule:     "4.1",
		Message:  "Do not use root user",
		Line:     18,
	}
	if violation.Standard != "CIS Docker Benchmark" {
		t.Errorf("Expected Standard to be 'CIS Docker Benchmark', got '%s'", violation.Standard)
	}
	if violation.Rule != "4.1" {
		t.Errorf("Expected Rule to be '4.1', got '%s'", violation.Rule)
	}
	if violation.Message != "Do not use root user" {
		t.Errorf("Expected Message to be 'Do not use root user', got '%s'", violation.Message)
	}
	if violation.Line != 18 {
		t.Errorf("Expected Line to be 18, got %d", violation.Line)
	}
}
