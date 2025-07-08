package analyze

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/core/analysis"
	"github.com/Azure/container-kit/pkg/core/git"
	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/core"
	internalcommon "github.com/Azure/container-kit/pkg/mcp/internal/common"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"

	internaltypes "github.com/Azure/container-kit/pkg/mcp/core"
	validation "github.com/Azure/container-kit/pkg/mcp/security"

	"github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/rs/zerolog"
)

// Register tools with global registry
func init() {
	core.RegisterTool("atomic_analyze_repository", func() api.Tool {
		return &AtomicAnalyzeRepositoryTool{}
	})
}

type AtomicAnalyzeRepositoryArgs struct {
	internaltypes.BaseToolArgs
	DryRun               bool     `json:"dry_run,omitempty" description:"Preview changes without executing"`
	SessionID            string   `json:"session_id,omitempty" validate:"omitempty,session_id" description:"Session ID for state correlation"`
	RepoURL              string   `json:"repo_url" validate:"required,git_url" description:"Repository URL (GitHub, GitLab, etc.) or local path"`
	Branch               string   `json:"branch,omitempty" validate:"omitempty,git_branch" description:"Git branch to analyze (default: main)"`
	Context              string   `json:"context,omitempty" validate:"omitempty,max=1000" description:"Additional context about the application"`
	LanguageHint         string   `json:"language_hint,omitempty" validate:"omitempty,language" description:"Primary programming language hint"`
	Shallow              bool     `json:"shallow,omitempty" description:"Perform shallow clone for faster analysis"`
	IncludeDependencies  bool     `json:"include_dependencies,omitempty" description:"Include dependency analysis"`
	IncludeSecurityScan  bool     `json:"include_security_scan,omitempty" description:"Include security vulnerability scan"`
	IncludeBuildAnalysis bool     `json:"include_build_analysis,omitempty" description:"Include build system analysis"`
	LanguageHints        []string `json:"language_hints,omitempty" validate:"omitempty,dive,language" description:"Programming language hints"`
}

type AtomicAnalysisResult struct {
	types.BaseToolResponse
	core.BaseAIContextResult
	Success bool `json:"success"`

	SessionID    string `json:"session_id"`
	WorkspaceDir string `json:"workspace_dir"`

	RepoURL  string `json:"repo_url"`
	Branch   string `json:"branch"`
	CloneDir string `json:"clone_dir"`

	Analysis *analysis.AnalysisResult `json:"analysis"`

	CloneResult *git.CloneResult `json:"clone_result,omitempty"`

	CloneDuration    time.Duration `json:"clone_duration"`
	AnalysisDuration time.Duration `json:"analysis_duration"`
	TotalDuration    time.Duration `json:"total_duration"`

	AnalysisContext *AnalysisContext `json:"analysis_context"`

	ContainerizationAssessment *ContainerizationAssessment `json:"containerization_assessment"`
}

type AtomicAnalyzeRepositoryTool struct {
	pipelineAdapter  core.TypedPipelineOperations
	sessionStore     services.SessionStore
	sessionState     services.SessionState
	logger           *slog.Logger
	gitManager       *git.Manager
	repoAnalyzer     *analysis.RepositoryAnalyzer
	repoCloner       *git.Manager
	contextGenerator *ContextGenerator
}

// NewAtomicAnalyzeRepositoryToolWithServices creates a new atomic analyze repository tool using service interfaces
func NewAtomicAnalyzeRepositoryToolWithServices(adapter core.TypedPipelineOperations, serviceContainer services.ServiceContainer, logger *slog.Logger) *AtomicAnalyzeRepositoryTool {
	toolLogger := logger.With("tool", "atomic_analyze_repository")

	return &AtomicAnalyzeRepositoryTool{
		pipelineAdapter:  adapter,
		sessionStore:     serviceContainer.SessionStore(),
		sessionState:     serviceContainer.SessionState(),
		logger:           toolLogger,
		gitManager:       git.NewManager(toolLogger),
		repoAnalyzer:     analysis.NewRepositoryAnalyzer(toolLogger),
		repoCloner:       git.NewManager(toolLogger),
		contextGenerator: NewContextGenerator(toolLogger),
	}
}

func (t *AtomicAnalyzeRepositoryTool) ExecuteRepositoryAnalysis(ctx context.Context, args AtomicAnalyzeRepositoryArgs) (*AtomicAnalysisResult, error) {
	return t.executeWithoutProgress(ctx, args)
}

func (t *AtomicAnalyzeRepositoryTool) executeWithoutProgress(ctx context.Context, args AtomicAnalyzeRepositoryArgs) (*AtomicAnalysisResult, error) {
	return t.performAnalysis(ctx, args, nil)
}

func (t *AtomicAnalyzeRepositoryTool) performAnalysis(ctx context.Context, args AtomicAnalyzeRepositoryArgs, reporter interface{}) (*AtomicAnalysisResult, error) {
	startTime := time.Now()

	// Pipeline Stage 1: Session Initialization
	session, result, err := t.initializeSession(args, startTime)
	if err != nil {
		return result, err
	}

	// Pipeline Stage 2: Early Return for Dry Run
	if args.DryRun {
		return t.handleDryRun(result, startTime), nil
	}

	// Pipeline Stage 3: Repository Setup (clone or validate local)
	if err := t.setupRepository(ctx, args, session, result); err != nil {
		return t.finalizeWithError(result, startTime, err), err
	}

	// Pipeline Stage 4: Cache Check
	if cachedResult := t.checkCache(session, result, startTime); cachedResult != nil {
		return cachedResult, nil
	}

	// Pipeline Stage 5: Fresh Analysis
	if err := t.performFreshAnalysis(args, session, result); err != nil {
		return t.finalizeWithError(result, startTime, err), err
	}

	// Pipeline Stage 6: Assessment and Finalization
	return t.finalizeAnalysis(session, result, startTime), nil
}

// handleDryRun returns early result for dry run mode
func (t *AtomicAnalyzeRepositoryTool) handleDryRun(result *AtomicAnalysisResult, startTime time.Time) *AtomicAnalysisResult {
	result.AnalysisContext.NextStepSuggestions = []string{
		"This is a dry-run - actual repository cloning and analysis would be performed",
		"Session workspace would be created at: " + result.WorkspaceDir,
	}
	result.TotalDuration = time.Since(startTime)
	return result
}

// setupRepository handles repository cloning or local path validation
func (t *AtomicAnalyzeRepositoryTool) setupRepository(ctx context.Context, args AtomicAnalyzeRepositoryArgs, session *core.SessionState, result *AtomicAnalysisResult) error {
	localPath := t.resolveRepositoryPath(args.RepoURL)

	if t.isURL(args.RepoURL) && !strings.HasPrefix(args.RepoURL, "file://") {
		return t.cloneRemoteRepository(ctx, args, session, result)
	}

	return t.validateLocalRepository(localPath, session, result)
}

// resolveRepositoryPath converts file:// URLs to local paths
func (t *AtomicAnalyzeRepositoryTool) resolveRepositoryPath(repoURL string) string {
	if strings.HasPrefix(repoURL, "file://") {
		localPath := strings.TrimPrefix(repoURL, "file://")
		t.logger.Info("Converting file:// URL to local path",
			"original_url", repoURL,
			"local_path", localPath)
		return localPath
	}
	return repoURL
}

// cloneRemoteRepository handles remote repository cloning
func (t *AtomicAnalyzeRepositoryTool) cloneRemoteRepository(ctx context.Context, args AtomicAnalyzeRepositoryArgs, session *core.SessionState, result *AtomicAnalysisResult) error {
	cloneResult, err := t.cloneRepository(ctx, session.SessionID, args)
	result.CloneResult = cloneResult
	if cloneResult != nil {
		result.CloneDuration = cloneResult.Duration
		result.CloneDir = cloneResult.RepoPath
	}

	if err != nil {
		t.logger.Error("Repository clone failed",
			"error", err,
			"repo_url", args.RepoURL,
			"session_id", session.SessionID)
		return errors.NewError().
			Code(errors.CodeResourceNotFound).
			Type(errors.ErrTypeResource).
			Severity(errors.SeverityHigh).
			Message("Failed to clone repository").
			Context("module", "analyze/repository-atomic").
			Context("repo_url", args.RepoURL).
			Context("branch", args.Branch).
			Context("session_id", session.SessionID).
			Cause(err).
			Suggestion("Check repository URL and network connectivity").
			WithLocation().
			Build()
	}

	t.logger.Info("Repository cloned successfully",
		"session_id", session.SessionID,
		"clone_dir", result.CloneDir,
		"clone_duration", result.CloneDuration)

	return nil
}

// validateLocalRepository handles local path validation
func (t *AtomicAnalyzeRepositoryTool) validateLocalRepository(localPath string, session *core.SessionState, result *AtomicAnalysisResult) error {
	if err := internalcommon.NewPathUtils().ValidateLocalPath(localPath); err != nil {
		t.logger.Error("Invalid local path for repository",
			"error", err,
			"local_path", localPath,
			"session_id", session.SessionID)
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Message("Invalid local path for repository").
			Context("module", "analyze/repository-atomic").
			Context("local_path", localPath).
			Context("session_id", session.SessionID).
			Cause(err).
			Suggestion("Provide a valid local path to the repository").
			WithLocation().
			Build()
	}
	result.CloneDir = localPath
	return nil
}

// performFreshAnalysis executes a new repository analysis
func (t *AtomicAnalyzeRepositoryTool) performFreshAnalysis(args AtomicAnalyzeRepositoryArgs, session *core.SessionState, result *AtomicAnalysisResult) error {
	analysisStartTime := time.Now()
	analysisOpts := AnalysisOptions{
		RepoPath:     result.CloneDir,
		Context:      args.Context,
		LanguageHint: args.LanguageHint,
		SessionID:    session.SessionID,
	}

	coreAnalysisResult, err := t.repoAnalyzer.AnalyzeRepository(analysisOpts.RepoPath)
	if err != nil {
		t.logger.Error("Repository analysis failed",
			"error", err,
			"clone_dir", result.CloneDir,
			"session_id", session.SessionID,
			"is_local", !t.isURL(args.RepoURL))
		return errors.NewError().
			Code(errors.CodeValidationFailed).
			Type(errors.ErrTypeBusiness).
			Severity(errors.SeverityHigh).
			Message("Failed to analyze repository").
			Context("module", "analyze/repository-atomic").
			Context("repo_url", args.RepoURL).
			Context("clone_dir", result.CloneDir).
			Context("session_id", session.SessionID).
			Context("is_local", !t.isURL(args.RepoURL)).
			Cause(err).
			Suggestion("Check repository structure and ensure it contains valid source code").
			WithLocation().
			Build()
	}

	repoAnalysisResult := &AnalysisResult{
		AnalysisResult: coreAnalysisResult,
		Duration:       time.Since(analysisStartTime),
		Context:        t.generateAnalysisContext(analysisOpts.RepoPath, coreAnalysisResult),
	}

	result.Analysis = repoAnalysisResult.AnalysisResult
	result.AnalysisContext = repoAnalysisResult.Context
	result.AnalysisDuration = time.Since(analysisStartTime)

	return nil
}

// finalizeAnalysis generates assessment and updates session state
func (t *AtomicAnalyzeRepositoryTool) finalizeAnalysis(session *core.SessionState, result *AtomicAnalysisResult, startTime time.Time) *AtomicAnalysisResult {
	assessment, err := t.contextGenerator.GenerateContainerizationAssessment(result.Analysis, result.AnalysisContext)
	if err != nil {
		t.logger.Warn("Failed to generate containerization assessment", "error", err)
	} else {
		result.ContainerizationAssessment = assessment
	}

	if err := t.updateSessionState(session, result); err != nil {
		t.logger.Warn("Failed to update session state", "error", err)
	}

	result.Success = true
	result.TotalDuration = time.Since(startTime)
	result.Duration = result.TotalDuration

	t.logger.Info("Atomic repository analysis completed successfully",
		"session_id", session.SessionID,
		"language", result.Analysis.Language,
		"framework", result.Analysis.Framework,
		"files_analyzed", result.AnalysisContext.FilesAnalyzed,
		"total_duration", result.TotalDuration)

	return result
}

// finalizeWithError sets error state and timing
func (t *AtomicAnalyzeRepositoryTool) finalizeWithError(result *AtomicAnalysisResult, startTime time.Time, _ error) *AtomicAnalysisResult {
	result.Success = false
	result.TotalDuration = time.Since(startTime)
	return result
}

func (t *AtomicAnalyzeRepositoryTool) isURL(path string) bool {
	return strings.HasPrefix(path, "http://") ||
		strings.HasPrefix(path, "https://") ||
		strings.HasPrefix(path, "git@") ||
		strings.HasPrefix(path, "ssh://") ||
		strings.HasPrefix(path, "file://")
}

func (t *AtomicAnalyzeRepositoryTool) generateAnalysisContext(repoPath string, analysis *analysis.AnalysisResult) *AnalysisContext {
	return &AnalysisContext{
		FilesAnalyzed:               len(analysis.ConfigFiles),
		ConfigFilesFound:            []string{},
		EntryPointsFound:            analysis.EntryPoints,
		TestFilesFound:              []string{},
		BuildFilesFound:             analysis.BuildFiles,
		PackageManagers:             []string{},
		DatabaseFiles:               []string{},
		DockerFiles:                 []string{},
		K8sFiles:                    []string{},
		HasGitIgnore:                false,
		HasReadme:                   false,
		HasLicense:                  false,
		HasCI:                       false,
		RepositorySize:              0,
		ContainerizationSuggestions: []string{},
		NextStepSuggestions:         []string{},
	}
}

func (t *AtomicAnalyzeRepositoryTool) GetMetadata() api.ToolMetadata {
	return api.ToolMetadata{
		Name:         "atomic_analyze_repository",
		Description:  "Analyzes repository structure, detects programming language, framework, and generates containerization recommendations. Creates a new session to track the analysis workflow",
		Version:      "1.0.0",
		Category:     api.ToolCategory("analysis"),
		Tags:         []string{"analysis", "repository", "atomic"},
		Status:       api.ToolStatus("active"),
		Dependencies: []string{"git"},
		Capabilities: []string{
			"supports_streaming",
			"repository_analysis",
		},
		Requirements: []string{"git_access"},
		RegisteredAt: time.Now(),
		LastModified: time.Now(),
	}
}

func (t *AtomicAnalyzeRepositoryTool) Validate(ctx context.Context, args interface{}) error {
	// Validate using tag-based validation
	return validation.ValidateTaggedStruct(args)
}

// Name implements the core.Tool interface
func (t *AtomicAnalyzeRepositoryTool) Name() string {
	return "atomic_analyze_repository"
}

// Description implements the core.Tool interface
func (t *AtomicAnalyzeRepositoryTool) Description() string {
	return "Analyzes repository structure, detects programming language, framework, and generates containerization recommendations. Creates a new session to track the analysis workflow"
}

// Schema implements the api.Tool interface
func (t *AtomicAnalyzeRepositoryTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        t.Name(),
		Description: t.Description(),
		Version:     "1.0.0",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for tracking the analysis workflow",
				},
				"repo_url": map[string]interface{}{
					"type":        "string",
					"description": "Repository URL (GitHub, GitLab, etc.) or local path",
				},
				"branch": map[string]interface{}{
					"type":        "string",
					"description": "Git branch to analyze (default: main)",
				},
				"context": map[string]interface{}{
					"type":        "string",
					"description": "Additional context about the application",
				},
				"language_hint": map[string]interface{}{
					"type":        "string",
					"description": "Primary programming language hint",
				},
				"shallow": map[string]interface{}{
					"type":        "boolean",
					"description": "Perform shallow clone for faster analysis",
				},
			},
			"required": []string{"repo_url"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type": "boolean",
				},
				"language": map[string]interface{}{
					"type": "string",
				},
				"framework": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}
}

// ExecuteTypedInterface executes with typed parameters (for typed tool interface)
func (t *AtomicAnalyzeRepositoryTool) ExecuteTypedInterface(ctx context.Context, params interface{}) (interface{}, error) {
	// Convert typed params to internal args format
	var args AtomicAnalyzeRepositoryArgs

	// Type assert the params to expected type
	switch p := params.(type) {
	case map[string]interface{}:
		// Handle generic map input
		if sessionID, ok := p["session_id"].(string); ok {
			args.SessionID = sessionID
		}
		if repoURL, ok := p["repo_url"].(string); ok {
			args.RepoURL = repoURL
		}
		if branch, ok := p["branch"].(string); ok {
			args.Branch = branch
		}
		if context, ok := p["context"].(string); ok {
			args.Context = context
		}
		if languageHint, ok := p["language_hint"].(string); ok {
			args.LanguageHint = languageHint
		}
		if shallow, ok := p["shallow"].(bool); ok {
			args.Shallow = shallow
		}
	default:
		// For now, create a simple conversion - this would be enhanced based on actual typed parameter types
		args = AtomicAnalyzeRepositoryArgs{
			SessionID: "default", // Would be properly extracted from typed params
			RepoURL:   "",        // Would be properly extracted from typed params
			Branch:    "main",    // Default branch
		}
	}

	// Execute using existing implementation
	result, err := t.ExecuteRepositoryAnalysis(ctx, args)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// slogToZerolog creates a zerolog logger from an slog logger for compatibility
// This is a temporary adapter during the migration from zerolog to slog
func slogToZerolog(slogLogger *slog.Logger) zerolog.Logger {
	// Create a no-op zerolog logger for now
	// In a full migration, this would properly bridge the two loggers
	return zerolog.Nop()
}

// Execute implements the api.Tool interface
func (t *AtomicAnalyzeRepositoryTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	// Convert ToolInput to our internal args format
	var analyzeArgs AtomicAnalyzeRepositoryArgs

	// Extract session ID from input
	analyzeArgs.SessionID = input.SessionID

	// Convert input data to our args struct
	if err := t.convertToolInputToArgs(input, &analyzeArgs); err != nil {
		return t.createErrorOutput(err), err
	}

	// Use existing execution implementation
	result, err := t.executeWithoutProgress(ctx, analyzeArgs)
	if err != nil {
		return t.createErrorOutput(err), err
	}

	// Convert result to ToolOutput
	return t.convertResultToToolOutput(result), nil
}

func (t *AtomicAnalyzeRepositoryTool) GetVersion() string {
	return t.GetMetadata().Version
}

type ToolCapabilities struct {
	SupportsDryRun    bool
	SupportsStreaming bool
	IsLongRunning     bool
	RequiresAuth      bool
}

func (t *AtomicAnalyzeRepositoryTool) GetCapabilities() ToolCapabilities {
	return ToolCapabilities{
		SupportsDryRun:    true,
		SupportsStreaming: true,
		IsLongRunning:     true,
		RequiresAuth:      false,
	}
}

// Helper methods for the new Tool interface
func (t *AtomicAnalyzeRepositoryTool) convertToolInputToArgs(input api.ToolInput, args *AtomicAnalyzeRepositoryArgs) error {
	// Extract session ID
	args.SessionID = input.SessionID

	// Extract parameters from data map
	if repoURL, ok := input.Data["repo_url"].(string); ok {
		args.RepoURL = repoURL
	}

	if branch, ok := input.Data["branch"].(string); ok {
		args.Branch = branch
	}

	if context, ok := input.Data["context"].(string); ok {
		args.Context = context
	}

	if languageHint, ok := input.Data["language_hint"].(string); ok {
		args.LanguageHint = languageHint
	}

	if shallow, ok := input.Data["shallow"].(bool); ok {
		args.Shallow = shallow
	}

	if dryRun, ok := input.Data["dry_run"].(bool); ok {
		args.DryRun = dryRun
	}

	// Optional boolean flags
	if includeDeps, ok := input.Data["include_dependencies"].(bool); ok {
		args.IncludeDependencies = includeDeps
	}

	if includeSecurity, ok := input.Data["include_security_scan"].(bool); ok {
		args.IncludeSecurityScan = includeSecurity
	}

	if includeBuild, ok := input.Data["include_build_analysis"].(bool); ok {
		args.IncludeBuildAnalysis = includeBuild
	}

	// Handle language hints array
	if hints, ok := input.Data["language_hints"].([]interface{}); ok {
		for _, hint := range hints {
			if hintStr, ok := hint.(string); ok {
				args.LanguageHints = append(args.LanguageHints, hintStr)
			}
		}
	}

	return nil
}

func (t *AtomicAnalyzeRepositoryTool) createErrorOutput(err error) api.ToolOutput {
	return api.ToolOutput{
		Success: false,
		Data:    map[string]interface{}{},
		Error:   err.Error(),
	}
}

func (t *AtomicAnalyzeRepositoryTool) convertResultToToolOutput(result *AtomicAnalysisResult) api.ToolOutput {
	// Extract data from nested Analysis field
	var language, framework string
	var dependencies []interface{}

	if result.Analysis != nil {
		language = result.Analysis.Language
		framework = result.Analysis.Framework
		// Convert dependencies to interface slice
		for _, dep := range result.Analysis.Dependencies {
			dependencies = append(dependencies, dep)
		}
	}

	return api.ToolOutput{
		Success: result.Success,
		Data: map[string]interface{}{
			"session_id":    result.SessionID,
			"workspace_dir": result.WorkspaceDir,
			"language":      language,
			"framework":     framework,
			"dependencies":  dependencies,
		},
	}
}
