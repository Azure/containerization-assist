package test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/report"
)

func TestMCPReportCreation(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "mcp_report_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test parameters
	workflowID := "test-workflow-123"
	stepName := "analyze_repository"

	// Simulate successful step execution
	startTime := time.Now()
	endTime := time.Now().Add(5 * time.Second)

	outputs := map[string]interface{}{
		"session_id": workflowID,
		"repo_path":  tempDir,
		"language":   "go",
		"framework":  "none",
	}

	artifacts := []report.GeneratedArtifact{
		{
			Type:        "dockerfile",
			Path:        "Dockerfile",
			Description: "Generated Dockerfile for containerization",
			CreatedAt:   endTime,
		},
	}

	// Execute the reporting
	err = report.ReportStepExecution(workflowID, stepName, tempDir, true, startTime, &endTime, "", outputs, artifacts)
	if err != nil {
		t.Fatalf("Failed to report step execution: %v", err)
	}

	// Verify report files were created
	reportDir := filepath.Join(tempDir, report.MCPReportDirectory)

	// Check JSON report
	jsonReportPath := filepath.Join(reportDir, report.MCPReportFileName)
	if _, err := os.Stat(jsonReportPath); os.IsNotExist(err) {
		t.Errorf("Expected MCP JSON report file to exist at %s", jsonReportPath)
	}

	// Check Markdown report
	mdReportPath := filepath.Join(reportDir, report.MCPMarkdownFileName)
	if _, err := os.Stat(mdReportPath); os.IsNotExist(err) {
		t.Errorf("Expected MCP Markdown report file to exist at %s", mdReportPath)
	}

	// Verify JSON report content
	jsonContent, err := os.ReadFile(jsonReportPath)
	if err != nil {
		t.Fatalf("Failed to read JSON report: %v", err)
	}

	var mcpReport report.MCPProgressiveReport
	err = json.Unmarshal(jsonContent, &mcpReport)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON report: %v", err)
	}

	// Verify report structure
	if mcpReport.WorkflowID != workflowID {
		t.Errorf("Expected workflow ID %s, got %s", workflowID, mcpReport.WorkflowID)
	}

	if len(mcpReport.StepResults) != 1 {
		t.Errorf("Expected 1 step in report, got %d", len(mcpReport.StepResults))
	}

	step := mcpReport.StepResults[0]
	if step.StepName != stepName {
		t.Errorf("Expected step name %s, got %s", stepName, step.StepName)
	}

	if !step.Success {
		t.Error("Expected step to be successful")
	}

	if len(step.Artifacts) != 1 {
		t.Errorf("Expected 1 artifact, got %d", len(step.Artifacts))
	}

	// Verify summary
	if mcpReport.Summary.TotalSteps != 1 {
		t.Errorf("Expected total steps to be 1, got %d", mcpReport.Summary.TotalSteps)
	}

	if mcpReport.Summary.CompletedSteps != 1 {
		t.Errorf("Expected completed steps to be 1, got %d", mcpReport.Summary.CompletedSteps)
	}

	if mcpReport.Summary.FailedSteps != 0 {
		t.Errorf("Expected failed steps to be 0, got %d", mcpReport.Summary.FailedSteps)
	}

	fmt.Printf("✅ MCP Report Integration Test Passed!\n")
	fmt.Printf("📁 Report created at: %s\n", reportDir)
	fmt.Printf("📊 Report contains %d steps\n", len(mcpReport.StepResults))
}

func TestMCPReportFailedStep(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "mcp_report_fail_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test parameters for failed step
	workflowID := "test-workflow-fail-456"
	stepName := "generate_dockerfile"

	startTime := time.Now()
	endTime := time.Now().Add(2 * time.Second)
	errorMsg := "Failed to generate Dockerfile: missing dependencies"

	outputs := map[string]interface{}{
		"session_id": workflowID,
		"error_type": "dependency_error",
	}

	// Execute the reporting for failed step
	err = report.ReportStepExecution(workflowID, stepName, tempDir, false, startTime, &endTime, errorMsg, outputs, nil)
	if err != nil {
		t.Fatalf("Failed to report failed step execution: %v", err)
	}

	// Verify report files were created
	reportDir := filepath.Join(tempDir, report.MCPReportDirectory)
	jsonReportPath := filepath.Join(reportDir, report.MCPReportFileName)

	// Verify JSON report content
	jsonContent, err := os.ReadFile(jsonReportPath)
	if err != nil {
		t.Fatalf("Failed to read JSON report: %v", err)
	}

	var mcpReport report.MCPProgressiveReport
	err = json.Unmarshal(jsonContent, &mcpReport)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON report: %v", err)
	}

	// Verify failed step
	if len(mcpReport.StepResults) != 1 {
		t.Errorf("Expected 1 step in report, got %d", len(mcpReport.StepResults))
	}

	step := mcpReport.StepResults[0]
	if step.Success {
		t.Error("Expected step to be failed")
	}

	if step.ErrorMessage != errorMsg {
		t.Errorf("Expected error message %s, got %s", errorMsg, step.ErrorMessage)
	}

	// Verify summary reflects failure
	if mcpReport.Summary.TotalSteps != 1 {
		t.Errorf("Expected total steps to be 1, got %d", mcpReport.Summary.TotalSteps)
	}

	if mcpReport.Summary.CompletedSteps != 0 {
		t.Errorf("Expected completed steps to be 0, got %d", mcpReport.Summary.CompletedSteps)
	}

	if mcpReport.Summary.FailedSteps != 1 {
		t.Errorf("Expected failed steps to be 1, got %d", mcpReport.Summary.FailedSteps)
	}

	fmt.Printf("✅ MCP Report Failed Step Test Passed!\n")
}
