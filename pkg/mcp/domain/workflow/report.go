// Package workflow provides MCP workflow orchestration and reporting functionality.
//
// This file re-exports the comprehensive MCP reporting functionality from the report sub-package
// to maintain backward compatibility while organizing the code into logical components.
package workflow

import (
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/report"
	"github.com/Azure/container-kit/pkg/pipeline"
)

// Re-export types from report package for backward compatibility
type (
	MCPProgressiveReport = report.MCPProgressiveReport
	MCPStepResult        = report.MCPStepResult
	MCPReportSummary     = report.MCPReportSummary
	GeneratedArtifact    = report.GeneratedArtifact
	MCPStageVisit        = report.MCPStageVisit
	MCPTokenUsage        = report.MCPTokenUsage
	MCPOutcome           = report.MCPOutcome
)

// Re-export constants from report package
const (
	MCPReportDirectory   = report.MCPReportDirectory
	MCPReportFileName    = report.MCPReportFileName
	MCPMarkdownFileName  = report.MCPMarkdownFileName
	MCPOutcomeSuccess    = report.MCPOutcomeSuccess
	MCPOutcomeFailure    = report.MCPOutcomeFailure
	MCPOutcomeInProgress = report.MCPOutcomeInProgress
	MCPOutcomeTimeout    = report.MCPOutcomeTimeout
)

// Re-export functions from report package for backward compatibility

// LoadOrCreateMCPReport loads an existing MCP report or creates a new one
func LoadOrCreateMCPReport(workflowID, targetDir string) (*MCPProgressiveReport, error) {
	return report.LoadOrCreateMCPReport(workflowID, targetDir)
}

// SaveMCPReport saves the MCP report to disk in both JSON and Markdown formats
func SaveMCPReport(rpt *MCPProgressiveReport, targetDir string) error {
	return report.SaveMCPReport(rpt, targetDir)
}

// ReportStepExecution updates the MCP report with step execution results
func ReportStepExecution(workflowID, stepName, targetDir string, success bool, startTime time.Time, endTime *time.Time, errorMsg string, outputs map[string]interface{}, artifacts []GeneratedArtifact) error {
	return report.ReportStepExecution(workflowID, stepName, targetDir, success, startTime, endTime, errorMsg, outputs, artifacts)
}

// UpdateStepResult updates or adds a step result to the report
func UpdateStepResult(rpt *MCPProgressiveReport, stepName string, success bool, startTime time.Time, endTime *time.Time, errorMsg string, outputs map[string]interface{}, artifacts []GeneratedArtifact) error {
	return report.UpdateStepResult(rpt, stepName, success, startTime, endTime, errorMsg, outputs, artifacts)
}

// UpdateSummary recalculates the summary statistics
func UpdateSummary(rpt *MCPProgressiveReport) {
	report.UpdateSummary(rpt)
}

// AddDatabaseDetection adds database detection results to the report
func AddDatabaseDetection(rpt *MCPProgressiveReport, databases []pipeline.DatabaseDetectionResult) {
	report.AddDatabaseDetection(rpt, databases)
}

// UpdateTokenUsage updates token usage statistics in the report
func UpdateTokenUsage(rpt *MCPProgressiveReport, promptTokens, completionTokens int) {
	report.UpdateTokenUsage(rpt, promptTokens, completionTokens)
}

// AddStageVisit adds a stage visit to the history
func AddStageVisit(rpt *MCPProgressiveReport, stageID string, outcome pipeline.RunOutcome, startTime time.Time, endTime *time.Time, retryCount int) {
	report.AddStageVisit(rpt, stageID, outcome, startTime, endTime, retryCount)
}

// FormatMCPMarkdownReport generates a markdown report similar to the CLI version
func FormatMCPMarkdownReport(rpt *MCPProgressiveReport) string {
	return report.FormatMCPMarkdownReport(rpt)
}
