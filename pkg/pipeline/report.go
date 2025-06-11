package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/container-copilot/pkg/logger"
)

type RunOutcome string

const (
	RunOutcomeSuccess RunOutcome = "success"
	RunOutcomeFailure RunOutcome = "failure"
	RunOutcomeTimeout RunOutcome = "timeout"
)

type RunReport struct {
	IterationCount int          `json:"iteration_count"`
	Outcome        RunOutcome   `json:"outcome"`
	StageHistory   []StageVisit `json:"stage_history"`
}

func NewReport(ctx context.Context, state *PipelineState, targetDir string) *RunReport {
	outcome := RunOutcomeSuccess
	// if deadline exceeded or canceled, set outcome to timeout
	if ctx.Err() == context.DeadlineExceeded || ctx.Err() == context.Canceled {
		outcome = RunOutcomeTimeout
	}
	if !state.Success {
		outcome = RunOutcomeFailure
	}
	return &RunReport{
		IterationCount: state.IterationCount,
		Outcome:        outcome,
		StageHistory:   state.StageHistory,
	}
}

// formatMarkdownReport creates a markdown string from the pipeline state and context.
// Uses the context to determine the run outcome like in the Runreport struct.
// Uses the pipeline to get the stage history and iteration count and other details that are not present in the RunReport struct.
func formatMarkdownReport(ctx context.Context, state *PipelineState) string {
	var md strings.Builder

	outcome := RunOutcomeSuccess
	if ctx.Err() == context.DeadlineExceeded || ctx.Err() == context.Canceled {
		outcome = RunOutcomeTimeout
	}
	if !state.Success {
		outcome = RunOutcomeFailure
	}

	// md.WriteString(fmt.Sprintf("# Run Report\n\n"))
	md.WriteString(fmt.Sprintf("**Outcome:** %s\n\n", outcome))
	md.WriteString(fmt.Sprintf("**Total Iterations:** %d\n\n", state.IterationCount))
	md.WriteString(fmt.Sprintf("## Stage History\n\n"))

	if len(state.StageHistory) == 0 {
		md.WriteString("No stage history recorded.\n")
	} else {
		md.WriteString("| Stage ID | Retry Count | Outcome |\n")
		md.WriteString("|----------|-------------|---------|\n")
		for _, visit := range state.StageHistory {
			md.WriteString(fmt.Sprintf("| %s | %d | %s |\n", visit.StageID, visit.RetryCount, visit.Outcome))
		}
	}
	if state.Success {
		md.WriteString(fmt.Sprintf("\n## Token Usage\n\n"))
		md.WriteString(fmt.Sprintf("Prompt Tokens: %d\n", state.TokenUsage.PromptTokens))
		md.WriteString(fmt.Sprintf("Completion Tokens: %d\n", state.TokenUsage.CompletionTokens))
		md.WriteString(fmt.Sprintf("Total Tokens: %d\n", state.TokenUsage.TotalTokens))
	}
	return md.String()
}

func WriteReport(ctx context.Context, state *PipelineState, targetDir string) error {
	reportDirectoryPath := filepath.Join(targetDir, ReportDirectory)
	if err := os.MkdirAll(reportDirectoryPath, 0755); err != nil {
		logger.Errorf("Error creating report directory %s: %v", reportDirectoryPath, err)
		return fmt.Errorf("creating report directory: %w", err)
	}

	report := NewReport(ctx, state, targetDir)
	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		logger.Warnf("Error marshalling stage history: %v", err)
	}
	reportFile := filepath.Join(reportDirectoryPath, "run_report.json")
	logger.Debugf("Writing stage history to %s", reportFile)
	if err := os.WriteFile(reportFile, reportJSON, 0644); err != nil {
		logger.Errorf("Error writing stage history to file: %v", err)
	}

	// Generate and write the markdown report using context and pipeline state
	markdownReportContent := formatMarkdownReport(ctx, state)
	reportMarkdownFile := filepath.Join(reportDirectoryPath, "report.md")
	logger.Debugf("Writing markdown report to %s", reportMarkdownFile)
	if err := os.WriteFile(reportMarkdownFile, []byte(markdownReportContent), 0644); err != nil {
		logger.Errorf("Error writing markdown report to file: %v", err)
	}

	return nil
}
