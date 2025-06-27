package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/internal/types"
)

// PathUtils provides centralized path validation and manipulation utilities
// This consolidates duplicate path validation functions found across the codebase

// ValidateLocalPath validates a local file system path with security checks
// Consolidates validateLocalPath functions from:
// - pkg/mcp/internal/analyze/analyze_repository_atomic.go
// - pkg/mcp/internal/analyze/analyze_simple.go
func ValidateLocalPath(path string) error {
	if path == "" {
		return types.NewRichError("INVALID_ARGUMENTS", "path cannot be empty", "validation_error")
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return types.NewRichError("INVALID_PATH", fmt.Sprintf("failed to resolve absolute path for '%s': %v", path, err), types.ErrTypeValidation)
	}

	// Check for path traversal attacks
	if strings.Contains(absPath, "..") {
		return types.NewRichError("PATH_TRAVERSAL_DENIED", fmt.Sprintf("path traversal not allowed for '%s' (resolved to: %s)", path, absPath), types.ErrTypeSecurity)
	}

	// Check if path exists
	if _, err := os.Stat(absPath); err != nil {
		if os.IsNotExist(err) {
			return types.NewRichError("INVALID_ARGUMENTS", "path does not exist: "+absPath, "validation_error")
		}
		return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to stat path '%s': %v", absPath, err), "filesystem_error")
	}

	return nil
}

// ValidateLocalPathExists checks if a path exists without security validation
func ValidateLocalPathExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// SanitizePath cleans and normalizes a file path
func SanitizePath(path string) string {
	if path == "" {
		return ""
	}

	// Clean the path (removes redundant separators, resolves . and ..)
	cleaned := filepath.Clean(path)

	// Convert to forward slashes for consistency (works on Windows too)
	cleaned = filepath.ToSlash(cleaned)

	return cleaned
}

// IsURL checks if a string represents a URL
// Consolidates URL detection logic from multiple files
func IsURL(path string) bool {
	return strings.HasPrefix(path, "http://") ||
		strings.HasPrefix(path, "https://") ||
		strings.HasPrefix(path, "git@") ||
		strings.HasPrefix(path, "ssh://") ||
		strings.HasPrefix(path, "ftp://") ||
		strings.HasPrefix(path, "ftps://")
}

// IsAbsolutePath checks if a path is absolute
func IsAbsolutePath(path string) bool {
	return filepath.IsAbs(path)
}

// EnsureDirectoryExists creates a directory if it doesn't exist
func EnsureDirectoryExists(dirPath string) error {
	if dirPath == "" {
		return types.NewRichError("INVALID_ARGUMENTS", "directory path cannot be empty", "validation_error")
	}

	// Check if directory already exists
	if info, err := os.Stat(dirPath); err == nil {
		if !info.IsDir() {
			return types.NewRichError("INVALID_ARGUMENTS", fmt.Sprintf("path exists but is not a directory: %s", dirPath), "validation_error")
		}
		return nil // Directory already exists
	}

	// Create directory with proper permissions
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to create directory '%s': %v", dirPath, err), "filesystem_error")
	}

	return nil
}

// GetFileExtension returns the file extension (with dot)
func GetFileExtension(filename string) string {
	return filepath.Ext(filename)
}

// GetBaseName returns the base name of a file without extension
func GetBaseName(filename string) string {
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

// JoinPaths safely joins path components
func JoinPaths(paths ...string) string {
	return filepath.Join(paths...)
}

// RelativePath returns the relative path from base to target
func RelativePath(base, target string) (string, error) {
	return filepath.Rel(base, target)
}

// IsSubdirectory checks if childPath is a subdirectory of parentPath
func IsSubdirectory(parentPath, childPath string) (bool, error) {
	parent, err := filepath.Abs(parentPath)
	if err != nil {
		return false, err
	}

	child, err := filepath.Abs(childPath)
	if err != nil {
		return false, err
	}

	// Ensure paths end with separator for accurate comparison
	if !strings.HasSuffix(parent, string(filepath.Separator)) {
		parent += string(filepath.Separator)
	}

	return strings.HasPrefix(child+string(filepath.Separator), parent), nil
}

// ListFiles returns all files in a directory (non-recursive)
func ListFiles(dirPath string) ([]string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to read directory '%s': %v", dirPath, err), "filesystem_error")
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	return files, nil
}

// ListDirectories returns all subdirectories in a directory (non-recursive)
func ListDirectories(dirPath string) ([]string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to read directory '%s': %v", dirPath, err), "filesystem_error")
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}

	return dirs, nil
}

// GetFileSize returns the size of a file in bytes
func GetFileSize(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, types.NewRichError("INTERNAL_SERVER_ERROR", fmt.Sprintf("failed to stat file '%s': %v", filePath, err), "filesystem_error")
	}

	return info.Size(), nil
}

// IsRegularFile checks if the path points to a regular file
func IsRegularFile(filePath string) bool {
	info, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

// IsDirectory checks if the path points to a directory
func IsDirectory(dirPath string) bool {
	info, err := os.Stat(dirPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}
