package validators

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/validation/core"
)

// ContextValidator validates build context and file operations
type ContextValidator struct {
	*BaseValidatorImpl
	pathValidator *PathValidator
}

// PathValidator handles path-specific validation
type PathValidator struct {
	*BaseValidatorImpl
}

// NewContextValidator creates a new context validator
func NewContextValidator() *ContextValidator {
	return &ContextValidator{
		BaseValidatorImpl: NewBaseValidator("context", "1.0.0", []string{"build_context", "file_operations"}),
		pathValidator:     NewPathValidator(),
	}
}

// NewPathValidator creates a new path validator
func NewPathValidator() *PathValidator {
	return &PathValidator{
		BaseValidatorImpl: NewBaseValidator("path", "1.0.0", []string{"file_path", "directory"}),
	}
}

// Validate validates build context data
func (c *ContextValidator) Validate(ctx context.Context, data interface{}, options *core.ValidationOptions) *core.ValidationResult {
	result := c.BaseValidatorImpl.Validate(ctx, data, options)

	// Determine what type of context validation to perform
	validationType := ""
	if options != nil && options.Context != nil {
		if vt, ok := options.Context["validation_type"].(string); ok {
			validationType = vt
		}
	}

	switch validationType {
	case "dockerfile_context":
		c.validateDockerfileContext(data, result, options)
	case "build_files":
		c.validateBuildFiles(data, result, options)
	case "copy_operations":
		c.validateCopyOperations(data, result, options)
	default:
		// Try to detect the type of validation needed
		c.autoDetectAndValidate(data, result, options)
	}

	return result
}

// ContextInstruction represents a COPY/ADD instruction in a Dockerfile
type ContextInstruction struct {
	Type        string // COPY, ADD
	Source      string
	Destination string
	Line        int
	Options     map[string]string
}

// ContextData represents build context data for validation
type ContextData struct {
	DockerfilePath string
	ContextPath    string
	Instructions   []ContextInstruction
}

// validateDockerfileContext validates Dockerfile context references
func (c *ContextValidator) validateDockerfileContext(data interface{}, result *core.ValidationResult, options *core.ValidationOptions) {

	contextData, ok := data.(*ContextData)
	if !ok {
		// Try to parse from map
		if m, ok := data.(map[string]interface{}); ok {
			contextData = &ContextData{}
			if path, ok := m["dockerfile_path"].(string); ok {
				contextData.DockerfilePath = path
			}
			if path, ok := m["context_path"].(string); ok {
				contextData.ContextPath = path
			}
			// Parse instructions if available
			if instructions, ok := m["instructions"].([]interface{}); ok {
				for _, inst := range instructions {
					if instMap, ok := inst.(map[string]interface{}); ok {
						instruction := ContextInstruction{
							Type:        getStringFromMap(instMap, "type"),
							Source:      getStringFromMap(instMap, "source"),
							Destination: getStringFromMap(instMap, "destination"),
							Line:        getIntFromMap(instMap, "line"),
						}
						contextData.Instructions = append(contextData.Instructions, instruction)
					}
				}
			}
		} else {
			result.AddError(core.NewValidationError(
				"INVALID_CONTEXT_DATA",
				"Context data must be a ContextData struct or compatible map",
				core.ErrTypeValidation,
				core.SeverityHigh,
			))
			return
		}
	}

	// Validate each instruction
	for _, instruction := range contextData.Instructions {
		c.validateContextInstruction(instruction, contextData.ContextPath, result)
	}
}

// validateContextInstruction validates a single context instruction
func (c *ContextValidator) validateContextInstruction(instruction ContextInstruction, contextPath string, result *core.ValidationResult) {
	// Validate source path
	if instruction.Source == "" {
		result.AddError(core.NewValidationError(
			"EMPTY_SOURCE_PATH",
			fmt.Sprintf("%s instruction has empty source path", instruction.Type),
			core.ErrTypeValidation,
			core.SeverityHigh,
		).WithLine(instruction.Line))
		return
	}

	// Check for path traversal attempts
	if strings.Contains(instruction.Source, "..") {
		if c.isPathOutsideContext(instruction.Source, contextPath) {
			result.AddError(core.NewValidationError(
				"PATH_TRAVERSAL",
				fmt.Sprintf("Source path '%s' references files outside build context", instruction.Source),
				core.ErrTypeSecurity,
				core.SeverityHigh,
			).WithLine(instruction.Line).
				WithSuggestion("Ensure all source files are within the build context"))
		}
	}

	// Validate destination path
	if instruction.Destination == "" {
		result.AddError(core.NewValidationError(
			"EMPTY_DESTINATION_PATH",
			fmt.Sprintf("%s instruction has empty destination path", instruction.Type),
			core.ErrTypeValidation,
			core.SeverityHigh,
		).WithLine(instruction.Line))
		return
	}

	// Check for absolute paths in source (should be relative to context)
	if filepath.IsAbs(instruction.Source) {
		warning := core.NewValidationWarning(
			"ABSOLUTE_SOURCE_PATH",
			fmt.Sprintf("Source path '%s' is absolute. It should be relative to build context", instruction.Source),
		)
		warning.ValidationError.WithLine(instruction.Line)
		result.AddWarning(warning)
	}

	// Check for sensitive file patterns
	c.checkSensitiveFiles(instruction.Source, instruction.Line, result)

	// Validate COPY --from usage
	if instruction.Type == "COPY" && instruction.Options != nil {
		if from, ok := instruction.Options["from"]; ok && from != "" {
			// This is a multi-stage copy, validate the stage reference
			if !c.isValidStageOrImageReference(from) {
				result.AddError(core.NewValidationError(
					"INVALID_COPY_FROM",
					fmt.Sprintf("Invalid --from reference: %s", from),
					core.ErrTypeSyntax,
					core.SeverityMedium,
				).WithLine(instruction.Line))
			}
		}
	}

	// ADD-specific validations
	if instruction.Type == "ADD" {
		c.validateAddInstruction(instruction, result)
	}
}

// validateBuildFiles validates files in build context
func (c *ContextValidator) validateBuildFiles(data interface{}, result *core.ValidationResult, options *core.ValidationOptions) {
	files, ok := data.([]string)
	if !ok {
		if m, ok := data.(map[string]interface{}); ok {
			if f, ok := m["files"].([]string); ok {
				files = f
			}
		}
		if files == nil {
			result.AddError(core.NewValidationError(
				"INVALID_FILE_LIST",
				"Build files must be provided as a string array",
				core.ErrTypeValidation,
				core.SeverityHigh,
			))
			return
		}
	}

	// Check for .dockerignore
	hasDockerignore := false
	sensitiveFileCount := 0

	for _, file := range files {
		if filepath.Base(file) == ".dockerignore" {
			hasDockerignore = true
		}

		// Check for sensitive files that shouldn't be in build context
		if c.isSensitiveFile(file) {
			sensitiveFileCount++
			warning := core.NewValidationWarning(
				"SENSITIVE_FILE_IN_CONTEXT",
				fmt.Sprintf("Sensitive file in build context: %s", file),
			)
			warning.ValidationError.WithSuggestion("Add sensitive files to .dockerignore")
			result.AddWarning(warning)
		}
	}

	if !hasDockerignore {
		result.AddWarning(core.NewValidationWarning(
			"MISSING_DOCKERIGNORE",
			"No .dockerignore file found. Consider adding one to exclude unnecessary files",
		))
	}

	if sensitiveFileCount > 0 {
		result.AddWarning(core.NewValidationWarning(
			"SENSITIVE_FILES_COUNT",
			fmt.Sprintf("Found %d potentially sensitive files in build context", sensitiveFileCount),
		))
	}

	// Check build context size (if size information is provided)
	if options != nil && options.Context != nil {
		if size, ok := options.Context["context_size_mb"].(float64); ok && size > 100 {
			result.AddWarning(core.NewValidationWarning(
				"LARGE_BUILD_CONTEXT",
				fmt.Sprintf("Build context is %.2f MB. Consider using .dockerignore to reduce size", size),
			))
		}
	}
}

// validateCopyOperations validates COPY/ADD operations
func (c *ContextValidator) validateCopyOperations(data interface{}, result *core.ValidationResult, options *core.ValidationOptions) {
	operations, ok := data.([]map[string]interface{})
	if !ok {
		result.AddError(core.NewValidationError(
			"INVALID_COPY_OPERATIONS",
			"Copy operations must be provided as an array of operation maps",
			core.ErrTypeValidation,
			core.SeverityHigh,
		))
		return
	}

	for i, op := range operations {
		instruction, ok := op["instruction"].(string)
		if !ok {
			result.AddError(core.NewValidationError(
				"INVALID_INSTRUCTION",
				fmt.Sprintf("Operation %d missing or invalid instruction field", i),
				core.ErrTypeValidation,
				core.SeverityHigh,
			))
			continue
		}
		source, ok := op["source"].(string)
		if !ok {
			result.AddError(core.NewValidationError(
				"INVALID_SOURCE",
				fmt.Sprintf("Operation %d missing or invalid source field", i),
				core.ErrTypeValidation,
				core.SeverityHigh,
			))
			continue
		}
		dest, ok := op["destination"].(string)
		if !ok {
			result.AddError(core.NewValidationError(
				"INVALID_DESTINATION",
				fmt.Sprintf("Operation %d missing or invalid destination field", i),
				core.ErrTypeValidation,
				core.SeverityHigh,
			))
			continue
		}
		line := i + 1
		if l, ok := op["line"].(int); ok {
			line = l
		}

		// Validate each operation
		if instruction == "ADD" && !c.needsAddInstruction(source) {
			warning := core.NewValidationWarning(
				"PREFER_COPY_OVER_ADD",
				fmt.Sprintf("Use COPY instead of ADD for '%s' (ADD not needed for simple file copy)", source),
			)
			warning.ValidationError.WithLine(line)
			result.AddWarning(warning)
		}

		// Check for wildcards
		if strings.Contains(source, "*") || strings.Contains(source, "?") {
			warning := core.NewValidationWarning(
				"WILDCARD_COPY",
				fmt.Sprintf("Using wildcards in %s source: %s. Be explicit about files to copy when possible", instruction, source),
			)
			warning.ValidationError.WithLine(line)
			result.AddWarning(warning)
		}

		// Validate destination
		if !filepath.IsAbs(dest) {
			warning := core.NewValidationWarning(
				"RELATIVE_DESTINATION",
				fmt.Sprintf("Destination path '%s' is relative. Consider using absolute paths for clarity", dest),
			)
			warning.ValidationError.WithLine(line)
			result.AddWarning(warning)
		}
	}
}

// autoDetectAndValidate attempts to detect the validation type
func (c *ContextValidator) autoDetectAndValidate(data interface{}, result *core.ValidationResult, options *core.ValidationOptions) {
	switch v := data.(type) {
	case string:
		// Single path validation
		c.pathValidator.ValidatePath(v, result)
	case []string:
		// Multiple files validation
		c.validateBuildFiles(data, result, options)
	case map[string]interface{}:
		// Check what keys are present to determine validation type
		if _, hasInstructions := v["instructions"]; hasInstructions {
			c.validateDockerfileContext(data, result, options)
		} else if _, hasFiles := v["files"]; hasFiles {
			c.validateBuildFiles(data, result, options)
		} else {
			result.AddError(core.NewValidationError(
				"UNKNOWN_CONTEXT_FORMAT",
				"Unable to determine context validation type from data",
				core.ErrTypeValidation,
				core.SeverityMedium,
			))
		}
	default:
		result.AddError(core.NewValidationError(
			"UNSUPPORTED_CONTEXT_TYPE",
			fmt.Sprintf("Unsupported context data type: %T", data),
			core.ErrTypeValidation,
			core.SeverityHigh,
		))
	}
}

// Helper methods

func (c *ContextValidator) isPathOutsideContext(path, contextPath string) bool {
	// Resolve the path relative to context
	cleaned := filepath.Clean(path)
	if filepath.IsAbs(cleaned) {
		return true
	}

	// Check if the resolved path escapes the context
	resolved := filepath.Join(contextPath, cleaned)
	rel, err := filepath.Rel(contextPath, resolved)
	if err != nil {
		return true
	}

	return strings.HasPrefix(rel, "..")
}

func (c *ContextValidator) checkSensitiveFiles(path string, line int, result *core.ValidationResult) {
	sensitivePatterns := []string{
		".git", ".env", ".aws", ".ssh", "id_rsa", "id_dsa",
		".npmrc", ".pypirc", ".netrc", ".gitconfig",
		"*.pem", "*.key", "*.p12", "*.pfx",
		"password", "passwd", "shadow", "credentials",
	}

	lowercasePath := strings.ToLower(path)
	for _, pattern := range sensitivePatterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(lowercasePath)); matched {
			warning := core.NewValidationWarning(
				"SENSITIVE_FILE_COPY",
				fmt.Sprintf("Copying potentially sensitive file: %s", path),
			)
			warning.ValidationError.WithLine(line).WithSuggestion("Ensure this file should be included in the image")
			result.AddWarning(warning)
			break
		}
	}
}

func (c *ContextValidator) isValidStageOrImageReference(ref string) bool {
	// Check if it's a numeric stage reference
	if _, err := strconv.Atoi(ref); err == nil {
		return true
	}

	// Check if it's a valid stage name (alphanumeric, hyphen, underscore)
	stageNameRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if stageNameRegex.MatchString(ref) {
		return true
	}

	// Check if it's a valid image reference
	imageRegex := regexp.MustCompile(`^([a-z0-9\-._]+\/)?[a-z0-9\-._\/]+(?::[a-zA-Z0-9\-._]+)?(?:@sha256:[a-f0-9]{64})?$`)
	return imageRegex.MatchString(ref)
}

func (c *ContextValidator) validateAddInstruction(instruction ContextInstruction, result *core.ValidationResult) {
	source := instruction.Source

	// Check if ADD is being used appropriately
	if !c.needsAddInstruction(source) {
		warning := core.NewValidationWarning(
			"UNNECESSARY_ADD",
			fmt.Sprintf("ADD is not needed for '%s'. Use COPY for simple file operations", source),
		)
		warning.ValidationError.WithLine(instruction.Line)
		result.AddWarning(warning)
	}

	// Warn about remote URLs
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		warning := core.NewValidationWarning(
			"ADD_REMOTE_URL",
			"Using ADD with remote URLs can be unpredictable. Consider using RUN with curl/wget for better control",
		)
		warning.ValidationError.WithLine(instruction.Line)
		result.AddWarning(warning)

		// Check for insecure HTTP
		if strings.HasPrefix(source, "http://") {
			result.AddError(core.NewValidationError(
				"INSECURE_ADD_URL",
				"Using ADD with HTTP (non-HTTPS) URL is insecure",
				core.ErrTypeSecurity,
				core.SeverityHigh,
			).WithLine(instruction.Line))
		}
	}
}

func (c *ContextValidator) needsAddInstruction(source string) bool {
	// ADD is needed for:
	// 1. Extracting tar archives
	// 2. Downloading from URLs
	return strings.HasSuffix(source, ".tar") ||
		strings.HasSuffix(source, ".tar.gz") ||
		strings.HasSuffix(source, ".tar.bz2") ||
		strings.HasSuffix(source, ".tar.xz") ||
		strings.HasPrefix(source, "http://") ||
		strings.HasPrefix(source, "https://")
}

func (c *ContextValidator) isSensitiveFile(path string) bool {
	sensitiveFiles := []string{
		".env", ".env.local", ".env.production", ".env.development",
		".git/config", ".ssh/", ".aws/credentials", ".aws/config",
		".npmrc", ".pypirc", ".netrc", ".gitconfig",
		"id_rsa", "id_dsa", "id_ecdsa", "id_ed25519",
		".pgpass", ".my.cnf", ".bash_history",
		"*.pem", "*.key", "*.p12", "*.pfx", "*.jks",
		"passwords.txt", "credentials.json", "secrets.yaml",
	}

	lowercasePath := strings.ToLower(path)
	for _, sensitive := range sensitiveFiles {
		if matched, _ := filepath.Match(sensitive, lowercasePath); matched {
			return true
		}
		if strings.Contains(lowercasePath, sensitive) {
			return true
		}
	}

	return false
}

// ValidatePath validates a single path
func (p *PathValidator) ValidatePath(path string, result *core.ValidationResult) {
	if path == "" {
		result.AddError(core.NewValidationError(
			"EMPTY_PATH",
			"Path cannot be empty",
			core.ErrTypeValidation,
			core.SeverityHigh,
		))
		return
	}

	// Check for null bytes
	if strings.Contains(path, "\x00") {
		result.AddError(core.NewValidationError(
			"NULL_IN_PATH",
			"Path contains null bytes",
			core.ErrTypeSecurity,
			core.SeverityHigh,
		))
		return
	}

	// Check for suspicious patterns
	suspiciousPatterns := []string{
		"../..",
		"/../",
		"/..",
		"..\\",
		"\\..\\",
		"..\\..\\",
	}

	for _, pattern := range suspiciousPatterns {
		if strings.Contains(path, pattern) {
			result.AddError(core.NewValidationError(
				"SUSPICIOUS_PATH_PATTERN",
				fmt.Sprintf("Path contains suspicious pattern: %s", pattern),
				core.ErrTypeSecurity,
				core.SeverityHigh,
			))
		}
	}

	// Validate path length
	if len(path) > 4096 {
		result.AddError(core.NewValidationError(
			"PATH_TOO_LONG",
			"Path exceeds maximum length of 4096 characters",
			core.ErrTypeValidation,
			core.SeverityMedium,
		))
	}
}

// Helper functions

func getStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getIntFromMap(m map[string]interface{}, key string) int {
	if v, ok := m[key].(int); ok {
		return v
	}
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}
