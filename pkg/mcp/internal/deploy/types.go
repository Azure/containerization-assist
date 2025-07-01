package deploy

import (
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/Azure/container-kit/pkg/mcp/types/tools"
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
	// Basic validation - in production would use validator package
	if p.ManifestPath == "" {
		return tools.NewValidationError("manifest_path", "required")
	}
	if p.Namespace == "" {
		return tools.NewValidationError("namespace", "required")
	}
	if p.DeploymentKey == "" {
		return tools.NewValidationError("deployment_key", "required")
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

// ===== STRONGLY-TYPED SECURITY SCAN TOOLS =====

// SecurityScanParams defines strongly-typed parameters for security scanning
type SecurityScanParams struct {
	// Required parameters
	Target   string `json:"target" validate:"required"`
	ScanType string `json:"scan_type" validate:"required,oneof=image container filesystem"`

	// Optional parameters
	Scanner  string   `json:"scanner,omitempty" validate:"omitempty,oneof=trivy grype"`
	Format   string   `json:"format,omitempty" validate:"omitempty,oneof=json yaml table"`
	Severity []string `json:"severity,omitempty" validate:"omitempty,dive,oneof=UNKNOWN LOW MEDIUM HIGH CRITICAL"`

	// Filtering options
	IgnoreUnfixed bool     `json:"ignore_unfixed,omitempty"`
	IgnoreFiles   []string `json:"ignore_files,omitempty"`
	PolicyPath    string   `json:"policy_path,omitempty" validate:"omitempty,file"`

	// Output options
	OutputPath string `json:"output_path,omitempty"`
	ExitCode   bool   `json:"exit_code,omitempty"`

	// Session tracking
	SessionID string `json:"session_id,omitempty"`

	// Registry authentication (for image scans)
	Registry struct {
		Username string `json:"username,omitempty"`
		Password string `json:"password,omitempty"`
		Token    string `json:"token,omitempty"`
	} `json:"registry,omitempty"`
}

// Validate implements tools.ToolParams
func (p SecurityScanParams) Validate() error {
	// Basic validation - in production would use validator package
	if p.Target == "" {
		return tools.NewValidationError("target", "required")
	}
	if p.ScanType == "" {
		return tools.NewValidationError("scan_type", "required")
	}
	// Validate scan_type enum
	validScanTypes := map[string]bool{
		"image":      true,
		"container":  true,
		"filesystem": true,
	}
	if !validScanTypes[p.ScanType] {
		return tools.NewValidationError("scan_type", "must be one of: image, container, filesystem")
	}
	return nil
}

// GetSessionID implements tools.ToolParams
func (p SecurityScanParams) GetSessionID() string {
	return p.SessionID
}

// SecurityScanResult defines strongly-typed results for security scanning
type SecurityScanResult struct {
	// Success status
	Success bool `json:"success"`

	// Scan details
	Target   string        `json:"target"`
	ScanType string        `json:"scan_type"`
	Scanner  string        `json:"scanner"`
	Duration time.Duration `json:"duration"`

	// Vulnerability summary
	TotalVulnerabilities      int            `json:"total_vulnerabilities"`
	VulnerabilitiesBySeverity map[string]int `json:"vulnerabilities_by_severity"`

	// Detailed results
	Vulnerabilities []SecurityVulnerability `json:"vulnerabilities,omitempty"`

	// Compliance results
	ComplianceResults []ComplianceResult `json:"compliance_results,omitempty"`

	// Secret detection
	Secrets []DetectedSecret `json:"secrets,omitempty"`

	// License information
	Licenses []LicenseInfo `json:"licenses,omitempty"`

	// Session tracking
	SessionID string `json:"session_id,omitempty"`

	// Risk assessment
	RiskScore float64 `json:"risk_score,omitempty"`
	RiskLevel string  `json:"risk_level,omitempty"`

	// Remediation suggestions
	Recommendations []string `json:"recommendations,omitempty"`
}

// IsSuccess implements tools.ToolResult
func (r SecurityScanResult) IsSuccess() bool {
	return r.Success
}

// GetDuration implements tools.ToolResult
func (r SecurityScanResult) GetDuration() time.Duration {
	return r.Duration
}

// SecurityVulnerability represents a detected security vulnerability
type SecurityVulnerability struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Severity    string  `json:"severity"`
	CVSS        float64 `json:"cvss,omitempty"`

	// Package information
	Package struct {
		Name           string `json:"name"`
		Version        string `json:"version"`
		FixedVersion   string `json:"fixed_version,omitempty"`
		PackageManager string `json:"package_manager,omitempty"`
	} `json:"package"`

	// References
	References []string `json:"references,omitempty"`

	// Fix information
	Fixed bool   `json:"fixed"`
	Fix   string `json:"fix,omitempty"`
}

// ComplianceResult represents compliance check results
type ComplianceResult struct {
	Standard    string `json:"standard"`
	Control     string `json:"control"`
	Status      string `json:"status"`
	Description string `json:"description"`
	Remediation string `json:"remediation,omitempty"`
}

// DetectedSecret represents a detected secret or sensitive information
type DetectedSecret struct {
	Type        string `json:"type"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

// LicenseInfo represents license information for dependencies
type LicenseInfo struct {
	Package string `json:"package"`
	License string `json:"license"`
	Type    string `json:"type"`
	Risk    string `json:"risk,omitempty"`
}

// Type aliases for tool interfaces
type KubernetesDeployTool = tools.Tool[KubernetesDeployParams, KubernetesDeployResult]
type SecurityScanTool = tools.Tool[SecurityScanParams, SecurityScanResult]
