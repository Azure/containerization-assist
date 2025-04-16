package main

// ManifestDeployResult stores the result of a single manifest deployment
type ManifestDeployResult struct {
	Path    string
	Success bool
	Output  string
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

type Dockerfile struct {
	Content     string
	Path        string
	BuildErrors string
}

// PipelineState holds state across steps and iterations
type PipelineState struct {
	RepoFileTree   string
	Dockerfile     Dockerfile
	RegistryURL    string
	ImageName      string
	K8sObjects     map[string]*K8sObject
	Success        bool
	IterationCount int
	Metadata       map[string]interface{} //Flexible storage //Could store summary of changes that will get displayed to the user at the end
}

type K8sObject struct {
	ApiVersion             string      `yaml:"apiVersion"`
	Kind                   string      `yaml:"kind"`
	Metadata               K8sMetadata `yaml:"metadata"`
	Content                []byte
	ManifestPath           string
	isSuccessfullyDeployed bool
	isDeploymentType       bool
	errorLog               string
}

type K8sMetadata struct {
	Name   string            `yaml:"name"`
	Labels map[string]string `yaml:"labels"`
}
