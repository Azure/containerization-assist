package observability

import (
	"context"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/rs/zerolog"
)

func TestTelemetryManager_RecordLLMTokenUsage(t *testing.T) {
	// Create telemetry manager
	config := TelemetryConfig{
		P95Target:        2 * time.Second,
		Logger:           zerolog.New(zerolog.NewTestWriter(t)),
		EnableAutoExport: false,
	}
	tm := NewTelemetryManager(config)
	defer func() {
		if err := tm.Shutdown(context.Background()); err != nil {
			t.Errorf("Failed to shutdown telemetry manager: %v", err)
		}
	}()

	tests := []struct {
		name             string
		tool             string
		model            string
		promptTokens     int
		completionTokens int
		expectedPrompt   float64
		expectedComplete float64
		expectedTotal    float64
	}{
		{
			name:             "record tokens for chat tool",
			tool:             "chat",
			model:            "gpt-4",
			promptTokens:     100,
			completionTokens: 50,
			expectedPrompt:   100,
			expectedComplete: 50,
			expectedTotal:    150,
		},
		{
			name:             "record tokens for analysis tool",
			tool:             "analyze_repository",
			model:            "claude-3",
			promptTokens:     250,
			completionTokens: 150,
			expectedPrompt:   250,
			expectedComplete: 150,
			expectedTotal:    400,
		},
		{
			name:             "zero tokens",
			tool:             "test",
			model:            "test-model",
			promptTokens:     0,
			completionTokens: 0,
			expectedPrompt:   0,
			expectedComplete: 0,
			expectedTotal:    0,
		},
		{
			name:             "only prompt tokens",
			tool:             "generate_dockerfile",
			model:            "gpt-4",
			promptTokens:     75,
			completionTokens: 0,
			expectedPrompt:   75,
			expectedComplete: 0,
			expectedTotal:    75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Record token usage
			tm.RecordLLMTokenUsage(tt.tool, tt.model, tt.promptTokens, tt.completionTokens)

			// Check prompt tokens counter
			promptCounter, err := tm.promptTokens.GetMetricWithLabelValues(tt.tool, tt.model)
			if err != nil {
				t.Fatal(err)
			}

			promptDto := &dto.Metric{}
			if err := promptCounter.Write(promptDto); err != nil {
				t.Errorf("Failed to write prompt DTO: %v", err)
			}
			if promptDto.Counter.GetValue() != tt.expectedPrompt {
				t.Errorf("Expected prompt tokens %f, got %f", tt.expectedPrompt, promptDto.Counter.GetValue())
			}

			// Check completion tokens counter
			completionCounter, err := tm.completionTokens.GetMetricWithLabelValues(tt.tool, tt.model)
			if err != nil {
				t.Fatal(err)
			}

			completionDto := &dto.Metric{}
			if err := completionCounter.Write(completionDto); err != nil {
				t.Errorf("Failed to write completion DTO: %v", err)
			}
			if completionDto.Counter.GetValue() != tt.expectedComplete {
				t.Errorf("Expected completion tokens %f, got %f", tt.expectedComplete, completionDto.Counter.GetValue())
			}

			// Check legacy total tokens counter
			totalCounter, err := tm.tokenUsage.GetMetricWithLabelValues(tt.tool)
			if err != nil {
				t.Fatal(err)
			}

			totalDto := &dto.Metric{}
			if err := totalCounter.Write(totalDto); err != nil {
				t.Errorf("Failed to write total DTO: %v", err)
			}
			if totalDto.Counter.GetValue() != tt.expectedTotal {
				t.Errorf("Expected total tokens %f, got %f", tt.expectedTotal, totalDto.Counter.GetValue())
			}
		})
	}
}

func TestTelemetryManager_TokenMetricsLabels(t *testing.T) {
	// Create telemetry manager
	config := TelemetryConfig{
		Logger:           zerolog.New(zerolog.NewTestWriter(t)),
		EnableAutoExport: false,
	}
	tm := NewTelemetryManager(config)
	defer func() {
		if err := tm.Shutdown(context.Background()); err != nil {
			t.Errorf("Failed to shutdown telemetry manager: %v", err)
		}
	}()

	// Record usage for different tools and models
	tm.RecordLLMTokenUsage("chat", "gpt-4", 100, 50)
	tm.RecordLLMTokenUsage("chat", "claude-3", 200, 100)
	tm.RecordLLMTokenUsage("analyze", "gpt-4", 150, 75)

	// Verify the metrics have correct labels
	// For prompt tokens
	promptMetrics, err := tm.registry.Gather()
	if err != nil {
		t.Fatal(err)
	}

	var foundPromptMetric bool
	for _, mf := range promptMetrics {
		if mf.GetName() == "llm_prompt_tokens_total" {
			foundPromptMetric = true

			// Should have 3 metric instances (3 unique tool/model combinations)
			if len(mf.GetMetric()) != 3 {
				t.Errorf("Expected 3 prompt token metrics, got %d", len(mf.GetMetric()))
			}

			// Verify labels exist
			for _, m := range mf.GetMetric() {
				labels := m.GetLabel()
				if len(labels) != 2 {
					t.Errorf("Expected 2 labels (tool, model), got %d", len(labels))
				}

				var hasToolLabel, hasModelLabel bool
				for _, label := range labels {
					if label.GetName() == "tool" {
						hasToolLabel = true
					}
					if label.GetName() == "model" {
						hasModelLabel = true
					}
				}

				if !hasToolLabel || !hasModelLabel {
					t.Error("Missing required labels")
				}
			}
		}
	}

	if !foundPromptMetric {
		t.Error("llm_prompt_tokens_total metric not found")
	}
}

func TestTelemetryManager_ConcurrentTokenRecording(t *testing.T) {
	// Create telemetry manager
	config := TelemetryConfig{
		Logger:           zerolog.New(zerolog.NewTestWriter(t)),
		EnableAutoExport: false,
	}
	tm := NewTelemetryManager(config)
	defer func() {
		if err := tm.Shutdown(context.Background()); err != nil {
			t.Errorf("Failed to shutdown telemetry manager: %v", err)
		}
	}()

	// Record tokens concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(i int) {
			tool := "tool" + string(rune('0'+i%3))
			model := "model" + string(rune('0'+i%2))
			tm.RecordLLMTokenUsage(tool, model, 10*(i+1), 5*(i+1))
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify no panics and metrics are recorded
	metrics, err := tm.registry.Gather()
	if err != nil {
		t.Fatal(err)
	}

	// Should have prompt and completion token metrics
	var hasPromptMetrics, hasCompletionMetrics bool
	for _, mf := range metrics {
		if mf.GetName() == "llm_prompt_tokens_total" {
			hasPromptMetrics = true
		}
		if mf.GetName() == "llm_completion_tokens_total" {
			hasCompletionMetrics = true
		}
	}

	if !hasPromptMetrics {
		t.Error("Missing prompt token metrics after concurrent recording")
	}
	if !hasCompletionMetrics {
		t.Error("Missing completion token metrics after concurrent recording")
	}
}
