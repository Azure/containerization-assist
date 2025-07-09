package common

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// PathUtils provides comprehensive path and file system utilities
type PathUtils struct{}

// NewPathUtils creates a new path utilities instance
func NewPathUtils() *PathUtils {
	return &PathUtils{}
}

// ValidateLocalPath validates a local file system path with security checks
func (pu *PathUtils) ValidateLocalPath(path string) error {
	if path == "" {
		return errors.MissingParameterError("path")
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return errors.NewError().
			Code(errors.CodeInvalidParameter).
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Messagef("Failed to resolve absolute path for '%s'", path).
			Context("original_path", path).
			Cause(err).
			Suggestion("Provide a valid file system path").
			WithLocation().
			Build()
	}

	// Check for path traversal attacks
	if strings.Contains(absPath, "..") {
		return errors.NewError().
			Code(errors.CodeInvalidParameter).
			Type(errors.ErrTypeSecurity).
			Severity(errors.SeverityHigh).
			Messagef("Path traversal not allowed for '%s' (resolved to: %s)", path, absPath).
			Context("original_path", path).
			Context("resolved_path", absPath).
			Suggestion("Use a valid file path without '..' components").
			WithLocation().
			Build()
	}

	// Check if path exists
	if _, err := os.Stat(absPath); err != nil {
		if os.IsNotExist(err) {
			return errors.NewError().
				Code(errors.CodeResourceNotFound).
				Type(errors.ErrTypeResource).
				Severity(errors.SeverityMedium).
				Messagef("File not found: %s", absPath).
				Context("original_path", path).
				Context("resolved_path", absPath).
				Suggestion("Ensure the file exists at the specified path").
				WithLocation().
				Build()
		}
		return errors.NewError().
			Code(errors.CodeIOError).
			Type(errors.ErrTypeIO).
			Severity(errors.SeverityHigh).
			Messagef("Failed to stat path '%s'", absPath).
			Context("original_path", path).
			Context("resolved_path", absPath).
			Cause(err).
			Suggestion("Check path permissions and accessibility").
			WithLocation().
			Build()
	}

	return nil
}

// ValidateLocalPathExists checks if a path exists without security validation
func (pu *PathUtils) ValidateLocalPathExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// SanitizePath cleans and normalizes a file path
func (pu *PathUtils) SanitizePath(path string) string {
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
func (pu *PathUtils) IsURL(path string) bool {
	return strings.HasPrefix(path, "http://") ||
		strings.HasPrefix(path, "https://") ||
		strings.HasPrefix(path, "git@") ||
		strings.HasPrefix(path, "ssh://") ||
		strings.HasPrefix(path, "ftp://") ||
		strings.HasPrefix(path, "ftps://")
}

// IsAbsolutePath checks if a path is absolute
func (pu *PathUtils) IsAbsolutePath(path string) bool {
	return filepath.IsAbs(path)
}

// EnsureDirectoryExists creates a directory if it doesn't exist
func (pu *PathUtils) EnsureDirectoryExists(dirPath string) error {
	if dirPath == "" {
		return errors.MissingParameterError("dirPath")
	}

	// Check if directory already exists
	if info, err := os.Stat(dirPath); err == nil {
		if !info.IsDir() {
			return errors.NewError().
				Code(errors.CodeInvalidParameter).
				Type(errors.ErrTypeValidation).
				Severity(errors.SeverityMedium).
				Messagef("Path exists but is not a directory: %s", dirPath).
				Context("path", dirPath).
				Context("is_file", true).
				Suggestion("Use a different path or remove the existing file").
				WithLocation().
				Build()
		}
		return nil // Directory already exists
	}

	// Create directory with proper permissions
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return errors.NewError().
			Code(errors.CodeIOError).
			Type(errors.ErrTypeIO).
			Severity(errors.SeverityHigh).
			Messagef("Failed to create directory '%s'", dirPath).
			Context("path", dirPath).
			Cause(err).
			Suggestion("Check parent directory permissions and disk space").
			WithLocation().
			Build()
	}

	return nil
}

// GetFileExtension returns the file extension (with dot)
func (pu *PathUtils) GetFileExtension(filename string) string {
	return filepath.Ext(filename)
}

// GetBaseName returns the base name of a file without extension
func (pu *PathUtils) GetBaseName(filename string) string {
	base := filepath.Base(filename)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

// JoinPaths safely joins path components
func (pu *PathUtils) JoinPaths(paths ...string) string {
	return filepath.Join(paths...)
}

// RelativePath returns the relative path from base to target
func (pu *PathUtils) RelativePath(base, target string) (string, error) {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return "", errors.NewError().
			Code(errors.CodeInvalidParameter).
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Message("Failed to calculate relative path").
			Context("base", base).
			Context("target", target).
			Cause(err).
			Suggestion("Ensure both paths are valid").
			WithLocation().
			Build()
	}
	return rel, nil
}

// IsSubdirectory checks if childPath is a subdirectory of parentPath
func (pu *PathUtils) IsSubdirectory(parentPath, childPath string) (bool, error) {
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
func (pu *PathUtils) ListFiles(dirPath string) ([]string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeIOError).
			Type(errors.ErrTypeIO).
			Severity(errors.SeverityMedium).
			Messagef("Failed to read directory '%s'", dirPath).
			Context("directory", dirPath).
			Cause(err).
			Suggestion("Check directory permissions and existence").
			WithLocation().
			Build()
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
func (pu *PathUtils) ListDirectories(dirPath string) ([]string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeIOError).
			Type(errors.ErrTypeIO).
			Severity(errors.SeverityMedium).
			Messagef("Failed to read directory '%s'", dirPath).
			Context("directory", dirPath).
			Cause(err).
			Suggestion("Check directory permissions and existence").
			WithLocation().
			Build()
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
func (pu *PathUtils) GetFileSize(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, errors.NewError().
			Code(errors.CodeIOError).
			Type(errors.ErrTypeIO).
			Severity(errors.SeverityMedium).
			Messagef("Failed to stat file '%s'", filePath).
			Context("file", filePath).
			Cause(err).
			Suggestion("Check file permissions and existence").
			WithLocation().
			Build()
	}

	return info.Size(), nil
}

// IsRegularFile checks if the path points to a regular file
func (pu *PathUtils) IsRegularFile(filePath string) bool {
	info, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

// IsDirectory checks if the path points to a directory
func (pu *PathUtils) IsDirectory(dirPath string) bool {
	info, err := os.Stat(dirPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// GetDirectorySize calculates the total size of a directory recursively
func (pu *PathUtils) GetDirectorySize(dirPath string) (int64, error) {
	var size int64

	err := filepath.Walk(dirPath, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	if err != nil {
		return 0, errors.NewError().
			Code(errors.CodeIOError).
			Type(errors.ErrTypeIO).
			Severity(errors.SeverityMedium).
			Messagef("Failed to calculate directory size for '%s'", dirPath).
			Context("directory", dirPath).
			Cause(err).
			Suggestion("Check directory permissions and accessibility").
			WithLocation().
			Build()
	}

	return size, nil
}

// CopyFile copies a file from source to destination
func (pu *PathUtils) CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return errors.NewError().
			Code(errors.CodeIOError).
			Type(errors.ErrTypeIO).
			Severity(errors.SeverityMedium).
			Messagef("Failed to open source file '%s'", src).
			Context("source", src).
			Cause(err).
			Suggestion("Check source file permissions and existence").
			WithLocation().
			Build()
	}
	defer sourceFile.Close()

	// Ensure destination directory exists
	destDir := filepath.Dir(dst)
	if err := pu.EnsureDirectoryExists(destDir); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return errors.NewError().
			Code(errors.CodeIOError).
			Type(errors.ErrTypeIO).
			Severity(errors.SeverityMedium).
			Messagef("Failed to create destination file '%s'", dst).
			Context("destination", dst).
			Cause(err).
			Suggestion("Check destination directory permissions").
			WithLocation().
			Build()
	}
	defer destFile.Close()

	// Copy file contents
	buf := make([]byte, 32*1024) // 32KB buffer
	for {
		n, err := sourceFile.Read(buf)
		if err != nil && err.Error() != "EOF" {
			return errors.NewError().
				Code(errors.CodeIOError).
				Type(errors.ErrTypeIO).
				Severity(errors.SeverityMedium).
				Message("Failed to read from source file").
				Context("source", src).
				Cause(err).
				WithLocation().
				Build()
		}
		if n == 0 {
			break
		}

		if _, err := destFile.Write(buf[:n]); err != nil {
			return errors.NewError().
				Code(errors.CodeIOError).
				Type(errors.ErrTypeIO).
				Severity(errors.SeverityMedium).
				Message("Failed to write to destination file").
				Context("destination", dst).
				Cause(err).
				WithLocation().
				Build()
		}
	}

	return nil
}

// MoveFile moves a file from source to destination
func (pu *PathUtils) MoveFile(src, dst string) error {
	// Try rename first (works if on same filesystem)
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// Fall back to copy and delete
	if err := pu.CopyFile(src, dst); err != nil {
		return err
	}

	// Remove source file
	if err := os.Remove(src); err != nil {
		return errors.NewError().
			Code(errors.CodeIOError).
			Type(errors.ErrTypeIO).
			Severity(errors.SeverityMedium).
			Messagef("Failed to remove source file '%s' after copy", src).
			Context("source", src).
			Context("destination", dst).
			Cause(err).
			Suggestion("Manually remove the source file").
			WithLocation().
			Build()
	}

	return nil
}

// RemoveFile safely removes a file
func (pu *PathUtils) RemoveFile(filePath string) error {
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist, that's OK
		}
		return errors.NewError().
			Code(errors.CodeIOError).
			Type(errors.ErrTypeIO).
			Severity(errors.SeverityMedium).
			Messagef("Failed to remove file '%s'", filePath).
			Context("file", filePath).
			Cause(err).
			Suggestion("Check file permissions").
			WithLocation().
			Build()
	}
	return nil
}

// RemoveDirectory safely removes a directory and all its contents
func (pu *PathUtils) RemoveDirectory(dirPath string) error {
	if err := os.RemoveAll(dirPath); err != nil {
		return errors.NewError().
			Code(errors.CodeIOError).
			Type(errors.ErrTypeIO).
			Severity(errors.SeverityMedium).
			Messagef("Failed to remove directory '%s'", dirPath).
			Context("directory", dirPath).
			Cause(err).
			Suggestion("Check directory permissions").
			WithLocation().
			Build()
	}
	return nil
}

// CreateTempFile creates a temporary file and returns its path
func (pu *PathUtils) CreateTempFile(pattern string) (string, error) {
	tmpFile, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", errors.NewError().
			Code(errors.CodeIOError).
			Type(errors.ErrTypeIO).
			Severity(errors.SeverityMedium).
			Message("Failed to create temporary file").
			Context("pattern", pattern).
			Cause(err).
			Suggestion("Check system temp directory permissions").
			WithLocation().
			Build()
	}
	defer tmpFile.Close()

	return tmpFile.Name(), nil
}

// CreateTempDirectory creates a temporary directory and returns its path
func (pu *PathUtils) CreateTempDirectory(pattern string) (string, error) {
	tmpDir, err := os.MkdirTemp("", pattern)
	if err != nil {
		return "", errors.NewError().
			Code(errors.CodeIOError).
			Type(errors.ErrTypeIO).
			Severity(errors.SeverityMedium).
			Message("Failed to create temporary directory").
			Context("pattern", pattern).
			Cause(err).
			Suggestion("Check system temp directory permissions").
			WithLocation().
			Build()
	}

	return tmpDir, nil
}
