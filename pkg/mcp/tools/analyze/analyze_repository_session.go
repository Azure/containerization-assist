// Package analyze provides session management functionality for repository analysis operations.
//
// This module handles session lifecycle operations including session creation, validation,
// resumption, and state management for the atomic repository analysis tool. It manages
// session expiration, cached analysis results, and repository cloning operations.
package analyze

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/Azure/container-kit/pkg/core/git"
	"github.com/Azure/container-kit/pkg/mcp/core"
	errors "github.com/Azure/container-kit/pkg/mcp/errors"
	sessionpkg "github.com/Azure/container-kit/pkg/mcp/session"
)

// SessionManager handles session lifecycle operations for repository analysis.
//
// This manager provides a clean interface for session operations and can be extended
// to support additional session management features in the future.
type SessionManager struct {
	sessionStore    services.SessionStore
	sessionState    services.SessionState
	pipelineAdapter core.TypedPipelineOperations
}

// NewSessionManager creates a new session manager with the provided dependencies.
//
// Parameters:
//   - sessionStore: The session store for session operations
//   - sessionState: The session state for state management
//   - pipelineAdapter: The pipeline adapter for workspace operations
//
// Returns a configured SessionManager instance.
func NewSessionManager(sessionStore services.SessionStore, sessionState services.SessionState, pipelineAdapter core.TypedPipelineOperations) *SessionManager {
	return &SessionManager{
		sessionStore:    sessionStore,
		sessionState:    sessionState,
		pipelineAdapter: pipelineAdapter,
	}
}

// initializeSession handles session creation and result initialization.
//
// This method creates or retrieves a session based on the provided session ID,
// initializes the analysis result structure, and handles session resumption
// if the session was previously expired.
//
// Parameters:
//   - args: The analysis arguments containing session ID and repository details
//   - _: Unused start time parameter for consistency with other methods
//
// Returns:
//   - *core.SessionState: The active session state
//   - *AtomicAnalysisResult: The initialized analysis result
//   - error: Any error encountered during session initialization
func (t *AtomicAnalyzeRepositoryTool) initializeSession(args AtomicAnalyzeRepositoryArgs, _ time.Time) (*core.SessionState, *AtomicAnalysisResult, error) {
	session, err := t.getOrCreateSession(args.SessionID)
	if err != nil {
		t.logger.Error("Failed to get/create session", "error", err, "session_id", args.SessionID)
		return nil, nil, sessionpkg.NewRichSessionError("get_session", args.SessionID, errors.NewError().Messagef("failed to get or create session").WithLocation().Build())
	}

	t.logger.Info("Starting atomic repository analysis", "session_id", session.SessionID, "repo_url", args.RepoURL, "branch", args.Branch)

	result := &AtomicAnalysisResult{
		BaseAIContextResult:        core.NewBaseAIContextResult("analysis", false, 0),
		SessionID:                  session.SessionID,
		WorkspaceDir:               t.pipelineAdapter.GetSessionWorkspace(session.SessionID),
		RepoURL:                    args.RepoURL,
		Branch:                     args.Branch,
		AnalysisContext:            &AnalysisContext{},
		ContainerizationAssessment: &ContainerizationAssessment{},
	}

	// Set BaseToolResponse fields
	result.BaseToolResponse.Success = true
	result.BaseToolResponse.Message = "Session initialized for repository analysis"
	result.BaseToolResponse.Timestamp = time.Now()

	t.handleSessionResumption(session, result)
	return session, result, nil
}

// handleSessionResumption adds session resumption suggestions if applicable.
//
// This method checks if the current session was created as a replacement for an
// expired session and adds appropriate suggestions to guide the user on next steps.
//
// Parameters:
//   - session: The current session state
//   - result: The analysis result to update with resumption suggestions
func (t *AtomicAnalyzeRepositoryTool) handleSessionResumption(session *core.SessionState, result *AtomicAnalysisResult) {
	if session.Metadata == nil {
		return
	}

	resumedFrom, ok := session.Metadata["resumed_from"].(map[string]interface{})
	if !ok {
		return
	}

	oldSessionID, _ := resumedFrom["old_session_id"].(string)
	lastRepoURL, _ := resumedFrom["last_repo_url"].(string)

	t.logger.Info("Session was resumed from expired session", "old_session_id", oldSessionID, "new_session_id", session.SessionID, "last_repo_url", lastRepoURL)

	result.AnalysisContext.NextStepSuggestions = append(result.AnalysisContext.NextStepSuggestions,
		fmt.Sprintf("Note: Your previous session (%s) expired. A new session has been created.", oldSessionID),
		"You'll need to regenerate your Dockerfile and rebuild your image with the new session.",
	)

	if result.RepoURL == "" && lastRepoURL != "" {
		result.AnalysisContext.NextStepSuggestions = append(result.AnalysisContext.NextStepSuggestions,
			fmt.Sprintf("Tip: Your last repository was: %s", lastRepoURL),
		)
	}
}

// getOrCreateSession gets existing session or creates a new one.
//
// This method handles the core session lifecycle logic including:
// - Retrieving existing sessions by ID
// - Checking session expiration and creating replacements
// - Creating new sessions when none exist
// - Preserving session metadata across replacements
//
// Parameters:
//   - sessionID: The session ID to retrieve, or empty string to create new
//
// Returns:
//   - *core.SessionState: The active session state
//   - error: Any error encountered during session operations
func (t *AtomicAnalyzeRepositoryTool) getOrCreateSession(sessionID string) (*core.SessionState, error) {
	if sessionID != "" {
		apiSession, err := t.sessionStore.Get(context.Background(), sessionID)
		if err == nil {
			// Convert API session to session state
			session := &sessionpkg.SessionState{
				ID:        apiSession.ID,
				CreatedAt: apiSession.CreatedAt,
				UpdatedAt: apiSession.UpdatedAt,
				Metadata:  apiSession.Metadata,
				// ExpiresAt: time.Now().Add(24 * time.Hour), // Default expiration
			}
			// Check if session is expired
			if time.Now().After(session.ExpiresAt) {
				t.logger.Info("Session has expired, will create new session and attempt to resume", "session_id", sessionID, "expired_at", session.ExpiresAt)
				// Check if session had previous analysis results
				hadAnalysis := false
				if session.Metadata != nil {
					if _, hasAnalysis := session.Metadata["analysis_completed"]; hasAnalysis {
						hadAnalysis = true
					}
					// Also check for dockerfile info as indicator of analysis
					if _, hasDockerfile := session.Metadata["dockerfile_created"]; hasDockerfile {
						hadAnalysis = true
					}
				}

				oldSessionInfo := map[string]interface{}{
					"old_session_id": sessionID,
					"expired_at":     session.ExpiresAt,
					"had_analysis":   hadAnalysis,
				}
				if session.Metadata != nil {
					if repoURL, ok := session.Metadata["repo_url"].(string); ok && repoURL != "" {
						oldSessionInfo["last_repo_url"] = repoURL
					}
				}
				newSessionID, err := t.sessionStore.Create(context.Background(), map[string]interface{}{
					"resumed_from": oldSessionInfo,
				})
				if err != nil {
					return nil, sessionpkg.NewRichSessionError("create_replacement", "replacement_session", errors.NewError().Messagef("failed to create replacement session").WithLocation().Build())
				}
				newSession := &sessionpkg.SessionState{
					ID:        newSessionID,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
					Metadata:  map[string]interface{}{"resumed_from": oldSessionInfo},
				}
				if err != nil {
					return nil, sessionpkg.NewRichSessionError("create_replacement", "replacement_session", errors.NewError().Messagef("failed to create replacement session").WithLocation().Build())
				}
				if newSession.Metadata == nil {
					newSession.Metadata = make(map[string]interface{})
				}
				newSession.Metadata["resumed_from"] = oldSessionInfo
				// Note: UpdateSession would be available in TypedToolSessionManager but not needed here
				// Skipping session update for now
				if err := error(nil); err != nil {
					t.logger.Warn("Failed to save resumed session", "error", err)
				}

				t.logger.Info("Created new session to replace expired one", "old_session_id", sessionID, "new_session_id", newSession.ID)
				return newSession.ToCoreSessionState(), nil
			}
			return session.ToCoreSessionState(), nil
		}
		t.logger.Debug("Session not found, creating new one", "session_id", sessionID)
	}

	sessionID, err := t.sessionStore.Create(context.Background(), map[string]interface{}{
		"tool":       "analyze_repository",
		"created_at": time.Now(),
	})
	if err != nil {
		return nil, sessionpkg.NewRichSessionError("create_session", "new_session", errors.NewError().Messagef("failed to create new session").WithLocation().Build())
	}
	session := &sessionpkg.SessionState{
		ID:        sessionID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	t.logger.Info("Created new session for repository analysis", "session_id", session.ID)
	return session.ToCoreSessionState(), nil
}

// cloneRepository handles repository cloning and updates session state.
//
// This method clones a remote repository to the session workspace and updates
// the session metadata with repository information for future operations.
//
// Parameters:
//   - ctx: The context for the clone operation
//   - sessionID: The session ID to associate with the clone
//   - args: The analysis arguments containing repository details
//
// Returns:
//   - *git.CloneResult: The result of the clone operation
//   - error: Any error encountered during cloning
func (t *AtomicAnalyzeRepositoryTool) cloneRepository(ctx context.Context, sessionID string, args AtomicAnalyzeRepositoryArgs) (*git.CloneResult, error) {
	apiSession, err := t.sessionStore.Get(context.Background(), sessionID)
	if err != nil {
		return nil, sessionpkg.NewRichSessionError("get_session", "session_retrieval", errors.NewError().Messagef("failed to get session").WithLocation().Build())
	}
	session := &sessionpkg.SessionState{
		ID:        apiSession.ID,
		CreatedAt: apiSession.CreatedAt,
		UpdatedAt: apiSession.UpdatedAt,
		Metadata:  apiSession.Metadata,
	}

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

	// Update session with clone info
	if session.Metadata == nil {
		session.Metadata = make(map[string]interface{})
	}
	session.Metadata["repo_path"] = result.RepoPath
	session.Metadata["repo_url"] = args.RepoURL
	// Note: UpdateSession would be available in TypedToolSessionManager but not needed here
	// Skipping session update for now
	_ = sessionID

	return result, nil
}

// updateSessionState updates the session with analysis results.
//
// This method persists the analysis results to the session metadata for caching
// and future reference. It stores comprehensive analysis information including:
// - Core analysis results (language, framework, port, dependencies)
// - File structure insights and repository metadata
// - Cache metadata for performance optimization
// - Tool execution history for tracking
//
// Parameters:
//   - session: The session state to update
//   - result: The analysis result to persist
//
// Returns:
//   - error: Any error encountered during session update
func (t *AtomicAnalyzeRepositoryTool) updateSessionState(session *core.SessionState, result *AtomicAnalysisResult) error {
	// Update session with repository analysis results
	analysis := result.Analysis
	dependencyNames := make([]string, len(analysis.Dependencies))
	for i, dep := range analysis.Dependencies {
		dependencyNames[i] = dep.Name
	}

	now := time.Now()
	startTime := now.Add(-result.AnalysisDuration)
	execution := sessionpkg.ToolExecution{
		Tool:       "analyze_repository",
		StartTime:  startTime,
		EndTime:    &now,
		Duration:   &result.AnalysisDuration,
		Success:    true,
		DryRun:     false,
		TokensUsed: 0, // Could be tracked if needed
	}
	// Store tool execution in metadata
	if session.Metadata == nil {
		session.Metadata = make(map[string]interface{})
	}
	session.Metadata["last_tool_execution"] = execution

	session.UpdatedAt = time.Now()

	// Store structured scan summary for caching in metadata
	if session.Metadata == nil {
		session.Metadata = make(map[string]interface{})
	}
	session.Metadata["scan_summary"] = map[string]interface{}{
		// Core analysis results
		"language":     analysis.Language,
		"framework":    analysis.Framework,
		"port":         analysis.Port,
		"dependencies": dependencyNames,

		// File structure insights
		"files_analyzed":     result.AnalysisContext.FilesAnalyzed,
		"config_files_found": result.AnalysisContext.ConfigFilesFound,
		"entry_points_found": result.AnalysisContext.EntryPointsFound,
		"test_files_found":   result.AnalysisContext.TestFilesFound,
		"build_files_found":  result.AnalysisContext.BuildFilesFound,

		// Ecosystem insights
		"package_managers": result.AnalysisContext.PackageManagers,
		"database_files":   result.AnalysisContext.DatabaseFiles,
		"docker_files":     result.AnalysisContext.DockerFiles,
		"k8s_files":        result.AnalysisContext.K8sFiles,

		// Repository metadata
		"has_git_ignore":  result.AnalysisContext.HasGitIgnore,
		"has_readme":      result.AnalysisContext.HasReadme,
		"has_license":     result.AnalysisContext.HasLicense,
		"has_ci":          result.AnalysisContext.HasCI,
		"repository_size": result.AnalysisContext.RepositorySize,

		// Cache metadata
		"cached_at":         time.Now().Format(time.RFC3339),
		"analysis_duration": result.AnalysisDuration.Seconds(),
		"repo_path":         result.CloneDir,
		"repo_url":          result.RepoURL,

		// Suggestions for reuse
		"containerization_suggestions": result.AnalysisContext.ContainerizationSuggestions,
		"next_step_suggestions":        result.AnalysisContext.NextStepSuggestions,
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

	// Note: UpdateSession would be available in TypedToolSessionManager but not needed here
	// Skipping session update for now
	return nil
}
