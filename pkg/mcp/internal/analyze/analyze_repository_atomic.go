package analyze

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/core/analysis"
	"github.com/Azure/container-kit/pkg/core/git"
	sessiontypes "github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	mcperror "github.com/Azure/container-kit/pkg/mcp/internal/utils"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/Azure/container-kit/pkg/mcp/utils"
	"github.com/localrivet/gomcp/server"
	"github.com/rs/zerolog"
)

type AtomicAnalyzeRepositoryArgs struct {
	types.BaseToolArgs
	RepoURL      string `json:"repo_url" description:"Repository URL (GitHub, GitLab, etc.) or local path"`
	Branch       string `json:"branch,omitempty" description:"Git branch to analyze (default: main)"`
	Context      string `json:"context,omitempty" description:"Additional context about the application"`
	LanguageHint string `json:"language_hint,omitempty" description:"Primary programming language hint"`
	Shallow      bool   `json:"shallow,omitempty" description:"Perform shallow clone for faster analysis"`
}

type AtomicAnalysisResult struct {
	types.BaseToolResponse
	mcptypes.BaseAIContextResult
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
	pipelineAdapter  mcptypes.PipelineOperations
	sessionManager   mcptypes.ToolSessionManager
	logger           zerolog.Logger
	gitManager       *git.Manager
	repoAnalyzer     *analysis.RepositoryAnalyzer
	repoCloner       *git.Manager
	contextGenerator *ContextGenerator
}

func NewAtomicAnalyzeRepositoryTool(adapter mcptypes.PipelineOperations, sessionManager mcptypes.ToolSessionManager, logger zerolog.Logger) *AtomicAnalyzeRepositoryTool {
	return &AtomicAnalyzeRepositoryTool{
		pipelineAdapter:  adapter,
		sessionManager:   sessionManager,
		logger:           logger.With().Str("tool", "atomic_analyze_repository").Logger(),
		gitManager:       git.NewManager(logger),
		repoAnalyzer:     analysis.NewRepositoryAnalyzer(logger),
		repoCloner:       git.NewManager(logger),
		contextGenerator: NewContextGenerator(logger),
	}
}

func (t *AtomicAnalyzeRepositoryTool) ExecuteRepositoryAnalysis(ctx context.Context, args AtomicAnalyzeRepositoryArgs) (*AtomicAnalysisResult, error) {
	return t.executeWithoutProgress(ctx, args)
}

func (t *AtomicAnalyzeRepositoryTool) ExecuteWithContext(serverCtx *server.Context, args AtomicAnalyzeRepositoryArgs) (*AtomicAnalysisResult, error) {
	_ = mcptypes.NewGoMCPProgressAdapter(serverCtx, []mcptypes.LocalProgressStage{{Name: "Initialize", Weight: 0.10, Description: "Loading session"}, {Name: "Analyze", Weight: 0.80, Description: "Analyzing"}, {Name: "Finalize", Weight: 0.10, Description: "Updating state"}})

	ctx := context.Background()
	result, err := t.performAnalysis(ctx, args, nil)

	if err != nil {
		t.logger.Info().Msg("Analysis failed")
		if result != nil {
			result.Success = false
		}
		return result, nil
	} else {
		t.logger.Info().Msg("Analysis completed successfully")
	}

	return result, nil
}

func (t *AtomicAnalyzeRepositoryTool) executeWithoutProgress(ctx context.Context, args AtomicAnalyzeRepositoryArgs) (*AtomicAnalysisResult, error) {
	return t.performAnalysis(ctx, args, nil)
}

func (t *AtomicAnalyzeRepositoryTool) performAnalysis(ctx context.Context, args AtomicAnalyzeRepositoryArgs, reporter interface{}) (*AtomicAnalysisResult, error) {
	startTime := time.Now()

	session, err := t.getOrCreateSession(args.SessionID)
	if err != nil {
		result := &AtomicAnalysisResult{
			BaseToolResponse:           types.NewBaseResponse("atomic_analyze_repository", args.SessionID, args.DryRun),
			BaseAIContextResult:        mcptypes.NewBaseAIContextResult("analysis", false, time.Since(startTime)),
			SessionID:                  args.SessionID,
			RepoURL:                    args.RepoURL,
			Branch:                     args.Branch,
			TotalDuration:              time.Since(startTime),
			AnalysisContext:            &AnalysisContext{},
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

	result := &AtomicAnalysisResult{
		BaseToolResponse:           types.NewBaseResponse("atomic_analyze_repository", session.SessionID, args.DryRun),
		BaseAIContextResult:        mcptypes.NewBaseAIContextResult("analysis", false, 0),
		SessionID:                  session.SessionID,
		WorkspaceDir:               t.pipelineAdapter.GetSessionWorkspace(session.SessionID),
		RepoURL:                    args.RepoURL,
		Branch:                     args.Branch,
		AnalysisContext:            &AnalysisContext{},
		ContainerizationAssessment: &ContainerizationAssessment{},
	}

	if session.Metadata != nil {
		if resumedFrom, ok := session.Metadata["resumed_from"].(map[string]interface{}); ok {
			oldSessionID, _ := resumedFrom["old_session_id"].(string)
			lastRepoURL, _ := resumedFrom["last_repo_url"].(string)

			t.logger.Info().
				Str("old_session_id", oldSessionID).
				Str("new_session_id", session.SessionID).
				Str("last_repo_url", lastRepoURL).
				Msg("Session was resumed from expired session")

			result.AnalysisContext.NextStepSuggestions = append(result.AnalysisContext.NextStepSuggestions,
				fmt.Sprintf("Note: Your previous session (%s) expired. A new session has been created.", oldSessionID),
				"You'll need to regenerate your Dockerfile and rebuild your image with the new session.",
			)

			if args.RepoURL == "" && lastRepoURL != "" {
				result.AnalysisContext.NextStepSuggestions = append(result.AnalysisContext.NextStepSuggestions,
					fmt.Sprintf("Tip: Your last repository was: %s", lastRepoURL),
				)
			}
		}
	}

	if args.DryRun {
		result.AnalysisContext.NextStepSuggestions = []string{
			"This is a dry-run - actual repository cloning and analysis would be performed",
			"Session workspace would be created at: " + result.WorkspaceDir,
		}
		result.TotalDuration = time.Since(startTime)
		return result, nil
	}

	if t.isURL(args.RepoURL) {

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

	} else {
		if err := utils.ValidateLocalPath(args.RepoURL); err != nil {
			t.logger.Error().Err(err).
				Str("local_path", args.RepoURL).
				Str("session_id", session.SessionID).
				Msg("Invalid local path for repository")
			result.Success = false
			result.TotalDuration = time.Since(startTime)
			return result, nil
		}
		result.CloneDir = args.RepoURL

	}

	if session.ScanSummary != nil && session.ScanSummary.RepoPath == result.CloneDir {
		if time.Since(session.ScanSummary.CachedAt) < time.Hour {
			t.logger.Info().
				Str("session_id", session.SessionID).
				Str("repo_path", result.CloneDir).
				Time("cached_at", session.ScanSummary.CachedAt).
				Msg("Using cached repository analysis results")

			result.Analysis = &analysis.AnalysisResult{
				Language:     session.ScanSummary.Language,
				Framework:    session.ScanSummary.Framework,
				Port:         session.ScanSummary.Port,
				Dependencies: make([]analysis.Dependency, len(session.ScanSummary.Dependencies)),
			}

			for i, dep := range session.ScanSummary.Dependencies {
				result.Analysis.Dependencies[i] = analysis.Dependency{Name: dep}
			}

			result.AnalysisContext = &AnalysisContext{
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
			result.IsSuccessful = true
			result.Duration = result.TotalDuration

			t.logger.Info().
				Str("session_id", session.SessionID).
				Str("language", result.Analysis.Language).
				Str("framework", result.Analysis.Framework).
				Dur("cached_analysis_duration", result.AnalysisDuration).
				Dur("total_duration", result.TotalDuration).
				Msg("Repository analysis completed using cached results")

			return result, nil
		} else {
			t.logger.Info().
				Str("session_id", session.SessionID).
				Time("cached_at", session.ScanSummary.CachedAt).
				Dur("cache_age", time.Since(session.ScanSummary.CachedAt)).
				Msg("Cached analysis results are stale, performing fresh analysis")
		}
	}

	analysisStartTime := time.Now()
	analysisOpts := AnalysisOptions{
		RepoPath:     result.CloneDir,
		Context:      args.Context,
		LanguageHint: args.LanguageHint,
		SessionID:    session.SessionID,
	}

	coreAnalysisResult, err := t.repoAnalyzer.AnalyzeRepository(analysisOpts.RepoPath)
	if err != nil {
		return result, err
	}

	repoAnalysisResult := &AnalysisResult{
		AnalysisResult: coreAnalysisResult,
		Duration:       time.Since(analysisStartTime),
		Context:        t.generateAnalysisContext(analysisOpts.RepoPath, coreAnalysisResult),
	}
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

	assessment, err := t.contextGenerator.GenerateContainerizationAssessment(result.Analysis, result.AnalysisContext)
	if err != nil {
		t.logger.Warn().Err(err).Msg("Failed to generate containerization assessment")
	} else {
		result.ContainerizationAssessment = assessment
	}

	if err := t.updateSessionState(session, result); err != nil {
		t.logger.Warn().Err(err).Msg("Failed to update session state")
	}

	result.Success = true
	result.TotalDuration = time.Since(startTime)

	result.IsSuccessful = true
	result.Duration = result.TotalDuration

	t.logger.Info().
		Str("session_id", session.SessionID).
		Str("language", result.Analysis.Language).
		Str("framework", result.Analysis.Framework).
		Int("files_analyzed", result.AnalysisContext.FilesAnalyzed).
		Dur("total_duration", result.TotalDuration).
		Msg("Atomic repository analysis completed successfully")

	return result, nil
}

func (t *AtomicAnalyzeRepositoryTool) getOrCreateSession(sessionID string) (*sessiontypes.SessionState, error) {
	if sessionID != "" {
		sessionInterface, err := t.sessionManager.GetSession(sessionID)
		if err == nil {
			session := sessionInterface.(*sessiontypes.SessionState)
			if time.Now().After(session.ExpiresAt) {
				t.logger.Info().
					Str("session_id", sessionID).
					Time("expired_at", session.ExpiresAt).
					Msg("Session has expired, will create new session and attempt to resume")
				oldSessionInfo := map[string]interface{}{
					"old_session_id": sessionID,
					"expired_at":     session.ExpiresAt,
					"had_analysis":   session.ScanSummary != nil && session.ScanSummary.FilesAnalyzed > 0,
				}
				if session.ScanSummary != nil && session.ScanSummary.RepoURL != "" {
					oldSessionInfo["last_repo_url"] = session.ScanSummary.RepoURL
				}
				newSessionInterface, err := t.sessionManager.GetOrCreateSession("")
				if err != nil {
					return nil, mcperror.NewSessionNotFound("replacement_session")
				}
				newSession := newSessionInterface.(*sessiontypes.SessionState)
				if newSession.Metadata == nil {
					newSession.Metadata = make(map[string]interface{})
				}
				newSession.Metadata["resumed_from"] = oldSessionInfo
				if err := t.sessionManager.UpdateSession(newSession.SessionID, func(s interface{}) {
					if sess, ok := s.(*sessiontypes.SessionState); ok {
						*sess = *newSession
					}
				}); err != nil {
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

	sessionInterface, err := t.sessionManager.GetOrCreateSession("")
	if err != nil {
		return nil, mcperror.NewSessionNotFound("new_session")
	}
	session := sessionInterface.(*sessiontypes.SessionState)

	t.logger.Info().Str("session_id", session.SessionID).Msg("Created new session for repository analysis")
	return session, nil
}

func (t *AtomicAnalyzeRepositoryTool) cloneRepository(ctx context.Context, sessionID string, args AtomicAnalyzeRepositoryArgs) (*git.CloneResult, error) {
	sessionInterface, err := t.sessionManager.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	session := sessionInterface.(*sessiontypes.SessionState)

	cloneOpts := CloneOptions{
		RepoURL:   args.RepoURL,
		Branch:    args.Branch,
		Shallow:   args.Shallow,
		TargetDir: filepath.Join(session.WorkspaceDir, "repo"),
		SessionID: sessionID,
	}

	result, err := t.repoCloner.CloneRepository(ctx, cloneOpts.TargetDir, git.CloneOptions{
		URL:          cloneOpts.RepoURL,
		Branch:       cloneOpts.Branch,
		Depth:        1,
		SingleBranch: true,
		Recursive:    false,
	})
	if err != nil {
		return nil, err
	}

	session.RepoPath = result.RepoPath
	session.RepoURL = args.RepoURL
	t.sessionManager.UpdateSession(sessionID, func(s interface{}) {
		if sess, ok := s.(*sessiontypes.SessionState); ok {
			sess.RepoPath = result.RepoPath
			sess.RepoURL = args.RepoURL
		}
	})

	return result, nil
}

func (t *AtomicAnalyzeRepositoryTool) updateSessionState(session *sessiontypes.SessionState, result *AtomicAnalysisResult) error {
	analysis := result.Analysis
	dependencyNames := make([]string, len(analysis.Dependencies))
	for i, dep := range analysis.Dependencies {
		dependencyNames[i] = dep.Name
	}

	now := time.Now()
	startTime := now.Add(-result.AnalysisDuration)
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

	session.ScanSummary = &types.RepositoryScanSummary{
		Language:     analysis.Language,
		Framework:    analysis.Framework,
		Port:         analysis.Port,
		Dependencies: dependencyNames,

		FilesAnalyzed:    result.AnalysisContext.FilesAnalyzed,
		ConfigFilesFound: result.AnalysisContext.ConfigFilesFound,
		EntryPointsFound: result.AnalysisContext.EntryPointsFound,
		TestFilesFound:   result.AnalysisContext.TestFilesFound,
		BuildFilesFound:  result.AnalysisContext.BuildFilesFound,

		PackageManagers: result.AnalysisContext.PackageManagers,
		DatabaseFiles:   result.AnalysisContext.DatabaseFiles,
		DockerFiles:     result.AnalysisContext.DockerFiles,
		K8sFiles:        result.AnalysisContext.K8sFiles,

		HasGitIgnore:   result.AnalysisContext.HasGitIgnore,
		HasReadme:      result.AnalysisContext.HasReadme,
		HasLicense:     result.AnalysisContext.HasLicense,
		HasCI:          result.AnalysisContext.HasCI,
		RepositorySize: result.AnalysisContext.RepositorySize,

		CachedAt:         time.Now(),
		AnalysisDuration: result.AnalysisDuration.Seconds(),
		RepoPath:         result.CloneDir,
		RepoURL:          result.RepoURL,

		ContainerizationSuggestions: result.AnalysisContext.ContainerizationSuggestions,
		NextStepSuggestions:         result.AnalysisContext.NextStepSuggestions,
	}

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

	return t.sessionManager.UpdateSession(session.SessionID, func(s interface{}) {
		if sess, ok := s.(*sessiontypes.SessionState); ok {
			*sess = *session
		}
	})
}

func (t *AtomicAnalyzeRepositoryTool) isURL(path string) bool {
	return strings.HasPrefix(path, "http://") ||
		strings.HasPrefix(path, "https://") ||
		strings.HasPrefix(path, "git@") ||
		strings.HasPrefix(path, "ssh://")
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

func (t *AtomicAnalyzeRepositoryTool) Validate(ctx context.Context, args interface{}) error {
	var analyzeArgs AtomicAnalyzeRepositoryArgs
	switch v := args.(type) {
	case AtomicAnalyzeRepositoryArgs:
		analyzeArgs = v
	case *AtomicAnalyzeRepositoryArgs:
		analyzeArgs = *v
	default:
		return mcperror.NewWithData("invalid_arguments", "Invalid argument type for atomic_analyze_repository", map[string]interface{}{
			"expected": "AtomicAnalyzeRepositoryArgs or *AtomicAnalyzeRepositoryArgs",
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

func (t *AtomicAnalyzeRepositoryTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	var analyzeArgs AtomicAnalyzeRepositoryArgs

	switch v := args.(type) {
	case AtomicAnalyzeRepositoryArgs:
		analyzeArgs = v
	case *AtomicAnalyzeRepositoryArgs:
		analyzeArgs = *v
	default:
		if converted := t.convertFromOrchestrationArgs(args); converted != nil {
			analyzeArgs = *converted
		} else {
			t.logger.Error().Str("received_type", fmt.Sprintf("%T", args)).Msg("Invalid argument type received")
			return nil, mcperror.NewWithData("invalid_arguments", "Invalid argument type for atomic_analyze_repository", map[string]interface{}{
				"expected": "AtomicAnalyzeRepositoryArgs, *AtomicAnalyzeRepositoryArgs, or orchestration types",
				"received": fmt.Sprintf("%T", args),
			})
		}
	}

	return t.ExecuteTyped(ctx, analyzeArgs)
}

func (t *AtomicAnalyzeRepositoryTool) convertFromOrchestrationArgs(args interface{}) *AtomicAnalyzeRepositoryArgs {
	switch v := args.(type) {
	case interface{}:
		if converted := t.extractFieldsFromInterface(v); converted != nil {
			return converted
		}
	}

	return nil
}

func (t *AtomicAnalyzeRepositoryTool) extractFieldsFromInterface(v interface{}) *AtomicAnalyzeRepositoryArgs {
	return t.convertViaJSON(v)
}

func (t *AtomicAnalyzeRepositoryTool) convertViaJSON(v interface{}) *AtomicAnalyzeRepositoryArgs {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		t.logger.Error().Err(err).Msg("Failed to marshal args to JSON")
		return nil
	}

	var result AtomicAnalyzeRepositoryArgs
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		t.logger.Error().Err(err).Msg("Failed to unmarshal JSON to AtomicAnalyzeRepositoryArgs")
		return nil
	}

	t.logger.Info().Msg("Successfully converted orchestration args via JSON")
	return &result
}

func (t *AtomicAnalyzeRepositoryTool) GetName() string {
	return t.GetMetadata().Name
}

func (t *AtomicAnalyzeRepositoryTool) GetDescription() string {
	return t.GetMetadata().Description
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

func (t *AtomicAnalyzeRepositoryTool) ExecuteTyped(ctx context.Context, args AtomicAnalyzeRepositoryArgs) (*AtomicAnalysisResult, error) {
	return t.executeWithoutProgress(ctx, args)
}
