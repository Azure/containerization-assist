package core

import (
	"context"
	"time"
)

// ProgressReporter provides unified progress reporting across all tools.
type ProgressReporter interface {
	StartStage(stage string) ProgressToken
	UpdateProgress(token ProgressToken, message string, percent int)
	CompleteStage(token ProgressToken, success bool, message string)
}

// ProgressToken represents a unique identifier for a progress stage
type ProgressToken string

// ProgressStage represents the state of a progress stage
type ProgressStage struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
	Progress    int     `json:"progress"`
	Message     string  `json:"message"`
	Weight      float64 `json:"weight"`
}

// Analyzer provides unified analysis functionality combining repository analysis and AI analysis.
type Analyzer interface {
	AnalyzeStructure(ctx context.Context, path string) (*RepositoryInfo, error)
	AnalyzeDockerfile(ctx context.Context, path string) (*DockerfileInfo, error)
	GetBuildRecommendations(ctx context.Context, repo *RepositoryInfo) (*BuildRecommendations, error)

	Analyze(ctx context.Context, prompt string) (string, error)
	AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error)
	AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error)
	GetTokenUsage() TokenUsage
	ResetTokenUsage()
}

// RepositoryAnalyzer provides repository analysis functionality.
type RepositoryAnalyzer interface {
	AnalyzeStructure(ctx context.Context, path string) (*RepositoryInfo, error)
	AnalyzeDockerfile(ctx context.Context, path string) (*DockerfileInfo, error)
	GetBuildRecommendations(ctx context.Context, repo *RepositoryInfo) (*BuildRecommendations, error)
}

// AIAnalyzer provides AI analysis functionality.
type AIAnalyzer interface {
	Analyze(ctx context.Context, prompt string) (string, error)
	AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error)
	AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error)
	GetTokenUsage() TokenUsage
	ResetTokenUsage()
}

// RepositoryInfo represents repository analysis results
type RepositoryInfo struct {
	Path          string            `json:"path"`
	Type          string            `json:"type"`
	Language      string            `json:"language"`
	Framework     string            `json:"framework"`
	Languages     []string          `json:"languages"`
	Dependencies  map[string]string `json:"dependencies"`
	BuildTools    []string          `json:"build_tools"`
	EntryPoint    string            `json:"entry_point"`
	Port          int               `json:"port"`
	HasDockerfile bool              `json:"has_dockerfile"`
	Metadata      map[string]string `json:"metadata"`
}

// DockerfileInfo represents Dockerfile analysis results
type DockerfileInfo struct {
	Path           string            `json:"path"`
	BaseImage      string            `json:"base_image"`
	ExposedPorts   []int             `json:"exposed_ports"`
	WorkingDir     string            `json:"working_dir"`
	EntryPoint     []string          `json:"entry_point"`
	Cmd            []string          `json:"cmd"`
	HealthCheck    *HealthCheckInfo  `json:"health_check,omitempty"`
	Labels         map[string]string `json:"labels"`
	BuildArgs      map[string]string `json:"build_args"`
	MultiStage     bool              `json:"multi_stage"`
	SecurityIssues []string          `json:"security_issues"`
}

// HealthCheckInfo represents Docker health check configuration
type HealthCheckInfo struct {
	Test     []string      `json:"test"`
	Interval time.Duration `json:"interval"`
	Timeout  time.Duration `json:"timeout"`
	Retries  int           `json:"retries"`
}

// BuildRecommendations represents build optimization recommendations
type BuildRecommendations struct {
	OptimizationTips []string          `json:"optimization_tips"`
	SecurityTips     []string          `json:"security_tips"`
	PerformanceTips  []string          `json:"performance_tips"`
	BestPractices    []string          `json:"best_practices"`
	Suggestions      map[string]string `json:"suggestions"`
}

// TokenUsage represents token usage tracking
type TokenUsage struct {
	CompletionTokens int `json:"completion_tokens"`
	PromptTokens     int `json:"prompt_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// FixingResult represents the result of a fixing operation
type FixingResult struct {
	Success         bool          `json:"success"`
	AttemptsUsed    int           `json:"attempts_used"`
	OriginalError   error         `json:"original_error,omitempty"`
	FinalError      error         `json:"final_error,omitempty"`
	FixApplied      bool          `json:"fix_applied"`
	FixDescription  string        `json:"fix_description,omitempty"`
	AllAttempts     []interface{} `json:"all_attempts"`
	TotalAttempts   int           `json:"total_attempts"`
	Duration        time.Duration `json:"duration"`
	LastAttemptTime time.Time     `json:"last_attempt_time"`
}

// BaseAIContextResult provides AI context result information
type BaseAIContextResult struct {
	AIContextType     string        `json:"ai_context_type"`
	IsSuccessful      bool          `json:"is_successful"`
	Duration          time.Duration `json:"duration"`
	TokensUsed        int           `json:"tokens_used,omitempty"`
	ContextEnhanced   bool          `json:"context_enhanced"`
	EnhancementErrors []string      `json:"enhancement_errors,omitempty"`
}

// NewBaseAIContextResult creates a new BaseAIContextResult
func NewBaseAIContextResult(contextType string, successful bool, duration time.Duration) BaseAIContextResult {
	return BaseAIContextResult{
		AIContextType:     contextType,
		IsSuccessful:      successful,
		Duration:          duration,
		ContextEnhanced:   false,
		EnhancementErrors: []string{},
	}
}
