package steps

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Azure/containerization-assist/pkg/domain/workflow"
	"github.com/Azure/containerization-assist/pkg/infrastructure/core"
)

type AnalyzeResult struct {
	Language        string                 `json:"language"`
	LanguageVersion string                 `json:"language_version"`
	Framework       string                 `json:"framework"`
	Port            int                    `json:"port"`
	Dependencies    []workflow.Dependency  `json:"dependencies"`
	Analysis        map[string]interface{} `json:"analysis"`
	RepoPath        string                 `json:"repo_path"`
	SessionID       string                 `json:"session_id"`
}

// AnalyzeRepository analyzes repositories from URLs or local paths, handling git cloning when needed
func AnalyzeRepository(input, branch string, logger *slog.Logger) (*AnalyzeResult, error) {
	isURL := strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "git@")

	if isURL {
	} else {
	}

	if input == "" {
		return nil, fmt.Errorf("repository input (URL or path) is required")
	}

	var repoPath string
	var needsCleanup bool

	if isURL {
		tempDir, err := os.MkdirTemp("", "containerization-assist-analysis-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp directory: %v", err)
		}

		repoPath = tempDir
		needsCleanup = true

		if err := cloneRepository(input, branch, tempDir, logger); err != nil {
			if needsCleanup {
				_ = os.RemoveAll(tempDir)
			}
			return nil, fmt.Errorf("git clone failed: %v", err)
		}

	} else {
		repoPath = strings.TrimPrefix(input, "file://")

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

	}

	analyzer := core.NewRepositoryAnalyzer(logger)

	result, err := analyzer.AnalyzeRepository(repoPath)
	if err != nil {
		return nil, fmt.Errorf("analysis failed: %v", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("analysis error: %s", result.Error.Message)
	}

	sessionID := fmt.Sprintf("session_%d", time.Now().Unix())

	analysisMap := map[string]interface{}{
		"structure":         result.Structure,
		"files_analyzed":    len(result.ConfigFiles),
		"language":          result.Language,
		"language_version":  result.LanguageVersion,
		"framework":         result.Framework,
		"dependencies":      result.Dependencies,
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
		"language_version", result.LanguageVersion,
		"framework", result.Framework,
		"port", result.Port)

	// Convert utils.Dependency to workflow.Dependency
	workflowDeps := make([]workflow.Dependency, len(result.Dependencies))
	for i, dep := range result.Dependencies {
		workflowDeps[i] = workflow.Dependency{
			Name:    dep.Name,
			Version: dep.Version,
			Type:    dep.Type,
			Manager: dep.Manager,
		}
	}

	logger.Info("Returning analysis result", "repo_path", repoPath, "language", result.Language)
	return &AnalyzeResult{
		Language:        result.Language,
		LanguageVersion: result.LanguageVersion,
		Framework:       result.Framework,
		Port:            result.Port,
		Dependencies:    workflowDeps,
		Analysis:        analysisMap,
		RepoPath:        repoPath,
		SessionID:       sessionID,
	}, nil
}

func cloneRepository(repoURL, branch, destDir string, logger *slog.Logger) error {
	var attemptedBranches []string

	if branch != "" {
		attemptedBranches = []string{branch}
		if branch == "main" {
			attemptedBranches = append(attemptedBranches, "master", "develop", "dev")
		} else if branch == "master" {
			attemptedBranches = append(attemptedBranches, "main", "develop", "dev")
		}
	} else {
		attemptedBranches = []string{"main", "master", "develop", "dev"}
	}

	attemptedBranches = append(attemptedBranches, "")

	var lastErr error
	var lastOutput string

	for _, branchToTry := range attemptedBranches {
		var cloneCmd []string

		if branchToTry != "" {
			cloneCmd = []string{"git", "clone", "--depth", "1", "--branch", branchToTry, repoURL, destDir}
		} else {
			cloneCmd = []string{"git", "clone", "--depth", "1", repoURL, destDir}
		}

		// Execute git clone command
		cmd := exec.Command(cloneCmd[0], cloneCmd[1:]...)
		output, err := cmd.CombinedOutput()

		if err == nil {
			// Success!
			if branchToTry != "" {
			} else {
			}
			return nil
		}

		// Clone failed, log the attempt and continue to next branch
		lastErr = err
		lastOutput = string(output)

		if branchToTry != "" {
		} else {
		}

		// Check if this is a branch-not-found error (we can retry with different branch)
		outputLower := strings.ToLower(lastOutput)
		if strings.Contains(outputLower, "remote branch") && strings.Contains(outputLower, "not found") ||
			strings.Contains(outputLower, "couldn't find remote ref") ||
			strings.Contains(outputLower, "not found in upstream") {
			continue
		}

		// If it's not a branch issue, might be network/auth/repo issue
		// Still try other branches but log this as a more serious error
		if strings.Contains(outputLower, "could not resolve host") ||
			strings.Contains(outputLower, "connection refused") ||
			strings.Contains(outputLower, "permission denied") ||
			strings.Contains(outputLower, "repository not found") {
			// Continue trying in case it's a branch-specific issue, but this is less likely to succeed
		}

		// Clean up any partial clone directory before retrying
		if _, statErr := os.Stat(destDir); statErr == nil {
			_ = os.RemoveAll(destDir)
		}
	}

	// All attempts failed

	return fmt.Errorf("git clone failed after trying branches %v: %v\nFinal output: %s",
		attemptedBranches, lastErr, lastOutput)
}
