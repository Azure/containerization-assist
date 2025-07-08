// Package knowledge provides shared knowledge base functionality for cross-tool insights
package knowledge

import (
	"time"
)

// ToolInsights represents insights gathered from a tool
type ToolInsights struct {
	ToolName           string                   `json:"tool_name"`
	FailurePattern     *FailurePattern          `json:"failure_pattern,omitempty"`
	Timestamp          time.Time                `json:"timestamp"`
	OptimizationTips   []GeneralOptimizationTip `json:"optimization_tips,omitempty"`
	PerformanceMetrics *AggregatedMetrics       `json:"performance_metrics,omitempty"`
	Data               map[string]interface{}   `json:"data,omitempty"`
}

// FailurePattern represents a pattern of failure
type FailurePattern struct {
	PatternName      string                 `json:"pattern_name"`
	Pattern          string                 `json:"pattern"`
	FailureType      string                 `json:"failure_type"`
	Frequency        int                    `json:"frequency"`
	CommonCauses     []string               `json:"common_causes,omitempty"`
	TypicalSolutions []string               `json:"typical_solutions,omitempty"`
	Context          map[string]interface{} `json:"context,omitempty"`
}

// SharedKnowledge represents accumulated knowledge from tools
type SharedKnowledge struct {
	Domain           string                 `json:"domain"`
	CommonPatterns   []interface{}          `json:"common_patterns"`
	BestPractices    []interface{}          `json:"best_practices"`
	OptimizationTips []interface{}          `json:"optimization_tips"`
	SuccessMetrics   map[string]interface{} `json:"success_metrics"`
	LastUpdated      time.Time              `json:"last_updated"`
	SourceTools      []string               `json:"source_tools"`
	Data             map[string]interface{} `json:"data"`
}

// GeneralOptimizationTip represents an optimization tip
type GeneralOptimizationTip struct {
	Title         string `json:"title"`
	Tip           string `json:"tip"`
	Description   string `json:"description"`
	Category      string `json:"category"`
	Impact        string `json:"impact"`
	Difficulty    string `json:"difficulty"`
	Applicability string `json:"applicability"`
}

// AnalysisRequest represents a request for analysis
type AnalysisRequest struct {
	SessionID string                 `json:"session_id"`
	Type      string                 `json:"type"`
	Data      map[string]interface{} `json:"data"`
	Error     error                  `json:"error,omitempty"`
}

// RelatedFailure represents a failure related to the current context
type RelatedFailure struct {
	Type         string    `json:"type"`
	FailureType  string    `json:"failure_type"`
	Message      string    `json:"message"`
	Similarity   float64   `json:"similarity"`
	Resolution   string    `json:"resolution"`
	LastOccurred time.Time `json:"last_occurred"`
	Frequency    int       `json:"frequency"`
}

// AggregatedMetrics represents aggregated performance metrics
type AggregatedMetrics struct {
	TotalRequests   int64           `json:"total_requests"`
	TotalOperations int64           `json:"total_operations"`
	SuccessRate     float64         `json:"success_rate"`
	AvgDuration     time.Duration   `json:"avg_duration"`
	AverageTime     time.Duration   `json:"average_time"`
	ErrorRate       float64         `json:"error_rate"`
	P95Duration     time.Duration   `json:"p95_duration"`
	P99Duration     time.Duration   `json:"p99_duration"`
	ResourceUsage   ResourceMetrics `json:"resource_usage"`
}

// ResourceMetrics represents resource usage metrics
type ResourceMetrics struct {
	CPUUsage      float64 `json:"cpu_usage"`
	MemoryUsage   int64   `json:"memory_usage"`
	DiskIORead    int64   `json:"disk_io_read"`
	DiskIOWrite   int64   `json:"disk_io_write"`
	NetworkIOSent int64   `json:"network_io_sent"`
	NetworkIORecv int64   `json:"network_io_recv"`
}
