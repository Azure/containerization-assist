// Package metrics provides metrics middleware for LLM operations
package metrics

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/ai_ml/sampling"
)

// MetricsClient wraps a sampling client with comprehensive metrics collection
type MetricsClient struct {
	client  *sampling.Client
	metrics *LLMMetrics
	logger  *slog.Logger
}

// NewMetricsClient creates a new sampling client with metrics collection
func NewMetricsClient(client *sampling.Client, llmMetrics *LLMMetrics, logger *slog.Logger) *MetricsClient {
	return &MetricsClient{
		client:  client,
		metrics: llmMetrics,
		logger:  logger.With("component", "sampling-metrics"),
	}
}

// SampleInternal performs sampling with comprehensive metrics collection
func (m *MetricsClient) SampleInternal(ctx context.Context, req sampling.SamplingRequest) (*sampling.SamplingResponse, error) {
	start := time.Now()

	// Extract context information for metrics
	workflowID := sampling.GetWorkflowIDFromContext(ctx)
	stepName := sampling.GetStepNameFromContext(ctx)

	// Prepare metrics
	requestMetrics := LLMRequestMetrics{
		WorkflowID:   workflowID,
		StepName:     stepName,
		PromptLength: len(req.Prompt),
		Temperature:  req.Temperature,
		MaxTokens:    req.MaxTokens,
		RetryAttempt: 1, // Will be updated for retries
	}

	// Call the underlying client
	resp, err := m.client.SampleInternal(ctx, req)

	// Calculate final metrics
	duration := time.Since(start)
	requestMetrics.Duration = duration
	requestMetrics.Success = err == nil

	if err != nil {
		// Categorize error type for better metrics
		errorType := m.categorizeError(err)
		requestMetrics.ErrorType = errorType

		// Record error metrics
		m.metrics.RecordError(ctx, workflowID, stepName, "mcp-sampling", errorType)

		m.logger.Debug("LLM request failed with metrics",
			"workflow_id", workflowID,
			"step", stepName,
			"duration", duration,
			"error_type", errorType,
			"prompt_length", requestMetrics.PromptLength)
	} else {
		// Record success metrics
		requestMetrics.ResponseLength = len(resp.Content)
		requestMetrics.TokensUsed = resp.TokensUsed
		requestMetrics.Model = resp.Model

		// Calculate tokens per second
		if duration.Seconds() > 0 {
			requestMetrics.TokensPerSecond = float64(resp.TokensUsed) / duration.Seconds()
		}

		// Record detailed token usage
		promptTokens := sampling.EstimateTokenCount(req.Prompt)
		m.metrics.RecordTokenUsage(ctx, workflowID, stepName, resp.Model, promptTokens, resp.TokensUsed)

		m.logger.Debug("LLM request succeeded with metrics",
			"workflow_id", workflowID,
			"step", stepName,
			"duration", duration,
			"tokens_used", resp.TokensUsed,
			"tokens_per_second", requestMetrics.TokensPerSecond,
			"model", resp.Model,
			"response_length", requestMetrics.ResponseLength)
	}

	// Record overall request metrics
	m.metrics.RecordRequest(ctx, requestMetrics)

	return resp, err
}

// AnalyzeError performs error analysis with metrics collection
func (m *MetricsClient) AnalyzeError(ctx context.Context, inputErr error, contextInfo string) (*sampling.ErrorAnalysis, error) {
	start := time.Now()

	workflowID := sampling.GetWorkflowIDFromContext(ctx)
	stepName := "error_analysis"

	// Call the underlying client
	analysis, err := m.client.AnalyzeError(ctx, inputErr, contextInfo)

	duration := time.Since(start)

	// Prepare metrics for error analysis
	requestMetrics := LLMRequestMetrics{
		WorkflowID:   workflowID,
		StepName:     stepName,
		PromptLength: len(inputErr.Error()) + len(contextInfo),
		Duration:     duration,
		Success:      err == nil,
		RetryAttempt: 1,
		Model:        "error-analysis",
		Temperature:  0.1, // Error analysis typically uses low temperature
	}

	if err != nil {
		errorType := m.categorizeError(err)
		requestMetrics.ErrorType = errorType
		m.metrics.RecordError(ctx, workflowID, stepName, "error-analysis", errorType)
	} else if analysis != nil {
		// Estimate response content length
		responseLength := len(analysis.RootCause) + len(analysis.Fix)
		for _, step := range analysis.FixSteps {
			responseLength += len(step)
		}
		requestMetrics.ResponseLength = responseLength
		requestMetrics.TokensUsed = sampling.EstimateTokenCount(analysis.RootCause + analysis.Fix)

		if duration.Seconds() > 0 {
			requestMetrics.TokensPerSecond = float64(requestMetrics.TokensUsed) / duration.Seconds()
		}
	}

	// Record metrics
	m.metrics.RecordRequest(ctx, requestMetrics)

	m.logger.Debug("Error analysis completed with metrics",
		"workflow_id", workflowID,
		"duration", duration,
		"success", err == nil,
		"input_error_type", m.categorizeError(inputErr))

	return analysis, err
}

// SetTokenBudget delegates to the underlying client
func (m *MetricsClient) SetTokenBudget(budget int) {
	m.client.SetTokenBudget(budget)
}

// GetTokenBudget delegates to the underlying client
func (m *MetricsClient) GetTokenBudget() int {
	return m.client.GetTokenBudget()
}

// categorizeError categorizes errors for better metrics
func (m *MetricsClient) categorizeError(err error) string {
	if err == nil {
		return "none"
	}

	errStr := err.Error()

	// Network and connectivity errors
	if sampling.IsRetryable(err) {
		return "retryable"
	}

	// Rate limiting
	if sampling.Contains(errStr, "rate limit") || sampling.Contains(errStr, "quota") {
		return "rate_limit"
	}

	// Authentication errors
	if sampling.Contains(errStr, "unauthorized") || sampling.Contains(errStr, "authentication") || sampling.Contains(errStr, "auth") {
		return "auth"
	}

	// Content policy violations
	if sampling.Contains(errStr, "content policy") || sampling.Contains(errStr, "safety") || sampling.Contains(errStr, "inappropriate") {
		return "content_policy"
	}

	// Token limit errors
	if sampling.Contains(errStr, "token") && (sampling.Contains(errStr, "limit") || sampling.Contains(errStr, "maximum")) {
		return "token_limit"
	}

	// Timeout errors
	if sampling.Contains(errStr, "timeout") || sampling.Contains(errStr, "deadline") {
		return "timeout"
	}

	// Invalid request format
	if sampling.Contains(errStr, "invalid") || sampling.Contains(errStr, "bad request") || sampling.Contains(errStr, "malformed") {
		return "invalid_request"
	}

	// Model errors
	if sampling.Contains(errStr, "model") && (sampling.Contains(errStr, "not found") || sampling.Contains(errStr, "unavailable")) {
		return "model_unavailable"
	}

	// Server errors
	if sampling.Contains(errStr, "internal server error") || sampling.Contains(errStr, "server error") {
		return "server_error"
	}

	// MCP-specific errors
	if sampling.Contains(errStr, "MCP") || sampling.Contains(errStr, "sampling") {
		return "mcp_error"
	}

	// Generic classification
	return "unknown"
}
