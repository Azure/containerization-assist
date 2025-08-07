package report

import (
	"fmt"
	"strings"
)

// FormatMCPMarkdownReport generates a markdown report similar to the CLI version
func FormatMCPMarkdownReport(report *MCPProgressiveReport) string {
	var md strings.Builder

	md.WriteString(fmt.Sprintf("# MCP Workflow Report\n\n"))
	md.WriteString(fmt.Sprintf("**Workflow ID:** %s\n\n", report.WorkflowID))
	md.WriteString(fmt.Sprintf("**Outcome:** %s\n\n", report.Summary.Outcome))
	md.WriteString(fmt.Sprintf("**Total Iterations:** %d\n\n", report.Summary.IterationCount))
	md.WriteString(fmt.Sprintf("**Success Rate:** %.1f%%\n\n", report.Summary.SuccessRate))
	md.WriteString(fmt.Sprintf("**Total Duration:** %s\n\n", report.Summary.TotalDuration))

	// Step Results
	md.WriteString("## Step Results\n\n")
	if len(report.StepResults) == 0 {
		md.WriteString("No steps executed yet.\n\n")
	} else {
		md.WriteString("| Step Name | Status | Duration | Success | Artifacts |\n")
		md.WriteString("|-----------|--------|----------|---------|----------|\n")
		for _, step := range report.StepResults {
			status := "✅"
			if !step.Success {
				status = "❌"
			}
			artifactCount := len(step.Artifacts)
			md.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %d |\n",
				step.StepName, step.Status, step.Duration, status, artifactCount))
		}
		md.WriteString("\n")
	}

	// Stage History
	md.WriteString("## Stage History\n\n")
	if len(report.StageHistory) == 0 {
		md.WriteString("No stage history recorded.\n\n")
	} else {
		md.WriteString("| Stage ID | Retry Count | Outcome |\n")
		md.WriteString("|----------|-------------|---------|\n")
		for _, visit := range report.StageHistory {
			md.WriteString(fmt.Sprintf("| %s | %d | %s |\n", visit.StageID, visit.RetryCount, visit.Outcome))
		}
		md.WriteString("\n")
	}

	// Detected Databases
	md.WriteString("## Detected Databases\n\n")
	if len(report.DetectedDatabases) == 0 {
		md.WriteString("No databases detected.\n\n")
	} else {
		md.WriteString("| Database Type | Version | Source |\n")
		md.WriteString("|---------------|---------|--------|\n")
		for _, db := range report.DetectedDatabases {
			md.WriteString(fmt.Sprintf("| %s | %s | %s |\n", db.Type, db.Version, db.Source))
		}
		md.WriteString("\n")
	}

	// Generated Artifacts
	md.WriteString("## Generated Artifacts\n\n")
	if len(report.GeneratedFiles) == 0 {
		md.WriteString("No artifacts generated yet.\n\n")
	} else {
		md.WriteString("| Type | Path | Description |\n")
		md.WriteString("|------|------|-------------|\n")
		for _, artifact := range report.GeneratedFiles {
			md.WriteString(fmt.Sprintf("| %s | %s | %s |\n", artifact.Type, artifact.Path, artifact.Description))
		}
		md.WriteString("\n")
	}

	// Token Usage
	md.WriteString("## Token Usage\n\n")
	md.WriteString(fmt.Sprintf("**Prompt Tokens:** %d\n\n", report.TokenUsage.PromptTokens))
	md.WriteString(fmt.Sprintf("**Completion Tokens:** %d\n\n", report.TokenUsage.CompletionTokens))
	md.WriteString(fmt.Sprintf("**Total Tokens:** %d\n\n", report.TokenUsage.TotalTokens))

	return md.String()
}
