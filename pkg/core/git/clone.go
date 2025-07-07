// Package git provides core Git operations extracted from the Container Kit pipeline.
// This package contains mechanical Git operations without AI dependencies,
// designed to be used by atomic MCP tools.
package git

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	mcperrors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// Manager provides Git operations for repository management
type Manager struct {
	logger *slog.Logger
	jail   *FilesystemJail
}

// NewManager creates a new Git manager
func NewManager(logger *slog.Logger) *Manager {
	return &Manager{
		logger: logger.With("component", "git_manager"),
	}
}

// NewSecureManager creates a new Git manager with filesystem jail
func NewSecureManager(logger *slog.Logger, securityOpts *SecurityOptions) (*Manager, error) {
	jail, err := NewFilesystemJail(securityOpts)
	if err != nil {
		return nil, mcperrors.NewError().Messagef("failed to create filesystem jail: %w", err).WithLocation().Build()
	}

	return &Manager{
		logger: logger.With("component", "git_manager"),
		jail:   jail,
	}, nil
}

// CloneOptions contains options for cloning repositories
type CloneOptions struct {
	URL          string
	Branch       string
	Depth        int
	SingleBranch bool
	Recursive    bool
	Timeout      time.Duration
	AuthToken    string
	AuthUsername string
	AuthPassword string
}

// CloneResult contains the result of a clone operation
type CloneResult struct {
	Success    bool                   `json:"success"`
	RepoPath   string                 `json:"repo_path"`
	Branch     string                 `json:"branch"`
	CommitHash string                 `json:"commit_hash"`
	RemoteURL  string                 `json:"remote_url"`
	Duration   time.Duration          `json:"duration"`
	Context    map[string]interface{} `json:"context"`
	Error      *GitError              `json:"error,omitempty"`
}

// GitError provides detailed Git error information
type GitError struct {
	Type    string                 `json:"type"` // "auth_error", "network_error", "invalid_repo", "clone_error"
	Message string                 `json:"message"`
	RepoURL string                 `json:"repo_url"`
	Output  string                 `json:"output"`
	Context map[string]interface{} `json:"context"`
}

// CloneRepository clones a Git repository to the specified directory
func (gm *Manager) CloneRepository(ctx context.Context, targetDir string, options CloneOptions) (*CloneResult, error) {
	startTime := time.Now()

	result := &CloneResult{
		RemoteURL: options.URL,
		Context:   make(map[string]interface{}),
	}

	gm.logger.Info("Starting Git clone",
		"repo_url", options.URL,
		"target_dir", targetDir,
		"branch", options.Branch)

	// Validate inputs
	if err := gm.validateCloneInputs(targetDir, options); err != nil {
		result.Error = &GitError{
			Type:    "validation_error",
			Message: err.Error(),
			RepoURL: options.URL,
			Context: map[string]interface{}{
				"target_dir": targetDir,
				"options":    options,
			},
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Apply filesystem jail if configured
	if gm.jail != nil {
		// Validate repository URL
		if err := gm.jail.ValidateURL(options.URL); err != nil {
			result.Error = &GitError{
				Type:    "security_error",
				Message: fmt.Sprintf("Repository URL failed security validation: %v", err),
				RepoURL: options.URL,
				Context: map[string]interface{}{
					"security_check": "url_validation",
				},
			}
			result.Duration = time.Since(startTime)
			return result, nil
		}

		// Secure the target directory path
		securePath, err := gm.jail.SecureTargetPath(targetDir)
		if err != nil {
			result.Error = &GitError{
				Type:    "security_error",
				Message: fmt.Sprintf("Target directory failed security validation: %v", err),
				RepoURL: options.URL,
				Context: map[string]interface{}{
					"target_dir":     targetDir,
					"security_check": "path_validation",
				},
			}
			result.Duration = time.Since(startTime)
			return result, nil
		}
		targetDir = securePath
		gm.logger.Debug("Using secured target path", "secure_path", targetDir)
	}

	// Check Git installation
	if err := gm.checkGitInstalled(); err != nil {
		result.Error = &GitError{
			Type:    "git_not_available",
			Message: err.Error(),
			RepoURL: options.URL,
			Context: map[string]interface{}{
				"suggestion": "Install Git or ensure it's in PATH",
			},
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Prepare target directory
	if err := gm.prepareTargetDirectory(targetDir); err != nil {
		result.Error = &GitError{
			Type:    "filesystem_error",
			Message: fmt.Sprintf("Failed to prepare target directory: %v", err),
			RepoURL: options.URL,
			Context: map[string]interface{}{
				"target_dir": targetDir,
			},
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Set timeout context if specified
	cloneCtx := ctx
	if options.Timeout > 0 {
		var cancel context.CancelFunc
		cloneCtx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

	// Perform the clone
	output, err := gm.executeClone(cloneCtx, targetDir, options)
	if err != nil {
		gm.logger.Error("Git clone failed", "error", err, "output", output)

		result.Error = &GitError{
			Type:    gm.categorizeError(err, output),
			Message: fmt.Sprintf("Git clone failed: %v", err),
			RepoURL: options.URL,
			Output:  output,
			Context: map[string]interface{}{
				"target_dir": targetDir,
				"options":    options,
			},
		}
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Get repository information
	repoInfo, err := gm.getRepositoryInfo(targetDir)
	if err != nil {
		gm.logger.Warn("Failed to get repository info after clone", "error", err)
		// Don't fail the clone for this
	} else {
		result.Branch = repoInfo.Branch
		result.CommitHash = repoInfo.CommitHash
	}

	// Clone succeeded
	result.Success = true
	result.RepoPath = targetDir
	result.Duration = time.Since(startTime)
	result.Context = map[string]interface{}{
		"clone_time":    result.Duration.Seconds(),
		"branch":        result.Branch,
		"commit_hash":   result.CommitHash,
		"depth":         options.Depth,
		"single_branch": options.SingleBranch,
	}

	gm.logger.Info("Git clone completed successfully",
		"repo_url", options.URL,
		"repo_path", result.RepoPath,
		"branch", result.Branch,
		"duration", result.Duration)

	return result, nil
}

// RepositoryInfo contains information about a Git repository
type RepositoryInfo struct {
	Branch     string `json:"branch"`
	CommitHash string `json:"commit_hash"`
	RemoteURL  string `json:"remote_url"`
	IsClean    bool   `json:"is_clean"`
}

// GetRepositoryInfo returns information about a Git repository
func (gm *Manager) GetRepositoryInfo(repoPath string) (*RepositoryInfo, error) {
	return gm.getRepositoryInfo(repoPath)
}

// CheckGitInstallation verifies Git is installed and accessible
func (gm *Manager) CheckGitInstallation() error {
	return gm.checkGitInstalled()
}

// IsGitRepository checks if a directory is a Git repository
func (gm *Manager) IsGitRepository(path string) bool {
	gitDir := filepath.Join(path, ".git")
	if stat, err := os.Stat(gitDir); err == nil {
		return stat.IsDir()
	}
	return false
}

// Helper methods

func (gm *Manager) validateCloneInputs(targetDir string, options CloneOptions) error {
	if options.URL == "" {
		return fmt.Errorf("repository URL is required")
	}

	if targetDir == "" {
		return fmt.Errorf("target directory is required")
	}

	// Basic URL validation
	if !strings.Contains(options.URL, "://") && !strings.Contains(options.URL, "@") {
		return fmt.Errorf("repository URL appears to be invalid: %s", options.URL)
	}

	return nil
}

func (gm *Manager) checkGitInstalled() error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git executable not found in PATH. Please install Git")
	}

	// Check Git version
	cmd := exec.Command("git", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git is not functioning properly: %v", err)
	}

	return nil
}

func (gm *Manager) prepareTargetDirectory(targetDir string) error {
	// Validate path with jail if configured
	if gm.jail != nil {
		if err := gm.jail.ValidatePath(targetDir); err != nil {
			return fmt.Errorf("target directory failed security validation: %w", err)
		}
	}

	// Check if directory exists
	if stat, err := os.Stat(targetDir); err == nil {
		if !stat.IsDir() {
			return fmt.Errorf("target path exists but is not a directory: %s", targetDir)
		}

		// Check if directory is empty
		entries, err := os.ReadDir(targetDir)
		if err != nil {
			return fmt.Errorf("cannot read target directory: %v", err)
		}

		if len(entries) > 0 {
			return fmt.Errorf("target directory is not empty: %s", targetDir)
		}
	} else if os.IsNotExist(err) {
		// Create the directory
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return fmt.Errorf("failed to create target directory: %v", err)
		}
	} else {
		return fmt.Errorf("cannot access target directory: %v", err)
	}

	return nil
}

func (gm *Manager) executeClone(ctx context.Context, targetDir string, options CloneOptions) (string, error) {
	args := []string{"clone"}

	// Apply security restrictions if jail is configured
	if gm.jail != nil {
		restrictedArgs, err := gm.jail.RestrictGitCommand(args)
		if err != nil {
			return "", fmt.Errorf("failed to apply security restrictions: %w", err)
		}
		args = restrictedArgs
	}

	// Add clone options
	if options.Depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", options.Depth))
	}

	if options.SingleBranch {
		args = append(args, "--single-branch")
	}

	if options.Recursive {
		args = append(args, "--recursive")
	}

	if options.Branch != "" {
		args = append(args, "--branch", options.Branch)
	}

	// Add URL and target directory
	args = append(args, options.URL, targetDir)

	// Create command
	cmd := exec.CommandContext(ctx, "git", args...)

	// Set up authentication if provided
	if options.AuthToken != "" {
		// For token-based auth, modify the URL
		authURL := gm.injectTokenIntoURL(options.URL, options.AuthToken)
		args[len(args)-2] = authURL // Replace URL
		cmd = exec.CommandContext(ctx, "git", args...)
	} else if options.AuthUsername != "" && options.AuthPassword != "" {
		// For username/password auth
		authURL := gm.injectCredentialsIntoURL(options.URL, options.AuthUsername, options.AuthPassword)
		args[len(args)-2] = authURL // Replace URL
		cmd = exec.CommandContext(ctx, "git", args...)
	}

	// Execute command and capture output
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (gm *Manager) getRepositoryInfo(repoPath string) (*RepositoryInfo, error) {
	info := &RepositoryInfo{}

	// Get current branch
	cmd := exec.Command("git", "-C", repoPath, "branch", "--show-current")
	if output, err := cmd.Output(); err == nil {
		info.Branch = strings.TrimSpace(string(output))
	}

	// Get commit hash
	cmd = exec.Command("git", "-C", repoPath, "rev-parse", "HEAD")
	if output, err := cmd.Output(); err == nil {
		info.CommitHash = strings.TrimSpace(string(output))
	}

	// Get remote URL
	cmd = exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")
	if output, err := cmd.Output(); err == nil {
		info.RemoteURL = strings.TrimSpace(string(output))
	}

	// Check if working directory is clean
	cmd = exec.Command("git", "-C", repoPath, "status", "--porcelain")
	if output, err := cmd.Output(); err == nil {
		info.IsClean = strings.TrimSpace(string(output)) == ""
	}

	return info, nil
}

func (gm *Manager) categorizeError(err error, output string) string {
	errStr := strings.ToLower(err.Error())
	outputStr := strings.ToLower(output)

	// Authentication errors
	if strings.Contains(errStr, "authentication") || strings.Contains(outputStr, "authentication") ||
		strings.Contains(errStr, "permission denied") || strings.Contains(outputStr, "permission denied") ||
		strings.Contains(errStr, "unauthorized") || strings.Contains(outputStr, "unauthorized") {
		return "auth_error"
	}

	// Network errors
	if strings.Contains(errStr, "network") || strings.Contains(outputStr, "network") ||
		strings.Contains(errStr, "timeout") || strings.Contains(outputStr, "timeout") ||
		strings.Contains(errStr, "connection") || strings.Contains(outputStr, "connection") {
		return "network_error"
	}

	// Invalid repository
	if strings.Contains(errStr, "not found") || strings.Contains(outputStr, "not found") ||
		strings.Contains(errStr, "does not exist") || strings.Contains(outputStr, "does not exist") ||
		strings.Contains(errStr, "repository") && strings.Contains(outputStr, "not found") {
		return "invalid_repo"
	}

	// Default to generic clone error
	return "clone_error"
}

func (gm *Manager) injectTokenIntoURL(url, token string) string {
	// Simple token injection for HTTPS URLs
	if strings.HasPrefix(url, "https://") {
		return strings.Replace(url, "https://", fmt.Sprintf("https://%s@", token), 1)
	}
	return url
}

func (gm *Manager) injectCredentialsIntoURL(url, username, password string) string {
	// Simple credential injection for HTTPS URLs
	if strings.HasPrefix(url, "https://") {
		return strings.Replace(url, "https://", fmt.Sprintf("https://%s:%s@", username, password), 1)
	}
	return url
}
