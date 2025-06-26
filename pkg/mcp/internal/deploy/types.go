package deploy

import (
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
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
