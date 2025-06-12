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

	if report.Outcome != RunOutcomeSuccess {
		t.Errorf("Expected outcome %s, got %s", RunOutcomeSuccess, report.Outcome)
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

	if !strings.Contains(mdContent, fmt.Sprintf("**Outcome:** %s", RunOutcomeSuccess)) {
		t.Errorf("Markdown report missing or incorrect outcome info. Expected to contain '**Outcome:** %s'. Got content: %s", RunOutcomeSuccess, mdContent)
	}

	if !strings.Contains(mdContent, "## Stage History") {
		t.Errorf("Markdown report missing '## Stage History' section. Got content: %s", mdContent)
	}
	if len(state.StageHistory) > 0 {
		expectedStageMd := fmt.Sprintf("| %s | %d | %s |", state.StageHistory[0].StageID, state.StageHistory[0].RetryCount, state.StageHistory[0].Outcome)
		if !strings.Contains(mdContent, expectedStageMd) {
			t.Errorf("Markdown report missing or incorrect stage history entry. Expected to contain '%s'. Got content: %s", expectedStageMd, mdContent)
		}
	}
	if state.Success {
		if !strings.Contains(mdContent, "## Token Usage") {
			t.Errorf("Markdown report missing '## Token Usage' section for a successful run. Got content: %s", mdContent)
		}
		if !strings.Contains(mdContent, fmt.Sprintf("Prompt Tokens: %d", state.TokenUsage.PromptTokens)) {
			t.Errorf("Markdown report missing or incorrect prompt tokens. Got content: %s", mdContent)
		}
		if !strings.Contains(mdContent, fmt.Sprintf("Completion Tokens: %d", state.TokenUsage.CompletionTokens)) {
			t.Errorf("Markdown report missing or incorrect completion tokens. Got content: %s", mdContent)
		}
		if !strings.Contains(mdContent, fmt.Sprintf("Total Tokens: %d", state.TokenUsage.TotalTokens)) {
			t.Errorf("Markdown report missing or incorrect total tokens. Got content: %s", mdContent)
		}
	}
}
