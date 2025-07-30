package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/container-kit/pkg/pipeline"
)

// LoadOrCreateMCPReport loads an existing MCP report or creates a new one
func LoadOrCreateMCPReport(workflowID, targetDir string) (*MCPProgressiveReport, error) {
	reportPath := filepath.Join(targetDir, MCPReportDirectory, MCPReportFileName)

	// Try to load existing report
	if data, err := os.ReadFile(reportPath); err == nil {
		var report MCPProgressiveReport
		if err := json.Unmarshal(data, &report); err == nil {
			// Update last accessed time
			report.LastUpdated = time.Now()
			return &report, nil
		}
		// If unmarshaling fails, create new report
	}

	// Create new report
	now := time.Now()
	return &MCPProgressiveReport{
		WorkflowID:  workflowID,
		StartTime:   now,
		LastUpdated: now,
		Summary: MCPReportSummary{
			LastUpdated:    now,
			Outcome:        MCPOutcomeInProgress,
			IterationCount: 1,
		},
		StepResults:       []MCPStepResult{},
		DetectedDatabases: []pipeline.DatabaseDetectionResult{},
		TokenUsage:        MCPTokenUsage{},
		StageHistory:      []MCPStageVisit{},
		GeneratedFiles:    []GeneratedArtifact{},
		Metadata:          make(map[string]interface{}),
	}, nil
}

// SaveMCPReport saves the MCP report to disk in both JSON and Markdown formats
func SaveMCPReport(report *MCPProgressiveReport, targetDir string) error {
	reportDir := filepath.Join(targetDir, MCPReportDirectory)
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		return fmt.Errorf("creating report directory: %w", err)
	}

	// Save JSON report
	jsonPath := filepath.Join(reportDir, MCPReportFileName)
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling report: %w", err)
	}
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		return fmt.Errorf("writing JSON report: %w", err)
	}

	// Save Markdown report
	markdownPath := filepath.Join(reportDir, MCPMarkdownFileName)
	markdownContent := FormatMCPMarkdownReport(report)
	if err := os.WriteFile(markdownPath, []byte(markdownContent), 0644); err != nil {
		return fmt.Errorf("writing Markdown report: %w", err)
	}

	return nil
}
