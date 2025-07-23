package steps

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/core/utilities"
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

// AnalyzeRepository performs repository analysis supporting both URLs and local paths
// This function handles git cloning when needed and ensures all artifacts are written to the repository directory
func AnalyzeRepository(input, branch string, logger *slog.Logger) (*AnalyzeResult, error) {
	// Determine input type and log accordingly
	isURL := strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "git@")

	if isURL {
		logger.Info("Starting repository analysis with URL", "repo_url", input, "branch", branch)
	} else {
		logger.Info("Starting repository analysis with local path", "repo_path", input)
	}

	// Basic validation
	if input == "" {
		return nil, fmt.Errorf("repository input (URL or path) is required")
	}

	var repoPath string
	var needsCleanup bool

	if isURL {
		// Git URL - needs cloning
		logger.Info("Detected git URL, will clone repository", "url", input)

		// Create temporary directory for cloning
		tempDir, err := os.MkdirTemp("", "container-kit-analysis-*")
		if err != nil {
			logger.Error("Failed to create temp directory", "error", err)
			return nil, fmt.Errorf("failed to create temp directory: %v", err)
		}

		repoPath = tempDir
		needsCleanup = true

		// Attempt to clone the repository
		logger.Info("Cloning repository", "url", input, "destination", tempDir, "branch", branch)

		if err := cloneRepository(input, branch, tempDir, logger); err != nil {
			if needsCleanup {
				os.RemoveAll(tempDir)
			}
			return nil, fmt.Errorf("git clone failed: %v", err)
		}

		logger.Info("Git clone successful", "destination", tempDir)
	} else {
		// Local path or file:// URL
		repoPath = strings.TrimPrefix(input, "file://")

		// Validate local path exists and is a directory
		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("repository path does not exist: %s", repoPath)
		}

		fileInfo, err := os.Stat(repoPath)
		if err != nil {
			return nil, fmt.Errorf("failed to stat repository path: %v", err)
		}

		if !fileInfo.IsDir() {
			return nil, fmt.Errorf("repository path is not a directory: %s", repoPath)
		}

		logger.Info("Using local repository path", "path", repoPath)
	}

	// Note: We do NOT clean up the temporary directory here because
	// subsequent steps (like Dockerfile generation and build) need access to the repository files.
	// The cleanup should be handled by the workflow or session manager.

	// Create analysis engine with enhanced logging
	analyzer := utilities.NewRepositoryAnalyzer(logger.With("component", "analyze_repository"))

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
