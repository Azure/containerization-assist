// Package workflow provides comprehensive metrics collection for containerization workflows
package workflow

import (
	"time"
)

// ExtendedMetricsCollector provides comprehensive metrics collection for workflows
type ExtendedMetricsCollector interface {
	MetricsCollector

	// Workflow-level metrics
	RecordWorkflowStart(workflowID string)
	RecordWorkflowEnd(workflowID string, success bool, duration time.Duration)
	RecordWorkflowRetry(workflowID string, stepName string, attempt int)

	// Resource usage metrics
	RecordDockerBuildTime(duration time.Duration, imageSize int64)
	RecordImagePushTime(duration time.Duration, registryURL string)
	RecordKubernetesDeployTime(duration time.Duration, namespace string)

	// Error metrics
	RecordErrorByCategory(category string, stepName string)
	RecordAdaptationApplied(adaptationType AdaptationType, success bool)

	// Performance metrics
	RecordStepQueueTime(stepName string, duration time.Duration)
	RecordStepProcessingTime(stepName string, duration time.Duration)
	RecordConcurrentSteps(count int)

	// Business metrics
	RecordDeploymentSuccess(repoURL string, branch string)
	RecordDeploymentFailure(repoURL string, branch string, reason string)
	RecordScanVulnerabilities(critical, high, medium, low int)

	// Cache metrics
	RecordCacheHit(cacheType string)
	RecordCacheMiss(cacheType string)

	// AI/ML metrics
	RecordLLMTokenUsage(promptTokens, completionTokens int)
	RecordPatternRecognitionAccuracy(predicted, actual string, correct bool)
	RecordStepEnhancementImpact(stepName string, improvementPercent float64)

	// Get current metrics snapshot
	GetMetricsSnapshot() *MetricsSnapshot
}

// MetricsSnapshot represents a point-in-time view of all metrics
type MetricsSnapshot struct {
	// Workflow metrics
	TotalWorkflows      int64         `json:"total_workflows"`
	SuccessfulWorkflows int64         `json:"successful_workflows"`
	FailedWorkflows     int64         `json:"failed_workflows"`
	AvgWorkflowDuration time.Duration `json:"avg_workflow_duration"`

	// Step metrics
	StepMetrics map[string]*StepMetrics `json:"step_metrics"`

	// Error metrics
	ErrorsByCategory map[string]int64 `json:"errors_by_category"`
	TotalRetries     int64            `json:"total_retries"`

	// Resource metrics
	AvgDockerBuildTime time.Duration `json:"avg_docker_build_time"`
	AvgImageSize       int64         `json:"avg_image_size"`
	AvgDeployTime      time.Duration `json:"avg_deploy_time"`

	// Cache metrics
	CacheHitRate float64 `json:"cache_hit_rate"`

	// AI/ML metrics
	TotalTokensUsed      int64   `json:"total_tokens_used"`
	PatternAccuracy      float64 `json:"pattern_accuracy"`
	AvgEnhancementImpact float64 `json:"avg_enhancement_impact"`

	// Business metrics
	DeploymentSuccessRate float64          `json:"deployment_success_rate"`
	TopFailureReasons     map[string]int64 `json:"top_failure_reasons"`
	SecurityMetrics       *SecurityMetrics `json:"security_metrics"`

	// Timestamp
	Timestamp time.Time `json:"timestamp"`
}

// StepMetrics contains detailed metrics for a specific workflow step
type StepMetrics struct {
	ExecutionCount int64         `json:"execution_count"`
	SuccessCount   int64         `json:"success_count"`
	FailureCount   int64         `json:"failure_count"`
	AvgDuration    time.Duration `json:"avg_duration"`
	MinDuration    time.Duration `json:"min_duration"`
	MaxDuration    time.Duration `json:"max_duration"`
	P95Duration    time.Duration `json:"p95_duration"`
	P99Duration    time.Duration `json:"p99_duration"`
	RetryCount     int64         `json:"retry_count"`
	LastExecuted   time.Time     `json:"last_executed"`
}

// SecurityMetrics contains security scanning results
type SecurityMetrics struct {
	TotalScans       int64   `json:"total_scans"`
	CriticalVulns    int64   `json:"critical_vulnerabilities"`
	HighVulns        int64   `json:"high_vulnerabilities"`
	MediumVulns      int64   `json:"medium_vulnerabilities"`
	LowVulns         int64   `json:"low_vulnerabilities"`
	AvgVulnsPerImage float64 `json:"avg_vulns_per_image"`
}

// WorkflowMetricsCollector is a domain interface for collecting workflow-specific metrics
type WorkflowMetricsCollector interface {
	ExtendedMetricsCollector

	// Health metrics
	GetHealthStatus() HealthStatus
	RecordHealthCheck(status HealthStatus, details map[string]interface{})

	// Real-time metrics
	GetCurrentWorkflowCount() int
	GetCurrentStepExecutions() map[string]int
	GetRecentErrors(limit int) []ErrorEvent

	// Alerting support
	CheckThresholds() []Alert
	SetThreshold(metric string, threshold float64)
}

// HealthStatus represents the overall health of the workflow system
type HealthStatus struct {
	Status           string                 `json:"status"` // healthy, degraded, unhealthy
	WorkflowHealth   ComponentHealth        `json:"workflow_health"`
	DockerHealth     ComponentHealth        `json:"docker_health"`
	KubernetesHealth ComponentHealth        `json:"kubernetes_health"`
	AIHealth         ComponentHealth        `json:"ai_health"`
	Details          map[string]interface{} `json:"details"`
	LastChecked      time.Time              `json:"last_checked"`
}

// ComponentHealth represents health of a specific component
type ComponentHealth struct {
	Status       string        `json:"status"`
	Latency      time.Duration `json:"latency"`
	ErrorRate    float64       `json:"error_rate"`
	Availability float64       `json:"availability"`
}

// ErrorEvent represents a recent error occurrence
type ErrorEvent struct {
	Timestamp  time.Time              `json:"timestamp"`
	WorkflowID string                 `json:"workflow_id"`
	StepName   string                 `json:"step_name"`
	Category   string                 `json:"category"`
	Message    string                 `json:"message"`
	Details    map[string]interface{} `json:"details"`
}

// Alert represents a metric threshold violation
type Alert struct {
	Metric    string    `json:"metric"`
	Threshold float64   `json:"threshold"`
	Current   float64   `json:"current"`
	Severity  string    `json:"severity"` // warning, critical
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}
