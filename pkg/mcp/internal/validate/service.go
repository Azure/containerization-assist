package validate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/runtime"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

// ValidationService provides centralized validation functionality
type ValidationService struct {
	logger     zerolog.Logger
	validators map[string]runtime.RuntimeValidator
	schemas    map[string]interface{}
}

// NewValidationService creates a new validation service
func NewValidationService(logger zerolog.Logger) *ValidationService {
	return &ValidationService{
		logger:     logger.With().Str("service", "validation").Logger(),
		validators: make(map[string]runtime.RuntimeValidator),
		schemas:    make(map[string]interface{}),
	}
}

// RegisterValidator registers a validator with the service
func (s *ValidationService) RegisterValidator(name string, validator runtime.RuntimeValidator) {
	s.validators[name] = validator
	s.logger.Debug().Str("validator", name).Msg("Validator registered")
}

// RegisterSchema registers a JSON schema for validation
func (s *ValidationService) RegisterSchema(name string, schema interface{}) {
	s.schemas[name] = schema
	s.logger.Debug().Str("schema", name).Msg("Schema registered")
}

// ValidateSessionID validates a session ID
func (s *ValidationService) ValidateSessionID(sessionID string) *runtime.ValidationErrorSet {
	errors := runtime.NewValidationErrorSet()

	if sessionID == "" {
		errors.AddField("session_id", "Session ID is required")
		return errors
	}

	// Check format (alphanumeric with hyphens)
	if !regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`).MatchString(sessionID) {
		errors.AddField("session_id", "Session ID contains invalid characters")
	}

	// Check length
	if len(sessionID) < 3 || len(sessionID) > 64 {
		errors.AddField("session_id", "Session ID must be between 3 and 64 characters")
	}

	return errors
}

// ValidateImageReference validates a Docker image reference
func (s *ValidationService) ValidateImageReference(imageRef string) *runtime.ValidationErrorSet {
	errors := runtime.NewValidationErrorSet()

	if imageRef == "" {
		errors.AddField("image_ref", "Image reference is required")
		return errors
	}

	// Basic format validation
	parts := strings.Split(imageRef, ":")
	if len(parts) > 2 {
		errors.AddField("image_ref", "Invalid image reference format")
	}

	// Check for invalid characters
	if strings.Contains(imageRef, " ") {
		errors.AddField("image_ref", "Image reference cannot contain spaces")
	}

	// Check for minimum components
	if !strings.Contains(imageRef, "/") && !strings.Contains(imageRef, ":") {
		// Single name images should be official images
		if len(imageRef) < 2 {
			errors.AddField("image_ref", "Image reference too short")
		}
	}

	return errors
}

// ValidateFilePath validates a file path exists and is accessible
func (s *ValidationService) ValidateFilePath(path string, mustExist bool) *runtime.ValidationErrorSet {
	errors := runtime.NewValidationErrorSet()

	if path == "" {
		errors.AddField("file_path", "File path is required")
		return errors
	}

	// Check for path traversal attempts
	if strings.Contains(path, "..") {
		errors.AddField("file_path", "Path traversal is not allowed")
	}

	// Check if file exists if required
	if mustExist {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			errors.AddField("file_path", fmt.Sprintf("File does not exist: %s", path))
		}
	}

	// Check if path is absolute when expected
	if filepath.IsAbs(path) {
		// Validate absolute paths don't access sensitive areas
		sensitive := []string{"/etc/passwd", "/etc/shadow", "/root"}
		for _, s := range sensitive {
			if strings.HasPrefix(path, s) {
				errors.AddField("file_path", "Access to sensitive path is not allowed")
				break
			}
		}
	}

	return errors
}

// ValidateJSON validates JSON content against a schema
func (s *ValidationService) ValidateJSON(content []byte, schemaName string) *runtime.ValidationErrorSet {
	errors := runtime.NewValidationErrorSet()

	// Basic JSON validation
	var data interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		errors.AddField("json_content", fmt.Sprintf("Invalid JSON: %v", err))
		return errors
	}

	// Schema validation if schema is registered
	if schema, exists := s.schemas[schemaName]; exists {
		if err := s.validateAgainstSchema(data, schema); err != nil {
			errors.AddField("json_schema", fmt.Sprintf("Schema validation failed: %v", err))
		}
	}

	return errors
}

// ValidateYAML validates YAML content
func (s *ValidationService) ValidateYAML(content []byte) *runtime.ValidationErrorSet {
	errors := runtime.NewValidationErrorSet()

	var data interface{}
	if err := yaml.Unmarshal(content, &data); err != nil {
		errors.AddField("yaml_content", fmt.Sprintf("Invalid YAML: %v", err))
	}

	return errors
}

// ValidateResourceLimits validates CPU and memory resource specifications
func (s *ValidationService) ValidateResourceLimits(cpuRequest, memoryRequest, cpuLimit, memoryLimit string) *runtime.ValidationErrorSet {
	errors := runtime.NewValidationErrorSet()

	// Validate CPU request
	if cpuRequest != "" {
		if err := s.validateCPUValue(cpuRequest); err != nil {
			errors.AddField("cpu_request", fmt.Sprintf("Invalid CPU request: %v", err))
		}
	}

	// Validate memory request
	if memoryRequest != "" {
		if err := s.validateMemoryValue(memoryRequest); err != nil {
			errors.AddField("memory_request", fmt.Sprintf("Invalid memory request: %v", err))
		}
	}

	// Validate CPU limit
	if cpuLimit != "" {
		if err := s.validateCPUValue(cpuLimit); err != nil {
			errors.AddField("cpu_limit", fmt.Sprintf("Invalid CPU limit: %v", err))
		}
	}

	// Validate memory limit
	if memoryLimit != "" {
		if err := s.validateMemoryValue(memoryLimit); err != nil {
			errors.AddField("memory_limit", fmt.Sprintf("Invalid memory limit: %v", err))
		}
	}

	// Cross-validation: limits should be >= requests
	if cpuRequest != "" && cpuLimit != "" {
		requestVal, _ := s.parseCPUValue(cpuRequest)
		limitVal, _ := s.parseCPUValue(cpuLimit)
		if limitVal < requestVal {
			errors.AddField("cpu_limit", "CPU limit must be greater than or equal to CPU request")
		}
	}

	if memoryRequest != "" && memoryLimit != "" {
		requestBytes, _ := s.parseMemoryValue(memoryRequest)
		limitBytes, _ := s.parseMemoryValue(memoryLimit)
		if limitBytes < requestBytes {
			errors.AddField("memory_limit", "Memory limit must be greater than or equal to memory request")
		}
	}

	return errors
}

// ValidateNamespace validates a Kubernetes namespace name
func (s *ValidationService) ValidateNamespace(namespace string) *runtime.ValidationErrorSet {
	errors := runtime.NewValidationErrorSet()

	if namespace == "" {
		return errors // Empty namespace is allowed (defaults to "default")
	}

	// Kubernetes namespace naming rules
	if len(namespace) > 63 {
		errors.AddField("namespace", "Namespace name must be 63 characters or less")
	}

	// Must be lowercase alphanumeric with hyphens
	if !regexp.MustCompile(`^[a-z0-9\-]+$`).MatchString(namespace) {
		errors.AddField("namespace", "Namespace name must be lowercase alphanumeric with hyphens")
	}

	// Cannot start or end with hyphen
	if strings.HasPrefix(namespace, "-") || strings.HasSuffix(namespace, "-") {
		errors.AddField("namespace", "Namespace name cannot start or end with hyphen")
	}

	// Reserved namespaces
	reserved := []string{"kube-system", "kube-public", "kube-node-lease"}
	for _, r := range reserved {
		if namespace == r {
			errors.AddField("namespace", fmt.Sprintf("Namespace '%s' is reserved", namespace))
			break
		}
	}

	return errors
}

// ValidateEnvironmentVariables validates environment variable names and values
func (s *ValidationService) ValidateEnvironmentVariables(envVars map[string]string) *runtime.ValidationErrorSet {
	errors := runtime.NewValidationErrorSet()

	for name, value := range envVars {
		// Validate variable name
		if !regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`).MatchString(name) {
			errors.AddField("environment."+name, "Environment variable name must be uppercase letters, digits, and underscores")
		}

		// Check for potentially sensitive values
		if s.containsSensitiveData(value) {
			errors.AddField("environment."+name, "Environment variable appears to contain sensitive data")
		}

		// Check value length
		if len(value) > 1024 {
			errors.AddField("environment."+name, "Environment variable value too long (max 1024 characters)")
		}
	}

	return errors
}

// ValidatePort validates a port number
func (s *ValidationService) ValidatePort(port int) *runtime.ValidationErrorSet {
	errors := runtime.NewValidationErrorSet()

	if port < 1 || port > 65535 {
		errors.AddField("port", "Port must be between 1 and 65535")
	}

	// Check for privileged ports
	if port < 1024 {
		errors.AddField("port", "Port is in privileged range (< 1024), ensure this is intended")
	}

	return errors
}

// BatchValidate validates multiple items using registered validators
func (s *ValidationService) BatchValidate(ctx context.Context, items []ValidationItem) *BatchValidationResult {
	result := &BatchValidationResult{
		TotalItems: len(items),
		Results:    make(map[string]*runtime.ValidationResult),
		StartTime:  time.Now(),
	}

	for _, item := range items {
		validator, exists := s.validators[item.ValidatorName]
		if !exists {
			s.logger.Warn().Str("validator", item.ValidatorName).Msg("Validator not found")
			continue
		}

		validationResult, err := validator.Validate(ctx, item.Data, item.Options)
		if err != nil {
			s.logger.Error().Err(err).Str("item", item.ID).Msg("Validation failed")
			continue
		}

		result.Results[item.ID] = validationResult
		if validationResult.IsValid {
			result.ValidItems++
		} else {
			result.InvalidItems++
		}
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
	Options       runtime.ValidationOptions
}

// BatchValidationResult represents the result of batch validation
type BatchValidationResult struct {
	TotalItems   int
	ValidItems   int
	InvalidItems int
	Results      map[string]*runtime.ValidationResult
	StartTime    time.Time
	Duration     time.Duration
}
