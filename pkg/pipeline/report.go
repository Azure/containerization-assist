package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/containerization-assist/pkg/common/logger"
)

type RunOutcome string

const (
	RunOutcomeSuccess RunOutcome = "success"
	RunOutcomeFailure RunOutcome = "failure"
	RunOutcomeTimeout RunOutcome = "timeout"
)

const RunReportFileName = "run_report.json"

const ReportMarkdownFileName = "report.md"

type RunReport struct {
	IterationCount    int                       `json:"iteration_count"`
	Outcome           RunOutcome                `json:"outcome"`
	StageHistory      []StageVisit              `json:"stage_history"`
	DetectedDatabases []DatabaseDetectionResult `json:"detected_databases"`
}

func NewReport(ctx context.Context, state *PipelineState) *RunReport {
	outcome := RunOutcomeFailure
	if ctx.Err() == context.DeadlineExceeded || ctx.Err() == context.Canceled {
		outcome = RunOutcomeTimeout
	}
	if state.Success {
		outcome = RunOutcomeSuccess
	}

	var detectedDatabases []DatabaseDetectionResult
	if state.Metadata["detectedDatabaseErrors"] == nil && len(state.DetectedDatabases) > 0 {
		detectedDatabases = state.DetectedDatabases
	}

	return &RunReport{
		IterationCount:    state.IterationCount,
		Outcome:           outcome,
		StageHistory:      state.StageHistory,
		DetectedDatabases: detectedDatabases,
	}
}

// formatMarkdownReport creates a markdown string from the pipeline state and context.
// Uses the context to determine the run outcome like in the Runreport struct.
// Uses the pipeline to get the stage history and iteration count and other details that are not present in the RunReport struct.
func formatMarkdownReport(ctx context.Context, state *PipelineState) string {
	var md strings.Builder

	outcome := RunOutcomeFailure
	if ctx.Err() == context.DeadlineExceeded || ctx.Err() == context.Canceled {
		outcome = RunOutcomeTimeout
	}
	if state.Success {
		outcome = RunOutcomeSuccess
	}

	md.WriteString(fmt.Sprintf("**Outcome:** %s\n\n", outcome))
	md.WriteString(fmt.Sprintf("**Total Iterations:** %d\n\n", state.IterationCount))
	md.WriteString("## Stage History\n\n")

	if len(state.StageHistory) == 0 {
		md.WriteString("No stage history recorded.\n")
	} else {
		md.WriteString("| Stage ID | Retry Count | Outcome |\n")
		md.WriteString("|----------|-------------|---------|\n")
		for _, visit := range state.StageHistory {
			md.WriteString(fmt.Sprintf("| %s | %d | %s |\n", visit.StageID, visit.RetryCount, visit.Outcome))
		}
	}

	md.WriteString("\n## Detected Databases\n\n")
	if state.Metadata["detectedDatabaseErrors"] == nil && len(state.DetectedDatabases) > 0 {
		md.WriteString("| Database Type | Version | Source |\n")
		md.WriteString("|---------------|---------|--------|\n")
		for _, db := range state.DetectedDatabases {
			md.WriteString(fmt.Sprintf("| %s | %s | %s |\n", db.Type, db.Version, db.Source))
		}
	} else {
		md.WriteString("No databases detected.\n")
	}

	md.WriteString("\n## Token Usage\n\n")
	md.WriteString(fmt.Sprintf("Prompt Tokens: %d\n", state.TokenUsage.PromptTokens))
	md.WriteString(fmt.Sprintf("Completion Tokens: %d\n", state.TokenUsage.CompletionTokens))
	md.WriteString(fmt.Sprintf("Total Tokens: %d\n", state.TokenUsage.TotalTokens))

	return md.String()
}

func WriteReport(ctx context.Context, state *PipelineState, targetDir string) error {
	reportDirectoryPath := filepath.Join(targetDir, ReportDirectory)
	if err := os.MkdirAll(reportDirectoryPath, 0755); err != nil {
		logger.Errorf("Error creating report directory %s: %v", reportDirectoryPath, err)
		return fmt.Errorf("creating report directory: %w", err)
	}

	report := NewReport(ctx, state)
	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		logger.Warnf("Error marshalling stage history: %v", err)
		return fmt.Errorf("marshalling stage history: %w", err)
	}
	reportFile := filepath.Join(reportDirectoryPath, RunReportFileName)
	logger.Debugf("Writing stage history to %s", reportFile)
	if err := os.WriteFile(reportFile, reportJSON, 0644); err != nil {
		logger.Errorf("Error writing stage history to file: %v", err)
		return fmt.Errorf("writing stage history to file: %w", err)
	}

	// Generate and write the markdown report using context and pipeline state
	markdownReportContent := formatMarkdownReport(ctx, state)
	reportMarkdownFile := filepath.Join(reportDirectoryPath, ReportMarkdownFileName)
	logger.Debugf("Writing markdown report to %s", reportMarkdownFile)
	if err := os.WriteFile(reportMarkdownFile, []byte(markdownReportContent), 0644); err != nil {
		logger.Errorf("Error writing markdown report to file: %v", err)
		return fmt.Errorf("writing markdown report to file: %w", err)
	}

	return nil
}
