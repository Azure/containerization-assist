// Package git provides core Git operations extracted from the Container Kit pipeline.
package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
		return nil, fmt.Errorf("workspace root is required for filesystem jail")
	}

	// Resolve workspace root to absolute path
	absRoot, err := filepath.Abs(opts.WorkspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve workspace root: %w", err)
	}

	// Ensure workspace root exists
	if stat, err := os.Stat(absRoot); err != nil {
		return nil, fmt.Errorf("workspace root does not exist: %w", err)
	} else if !stat.IsDir() {
		return nil, fmt.Errorf("workspace root is not a directory: %s", absRoot)
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
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if path contains any blocked patterns
	for _, blocked := range j.blockedPaths {
		if strings.Contains(absPath, blocked) {
			return fmt.Errorf("path contains blocked pattern '%s': %s", blocked, absPath)
		}
	}

	// Check if path is within workspace root
	if !strings.HasPrefix(absPath, j.workspaceRoot) {
		// If path is outside workspace, check against restricted prefixes
		for _, restricted := range RestrictedPathPrefixes {
			if strings.HasPrefix(absPath, restricted) {
				return fmt.Errorf("path is in restricted location '%s': %s", restricted, absPath)
			}
		}
		return fmt.Errorf("path is outside workspace root: %s", absPath)
	}

	// Check for path traversal attempts
	cleanPath := filepath.Clean(absPath)
	if cleanPath != absPath {
		// Additional check for sneaky traversal attempts
		rel, err := filepath.Rel(j.workspaceRoot, cleanPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("path traversal detected: %s", path)
		}
	}

	// Check symlinks if not allowed
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
		return fmt.Errorf("file:// URLs are not allowed for security reasons")
	}

	// Block URLs with suspicious patterns
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
			return fmt.Errorf("URL contains suspicious pattern '%s'", pattern)
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
			return nil, fmt.Errorf("suspicious pattern in argument[%d]: %s", i, arg)
		}
	}

	return restricted, nil
}

// GetWorkspaceRoot returns the workspace root directory
func (j *FilesystemJail) GetWorkspaceRoot() string {
	return j.workspaceRoot
}
