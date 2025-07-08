// Package git provides core Git operations extracted from the Container Kit pipeline.
package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mcperrors "github.com/Azure/container-kit/pkg/mcp/errors"
)

// SecurityOptions contains security settings for Git operations
type SecurityOptions struct {
	// WorkspaceRoot is the root directory that all operations must stay within
	WorkspaceRoot string
	// BlockedPaths are paths that should never be accessed (e.g., /etc, /root)
	BlockedPaths []string
	// AllowSymlinks controls whether symbolic links are allowed
	AllowSymlinks bool
}

// DefaultSecurityOptions returns secure default options
func DefaultSecurityOptions() *SecurityOptions {
	return &SecurityOptions{
		BlockedPaths: []string{
			"/..",
			"...",
		},
		AllowSymlinks: false,
	}
}

// RestrictedPathPrefixes returns paths that should be blocked when outside workspace
var RestrictedPathPrefixes = []string{
	"/etc/",
	"/root/",
	"/var/log/",
	"/usr/bin/",
	"/usr/sbin/",
	"/bin/",
	"/sbin/",
	"/lib/",
	"/lib64/",
	"/proc/",
	"/sys/",
	"/dev/",
}

// FilesystemJail provides filesystem security enforcement
type FilesystemJail struct {
	workspaceRoot string
	blockedPaths  []string
	allowSymlinks bool
}

// NewFilesystemJail creates a new filesystem jail
func NewFilesystemJail(opts *SecurityOptions) (*FilesystemJail, error) {
	if opts.WorkspaceRoot == "" {
		return nil, mcperrors.NewError().Messagef("workspace root is required for filesystem jail").WithLocation(

		// Resolve workspace root to absolute path
		).Build()
	}

	absRoot, err := filepath.Abs(opts.WorkspaceRoot)
	if err != nil {
		return nil, mcperrors.NewError().Messagef("failed to resolve workspace root: %w", err).WithLocation(

		// Ensure workspace root exists
		).Build()
	}

	if stat, err := os.Stat(absRoot); err != nil {
		return nil, mcperrors.NewError().Messagef("workspace root does not exist: %w", err).WithLocation().Build()
	} else if !stat.IsDir() {
		return nil, mcperrors.NewError().Messagef("workspace root is not a directory: %s", absRoot).WithLocation().Build()
	}

	return &FilesystemJail{
		workspaceRoot: absRoot,
		blockedPaths:  opts.BlockedPaths,
		allowSymlinks: opts.AllowSymlinks,
	}, nil
}

// ValidatePath checks if a path is allowed within the jail
func (j *FilesystemJail) ValidatePath(path string) error {
	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return mcperrors.NewError().Messagef("failed to resolve path: %w", err).WithLocation(

		// Check if path contains any blocked patterns
		).Build()
	}

	for _, blocked := range j.blockedPaths {
		if strings.Contains(absPath, blocked) {
			return mcperrors.NewError().Messagef("path contains blocked pattern '%s': %s", blocked, absPath).WithLocation(

			// Check if path is within workspace root
			).Build()
		}
	}

	if !strings.HasPrefix(absPath, j.workspaceRoot) {
		// If path is outside workspace, check against restricted prefixes
		for _, restricted := range RestrictedPathPrefixes {
			if strings.HasPrefix(absPath, restricted) {
				return mcperrors.NewError().Messagef("path is in restricted location '%s': %s", restricted, absPath).WithLocation().Build()
			}
		}
		return mcperrors.NewError().Messagef("path is outside workspace root: %s", absPath).WithLocation(

		// Check for path traversal attempts
		).Build()
	}

	cleanPath := filepath.Clean(absPath)
	if cleanPath != absPath {
		// Additional check for sneaky traversal attempts
		rel, err := filepath.Rel(j.workspaceRoot, cleanPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			return mcperrors.NewError().Messagef("path traversal detected: %s", path).WithLocation(

			// Check symlinks if not allowed
			).Build()
		}
	}

	if !j.allowSymlinks {
		if err := j.checkSymlinks(absPath); err != nil {
			return err
		}
	}

	return nil
}

// ValidateURL performs basic security checks on repository URLs
func (j *FilesystemJail) ValidateURL(url string) error {
	// Block file:// URLs that could access local filesystem
	if strings.HasPrefix(strings.ToLower(url), "file://") {
		return mcperrors.NewError().Messagef("file:// URLs are not allowed for security reasons").WithLocation(

		// Block URLs with suspicious patterns
		).Build()
	}

	suspiciousPatterns := []string{
		"..",
		"~",
		"${",
		"$(",
		"`",
		"|",
		";",
		"&",
		">",
		"<",
	}

	for _, pattern := range suspiciousPatterns {
		if strings.Contains(url, pattern) {
			return mcperrors.NewError().Messagef("URL contains suspicious pattern '%s'", pattern).WithLocation().Build()
		}
	}

	return nil
}

// SecureTargetPath ensures a target path is safe and within the jail
func (j *FilesystemJail) SecureTargetPath(targetDir string) (string, error) {
	// Ensure target is within workspace
	if !filepath.IsAbs(targetDir) {
		targetDir = filepath.Join(j.workspaceRoot, targetDir)
	}

	// Validate the path
	if err := j.ValidatePath(targetDir); err != nil {
		return "", err
	}

	// Clean and return the path
	return filepath.Clean(targetDir), nil
}

// checkSymlinks verifies no symlinks exist in the path
func (j *FilesystemJail) checkSymlinks(path string) error {
	// Walk up the path checking each component
	currentPath := path
	for currentPath != "/" && currentPath != j.workspaceRoot {
		info, err := os.Lstat(currentPath)
		if err != nil {
			if os.IsNotExist(err) {
				// Path doesn't exist yet, which is fine
				currentPath = filepath.Dir(currentPath)
				continue
			}
			return fmt.Errorf("failed to check path component: %w", err)
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic links are not allowed: %s", currentPath)
		}

		currentPath = filepath.Dir(currentPath)
	}

	return nil
}

// RestrictGitCommand adds security restrictions to git command arguments
func (j *FilesystemJail) RestrictGitCommand(args []string) ([]string, error) {
	restricted := make([]string, 0, len(args)+4)

	// Add Git configuration to prevent executing hooks or accessing sensitive files
	restricted = append(restricted,
		"-c", "core.hooksPath=/dev/null", // Disable Git hooks
		"-c", "protocol.file.allow=never", // Disable file:// protocol
	)

	// Add original arguments
	restricted = append(restricted, args...)

	// Validate any paths in arguments
	for i, arg := range args {
		// Check for suspicious patterns in all arguments
		if strings.Contains(arg, "..") || strings.Contains(arg, "~") {
			return nil, mcperrors.NewError().Messagef("suspicious pattern in argument[%d]: %s", i, arg).WithLocation().Build()
		}
	}

	return restricted, nil
}

// GetWorkspaceRoot returns the workspace root directory
func (j *FilesystemJail) GetWorkspaceRoot() string {
	return j.workspaceRoot
}
