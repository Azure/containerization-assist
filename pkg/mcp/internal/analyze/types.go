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

// Dockerfile generation types

// GenerateDockerfileArgs represents the arguments for generating a Dockerfile
type GenerateDockerfileArgs struct {
	BaseImage          string            `json:"base_image,omitempty" description:"Override detected base image"`
	Template           string            `json:"template,omitempty" jsonschema:"enum=go,node,python,java,rust,php,ruby,dotnet,golang" description:"Use specific template (go, node, python, etc.)"`
	Optimization       string            `json:"optimization,omitempty" jsonschema:"enum=size,speed,security,balanced" description:"Optimization level (size, speed, security)"`
	IncludeHealthCheck bool              `json:"include_health_check,omitempty" description:"Add health check to Dockerfile"`
	BuildArgs          map[string]string `json:"build_args,omitempty" description:"Docker build arguments"`
	Platform           string            `json:"platform,omitempty" jsonschema:"enum=linux/amd64,linux/arm64,linux/arm/v7" description:"Target platform (e.g., linux/amd64)"`
	SessionID          string            `json:"session_id,omitempty" description:"Session ID for context"`
	DryRun             bool              `json:"dry_run,omitempty" description:"Preview without writing file"`
}

// GenerateDockerfileResult represents the result of generating a Dockerfile
type GenerateDockerfileResult struct {
	Content      string      `json:"content"`
	BaseImage    string      `json:"base_image"`
	ExposedPorts []int       `json:"exposed_ports"`
	HealthCheck  string      `json:"health_check,omitempty"`
	BuildSteps   []string    `json:"build_steps"`
	Template     string      `json:"template_used"`
	FilePath     string      `json:"file_path"`
	Message      string      `json:"message,omitempty"`
	Validation   interface{} `json:"validation,omitempty"`

	TemplateSelection *TemplateSelectionContext `json:"template_selection,omitempty"`
	OptimizationHints *OptimizationContext      `json:"optimization_hints,omitempty"`
}

// TemplateSelectionContext provides context about template selection
type TemplateSelectionContext struct {
	DetectedLanguage    string                `json:"detected_language"`
	DetectedFramework   string                `json:"detected_framework"`
	AvailableTemplates  []TemplateOption      `json:"available_templates"`
	RecommendedTemplate string                `json:"recommended_template"`
	SelectionReasoning  []string              `json:"selection_reasoning"`
	AlternativeOptions  []AlternativeTemplate `json:"alternative_options"`
}

// TemplateOption represents a Dockerfile template option
type TemplateOption struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	BestFor     []string `json:"best_for"`
	Limitations []string `json:"limitations"`
	MatchScore  int      `json:"match_score"`
}

// AlternativeTemplate represents an alternative template suggestion
type AlternativeTemplate struct {
	Template  string   `json:"template"`
	Reason    string   `json:"reason"`
	TradeOffs []string `json:"trade_offs"`
	UseCases  []string `json:"use_cases"`
}

// OptimizationContext provides optimization hints and suggestions
type OptimizationContext struct {
	CurrentSize       string               `json:"current_size,omitempty"`
	OptimizationGoals []string             `json:"optimization_goals"`
	SuggestedChanges  []OptimizationChange `json:"suggested_changes"`
	SecurityConcerns  []SecurityConcern    `json:"security_concerns"`
	BestPractices     []string             `json:"best_practices"`
}

// OptimizationChange represents a suggested optimization
type OptimizationChange struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
	Example     string `json:"example,omitempty"`
}

// SecurityConcern represents a security issue and suggestion
type SecurityConcern struct {
	Issue      string `json:"issue"`
	Severity   string `json:"severity"`
	Suggestion string `json:"suggestion"`
	Reference  string `json:"reference,omitempty"`
}
