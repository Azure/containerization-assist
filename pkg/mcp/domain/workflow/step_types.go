// Package workflow defines types for workflow steps.
package workflow

// AnalyzeResult represents the output of repository analysis
type AnalyzeResult struct {
	Language        string                 `json:"language"`
	Framework       string                 `json:"framework"`
	Port            int                    `json:"port"`
	BuildCommand    string                 `json:"build_command"`
	StartCommand    string                 `json:"start_command"`
	Dependencies    []string               `json:"dependencies"`
	DevDependencies []string               `json:"dev_dependencies"`
	Metadata        map[string]interface{} `json:"metadata"`
	RepoPath        string                 `json:"repo_path"`
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
}

// K8sResult represents the output of Kubernetes manifest generation
type K8sResult struct {
	Manifests   []string               `json:"manifests"`
	Namespace   string                 `json:"namespace"`
	ServiceName string                 `json:"service_name"`
	Endpoint    string                 `json:"endpoint"`
	Metadata    map[string]interface{} `json:"metadata"`
}
