// Package metrics provides LLM-specific metrics collection and observability
package metrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// LLMMetrics provides comprehensive metrics collection for LLM operations
type LLMMetrics struct {
	// Prometheus metrics
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	tokensTotal     *prometheus.CounterVec
	errorRate       *prometheus.CounterVec
	retryCount      *prometheus.CounterVec
	tokensPerSecond *prometheus.HistogramVec
	promptLength    *prometheus.HistogramVec
	responseLength  *prometheus.HistogramVec
}

// LLMRequestMetrics contains metrics for a single LLM request
type LLMRequestMetrics struct {
	WorkflowID      string
	StepName        string
	PromptLength    int
	ResponseLength  int
	TokensUsed      int
	Duration        time.Duration
	Success         bool
	RetryAttempt    int
	ErrorType       string
	Model           string
	TokensPerSecond float64
	Temperature     float32
	MaxTokens       int32
}

// NewLLMMetrics creates a new LLM metrics collector
func NewLLMMetrics() (*LLMMetrics, error) {
	// Initialize Prometheus metrics
	requestsTotal := promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "llm_requests_total",
		Help: "Total number of LLM requests",
	}, []string{"workflow_id", "step", "model", "status"})

	requestDuration := promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "llm_request_duration_seconds",
		Help:    "Duration of LLM requests in seconds",
		Buckets: []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0, 60.0},
	}, []string{"workflow_id", "step", "model", "status"})

	tokensTotal := promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "llm_tokens_total",
		Help: "Total number of tokens processed",
	}, []string{"workflow_id", "step", "model", "type"}) // type: prompt, completion

	errorRate := promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "llm_errors_total",
		Help: "Total number of LLM errors",
	}, []string{"workflow_id", "step", "model", "error_type"})

	retryCount := promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "llm_retries_total",
		Help: "Total number of LLM retry attempts",
	}, []string{"workflow_id", "step", "model"})

	tokensPerSecond := promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "llm_tokens_per_second",
		Help:    "Tokens generated per second",
		Buckets: []float64{1, 5, 10, 20, 50, 100, 200, 500},
	}, []string{"workflow_id", "step", "model"})

	promptLength := promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "llm_prompt_length_chars",
		Help:    "Length of LLM prompts in characters",
		Buckets: []float64{100, 500, 1000, 2000, 5000, 10000, 20000, 50000},
	}, []string{"workflow_id", "step", "model"})

	responseLength := promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "llm_response_length_chars",
		Help:    "Length of LLM responses in characters",
		Buckets: []float64{100, 500, 1000, 2000, 5000, 10000, 20000},
	}, []string{"workflow_id", "step", "model"})

	return &LLMMetrics{
		requestsTotal:   requestsTotal,
		requestDuration: requestDuration,
		tokensTotal:     tokensTotal,
		errorRate:       errorRate,
		retryCount:      retryCount,
		tokensPerSecond: tokensPerSecond,
		promptLength:    promptLength,
		responseLength:  responseLength,
	}, nil
}

// RecordRequest records metrics for an LLM request
func (m *LLMMetrics) RecordRequest(ctx context.Context, metrics LLMRequestMetrics) {
	// Prepare labels
	labels := prometheus.Labels{
		"workflow_id": metrics.WorkflowID,
		"step":        metrics.StepName,
		"model":       metrics.Model,
		"status":      m.getStatusLabel(metrics.Success),
	}

	// Record Prometheus metrics
	m.requestsTotal.With(labels).Inc()
	m.requestDuration.With(labels).Observe(metrics.Duration.Seconds())
	m.promptLength.With(labels).Observe(float64(metrics.PromptLength))

	if metrics.Success {
		m.responseLength.With(labels).Observe(float64(metrics.ResponseLength))
		m.tokensPerSecond.With(labels).Observe(metrics.TokensPerSecond)

		// Record token usage
		tokenLabels := prometheus.Labels{
			"workflow_id": metrics.WorkflowID,
			"step":        metrics.StepName,
			"model":       metrics.Model,
			"type":        "completion",
		}
		m.tokensTotal.With(tokenLabels).Add(float64(metrics.TokensUsed))

		// Estimate prompt tokens and record
		promptTokens := metrics.PromptLength / 4 // Rough estimation
		promptTokenLabels := prometheus.Labels{
			"workflow_id": metrics.WorkflowID,
			"step":        metrics.StepName,
			"model":       metrics.Model,
			"type":        "prompt",
		}
		m.tokensTotal.With(promptTokenLabels).Add(float64(promptTokens))
	} else {
		// Record error
		errorLabels := prometheus.Labels{
			"workflow_id": metrics.WorkflowID,
			"step":        metrics.StepName,
			"model":       metrics.Model,
			"error_type":  metrics.ErrorType,
		}
		m.errorRate.With(errorLabels).Inc()
	}

	// Record retries if applicable
	if metrics.RetryAttempt > 1 {
		retryLabels := prometheus.Labels{
			"workflow_id": metrics.WorkflowID,
			"step":        metrics.StepName,
			"model":       metrics.Model,
		}
		m.retryCount.With(retryLabels).Add(float64(metrics.RetryAttempt - 1))
	}

}

// RecordTokenUsage records detailed token usage metrics
func (m *LLMMetrics) RecordTokenUsage(ctx context.Context, workflowID, stepName, model string, promptTokens, completionTokens int) {
	labels := prometheus.Labels{
		"workflow_id": workflowID,
		"step":        stepName,
		"model":       model,
	}

	// Record prompt tokens
	promptLabels := labels
	promptLabels["type"] = "prompt"
	m.tokensTotal.With(promptLabels).Add(float64(promptTokens))

	// Record completion tokens
	completionLabels := labels
	completionLabels["type"] = "completion"
	m.tokensTotal.With(completionLabels).Add(float64(completionTokens))

}

// RecordError records a specific error occurrence
func (m *LLMMetrics) RecordError(ctx context.Context, workflowID, stepName, model, errorType string) {
	labels := prometheus.Labels{
		"workflow_id": workflowID,
		"step":        stepName,
		"model":       model,
		"error_type":  errorType,
	}
	m.errorRate.With(labels).Inc()
}

// RecordRetry records a retry attempt
func (m *LLMMetrics) RecordRetry(ctx context.Context, workflowID, stepName, model string, attemptNumber int) {
	labels := prometheus.Labels{
		"workflow_id": workflowID,
		"step":        stepName,
		"model":       model,
	}
	m.retryCount.With(labels).Inc()
}

// getStatusLabel converts boolean success to string label
func (m *LLMMetrics) getStatusLabel(success bool) string {
	if success {
		return "success"
	}
	return "error"
}

// GetPrometheusRegistry returns the Prometheus metrics for registration
func (m *LLMMetrics) GetPrometheusRegistry() prometheus.Gatherer {
	return prometheus.DefaultGatherer
}
