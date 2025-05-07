package pipeline

import (
	"github.com/Azure/container-copilot/pkg/ai"
	"github.com/Azure/container-copilot/pkg/docker"
	"github.com/Azure/container-copilot/pkg/k8s"
)

// PipelineState holds state across steps and iterations
type PipelineState struct {
	RepoFileTree   string
	Dockerfile     docker.Dockerfile
	RegistryURL    string
	ImageName      string
	K8sObjects     map[string]*k8s.K8sObject
	Success        bool
	IterationCount int
	TokenUsage     ai.TokenUsage
	Metadata       map[string]interface{} //Flexible storage //Could store summary of changes that will get displayed to the user at the end
}

// FileAnalysisInput represents the common input structure for file analysis
type FileAnalysisInput struct {
	Content       string `json:"content"` // Plain text content of the file
	ErrorMessages string `json:"error_messages,omitempty"`
	RepoFileTree  string `json:"repo_files,omitempty"` // String representation of the file tree
	FilePath      string `json:"file_path,omitempty"`  // Path to the original file
}

// FileAnalysisResult represents the common analysis result
type FileAnalysisResult struct {
	FixedContent string `json:"fixed_content"`
	Analysis     string `json:"analysis"`
}
