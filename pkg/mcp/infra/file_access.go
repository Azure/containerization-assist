package infra

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/Azure/container-kit/pkg/mcp/application/services"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// fileAccessService implements secure file access operations for MCP tools
type fileAccessService struct {
	sessionStore services.SessionStore
	sessionState services.SessionState
	logger       *slog.Logger
	maxFileSize  int64
	allowedExts  map[string]bool
	blockedPaths []string
}

// NewFileAccessService creates a new file access service
func NewFileAccessService(
	sessionStore services.SessionStore,
	sessionState services.SessionState,
	logger *slog.Logger,
) services.FileAccessService {
	return &fileAccessService{
		sessionStore: sessionStore,
		sessionState: sessionState,
		logger:       logger,
		maxFileSize:  10 * 1024 * 1024, // 10MB default
		allowedExts:  defaultAllowedExtensions(),
		blockedPaths: defaultBlockedPaths(),
	}
}

// ReadFile reads a file within the session workspace
func (f *fileAccessService) ReadFile(ctx context.Context, sessionID, path string) (string, error) {
	absolutePath, err := f.validatePath(ctx, sessionID, path)
	if err != nil {
		return "", err
	}

	// Check if file exists and is readable
	info, err := os.Stat(absolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", errors.NewError().
				Code(errors.CodeNotFound).
				Message("file not found").
				Context("path", path).
				Build()
		}
		return "", errors.NewError().
			Code(errors.CodeInternalError).
			Messagef("unable to access file: %w", err).
			Context("path", path).
			Build()
	}

	// Check if it's a directory
	if info.IsDir() {
		return "", errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("path is a directory, not a file").
			Context("path", path).
			Build()
	}

	// Check file size
	if info.Size() > f.maxFileSize {
		return "", errors.NewError().
			Code(errors.CodeResourceExhausted).
			Message("file too large").
			Context("path", path).
			Context("size", info.Size()).
			Context("max_size", f.maxFileSize).
			Build()
	}

	// Check file extension
	if !f.isAllowedExtension(absolutePath) {
		return "", errors.NewError().
			Code(errors.CodePermissionDenied).
			Message("file type not allowed").
			Context("path", path).
			Build()
	}

	// Read file content
	content, err := os.ReadFile(absolutePath)
	if err != nil {
		return "", errors.NewError().
			Code(errors.CodeInternalError).
			Messagef("unable to read file: %w", err).
			Context("path", path).
			Build()
	}

	// Ensure content is valid UTF-8
	if !utf8.Valid(content) {
		return "", errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("file contains invalid UTF-8 encoding").
			Context("path", path).
			Build()
	}

	f.logger.Debug("file read successfully",
		"session_id", sessionID,
		"path", path,
		"size", len(content))

	return string(content), nil
}

// ListDirectory lists files and directories within the session workspace
func (f *fileAccessService) ListDirectory(ctx context.Context, sessionID, path string) ([]services.FileInfo, error) {
	absolutePath, err := f.validatePath(ctx, sessionID, path)
	if err != nil {
		return nil, err
	}

	// Check if directory exists
	info, err := os.Stat(absolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.NewError().
				Code(errors.CodeNotFound).
				Message("directory not found").
				Context("path", path).
				Build()
		}
		return nil, errors.NewError().
			Code(errors.CodeInternalError).
			Messagef("unable to access directory: %w", err).
			Context("path", path).
			Build()
	}

	// Check if it's actually a directory
	if !info.IsDir() {
		return nil, errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("path is a file, not a directory").
			Context("path", path).
			Build()
	}

	// Read directory contents
	entries, err := os.ReadDir(absolutePath)
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeInternalError).
			Messagef("unable to read directory: %w", err).
			Context("path", path).
			Build()
	}

	// Convert to FileInfo
	var files []services.FileInfo
	for _, entry := range entries {
		// Skip hidden files and blocked paths
		if f.shouldSkipFile(entry.Name()) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			f.logger.Warn("unable to get file info",
				"session_id", sessionID,
				"path", path,
				"file", entry.Name(),
				"error", err)
			continue
		}

		relativePath := path
		if relativePath == "." || relativePath == "" {
			relativePath = entry.Name()
		} else {
			relativePath = filepath.Join(path, entry.Name())
		}

		files = append(files, services.FileInfo{
			Name:    entry.Name(),
			Path:    relativePath,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   info.IsDir(),
			Mode:    info.Mode().String(),
		})
	}

	f.logger.Debug("directory listed successfully",
		"session_id", sessionID,
		"path", path,
		"count", len(files))

	return files, nil
}

// FileExists checks if a file exists within the session workspace
func (f *fileAccessService) FileExists(ctx context.Context, sessionID, path string) (bool, error) {
	absolutePath, err := f.validatePath(ctx, sessionID, path)
	if err != nil {
		return false, err
	}

	_, err = os.Stat(absolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, errors.NewError().
			Code(errors.CodeInternalError).
			Messagef("unable to check file existence: %w", err).
			Context("path", path).
			Build()
	}

	return true, nil
}

// GetFileTree returns a tree representation of the directory structure
func (f *fileAccessService) GetFileTree(ctx context.Context, sessionID, rootPath string) (string, error) {
	absolutePath, err := f.validatePath(ctx, sessionID, rootPath)
	if err != nil {
		return "", err
	}

	var result strings.Builder
	err = f.buildTree(absolutePath, "", &result, 0, 3) // Max depth of 3
	if err != nil {
		return "", err
	}

	return result.String(), nil
}

// ReadFileWithMetadata reads a file with additional metadata
func (f *fileAccessService) ReadFileWithMetadata(ctx context.Context, sessionID, path string) (*services.FileContent, error) {
	absolutePath, err := f.validatePath(ctx, sessionID, path)
	if err != nil {
		return nil, err
	}

	// Get file info
	info, err := os.Stat(absolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.NewError().
				Code(errors.CodeNotFound).
				Message("file not found").
				Context("path", path).
				Build()
		}
		return nil, errors.NewError().
			Code(errors.CodeInternalError).
			Messagef("unable to access file: %w", err).
			Context("path", path).
			Build()
	}

	// Read content
	content, err := f.ReadFile(ctx, sessionID, path)
	if err != nil {
		return nil, err
	}

	// Count lines
	lines := strings.Count(content, "\n")
	if !strings.HasSuffix(content, "\n") && len(content) > 0 {
		lines++
	}

	return &services.FileContent{
		Path:     path,
		Content:  content,
		Size:     info.Size(),
		ModTime:  info.ModTime(),
		Encoding: "UTF-8",
		Lines:    lines,
	}, nil
}

// SearchFiles searches for files matching a pattern within the session workspace
func (f *fileAccessService) SearchFiles(ctx context.Context, sessionID, pattern string) ([]services.FileInfo, error) {
	workspaceDir, err := f.sessionState.GetWorkspaceDir(ctx, sessionID)
	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeInternalError).
			Messagef("unable to get workspace directory: %w", err).
			Context("session_id", sessionID).
			Build()
	}

	var matches []services.FileInfo
	err = filepath.WalkDir(workspaceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors and continue
		}

		// Skip hidden files and blocked paths
		if f.shouldSkipFile(d.Name()) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		// Check if file matches pattern
		matched, err := filepath.Match(pattern, d.Name())
		if err != nil {
			return nil // Skip invalid patterns
		}

		if matched {
			info, err := d.Info()
			if err != nil {
				return nil // Skip files we can't get info for
			}

			// Make path relative to workspace
			relativePath, err := filepath.Rel(workspaceDir, path)
			if err != nil {
				return nil
			}

			matches = append(matches, services.FileInfo{
				Name:    d.Name(),
				Path:    relativePath,
				Size:    info.Size(),
				ModTime: info.ModTime(),
				IsDir:   info.IsDir(),
				Mode:    info.Mode().String(),
			})
		}

		return nil
	})

	if err != nil {
		return nil, errors.NewError().
			Code(errors.CodeInternalError).
			Messagef("unable to search files: %w", err).
			Context("pattern", pattern).
			Build()
	}

	return matches, nil
}

// validatePath validates that the path is within the session workspace and safe to access
func (f *fileAccessService) validatePath(ctx context.Context, sessionID, relativePath string) (string, error) {
	// Get session workspace directory
	workspaceDir, err := f.sessionState.GetWorkspaceDir(ctx, sessionID)
	if err != nil {
		return "", errors.NewError().
			Code(errors.CodeInternalError).
			Messagef("unable to get workspace directory: %w", err).
			Context("session_id", sessionID).
			Build()
	}

	if workspaceDir == "" {
		return "", errors.NewError().
			Code(errors.CodeInternalError).
			Message("session has no workspace directory").
			Context("session_id", sessionID).
			Build()
	}

	// Clean and resolve the path
	cleanPath := filepath.Clean(relativePath)

	// Prevent path traversal
	if strings.Contains(cleanPath, "..") {
		return "", errors.NewError().
			Code(errors.CodePermissionDenied).
			Message("path traversal not allowed").
			Context("path", relativePath).
			Build()
	}

	// Build absolute path
	absolutePath := filepath.Join(workspaceDir, cleanPath)

	// Ensure the resolved path is within the workspace
	if !strings.HasPrefix(absolutePath, workspaceDir) {
		return "", errors.NewError().
			Code(errors.CodePermissionDenied).
			Message("path outside workspace").
			Context("path", relativePath).
			Build()
	}

	// Check blocked paths
	for _, blocked := range f.blockedPaths {
		if strings.Contains(cleanPath, blocked) {
			return "", errors.NewError().
				Code(errors.CodePermissionDenied).
				Message("access to this path is blocked").
				Context("path", relativePath).
				Context("blocked_pattern", blocked).
				Build()
		}
	}

	return absolutePath, nil
}

// isAllowedExtension checks if a file extension is allowed
func (f *fileAccessService) isAllowedExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))

	// Check exact filename matches (like "Dockerfile")
	if f.allowedExts[base] {
		return true
	}

	// Check extension matches
	if ext != "" && f.allowedExts[ext] {
		return true
	}

	return false
}

// shouldSkipFile determines if a file should be skipped during directory listing
func (f *fileAccessService) shouldSkipFile(name string) bool {
	// Skip hidden files
	if strings.HasPrefix(name, ".") && name != "." && name != ".." {
		return true
	}

	// Skip common build artifacts and caches
	skipPatterns := []string{
		"node_modules",
		"__pycache__",
		".git",
		".vscode",
		".idea",
		"target",
		"build",
		"dist",
		"*.tmp",
		"*.log",
	}

	for _, pattern := range skipPatterns {
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
	}

	return false
}

// buildTree recursively builds a tree representation
func (f *fileAccessService) buildTree(path, prefix string, result *strings.Builder, depth, maxDepth int) error {
	if depth > maxDepth {
		return nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for i, entry := range entries {
		if f.shouldSkipFile(entry.Name()) {
			continue
		}

		isLast := i == len(entries)-1
		currentPrefix := "├── "
		nextPrefix := "│   "

		if isLast {
			currentPrefix = "└── "
			nextPrefix = "    "
		}

		result.WriteString(prefix + currentPrefix + entry.Name())

		if entry.IsDir() {
			result.WriteString("/\n")
			subPath := filepath.Join(path, entry.Name())
			err := f.buildTree(subPath, prefix+nextPrefix, result, depth+1, maxDepth)
			if err != nil {
				// Log error but continue
				f.logger.Warn("error building tree for subdirectory",
					"path", subPath,
					"error", err)
			}
		} else {
			info, err := entry.Info()
			if err == nil {
				result.WriteString(fmt.Sprintf(" (%d bytes)", info.Size()))
			}
			result.WriteString("\n")
		}
	}

	return nil
}

// defaultAllowedExtensions returns the default set of allowed file extensions
func defaultAllowedExtensions() map[string]bool {
	return map[string]bool{
		// Source code files
		".go":    true,
		".js":    true,
		".jsx":   true,
		".ts":    true,
		".tsx":   true,
		".py":    true,
		".java":  true,
		".c":     true,
		".cpp":   true,
		".cc":    true,
		".cxx":   true,
		".h":     true,
		".hpp":   true,
		".cs":    true,
		".rb":    true,
		".php":   true,
		".rs":    true,
		".swift": true,
		".kt":    true,
		".scala": true,
		".clj":   true,
		".ex":    true,
		".exs":   true,
		".erl":   true,
		".hrl":   true,

		// Configuration files
		".json":       true,
		".yaml":       true,
		".yml":        true,
		".toml":       true,
		".ini":        true,
		".cfg":        true,
		".conf":       true,
		".config":     true,
		".xml":        true,
		".properties": true,
		".env":        true,

		// Build and project files
		".dockerfile":       true,
		"dockerfile":        true,
		".dockerignore":     true,
		"makefile":          true,
		".makefile":         true,
		".gradle":           true,
		".maven":            true,
		"go.mod":            true,
		"go.sum":            true,
		"package.json":      true,
		"package-lock.json": true,
		"yarn.lock":         true,
		"composer.json":     true,
		"composer.lock":     true,
		"requirements.txt":  true,
		"setup.py":          true,
		"pyproject.toml":    true,
		"pom.xml":           true,
		"build.gradle":      true,
		"build.xml":         true,
		"cargo.toml":        true,
		"cargo.lock":        true,

		// Documentation
		".md":   true,
		".txt":  true,
		".rst":  true,
		".adoc": true,
		".org":  true,

		// Shell scripts
		".sh":   true,
		".bash": true,
		".zsh":  true,
		".fish": true,
		".ps1":  true,
		".bat":  true,
		".cmd":  true,

		// Web files
		".html":   true,
		".htm":    true,
		".css":    true,
		".scss":   true,
		".sass":   true,
		".less":   true,
		".vue":    true,
		".svelte": true,

		// Data files
		".csv":     true,
		".tsv":     true,
		".sql":     true,
		".graphql": true,
		".gql":     true,
	}
}

// defaultBlockedPaths returns the default set of blocked path patterns
func defaultBlockedPaths() []string {
	return []string{
		".git/objects",
		".git/hooks",
		".git/refs",
		"node_modules",
		"__pycache__",
		".pytest_cache",
		".env.local",
		".env.production",
		".env.staging",
		"secrets",
		"credentials",
		".ssh",
		".gnupg",
		".aws",
		".docker",
		"coverage",
		".coverage",
		".nyc_output",
		"tmp",
		"temp",
		".tmp",
		".temp",
		"logs",
		"log",
		".log",
	}
}
