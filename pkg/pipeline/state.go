package pipeline

import (
	"github.com/Azure/container-kit/pkg/ai"
	"github.com/Azure/container-kit/pkg/docker"
	"github.com/Azure/container-kit/pkg/k8s"
)

// PipelineState holds state across steps and iterations
type PipelineState struct {
	IterationCount int
	RetryCount     int
	RepoFileTree   string
	Dockerfile     docker.Dockerfile
	RegistryURL    string
	ImageName      string
	K8sObjects     map[string]*k8s.K8sObject
	Success        bool
	TokenUsage     ai.TokenUsage
	Metadata       map[MetadataKey]any //Flexible storage
	StageHistory   []StageVisit
	ExtraContext   string             // Additional context for AI models passed from the CLI
	LLMCompletions []ai.LLMCompletion `json:"llm_completions,omitempty"`
}

type StageOutcome string

const (
	StageOutcomeSuccess StageOutcome = "success"
	StageOutcomeFailure StageOutcome = "failure"
)

type StageVisit struct {
	StageID    string
	RetryCount int
	Outcome    StageOutcome
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

type MetadataKey string

// Known metadata keys used across pipeline stages
const (
	RepoAnalysisResultKey MetadataKey = "RepoAnalysisResult"
	RepoAnalysisCallsKey  MetadataKey = "RepoAnalysisCalls"
	RepoAnalysisErrorKey  MetadataKey = "RepoAnalysisError"
)
