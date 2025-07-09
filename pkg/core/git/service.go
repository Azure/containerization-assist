package git

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// Service provides a unified interface to all Git operations
type Service interface {
	// Repository operations
	Clone(ctx context.Context, url, targetDir string, options CloneOptions) error
	Init(ctx context.Context, dir string) error
	Status(ctx context.Context, dir string) (*StatusResult, error)

	// Branch operations
	CreateBranch(ctx context.Context, dir, branch string) error
	CheckoutBranch(ctx context.Context, dir, branch string) error
	ListBranches(ctx context.Context, dir string) ([]string, error)

	// Commit operations
	Add(ctx context.Context, dir string, files []string) error
	Commit(ctx context.Context, dir, message string) error
	Push(ctx context.Context, dir string, options PushOptions) error
	Pull(ctx context.Context, dir string, options PullOptions) error

	// Information
	GetRemoteURL(ctx context.Context, dir string) (string, error)
	GetCurrentBranch(ctx context.Context, dir string) (string, error)
	IsGitRepo(ctx context.Context, dir string) bool
}

// ServiceImpl implements the Git Service interface
type ServiceImpl struct {
	logger *slog.Logger
}

// NewGitService creates a new Git service
func NewGitService(logger *slog.Logger) Service {
	return &ServiceImpl{
		logger: logger.With("component", "git_service"),
	}
}

// StatusResult contains git status information
type StatusResult struct {
	IsClean        bool
	ModifiedFiles  []string
	StagedFiles    []string
	UntrackedFiles []string
	CurrentBranch  string
}

// PushOptions contains options for git push
type PushOptions struct {
	Remote string
	Branch string
	Force  bool
	Auth   *AuthConfig
}

// PullOptions contains options for git pull
type PullOptions struct {
	Remote string
	Branch string
	Auth   *AuthConfig
}

// AuthConfig contains Git authentication
type AuthConfig struct {
	Username string
	Password string
	Token    string
}

// Clone clones a Git repository
func (s *ServiceImpl) Clone(ctx context.Context, url, targetDir string, options CloneOptions) error {
	s.logger.Info("Cloning Git repository", "url", url, "target", targetDir)

	args := []string{"clone"}

	if options.Depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", options.Depth))
	}

	if options.Branch != "" {
		args = append(args, "--branch", options.Branch)
	}

	if options.Recursive {
		args = append(args, "--recursive")
	}

	args = append(args, url, targetDir)

	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.NewError().
			Message("failed to clone Git repository").
			Cause(err).
			Context("url", url).
			Context("output", string(output)).
			WithLocation().
			Build()
	}

	s.logger.Info("Successfully cloned Git repository", "url", url, "target", targetDir)
	return nil
}

// Init initializes a new Git repository
func (s *ServiceImpl) Init(ctx context.Context, dir string) error {
	s.logger.Info("Initializing Git repository", "dir", dir)

	cmd := exec.CommandContext(ctx, "git", "init", dir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.NewError().
			Message("failed to initialize Git repository").
			Cause(err).
			Context("dir", dir).
			Context("output", string(output)).
			WithLocation().
			Build()
	}

	return nil
}

// Status returns the current status of the Git repository
func (s *ServiceImpl) Status(ctx context.Context, dir string) (*StatusResult, error) {
	if !s.IsGitRepo(ctx, dir) {
		return nil, errors.NewError().
			Messagef("directory is not a git repository: %s", dir).
			WithLocation().
			Build()
	}

	// Get status in porcelain format
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "status", "--porcelain")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, errors.NewError().
			Message("failed to get Git status").
			Cause(err).
			Context("dir", dir).
			WithLocation().
			Build()
	}

	// Get current branch
	branchCmd := exec.CommandContext(ctx, "git", "-C", dir, "branch", "--show-current")
	branchOutput, err := branchCmd.CombinedOutput()
	if err != nil {
		return nil, errors.NewError().
			Message("failed to get current branch").
			Cause(err).
			Context("dir", dir).
			WithLocation().
			Build()
	}

	result := &StatusResult{
		CurrentBranch: strings.TrimSpace(string(branchOutput)),
		IsClean:       len(output) == 0,
	}

	// Parse status output
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}

		status := line[:2]
		file := line[3:]

		switch {
		case status[0] != ' ' && status[0] != '?':
			result.StagedFiles = append(result.StagedFiles, file)
		case status[1] != ' ' && status[1] != '?':
			result.ModifiedFiles = append(result.ModifiedFiles, file)
		case status == "??":
			result.UntrackedFiles = append(result.UntrackedFiles, file)
		}
	}

	return result, nil
}

// CreateBranch creates a new branch
func (s *ServiceImpl) CreateBranch(ctx context.Context, dir, branch string) error {
	s.logger.Info("Creating Git branch", "dir", dir, "branch", branch)

	cmd := exec.CommandContext(ctx, "git", "-C", dir, "checkout", "-b", branch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.NewError().
			Message("failed to create Git branch").
			Cause(err).
			Context("dir", dir).
			Context("branch", branch).
			Context("output", string(output)).
			WithLocation().
			Build()
	}

	return nil
}

// CheckoutBranch checks out an existing branch
func (s *ServiceImpl) CheckoutBranch(ctx context.Context, dir, branch string) error {
	s.logger.Info("Checking out Git branch", "dir", dir, "branch", branch)

	cmd := exec.CommandContext(ctx, "git", "-C", dir, "checkout", branch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.NewError().
			Message("failed to checkout Git branch").
			Cause(err).
			Context("dir", dir).
			Context("branch", branch).
			Context("output", string(output)).
			WithLocation().
			Build()
	}

	return nil
}

// ListBranches lists all branches in the repository
func (s *ServiceImpl) ListBranches(ctx context.Context, dir string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "branch", "--list")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, errors.NewError().
			Message("failed to list Git branches").
			Cause(err).
			Context("dir", dir).
			WithLocation().
			Build()
	}

	var branches []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		branch := strings.TrimSpace(strings.TrimPrefix(line, "*"))
		if branch != "" {
			branches = append(branches, branch)
		}
	}

	return branches, nil
}

// Add stages files for commit
func (s *ServiceImpl) Add(ctx context.Context, dir string, files []string) error {
	s.logger.Info("Staging files for commit", "dir", dir, "files", files)

	args := []string{"-C", dir, "add"}
	args = append(args, files...)

	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.NewError().
			Message("failed to stage files").
			Cause(err).
			Context("dir", dir).
			Context("files", files).
			Context("output", string(output)).
			WithLocation().
			Build()
	}

	return nil
}

// Commit creates a new commit
func (s *ServiceImpl) Commit(ctx context.Context, dir, message string) error {
	s.logger.Info("Creating Git commit", "dir", dir, "message", message)

	cmd := exec.CommandContext(ctx, "git", "-C", dir, "commit", "-m", message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.NewError().
			Message("failed to create commit").
			Cause(err).
			Context("dir", dir).
			Context("message", message).
			Context("output", string(output)).
			WithLocation().
			Build()
	}

	return nil
}

// Push pushes commits to remote repository
func (s *ServiceImpl) Push(ctx context.Context, dir string, options PushOptions) error {
	s.logger.Info("Pushing to remote repository", "dir", dir, "remote", options.Remote, "branch", options.Branch)

	args := []string{"-C", dir, "push"}

	if options.Force {
		args = append(args, "--force")
	}

	if options.Remote != "" {
		args = append(args, options.Remote)
	}

	if options.Branch != "" {
		args = append(args, options.Branch)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.NewError().
			Message("failed to push to remote").
			Cause(err).
			Context("dir", dir).
			Context("output", string(output)).
			WithLocation().
			Build()
	}

	return nil
}

// Pull pulls changes from remote repository
func (s *ServiceImpl) Pull(ctx context.Context, dir string, options PullOptions) error {
	s.logger.Info("Pulling from remote repository", "dir", dir, "remote", options.Remote, "branch", options.Branch)

	args := []string{"-C", dir, "pull"}

	if options.Remote != "" {
		args = append(args, options.Remote)
	}

	if options.Branch != "" {
		args = append(args, options.Branch)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.NewError().
			Message("failed to pull from remote").
			Cause(err).
			Context("dir", dir).
			Context("output", string(output)).
			WithLocation().
			Build()
	}

	return nil
}

// GetRemoteURL returns the URL of the remote repository
func (s *ServiceImpl) GetRemoteURL(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "remote", "get-url", "origin")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.NewError().
			Message("failed to get remote URL").
			Cause(err).
			Context("dir", dir).
			WithLocation().
			Build()
	}

	return strings.TrimSpace(string(output)), nil
}

// GetCurrentBranch returns the current branch name
func (s *ServiceImpl) GetCurrentBranch(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "branch", "--show-current")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.NewError().
			Message("failed to get current branch").
			Cause(err).
			Context("dir", dir).
			WithLocation().
			Build()
	}

	return strings.TrimSpace(string(output)), nil
}

// IsGitRepo checks if the directory is a Git repository
func (s *ServiceImpl) IsGitRepo(ctx context.Context, dir string) bool {
	_, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--git-dir").CombinedOutput()
	return err == nil
}
