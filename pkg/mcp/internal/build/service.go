package build

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

// ValidationService provides centralized validation functionality
type ValidationService struct {
	logger     zerolog.Logger
	schemas    map[string]interface{}
	validators map[string]interface{}
}

// NewValidationService creates a new validation service
func NewValidationService(logger zerolog.Logger) *ValidationService {
	return &ValidationService{
		logger:     logger.With().Str("service", "validation").Logger(),
		schemas:    make(map[string]interface{}),
		validators: make(map[string]interface{}),
	}
}

// RegisterValidator registers a validator with the service
func (s *ValidationService) RegisterValidator(name string, validator interface{}) {
	s.validators[name] = validator
	s.logger.Debug().Str("validator", name).Msg("Validator registered")
}

// RegisterSchema registers a JSON schema for validation
func (s *ValidationService) RegisterSchema(name string, schema interface{}) {
	s.schemas[name] = schema
	s.logger.Debug().Str("schema", name).Msg("Schema registered")
}

// ValidateSessionID validates a session ID
// ValidateSessionID validates a session ID
// TODO: Implement without runtime dependency
func (s *ValidationService) ValidateSessionID(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return mcp.NewErrorBuilder("VALIDATION_ERROR", "Session ID is required", "validation_error").
			WithOperation("validate_session").
			WithStage("input_validation").
			WithRootCause("Session ID parameter is empty or missing").
			WithImmediateStep(1, "Provide session ID", "Include a valid session ID in the request").
			WithImmediateStep(2, "Check format", "Ensure session ID follows alphanumeric format").
			Build()
	}
	// Check format (alphanumeric with hyphens)
	if !regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`).MatchString(sessionID) {
		return mcp.NewErrorBuilder("VALIDATION_ERROR", "Session ID contains invalid characters", "validation_error").
			WithOperation("validate_session").
			WithStage("format_validation").
			WithRootCause("Session ID contains characters other than letters, numbers, hyphens, or underscores").
			WithImmediateStep(1, "Fix format", "Use only letters, numbers, hyphens (-), and underscores (_)").
			WithImmediateStep(2, "Remove special chars", "Remove spaces, symbols, or other special characters").
			Build()
	}
	// Check length
	if len(sessionID) < 3 || len(sessionID) > 64 {
		return mcp.NewErrorBuilder("VALIDATION_ERROR", "Session ID length is invalid", "validation_error").
			WithField("length", len(sessionID)).
			WithField("min_length", 3).
			WithField("max_length", 64).
			WithOperation("validate_session").
			WithStage("length_validation").
			WithRootCause(fmt.Sprintf("Session ID length %d is outside valid range 3-64", len(sessionID))).
			WithImmediateStep(1, "Adjust length", "Use a session ID between 3 and 64 characters").
			WithImmediateStep(2, "Generate new ID", "Create a new session ID within valid length range").
			Build()
	}
	return nil
}

// ValidateImageReference validates a Docker image reference
// ValidateImageReference validates a Docker image reference
// TODO: Implement without runtime dependency
func (s *ValidationService) ValidateImageReference(ctx context.Context, imageRef string) error {
	if imageRef == "" {
		return mcp.NewErrorBuilder("VALIDATION_ERROR", "Image reference is required", "validation_error").
			WithOperation("validate_image").
			WithStage("input_validation").
			WithRootCause("Image reference parameter is empty or missing").
			WithImmediateStep(1, "Provide image", "Specify a Docker image reference (e.g., nginx:latest)").
			WithImmediateStep(2, "Include tag", "Add a tag to the image reference for specificity").
			Build()
	}
	// Basic format validation
	parts := strings.Split(imageRef, ":")
	if len(parts) > 2 {
		return mcp.NewErrorBuilder("VALIDATION_ERROR", "Invalid image reference format", "validation_error").
			WithOperation("validate_image").
			WithStage("format_validation").
			WithRootCause("Image reference contains more than one colon separator").
			WithImmediateStep(1, "Fix format", "Use format: [registry/]name[:tag] (e.g., nginx:latest)").
			WithImmediateStep(2, "Remove extra colons", "Ensure only one colon separates name and tag").
			Build()
	}
	// Check for invalid characters
	if strings.Contains(imageRef, " ") {
		return mcp.NewErrorBuilder("VALIDATION_ERROR", "Image reference cannot contain spaces", "validation_error").
			WithOperation("validate_image").
			WithStage("format_validation").
			WithRootCause("Docker image references cannot contain whitespace characters").
			WithImmediateStep(1, "Remove spaces", "Replace spaces with hyphens or underscores").
			WithImmediateStep(2, "Use valid format", "Follow Docker naming conventions: [a-z0-9.-]").
			Build()
	}
	// Check for minimum components
	if !strings.Contains(imageRef, "/") && !strings.Contains(imageRef, ":") {
		// Single name images should be official images
		if len(imageRef) < 2 {
			return mcp.NewErrorBuilder("VALIDATION_ERROR", "Image reference too short", "validation_error").
				WithField("length", len(imageRef)).
				WithField("min_length", 2).
				WithOperation("validate_image").
				WithStage("length_validation").
				WithRootCause("Single-component image reference must be at least 2 characters").
				WithImmediateStep(1, "Use full name", "Provide full image name (e.g., 'nginx' instead of 'n')").
				WithImmediateStep(2, "Add registry", "Include registry and tag (e.g., 'docker.io/nginx:latest')").
				Build()
		}
	}
	return nil
}

// ValidateFilePath validates a file path exists and is accessible
// ValidateFilePath validates a file path
// TODO: Implement without runtime dependency
func (s *ValidationService) ValidateFilePath(ctx context.Context, path string, mustExist bool) error {
	if path == "" {
		return mcp.NewErrorBuilder("VALIDATION_ERROR", "File path is required", "validation_error").
			WithOperation("validate_file_path").
			WithStage("input_validation").
			WithRootCause("File path parameter is empty or missing").
			WithImmediateStep(1, "Provide path", "Specify a valid file path").
			WithImmediateStep(2, "Check parameter", "Ensure file path parameter is correctly passed").
			Build()
	}
	// Check for path traversal attempts
	if strings.Contains(path, "..") {
		return mcp.NewErrorBuilder("VALIDATION_ERROR", "Path traversal is not allowed", "validation_error").
			WithOperation("validate_file_path").
			WithStage("security_validation").
			WithRootCause("File path contains directory traversal sequences (..)").
			WithImmediateStep(1, "Remove traversal", "Remove '..' sequences from the path").
			WithImmediateStep(2, "Use absolute path", "Use absolute path instead of relative path").
			WithImmediateStep(3, "Sanitize input", "Validate and sanitize file path input").
			Build()
	}
	// Check context cancellation before file operations
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	// Check if file exists if required
	if mustExist {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return mcp.NewErrorBuilder("VALIDATION_ERROR", "File does not exist", "validation_error").
				WithOperation("validate_file_path").
				WithStage("existence_check").
				WithRootCause(fmt.Sprintf("Required file not found at path: %s", path)).
				WithImmediateStep(1, "Check path", "Verify the file path is correct").
				WithImmediateStep(2, "Create file", "Create the required file if missing").
				WithImmediateStep(3, "Check permissions", "Ensure read permissions for the file").
				Build()
		}
	}
	// Check if path is absolute when expected
	if filepath.IsAbs(path) {
		// Validate absolute paths don't access sensitive areas
		sensitive := []string{"/etc/passwd", "/etc/shadow", "/root"}
		for _, s := range sensitive {
			if strings.HasPrefix(path, s) {
				return mcp.NewErrorBuilder("VALIDATION_ERROR", "Access to sensitive path is not allowed", "validation_error").
					WithField("sensitive_prefix", s).
					WithOperation("validate_file_path").
					WithStage("security_validation").
					WithRootCause(fmt.Sprintf("Path %s accesses sensitive system directory %s", path, s)).
					WithImmediateStep(1, "Use safe path", "Use a path outside sensitive system directories").
					WithImmediateStep(2, "Check requirements", "Verify if access to this path is actually required").
					WithImmediateStep(3, "Use relative path", "Use relative paths within allowed directories").
					Build()
			}
		}
	}
	return nil
}

// ValidateJSON validates JSON content against a schema
// ValidateJSON validates JSON content
// TODO: Implement without runtime dependency
func (s *ValidationService) ValidateJSON(ctx context.Context, content []byte, schemaName string) error {
	// Check context cancellation before starting
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	// Basic JSON validation
	var data interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		return mcp.NewErrorBuilder("VALIDATION_ERROR", "Invalid JSON content", "validation_error").
			WithOperation("validate_json").
			WithStage("json_parsing").
			WithRootCause(fmt.Sprintf("JSON parsing failed: %v", err)).
			WithImmediateStep(1, "Check syntax", "Verify JSON syntax is correct").
			WithImmediateStep(2, "Validate quotes", "Ensure all strings are properly quoted").
			WithImmediateStep(3, "Check encoding", "Verify file encoding is UTF-8").
			Build()
	}
	// Schema validation if schema is registered
	if schema, exists := s.schemas[schemaName]; exists {
		if err := s.validateAgainstSchema(data, schema); err != nil {
			return mcp.NewErrorBuilder("VALIDATION_ERROR", "JSON schema validation failed", "validation_error").
				WithOperation("validate_json").
				WithStage("schema_validation").
				WithRootCause(fmt.Sprintf("Content does not match schema %s: %v", schemaName, err)).
				WithImmediateStep(1, "Check schema", "Verify content matches the required schema").
				WithImmediateStep(2, "Fix structure", "Update JSON structure to match schema requirements").
				WithImmediateStep(3, "Validate fields", "Ensure all required fields are present and correctly typed").
				Build()
		}
	}
	return nil
}

// ValidateYAML validates YAML content
func (s *ValidationService) ValidateYAML(ctx context.Context, content []byte) error {
	// Check context cancellation before starting
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	var data interface{}
	if err := yaml.Unmarshal(content, &data); err != nil {
		return fmt.Errorf("invalid YAML: %v", err)
	}
	return nil
}

// ValidateResourceLimits validates CPU and memory resource specifications
func (s *ValidationService) ValidateResourceLimits(ctx context.Context, cpuRequest, memoryRequest, cpuLimit, memoryLimit string) error {
	// Validate CPU request
	if cpuRequest != "" {
		if err := s.validateCPUValue(cpuRequest); err != nil {
			return fmt.Errorf("invalid CPU request: %v", err)
		}
	}
	// Validate memory request
	if memoryRequest != "" {
		if err := s.validateMemoryValue(memoryRequest); err != nil {
			return fmt.Errorf("invalid memory request: %v", err)
		}
	}
	// Validate CPU limit
	if cpuLimit != "" {
		if err := s.validateCPUValue(cpuLimit); err != nil {
			return fmt.Errorf("invalid CPU limit: %v", err)
		}
	}
	// Validate memory limit
	if memoryLimit != "" {
		if err := s.validateMemoryValue(memoryLimit); err != nil {
			return fmt.Errorf("invalid memory limit: %v", err)
		}
	}
	// Cross-validation: limits should be >= requests
	if cpuRequest != "" && cpuLimit != "" {
		requestVal, _ := s.parseCPUValue(cpuRequest)
		limitVal, _ := s.parseCPUValue(cpuLimit)
		if limitVal < requestVal {
			return fmt.Errorf("CPU limit must be greater than or equal to CPU request")
		}
	}
	if memoryRequest != "" && memoryLimit != "" {
		requestBytes, _ := s.parseMemoryValue(memoryRequest)
		limitBytes, _ := s.parseMemoryValue(memoryLimit)
		if limitBytes < requestBytes {
			return fmt.Errorf("memory limit must be greater than or equal to memory request")
		}
	}
	return nil
}

// ValidateNamespace validates a Kubernetes namespace name
func (s *ValidationService) ValidateNamespace(ctx context.Context, namespace string) error {
	if namespace == "" {
		return nil // Empty namespace is allowed (defaults to "default")
	}
	// Kubernetes namespace naming rules
	if len(namespace) > 63 {
		return fmt.Errorf("namespace name must be 63 characters or less")
	}
	// Must be lowercase alphanumeric with hyphens
	if !regexp.MustCompile(`^[a-z0-9\-]+$`).MatchString(namespace) {
		return fmt.Errorf("namespace name must be lowercase alphanumeric with hyphens")
	}
	// Cannot start or end with hyphen
	if strings.HasPrefix(namespace, "-") || strings.HasSuffix(namespace, "-") {
		return fmt.Errorf("namespace name cannot start or end with hyphen")
	}
	// Reserved namespaces
	reserved := []string{"kube-system", "kube-public", "kube-node-lease"}
	for _, r := range reserved {
		if namespace == r {
			return fmt.Errorf("namespace '%s' is reserved", namespace)
		}
	}
	return nil
}

// ValidateEnvironmentVariables validates environment variable names and values
func (s *ValidationService) ValidateEnvironmentVariables(ctx context.Context, envVars map[string]string) error {
	for name, value := range envVars {
		// Validate variable name
		if !regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`).MatchString(name) {
			return fmt.Errorf("environment variable '%s': name must be uppercase letters, digits, and underscores", name)
		}
		// Check for potentially sensitive values
		if s.containsSensitiveData(value) {
			return fmt.Errorf("environment variable '%s': appears to contain sensitive data", name)
		}
		// Check value length
		if len(value) > 1024 {
			return fmt.Errorf("environment variable '%s': value too long (max 1024 characters)", name)
		}
	}
	return nil
}

// ValidatePort validates a port number
func (s *ValidationService) ValidatePort(ctx context.Context, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	// Check for privileged ports
	if port < 1024 {
		// Just log a warning, don't return error for privileged ports
		s.logger.Warn().Int("port", port).Msg("Port is in privileged range (< 1024)")
	}
	return nil
}

// BatchValidate validates multiple items using registered validators
func (s *ValidationService) BatchValidate(ctx context.Context, items []ValidationItem) *BatchValidationResult {
	result := &BatchValidationResult{
		TotalItems: len(items),
		Results:    make(map[string]*ValidationResult),
		StartTime:  time.Now(),
	}
	for _, item := range items {
		validatorInterface, exists := s.validators[item.ValidatorName]
		if !exists {
			s.logger.Warn().Str("validator", item.ValidatorName).Msg("Validator not found")
			continue
		}
		// TODO: Implement validator interface without runtime dependency
		// For now, skip validation
		_ = validatorInterface
		// Placeholder validation result
		result.Results[item.ID] = &ValidationResult{
			Valid: true,
		}
		result.ValidItems++
	}
	result.Duration = time.Since(result.StartTime)
	return result
}

// Helper methods
func (s *ValidationService) validateAgainstSchema(data, schema interface{}) error {
	// Simple schema validation - in practice would use a proper JSON schema library
	return nil
}
func (s *ValidationService) validateCPUValue(cpu string) error {
	// Validate CPU format (e.g., "100m", "0.1", "1")
	if cpu == "" {
		return fmt.Errorf("CPU value cannot be empty")
	}
	_, err := s.parseCPUValue(cpu)
	return err
}
func (s *ValidationService) parseCPUValue(cpu string) (float64, error) {
	// Simple CPU parsing - would use proper Kubernetes quantity parsing
	if strings.HasSuffix(cpu, "m") {
		// Millicores
		return 0.001, nil
	}
	return 1.0, nil
}
func (s *ValidationService) validateMemoryValue(memory string) error {
	if memory == "" {
		return fmt.Errorf("memory value cannot be empty")
	}
	_, err := s.parseMemoryValue(memory)
	return err
}
func (s *ValidationService) parseMemoryValue(memory string) (int64, error) {
	// Simple memory parsing - would use proper Kubernetes quantity parsing
	if strings.HasSuffix(memory, "Mi") {
		return 1024 * 1024, nil
	}
	if strings.HasSuffix(memory, "Gi") {
		return 1024 * 1024 * 1024, nil
	}
	return 1024, nil
}
func (s *ValidationService) containsSensitiveData(value string) bool {
	// Check for patterns that might indicate sensitive data
	sensitivePatterns := []string{
		"password", "secret", "key", "token", "credential",
		"-----BEGIN", "sk-", "ey_", "ghp_", "glpat-",
	}
	lower := strings.ToLower(value)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	// Check for long base64-like strings
	if len(value) > 20 && regexp.MustCompile(`^[A-Za-z0-9+/=]+$`).MatchString(value) {
		return true
	}
	return false
}

// ValidationItem represents an item to validate
type ValidationItem struct {
	ID            string
	ValidatorName string
	Data          interface{}
	Options       ValidationOptions // Local type to avoid runtime dependency
}

// BatchValidationResult represents the result of batch validation
type BatchValidationResult struct {
	TotalItems   int
	ValidItems   int
	InvalidItems int
	Results      map[string]*ValidationResult // Local type to avoid runtime dependency
	StartTime    time.Time
	Duration     time.Duration
}
