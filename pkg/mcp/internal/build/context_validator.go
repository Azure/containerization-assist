package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
)

// ContextValidator handles build context validation
type ContextValidator struct {
	logger zerolog.Logger
}

// NewContextValidator creates a new context validator
func NewContextValidator(logger zerolog.Logger) *ContextValidator {
	return &ContextValidator{
		logger: logger.With().Str("component", "context_validator").Logger(),
	}
}

// Validate performs build context validation
func (v *ContextValidator) Validate(content string, options ValidationOptions) (*ValidationResult, error) {
	v.logger.Info().Msg("Starting build context validation")

	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]ValidationError, 0),
		Warnings: make([]ValidationWarning, 0),
	}

	lines := strings.Split(content, "\n")

	// Extract file operations
	fileOps := v.extractFileOperations(lines)

	// Validate file operations
	v.validateFileOperations(fileOps, result)

	// Check for build context issues
	v.checkBuildContextSize(fileOps, result)
	v.checkDockerignore(fileOps, result)
	v.checkFilePaths(fileOps, result)

	// Update result state  
	if len(result.Errors) > 0 {
		result.Valid = false
	}

	return result, nil
}

// Analyze provides context-specific analysis
func (v *ContextValidator) Analyze(lines []string, context ValidationContext) interface{} {
	fileOps := v.extractFileOperations(lines)

	analysis := ContextAnalysis{
		TotalFileOps:      len(fileOps),
		CopyOperations:    0,
		AddOperations:     0,
		LargeFileWarnings: make([]string, 0),
		BuildContextTips:  make([]string, 0),
	}

	// Count operation types
	for _, op := range fileOps {
		switch op.Type {
		case "COPY":
			analysis.CopyOperations++
		case "ADD":
			analysis.AddOperations++
		}
	}

	// Check for common patterns
	if analysis.AddOperations > 0 && analysis.CopyOperations > 0 {
		analysis.BuildContextTips = append(analysis.BuildContextTips,
			"Prefer COPY over ADD unless you need ADD's special features")
	}

	// Check for inefficient patterns
	hasWildcard := false
	for _, op := range fileOps {
		if strings.Contains(op.Source, "*") || strings.Contains(op.Source, "?") {
			hasWildcard = true
			break
		}
	}

	if hasWildcard {
		analysis.BuildContextTips = append(analysis.BuildContextTips,
			"Use .dockerignore to exclude unnecessary files when using wildcards")
	}

	// Check for large context operations
	for _, op := range fileOps {
		if op.Source == "." || op.Source == "./" {
			analysis.LargeFileWarnings = append(analysis.LargeFileWarnings,
				fmt.Sprintf("Line %d: Copying entire context with '%s'", op.Line, op.Source))
			analysis.BuildContextTips = append(analysis.BuildContextTips,
				"Be specific about what files to copy to minimize build context")
		}
	}

	return analysis
}

// FileOperation represents a file operation in Dockerfile
type FileOperation struct {
	Line        int
	Type        string // COPY, ADD
	Source      string
	Destination string
	Flags       []string
}

// ContextAnalysis contains build context analysis results
type ContextAnalysis struct {
	TotalFileOps      int
	CopyOperations    int
	AddOperations     int
	LargeFileWarnings []string
	BuildContextTips  []string
}

// extractFileOperations extracts COPY and ADD operations
func (v *ContextValidator) extractFileOperations(lines []string) []FileOperation {
	operations := make([]FileOperation, 0)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		if strings.HasPrefix(upper, "COPY") || strings.HasPrefix(upper, "ADD") {
			op := FileOperation{
				Line: i + 1,
			}

			parts := strings.Fields(trimmed)
			if len(parts) >= 3 {
				op.Type = strings.ToUpper(parts[0])

				// Parse flags
				j := 1
				for j < len(parts) && strings.HasPrefix(parts[j], "--") {
					op.Flags = append(op.Flags, parts[j])
					j++
				}

				// Get source and destination
				if j < len(parts)-1 {
					op.Source = parts[j]
					op.Destination = parts[len(parts)-1]
				}

				operations = append(operations, op)
			}
		}
	}

	return operations
}

// validateFileOperations validates file operations
func (v *ContextValidator) validateFileOperations(operations []FileOperation, result *ValidationResult) {
	for _, op := range operations {
		// Check for ADD with local files (prefer COPY)
		if op.Type == "ADD" && !v.isRemoteURL(op.Source) && !v.isArchive(op.Source) {
			result.Warnings = append(result.Warnings, ValidationWarning{
				//Type:       "add_local_files",
				Line:       op.Line,
				Message:    "Using ADD for local files",
				//Suggestion: "Use COPY instead of ADD for local files",
				//Impact:     "clarity",
			})
		}

		// Check for copying to root
		if op.Destination == "/" {
			result.Warnings = append(result.Warnings, ValidationWarning{
				//Type:       "copy_to_root",
				Line:       op.Line,
				Message:    "Copying files directly to root directory",
				//Suggestion: "Copy files to a specific directory instead of root",
				//Impact:     "organization",
			})
		}

		// Check for absolute source paths
		if filepath.IsAbs(op.Source) && !v.hasFromFlag(op.Flags) {
			result.Errors = append(result.Errors, ValidationError{
				//Type:     "absolute_source_path",
				Line:     op.Line,
				Message:  fmt.Sprintf("Absolute source path '%s' is not allowed", op.Source),
				//Severity: "error",
			})
		}

		// Check for copying sensitive files
		if v.isSensitiveFile(op.Source) {
			result.Warnings = append(result.Warnings, ValidationWarning{
				//Type:       "sensitive_file_copy",
				Line:       op.Line,
				Message:    fmt.Sprintf("Copying potentially sensitive file: %s", op.Source),
				//Suggestion: "Ensure sensitive files are excluded via .dockerignore",
				//Impact:     "security",
			})
		}
	}
}

// checkBuildContextSize checks for operations that might increase context size
func (v *ContextValidator) checkBuildContextSize(operations []FileOperation, result *ValidationResult) {
	wholeDirCopies := 0

	for _, op := range operations {
		// Check for copying entire directories
		if op.Source == "." || op.Source == "./" || strings.HasSuffix(op.Source, "/") {
			wholeDirCopies++
		}

		// Check for recursive wildcards
		if strings.Contains(op.Source, "**") {
			result.Warnings = append(result.Warnings, ValidationWarning{
				//Type:       "recursive_wildcard",
				Line:       op.Line,
				Message:    "Using recursive wildcard in COPY/ADD",
				//Suggestion: "Be specific about files to copy to reduce build context",
				//Impact:     "build_time",
			})
		}
	}

	if wholeDirCopies > 2 {
		result.Warnings = append(result.Warnings, ValidationWarning{
			//Type:       "excessive_dir_copies",
			Line:       0,
			Message:    fmt.Sprintf("Multiple whole directory copies detected (%d)", wholeDirCopies),
			//Suggestion: "Consider being more selective about what to copy",
			//Impact:     "build_time",
		})
	}
}

// checkDockerignore checks for .dockerignore best practices
func (v *ContextValidator) checkDockerignore(operations []FileOperation, result *ValidationResult) {
	// Check if we're copying the entire context
	hasContextCopy := false
	for _, op := range operations {
		if op.Source == "." || op.Source == "./" {
			hasContextCopy = true
			break
		}
	}

	if hasContextCopy {
		// Note: Suggestions field removed from ValidationResult

		// Check for common files that should be ignored
		suspiciousPatterns := []string{
			".git", ".gitignore", "*.log", "*.tmp",
			"node_modules", "__pycache__", ".env",
		}

		for _, op := range operations {
			for _, pattern := range suspiciousPatterns {
				if strings.Contains(op.Source, pattern) {
					result.Warnings = append(result.Warnings, ValidationWarning{
						//Type:       "unfiltered_copy",
						Line:       op.Line,
						Message:    fmt.Sprintf("Copying '%s' - should this be in .dockerignore?", pattern),
						//Suggestion: "Add unnecessary files to .dockerignore",
						//Impact:     "build_time",
					})
					break
				}
			}
		}
	}
}

// checkFilePaths checks for problematic file paths
func (v *ContextValidator) checkFilePaths(operations []FileOperation, result *ValidationResult) {
	for _, op := range operations {
		// Check for parent directory references
		if strings.Contains(op.Source, "..") {
			result.Errors = append(result.Errors, ValidationError{
				//Type:     "parent_dir_reference",
				Line:     op.Line,
				Message:  "Cannot reference parent directory in build context",
				//Severity: "error",
			})
		}

		// Check for Windows-style paths on Linux
		if strings.Contains(op.Source, "\\") || strings.Contains(op.Destination, "\\") {
			result.Warnings = append(result.Warnings, ValidationWarning{
				//Type:       "windows_path",
				Line:       op.Line,
				Message:    "Windows-style path detected",
				//Suggestion: "Use forward slashes for cross-platform compatibility",
				//Impact:     "portability",
			})
		}

		// Check for spaces in paths
		if strings.Contains(op.Source, " ") || strings.Contains(op.Destination, " ") {
			result.Warnings = append(result.Warnings, ValidationWarning{
				//Type:       "spaces_in_path",
				Line:       op.Line,
				Message:    "Path contains spaces",
				//Suggestion: "Avoid spaces in file paths or properly quote them",
				//Impact:     "reliability",
			})
		}
	}
}

// Helper functions

func (v *ContextValidator) isRemoteURL(source string) bool {
	return strings.HasPrefix(source, "http://") ||
		strings.HasPrefix(source, "https://") ||
		strings.HasPrefix(source, "ftp://")
}

func (v *ContextValidator) isArchive(source string) bool {
	archiveExts := []string{
		".tar", ".tar.gz", ".tgz", ".tar.bz2",
		".tar.xz", ".zip", ".gz", ".bz2",
	}

	lower := strings.ToLower(source)
	for _, ext := range archiveExts {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func (v *ContextValidator) hasFromFlag(flags []string) bool {
	for _, flag := range flags {
		if strings.HasPrefix(flag, "--from=") {
			return true
		}
	}
	return false
}

func (v *ContextValidator) isSensitiveFile(source string) bool {
	sensitivePatterns := []string{
		".env", "secrets", "credentials", "password",
		".ssh", "id_rsa", "id_dsa", ".pem", ".key",
		"kubeconfig", ".aws", ".gcp", ".azure",
	}

	lower := strings.ToLower(source)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}

// ValidateWithContext validates Dockerfile with actual build context
func (v *ContextValidator) ValidateWithContext(dockerfilePath, contextPath string) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]ValidationError, 0),
		Warnings: make([]ValidationWarning, 0),
	}

	// Check if context exists
	if _, err := os.Stat(contextPath); os.IsNotExist(err) {
		result.Errors = append(result.Errors, ValidationError{
			//Type:     "missing_context",
			Line:     0,
			Message:  fmt.Sprintf("Build context directory does not exist: %s", contextPath),
			//Severity: "error",
		})
		result.Valid = false
		return result, nil
	}

	// Check .dockerignore
	dockerignorePath := filepath.Join(contextPath, ".dockerignore")
	if _, err := os.Stat(dockerignorePath); os.IsNotExist(err) {
		result.Warnings = append(result.Warnings, ValidationWarning{
			//Type:       "missing_dockerignore",
			Line:       0,
			Message:    "No .dockerignore file found",
			//Suggestion: "Create .dockerignore to exclude unnecessary files from build context",
			//Impact:     "build_time",
		})
	}

	// Check context size
	size, err := v.calculateContextSize(contextPath)
	if err == nil {
		// Note: Context field removed from ValidationResult

		// Warn if context is too large
		if size > 100*1024*1024 { // 100MB
			result.Warnings = append(result.Warnings, ValidationWarning{
				//Type:       "large_context",
				Line:       0,
				Message:    fmt.Sprintf("Build context is large: %.2f MB", float64(size)/(1024*1024)),
				//Suggestion: "Use .dockerignore to exclude unnecessary files",
				//Impact:     "build_time",
			})
		}
	}

	// Note: TotalIssues field removed from ValidationResult

	return result, nil
}

func (v *ContextValidator) calculateContextSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}
