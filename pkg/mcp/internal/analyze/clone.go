package analyze

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/core/git"
	"github.com/rs/zerolog"
)

// Cloner handles repository cloning operations
type Cloner struct {
	logger zerolog.Logger
}

// NewCloner creates a new repository cloner
func NewCloner(logger zerolog.Logger) *Cloner {
	return &Cloner{
		logger: logger.With().Str("component", "repository_cloner").Logger(),
	}
}

// Clone clones a repository with the given options
func (c *Cloner) Clone(ctx context.Context, opts CloneOptions) (*CloneResult, error) {
	startTime := time.Now()

	// Validate options
	if err := c.validateCloneOptions(opts); err != nil {
		return nil, fmt.Errorf("invalid clone options: %w", err)
	}

	// Determine if it's a URL or local path
	isURL := c.isURL(opts.RepoURL)

	var result *git.CloneResult
	var err error

	if isURL {
		// Clone from URL
		cloneOpts := git.CloneOptions{
			URL:    opts.RepoURL,
			Branch: opts.Branch,
		}

		if opts.Shallow {
			cloneOpts.Depth = 1 // Shallow clone if requested
		}

		c.logger.Info().
			Str("url", opts.RepoURL).
			Str("branch", opts.Branch).
			Str("target_dir", opts.TargetDir).
			Bool("shallow", opts.Shallow).
			Msg("Cloning repository from URL")

		// Create git manager
		gitManager := git.NewManager(c.logger)

		// Clone to target directory
		result, err = gitManager.CloneRepository(ctx, opts.TargetDir, cloneOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to clone repository: %w", err)
		}
	} else {
		// Handle local path
		if err := c.validateLocalPath(opts.RepoURL); err != nil {
			return nil, fmt.Errorf("invalid local path: %w", err)
		}

		c.logger.Info().
			Str("path", opts.RepoURL).
			Str("target_dir", opts.TargetDir).
			Msg("Using local repository path")

		// Create a mock result for local paths
		result = &git.CloneResult{
			Success:    true,
			RepoPath:   opts.RepoURL,
			Branch:     "local",
			CommitHash: "local",
			RemoteURL:  opts.RepoURL,
			Duration:   time.Since(startTime),
		}
	}

	return &CloneResult{
		CloneResult: result,
		Duration:    time.Since(startTime),
	}, nil
}

// validateCloneOptions validates the clone options
func (c *Cloner) validateCloneOptions(opts CloneOptions) error {
	if opts.RepoURL == "" {
		return fmt.Errorf("repository URL or path is required")
	}

	if opts.TargetDir == "" && c.isURL(opts.RepoURL) {
		return fmt.Errorf("target directory is required for URL cloning")
	}

	// Branch is optional for git.CloneOptions

	return nil
}

// isURL determines if the given path is a URL
func (c *Cloner) isURL(path string) bool {
	return strings.HasPrefix(path, "http://") ||
		strings.HasPrefix(path, "https://") ||
		strings.HasPrefix(path, "git@") ||
		strings.HasPrefix(path, "ssh://") ||
		strings.Contains(path, "github.com") ||
		strings.Contains(path, "gitlab.com")
}

// validateLocalPath validates a local repository path
func (c *Cloner) validateLocalPath(path string) error {
	// Check if path exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("local path does not exist: %s", path)
		}
		return fmt.Errorf("failed to access local path: %w", err)
	}

	// Check if it's a directory
	if !info.IsDir() {
		return fmt.Errorf("local path is not a directory: %s", path)
	}

	// Check if it looks like a git repository or code directory
	gitPath := filepath.Join(path, ".git")
	if _, err := os.Stat(gitPath); err == nil {
		// It's a git repository
		return nil
	}

	// Check if it contains code files
	// This is a simplified check - just ensure the directory is not empty
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	if len(entries) == 0 {
		return fmt.Errorf("directory is empty: %s", path)
	}

	return nil
}
