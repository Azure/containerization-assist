package build

import (
	"context"
	"time"
)

// BuildService consolidates all build-related operations
// Replaces: UnifiedAnalyzer, BuildStrategy, BuildValidator, BuildExecutor
// Replaces: BuildImageSessionManager, BuildImagePipelineAdapter, BuildImageScanner
type BuildService interface {
	// Analysis capabilities (was UnifiedAnalyzer)
	AnalyzeDockerfile(ctx context.Context, path string) (*AnalysisResult, error)
	AnalyzeBuildContext(ctx context.Context, contextPath string) (*ContextAnalysis, error)

	// Build execution (was BuildStrategy + BuildExecutor)
	BuildImage(ctx context.Context, config BuildConfig) (*BuildResult, error)
	ValidateBuild(ctx context.Context, config BuildConfig) error

	// Session management (was BuildImageSessionManager)
	CreateBuildSession(ctx context.Context, sessionID string) error
	GetBuildSession(ctx context.Context, sessionID string) (*BuildSession, error)

	// Security scanning (was BuildImageScanner)
	ScanBuildSecurity(ctx context.Context, imageRef string) (*SecurityScanResult, error)

	// Progress reporting (consolidates multiple reporting interfaces)
	SetProgressCallback(callback func(progress BuildProgress))

	// Event handling (was EventService)
	PublishBuildEvent(event BuildEvent) error
}

// BuildProgress consolidates progress reporting
// Replaces: BuildProgressReporter, ExtendedBuildReporter
type BuildProgress struct {
	Stage      string
	Step       int
	Total      int
	Message    string
	Complete   bool
	Percentage float64
	Duration   time.Duration
	Details    map[string]interface{}
}

// DockerfileService handles Dockerfile operations
// Replaces: DockerfileValidator, DockerfileAnalyzer, DockerfileFixer
type DockerfileService interface {
	Validate(dockerfile string) error
	Analyze(dockerfile string) (*DockerfileAnalysis, error)
	Fix(dockerfile string) (string, error)
}

// SecurityService handles build security
// Replaces: SecurityChecksProvider, BuildRecoveryStrategyInterface
type SecurityService interface {
	ScanBuildContext(ctx context.Context, path string) (*SecurityReport, error)
	GetSecurityRecommendations(issues []SecurityIssue) []string
	RecoverFromSecurityIssues(ctx context.Context, issues []SecurityIssue) error
}

// NOTE: EventService has been merged into BuildService for simplicity.

// Supporting types for the unified interfaces
type BuildConfig struct {
	SessionID      string
	WorkspaceDir   string
	ImageName      string
	ImageTag       string
	DockerfilePath string
	BuildPath      string
	Platform       string
	NoCache        bool
	BuildArgs      map[string]string
	Labels         map[string]string
}

type BuildSession struct {
	ID            string
	Status        string
	StartTime     time.Time
	Duration      time.Duration
	Configuration BuildConfig
}

type SecurityScanResult struct {
	Vulnerabilities []SecurityIssue
	Recommendations []string
	RiskScore       int
}

type SecurityIssue struct {
	Type        string
	Severity    string
	Description string
	Fix         string
}

type SecurityReport struct {
	Issues          []SecurityIssue
	Score           int
	Recommendations []string
}

type BuildEvent struct {
	Type      string
	Message   string
	Timestamp time.Time
	Data      map[string]interface{}
}

type AnalysisResult struct {
	Language     string
	Framework    string
	Dependencies []string
	Issues       []string
	Score        int
}

type ContextAnalysis struct {
	Files       []string
	Size        int64
	Dockerfile  string
	Complexity  int
	Suggestions []string
}

type DockerfileAnalysis struct {
	Valid       bool
	Issues      []string
	Suggestions []string
	Score       int
}
