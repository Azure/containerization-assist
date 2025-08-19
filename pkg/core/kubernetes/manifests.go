// Package kubernetes provides core Kubernetes operations extracted from the Containerization Assist pipeline.
// This package contains mechanical K8s operations without AI dependencies,
// designed to be used by atomic MCP tools.
package kubernetes

import (
	"time"

	"github.com/Azure/containerization-assist/pkg/mcp/api"
)

// ManifestGenerationResult contains the result of manifest generation
type ManifestGenerationResult struct {
	Success      bool                   `json:"success"`
	Manifests    []GeneratedManifest    `json:"manifests"`
	Template     string                 `json:"template"`
	OutputDir    string                 `json:"output_dir"`
	ManifestPath string                 `json:"manifest_path"` // Path to generated manifests
	Duration     time.Duration          `json:"duration"`
	Context      map[string]interface{} `json:"context"`
	Error        *ManifestError         `json:"error,omitempty"`
}

// GeneratedManifest represents a generated Kubernetes manifest
type GeneratedManifest struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Path    string `json:"path"`
	Content string `json:"content"`
	Size    int    `json:"size"`
	Valid   bool   `json:"valid"`
}

// ManifestDiscoveryResult contains discovered manifests
type ManifestDiscoveryResult struct {
	Success   bool                   `json:"success"`
	Manifests []DiscoveredManifest   `json:"manifests"`
	Directory string                 `json:"directory"`
	Context   map[string]interface{} `json:"context"`
	Error     *ManifestError         `json:"error,omitempty"`
}

// DiscoveredManifest represents a discovered Kubernetes manifest
type DiscoveredManifest struct {
	Name             string            `json:"name"`
	Kind             string            `json:"kind"`
	ApiVersion       string            `json:"api_version"`
	Path             string            `json:"path"`
	Size             int64             `json:"size"`
	Valid            bool              `json:"valid"`
	Metadata         map[string]string `json:"metadata"`
	ValidationErrors []string          `json:"validation_errors,omitempty"`
}

// ManifestError provides detailed manifest error information
type ManifestError struct {
	Type         string                 `json:"type"` // "generation_error", "discovery_error", "validation_error"
	Message      string                 `json:"message"`
	Path         string                 `json:"path,omitempty"`
	ManifestName string                 `json:"manifest_name,omitempty"`
	Context      map[string]interface{} `json:"context"`
}

// ManifestOptions contains options for manifest generation
type ManifestOptions struct {
	ImageRef       string
	AppName        string
	Namespace      string
	Port           int
	Replicas       int
	Template       string
	OutputDir      string
	IncludeService bool
	IncludeIngress bool
	Labels         map[string]string
	Annotations    map[string]string
	Resources      *ResourceRequirements
}

// ResourceRequirements defines resource requests and limits
type ResourceRequirements struct {
	Requests *ResourceQuantity `json:"requests,omitempty"`
	Limits   *ResourceQuantity `json:"limits,omitempty"`
}

// ResourceQuantity defines CPU and memory quantities
type ResourceQuantity struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// ManifestValidationResult now uses the unified validation framework for deploy domain
type ManifestValidationResult = api.ManifestValidationResult

// ValidationError now uses the unified validation framework
type ValidationError = api.ValidationError

// TemplateInfo contains information about a manifest template
type TemplateInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Files       []string `json:"files"`
}
