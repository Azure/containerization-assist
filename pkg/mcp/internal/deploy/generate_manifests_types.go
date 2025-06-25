package deploy

import (
	"github.com/Azure/container-copilot/pkg/mcp/internal"
	"time"

	corek8s "github.com/Azure/container-copilot/pkg/core/kubernetes"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
)

// AtomicGenerateManifestsArgs defines arguments for atomic Kubernetes manifest generation
type AtomicGenerateManifestsArgs struct {
	types.BaseToolArgs

	// Image and app configuration
	ImageRef  string `json:"image_ref" description:"Container image reference (required)"`
	AppName   string `json:"app_name,omitempty" description:"Application name (derived from image if not provided)"`
	Namespace string `json:"namespace,omitempty" description:"Kubernetes namespace (default: default)"`

	// Deployment configuration
	Replicas       int    `json:"replicas,omitempty" description:"Number of replicas (default: 1)"`
	Port           int    `json:"port,omitempty" description:"Application port (default: 8080)"`
	ServiceType    string `json:"service_type,omitempty" description:"Service type: ClusterIP, NodePort, LoadBalancer (default: ClusterIP)"`
	IncludeIngress bool   `json:"include_ingress,omitempty" description:"Generate Ingress resource"`

	// Environment and secrets
	Environment    map[string]string `json:"environment,omitempty" description:"Environment variables"`
	SecretHandling string            `json:"secret_handling,omitempty" description:"Secret handling: auto, prompt, inline (default: auto)"`
	SecretManager  string            `json:"secret_manager,omitempty" description:"Preferred secret manager: kubernetes-secrets, sealed-secrets, external-secrets"`

	// Resource limits
	CPURequest    string `json:"cpu_request,omitempty" description:"CPU request (e.g., 100m)"`
	MemoryRequest string `json:"memory_request,omitempty" description:"Memory request (e.g., 128Mi)"`
	CPULimit      string `json:"cpu_limit,omitempty" description:"CPU limit (e.g., 500m)"`
	MemoryLimit   string `json:"memory_limit,omitempty" description:"Memory limit (e.g., 512Mi)"`

	// Advanced options
	GenerateHelm bool `json:"generate_helm,omitempty" description:"Generate as Helm chart"`
	GitOpsReady  bool `json:"gitops_ready,omitempty" description:"Make manifests GitOps-ready (externalize all secrets)"`
}

// AtomicGenerateManifestsResult defines the response from atomic manifest generation
type AtomicGenerateManifestsResult struct {
	types.BaseToolResponse
	internal.BaseAIContextResult
	Success bool `json:"success"`

	// Session context
	SessionID    string `json:"session_id"`
	WorkspaceDir string `json:"workspace_dir"`

	// Configuration used
	ImageRef  string `json:"image_ref"`
	AppName   string `json:"app_name"`
	Namespace string `json:"namespace"`

	// Generation results
	ManifestResult *corek8s.ManifestGenerationResult `json:"manifest_result"`

	// Secret handling results
	SecretsDetected []DetectedSecret    `json:"secrets_detected,omitempty"`
	SecretsPlan     *SecretsPlan        `json:"secrets_plan,omitempty"`
	SecretManifests []GeneratedManifest `json:"secret_manifests,omitempty"`

	// Timing information
	GenerationDuration time.Duration `json:"generation_duration"`
	TotalDuration      time.Duration `json:"total_duration"`

	// Rich context for Claude reasoning
	ManifestContext *ManifestContext `json:"manifest_context"`

	// AI context for decision-making
	DeploymentStrategyContext *DeploymentStrategyContext `json:"deployment_strategy_context"`

	// Rich error information if operation failed
}

// DetectedSecret represents a detected sensitive environment variable
type DetectedSecret struct {
	Name          string `json:"name"`
	RedactedValue string `json:"redacted_value"`
	SuggestedRef  string `json:"suggested_ref"`
	Pattern       string `json:"pattern"`
}

// SecretsPlan represents the plan for handling secrets
type SecretsPlan struct {
	Strategy         string               `json:"strategy"`
	SecretManager    string               `json:"secret_manager"`
	SecretReferences map[string]SecretRef `json:"secret_references"`
	ConfigMapEntries map[string]string    `json:"configmap_entries"`
	Instructions     []string             `json:"instructions"`
}

// GeneratedManifest represents a generated manifest file
type GeneratedManifest struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Path    string `json:"path"`
	Purpose string `json:"purpose"`
}

// ManifestContext provides rich context for Claude to reason about
type ManifestContext struct {
	// Manifest analysis
	ManifestsGenerated int      `json:"manifests_generated"`
	ResourceTypes      []string `json:"resource_types"`
	TotalResources     int      `json:"total_resources"`

	// Secret handling
	SecretsDetected     int    `json:"secrets_detected"`
	SecretsExternalized int    `json:"secrets_externalized"`
	SecretStrategy      string `json:"secret_strategy"`
	SecurityLevel       string `json:"security_level"`

	// Configuration summary
	DeploymentConfig map[string]interface{} `json:"deployment_config"`

	// Configuration management
	ConfigMapsCreated      int    `json:"configmaps_created"`
	ResourceLimitsSet      bool   `json:"resource_limits_set"`
	HealthChecksConfigured bool   `json:"health_checks_configured"`
	ComplexityLevel        string `json:"complexity_level"`

	// Best practices validation
	BestPractices  []string `json:"best_practices_followed"`
	SecurityIssues []string `json:"security_issues,omitempty"`

	// Next steps
	NextSteps      []string `json:"next_steps"`
	DeploymentTips []string `json:"deployment_tips"`

	// AI insights for context enrichment
	AIInsights []string `json:"ai_insights,omitempty"`
}

// DeploymentStrategyContext provides AI decision-making context for deployment strategies
type DeploymentStrategyContext struct {
	RecommendedStrategy   string                   `json:"recommended_strategy"`
	StrategyOptions       []DeploymentOption       `json:"strategy_options"`
	ResourceSizing        ResourceRecommendation   `json:"resource_sizing"`
	SecurityPosture       SecurityAssessment       `json:"security_posture"`
	ScalingConsiderations ScalingAnalysis          `json:"scaling_considerations"`
	EnvironmentProfiles   []EnvironmentProfile     `json:"environment_profiles"`
	TemplateContext       *ManifestTemplateContext `json:"template_context,omitempty"`
}

// ManifestTemplateContext provides template-specific context
type ManifestTemplateContext struct {
	TemplateName    string            `json:"template_name"`
	TemplateVersion string            `json:"template_version"`
	Parameters      map[string]string `json:"parameters"`
}

// DeploymentOption provides deployment strategy options with trade-offs
type DeploymentOption struct {
	Strategy     string   `json:"strategy"` // rolling, blue-green, canary, recreate
	Description  string   `json:"description"`
	Pros         []string `json:"pros"`
	Cons         []string `json:"cons"`
	Complexity   string   `json:"complexity"` // simple, moderate, complex
	UseCase      string   `json:"use_case"`
	Requirements []string `json:"requirements"` // What's needed to implement
	RiskLevel    string   `json:"risk_level"`   // low, medium, high
}

// ResourceRecommendation provides resource sizing guidance
type ResourceRecommendation struct {
	RecommendedProfile   string          `json:"recommended_profile"` // small, medium, large, custom
	CPURecommendation    ResourceSpec    `json:"cpu_recommendation"`
	MemoryRecommendation ResourceSpec    `json:"memory_recommendation"`
	ScalingMetrics       []ScalingMetric `json:"scaling_metrics"`
	CostImplications     []string        `json:"cost_implications"`
	Rationale            string          `json:"rationale"`
}

// ResourceSpec provides specific resource recommendations
type ResourceSpec struct {
	Request      string   `json:"request"`
	Limit        string   `json:"limit"`
	Rationale    string   `json:"rationale"`
	Alternatives []string `json:"alternatives"`
}

// ScalingMetric provides scaling configuration recommendations
type ScalingMetric struct {
	Type        string `json:"type"`      // cpu, memory, custom
	Threshold   string `json:"threshold"` // e.g., "70%"
	MinReplicas int    `json:"min_replicas"`
	MaxReplicas int    `json:"max_replicas"`
	Behavior    string `json:"behavior"` // aggressive, conservative, balanced
}

// SecurityAssessment provides security posture analysis
type SecurityAssessment struct {
	OverallRating    string                    `json:"overall_rating"` // excellent, good, needs-improvement, poor
	SecurityControls []SecurityControl         `json:"security_controls"`
	Vulnerabilities  []DeploymentSecurityIssue `json:"vulnerabilities"`
	Compliance       []ComplianceCheck         `json:"compliance"`
	Recommendations  []string                  `json:"recommendations"`
}

// SecurityControl represents implemented security measures
type SecurityControl struct {
	Name        string `json:"name"`
	Implemented bool   `json:"implemented"`
	Description string `json:"description"`
	Impact      string `json:"impact"` // low, medium, high
}

// DeploymentSecurityIssue represents potential security concerns
type DeploymentSecurityIssue struct {
	Category    string   `json:"category"` // secrets, rbac, network, image
	Severity    string   `json:"severity"` // low, medium, high, critical
	Description string   `json:"description"`
	Remediation []string `json:"remediation"`
}

// ComplianceCheck represents compliance framework assessments
type ComplianceCheck struct {
	Framework    string   `json:"framework"` // PCI-DSS, SOC2, GDPR, etc.
	Status       string   `json:"status"`    // compliant, non-compliant, partial
	Requirements []string `json:"requirements"`
}

// ScalingAnalysis provides scaling strategy recommendations
type ScalingAnalysis struct {
	RecommendedPattern string              `json:"recommended_pattern"` // horizontal, vertical, both
	AutoscalingOptions []AutoscalingOption `json:"autoscaling_options"`
	LoadTesting        LoadTestingGuidance `json:"load_testing"`
	MonitoringStrategy MonitoringStrategy  `json:"monitoring_strategy"`
}

// AutoscalingOption provides autoscaling configuration choices
type AutoscalingOption struct {
	Type        string   `json:"type"` // HPA, VPA, KEDA
	Description string   `json:"description"`
	Triggers    []string `json:"triggers"` // cpu, memory, custom metrics
	Pros        []string `json:"pros"`
	Cons        []string `json:"cons"`
	Complexity  string   `json:"complexity"`
}

// LoadTestingGuidance provides load testing recommendations
type LoadTestingGuidance struct {
	RecommendedApproach string   `json:"recommended_approach"`
	TestScenarios       []string `json:"test_scenarios"`
	Tools               []string `json:"tools"`
	Metrics             []string `json:"metrics"`
}

// MonitoringStrategy provides monitoring recommendations
type MonitoringStrategy struct {
	KeyMetrics     []string `json:"key_metrics"`
	AlertingRules  []string `json:"alerting_rules"`
	DashboardTypes []string `json:"dashboard_types"`
	LoggingLevel   string   `json:"logging_level"`
}

// EnvironmentProfile provides environment-specific configurations
type EnvironmentProfile struct {
	Environment     string            `json:"environment"` // dev, staging, prod
	Configuration   map[string]string `json:"configuration"`
	ResourceProfile string            `json:"resource_profile"`
	SecurityLevel   string            `json:"security_level"`
	Monitoring      string            `json:"monitoring"`
	Backup          string            `json:"backup"`
	Compliance      []string          `json:"compliance"`
}
