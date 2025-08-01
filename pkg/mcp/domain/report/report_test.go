package report

import (
	"strings"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/pipeline"
)

func TestMCPProgressiveReport(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()

	workflowID := "test-workflow-123"

	// Test creating new report
	report, err := LoadOrCreateMCPReport(workflowID, tempDir)
	if err != nil {
		t.Fatalf("Failed to create new report: %v", err)
	}

	if report.WorkflowID != workflowID {
		t.Errorf("Expected WorkflowID %s, got %s", workflowID, report.WorkflowID)
	}

	if report.Summary.Outcome != MCPOutcomeInProgress {
		t.Errorf("Expected initial outcome %s, got %s", MCPOutcomeInProgress, report.Summary.Outcome)
	}

	// Test adding step results
	startTime := time.Now()
	endTime := startTime.Add(5 * time.Second)

	outputs := map[string]interface{}{
		"dockerfile_path":    "/app/Dockerfile",
		"analysis_summary":   "Go application detected",
		"detected_framework": "gin",
	}

	artifacts := []GeneratedArtifact{
		{
			Type:        "dockerfile",
			Path:        "/app/Dockerfile",
			Description: "Generated Dockerfile for Go application",
			CreatedAt:   time.Now(),
		},
	}

	err = UpdateStepResult(report, "analyze_repository", true, startTime, &endTime, "", outputs, artifacts)
	if err != nil {
		t.Fatalf("Failed to update step result: %v", err)
	}

	// Verify step was added
	if len(report.StepResults) != 1 {
		t.Errorf("Expected 1 step result, got %d", len(report.StepResults))
	}

	step := report.StepResults[0]
	if step.StepName != "analyze_repository" {
		t.Errorf("Expected step name 'analyze_repository', got %s", step.StepName)
	}

	if !step.Success {
		t.Errorf("Expected step to be successful")
	}

	if len(step.Artifacts) != 1 {
		t.Errorf("Expected 1 artifact, got %d", len(step.Artifacts))
	}

	// Test database detection
	databases := []pipeline.DatabaseDetectionResult{
		{
			Type:    "postgresql",
			Version: "13",
			Source:  "docker-compose.yml",
		},
		{
			Type:    "redis",
			Version: "6.2",
			Source:  "requirements.txt",
		},
	}

	AddDatabaseDetection(report, databases)

	if len(report.DetectedDatabases) != 2 {
		t.Errorf("Expected 2 detected databases, got %d", len(report.DetectedDatabases))
	}

	// Test token usage
	UpdateTokenUsage(report, 100, 50)

	if report.TokenUsage.PromptTokens != 100 {
		t.Errorf("Expected 100 prompt tokens, got %d", report.TokenUsage.PromptTokens)
	}

	if report.TokenUsage.TotalTokens != 150 {
		t.Errorf("Expected 150 total tokens, got %d", report.TokenUsage.TotalTokens)
	}

	// Test saving (now prepares content in metadata instead of writing files)
	err = SaveMCPReport(report, tempDir)
	if err != nil {
		t.Fatalf("Failed to prepare report content: %v", err)
	}

	// Verify structured files were prepared in metadata
	if report.Metadata == nil {
		t.Errorf("Report metadata should not be nil after saving")
		return
	}

	files, exists := report.Metadata["files"]
	if !exists {
		t.Errorf("Files should be prepared in metadata")
		return
	}

	filesMap, ok := files.(map[string]interface{})
	if !ok {
		t.Errorf("Files metadata should be a map")
		return
	}

	if _, exists := filesMap["mcp_report.json"]; !exists {
		t.Errorf("JSON file should be prepared in metadata")
	}
	if _, exists := filesMap["mcp_report.md"]; !exists {
		t.Errorf("Markdown file should be prepared in metadata")
	}

	// Test creating another report (since we no longer read from disk, each call creates new)
	newReport, err := LoadOrCreateMCPReport(workflowID, tempDir)
	if err != nil {
		t.Fatalf("Failed to create new report: %v", err)
	}

	// Since we don't persist to disk, this should be a fresh report
	if len(newReport.StepResults) != 0 {
		t.Errorf("Expected 0 step results in fresh report, got %d", len(newReport.StepResults))
	}

	// Test summary updates using the original report (since we don't have persistence)
	UpdateSummary(report)

	if report.Summary.TotalSteps != 1 {
		t.Errorf("Expected 1 total step, got %d", report.Summary.TotalSteps)
	}

	if report.Summary.CompletedSteps != 1 {
		t.Errorf("Expected 1 completed step, got %d", report.Summary.CompletedSteps)
	}

	if report.Summary.SuccessRate != 100.0 {
		t.Errorf("Expected 100%% success rate, got %.1f%%", report.Summary.SuccessRate)
	}
}

func TestMCPReportFailedStep(t *testing.T) {
	tempDir := t.TempDir()
	workflowID := "test-workflow-failed"

	report, err := LoadOrCreateMCPReport(workflowID, tempDir)
	if err != nil {
		t.Fatalf("Failed to create new report: %v", err)
	}

	// Add a failed step
	startTime := time.Now()
	endTime := startTime.Add(2 * time.Second)

	err = UpdateStepResult(report, "build_image", false, startTime, &endTime, "Docker build failed", nil, nil)
	if err != nil {
		t.Fatalf("Failed to update failed step result: %v", err)
	}

	UpdateSummary(report)

	if report.Summary.Outcome != MCPOutcomeFailure {
		t.Errorf("Expected outcome %s, got %s", MCPOutcomeFailure, report.Summary.Outcome)
	}

	if report.Summary.FailedSteps != 1 {
		t.Errorf("Expected 1 failed step, got %d", report.Summary.FailedSteps)
	}

	if report.Summary.SuccessRate != 0.0 {
		t.Errorf("Expected 0%% success rate, got %.1f%%", report.Summary.SuccessRate)
	}
}

func TestMCPReportStageVisits(t *testing.T) {
	tempDir := t.TempDir()
	workflowID := "test-workflow-stages"

	report, err := LoadOrCreateMCPReport(workflowID, tempDir)
	if err != nil {
		t.Fatalf("Failed to create new report: %v", err)
	}

	// Add stage visits
	startTime := time.Now()
	endTime := startTime.Add(3 * time.Second)

	AddStageVisit(report, "analyze", pipeline.RunOutcomeSuccess, startTime, &endTime, 0)
	AddStageVisit(report, "build", pipeline.RunOutcomeFailure, startTime, &endTime, 1)

	if len(report.StageHistory) != 2 {
		t.Errorf("Expected 2 stage visits, got %d", len(report.StageHistory))
	}

	firstVisit := report.StageHistory[0]
	if firstVisit.StageID != "analyze" {
		t.Errorf("Expected first visit stage ID 'analyze', got %s", firstVisit.StageID)
	}

	if firstVisit.Outcome != pipeline.RunOutcomeSuccess {
		t.Errorf("Expected first visit outcome success, got %s", firstVisit.Outcome)
	}

	secondVisit := report.StageHistory[1]
	if secondVisit.RetryCount != 1 {
		t.Errorf("Expected second visit retry count 1, got %d", secondVisit.RetryCount)
	}
}

func TestMCPReportMarkdownGeneration(t *testing.T) {
	tempDir := t.TempDir()
	workflowID := "test-workflow-markdown"

	report, err := LoadOrCreateMCPReport(workflowID, tempDir)
	if err != nil {
		t.Fatalf("Failed to create new report: %v", err)
	}

	// Add some data
	startTime := time.Now()
	endTime := startTime.Add(2 * time.Second)

	artifacts := []GeneratedArtifact{
		{
			Type:        "dockerfile",
			Path:        "Dockerfile",
			Description: "Main application Dockerfile",
			CreatedAt:   time.Now(),
		},
	}

	err = UpdateStepResult(report, "generate_dockerfile", true, startTime, &endTime, "", nil, artifacts)
	if err != nil {
		t.Fatalf("Failed to update step result: %v", err)
	}
	UpdateTokenUsage(report, 200, 100)
	UpdateSummary(report)

	// Generate markdown
	markdown := FormatMCPMarkdownReport(report)

	// Verify markdown contains expected content
	if !strings.Contains(markdown, "# MCP Workflow Report") {
		t.Error("Markdown should contain main heading")
	}

	if !strings.Contains(markdown, workflowID) {
		t.Error("Markdown should contain workflow ID")
	}

	if !strings.Contains(markdown, "generate_dockerfile") {
		t.Error("Markdown should contain step name")
	}

	if !strings.Contains(markdown, "## Token Usage") {
		t.Error("Markdown should contain token usage section")
	}

	if !strings.Contains(markdown, "200") {
		t.Error("Markdown should contain prompt token count")
	}
}

func TestReportStepExecution(t *testing.T) {
	tempDir := t.TempDir()
	workflowID := "test-workflow-execution"

	startTime := time.Now()
	endTime := startTime.Add(1 * time.Second)

	outputs := map[string]interface{}{
		"image_id": "sha256:abc123",
		"size":     "500MB",
	}

	artifacts := []GeneratedArtifact{
		{
			Type:        "image",
			Path:        "myapp:latest",
			Description: "Built container image",
			CreatedAt:   time.Now(),
		},
	}

	// Test the high-level ReportStepExecution function (now returns report and error)
	report, err := ReportStepExecution(workflowID, "build_image", tempDir, true, startTime, &endTime, "", outputs, artifacts)
	if err != nil {
		t.Fatalf("Failed to report step execution: %v", err)
	}

	if report == nil {
		t.Fatalf("ReportStepExecution should return a report")
	}

	// Verify report contains the step
	if len(report.StepResults) != 1 {
		t.Errorf("Expected 1 step result, got %d", len(report.StepResults))
	}

	step := report.StepResults[0]
	if step.StepName != "build_image" {
		t.Errorf("Expected step name 'build_image', got %s", step.StepName)
	}

	if !step.Success {
		t.Error("Expected step to be successful")
	}

	if len(step.Artifacts) != 1 {
		t.Errorf("Expected 1 artifact, got %d", len(step.Artifacts))
	}

	// Verify structured files were prepared in metadata instead of files being written
	if report.Metadata == nil {
		t.Error("Report metadata should not be nil after step execution")
		return
	}

	files, exists := report.Metadata["files"]
	if !exists {
		t.Error("Files should be prepared in metadata after step execution")
		return
	}

	filesMap, ok := files.(map[string]interface{})
	if !ok {
		t.Error("Files metadata should be a map")
		return
	}

	if _, exists := filesMap["mcp_report.json"]; !exists {
		t.Error("JSON file should be prepared in metadata after step execution")
	}
	if _, exists := filesMap["mcp_report.md"]; !exists {
		t.Error("Markdown file should be prepared in metadata after step execution")
	}
}
