package analyze

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"log/slog"

	"github.com/Azure/container-kit/pkg/core/git"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// Cloner handles repository cloning operations
type Cloner struct {
	logger *slog.Logger
}

// NewCloner creates a new repository cloner
func NewCloner(logger *slog.Logger) *Cloner {
	return &Cloner{
		logger: logger.With("component", "repository_cloner"),
	}
}

// Clone clones a repository with the given options
func (c *Cloner) Clone(ctx context.Context, opts CloneOptions) (*CloneResult, error) {
	startTime := time.Now()

	if err := c.validateCloneOptions(opts); err != nil {
		return nil, errors.NewError().Message("invalid clone options").Cause(err).WithLocation().Build()
	}

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
			cloneOpts.Depth = 1
		}

		c.logger.Info("Cloning repository from URL",
			"url", opts.RepoURL,
			"branch", opts.Branch,
			"target_dir", opts.TargetDir,
			"shallow", opts.Shallow)

		gitManager := git.NewManager(c.logger)

		result, err = gitManager.CloneRepository(ctx, opts.TargetDir, cloneOpts)
		if err != nil {
			return nil, errors.NewError().Message("failed to clone repository").Cause(err).Build()
		}
	} else {

		if err := c.validateLocalPath(opts.RepoURL); err != nil {
			return nil, errors.NewError().Message("invalid local path").Cause(err).Build()
		}

		c.logger.Info("Using local repository path",
			"path", opts.RepoURL,
			"target_dir", opts.TargetDir)

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
		return errors.NewError().Messagef("repository URL or path is required").WithLocation().Build()
	}

	if opts.TargetDir == "" && c.isURL(opts.RepoURL) {
		return errors.NewError().Messagef("target directory is required for URL cloning").WithLocation().Build()
	}

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
			return errors.NewError().Messagef("local path does not exist: %s", path).WithLocation().Build()
		}
		return errors.NewError().Message("failed to access local path").Cause(err).WithLocation().Build()
	}

	if !info.IsDir() {
		return errors.NewError().Messagef("local path is not a directory: %s", path).WithLocation().Build()
	}

	gitPath := filepath.Join(path, ".git")
	if _, err := os.Stat(gitPath); err == nil {
		return nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return errors.NewError().Message("failed to read directory").Cause(err).WithLocation().Build()
	}

	if len(entries) == 0 {
		return errors.NewError().Messagef("directory is empty: %s", path).WithLocation().Build()
	}

	return nil
}
