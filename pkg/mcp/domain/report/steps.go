package report

import (
	"fmt"
	"time"
)

// ReportStepExecution updates the MCP report with step execution results
func ReportStepExecution(workflowID, stepName, targetDir string, success bool, startTime time.Time, endTime *time.Time, errorMsg string, outputs map[string]interface{}, artifacts []GeneratedArtifact) error {
	report, err := LoadOrCreateMCPReport(workflowID, targetDir)
	if err != nil {
		return fmt.Errorf("loading report: %w", err)
	}

	// Update step result
	if err := UpdateStepResult(report, stepName, success, startTime, endTime, errorMsg, outputs, artifacts); err != nil {
		return fmt.Errorf("updating step result: %w", err)
	}

	// Update summary
	UpdateSummary(report)

	// Save report
	if err := SaveMCPReport(report, targetDir); err != nil {
		return fmt.Errorf("saving report: %w", err)
	}

	return nil
}

// UpdateStepResult updates or adds a step result to the report
func UpdateStepResult(report *MCPProgressiveReport, stepName string, success bool, startTime time.Time, endTime *time.Time, errorMsg string, outputs map[string]interface{}, artifacts []GeneratedArtifact) error {
	now := time.Now()

	// Calculate duration
	var duration string
	if endTime != nil {
		duration = endTime.Sub(startTime).String()
	} else {
		duration = now.Sub(startTime).String()
		endTime = &now
	}

	// Find existing step result or create new one
	var stepResult *MCPStepResult
	for i := range report.StepResults {
		if report.StepResults[i].StepName == stepName {
			stepResult = &report.StepResults[i]
			break
		}
	}

	if stepResult == nil {
		// Create new step result
		newStep := MCPStepResult{
			StepName:  stepName,
			StartTime: startTime,
		}
		report.StepResults = append(report.StepResults, newStep)
		stepResult = &report.StepResults[len(report.StepResults)-1]
	}

	// Update step result
	stepResult.EndTime = endTime
	stepResult.Duration = duration
	stepResult.Success = success
	stepResult.ErrorMessage = errorMsg
	stepResult.Outputs = outputs
	stepResult.Artifacts = artifacts

	if success {
		stepResult.Status = "completed"
	} else {
		stepResult.Status = "failed"
		stepResult.RetryCount++
	}

	// Add artifacts to global list
	for _, artifact := range artifacts {
		report.GeneratedFiles = append(report.GeneratedFiles, artifact)
	}

	report.LastUpdated = now
	return nil
}
