package report

import (
	"time"

	"github.com/Azure/container-kit/pkg/pipeline"
)

// UpdateSummary recalculates the summary statistics
func UpdateSummary(report *MCPProgressiveReport) {
	summary := &report.Summary
	summary.TotalSteps = len(report.StepResults)
	summary.CompletedSteps = 0
	summary.FailedSteps = 0
	summary.TotalArtifacts = len(report.GeneratedFiles)

	for _, step := range report.StepResults {
		if step.Success {
			summary.CompletedSteps++
		} else {
			summary.FailedSteps++
		}
	}

	// Calculate success rate
	if summary.TotalSteps > 0 {
		summary.SuccessRate = float64(summary.CompletedSteps) / float64(summary.TotalSteps) * 100
	}

	// Determine overall outcome
	if summary.FailedSteps > 0 {
		summary.Outcome = MCPOutcomeFailure
	} else if summary.CompletedSteps == summary.TotalSteps && summary.TotalSteps > 0 {
		summary.Outcome = MCPOutcomeSuccess
	} else {
		summary.Outcome = MCPOutcomeInProgress
	}

	// Calculate total duration
	if len(report.StepResults) > 0 {
		firstStep := report.StepResults[0]
		var lastEndTime time.Time = report.LastUpdated

		for _, step := range report.StepResults {
			if step.EndTime != nil && step.EndTime.After(lastEndTime) {
				lastEndTime = *step.EndTime
			}
		}

		summary.TotalDuration = lastEndTime.Sub(firstStep.StartTime).String()
	}

	summary.LastUpdated = time.Now()
}

// AddDatabaseDetection adds database detection results to the report
func AddDatabaseDetection(report *MCPProgressiveReport, databases []pipeline.DatabaseDetectionResult) {
	report.DetectedDatabases = append(report.DetectedDatabases, databases...)
	report.LastUpdated = time.Now()
}

// UpdateTokenUsage updates token usage statistics in the report
func UpdateTokenUsage(report *MCPProgressiveReport, promptTokens, completionTokens int) {
	report.TokenUsage.PromptTokens += promptTokens
	report.TokenUsage.CompletionTokens += completionTokens
	report.TokenUsage.TotalTokens = report.TokenUsage.PromptTokens + report.TokenUsage.CompletionTokens
	report.LastUpdated = time.Now()
}

// AddStageVisit adds a stage visit to the history
func AddStageVisit(report *MCPProgressiveReport, stageID string, outcome pipeline.RunOutcome, startTime time.Time, endTime *time.Time, retryCount int) {
	visit := MCPStageVisit{
		StageID:    stageID,
		RetryCount: retryCount,
		Outcome:    outcome,
		StartTime:  startTime,
		EndTime:    endTime,
	}
	report.StageHistory = append(report.StageHistory, visit)
	report.LastUpdated = time.Now()
}
