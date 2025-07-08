package deploy

import (
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
)

// AtomicCheckHealthArgs defines arguments for atomic application health checking
type AtomicCheckHealthArgs struct {
	types.BaseToolArgs

	// Target specification
	Namespace     string `json:"namespace,omitempty" validate:"omitempty,namespace" description:"Kubernetes namespace (default: default)"`
	AppName       string `json:"app_name,omitempty" validate:"omitempty,k8s_name" description:"Application name for label selection"`
	LabelSelector string `json:"label_selector,omitempty" validate:"omitempty,k8s_selector" description:"Custom label selector (e.g., app=myapp,version=v1)"`

	// Health check configuration
	IncludeServices bool `json:"include_services,omitempty" description:"Include service health checks (default: true)"`
	IncludeEvents   bool `json:"include_events,omitempty" description:"Include pod events in analysis (default: true)"`
	WaitForReady    bool `json:"wait_for_ready,omitempty" description:"Wait for pods to become ready"`
	WaitTimeout     int  `json:"wait_timeout,omitempty" validate:"omitempty,min=30,max=3600" description:"Wait timeout in seconds (default: 300)"`

	// Analysis depth
	DetailedAnalysis bool `json:"detailed_analysis,omitempty" description:"Perform detailed container and condition analysis"`
	IncludeLogs      bool `json:"include_logs,omitempty" description:"Include recent container logs in analysis"`
	LogLines         int  `json:"log_lines,omitempty" validate:"omitempty,min=1,max=1000" description:"Number of log lines to include (default: 50)"`
}

// AtomicCheckHealthResult defines the response from atomic health checking
type AtomicCheckHealthResult struct {
	types.BaseToolResponse
	core.BaseAIContextResult      // Embed AI context methods
	Success                  bool `json:"success"`

	// Health status
	Namespace      string         `json:"namespace"`
	ApplicationURL string         `json:"application_url,omitempty"`
	HealthStatus   string         `json:"health_status"` // healthy, degraded, unhealthy, unknown
	OverallScore   int            `json:"overall_score"` // 0-100
	CheckedAt      string         `json:"checked_at"`
	Context        *HealthContext `json:"context,omitempty"`

	// Diagnostic information
	Summary         string `json:"summary"`
	Recommendations []string
	AnalysisDetails map[string]interface{} `json:"analysis_details"`

	// Pod and service status
	PodSummaries     []PodSummary     `json:"pod_summaries"`
	ServiceSummaries []ServiceSummary `json:"service_summaries"`

	// Issues and problems
	PodIssues       []PodIssue       `json:"pod_issues"`
	ContainerIssues []ContainerIssue `json:"container_issues"`

	// Performance and resource info
	ResourceUsage   *ResourceUsageInfo `json:"resource_usage,omitempty"`
	RestartAnalysis *RestartAnalysis   `json:"restart_analysis,omitempty"`
}

// HealthContext provides comprehensive context for AI-powered health analysis
type HealthContext struct {
	// Application metadata
	AppName          string            `json:"app_name"`
	Namespace        string            `json:"namespace"`
	LabelSelector    string            `json:"label_selector"`
	DeploymentType   string            `json:"deployment_type"` // Deployment, StatefulSet, DaemonSet
	DeploymentSpread map[string]int    `json:"deployment_spread"`
	Labels           map[string]string `json:"labels"`

	// Cluster and environment context
	ClusterInfo        map[string]string `json:"cluster_info"`
	NodeResourcesAvail bool              `json:"node_resources_available"`
	NamespaceQuotas    map[string]string `json:"namespace_quotas"`

	// Time and operational context
	Uptime             string `json:"uptime"`
	LastDeploymentTime string `json:"last_deployment_time,omitempty"`
	RecentEvents       int    `json:"recent_events"`
	PersistentIssues   bool   `json:"persistent_issues"`

	// Historical patterns
	HistoricalRestarts int     `json:"historical_restarts"`
	AverageReadyTime   float64 `json:"average_ready_time_seconds"`
	StabilityScore     int     `json:"stability_score"` // 0-100

	// Dependencies and external factors
	ExternalDependencies []string          `json:"external_dependencies"`
	NetworkPolicies      bool              `json:"network_policies_present"`
	SecurityContexts     map[string]string `json:"security_contexts"`
}

// PodSummary represents a summary of pod status
type PodSummary struct {
	Name    string            `json:"name"`
	Status  string            `json:"status"`
	Ready   string            `json:"ready"`
	Age     string            `json:"age"`
	Node    string            `json:"node"`
	Labels  map[string]string `json:"labels"`
	Metrics map[string]string `json:"metrics,omitempty"`
}

// PodIssue represents an issue with a specific pod
type PodIssue struct {
	PodName     string `json:"pod_name"`
	Issue       string `json:"issue"`
	Severity    string `json:"severity"` // critical, major, minor, warning
	Category    string `json:"category"` // resource, network, storage, configuration
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
	AutoFix     bool   `json:"auto_fix_available"`
}

// ContainerIssue represents an issue with a specific container
type ContainerIssue struct {
	PodName       string `json:"pod_name"`
	ContainerName string `json:"container_name"`
	Issue         string `json:"issue"`
	Severity      string `json:"severity"`
	Category      string `json:"category"`
	Description   string `json:"description"`
	Suggestion    string `json:"suggestion"`
	LogSnippet    string `json:"log_snippet,omitempty"`
	AutoFix       bool   `json:"auto_fix_available"`
}

// ServiceSummary represents a summary of service status
type ServiceSummary struct {
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	ClusterIP string            `json:"cluster_ip"`
	Ports     []string          `json:"ports"`
	Endpoints int               `json:"endpoints"`
	Labels    map[string]string `json:"labels"`
}

// ResourceUsageInfo provides resource usage information
type ResourceUsageInfo struct {
	CPU     map[string]string `json:"cpu"`    // requested, used, limit
	Memory  map[string]string `json:"memory"` // requested, used, limit
	Storage map[string]string `json:"storage,omitempty"`
}

// RestartAnalysis provides analysis of pod restart patterns
type RestartAnalysis struct {
	TotalRestarts     int               `json:"total_restarts"`
	RecentRestarts    int               `json:"recent_restarts_24h"`
	RestartReasons    map[string]int    `json:"restart_reasons"`
	RestartPattern    string            `json:"restart_pattern"` // none, occasional, frequent, continuous
	AffectedPods      []string          `json:"affected_pods"`
	RecommendedAction string            `json:"recommended_action"`
	Details           map[string]string `json:"details"`
}
