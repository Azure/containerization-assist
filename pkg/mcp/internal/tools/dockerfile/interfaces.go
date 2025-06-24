package dockerfile

import (
	"context"

	coredocker "github.com/Azure/container-copilot/pkg/core/docker"
	"github.com/rs/zerolog"
)

// TemplateSelectionEngine handles template selection logic
type TemplateSelectionEngine interface {
	// SelectTemplate automatically selects the best template based on repository analysis
	SelectTemplate(ctx context.Context, repoAnalysis map[string]interface{}) (string, error)

	// GetTemplateOptions returns available templates with metadata
	GetTemplateOptions(language, framework string, dependencies, configFiles []string) []TemplateOption

	// CalculateMatchScore calculates how well a template matches the project
	CalculateMatchScore(templateType, language, framework string, configFiles []string) int

	// GetAlternativeTemplates suggests alternatives with trade-offs
	GetAlternativeTemplates(language, framework string, dependencies []string) []AlternativeTemplate

	// GenerateTemplateSelectionContext creates rich context for AI template selection
	GenerateTemplateSelectionContext(language, framework string, dependencies, configFiles []string) *TemplateSelectionContext

	// MapCommonTemplateNames maps common language/framework names to actual template directory names
	MapCommonTemplateNames(name string) string
}

// TemplateGenerationEngine handles Dockerfile generation logic
type TemplateGenerationEngine interface {
	// GenerateDockerfile creates the actual Dockerfile
	GenerateDockerfile(ctx context.Context, templateName, dockerfilePath string, args GenerateDockerfileArgs, repoAnalysis map[string]interface{}) (string, error)

	// PreviewDockerfile generates a preview of the Dockerfile without writing to disk
	PreviewDockerfile(ctx context.Context, templateName string, args GenerateDockerfileArgs, repoAnalysis map[string]interface{}) (string, error)

	// ApplyCustomizations applies user-specified customizations to the Dockerfile
	ApplyCustomizations(content string, args GenerateDockerfileArgs, repoAnalysis map[string]interface{}) string

	// ExtractBuildSteps extracts the build steps from Dockerfile content
	ExtractBuildSteps(content string) []string

	// ExtractExposedPorts extracts exposed ports from Dockerfile content
	ExtractExposedPorts(content string) []int

	// ExtractBaseImage extracts the base image from Dockerfile content
	ExtractBaseImage(content string) string

	// ExtractHealthCheck extracts health check instruction from Dockerfile content
	ExtractHealthCheck(content string) string
}

// OptimizationEngine handles Dockerfile optimization logic
type OptimizationEngine interface {
	// ApplySizeOptimizations applies Docker best practices for smaller images
	ApplySizeOptimizations(lines []string) []string

	// ApplySecurityOptimizations applies security best practices
	ApplySecurityOptimizations(lines []string) []string

	// GenerateOptimizationContext creates optimization hints for the AI
	GenerateOptimizationContext(content string, args GenerateDockerfileArgs) *OptimizationContext

	// GenerateHealthCheck creates a health check instruction
	GenerateHealthCheck(language, framework string, exposedPorts []int) string

	// GetRecommendedBaseImage returns the recommended base image for a language/framework combination
	GetRecommendedBaseImage(language, framework string) string
}

// ValidationEngine handles Dockerfile validation logic
type ValidationEngine interface {
	// ValidateDockerfile validates the generated Dockerfile content
	ValidateDockerfile(ctx context.Context, content string) *coredocker.ValidationResult
}

// DockerfileOrchestrator coordinates all engines to provide a unified interface
type DockerfileOrchestrator interface {
	// Execute generates a Dockerfile using all engines in coordination
	Execute(ctx context.Context, args GenerateDockerfileArgs) (*GenerateDockerfileResult, error)

	// ExecuteWithContext runs the Dockerfile generation with progress tracking
	ExecuteWithContext(serverCtx interface{}, args GenerateDockerfileArgs) (*GenerateDockerfileResult, error)
}

// EngineConfiguration provides configuration for engines
type EngineConfiguration struct {
	Logger            zerolog.Logger
	EnableHadolint    bool
	TrustedRegistries []string
	MaxRetries        int
	TimeoutSeconds    int
}

// EngineContext provides context for engine operations
type EngineContext struct {
	SessionID      string
	RepositoryPath string
	ProjectName    string
	Metadata       map[string]interface{}
}

// GenerateDockerfileArgs represents the arguments for Dockerfile generation (imported from main tool)
type GenerateDockerfileArgs struct {
	BaseImage          string            `json:"base_image,omitempty"`
	Template           string            `json:"template,omitempty"`
	Optimization       string            `json:"optimization,omitempty"`
	IncludeHealthCheck bool              `json:"include_health_check,omitempty"`
	BuildArgs          map[string]string `json:"build_args,omitempty"`
	Platform           string            `json:"platform,omitempty"`
	SessionID          string            `json:"session_id"`
	DryRun             bool              `json:"dry_run,omitempty"`
}

// GenerateDockerfileResult represents the result of Dockerfile generation (imported from main tool)
type GenerateDockerfileResult struct {
	Content           string                       `json:"content"`
	BaseImage         string                       `json:"base_image"`
	ExposedPorts      []int                        `json:"exposed_ports"`
	HealthCheck       string                       `json:"health_check,omitempty"`
	BuildSteps        []string                     `json:"build_steps"`
	Template          string                       `json:"template_used"`
	FilePath          string                       `json:"file_path"`
	Validation        *coredocker.ValidationResult `json:"validation,omitempty"`
	Message           string                       `json:"message,omitempty"`
	TemplateSelection *TemplateSelectionContext    `json:"template_selection,omitempty"`
	OptimizationHints *OptimizationContext         `json:"optimization_hints,omitempty"`
	Success           bool                         `json:"success"`
	ErrorType         string                       `json:"error_type,omitempty"`
	ErrorMessage      string                       `json:"error_message,omitempty"`
}

// TemplateSelectionContext provides rich context for AI template selection
type TemplateSelectionContext struct {
	DetectedLanguage    string                `json:"detected_language"`
	DetectedFramework   string                `json:"detected_framework"`
	AvailableTemplates  []TemplateOption      `json:"available_templates"`
	RecommendedTemplate string                `json:"recommended_template"`
	SelectionReasoning  []string              `json:"selection_reasoning"`
	AlternativeOptions  []AlternativeTemplate `json:"alternative_options"`
}

// TemplateOption describes an available template
type TemplateOption struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	BestFor     []string `json:"best_for"`
	Limitations []string `json:"limitations"`
	MatchScore  int      `json:"match_score"` // 0-100
}

// AlternativeTemplate suggests alternatives with trade-offs
type AlternativeTemplate struct {
	Template  string   `json:"template"`
	Reason    string   `json:"reason"`
	TradeOffs []string `json:"trade_offs"`
	UseCases  []string `json:"use_cases"`
}

// OptimizationContext provides optimization guidance for AI
type OptimizationContext struct {
	CurrentSize       string               `json:"current_size,omitempty"`
	OptimizationGoals []string             `json:"optimization_goals"`
	SuggestedChanges  []OptimizationChange `json:"suggested_changes"`
	SecurityConcerns  []SecurityConcern    `json:"security_concerns"`
	BestPractices     []string             `json:"best_practices"`
}

// OptimizationChange describes a potential optimization
type OptimizationChange struct {
	Type        string `json:"type"` // "size", "security", "performance"
	Description string `json:"description"`
	Impact      string `json:"impact"`
	Example     string `json:"example,omitempty"`
}

// SecurityConcern describes a security issue
type SecurityConcern struct {
	Issue      string `json:"issue"`
	Severity   string `json:"severity"` // "high", "medium", "low"
	Suggestion string `json:"suggestion"`
	Reference  string `json:"reference,omitempty"`
}
