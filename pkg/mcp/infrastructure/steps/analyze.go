package steps

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/analysis"
)

// AnalyzeResult contains the results of repository analysis
type AnalyzeResult struct {
	Language  string                 `json:"language"`
	Framework string                 `json:"framework"`
	Port      int                    `json:"port"`
	Analysis  map[string]interface{} `json:"analysis"`
	RepoPath  string                 `json:"repo_path"`
	SessionID string                 `json:"session_id"`
}

// AnalyzeRepository performs repository analysis with git cloning support
func AnalyzeRepository(repoURL, branch string, logger *slog.Logger) (*AnalyzeResult, error) {
	logger.Info("Starting repository analysis",
		"repo_url", repoURL,
		"branch", branch)

	// Basic validation
	if repoURL == "" {
		return nil, fmt.Errorf("repo_url is required")
	}

	var repoPath string
	var needsCleanup bool

	// Handle different URL types
	if strings.HasPrefix(repoURL, "https://github.com/") ||
		strings.HasPrefix(repoURL, "http://github.com/") ||
		strings.HasPrefix(repoURL, "git@github.com:") {

		// Git URL - needs cloning
		logger.Info("Detected git URL, will clone repository", "url", repoURL)

		// Create temporary directory for cloning
		tempDir, err := os.MkdirTemp("", "container-kit-analysis-*")
		if err != nil {
			logger.Error("Failed to create temp directory", "error", err)
			return nil, fmt.Errorf("failed to create temp directory: %v", err)
		}

		repoPath = tempDir
		needsCleanup = true

		// Attempt to clone the repository
		logger.Info("Cloning repository", "url", repoURL, "destination", tempDir, "branch", branch)

		if err := cloneRepository(repoURL, branch, tempDir, logger); err != nil {
			if needsCleanup {
				os.RemoveAll(tempDir)
			}
			return nil, fmt.Errorf("git clone failed: %v", err)
		}

		logger.Info("Git clone successful", "destination", tempDir)
	} else {
		// Local path or file:// URL
		repoPath = strings.TrimPrefix(repoURL, "file://")
		logger.Info("Using local repository path", "path", repoPath)
	}

	// Note: We do NOT clean up the temporary directory here because
	// subsequent steps (like build) need access to the repository files.
	// The cleanup should be handled by the workflow or session manager.

	// Create analysis engine with enhanced logging
	analyzer := analysis.NewRepositoryAnalyzer(logger.With("component", "analyze_repository"))

	// Perform real repository analysis with detailed logging
	logger.Info("Starting repository analysis with analyzer", "path", repoPath)
	result, err := analyzer.AnalyzeRepository(repoPath)
	if err != nil {
		logger.Error("Repository analysis failed", "error", err, "repo_path", repoPath)
		return nil, fmt.Errorf("analysis failed: %v", err)
	}

	// Handle analysis errors
	if result.Error != nil {
		return nil, fmt.Errorf("analysis error: %s", result.Error.Message)
	}

	// Generate a session ID for this analysis
	sessionID := fmt.Sprintf("session_%d", time.Now().Unix())

	// Convert result to analysis map
	analysisMap := map[string]interface{}{
		"files_analyzed":    len(result.ConfigFiles),
		"language":          result.Language,
		"framework":         result.Framework,
		"dependencies":      len(result.Dependencies),
		"entry_points":      result.EntryPoints,
		"build_files":       result.BuildFiles,
		"port":              result.Port,
		"database_detected": result.DatabaseInfo.Detected,
		"database_types":    result.DatabaseInfo.Types,
		"suggestions":       result.Suggestions,
		"timestamp":         time.Now().Format(time.RFC3339),
		"session_id":        sessionID,
	}

	logger.Info("Repository analysis completed successfully",
		"language", result.Language,
		"framework", result.Framework,
		"port", result.Port)

	logger.Info("Returning analysis result", "repo_path", repoPath, "language", result.Language)
	return &AnalyzeResult{
		Language:  result.Language,
		Framework: result.Framework,
		Port:      result.Port,
		Analysis:  analysisMap,
		RepoPath:  repoPath,
		SessionID: sessionID,
	}, nil
}

// cloneRepository clones a git repository with enhanced branch fallback logic
func cloneRepository(repoURL, branch, destDir string, logger *slog.Logger) error {
	// Enhanced git clone with automatic branch fallback
	var attemptedBranches []string

	// Determine branches to try in order
	if branch != "" {
		attemptedBranches = []string{branch}
		// Add fallback branches for common patterns
		if branch == "main" {
			attemptedBranches = append(attemptedBranches, "master", "develop", "dev")
		} else if branch == "master" {
			attemptedBranches = append(attemptedBranches, "main", "develop", "dev")
		}
	} else {
		// Default branch priority: main -> master -> develop -> dev
		attemptedBranches = []string{"main", "master", "develop", "dev"}
	}

	// Also try without specifying branch (let git decide)
	attemptedBranches = append(attemptedBranches, "")

	var lastErr error
	var lastOutput string

	for i, branchToTry := range attemptedBranches {
		var cloneCmd []string

		if branchToTry != "" {
			cloneCmd = []string{"git", "clone", "--depth", "1", "--branch", branchToTry, repoURL, destDir}
			logger.Info("Attempting git clone", "branch", branchToTry, "attempt", i+1, "command", strings.Join(cloneCmd, " "))
		} else {
			cloneCmd = []string{"git", "clone", "--depth", "1", repoURL, destDir}
			logger.Info("Attempting git clone with default branch", "attempt", i+1, "command", strings.Join(cloneCmd, " "))
		}

		// Execute git clone command
		cmd := exec.Command(cloneCmd[0], cloneCmd[1:]...)
		output, err := cmd.CombinedOutput()

		if err == nil {
			// Success!
			if branchToTry != "" {
				logger.Info("Git clone completed successfully", "branch", branchToTry, "attempt", i+1)
			} else {
				logger.Info("Git clone completed successfully with default branch", "attempt", i+1)
			}
			return nil
		}

		// Clone failed, log the attempt and continue to next branch
		lastErr = err
		lastOutput = string(output)

		if branchToTry != "" {
			logger.Warn("Git clone attempt failed", "branch", branchToTry, "attempt", i+1, "error", err)
		} else {
			logger.Warn("Git clone attempt with default branch failed", "attempt", i+1, "error", err)
		}

		// Check if this is a branch-not-found error (we can retry with different branch)
		outputLower := strings.ToLower(lastOutput)
		if strings.Contains(outputLower, "remote branch") && strings.Contains(outputLower, "not found") ||
			strings.Contains(outputLower, "couldn't find remote ref") ||
			strings.Contains(outputLower, "not found in upstream") {
			logger.Info("Branch not found, will try next branch", "failed_branch", branchToTry)
			continue
		}

		// If it's not a branch issue, might be network/auth/repo issue
		// Still try other branches but log this as a more serious error
		if strings.Contains(outputLower, "could not resolve host") ||
			strings.Contains(outputLower, "connection refused") ||
			strings.Contains(outputLower, "permission denied") ||
			strings.Contains(outputLower, "repository not found") {
			logger.Error("Git clone failed with network/auth/repo error", "error", err, "output", lastOutput)
			// Continue trying in case it's a branch-specific issue, but this is less likely to succeed
		}

		// Clean up any partial clone directory before retrying
		if _, statErr := os.Stat(destDir); statErr == nil {
			os.RemoveAll(destDir)
		}
	}

	// All attempts failed
	logger.Error("All git clone attempts failed",
		"attempted_branches", attemptedBranches,
		"final_error", lastErr,
		"final_output", lastOutput)

	return fmt.Errorf("git clone failed after trying branches %v: %v\nFinal output: %s",
		attemptedBranches, lastErr, lastOutput)
}
