package report

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/pipeline"
)

// LoadOrCreateMCPReport loads an existing MCP report or creates a new one
func LoadOrCreateMCPReport(workflowID, targetDir string) (*MCPProgressiveReport, error) {
	// For MCP mode, we don't read from disk - we maintain state in memory only
	// This avoids file system operations and returns content via MCP responses

	// Create new report (previous state would be maintained by the calling workflow)

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

// SaveMCPReport prepares the MCP report content for response instead of writing to disk
func SaveMCPReport(report *MCPProgressiveReport, targetDir string) error {
	// In MCP mode, we don't write to disk - content is returned in MCP responses
	// This avoids file system operations and provides content directly to the client

	// Add instructions for user about report content
	if report.Metadata == nil {
		report.Metadata = make(map[string]interface{})
	}

	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling report: %w", err)
	}

	markdownContent := FormatMCPMarkdownReport(report)

	// Store structured file content for MCP client to handle
	report.Metadata["files"] = map[string]interface{}{
		"mcp_report.json": map[string]interface{}{
			"path":        ".container-kit/mcp_report.json",
			"content":     string(jsonData),
			"type":        "application/json",
			"description": "Complete MCP workflow report in JSON format",
		},
		"mcp_report.md": map[string]interface{}{
			"path":        ".container-kit/mcp_report.md",
			"content":     markdownContent,
			"type":        "text/markdown",
			"description": "Human-readable MCP workflow report in Markdown format",
		},
	}
	report.Metadata["instructions"] = "Report files are provided in this response. The MCP client can offer to create these files in your project."

	return nil
}
