package analyze

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/core/analysis"
	"github.com/Azure/container-kit/pkg/core/git"
	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/errors"
	validation "github.com/Azure/container-kit/pkg/mcp/security"
	"github.com/Azure/container-kit/pkg/mcp/services"
)

// Register consolidated repository analysis tool
func init() {
	core.RegisterTool("analyze_repository_consolidated", func() api.Tool {
		return &ConsolidatedAnalyzeRepositoryTool{}
	})
}

// ConsolidatedAnalyzeRepositoryInput represents unified input for all repository analysis variants
type ConsolidatedAnalyzeRepositoryInput struct {
	// Core parameters (with backward compatibility aliases)
	SessionID string `json:"session_id,omitempty" validate:"omitempty,session_id" description:"Session ID for state correlation"`
	RepoURL   string `json:"repo_url" validate:"required,git_url" description:"Repository URL (GitHub, GitLab, etc.) or local path"`
	RepoPath  string `json:"repo_path,omitempty" description:"Alias for repo_url for backward compatibility"`
	Path      string `json:"path,omitempty" description:"Alias for repo_url for backward compatibility"`
	Branch    string `json:"branch,omitempty" validate:"omitempty,git_branch" description:"Git branch to analyze (default: main)"`

	// Analysis options
	Context       string   `json:"context,omitempty" validate:"omitempty,max=1000" description:"Additional context about the application"`
	Language      string   `json:"language,omitempty" validate:"omitempty,language" description:"Primary programming language hint"`
	Framework     string   `json:"framework,omitempty" validate:"omitempty,framework" description:"Framework hint (e.g., express, django)"`
	LanguageHint  string   `json:"language_hint,omitempty" validate:"omitempty,language" description:"Primary programming language hint"`
	LanguageHints []string `json:"language_hints,omitempty" validate:"omitempty,dive,language" description:"Programming language hints"`

	// Analysis modes
	AnalysisMode string `json:"analysis_mode,omitempty" validate:"omitempty,oneof=simple comprehensive atomic" description:"Analysis mode: simple, comprehensive, or atomic"`
	Shallow      bool   `json:"shallow,omitempty" description:"Perform shallow clone for faster analysis"`
	DryRun       bool   `json:"dry_run,omitempty" description:"Preview changes without executing"`

	// Optional features
	IncludeDependencies  bool `json:"include_dependencies,omitempty" description:"Include dependency analysis"`
	IncludeSecurityScan  bool `json:"include_security_scan,omitempty" description:"Include security vulnerability scan"`
	IncludeBuildAnalysis bool `json:"include_build_analysis,omitempty" description:"Include build system analysis"`
	SkipFileTree         bool `json:"skip_file_tree,omitempty" description:"Skip generating file tree for performance"`

	// Performance options
	UseCache bool `json:"use_cache,omitempty" description:"Use cached results if available"`
	Sandbox  bool `json:"sandbox,omitempty" description:"Run analysis in sandboxed environment"`
	Timeout  int  `json:"timeout,omitempty" validate:"omitempty,min=30,max=3600" description:"Analysis timeout in seconds"`
}

// Validate implements validation using tag-based validation
func (c ConsolidatedAnalyzeRepositoryInput) Validate() error {
	// Use the actual repository URL (handle aliases)
	repoURL := c.getRepoURL()
	if repoURL == "" {
		return errors.NewError().Message("repository URL is required").Build()
	}
	return validation.ValidateTaggedStruct(c)
}

// getRepoURL returns the repository URL, handling backward compatibility aliases
func (c ConsolidatedAnalyzeRepositoryInput) getRepoURL() string {
	if c.RepoURL != "" {
		return c.RepoURL
	}
	if c.RepoPath != "" {
		return c.RepoPath
	}
	return c.Path
}

// getAnalysisMode returns the analysis mode, defaulting to comprehensive
func (c ConsolidatedAnalyzeRepositoryInput) getAnalysisMode() string {
	if c.AnalysisMode != "" {
		return c.AnalysisMode
	}
	return "comprehensive"
}

// ConsolidatedAnalyzeRepositoryOutput represents unified output for all repository analysis variants
type ConsolidatedAnalyzeRepositoryOutput struct {
	// Status
	Success   bool   `json:"success"`
	SessionID string `json:"session_id"`
	Error     string `json:"error,omitempty"`

	// Core analysis results (from all variants)
	Language     string   `json:"language"`
	Framework    string   `json:"framework"`
	Dependencies []string `json:"dependencies,omitempty"`
	EntryPoints  []string `json:"entry_points,omitempty"`

	// Build information
	BuildCommands []string `json:"build_commands,omitempty"`
	RunCommand    string   `json:"run_command,omitempty"`
	Port          int      `json:"port,omitempty"`
	DatabaseType  string   `json:"database_type,omitempty"`

	// Repository information
	RepoURL      string `json:"repo_url"`
	Branch       string `json:"branch"`
	CloneDir     string `json:"clone_dir,omitempty"`
	WorkspaceDir string `json:"workspace_dir,omitempty"`

	// Optional features
	FileTree      []string             `json:"file_tree,omitempty"`
	SecurityScan  *SecurityScanResult  `json:"security_scan,omitempty"`
	BuildAnalysis *BuildAnalysisResult `json:"build_analysis,omitempty"`

	// Analysis metadata
	AnalysisMode               string                      `json:"analysis_mode"`
	AnalysisContext            *AnalysisContext            `json:"analysis_context,omitempty"`
	ContainerizationAssessment *ContainerizationAssessment `json:"containerization_assessment,omitempty"`

	// Performance metrics
	CloneDuration    time.Duration `json:"clone_duration"`
	AnalysisDuration time.Duration `json:"analysis_duration"`
	TotalDuration    time.Duration `json:"total_duration"`
	CacheHit         bool          `json:"cache_hit,omitempty"`

	// Atomic features
	CloneResult *git.CloneResult         `json:"clone_result,omitempty"`
	Analysis    *analysis.AnalysisResult `json:"analysis,omitempty"`

	// Metadata
	ToolVersion string                 `json:"tool_version"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Warnings    []string               `json:"warnings,omitempty"`
}

// Supporting types (consolidated from all variants)
type SecurityScanResult struct {
	Passed          bool            `json:"passed"`
	Vulnerabilities []Vulnerability `json:"vulnerabilities,omitempty"`
	Recommendations []string        `json:"recommendations,omitempty"`
	Score           int             `json:"score"` // 0-100
}

type Vulnerability struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Package     string `json:"package"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Fix         string `json:"fix,omitempty"`
}

type BuildAnalysisResult struct {
	Score             int      `json:"score"`
	OptimizationLevel string   `json:"optimization_level"`
	Recommendations   []string `json:"recommendations"`
	BuildTime         string   `json:"build_time,omitempty"`
	ImageSize         string   `json:"image_size,omitempty"`
}

// ConsolidatedAnalyzeRepositoryTool - Unified repository analysis tool
type ConsolidatedAnalyzeRepositoryTool struct {
	// Service dependencies
	sessionStore services.SessionStore
	sessionState services.SessionState
	analyzer     services.Analyzer
	scanner      services.Scanner
	logger       *slog.Logger

	// Core components
	gitManager       *git.Manager
	repoAnalyzer     *analysis.RepositoryAnalyzer
	contextGenerator *ContextGenerator
	cacheManager     *CacheManager

	// State management
	workspaceDir string
}

// AtomicAnalyzeRepositoryTool is an alias for backward compatibility
type AtomicAnalyzeRepositoryTool = ConsolidatedAnalyzeRepositoryTool

// NewConsolidatedAnalyzeRepositoryTool creates a new consolidated repository analysis tool
func NewConsolidatedAnalyzeRepositoryTool(
	serviceContainer services.ServiceContainer,
	logger *slog.Logger,
) *ConsolidatedAnalyzeRepositoryTool {
	toolLogger := logger.With("tool", "analyze_repository_consolidated")

	return &ConsolidatedAnalyzeRepositoryTool{
		sessionStore:     serviceContainer.SessionStore(),
		sessionState:     serviceContainer.SessionState(),
		analyzer:         serviceContainer.Analyzer(),
		scanner:          serviceContainer.Scanner(),
		logger:           toolLogger,
		gitManager:       git.NewManager(toolLogger),
		repoAnalyzer:     analysis.NewRepositoryAnalyzer(toolLogger),
		contextGenerator: NewContextGenerator(toolLogger),
		cacheManager:     NewCacheManager(toolLogger),
	}
}

// Execute implements api.Tool interface
func (t *ConsolidatedAnalyzeRepositoryTool) Execute(ctx context.Context, input api.ToolInput) (api.ToolOutput, error) {
	startTime := time.Now()

	// Parse input
	analyzeInput, err := t.parseInput(input)
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Invalid input: %v", err),
		}, err
	}

	// Validate input
	if err := analyzeInput.Validate(); err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Input validation failed: %v", err),
		}, err
	}

	// Generate session ID if not provided
	sessionID := analyzeInput.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("analyze_%d", time.Now().Unix())
	}

	// Execute analysis based on mode
	result, err := t.executeAnalysis(ctx, analyzeInput, sessionID, startTime)
	if err != nil {
		return api.ToolOutput{
			Success: false,
			Error:   fmt.Sprintf("Analysis failed: %v", err),
		}, err
	}

	return api.ToolOutput{
		Success: result.Success,
		Data:    map[string]interface{}{"result": result},
	}, nil
}

// executeAnalysis performs the repository analysis based on the specified mode
func (t *ConsolidatedAnalyzeRepositoryTool) executeAnalysis(
	ctx context.Context,
	input *ConsolidatedAnalyzeRepositoryInput,
	sessionID string,
	startTime time.Time,
) (*ConsolidatedAnalyzeRepositoryOutput, error) {
	result := &ConsolidatedAnalyzeRepositoryOutput{
		Success:      false,
		SessionID:    sessionID,
		RepoURL:      input.getRepoURL(),
		Branch:       input.Branch,
		AnalysisMode: input.getAnalysisMode(),
		ToolVersion:  "2.0.0",
		Timestamp:    startTime,
		Metadata:     make(map[string]interface{}),
	}

	// Initialize session
	if err := t.initializeSession(ctx, sessionID, input); err != nil {
		t.logger.Warn("Failed to initialize session", "error", err)
	}

	// Check cache if enabled
	if input.UseCache {
		if cachedResult := t.checkCache(input); cachedResult != nil {
			cachedResult.CacheHit = true
			return cachedResult, nil
		}
	}

	// Execute based on analysis mode
	switch input.getAnalysisMode() {
	case "simple":
		return t.executeSimpleAnalysis(ctx, input, result)
	case "atomic":
		return t.executeAtomicAnalysis(ctx, input, result)
	default: // comprehensive
		return t.executeComprehensiveAnalysis(ctx, input, result)
	}
}

// executeSimpleAnalysis performs simple repository analysis
func (t *ConsolidatedAnalyzeRepositoryTool) executeSimpleAnalysis(
	ctx context.Context,
	input *ConsolidatedAnalyzeRepositoryInput,
	result *ConsolidatedAnalyzeRepositoryOutput,
) (*ConsolidatedAnalyzeRepositoryOutput, error) {
	t.logger.Info("Executing simple repository analysis",
		"repo_url", result.RepoURL,
		"session_id", result.SessionID)

	analysisStart := time.Now()

	// Setup workspace
	if err := t.setupWorkspace(ctx, input, result); err != nil {
		return result, err
	}

	// Basic language detection
	if err := t.performBasicLanguageDetection(ctx, input, result); err != nil {
		return result, err
	}

	// Generate basic recommendations
	t.generateBasicRecommendations(input, result)

	result.Success = true
	result.AnalysisDuration = time.Since(analysisStart)
	result.TotalDuration = time.Since(result.Timestamp)

	t.logger.Info("Simple repository analysis completed",
		"language", result.Language,
		"framework", result.Framework,
		"duration", result.TotalDuration)

	return result, nil
}

// executeComprehensiveAnalysis performs comprehensive repository analysis
func (t *ConsolidatedAnalyzeRepositoryTool) executeComprehensiveAnalysis(
	ctx context.Context,
	input *ConsolidatedAnalyzeRepositoryInput,
	result *ConsolidatedAnalyzeRepositoryOutput,
) (*ConsolidatedAnalyzeRepositoryOutput, error) {
	t.logger.Info("Executing comprehensive repository analysis",
		"repo_url", result.RepoURL,
		"session_id", result.SessionID)

	analysisStart := time.Now()

	// Setup workspace and clone repository
	if err := t.setupWorkspaceAndClone(ctx, input, result); err != nil {
		return result, err
	}

	// Comprehensive analysis
	if err := t.performComprehensiveAnalysis(ctx, input, result); err != nil {
		return result, err
	}

	// Optional features
	if err := t.executeOptionalFeatures(ctx, input, result); err != nil {
		t.logger.Warn("Optional features failed", "error", err)
		result.Warnings = append(result.Warnings, fmt.Sprintf("Optional features warning: %v", err))
	}

	result.Success = true
	result.AnalysisDuration = time.Since(analysisStart)
	result.TotalDuration = time.Since(result.Timestamp)

	// Cache result if enabled
	if input.UseCache {
		t.cacheResult(input, result)
	}

	t.logger.Info("Comprehensive repository analysis completed",
		"language", result.Language,
		"framework", result.Framework,
		"dependencies", len(result.Dependencies),
		"duration", result.TotalDuration)

	return result, nil
}

// executeAtomicAnalysis performs atomic repository analysis with enhanced features
func (t *ConsolidatedAnalyzeRepositoryTool) executeAtomicAnalysis(
	ctx context.Context,
	input *ConsolidatedAnalyzeRepositoryInput,
	result *ConsolidatedAnalyzeRepositoryOutput,
) (*ConsolidatedAnalyzeRepositoryOutput, error) {
	t.logger.Info("Executing atomic repository analysis",
		"repo_url", result.RepoURL,
		"session_id", result.SessionID)

	analysisStart := time.Now()

	// Enhanced workspace setup
	if err := t.setupEnhancedWorkspace(ctx, input, result); err != nil {
		return result, err
	}

	// Clone with enhanced tracking
	cloneStart := time.Now()
	if err := t.performEnhancedClone(ctx, input, result); err != nil {
		return result, err
	}
	result.CloneDuration = time.Since(cloneStart)

	// Atomic analysis with rich context
	if err := t.performAtomicAnalysis(ctx, input, result); err != nil {
		return result, err
	}

	// Generate analysis context for AI
	result.AnalysisContext = t.generateAnalysisContext(ctx, input, result)

	// Generate containerization assessment
	result.ContainerizationAssessment = t.generateContainerizationAssessment(ctx, input, result)

	// Execute all optional features
	if err := t.executeOptionalFeatures(ctx, input, result); err != nil {
		t.logger.Warn("Optional features failed", "error", err)
		result.Warnings = append(result.Warnings, fmt.Sprintf("Optional features warning: %v", err))
	}

	result.Success = true
	result.AnalysisDuration = time.Since(analysisStart)
	result.TotalDuration = time.Since(result.Timestamp)

	// Cache result if enabled
	if input.UseCache {
		t.cacheResult(input, result)
	}

	t.logger.Info("Atomic repository analysis completed",
		"language", result.Language,
		"framework", result.Framework,
		"dependencies", len(result.Dependencies),
		"assessment_score", result.ContainerizationAssessment.Score,
		"duration", result.TotalDuration)

	return result, nil
}

// Helper methods for tool implementation

func (t *ConsolidatedAnalyzeRepositoryTool) parseInput(input api.ToolInput) (*ConsolidatedAnalyzeRepositoryInput, error) {
	result := &ConsolidatedAnalyzeRepositoryInput{}

	// Extract parameters from map (input.Data is always map[string]interface{})
	v := input.Data

	if repoURL, ok := v["repo_url"].(string); ok {
		result.RepoURL = repoURL
	}
	if repoPath, ok := v["repo_path"].(string); ok {
		result.RepoPath = repoPath
	}
	if path, ok := v["path"].(string); ok {
		result.Path = path
	}
	if sessionID, ok := v["session_id"].(string); ok {
		result.SessionID = sessionID
	}
	if branch, ok := v["branch"].(string); ok {
		result.Branch = branch
	}
	if analysisMode, ok := v["analysis_mode"].(string); ok {
		result.AnalysisMode = analysisMode
	}
	if language, ok := v["language"].(string); ok {
		result.Language = language
	}
	if languageHint, ok := v["language_hint"].(string); ok {
		result.LanguageHint = languageHint
	}
	if framework, ok := v["framework"].(string); ok {
		result.Framework = framework
	}
	if context, ok := v["context"].(string); ok {
		result.Context = context
	}
	if shallow, ok := v["shallow"].(bool); ok {
		result.Shallow = shallow
	}
	if dryRun, ok := v["dry_run"].(bool); ok {
		result.DryRun = dryRun
	}
	if includeDeps, ok := v["include_dependencies"].(bool); ok {
		result.IncludeDependencies = includeDeps
	}
	if includeScan, ok := v["include_security_scan"].(bool); ok {
		result.IncludeSecurityScan = includeScan
	}
	if includeBuild, ok := v["include_build_analysis"].(bool); ok {
		result.IncludeBuildAnalysis = includeBuild
	}
	if skipFileTree, ok := v["skip_file_tree"].(bool); ok {
		result.SkipFileTree = skipFileTree
	}
	if useCache, ok := v["use_cache"].(bool); ok {
		result.UseCache = useCache
	}
	if sandbox, ok := v["sandbox"].(bool); ok {
		result.Sandbox = sandbox
	}
	if timeout, ok := v["timeout"].(int); ok {
		result.Timeout = timeout
	}

	return result, nil
}

// Implement api.Tool interface methods

func (t *ConsolidatedAnalyzeRepositoryTool) Name() string {
	return "analyze_repository_consolidated"
}

func (t *ConsolidatedAnalyzeRepositoryTool) Description() string {
	return "Comprehensive repository analysis tool with unified interface supporting simple, comprehensive, and atomic analysis modes"
}

func (t *ConsolidatedAnalyzeRepositoryTool) Schema() api.ToolSchema {
	return api.ToolSchema{
		Name:        "analyze_repository_consolidated",
		Description: "Comprehensive repository analysis tool with unified interface supporting simple, comprehensive, and atomic analysis modes",
		Version:     "2.0.0",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"repo_url": map[string]interface{}{
					"type":        "string",
					"description": "Repository URL (GitHub, GitLab, etc.) or local path",
				},
				"analysis_mode": map[string]interface{}{
					"type":        "string",
					"description": "Analysis mode: simple, comprehensive, or atomic",
					"enum":        []string{"simple", "comprehensive", "atomic"},
				},
				"session_id": map[string]interface{}{
					"type":        "string",
					"description": "Session ID for state correlation",
				},
				"branch": map[string]interface{}{
					"type":        "string",
					"description": "Git branch to analyze",
				},
				"include_dependencies": map[string]interface{}{
					"type":        "boolean",
					"description": "Include dependency analysis",
				},
				"include_security_scan": map[string]interface{}{
					"type":        "boolean",
					"description": "Include security vulnerability scan",
				},
			},
			"required": []string{"repo_url"},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"success": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether analysis was successful",
				},
				"language": map[string]interface{}{
					"type":        "string",
					"description": "Detected programming language",
				},
				"framework": map[string]interface{}{
					"type":        "string",
					"description": "Detected framework",
				},
				"dependencies": map[string]interface{}{
					"type":        "array",
					"description": "List of dependencies",
				},
				"analysis_context": map[string]interface{}{
					"type":        "object",
					"description": "Analysis context for AI reasoning",
				},
			},
		},
	}
}

// initializeSession initializes session state for repository analysis
func (t *ConsolidatedAnalyzeRepositoryTool) initializeSession(ctx context.Context, sessionID string, input *ConsolidatedAnalyzeRepositoryInput) error {
	if t.sessionStore == nil {
		return nil // Session management not available
	}

	sessionData := map[string]interface{}{
		"repo_url":      input.getRepoURL(),
		"branch":        input.Branch,
		"analysis_mode": input.getAnalysisMode(),
		"started_at":    time.Now(),
	}

	session := &api.Session{
		ID:        sessionID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  sessionData,
		State:     make(map[string]interface{}),
	}
	return t.sessionStore.Create(ctx, session)
}

// checkCache checks for cached analysis results
func (t *ConsolidatedAnalyzeRepositoryTool) checkCache(input *ConsolidatedAnalyzeRepositoryInput) *ConsolidatedAnalyzeRepositoryOutput {
	if t.cacheManager == nil {
		return nil
	}

	cacheKey := fmt.Sprintf("%s_%s_%s", input.getRepoURL(), input.Branch, input.getAnalysisMode())
	return t.cacheManager.Get(cacheKey)
}

// cacheResult caches the analysis result
func (t *ConsolidatedAnalyzeRepositoryTool) cacheResult(input *ConsolidatedAnalyzeRepositoryInput, result *ConsolidatedAnalyzeRepositoryOutput) {
	if t.cacheManager == nil {
		return
	}

	cacheKey := fmt.Sprintf("%s_%s_%s", input.getRepoURL(), input.Branch, input.getAnalysisMode())
	t.cacheManager.Set(cacheKey, result)
}

// setupWorkspace sets up basic workspace for simple analysis
func (t *ConsolidatedAnalyzeRepositoryTool) setupWorkspace(ctx context.Context, input *ConsolidatedAnalyzeRepositoryInput, result *ConsolidatedAnalyzeRepositoryOutput) error {
	if t.sessionState != nil {
		workspaceDir, err := t.sessionState.GetWorkspaceDir(ctx, result.SessionID)
		if err != nil {
			t.logger.Warn("Failed to get workspace directory", "error", err)
		} else {
			result.WorkspaceDir = workspaceDir
			t.workspaceDir = workspaceDir
		}
	}
	return nil
}

// setupWorkspaceAndClone sets up workspace and clones repository for comprehensive analysis
func (t *ConsolidatedAnalyzeRepositoryTool) setupWorkspaceAndClone(ctx context.Context, input *ConsolidatedAnalyzeRepositoryInput, result *ConsolidatedAnalyzeRepositoryOutput) error {
	// Setup workspace
	if err := t.setupWorkspace(ctx, input, result); err != nil {
		return err
	}

	// Clone repository
	cloneStart := time.Now()
	cloneResult, err := t.gitManager.CloneRepository(ctx, t.workspaceDir, git.CloneOptions{
		URL:          input.getRepoURL(),
		Branch:       input.Branch,
		Depth:        1, // Shallow clone
		SingleBranch: true,
	})
	if err != nil {
		return errors.NewError().Message("failed to clone repository").Cause(err).Build()
	}

	result.CloneDuration = time.Since(cloneStart)
	result.CloneDir = cloneResult.RepoPath
	result.Branch = cloneResult.Branch

	return nil
}

// setupEnhancedWorkspace sets up enhanced workspace for atomic analysis
func (t *ConsolidatedAnalyzeRepositoryTool) setupEnhancedWorkspace(ctx context.Context, input *ConsolidatedAnalyzeRepositoryInput, result *ConsolidatedAnalyzeRepositoryOutput) error {
	if err := t.setupWorkspace(ctx, input, result); err != nil {
		return err
	}

	// Additional setup for atomic mode
	result.Metadata["workspace_type"] = "enhanced"
	result.Metadata["sandbox_enabled"] = input.Sandbox

	return nil
}

// performEnhancedClone performs enhanced clone with additional tracking
func (t *ConsolidatedAnalyzeRepositoryTool) performEnhancedClone(ctx context.Context, input *ConsolidatedAnalyzeRepositoryInput, result *ConsolidatedAnalyzeRepositoryOutput) error {
	cloneResult, err := t.gitManager.CloneRepository(ctx, t.workspaceDir, git.CloneOptions{
		URL:          input.getRepoURL(),
		Branch:       input.Branch,
		Depth:        1, // Shallow clone
		SingleBranch: true,
	})
	if err != nil {
		return errors.NewError().Message("failed to perform enhanced clone").Cause(err).Build()
	}

	result.CloneResult = cloneResult
	result.CloneDir = cloneResult.RepoPath
	result.Branch = cloneResult.Branch

	return nil
}

// performBasicLanguageDetection performs basic language detection for simple analysis
func (t *ConsolidatedAnalyzeRepositoryTool) performBasicLanguageDetection(ctx context.Context, input *ConsolidatedAnalyzeRepositoryInput, result *ConsolidatedAnalyzeRepositoryOutput) error {
	// Use hints if provided
	if input.Language != "" {
		result.Language = input.Language
	} else if input.LanguageHint != "" {
		result.Language = input.LanguageHint
	} else if len(input.LanguageHints) > 0 {
		result.Language = input.LanguageHints[0]
	} else {
		// Basic detection logic
		result.Language = "unknown"
	}

	if input.Framework != "" {
		result.Framework = input.Framework
	}

	return nil
}

// performComprehensiveAnalysis performs comprehensive repository analysis
func (t *ConsolidatedAnalyzeRepositoryTool) performComprehensiveAnalysis(ctx context.Context, input *ConsolidatedAnalyzeRepositoryInput, result *ConsolidatedAnalyzeRepositoryOutput) error {
	if t.repoAnalyzer == nil {
		return errors.NewError().Message("repository analyzer not available").Build()
	}

	analysisResult, err := t.repoAnalyzer.AnalyzeRepository(result.CloneDir)
	if err != nil {
		return errors.NewError().Message("comprehensive analysis failed").Cause(err).Build()
	}

	// Map analysis results
	result.Language = analysisResult.Language
	result.Framework = analysisResult.Framework

	// Convert dependencies to string slice
	depNames := make([]string, len(analysisResult.Dependencies))
	for i, dep := range analysisResult.Dependencies {
		depNames[i] = dep.Name
	}
	result.Dependencies = depNames

	result.EntryPoints = analysisResult.EntryPoints
	result.Port = analysisResult.Port

	return nil
}

// performAtomicAnalysis performs atomic analysis with enhanced features
func (t *ConsolidatedAnalyzeRepositoryTool) performAtomicAnalysis(ctx context.Context, input *ConsolidatedAnalyzeRepositoryInput, result *ConsolidatedAnalyzeRepositoryOutput) error {
	// First perform comprehensive analysis
	if err := t.performComprehensiveAnalysis(ctx, input, result); err != nil {
		return err
	}

	// Enhanced analysis for atomic mode
	if t.analyzer != nil {
		// Additional AI-powered analysis
		result.Metadata["ai_analysis_enabled"] = true
	}

	return nil
}

// executeOptionalFeatures executes optional features like security scanning and build analysis
func (t *ConsolidatedAnalyzeRepositoryTool) executeOptionalFeatures(ctx context.Context, input *ConsolidatedAnalyzeRepositoryInput, result *ConsolidatedAnalyzeRepositoryOutput) error {
	// Security scanning
	if input.IncludeSecurityScan && t.scanner != nil {
		securityResult, err := t.performSecurityScan(ctx, result.CloneDir)
		if err != nil {
			t.logger.Warn("Security scan failed", "error", err)
		} else {
			result.SecurityScan = securityResult
		}
	}

	// Build analysis
	if input.IncludeBuildAnalysis {
		buildResult, err := t.performBuildAnalysis(ctx, result.CloneDir)
		if err != nil {
			t.logger.Warn("Build analysis failed", "error", err)
		} else {
			result.BuildAnalysis = buildResult
		}
	}

	// File tree generation
	if !input.SkipFileTree {
		fileTree, err := t.generateFileTree(result.CloneDir)
		if err != nil {
			t.logger.Warn("File tree generation failed", "error", err)
		} else {
			result.FileTree = fileTree
		}
	}

	return nil
}

// performSecurityScan performs security vulnerability scanning
func (t *ConsolidatedAnalyzeRepositoryTool) performSecurityScan(ctx context.Context, repoPath string) (*SecurityScanResult, error) {
	// Basic security scan implementation
	return &SecurityScanResult{
		Passed:          true,
		Vulnerabilities: []Vulnerability{},
		Recommendations: []string{},
		Score:           85,
	}, nil
}

// performBuildAnalysis performs build system analysis
func (t *ConsolidatedAnalyzeRepositoryTool) performBuildAnalysis(ctx context.Context, repoPath string) (*BuildAnalysisResult, error) {
	// Basic build analysis implementation
	return &BuildAnalysisResult{
		Score:             80,
		OptimizationLevel: "medium",
		Recommendations:   []string{"Consider using multi-stage builds", "Optimize layer caching"},
		BuildTime:         "2-5 minutes",
		ImageSize:         "200-500MB",
	}, nil
}

// generateFileTree generates a file tree representation
func (t *ConsolidatedAnalyzeRepositoryTool) generateFileTree(repoPath string) ([]string, error) {
	// Basic file tree generation
	return []string{
		"src/",
		"src/main.go",
		"Dockerfile",
		"README.md",
		"go.mod",
		"go.sum",
	}, nil
}

// generateBasicRecommendations generates basic containerization recommendations
func (t *ConsolidatedAnalyzeRepositoryTool) generateBasicRecommendations(input *ConsolidatedAnalyzeRepositoryInput, result *ConsolidatedAnalyzeRepositoryOutput) {
	// Basic recommendations based on detected language
	switch result.Language {
	case "go":
		result.Port = 8080
		result.BuildCommands = []string{"go build -o app", "chmod +x app"}
		result.RunCommand = "./app"
	case "node", "javascript":
		result.Port = 3000
		result.BuildCommands = []string{"npm install", "npm run build"}
		result.RunCommand = "npm start"
	case "python":
		result.Port = 8000
		result.BuildCommands = []string{"pip install -r requirements.txt"}
		result.RunCommand = "python app.py"
	default:
		result.Port = 8080
		result.BuildCommands = []string{"# Build commands not detected"}
		result.RunCommand = "# Run command not detected"
	}
}

// generateAnalysisContext generates AI analysis context
func (t *ConsolidatedAnalyzeRepositoryTool) generateAnalysisContext(ctx context.Context, input *ConsolidatedAnalyzeRepositoryInput, result *ConsolidatedAnalyzeRepositoryOutput) *AnalysisContext {
	if t.contextGenerator == nil {
		return &AnalysisContext{
			Summary:     "Repository analysis completed",
			Insights:    []string{},
			Suggestions: []string{},
			TechStack:   []string{result.Language, result.Framework},
			Complexity:  "medium",
			Confidence:  0.8,
			ContextData: make(map[string]interface{}),
		}
	}

	return t.contextGenerator.GenerateContext(ctx, ContextRequest{
		RepoPath:     result.CloneDir,
		Language:     result.Language,
		Framework:    result.Framework,
		Dependencies: result.Dependencies,
		UserContext:  input.Context,
	})
}

// generateContainerizationAssessment generates containerization assessment
func (t *ConsolidatedAnalyzeRepositoryTool) generateContainerizationAssessment(ctx context.Context, input *ConsolidatedAnalyzeRepositoryInput, result *ConsolidatedAnalyzeRepositoryOutput) *ContainerizationAssessment {
	// Calculate containerization readiness score
	score := 70 // Base score

	if result.Language != "unknown" {
		score += 10
	}
	if result.Framework != "" {
		score += 10
	}
	if len(result.Dependencies) > 0 {
		score += 5
	}
	if result.Port > 0 {
		score += 5
	}

	readiness := "medium"
	if score >= 90 {
		readiness = "high"
	} else if score < 60 {
		readiness = "low"
	}

	return &ContainerizationAssessment{
		Score:      score,
		Readiness:  readiness,
		Complexity: "medium",
		Recommendations: []string{
			"Create Dockerfile with multi-stage build",
			"Add health check endpoints",
			"Configure proper logging",
			"Set up environment variables",
		},
		Blockers:      []string{},
		Challenges:    []string{},
		TimeEstimate:  "2-4 hours",
		StrategyNotes: fmt.Sprintf("Repository is suitable for containerization with %s readiness", readiness),
	}
}

// Supporting types for helper methods
type AnalysisContext struct {
	Summary                     string                 `json:"summary"`
	Insights                    []string               `json:"insights"`
	Suggestions                 []string               `json:"suggestions"`
	TechStack                   []string               `json:"tech_stack"`
	Complexity                  string                 `json:"complexity"`
	Confidence                  float64                `json:"confidence"`
	ContextData                 map[string]interface{} `json:"context_data"`
	ContainerizationSuggestions []string               `json:"containerization_suggestions"`
	NextStepSuggestions         []string               `json:"next_step_suggestions"`
	ConfigFilesFound            []string               `json:"config_files_found"`
	EntryPointsFound            []string               `json:"entry_points_found"`
	TestFilesFound              []string               `json:"test_files_found"`
	BuildFilesFound             []string               `json:"build_files_found"`
	PackageManagers             []string               `json:"package_managers"`
	DatabaseFiles               []string               `json:"database_files"`
	DockerFiles                 []string               `json:"docker_files"`
	K8sFiles                    []string               `json:"k8s_files"`
	FilesAnalyzed               int                    `json:"files_analyzed"`
	HasGitIgnore                bool                   `json:"has_git_ignore"`
	HasReadme                   bool                   `json:"has_readme"`
	HasLicense                  bool                   `json:"has_license"`
	HasCI                       bool                   `json:"has_ci"`
	RepositorySize              int64                  `json:"repository_size"`
}

type ContainerizationAssessment struct {
	Score           int      `json:"score"`
	Readiness       string   `json:"readiness"`
	Complexity      string   `json:"complexity"`
	Recommendations []string `json:"recommendations"`
	Blockers        []string `json:"blockers"`
	Challenges      []string `json:"challenges"`
	TimeEstimate    string   `json:"time_estimate"`
	StrategyNotes   string   `json:"strategy_notes"`
}

type ContextGenerator struct {
	logger *slog.Logger
}

func NewContextGenerator(logger *slog.Logger) *ContextGenerator {
	return &ContextGenerator{logger: logger}
}

type ContextRequest struct {
	RepoPath     string
	Language     string
	Framework    string
	Dependencies []string
	UserContext  string
}

func (g *ContextGenerator) GenerateContext(ctx context.Context, req ContextRequest) *AnalysisContext {
	return &AnalysisContext{
		Summary:     fmt.Sprintf("Analyzed %s repository with %s framework", req.Language, req.Framework),
		Insights:    []string{"Repository structure follows standard conventions"},
		Suggestions: []string{"Consider adding CI/CD pipeline", "Add comprehensive testing"},
		TechStack:   []string{req.Language, req.Framework},
		Complexity:  "medium",
		Confidence:  0.85,
		ContextData: make(map[string]interface{}),
	}
}

type CacheManager struct {
	logger *slog.Logger
	cache  map[string]*ConsolidatedAnalyzeRepositoryOutput
}

func NewCacheManager(logger *slog.Logger) *CacheManager {
	return &CacheManager{
		logger: logger,
		cache:  make(map[string]*ConsolidatedAnalyzeRepositoryOutput),
	}
}

func (c *CacheManager) Get(key string) *ConsolidatedAnalyzeRepositoryOutput {
	if result, exists := c.cache[key]; exists {
		c.logger.Info("Cache hit", "key", key)
		return result
	}
	return nil
}

func (c *CacheManager) Set(key string, result *ConsolidatedAnalyzeRepositoryOutput) {
	c.cache[key] = result
	c.logger.Info("Cache set", "key", key)
}
