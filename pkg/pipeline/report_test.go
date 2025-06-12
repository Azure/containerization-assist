package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/container-copilot/pkg/ai"
)

// Dummy TokenUsage for testing.
type DummyTokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Dummy implementations for PipelineState and StageVisit.
// (In your actual code, these types are already defined.)
func TestWriteReport(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a dummy PipelineState for testing.
	state := &PipelineState{
		IterationCount: 3,
		Success:        true,
		RegistryURL:    "localhost:5000",
		ImageName:      "test-image",
		// Assuming TokenUsage is defined similarly in your code.
		TokenUsage: ai.TokenUsage{
			CompletionTokens: 50,
			PromptTokens:     100,
			TotalTokens:      150,
		},
		StageHistory: []StageVisit{
			{
				StageID:    "testStage",
				RetryCount: 1,
				Outcome:    StageOutcomeSuccess,
			},
		},
	}
	ctx := context.Background()

	// Call WriteReport which should write the JSON and Markdown reports.
	if err := WriteReport(ctx, state, tmpDir); err != nil {
		t.Fatalf("WriteReport returned error: %v", err)
	}

	// Verify JSON report exists and contains expected content.
	jsonFile := filepath.Join(tmpDir, ReportDirectory, "run_report.json")
	if _, err := os.Stat(jsonFile); os.IsNotExist(err) {
		t.Errorf("JSON report file does not exist at path: %s", jsonFile)
	}

	jsonData, err := os.ReadFile(jsonFile)
	if err != nil {
		t.Errorf("Error reading JSON report file: %v", err)
	}

	var report RunReport
	if err := json.Unmarshal(jsonData, &report); err != nil {
		t.Errorf("Error unmarshalling JSON report: %v", err)
	}

	if report.IterationCount != state.IterationCount {
		t.Errorf("Expected iteration count %d, got %d", state.IterationCount, report.IterationCount)
	}

	// Verify Markdown report exists and contains expected information.
	mdFile := filepath.Join(tmpDir, ReportDirectory, "report.md")
	if _, err := os.Stat(mdFile); os.IsNotExist(err) {
		t.Errorf("Markdown report file does not exist at path: %s", mdFile)
	}

	mdData, err := os.ReadFile(mdFile)
	if err != nil {
		t.Errorf("Error reading Markdown report file: %v", err)
	}
	mdContent := string(mdData)
	if !strings.Contains(mdContent, fmt.Sprintf("**Total Iterations:** %d", state.IterationCount)) {
		t.Errorf("Markdown report missing iteration info; got content: %s", mdContent)
	}
}
