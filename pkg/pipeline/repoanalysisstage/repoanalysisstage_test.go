package repoanalysisstage

import (
	"context"
	"strings"
	"testing"

	"github.com/Azure/container-kit/pkg/pipeline"
)

// TestFormatFileOperationLogs tests the FormatFileOperationLogs function
func TestFormatFileOperationLogs(t *testing.T) {
	// Test empty logs
	logs := []string{}
	result := FormatFileOperationLogs(logs)
	if result != "No file operations detected." {
		t.Errorf("FormatFileOperationLogs should return 'No file operations detected.' for empty logs, got: %s", result)
	}

	// Test mixed operations
	logs = []string{
		"📄 LLM reading file: /path/to/file.txt",
		"📂 LLM listing directory: /path/to/dir",
		"🔍 LLM checking if file exists: /path/to/check.txt",
	}
	result = FormatFileOperationLogs(logs)

	// Check if all operations are present in the result
	if !strings.Contains(result, "Files Read (1)") ||
		!strings.Contains(result, "/path/to/file.txt") ||
		!strings.Contains(result, "Directories Listed (1)") ||
		!strings.Contains(result, "/path/to/dir") ||
		!strings.Contains(result, "Files Checked (1)") ||
		!strings.Contains(result, "/path/to/check.txt") {
		t.Errorf("FormatFileOperationLogs did not format all operations correctly")
	}
}

// TestRepoAnalysisPipeline_Initialize tests the Initialize method
func TestRepoAnalysisPipeline_Initialize(t *testing.T) {
	// Create a test pipeline
	p := &RepoAnalysisStage{
		AIClient: nil,
	}

	// Create a test state
	state := &pipeline.PipelineState{}

	// Test initializing
	err := p.Initialize(context.Background(), state, "/test/path")
	if err != nil {
		t.Errorf("Initialize should not return an error, got: %v", err)
	}
}

// TestRepoAnalysisPipeline_Generate tests the Generate method
func TestRepoAnalysisPipeline_Generate(t *testing.T) {
	// Create a test pipeline
	p := &RepoAnalysisStage{
		AIClient: nil,
	}

	// Create a test state
	state := &pipeline.PipelineState{}

	// Test generating
	err := p.Generate(context.Background(), state, "/test/path")
	if err != nil {
		t.Errorf("Generate should not return an error, got: %v", err)
	}
}

// TestRepoAnalysisPipeline_GetErrors tests the GetErrors method
func TestRepoAnalysisPipeline_GetErrors(t *testing.T) {
	// Create a test pipeline
	p := &RepoAnalysisStage{}

	// Test with no errors
	state := &pipeline.PipelineState{
		Metadata: make(map[pipeline.MetadataKey]any),
	}
	errors := p.GetErrors(state)
	if errors != "" {
		t.Errorf("GetErrors should return empty string when no errors, got: %s", errors)
	}

	// Test with errors
	state.Metadata[pipeline.RepoAnalysisErrorKey] = "test analysis error"
	errors = p.GetErrors(state)
	if errors != "test analysis error" {
		t.Errorf("GetErrors should return the error message, expected: 'test analysis error', got: %s", errors)
	}
}

// TestRepoAnalysisPipeline_WriteSuccessfulFiles tests the WriteSuccessfulFiles method
func TestRepoAnalysisPipeline_WriteSuccessfulFiles(t *testing.T) {
	// Create a test pipeline
	p := &RepoAnalysisStage{}

	// Create a test state
	state := &pipeline.PipelineState{}

	// Test writing files
	err := p.WriteSuccessfulFiles(state)
	if err != nil {
		t.Errorf("WriteSuccessfulFiles should not return an error, got: %v", err)
	}
}

// TestRepoAnalysisPipeline_Deploy tests the Deploy method
func TestRepoAnalysisPipeline_Deploy(t *testing.T) {
	// Create a test pipeline
	p := &RepoAnalysisStage{}

	// Create a test state with analysis results
	state := &pipeline.PipelineState{
		Metadata: map[pipeline.MetadataKey]any{
			pipeline.RepoAnalysisResultKey: "Analysis result",
			pipeline.RepoAnalysisCallsKey:  "File operations",
		},
	}

	// Test deploying
	err := p.Deploy(context.Background(), state, nil)
	if err != nil {
		t.Errorf("Deploy should not return an error, got: %v", err)
	}
}

// TestRepoAnalysisPipeline_Run is a basic test for the Run method
func TestRepoAnalysisPipeline_Run(t *testing.T) {
	t.Skip("Skipping test that would require AI service")
}

// TestAnalyzeRepositoryWithFileAccess is a basic test for the AnalyzeRepositoryWithFileAccess function
func TestAnalyzeRepositoryWithFileAccess(t *testing.T) {
	t.Skip("Skipping test that would require AI service")
}
