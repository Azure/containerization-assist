package workflow

import (
	"log/slog"
	"time"
)

// LogStepStart logs the beginning of step execution
func LogStepStart(logger *slog.Logger, stepName string, workflowID string, stepIndex, totalSteps int) {
	logger.Info("Starting workflow step",
		"step", stepName,
		"workflow_id", workflowID,
		"step_number", stepIndex,
		"total_steps", totalSteps)
}

// LogStepComplete logs successful step completion
func LogStepComplete(logger *slog.Logger, stepName string, workflowID string, duration time.Duration) {
	logger.Info("Workflow step completed",
		"step", stepName,
		"workflow_id", workflowID,
		"duration", duration)
}

// LogStepFailed logs step execution failure
func LogStepFailed(logger *slog.Logger, stepName string, workflowID string, duration time.Duration, err error) {
	logger.Error("Workflow step failed",
		"step", stepName,
		"workflow_id", workflowID,
		"duration", duration,
		"error", err)
}

// LogWorkflowStart logs the beginning of workflow execution
func LogWorkflowStart(logger *slog.Logger, workflowID string, totalSteps int, repoURL, repoPath, branch string) {
	logger.Info("Starting sequential containerization workflow",
		"workflow_id", workflowID,
		"steps_count", totalSteps,
		"repo_url", repoURL,
		"repo_path", repoPath,
		"branch", branch)
}

// LogWorkflowComplete logs workflow completion
func LogWorkflowComplete(logger *slog.Logger, workflowID string, duration time.Duration, success bool, stepsExecuted int) {
	logger.Info("Sequential workflow completed",
		"workflow_id", workflowID,
		"success", success,
		"duration", duration,
		"steps_executed", stepsExecuted)
}
