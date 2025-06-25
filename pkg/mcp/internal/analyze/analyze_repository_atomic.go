package analyze

import (
	"github.com/Azure/container-copilot/pkg/mcp/internal"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/core/analysis"
	"github.com/Azure/container-copilot/pkg/core/git"
	"github.com/Azure/container-copilot/pkg/mcp/internal/api/contract"
	"github.com/Azure/container-copilot/pkg/mcp/internal/mcperror"
	"github.com/Azure/container-copilot/pkg/mcp/internal/repository"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/session"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

// AtomicAnalyzeRepositoryArgs defines arguments for atomic repository analysis
type AtomicAnalyzeRepositoryArgs struct {
	types.BaseToolArgs
	RepoURL      string `json:"repo_url" description:"Repository URL (GitHub, GitLab, etc.) or local path"`
	Branch       string `json:"branch,omitempty" description:"Git branch to analyze (default: main)"`
	Context      string `json:"context,omitempty" description:"Additional context about the application"`
	LanguageHint string `json:"language_hint,omitempty" description:"Primary programming language hint"`
	Shallow      bool   `json:"shallow,omitempty" description:"Perform shallow clone for faster analysis"`
}

// AtomicAnalysisResult defines the response from atomic repository analysis
type AtomicAnalysisResult struct {
	types.BaseToolResponse
	internal.BaseAIContextResult      // Embed AI context methods
	Success             bool `json:"success"`

	// Session context
	SessionID    string `json:"session_id"`
	WorkspaceDir string `json:"workspace_dir"`

	// Repository info
	RepoURL  string `json:"repo_url"`
	Branch   string `json:"branch"`
	CloneDir string `json:"clone_dir"`

	// Analysis results from core operations
	Analysis *analysis.AnalysisResult `json:"analysis"`

	// Clone results for debugging
	CloneResult *git.CloneResult `json:"clone_result,omitempty"`

	// Timing information
	CloneDuration    time.Duration `json:"clone_duration"`
	AnalysisDuration time.Duration `json:"analysis_duration"`
	TotalDuration    time.Duration `json:"total_duration"`

	// Rich context for Claude reasoning
	AnalysisContext *repository.AnalysisContext `json:"analysis_context"`

	// AI context for decision-making
	ContainerizationAssessment *ContainerizationAssessment `json:"containerization_assessment"`
}

// Type aliases for backward compatibility
type ContainerizationAssessment = repository.ContainerizationAssessment

// Type aliases for backward compatibility
type TechnologyStackAssessment = repository.TechnologyStackAssessment
type ContainerizationRisk = repository.ContainerizationRisk
type DeploymentRecommendation = repository.DeploymentRecommendation

// Uses interfaces from interfaces.go to avoid import cycles

// AtomicAnalyzeRepositoryTool implements atomic repository analysis using core operations
type AtomicAnalyzeRepositoryTool struct {
	pipelineAdapter mcptypes.PipelineOperations
	sessionManager  mcptypes.ToolSessionManager
	// errorHandler field removed - using direct error handling
	logger           zerolog.Logger
	repoCloner       *repository.Cloner
	repoAnalyzer     *repository.Analyzer
	contextGenerator *repository.ContextGenerator
}

// NewAtomicAnalyzeRepositoryTool creates a new atomic analyze repository tool
func NewAtomicAnalyzeRepositoryTool(adapter mcptypes.PipelineOperations, sessionManager mcptypes.ToolSessionManager, logger zerolog.Logger) *AtomicAnalyzeRepositoryTool {
	return &AtomicAnalyzeRepositoryTool{
		pipelineAdapter: adapter,
		sessionManager:  sessionManager,
		// errorHandler initialization removed - using direct error handling
		logger:           logger.With().Str("tool", "atomic_analyze_repository").Logger(),
		repoCloner:       repository.NewCloner(logger),
		repoAnalyzer:     repository.NewAnalyzer(logger),
		contextGenerator: repository.NewContextGenerator(logger),
	}
}

// Note: Using centralized stage definitions from core.StandardAnalysisStages()

// ExecuteRepositoryAnalysis runs the atomic repository analysis (legacy method)
func (t *AtomicAnalyzeRepositoryTool) ExecuteRepositoryAnalysis(ctx context.Context, args AtomicAnalyzeRepositoryArgs) (*AtomicAnalysisResult, error) {
	// Direct execution without progress tracking
	return t.executeWithoutProgress(ctx, args)
}

// ExecuteWithContext runs the atomic repository analysis with GoMCP progress tracking
func (t *AtomicAnalyzeRepositoryTool) ExecuteWithContext(serverCtx *server.Context, args AtomicAnalyzeRepositoryArgs) (*AtomicAnalysisResult, error) {
	// Create progress adapter for GoMCP using standard analysis stages
	_ = internal.NewGoMCPProgressAdapter(serverCtx, []mcptypes.ProgressStage{{Name: "Initialize", Weight: 0.10, Description: "Loading session"}, {Name: "Analyze", Weight: 0.80, Description: "Analyzing"}, {Name: "Finalize", Weight: 0.10, Description: "Updating state"}})

	// Execute with progress tracking
	ctx := context.Background()
	result, err := t.performAnalysis(ctx, args, nil)

	// Complete progress tracking
	if err != nil {
		t.logger.Info().Msg("Analysis failed")
		if result != nil {
			result.Success = false
		}
		return result, nil // Return result with error info, not the error itself
	} else {
		t.logger.Info().Msg("Analysis completed successfully")
	}

	return result, nil
}

// executeWithoutProgress executes without progress tracking
func (t *AtomicAnalyzeRepositoryTool) executeWithoutProgress(ctx context.Context, args AtomicAnalyzeRepositoryArgs) (*AtomicAnalysisResult, error) {
	return t.performAnalysis(ctx, args, nil)
}

// performAnalysis performs the actual repository analysis
func (t *AtomicAnalyzeRepositoryTool) performAnalysis(ctx context.Context, args AtomicAnalyzeRepositoryArgs, reporter mcptypes.ProgressReporter) (*AtomicAnalysisResult, error) {
	startTime := time.Now()

	// Get or create session
	session, err := t.getOrCreateSession(args.SessionID)
	if err != nil {
		// Create result with error for session failure
		result := &AtomicAnalysisResult{
			BaseToolResponse:           types.NewBaseResponse("atomic_analyze_repository", args.SessionID, args.DryRun),
			BaseAIContextResult:        internal.NewBaseAIContextResult("analysis", false, time.Since(startTime)),
			SessionID:                  args.SessionID,
			RepoURL:                    args.RepoURL,
			Branch:                     args.Branch,
			TotalDuration:              time.Since(startTime),
			AnalysisContext:            &repository.AnalysisContext{},
			ContainerizationAssessment: &ContainerizationAssessment{},
		}
		result.Success = false

		t.logger.Error().Err(err).Str("session_id", args.SessionID).Msg("Failed to get/create session")
		result.Success = false
		return result, mcperror.NewSessionNotFound(args.SessionID)
	}

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("repo_url", args.RepoURL).
		Str("branch", args.Branch).
		Msg("Starting atomic repository analysis")

	// Stage 1: Initialize
	// Progress reporting removed

	// Create base response
	result := &AtomicAnalysisResult{
		BaseToolResponse:           types.NewBaseResponse("atomic_analyze_repository", session.SessionID, args.DryRun),
		BaseAIContextResult:        internal.NewBaseAIContextResult("analysis", false, 0), // Duration and success will be updated later
		SessionID:                  session.SessionID,
		WorkspaceDir:               t.pipelineAdapter.GetSessionWorkspace(session.SessionID),
		RepoURL:                    args.RepoURL,
		Branch:                     args.Branch,
		AnalysisContext:            &repository.AnalysisContext{},
		ContainerizationAssessment: &ContainerizationAssessment{},
	}

	// Check if this is a resumed session
	if session.Metadata != nil {
		if resumedFrom, ok := session.Metadata["resumed_from"].(map[string]interface{}); ok {
			oldSessionID, _ := resumedFrom["old_session_id"].(string) //nolint:errcheck // Only for logging
			lastRepoURL, _ := resumedFrom["last_repo_url"].(string)   //nolint:errcheck // Only for logging

			t.logger.Info().
				Str("old_session_id", oldSessionID).
				Str("new_session_id", session.SessionID).
				Str("last_repo_url", lastRepoURL).
				Msg("Session was resumed from expired session")

			// Add context about the resume
			result.AnalysisContext.NextStepSuggestions = append(result.AnalysisContext.NextStepSuggestions,
				fmt.Sprintf("Note: Your previous session (%s) expired. A new session has been created.", oldSessionID),
				"You'll need to regenerate your Dockerfile and rebuild your image with the new session.",
			)

			// If no repo URL provided but we have the last one, suggest it
			if args.RepoURL == "" && lastRepoURL != "" {
				result.AnalysisContext.NextStepSuggestions = append(result.AnalysisContext.NextStepSuggestions,
					fmt.Sprintf("Tip: Your last repository was: %s", lastRepoURL),
				)
			}
		}
	}

	// Progress reporting removed

	// Handle dry-run
	if args.DryRun {
		result.AnalysisContext.NextStepSuggestions = []string{
			"This is a dry-run - actual repository cloning and analysis would be performed",
			"Session workspace would be created at: " + result.WorkspaceDir,
		}
		result.TotalDuration = time.Since(startTime)
		return result, nil
	}

	// Stage 2: Clone repository if it's a URL
	// Progress reporting removed

	if t.isURL(args.RepoURL) {
		// Progress reporting removed

		cloneResult, err := t.cloneRepository(ctx, session.SessionID, args)
		result.CloneResult = cloneResult
		if cloneResult != nil {
			result.CloneDuration = cloneResult.Duration
		}

		if err != nil {
			t.logger.Error().Err(err).
				Str("repo_url", args.RepoURL).
				Str("session_id", session.SessionID).
				Msg("Repository clone failed")
			result.Success = false
			result.TotalDuration = time.Since(startTime)
			return result, mcperror.NewWithData(mcperror.CodeAnalysisRequired, "Failed to clone repository", map[string]interface{}{
				"repo_url":   args.RepoURL,
				"branch":     args.Branch,
				"session_id": session.SessionID,
			})
		}

		result.CloneDir = cloneResult.RepoPath
		t.logger.Info().
			Str("session_id", session.SessionID).
			Str("clone_dir", result.CloneDir).
			Dur("clone_duration", result.CloneDuration).
			Msg("Repository cloned successfully")

		// Progress reporting removed
	} else {
		// Local path - validate and use directly
		if err := t.validateLocalPath(args.RepoURL); err != nil {
			t.logger.Error().Err(err).
				Str("local_path", args.RepoURL).
				Str("session_id", session.SessionID).
				Msg("Invalid local path for repository")
			// Local path validation error is returned directly
			result.Success = false
			result.TotalDuration = time.Since(startTime)
			return result, nil
		}
		result.CloneDir = args.RepoURL

		// Progress reporting removed
	}

	// Stage 3: Analyze repository
	// Progress reporting removed

	// Check for cached analysis results
	if session.ScanSummary != nil && session.ScanSummary.RepoPath == result.CloneDir {
		// Check if cache is still valid (less than 1 hour old)
		if time.Since(session.ScanSummary.CachedAt) < time.Hour {
			// Progress reporting removed
			t.logger.Info().
				Str("session_id", session.SessionID).
				Str("repo_path", result.CloneDir).
				Time("cached_at", session.ScanSummary.CachedAt).
				Msg("Using cached repository analysis results")

			// Build analysis result from cache
			result.Analysis = &analysis.AnalysisResult{
				Language:     session.ScanSummary.Language,
				Framework:    session.ScanSummary.Framework,
				Port:         session.ScanSummary.Port,
				Dependencies: make([]analysis.Dependency, len(session.ScanSummary.Dependencies)),
			}

			// Convert dependencies back
			for i, dep := range session.ScanSummary.Dependencies {
				result.Analysis.Dependencies[i] = analysis.Dependency{Name: dep}
			}

			// Populate analysis context from cache
			result.AnalysisContext = &repository.AnalysisContext{
				FilesAnalyzed:               session.ScanSummary.FilesAnalyzed,
				ConfigFilesFound:            session.ScanSummary.ConfigFilesFound,
				EntryPointsFound:            session.ScanSummary.EntryPointsFound,
				TestFilesFound:              session.ScanSummary.TestFilesFound,
				BuildFilesFound:             session.ScanSummary.BuildFilesFound,
				PackageManagers:             session.ScanSummary.PackageManagers,
				DatabaseFiles:               session.ScanSummary.DatabaseFiles,
				DockerFiles:                 session.ScanSummary.DockerFiles,
				K8sFiles:                    session.ScanSummary.K8sFiles,
				HasGitIgnore:                session.ScanSummary.HasGitIgnore,
				HasReadme:                   session.ScanSummary.HasReadme,
				HasLicense:                  session.ScanSummary.HasLicense,
				HasCI:                       session.ScanSummary.HasCI,
				RepositorySize:              session.ScanSummary.RepositorySize,
				ContainerizationSuggestions: session.ScanSummary.ContainerizationSuggestions,
				NextStepSuggestions:         session.ScanSummary.NextStepSuggestions,
			}

			result.AnalysisDuration = time.Duration(session.ScanSummary.AnalysisDuration * float64(time.Second))
			result.TotalDuration = time.Since(startTime)
			result.Success = true
			result.BaseAIContextResult.IsSuccessful = true
			result.BaseAIContextResult.Duration = result.TotalDuration

			t.logger.Info().
				Str("session_id", session.SessionID).
				Str("language", result.Analysis.Language).
				Str("framework", result.Analysis.Framework).
				Dur("cached_analysis_duration", result.AnalysisDuration).
				Dur("total_duration", result.TotalDuration).
				Msg("Repository analysis completed using cached results")

				// Progress reporting removed

			return result, nil
		} else {
			t.logger.Info().
				Str("session_id", session.SessionID).
				Time("cached_at", session.ScanSummary.CachedAt).
				Dur("cache_age", time.Since(session.ScanSummary.CachedAt)).
				Msg("Cached analysis results are stale, performing fresh analysis")
		}
	}

	// Perform mechanical analysis using repository module
	// Progress reporting removed

	analysisStartTime := time.Now()
	analysisOpts := repository.AnalysisOptions{
		RepoPath:     result.CloneDir,
		Context:      args.Context,
		LanguageHint: args.LanguageHint,
		SessionID:    session.SessionID,
	}

	repoAnalysisResult, err := t.repoAnalyzer.Analyze(ctx, analysisOpts)
	result.AnalysisDuration = time.Since(analysisStartTime)

	if err != nil {
		t.logger.Error().Err(err).
			Str("clone_dir", result.CloneDir).
			Str("session_id", session.SessionID).
			Bool("is_local", !t.isURL(args.RepoURL)).
			Msg("Repository analysis failed")
		result.Success = false
		result.TotalDuration = time.Since(startTime)
		return result, mcperror.NewWithData(mcperror.CodeAnalysisRequired, "Failed to analyze repository", map[string]interface{}{
			"repo_url":   args.RepoURL,
			"clone_dir":  result.CloneDir,
			"session_id": session.SessionID,
			"is_local":   !t.isURL(args.RepoURL),
		})
	}

	result.Analysis = repoAnalysisResult.AnalysisResult
	result.AnalysisContext = repoAnalysisResult.Context

	// Progress reporting removed

	// Stage 4: Generate analysis context
	// Progress reporting removed

	// Analysis context already generated by repository module
	// Progress reporting removed

	// Generate containerization assessment for AI decision-making
	assessment, err := t.contextGenerator.GenerateContainerizationAssessment(result.Analysis, result.AnalysisContext)
	if err != nil {
		t.logger.Warn().Err(err).Msg("Failed to generate containerization assessment")
	} else {
		result.ContainerizationAssessment = assessment
	}

	// Progress reporting removed

	// Stage 5: Finalize and save results
	// Progress reporting removed

	// Update session state
	if err := t.updateSessionState(session, result); err != nil {
		t.logger.Warn().Err(err).Msg("Failed to update session state")
	}

	// Progress reporting removed

	// Mark the operation as successful
	result.Success = true
	result.TotalDuration = time.Since(startTime)

	// Update internal.BaseAIContextResult fields
	result.BaseAIContextResult.IsSuccessful = true
	result.BaseAIContextResult.Duration = result.TotalDuration

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("language", result.Analysis.Language).
		Str("framework", result.Analysis.Framework).
		Int("files_analyzed", result.AnalysisContext.FilesAnalyzed).
		Dur("total_duration", result.TotalDuration).
		Msg("Atomic repository analysis completed successfully")

	// Progress reporting removed

	return result, nil
}

// getOrCreateSession gets existing session or creates a new one
func (t *AtomicAnalyzeRepositoryTool) getOrCreateSession(sessionID string) (*sessiontypes.SessionState, error) {
	if sessionID != "" {
		// Try to get existing session
		sessionInterface, err := t.sessionManager.GetSession(sessionID)
		if err == nil {
			session := sessionInterface.(*sessiontypes.SessionState)
			// Check if session is expired
			if time.Now().After(session.ExpiresAt) {
				t.logger.Info().
					Str("session_id", sessionID).
					Time("expired_at", session.ExpiresAt).
					Msg("Session has expired, will create new session and attempt to resume")
				// Store old session info for potential resume
				oldSessionInfo := map[string]interface{}{
					"old_session_id": sessionID,
					"expired_at":     session.ExpiresAt,
					"had_analysis":   session.ScanSummary != nil && session.ScanSummary.FilesAnalyzed > 0,
				}
				if session.ScanSummary != nil && session.ScanSummary.RepoURL != "" {
					oldSessionInfo["last_repo_url"] = session.ScanSummary.RepoURL
				}
				// Create new session with metadata about the old one
				newSessionInterface, err := t.sessionManager.GetOrCreateSession("")
				if err != nil {
					return nil, mcperror.NewSessionNotFound("replacement_session")
				}
				newSession := newSessionInterface.(*sessiontypes.SessionState)
				if newSession.Metadata == nil {
					newSession.Metadata = make(map[string]interface{})
				}
				newSession.Metadata["resumed_from"] = oldSessionInfo
				if err := t.sessionManager.UpdateSession(newSession.SessionID, func(s *sessiontypes.SessionState) { *s = *newSession }); err != nil {
					t.logger.Warn().Err(err).Msg("Failed to save resumed session")
				}

				t.logger.Info().
					Str("old_session_id", sessionID).
					Str("new_session_id", newSession.SessionID).
					Msg("Created new session to replace expired one")
				return newSession, nil
			}
			return session, nil
		}
		t.logger.Debug().Str("session_id", sessionID).Msg("Session not found, creating new one")
	}

	// Create new session
	sessionInterface, err := t.sessionManager.GetOrCreateSession("")
	if err != nil {
		return nil, mcperror.NewSessionNotFound("new_session")
	}
	session := sessionInterface.(*sessiontypes.SessionState)

	t.logger.Info().Str("session_id", session.SessionID).Msg("Created new session for repository analysis")
	return session, nil
}

// cloneRepository clones the repository using the repository module
func (t *AtomicAnalyzeRepositoryTool) cloneRepository(ctx context.Context, sessionID string, args AtomicAnalyzeRepositoryArgs) (*git.CloneResult, error) {
	// Get session to find workspace directory
	sessionInterface, err := t.sessionManager.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	session := sessionInterface.(*sessiontypes.SessionState)

	// Prepare clone options
	cloneOpts := repository.CloneOptions{
		RepoURL:   args.RepoURL,
		Branch:    args.Branch,
		Shallow:   args.Shallow,
		TargetDir: filepath.Join(session.WorkspaceDir, "repo"),
		SessionID: sessionID,
	}

	// Clone using the repository module
	result, err := t.repoCloner.Clone(ctx, cloneOpts)
	if err != nil {
		return nil, err
	}

	// Update session with clone info
	session.RepoPath = result.RepoPath
	session.RepoURL = args.RepoURL
	t.sessionManager.UpdateSession(sessionID, func(s *sessiontypes.SessionState) {
		s.RepoPath = result.RepoPath
		s.RepoURL = args.RepoURL
	})

	return result.CloneResult, nil
}

// updateSessionState updates the session with analysis results
func (t *AtomicAnalyzeRepositoryTool) updateSessionState(session *sessiontypes.SessionState, result *AtomicAnalysisResult) error {
	// Update session with repository analysis results
	analysis := result.Analysis
	// Convert dependencies to string slice
	dependencyNames := make([]string, len(analysis.Dependencies))
	for i, dep := range analysis.Dependencies {
		dependencyNames[i] = dep.Name
	}

	// Add to StageHistory for stage tracking
	now := time.Now()
	startTime := now.Add(-result.AnalysisDuration) // Calculate start time from duration
	execution := sessiontypes.ToolExecution{
		Tool:       "analyze_repository",
		StartTime:  startTime,
		EndTime:    &now,
		Duration:   &result.AnalysisDuration,
		Success:    true,
		DryRun:     false,
		TokensUsed: 0, // Could be tracked if needed
	}
	session.AddToolExecution(execution)

	session.UpdateLastAccessed()

	// Store structured scan summary for caching
	session.ScanSummary = &types.RepositoryScanSummary{
		// Core analysis results
		Language:     analysis.Language,
		Framework:    analysis.Framework,
		Port:         analysis.Port,
		Dependencies: dependencyNames,

		// File structure insights
		FilesAnalyzed:    result.AnalysisContext.FilesAnalyzed,
		ConfigFilesFound: result.AnalysisContext.ConfigFilesFound,
		EntryPointsFound: result.AnalysisContext.EntryPointsFound,
		TestFilesFound:   result.AnalysisContext.TestFilesFound,
		BuildFilesFound:  result.AnalysisContext.BuildFilesFound,

		// Ecosystem insights
		PackageManagers: result.AnalysisContext.PackageManagers,
		DatabaseFiles:   result.AnalysisContext.DatabaseFiles,
		DockerFiles:     result.AnalysisContext.DockerFiles,
		K8sFiles:        result.AnalysisContext.K8sFiles,

		// Repository metadata
		HasGitIgnore:   result.AnalysisContext.HasGitIgnore,
		HasReadme:      result.AnalysisContext.HasReadme,
		HasLicense:     result.AnalysisContext.HasLicense,
		HasCI:          result.AnalysisContext.HasCI,
		RepositorySize: result.AnalysisContext.RepositorySize,

		// Cache metadata
		CachedAt:         time.Now(),
		AnalysisDuration: result.AnalysisDuration.Seconds(),
		RepoPath:         result.CloneDir,
		RepoURL:          result.RepoURL,

		// Suggestions for reuse
		ContainerizationSuggestions: result.AnalysisContext.ContainerizationSuggestions,
		NextStepSuggestions:         result.AnalysisContext.NextStepSuggestions,
	}

	// Store additional context
	if session.Metadata == nil {
		session.Metadata = make(map[string]interface{})
	}
	session.Metadata["repo_url"] = result.RepoURL
	session.Metadata["clone_dir"] = result.CloneDir
	session.Metadata["files_analyzed"] = result.AnalysisContext.FilesAnalyzed
	session.Metadata["config_files"] = result.AnalysisContext.ConfigFilesFound
	session.Metadata["has_dockerfile"] = len(result.AnalysisContext.DockerFiles) > 0
	session.Metadata["has_k8s_files"] = len(result.AnalysisContext.K8sFiles) > 0
	session.Metadata["analysis_duration"] = result.AnalysisDuration.Seconds()

	return t.sessionManager.UpdateSession(session.SessionID, func(s *sessiontypes.SessionState) { *s = *session })
}

// Helper methods

func (t *AtomicAnalyzeRepositoryTool) isURL(path string) bool {
	return strings.HasPrefix(path, "http://") ||
		strings.HasPrefix(path, "https://") ||
		strings.HasPrefix(path, "git@") ||
		strings.HasPrefix(path, "ssh://")
}

func (t *AtomicAnalyzeRepositoryTool) validateLocalPath(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return mcperror.NewWithData(mcperror.CodeInvalidPath, "Failed to resolve absolute path", map[string]interface{}{
			"path": path,
		})
	}

	// Basic path validation (more could be added)
	if strings.Contains(absPath, "..") {
		return mcperror.NewWithData(mcperror.CodePermissionDenied, "Path traversal not allowed", map[string]interface{}{
			"path":          path,
			"resolved_path": absPath,
		})
	}

	return nil
}

// Unified AI Context Interface Implementations

// AI Context methods are now provided by embedded internal.BaseAIContextResult

// Tool interface implementation (unified interface)

// GetMetadata returns comprehensive tool metadata
func (t *AtomicAnalyzeRepositoryTool) GetMetadata() mcptypes.ToolMetadata {
	return mcptypes.ToolMetadata{
		Name:         "atomic_analyze_repository",
		Description:  "Analyzes repository structure, detects programming language, framework, and generates containerization recommendations",
		Version:      "1.0.0",
		Category:     "analysis",
		Dependencies: []string{"git"},
		Capabilities: []string{
			"supports_streaming",
			"repository_analysis",
		},
		Requirements: []string{"git_access"},
		Parameters: map[string]string{
			"repo_url":      "required - Repository URL or local path",
			"branch":        "optional - Git branch to analyze",
			"context":       "optional - Additional context about the application",
			"language_hint": "optional - Programming language hint",
			"shallow":       "optional - Perform shallow clone",
		},
		Examples: []mcptypes.ToolExample{
			{
				Name:        "analyze_repo",
				Description: "Analyze a Git repository structure",
				Input: map[string]interface{}{
					"session_id":    "session-123",
					"repo_url":      "https://github.com/user/myapp.git",
					"branch":        "main",
					"language_hint": "nodejs",
				},
				Output: map[string]interface{}{
					"success":           true,
					"detected_language": "javascript",
					"framework":         "express",
					"build_tool":        "npm",
				},
			},
		},
	}
}

// Validate validates the tool arguments (unified interface)
func (t *AtomicAnalyzeRepositoryTool) Validate(ctx context.Context, args interface{}) error {
	analyzeArgs, ok := args.(AtomicAnalyzeRepositoryArgs)
	if !ok {
		return mcperror.NewWithData("invalid_arguments", "Invalid argument type for atomic_analyze_repository", map[string]interface{}{
			"expected": "AtomicAnalyzeRepositoryArgs",
			"received": fmt.Sprintf("%T", args),
		})
	}

	if analyzeArgs.RepoURL == "" {
		return mcperror.NewWithData("missing_required_field", "RepoURL is required", map[string]interface{}{
			"field": "repo_url",
		})
	}

	if analyzeArgs.SessionID == "" {
		return mcperror.NewWithData("missing_required_field", "SessionID is required", map[string]interface{}{
			"field": "session_id",
		})
	}

	return nil
}

// Execute implements unified Tool interface
func (t *AtomicAnalyzeRepositoryTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	analyzeArgs, ok := args.(AtomicAnalyzeRepositoryArgs)
	if !ok {
		return nil, mcperror.NewWithData("invalid_arguments", "Invalid argument type for atomic_analyze_repository", map[string]interface{}{
			"expected": "AtomicAnalyzeRepositoryArgs",
			"received": fmt.Sprintf("%T", args),
		})
	}

	// Call the typed Execute method
	return t.ExecuteTyped(ctx, analyzeArgs)
}

// Legacy interface methods for backward compatibility

// GetName returns the tool name (legacy SimpleTool compatibility)
func (t *AtomicAnalyzeRepositoryTool) GetName() string {
	return t.GetMetadata().Name
}

// GetDescription returns the tool description (legacy SimpleTool compatibility)
func (t *AtomicAnalyzeRepositoryTool) GetDescription() string {
	return t.GetMetadata().Description
}

// GetVersion returns the tool version (legacy SimpleTool compatibility)
func (t *AtomicAnalyzeRepositoryTool) GetVersion() string {
	return t.GetMetadata().Version
}

// GetCapabilities returns the tool capabilities (legacy SimpleTool compatibility)
func (t *AtomicAnalyzeRepositoryTool) GetCapabilities() contract.ToolCapabilities {
	return contract.ToolCapabilities{
		SupportsDryRun:    true,
		SupportsStreaming: true,
		IsLongRunning:     true,
		RequiresAuth:      false,
	}
}

// ExecuteTyped provides the original typed execute method
func (t *AtomicAnalyzeRepositoryTool) ExecuteTyped(ctx context.Context, args AtomicAnalyzeRepositoryArgs) (*AtomicAnalysisResult, error) {
	// Direct execution without progress tracking
	return t.executeWithoutProgress(ctx, args)
}
