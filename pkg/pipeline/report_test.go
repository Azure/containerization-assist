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

func TestWriteReport(t *testing.T) {
	tmpDir := t.TempDir()

	state := &PipelineState{
		IterationCount: 3,
		Success:        true,
		RegistryURL:    "localhost:5000",
		ImageName:      "test-image",
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

	if err := WriteReport(ctx, state, tmpDir); err != nil {
		t.Fatalf("WriteReport returned error: %v", err)
	}

	jsonFile := filepath.Join(tmpDir, ReportDirectory, "run_report.json")
	if _, err := os.Stat(jsonFile); os.IsNotExist(err) {
		t.Errorf("JSON report file does not exist at path: %s", jsonFile)
	}

	mdFile := filepath.Join(tmpDir, ReportDirectory, "report.md")
	if _, err := os.Stat(mdFile); os.IsNotExist(err) {
		t.Errorf("Markdown report file does not exist at path: %s", mdFile)
	}

	validateReportFiles(t, jsonFile, mdFile, state)
}

func validateReportFiles(t *testing.T, jsonFile, mdFile string, state *PipelineState) {
	data, err := os.ReadFile(jsonFile)
	if err != nil {
		t.Fatalf("Failed to read JSON report at %s: %v", jsonFile, err)
	}

	var report RunReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("Failed to unmarshal JSON report: %v", err)
	}

	if report.IterationCount != state.IterationCount {
		t.Errorf("Expected iteration count %d, got %d", state.IterationCount, report.IterationCount)
	}
	if report.Outcome != RunOutcomeSuccess {
		t.Errorf("Expected outcome %q, got %q", RunOutcomeSuccess, report.Outcome)
	}
	if len(report.StageHistory) != len(state.StageHistory) {
		t.Errorf("Expected stage history length %d, got %d", len(state.StageHistory), len(report.StageHistory))
	} else {
		if len(report.StageHistory) > 0 {
			if report.StageHistory[0].StageID != state.StageHistory[0].StageID {
				t.Errorf("Expected StageHistory[0].StageID %s, got %s", state.StageHistory[0].StageID, report.StageHistory[0].StageID)
			}
			if report.StageHistory[0].Outcome != state.StageHistory[0].Outcome {
				t.Errorf("Expected StageHistory[0].Outcome %s, got %s", state.StageHistory[0].Outcome, report.StageHistory[0].Outcome)
			}
			if report.StageHistory[0].RetryCount != state.StageHistory[0].RetryCount {
				t.Errorf("Expected StageHistory[0].RetryCount %d, got %d", state.StageHistory[0].RetryCount, report.StageHistory[0].RetryCount)
			}
		}
	}

	mdData, err := os.ReadFile(mdFile)
	if err != nil {
		t.Fatalf("Failed to read Markdown report at %s: %v", mdFile, err)
	}
	mdContent := string(mdData)

	assertContains := func(substr, desc string) {
		if !strings.Contains(mdContent, substr) {
			t.Errorf("Missing %s in Markdown: expected to contain %q", desc, substr)
		}
	}

	assertContains(fmt.Sprintf("**Total Iterations:** %d", state.IterationCount), "iteration count")
	assertContains(fmt.Sprintf("**Outcome:** %s", RunOutcomeSuccess), "outcome")
	assertContains("## Stage History", "Stage History section")

	if len(state.StageHistory) > 0 {
		stage := state.StageHistory[0]
		stageRow := fmt.Sprintf("| %s | %d | %s |", stage.StageID, stage.RetryCount, stage.Outcome)
		assertContains(stageRow, "stage history table row")
	}

	assertContains("## Token Usage", "Token Usage section")
	assertContains(fmt.Sprintf("Prompt Tokens: %d", state.TokenUsage.PromptTokens), "prompt tokens")
	assertContains(fmt.Sprintf("Completion Tokens: %d", state.TokenUsage.CompletionTokens), "completion tokens")
	assertContains(fmt.Sprintf("Total Tokens: %d", state.TokenUsage.TotalTokens), "total tokens")
}
