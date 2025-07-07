package deploy

import (
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/types"
	"github.com/Azure/container-kit/pkg/mcp/domain/types/tools"
)

// GenerationOptions contains options for manifest generation
type GenerationOptions struct {
	ImageRef        types.ImageReference `json:"image_ref"`
	OutputPath      string               `json:"output_path,omitempty"`
	Namespace       string               `json:"namespace,omitempty"`
	ServiceType     string               `json:"service_type,omitempty"`
	Replicas        int                  `json:"replicas,omitempty"`
	Resources       ResourceRequests     `json:"resources,omitempty"`
	Environment     map[string]string    `json:"environment,omitempty"`
	Secrets         []SecretRef          `json:"secrets,omitempty"`
	IncludeIngress  bool                 `json:"include_ingress,omitempty"`
	HelmTemplate    bool                 `json:"helm_template,omitempty"`
	ConfigMapData   map[string]string    `json:"configmap_data,omitempty"`
	ConfigMapFiles  map[string]string    `json:"configmap_files,omitempty"`
	BinaryData      map[string][]byte    `json:"binary_data,omitempty"`
	IngressHosts    []IngressHost        `json:"ingress_hosts,omitempty"`
	IngressTLS      []IngressTLS         `json:"ingress_tls,omitempty"`
	IngressClass    string               `json:"ingress_class,omitempty"`
	ServicePorts    []ServicePort        `json:"service_ports,omitempty"`
	LoadBalancerIP  string               `json:"load_balancer_ip,omitempty"`
	SessionAffinity string               `json:"session_affinity,omitempty"`
	WorkflowLabels  map[string]string    `json:"workflow_labels,omitempty"`
}

// ResourceRequests represents Kubernetes resource requirements
type ResourceRequests struct {
	CPU     string `json:"cpu,omitempty"`
	Memory  string `json:"memory,omitempty"`
	Storage string `json:"storage,omitempty"`
}

// SecretRef represents a reference to a Kubernetes secret
type SecretRef struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// ServicePort represents a Kubernetes service port configuration
type ServicePort struct {
	Name       string `json:"name,omitempty"`
	Port       int    `json:"port"`
	TargetPort int    `json:"target_port,omitempty"`
	NodePort   int    `json:"node_port,omitempty"`
	Protocol   string `json:"protocol,omitempty"`
}

// IngressHost represents ingress host configuration
type IngressHost struct {
	Host  string        `json:"host"`
	Paths []IngressPath `json:"paths"`
}

// IngressPath represents a path configuration for an ingress host
type IngressPath struct {
	Path        string `json:"path"`
	PathType    string `json:"path_type,omitempty"`
	ServiceName string `json:"service_name"`
	ServicePort int    `json:"service_port"`
}

// IngressTLS represents TLS configuration for ingress
type IngressTLS struct {
	Hosts      []string `json:"hosts"`
	SecretName string   `json:"secret_name"`
}

// GenerationResult contains the result of manifest generation
type GenerationResult struct {
	Success           bool               `json:"success"`
	ManifestPath      string             `json:"manifest_path"`
	FilesGenerated    []string           `json:"files_generated"`
	ValidationSummary *ValidationSummary `json:"validation_summary,omitempty"`
	Duration          time.Duration      `json:"duration"`
	Errors            []string           `json:"errors,omitempty"`
	Warnings          []string           `json:"warnings,omitempty"`
}

// ValidationSummary contains manifest validation results
type ValidationSummary struct {
	Valid           bool                      `json:"valid"`
	TotalFiles      int                       `json:"total_files"`
	ValidFiles      int                       `json:"valid_files"`
	InvalidFiles    int                       `json:"invalid_files"`
	Results         map[string]FileValidation `json:"results"`
	OverallSeverity string                    `json:"overall_severity"`
}

// FileValidation contains validation results for a single file
type FileValidation struct {
	Valid    bool              `json:"valid"`
	Errors   []ValidationIssue `json:"errors,omitempty"`
	Warnings []ValidationIssue `json:"warnings,omitempty"`
	Info     []ValidationIssue `json:"info,omitempty"`
}

// ValidationIssue represents a validation issue
type ValidationIssue struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Field    string `json:"field,omitempty"`
	Code     string `json:"code,omitempty"`
}

// TemplateContext provides context for template selection and generation
type TemplateContext struct {
	Language       string
	Framework      string
	HasTests       bool
	HasDatabase    bool
	IsWebApp       bool
	HasStaticFiles bool
	Port           int
}

// ===== STRONGLY-TYPED KUBERNETES DEPLOY TOOLS =====

// KubernetesDeployParams defines strongly-typed parameters for Kubernetes deployment
type KubernetesDeployParams struct {
	// Required parameters
	ManifestPath  string `json:"manifest_path" validate:"required,file"`
	Namespace     string `json:"namespace" validate:"required,dns_label"`
	DeploymentKey string `json:"deployment_key" validate:"required"`

	// Optional parameters
	KubeConfig string            `json:"kube_config,omitempty" validate:"omitempty,file"`
	Context    string            `json:"context,omitempty"`
	DryRun     bool              `json:"dry_run,omitempty"`
	Wait       bool              `json:"wait,omitempty"`
	Timeout    time.Duration     `json:"timeout,omitempty"`
	Values     map[string]string `json:"values,omitempty"`
	SessionID  string            `json:"session_id,omitempty"`

	// Deployment strategy
	Strategy string `json:"strategy,omitempty" validate:"omitempty,oneof=RollingUpdate Recreate"`
	Force    bool   `json:"force,omitempty"`

	// Resource management
	ResourceQuota map[string]string `json:"resource_quota,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	Annotations   map[string]string `json:"annotations,omitempty"`
}

// Validate implements tools.ToolParams
func (p KubernetesDeployParams) Validate() error {
	// Basic validation - migrated to rich error system
	if p.ManifestPath == "" {
		return tools.NewRichValidationError("kubernetes-deploy", "manifest_path", "required")
	}
	if p.Namespace == "" {
		return tools.NewRichValidationError("kubernetes-deploy", "namespace", "required")
	}
	if p.DeploymentKey == "" {
		return tools.NewRichValidationError("kubernetes-deploy", "deployment_key", "required")
	}
	return nil
}

// GetSessionID implements tools.ToolParams
func (p KubernetesDeployParams) GetSessionID() string {
	return p.SessionID
}

// KubernetesDeployResult defines strongly-typed results for Kubernetes deployment
type KubernetesDeployResult struct {
	// Success status
	Success bool `json:"success"`

	// Deployment details
	DeploymentKey string        `json:"deployment_key"`
	Namespace     string        `json:"namespace"`
	Resources     []string      `json:"resources,omitempty"`
	Duration      time.Duration `json:"duration"`

	// Resource status
	ReadyReplicas   int32 `json:"ready_replicas,omitempty"`
	DesiredReplicas int32 `json:"desired_replicas,omitempty"`

	// Service details
	Services []KubernetesService `json:"services,omitempty"`

	// Rollout status
	RolloutStatus string `json:"rollout_status,omitempty"`
	Revision      string `json:"revision,omitempty"`

	// Session tracking
	SessionID string `json:"session_id,omitempty"`

	// Validation results
	ValidationWarnings []string `json:"validation_warnings,omitempty"`

	// Resource consumption
	ResourceUsage map[string]string `json:"resource_usage,omitempty"`
}

// IsSuccess implements tools.ToolResult
func (r KubernetesDeployResult) IsSuccess() bool {
	return r.Success
}

// GetData implements ToolOutput interface
func (r KubernetesDeployResult) GetData() interface{} {
	return r
}

// GetDuration implements tools.ToolResult
func (r KubernetesDeployResult) GetDuration() time.Duration {
	return r.Duration
}

// KubernetesService represents a deployed Kubernetes service
type KubernetesService struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	ClusterIP   string   `json:"cluster_ip,omitempty"`
	ExternalIPs []string `json:"external_ips,omitempty"`
	Ports       []int32  `json:"ports,omitempty"`
	Endpoints   []string `json:"endpoints,omitempty"`
}

// ===== SECURITY SCAN TOOLS - Type Aliases =====
// Security types moved to pkg/mcp/core/types/security.go for better architecture

// Type aliases for security types (from shared location)
type SecurityScanParams = types.SecurityScanParams
type SecurityScanResult = types.SecurityScanResult
type SecurityVulnerability = types.SecurityVulnerability
type ComplianceResult = types.ComplianceResult
type DetectedSecret = types.DetectedSecret
type LicenseInfo = types.LicenseInfo
