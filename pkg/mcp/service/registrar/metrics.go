// Package registrar handles tool and prompt registration
package registrar

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// MetricsCollector collects essential retry and performance metrics
type MetricsCollector struct {
	mu      sync.RWMutex
	metrics map[string]*ToolMetrics
}

// ToolMetrics holds essential metrics for a specific tool
type ToolMetrics struct {
	TotalCalls      int64
	SuccessfulCalls int64
	FailedCalls     int64
	RetryAttempts   int64
	LastError       string
	LastErrorTime   time.Time
	LastCallTime    time.Time
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics: make(map[string]*ToolMetrics),
	}
}

// RecordCall records a tool call
func (mc *MetricsCollector) RecordCall(toolName string, success bool, retryNumber int) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	metrics := mc.getOrCreateMetrics(toolName)
	metrics.TotalCalls++
	metrics.LastCallTime = time.Now()

	if success {
		metrics.SuccessfulCalls++
	} else {
		metrics.FailedCalls++
	}

	if retryNumber > 1 {
		metrics.RetryAttempts++
	}
}

// RecordError records an error occurrence
func (mc *MetricsCollector) RecordError(toolName string, errorMessage string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	metrics := mc.getOrCreateMetrics(toolName)
	metrics.LastError = errorMessage
	metrics.LastErrorTime = time.Now()
}

// GetMetrics returns metrics for a specific tool
func (mc *MetricsCollector) GetMetrics(toolName string) *ToolMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	if metrics, exists := mc.metrics[toolName]; exists {
		// Return a copy to prevent modification
		copy := *metrics
		return &copy
	}
	return nil
}

// GetAllMetrics returns metrics for all tools
func (mc *MetricsCollector) GetAllMetrics() map[string]*ToolMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := make(map[string]*ToolMetrics)
	for name, metrics := range mc.metrics {
		copy := *metrics
		result[name] = &copy
	}
	return result
}

// GetSummary returns a simple summary of all metrics
func (mc *MetricsCollector) GetSummary() MetricsSummary {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	summary := MetricsSummary{
		ToolSummaries: make(map[string]ToolSummary),
	}

	for toolName, metrics := range mc.metrics {
		successRate := float64(0)
		if metrics.TotalCalls > 0 {
			successRate = float64(metrics.SuccessfulCalls) / float64(metrics.TotalCalls) * 100
		}

		retryRate := float64(0)
		if metrics.TotalCalls > 0 {
			retryRate = float64(metrics.RetryAttempts) / float64(metrics.TotalCalls) * 100
		}

		toolSummary := ToolSummary{
			TotalCalls:   metrics.TotalCalls,
			SuccessRate:  successRate,
			FailureRate:  100 - successRate,
			RetryRate:    retryRate,
			LastError:    metrics.LastError,
			LastCallTime: metrics.LastCallTime,
		}

		summary.ToolSummaries[toolName] = toolSummary
		summary.TotalCalls += metrics.TotalCalls
		summary.TotalRetries += metrics.RetryAttempts
	}

	// Calculate overall success rate
	totalSuccess := int64(0)
	for _, metrics := range mc.metrics {
		totalSuccess += metrics.SuccessfulCalls
	}

	if summary.TotalCalls > 0 {
		summary.OverallSuccessRate = float64(totalSuccess) / float64(summary.TotalCalls) * 100
		summary.OverallRetryRate = float64(summary.TotalRetries) / float64(summary.TotalCalls) * 100
	}

	return summary
}

// Reset clears all metrics
func (mc *MetricsCollector) Reset() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.metrics = make(map[string]*ToolMetrics)
}

// getOrCreateMetrics gets or creates metrics for a tool
func (mc *MetricsCollector) getOrCreateMetrics(toolName string) *ToolMetrics {
	if metrics, exists := mc.metrics[toolName]; exists {
		return metrics
	}

	metrics := &ToolMetrics{}
	mc.metrics[toolName] = metrics
	return metrics
}

// MetricsSummary provides a high-level summary
type MetricsSummary struct {
	TotalCalls         int64
	TotalRetries       int64
	OverallSuccessRate float64
	OverallRetryRate   float64
	ToolSummaries      map[string]ToolSummary
}

// ToolSummary provides summary for a specific tool
type ToolSummary struct {
	TotalCalls   int64
	SuccessRate  float64
	FailureRate  float64
	RetryRate    float64
	LastError    string
	LastCallTime time.Time
}

// Global metrics collector instance
var globalMetrics = NewMetricsCollector()

// GetGlobalMetrics returns the global metrics collector
func GetGlobalMetrics() *MetricsCollector {
	return globalMetrics
}

// MetricsMiddleware wraps a handler to collect metrics
func MetricsMiddleware(toolName string, handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract retry number
		retryNumber := 1
		if args := req.GetArguments(); args != nil {
			if rn, ok := args["retryNumber"].(float64); ok {
				retryNumber = int(rn)
			}
		}

		// Execute handler
		result, err := handler(ctx, req)

		// Record metrics
		success := err == nil && isSuccessResult(result)

		globalMetrics.RecordCall(toolName, success, retryNumber)

		if err != nil {
			globalMetrics.RecordError(toolName, err.Error())
		}

		return result, err
	}
}

// isSuccessResult checks if a result indicates success
func isSuccessResult(result *mcp.CallToolResult) bool {
	if result == nil {
		return false
	}

	// Parse result to check success field
	for _, content := range result.Content {
		if textContent, ok := content.(mcp.TextContent); ok {
			return strings.Contains(textContent.Text, `"success":true`)
		}
	}

	return false
}
