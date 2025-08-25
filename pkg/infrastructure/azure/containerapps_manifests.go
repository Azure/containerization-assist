// Package azure provides Azure Container Apps manifest generation and validation
package azure

import (
	"time"

	"github.com/Azure/containerization-assist/pkg/api"
)

// AzureContainerAppsManifestOptions contains options for manifest generation
type AzureContainerAppsManifestOptions struct {
	ImageRef             string              `json:"image_ref"`
	AppName              string              `json:"app_name"`
	ResourceGroup        string              `json:"resource_group"`
	Location             string              `json:"location"`
	EnvironmentName      string              `json:"environment_name"`
	Port                 int                 `json:"port"`
	Replicas             int                 `json:"replicas"`
	Template             string              `json:"template"` // "bicep" or "arm"
	OutputDir            string              `json:"output_dir"`
	IncludeEnvironment   bool                `json:"include_environment"`
	IncludeIngress       bool                `json:"include_ingress"`
	EnableDapr           bool                `json:"enable_dapr"`
	DaprAppId            string              `json:"dapr_app_id,omitempty"`
	DaprAppPort          int                 `json:"dapr_app_port,omitempty"`
	Labels               map[string]string   `json:"labels,omitempty"`
	EnvironmentVariables map[string]string   `json:"environment_variables,omitempty"`
	Resources            *ContainerResources `json:"resources,omitempty"`
	ManagedIdentity      bool                `json:"managed_identity"`
	CustomDomain         string              `json:"custom_domain,omitempty"`
	MinReplicas          int                 `json:"min_replicas"`
	MaxReplicas          int                 `json:"max_replicas"`
}

// ContainerResources defines CPU and memory requirements
type ContainerResources struct {
	CPU    float64 `json:"cpu"`    // in cores (e.g., 0.5, 1.0)
	Memory string  `json:"memory"` // e.g., "1.0Gi", "2.0Gi"
}

// AzureContainerAppsManifestResult contains the generation result
type AzureContainerAppsManifestResult struct {
	Success      bool                     `json:"success"`
	Manifests    []GeneratedAzureManifest `json:"manifests"`
	Template     string                   `json:"template"`
	OutputDir    string                   `json:"output_dir"`
	ManifestPath string                   `json:"manifest_path"`
	Duration     time.Duration            `json:"duration"`
	Context      map[string]interface{}   `json:"context"`
	Error        *AzureManifestError      `json:"error,omitempty"`
}

// GeneratedAzureManifest represents a generated Azure manifest
type GeneratedAzureManifest struct {
	Name    string `json:"name"`
	Type    string `json:"type"` // "bicep" or "arm"
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int    `json:"size"`
	Valid   bool   `json:"valid"`
}

// AzureManifestError provides detailed manifest error information
type AzureManifestError struct {
	Type         string                 `json:"type"` // "generation_error", "validation_error"
	Message      string                 `json:"message"`
	Path         string                 `json:"path,omitempty"`
	ManifestName string                 `json:"manifest_name,omitempty"`
	Context      map[string]interface{} `json:"context"`
}

// ValidationResult contains validation results for Azure manifests
type ValidationResult struct {
	Valid    bool                   `json:"valid"`
	Errors   []ValidationError      `json:"errors"`
	Warnings []ValidationWarning    `json:"warnings"`
	Metadata map[string]interface{} `json:"metadata"`
}

// ValidationError represents a validation error
type ValidationError struct {
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "error", "critical"
	Rule     string `json:"rule,omitempty"`
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
	Message string `json:"message"`
	Rule    string `json:"rule,omitempty"`
}

// AzureValidationResult wraps the validation result with Azure-specific context
type AzureValidationResult struct {
	api.ValidationResult
	ManifestType string `json:"manifest_type"` // "bicep" or "arm"
	FilePath     string `json:"file_path"`
}
