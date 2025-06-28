package types

import (
	"context"
	"fmt"
	"time"
)

type ToolMetadata struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Version      string            `json:"version"`
	Category     string            `json:"category"`
	Dependencies []string          `json:"dependencies"`
	Capabilities []string          `json:"capabilities"`
	Requirements []string          `json:"requirements"`
	Parameters   map[string]string `json:"parameters"`
	Examples     []ToolExample     `json:"examples"`
}

type ToolExample struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	Output      map[string]interface{} `json:"output"`
}

type ProgressStage struct {
	Name        string
	Weight      float64
	Description string
}

type MCPRequest struct {
	ID     string      `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

type MCPResponse struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ToolFactory func() interface{}

type ArgConverter func(args map[string]interface{}) (interface{}, error)

type ResultConverter func(result interface{}) (map[string]interface{}, error)

type SessionState struct {
	ID        string
	SessionID string
	CreatedAt time.Time
	UpdatedAt time.Time
	ExpiresAt time.Time

	WorkspaceDir string

	RepositoryAnalyzed bool
	RepositoryInfo     *RepositoryInfo
	RepoURL            string

	DockerfileGenerated bool
	DockerfilePath      string
	ImageBuilt          bool
	ImageRef            string
	ImagePushed         bool

	ManifestsGenerated  bool
	ManifestPaths       []string
	DeploymentValidated bool

	CurrentStage string
	Status       string
	Stage        string
	Errors       []string
	Metadata     map[string]interface{}

	SecurityScan *SecurityScanResult
}

type SessionMetadata struct {
	CreatedAt      time.Time `json:"created_at"`
	LastAccessedAt time.Time `json:"last_accessed_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	WorkspaceSize  int64     `json:"workspace_size"`
	OperationCount int       `json:"operation_count"`
	CurrentStage   string    `json:"current_stage"`
	Labels         []string  `json:"labels"`
}

type RepositoryInfo struct {
	Language     string   `json:"language"`
	Framework    string   `json:"framework"`
	Port         int      `json:"port"`
	Dependencies []string `json:"dependencies"`

	Structure FileStructure `json:"structure"`

	Size      int64 `json:"size"`
	HasCI     bool  `json:"has_ci"`
	HasReadme bool  `json:"has_readme"`

	CachedAt         time.Time     `json:"cached_at"`
	AnalysisDuration time.Duration `json:"analysis_duration"`

	Recommendations []string `json:"recommendations"`
}

type FileStructure struct {
	TotalFiles      int      `json:"total_files"`
	ConfigFiles     []string `json:"config_files"`
	EntryPoints     []string `json:"entry_points"`
	TestFiles       []string `json:"test_files"`
	BuildFiles      []string `json:"build_files"`
	DockerFiles     []string `json:"docker_files"`
	KubernetesFiles []string `json:"kubernetes_files"`
	PackageManagers []string `json:"package_managers"`
}

type SecurityScanResult struct {
	Success         bool               `json:"success"`
	ScannedAt       time.Time          `json:"scanned_at"`
	ImageRef        string             `json:"image_ref"`
	Scanner         string             `json:"scanner"`
	Vulnerabilities VulnerabilityCount `json:"vulnerabilities"`
	FixableCount    int                `json:"fixable_count"`
}

type VulnerabilityCount struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Unknown  int `json:"unknown"`
	Total    int `json:"total"`
}

type AIContext interface {
	GetAssessment() *UnifiedAssessment
	GenerateRecommendations() []Recommendation
	GetToolContext() *ToolContext
	GetMetadata() map[string]interface{}
}

type ScoreCalculator interface {
	CalculateScore(data interface{}) int
	DetermineRiskLevel(score int, factors map[string]interface{}) string
	CalculateConfidence(evidence []string) int
}

type TradeoffAnalyzer interface {
	AnalyzeTradeoffs(options []string, context map[string]interface{}) []TradeoffAnalysis
	CompareAlternatives(alternatives []AlternativeStrategy) *ComparisonMatrix
	RecommendBestOption(analysis []TradeoffAnalysis) *DecisionRecommendation
}

type UnifiedAssessment struct{}
type Recommendation struct{}
type ToolContext struct{}
type TradeoffAnalysis struct{}
type AlternativeStrategy struct{}
type ComparisonMatrix struct{}
type DecisionRecommendation struct{}

type IterativeFixer interface {
	Fix(ctx context.Context, issue interface{}) (*FixingResult, error)
	AttemptFix(ctx context.Context, issue interface{}, attempt int) (*FixingResult, error)
	SetMaxAttempts(max int)
	GetFixHistory() []FixAttempt
	GetFailureRouting() map[string]string
	GetFixStrategies() []string
}

type ContextSharer interface {
	ShareContext(ctx context.Context, key string, value interface{}) error
	GetSharedContext(ctx context.Context, key string) (interface{}, bool)
}

type FixingResult struct {
	Success         bool                   `json:"success"`
	Error           error                  `json:"error,omitempty"`
	FixApplied      string                 `json:"fix_applied"`
	Attempts        int                    `json:"attempts"`
	Duration        time.Duration          `json:"duration"`
	TotalDuration   time.Duration          `json:"total_duration"`
	TotalAttempts   int                    `json:"total_attempts"`
	FixHistory      []FixAttempt           `json:"fix_history"`
	AllAttempts     []FixAttempt           `json:"all_attempts"`
	FinalAttempt    *FixAttempt            `json:"final_attempt"`
	RecommendedNext []string               `json:"recommended_next"`
	Metadata        map[string]interface{} `json:"metadata"`
}

type FixStrategy struct {
	Name          string                             `json:"name"`
	Description   string                             `json:"description"`
	Type          string                             `json:"type"`
	Priority      int                                `json:"priority"`
	EstimatedTime time.Duration                      `json:"estimated_time"`
	Applicable    func(error) bool                   `json:"-"`
	Apply         func(context.Context, error) error `json:"-"`
	FileChanges   []FileChange                       `json:"file_changes,omitempty"`
	Commands      []string                           `json:"commands,omitempty"`
	Metadata      map[string]interface{}             `json:"metadata"`
}

type FileChange struct {
	FilePath   string `json:"file_path"`
	Operation  string `json:"operation"`
	Content    string `json:"content,omitempty"`
	NewContent string `json:"new_content,omitempty"`
	Reason     string `json:"reason"`
}

type FixableOperation interface {
	ExecuteOnce(ctx context.Context) error
	GetFailureAnalysis(ctx context.Context, err error) (*RichError, error)
	PrepareForRetry(ctx context.Context, fixAttempt *FixAttempt) error
	Execute(ctx context.Context) error
	CanRetry() bool
	GetLastError() error
}

// RichError provides detailed error information (DEPRECATED - use pkg/mcp.RichError)
type RichError struct {
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Type      string    `json:"type"`
	Severity  string    `json:"severity"`
	Timestamp time.Time `json:"timestamp"`
	Tool      string    `json:"tool"`

	// Context for compatibility with migration.go
	Context struct {
		Input struct {
			Args interface{} `json:"args"`
		} `json:"input"`
		Operation   string                 `json:"operation"`
		Stage       string                 `json:"stage"`
		Component   string                 `json:"component"`
		Metadata    map[string]interface{} `json:"metadata"`
		SystemState struct {
			DockerAvailable bool `json:"docker_available"`
			K8sConnected    bool `json:"k8s_connected"`
			DiskSpaceMB     int  `json:"disk_space_mb"`
			WorkspaceQuota  int  `json:"workspace_quota"`
		} `json:"system_state"`
		ResourceUsage struct {
			CPUPercent  float64 `json:"cpu_percent"`
			MemoryMB    int     `json:"memory_mb"`
			DiskUsageMB int     `json:"disk_usage_mb"`
		} `json:"resource_usage"`
		Logs         []interface{} `json:"logs"`
		RelatedFiles []string      `json:"related_files"`
	} `json:"context"`

	// Diagnostics for compatibility
	Diagnostics struct {
		RootCause     string        `json:"root_cause"`
		ErrorPattern  string        `json:"error_pattern"`
		Symptoms      []string      `json:"symptoms"`
		Category      string        `json:"category"`
		Checks        []interface{} `json:"checks"`
		SimilarErrors []interface{} `json:"similar_errors"`
	} `json:"diagnostics"`

	// Resolution for compatibility
	Resolution struct {
		RetryStrategy struct {
			Recommended     bool     `json:"recommended"`
			WaitTime        int      `json:"wait_time"`
			MaxAttempts     int      `json:"max_attempts"`
			BackoffStrategy string   `json:"backoff_strategy"`
			Conditions      []string `json:"conditions"`
		} `json:"retry_strategy"`
		ImmediateSteps []struct {
			Order       int    `json:"order"`
			Action      string `json:"action"`
			Description string `json:"description"`
			Command     string `json:"command"`
			Expected    string `json:"expected"`
		} `json:"immediate_steps"`
		Alternatives []struct {
			Name        string   `json:"name"`
			Description string   `json:"description"`
			Steps       []string `json:"steps"`
			Confidence  float64  `json:"confidence"`
		} `json:"alternatives"`
		Prevention  string   `json:"prevention"`
		ManualSteps []string `json:"manual_steps"`
	} `json:"resolution"`

	// Session state for compatibility
	SessionState *struct {
		ID           string `json:"id"`
		CurrentStage string `json:"current_stage"`
	} `json:"session_state,omitempty"`

	// Additional compatibility fields
	AttemptNumber  int      `json:"attempt_number,omitempty"`
	PreviousErrors []string `json:"previous_errors,omitempty"`
}

func (e *RichError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Code, e.Type, e.Message)
}

type FixAttempt struct {
	AttemptNumber  int                    `json:"attempt_number"`
	Strategy       string                 `json:"strategy"`
	FixStrategy    FixStrategy            `json:"fix_strategy"`
	Error          error                  `json:"error,omitempty"`
	Success        bool                   `json:"success"`
	Duration       time.Duration          `json:"duration"`
	StartTime      time.Time              `json:"start_time"`
	EndTime        time.Time              `json:"end_time"`
	AnalysisPrompt string                 `json:"analysis_prompt,omitempty"`
	AnalysisResult string                 `json:"analysis_result,omitempty"`
	Changes        []string               `json:"changes"`
	FixedContent   string                 `json:"fixed_content,omitempty"`
	Metadata       map[string]interface{} `json:"metadata"`
}

type BuildResult struct {
	ImageID  string      `json:"image_id"`
	ImageRef string      `json:"image_ref"`
	Success  bool        `json:"success"`
	Error    *BuildError `json:"error,omitempty"`
	Logs     string      `json:"logs,omitempty"`
}

type BuildError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type HealthCheckResult struct {
	Healthy     bool              `json:"healthy"`
	Status      string            `json:"status"`
	PodStatuses []PodStatus       `json:"pod_statuses"`
	Error       *HealthCheckError `json:"error,omitempty"`
}

type PodStatus struct {
	Name   string `json:"name"`
	Ready  bool   `json:"ready"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type HealthCheckError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type PipelineOperations interface {
	GetSessionWorkspace(sessionID string) string
	UpdateSessionFromDockerResults(sessionID string, result interface{}) error

	BuildDockerImage(sessionID, imageRef, dockerfilePath string) (*BuildResult, error)
	PullDockerImage(sessionID, imageRef string) error
	PushDockerImage(sessionID, imageRef string) error
	TagDockerImage(sessionID, sourceRef, targetRef string) error
	ConvertToDockerState(sessionID string) (*DockerState, error)

	GenerateKubernetesManifests(sessionID, imageRef, appName string, port int, cpuRequest, memoryRequest, cpuLimit, memoryLimit string) (*KubernetesManifestResult, error)
	DeployToKubernetes(sessionID string, manifests []string) (*KubernetesDeploymentResult, error)
	CheckApplicationHealth(sessionID, namespace, deploymentName string, timeout time.Duration) (*HealthCheckResult, error)

	AcquireResource(sessionID, resourceType string) error
	ReleaseResource(sessionID, resourceType string) error
}

type ToolSessionManager interface {
	GetSession(sessionID string) (interface{}, error)
	GetSessionInterface(sessionID string) (interface{}, error)
	GetOrCreateSession(sessionID string) (interface{}, error)
	GetOrCreateSessionFromRepo(repoURL string) (interface{}, error)
	UpdateSession(sessionID string, updateFunc func(interface{})) error
	DeleteSession(ctx context.Context, sessionID string) error

	ListSessions(ctx context.Context, filter map[string]interface{}) ([]interface{}, error)
	FindSessionByRepo(ctx context.Context, repoURL string) (interface{}, error)
}

func UpdateSessionHelper[T any](manager ToolSessionManager, sessionID string, updater func(*T)) error {
	return manager.UpdateSession(sessionID, func(s interface{}) {
		if session, ok := s.(*T); ok {
			updater(session)
		}
	})
}

type DockerState struct {
	Images     []string `json:"images"`
	Containers []string `json:"containers"`
	Networks   []string `json:"networks"`
	Volumes    []string `json:"volumes"`
}

type KubernetesManifestResult struct {
	Success   bool                `json:"success"`
	Manifests []GeneratedManifest `json:"manifests"`
	Error     *RichError          `json:"error,omitempty"`
}

type GeneratedManifest struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

type KubernetesDeploymentResult struct {
	Success     bool       `json:"success"`
	Namespace   string     `json:"namespace"`
	Deployments []string   `json:"deployments"`
	Services    []string   `json:"services"`
	Error       *RichError `json:"error,omitempty"`
}

const (
	ErrorCodeInvalidRequest = -32600
)

type AIAnalyzer interface {
	Analyze(ctx context.Context, prompt string) (string, error)
	AnalyzeWithFileTools(ctx context.Context, prompt, baseDir string) (string, error)
	AnalyzeWithFormat(ctx context.Context, promptTemplate string, args ...interface{}) (string, error)
	GetTokenUsage() TokenUsage
	ResetTokenUsage()
}

type TokenUsage struct {
	CompletionTokens int `json:"completion_tokens"`
	PromptTokens     int `json:"prompt_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type SystemResources struct {
	CPUUsage    float64   `json:"cpu_usage_percent"`
	MemoryUsage float64   `json:"memory_usage_percent"`
	DiskUsage   float64   `json:"disk_usage_percent"`
	OpenFiles   int       `json:"open_files"`
	GoRoutines  int       `json:"goroutines"`
	HeapSize    int64     `json:"heap_size_bytes"`
	LastUpdated time.Time `json:"last_updated"`
}

type SessionHealthStats struct {
	ActiveSessions    int     `json:"active_sessions"`
	TotalSessions     int     `json:"total_sessions"`
	FailedSessions    int     `json:"failed_sessions"`
	AverageSessionAge float64 `json:"average_session_age_minutes"`
	SessionErrors     int     `json:"session_errors_last_hour"`
}

type CircuitBreakerStatus struct {
	State         string    `json:"state"`
	FailureCount  int       `json:"failure_count"`
	LastFailure   time.Time `json:"last_failure"`
	NextRetry     time.Time `json:"next_retry"`
	TotalRequests int64     `json:"total_requests"`
	SuccessCount  int64     `json:"success_count"`
}

type ServiceHealth struct {
	Name         string                 `json:"name"`
	Status       string                 `json:"status"`
	LastCheck    time.Time              `json:"last_check"`
	ResponseTime time.Duration          `json:"response_time"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type JobQueueStats struct {
	QueuedJobs      int     `json:"queued_jobs"`
	RunningJobs     int     `json:"running_jobs"`
	CompletedJobs   int64   `json:"completed_jobs"`
	FailedJobs      int64   `json:"failed_jobs"`
	AverageWaitTime float64 `json:"average_wait_time_seconds"`
}

type RecentError struct {
	Timestamp time.Time              `json:"timestamp"`
	Message   string                 `json:"message"`
	Component string                 `json:"component"`
	Severity  string                 `json:"severity"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

type ProgressTracker interface {
	RunWithProgress(
		ctx context.Context,
		operation string,
		stages []LocalProgressStage,
		fn func(ctx context.Context, reporter interface{}) error,
	) error
}

type LocalProgressStage struct {
	Name        string
	Weight      float64
	Description string
}

type SessionData struct {
	ID           string                 `json:"id"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	ExpiresAt    time.Time              `json:"expires_at"`
	CurrentStage string                 `json:"current_stage"`
	Metadata     map[string]interface{} `json:"metadata"`
	IsActive     bool                   `json:"is_active"`
	LastAccess   time.Time              `json:"last_access"`
}

type SessionManagerStats struct {
	TotalSessions   int     `json:"total_sessions"`
	ActiveSessions  int     `json:"active_sessions"`
	ExpiredSessions int     `json:"expired_sessions"`
	AverageAge      float64 `json:"average_age_hours"`
	OldestSession   string  `json:"oldest_session_id"`
	NewestSession   string  `json:"newest_session_id"`
}

type BaseAnalysisOptions struct {
	Depth                   string
	Aspects                 []string
	GenerateRecommendations bool
	CustomParams            map[string]interface{}
}

type BaseValidationOptions struct {
	Severity     string
	IgnoreRules  []string
	StrictMode   bool
	CustomParams map[string]interface{}
}

type BaseAnalysisResult struct {
	Summary         BaseAnalysisSummary
	Findings        []BaseFinding
	Recommendations []BaseRecommendation
	Metrics         map[string]interface{}
	RiskAssessment  BaseRiskAssessment
	Context         map[string]interface{}
	Metadata        BaseAnalysisMetadata
}

type BaseValidationResult struct {
	IsValid bool
	Score   int

	Errors   []BaseValidationError
	Warnings []BaseValidationWarning

	TotalIssues    int
	CriticalIssues int

	Context  map[string]interface{}
	Metadata BaseValidationMetadata
}

type BaseAnalyzerCapabilities struct {
	SupportedTypes   []string
	SupportedAspects []string
	RequiresContext  bool
	SupportsDeepScan bool
}

type BaseAnalysisSummary struct {
	TotalFindings    int
	CriticalFindings int
	Strengths        []string
	Weaknesses       []string
	OverallScore     int
}

type BaseFinding struct {
	ID          string
	Type        string
	Category    string
	Severity    string
	Title       string
	Description string
	Evidence    []string
	Impact      string
	Location    BaseFindingLocation
}

type BaseFindingLocation struct {
	File      string
	Line      int
	Component string
	Context   string
}

type BaseRecommendation struct {
	ID          string
	Priority    string
	Category    string
	Title       string
	Description string
	Benefits    []string
	Effort      string
	Impact      string
}

type BaseRiskAssessment struct {
	OverallRisk string
	RiskFactors []BaseRiskFactor
	Mitigations []BaseMitigation
}

type BaseRiskFactor struct {
	ID          string
	Category    string
	Description string
	Likelihood  string
	Impact      string
	Score       int
}

type BaseMitigation struct {
	RiskID        string
	Description   string
	Effort        string
	Effectiveness string
}

type BaseAnalysisMetadata struct {
	AnalyzerName    string
	AnalyzerVersion string
	Duration        time.Duration
	Timestamp       time.Time
	Parameters      map[string]interface{}
}

type BaseValidationError struct {
	Code          string
	Type          string
	Message       string
	Severity      string
	Location      BaseErrorLocation
	Fix           string
	Documentation string
}

type BaseValidationWarning struct {
	Code       string
	Type       string
	Message    string
	Suggestion string
	Impact     string
	Location   BaseWarningLocation
}

type BaseErrorLocation struct {
	File   string
	Line   int
	Column int
	Path   string
}

type BaseWarningLocation struct {
	File string
	Line int
	Path string
}

type BaseValidationMetadata struct {
	ValidatorName    string
	ValidatorVersion string
	Duration         time.Duration
	Timestamp        time.Time
	Parameters       map[string]interface{}
}

// =============================================================================
// ERROR HANDLING COMPATIBILITY TYPES
// =============================================================================

// RichError is defined above for compatibility with internal packages

// NewRichError creates a new RichError with basic information
func NewRichError(code, message, errorType string) *RichError {
	return &RichError{
		Code:      code,
		Message:   message,
		Type:      errorType,
		Severity:  "medium",
		Timestamp: time.Now(),
	}
}

// ErrorBuilder provides simplified error building for internal packages
type ErrorBuilder struct {
	error *RichError
}

// NewErrorBuilder creates a new error builder
func NewErrorBuilder(code, message, errorType string) *ErrorBuilder {
	return &ErrorBuilder{
		error: NewRichError(code, message, errorType),
	}
}

// WithField adds a field (simplified implementation)
func (eb *ErrorBuilder) WithField(key string, value interface{}) *ErrorBuilder {
	// For now, just return the builder without storing the field
	return eb
}

// WithOperation sets operation (simplified implementation)
func (eb *ErrorBuilder) WithOperation(operation string) *ErrorBuilder {
	return eb
}

// WithStage sets stage (simplified implementation)
func (eb *ErrorBuilder) WithStage(stage string) *ErrorBuilder {
	return eb
}

// WithSeverity sets severity (simplified implementation)
func (eb *ErrorBuilder) WithSeverity(severity string) *ErrorBuilder {
	eb.error.Severity = severity
	return eb
}

// WithRootCause sets root cause (simplified implementation)
func (eb *ErrorBuilder) WithRootCause(cause string) *ErrorBuilder {
	return eb
}

// WithImmediateStep adds immediate step (simplified implementation)
func (eb *ErrorBuilder) WithImmediateStep(order int, action, description string) *ErrorBuilder {
	return eb
}

// Build returns the constructed error
func (eb *ErrorBuilder) Build() *RichError {
	return eb.error
}
