// Package workflow defines types for workflow steps.
package workflow

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// AnalyzeResult represents the output of repository analysis
type AnalyzeResult struct {
	Language        string                 `json:"language" yaml:"language"`
	Framework       string                 `json:"framework" yaml:"framework"`
	Port            int                    `json:"port" yaml:"port"`
	BuildCommand    string                 `json:"build_command" yaml:"build_command"`
	StartCommand    string                 `json:"start_command" yaml:"start_command"`
	Dependencies    []string               `json:"dependencies" yaml:"dependencies"`
	DevDependencies []string               `json:"dev_dependencies" yaml:"dev_dependencies"`
	Metadata        map[string]interface{} `json:"metadata" yaml:"metadata"`
	RepoPath        string                 `json:"repo_path" yaml:"repo_path"`
}

// String returns a formatted string representation of AnalyzeResult
func (ar AnalyzeResult) String() string {
	data, err := yaml.Marshal(ar)
	if err != nil {
		return fmt.Sprintf("Error marshaling AnalyzeResult: %v", err)
	}
	return string(data)
}

// DockerfileResult represents the output of Dockerfile generation
type DockerfileResult struct {
	Content     string                 `json:"content"`
	Path        string                 `json:"path"`
	BaseImage   string                 `json:"base_image"`
	Metadata    map[string]interface{} `json:"metadata"`
	ExposedPort int                    `json:"exposed_port,omitempty"`
}

// BuildResult represents the output of Docker build
type BuildResult struct {
	ImageID   string                 `json:"image_id"`
	ImageRef  string                 `json:"image_ref"`
	ImageSize int64                  `json:"image_size"`
	BuildTime string                 `json:"build_time"`
	Metadata  map[string]interface{} `json:"metadata"`
	Errors    []string               `json:"errors,omitempty"`
}

// K8sResult represents the output of Kubernetes manifest generation
type K8sResult struct {
	Manifests   []string               `json:"manifests"`
	Namespace   string                 `json:"namespace"`
	ServiceName string                 `json:"service_name"`
	Endpoint    string                 `json:"endpoint"`
	Metadata    map[string]interface{} `json:"metadata"`
}
