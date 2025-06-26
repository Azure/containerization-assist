package analyze

import (
	"time"

	"github.com/Azure/container-kit/pkg/core/analysis"
	"github.com/Azure/container-kit/pkg/core/git"
)

// CloneOptions represents options for cloning a repository
type CloneOptions struct {
	RepoURL   string
	Branch    string
	Shallow   bool
	TargetDir string
	SessionID string
}

// CloneResult wraps the git clone result with additional metadata
type CloneResult struct {
	*git.CloneResult
	Duration time.Duration
}

// AnalysisOptions represents options for analyzing a repository
type AnalysisOptions struct {
	RepoPath     string
	Context      string
	LanguageHint string
	SessionID    string
}

// AnalysisResult wraps the core analysis result with additional metadata
type AnalysisResult struct {
	*analysis.AnalysisResult
	Duration time.Duration
	Context  *AnalysisContext
}

// AnalysisContext provides rich context for AI reasoning
type AnalysisContext struct {
	// File structure insights
	FilesAnalyzed    int      `json:"files_analyzed"`
	ConfigFilesFound []string `json:"config_files_found"`
	EntryPointsFound []string `json:"entry_points_found"`
	TestFilesFound   []string `json:"test_files_found"`
	BuildFilesFound  []string `json:"build_files_found"`

	// Language ecosystem insights
	PackageManagers []string `json:"package_managers"`
	DatabaseFiles   []string `json:"database_files"`
	DockerFiles     []string `json:"docker_files"`
	K8sFiles        []string `json:"k8s_files"`

	// Repository insights
	HasGitIgnore   bool  `json:"has_gitignore"`
	HasReadme      bool  `json:"has_readme"`
	HasLicense     bool  `json:"has_license"`
	HasCI          bool  `json:"has_ci"`
	RepositorySize int64 `json:"repository_size_bytes"`

	// Suggestions for containerization
	ContainerizationSuggestions []string `json:"containerization_suggestions"`
	NextStepSuggestions         []string `json:"next_step_suggestions"`
}

// ContainerizationAssessment provides AI decision-making context
type ContainerizationAssessment struct {
	ReadinessScore      int                        `json:"readiness_score"` // 0-100
	StrengthAreas       []string                   `json:"strength_areas"`
	ChallengeAreas      []string                   `json:"challenge_areas"`
	RecommendedApproach string                     `json:"recommended_approach"`
	TechnologyStack     TechnologyStackAssessment  `json:"technology_stack"`
	RiskAnalysis        []ContainerizationRisk     `json:"risk_analysis"`
	DeploymentOptions   []DeploymentRecommendation `json:"deployment_options"`
}

// TechnologyStackAssessment provides technology-specific recommendations
type TechnologyStackAssessment struct {
	Language               string   `json:"language"`
	Framework              string   `json:"framework"`
	BaseImageOptions       []string `json:"base_image_options"`
	BuildStrategy          string   `json:"build_strategy"`
	SecurityConsiderations []string `json:"security_considerations"`
}

// ContainerizationRisk identifies potential challenges
type ContainerizationRisk struct {
	Area       string `json:"area"`
	Risk       string `json:"risk"`
	Impact     string `json:"impact"` // low, medium, high
	Mitigation string `json:"mitigation"`
}

// DeploymentRecommendation provides deployment strategy options
type DeploymentRecommendation struct {
	Strategy   string   `json:"strategy"`
	Pros       []string `json:"pros"`
	Cons       []string `json:"cons"`
	Complexity string   `json:"complexity"` // simple, moderate, complex
	UseCase    string   `json:"use_case"`
}
