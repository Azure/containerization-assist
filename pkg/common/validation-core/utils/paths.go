package utils

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
)

// Path validation utilities consolidated from multiple packages

// ValidatePath validates a file or directory path
func ValidatePath(path, fieldName string) *core.Error {
	if path == "" {
		return core.NewFieldError(fieldName, "path cannot be empty")
	}

	// Clean the path to resolve any . or .. components
	cleanPath := filepath.Clean(path)

	// Check for path traversal attempts
	if strings.Contains(cleanPath, "..") {
		return core.NewFieldError(fieldName, "path traversal not allowed").
			WithSuggestion("Use absolute paths or paths within the current directory")
	}

	return nil
}

// ValidateAbsolutePath validates that the path is absolute
func ValidateAbsolutePath(path, fieldName string) *core.Error {
	if path == "" {
		return nil
	}

	if !filepath.IsAbs(path) {
		return core.NewFieldError(fieldName, "must be an absolute path").
			WithSuggestion("Start path with / on Unix or C:\\ on Windows")
	}

	return nil
}

// ValidateRelativePath validates that the path is relative
func ValidateRelativePath(path, fieldName string) *core.Error {
	if path == "" {
		return nil
	}

	if filepath.IsAbs(path) {
		return core.NewFieldError(fieldName, "must be a relative path").
			WithSuggestion("Remove leading / or drive letter")
	}

	return nil
}

// ValidateFileExists validates that the file exists
func ValidateFileExists(path, fieldName string) *core.Error {
	if path == "" {
		return nil
	}

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return core.NewFieldError(fieldName, "file does not exist: "+path)
	}
	if err != nil {
		return core.NewFieldError(fieldName, "cannot access file: "+err.Error())
	}

	if info.IsDir() {
		return core.NewFieldError(fieldName, "path is a directory, not a file: "+path)
	}

	return nil
}

// ValidateDirectoryExists validates that the directory exists
func ValidateDirectoryExists(path, fieldName string) *core.Error {
	if path == "" {
		return nil
	}

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return core.NewFieldError(fieldName, "directory does not exist: "+path)
	}
	if err != nil {
		return core.NewFieldError(fieldName, "cannot access directory: "+err.Error())
	}

	if !info.IsDir() {
		return core.NewFieldError(fieldName, "path is a file, not a directory: "+path)
	}

	return nil
}

// ValidatePathExists validates that the path exists (file or directory)
func ValidatePathExists(path, fieldName string) *core.Error {
	if path == "" {
		return nil
	}

	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return core.NewFieldError(fieldName, "path does not exist: "+path)
	}
	if err != nil {
		return core.NewFieldError(fieldName, "cannot access path: "+err.Error())
	}

	return nil
}

// ValidateFileReadable validates that the file exists and is readable
func ValidateFileReadable(path, fieldName string) *core.Error {
	if err := ValidateFileExists(path, fieldName); err != nil {
		return err
	}

	file, err := os.Open(path)
	if err != nil {
		return core.NewFieldError(fieldName, "file is not readable: "+err.Error())
	}
	file.Close()

	return nil
}

// ValidateFileWritable validates that the file can be written to
func ValidateFileWritable(path, fieldName string) *core.Error {
	if path == "" {
		return nil
	}

	// Check if file exists
	if _, err := os.Stat(path); err == nil {
		// File exists, check if writable
		file, err := os.OpenFile(path, os.O_WRONLY, 0)
		if err != nil {
			return core.NewFieldError(fieldName, "file is not writable: "+err.Error())
		}
		file.Close()
	} else if os.IsNotExist(err) {
		// File doesn't exist, check if parent directory is writable
		dir := filepath.Dir(path)
		if err := ValidateDirectoryWritable(dir, fieldName); err != nil {
			return core.NewFieldError(fieldName, "cannot create file in directory: "+err.Error())
		}
	} else {
		return core.NewFieldError(fieldName, "cannot access file: "+err.Error())
	}

	return nil
}

// ValidateDirectoryWritable validates that the directory is writable
func ValidateDirectoryWritable(path, fieldName string) *core.Error {
	if err := ValidateDirectoryExists(path, fieldName); err != nil {
		return err
	}

	// Try to create a temporary file in the directory
	tempFile, err := os.CreateTemp(path, "validate_write_")
	if err != nil {
		return core.NewFieldError(fieldName, "directory is not writable: "+err.Error())
	}

	// Clean up the temporary file
	tempFile.Close()
	os.Remove(tempFile.Name())

	return nil
}

// ValidateFileExtension validates that the file has one of the allowed extensions
func ValidateFileExtension(path, fieldName string, allowedExtensions []string) *core.Error {
	if path == "" {
		return nil
	}

	ext := strings.ToLower(filepath.Ext(path))

	for _, allowed := range allowedExtensions {
		if strings.ToLower(allowed) == ext {
			return nil
		}
	}

	return core.NewFieldError(fieldName,
		"file extension '"+ext+"' not allowed. Allowed extensions: "+strings.Join(allowedExtensions, ", "))
}

// ValidateDockerfilePath validates a Dockerfile path
func ValidateDockerfilePath(path, fieldName string) *core.Error {
	if err := ValidatePath(path, fieldName); err != nil {
		return err
	}

	// Dockerfile should exist
	if err := ValidateFileExists(path, fieldName); err != nil {
		return err
	}

	// Should be readable
	if err := ValidateFileReadable(path, fieldName); err != nil {
		return err
	}

	// Validate common Dockerfile names
	filename := filepath.Base(path)
	validNames := []string{"Dockerfile", "dockerfile", "Dockerfile.dev", "Dockerfile.prod"}

	nameValid := false
	for _, validName := range validNames {
		if filename == validName || strings.HasPrefix(filename, "Dockerfile.") {
			nameValid = true
			break
		}
	}

	if !nameValid {
		return core.NewFieldError(fieldName,
			"Dockerfile should be named 'Dockerfile' or 'Dockerfile.*'").
			WithSuggestion("Rename file to follow Docker conventions")
	}

	return nil
}

// ValidateKubernetesManifestPath validates a Kubernetes manifest path
func ValidateKubernetesManifestPath(path, fieldName string) *core.Error {
	if err := ValidatePath(path, fieldName); err != nil {
		return err
	}

	if err := ValidateFileExists(path, fieldName); err != nil {
		return err
	}

	if err := ValidateFileReadable(path, fieldName); err != nil {
		return err
	}

	// Validate file extension
	allowedExtensions := []string{".yaml", ".yml", ".json"}
	if err := ValidateFileExtension(path, fieldName, allowedExtensions); err != nil {
		return err
	}

	return nil
}

// ValidateConfigPath validates a configuration file path
func ValidateConfigPath(path, fieldName string) *core.Error {
	if err := ValidatePath(path, fieldName); err != nil {
		return err
	}

	if err := ValidateFileExists(path, fieldName); err != nil {
		return err
	}

	if err := ValidateFileReadable(path, fieldName); err != nil {
		return err
	}

	// Validate common config file extensions
	allowedExtensions := []string{".yaml", ".yml", ".json", ".toml", ".ini", ".conf", ".config"}
	if err := ValidateFileExtension(path, fieldName, allowedExtensions); err != nil {
		return err
	}

	return nil
}

// ValidateWorkingDirectory validates a working directory path
func ValidateWorkingDirectory(path, fieldName string) *core.Error {
	if err := ValidatePath(path, fieldName); err != nil {
		return err
	}

	if err := ValidateDirectoryExists(path, fieldName); err != nil {
		return err
	}

	// Should be readable and writable for working directory
	if err := ValidateDirectoryWritable(path, fieldName); err != nil {
		return err
	}

	return nil
}

// ValidateNoHiddenPaths validates that the path doesn't contain hidden components
func ValidateNoHiddenPaths(path, fieldName string) *core.Error {
	if path == "" {
		return nil
	}

	parts := strings.Split(filepath.Clean(path), string(filepath.Separator))

	for _, part := range parts {
		if strings.HasPrefix(part, ".") && part != "." && part != ".." {
			return core.NewFieldError(fieldName,
				"path cannot contain hidden files or directories (starting with '.')")
		}
	}

	return nil
}

// ValidatePathLength validates that the path is not too long
func ValidatePathLength(path, fieldName string, maxLength int) *core.Error {
	if path == "" {
		return nil
	}

	if len(path) > maxLength {
		return core.NewFieldError(fieldName,
			"path is too long ("+string(rune(len(path)))+" characters, max "+string(rune(maxLength))+")")
	}

	return nil
}

// ValidateSecurePath validates that the path is secure (no traversal, reasonable length, etc.)
func ValidateSecurePath(path, fieldName string) *core.Error {
	if err := ValidatePath(path, fieldName); err != nil {
		return err
	}

	// Check for reasonable path length (most filesystems support up to 4096)
	if err := ValidatePathLength(path, fieldName, 4096); err != nil {
		return err
	}

	// Additional security checks
	cleanPath := filepath.Clean(path)

	// Check for null bytes (security issue in some contexts)
	if strings.Contains(cleanPath, "\x00") {
		return core.NewFieldError(fieldName, "path contains null bytes")
	}

	// Check for suspicious patterns
	suspiciousPatterns := []string{
		"/etc/passwd", "/etc/shadow", "/proc/", "/sys/",
		"\\windows\\system32", "\\windows\\", "C:\\Windows\\",
	}

	lowerPath := strings.ToLower(cleanPath)
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(lowerPath, strings.ToLower(pattern)) {
			return core.NewFieldError(fieldName,
				"path accesses sensitive system location").
				WithSuggestion("Use application-specific directories")
		}
	}

	return nil
}
