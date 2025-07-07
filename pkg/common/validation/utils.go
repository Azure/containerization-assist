package validation

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Azure/container-kit/pkg/common/validation-core/core"
)

// Common validation utility functions that can be used across all tool validators

// ValidateDockerImageName validates a Docker image name according to Docker naming conventions
func ValidateDockerImageName(imageName string) *ValidationError {
	if imageName == "" {
		return NewRequiredFieldError("image_name")
	}

	// Docker image name validation regex
	// Allows: registry.com/namespace/image:tag, namespace/image:tag, image:tag, image
	imageNameRegex := regexp.MustCompile(`^([a-z0-9]+([._-][a-z0-9]+)*(/[a-z0-9]+([._-][a-z0-9]+)*)*)?(/[a-z0-9]+([._-][a-z0-9]+)*)*(:[a-zA-Z0-9][a-zA-Z0-9._-]*)?$`)

	if !imageNameRegex.MatchString(imageName) {
		return NewInvalidFormatError("image_name", "valid Docker image name")
	}

	return nil
}

// ValidateKubernetesResourceName validates a Kubernetes resource name
func ValidateKubernetesResourceName(name string) *ValidationError {
	if name == "" {
		return NewRequiredFieldError("name")
	}

	// Kubernetes naming conventions: lowercase letters, numbers, and hyphens
	// Must start and end with alphanumeric character
	k8sNameRegex := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

	if !k8sNameRegex.MatchString(name) {
		return NewInvalidFormatError("name", "valid Kubernetes resource name (lowercase alphanumeric with hyphens)")
	}

	// Length constraints
	if len(name) > 253 {
		return NewInvalidValueError("name", name, "maximum length is 253 characters")
	}

	return nil
}

// ValidateKubernetesNamespace validates a Kubernetes namespace name
func ValidateKubernetesNamespace(namespace string) *ValidationError {
	if namespace == "" {
		return NewRequiredFieldError("namespace")
	}

	// Same rules as resource names but with additional restrictions
	if err := ValidateKubernetesResourceName(namespace); err != nil {
		err.Field = "namespace"
		return err
	}

	// Reserved namespaces
	reservedNamespaces := []string{"kube-system", "kube-public", "kube-node-lease"}
	for _, reserved := range reservedNamespaces {
		if namespace == reserved {
			return NewInvalidValueError("namespace", namespace, "cannot use reserved system namespace")
		}
	}

	return nil
}

// ValidateFilePath validates that a file path is valid and safe
func ValidateFilePath(fieldName, path string, mustExist bool) *ValidationError {
	if path == "" {
		return NewRequiredFieldError(fieldName)
	}

	// Clean the path to resolve any relative components
	cleanPath := filepath.Clean(path)

	// Check for directory traversal attempts
	if strings.Contains(cleanPath, "..") {
		return NewInvalidValueError(fieldName, path, "path traversal not allowed")
	}

	// For now, we'll skip the file existence check since it requires filesystem access
	// In a real implementation, you'd check if mustExist && !fileExists(cleanPath)

	return nil
}

// ValidateRepositoryPath validates a repository path for analysis tools
func ValidateRepositoryPath(path string) *ValidationError {
	if path == "" {
		return NewRequiredFieldError("repository_path")
	}

	// Must be absolute path
	if !filepath.IsAbs(path) {
		return NewInvalidFormatError("repository_path", "absolute path")
	}

	// Validate as safe file path
	if err := ValidateFilePath("repository_path", path, false); err != nil {
		return err
	}

	return nil
}

// ValidateDockerfilePath validates a Dockerfile path
func ValidateDockerfilePath(path string) *ValidationError {
	if path == "" {
		// Dockerfile path is optional, defaults to ./Dockerfile
		return nil
	}

	if err := ValidateFilePath("dockerfile_path", path, false); err != nil {
		return err
	}

	// Check if it looks like a Dockerfile
	fileName := filepath.Base(path)
	if !strings.HasPrefix(strings.ToLower(fileName), "dockerfile") {
		return NewInvalidValueError("dockerfile_path", path, "file should be named 'Dockerfile' or start with 'dockerfile'")
	}

	return nil
}

// ValidatePort validates a network port number
func ValidatePort(fieldName string, port int) *ValidationError {
	if port <= 0 || port > 65535 {
		return NewInvalidValueError(fieldName, port, "must be between 1 and 65535")
	}

	// Check for privileged ports (optional warning)
	if port < 1024 {
		// This could be a warning rather than an error
		return NewInvalidValueError(fieldName, port, "port numbers below 1024 require elevated privileges")
	}

	return nil
}

// ValidateURL validates a URL format
func ValidateURL(fieldName, url string) *ValidationError {
	if url == "" {
		return NewRequiredFieldError(fieldName)
	}

	// Basic URL validation regex
	urlRegex := regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)

	if !urlRegex.MatchString(url) {
		return NewInvalidFormatError(fieldName, "valid HTTP/HTTPS URL")
	}

	return nil
}

// ValidateEmailAddress validates an email address format
func ValidateEmailAddress(fieldName, email string) *ValidationError {
	if email == "" {
		return NewRequiredFieldError(fieldName)
	}

	// Basic email validation regex
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

	if !emailRegex.MatchString(email) {
		return NewInvalidFormatError(fieldName, "valid email address")
	}

	return nil
}

// ValidateEnvironmentVariableName validates environment variable naming
func ValidateEnvironmentVariableName(varName string) *ValidationError {
	if varName == "" {
		return NewRequiredFieldError("env_var_name")
	}

	// Environment variable naming conventions
	envVarRegex := regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

	if !envVarRegex.MatchString(varName) {
		return NewInvalidFormatError("env_var_name", "uppercase letters, numbers, and underscores")
	}

	return nil
}

// BatchValidate runs multiple validation functions and collects all errors
func BatchValidate(validations ...func() *ValidationError) []*ValidationError {
	var errors []*ValidationError

	for _, validate := range validations {
		if err := validate(); err != nil {
			errors = append(errors, err)
		}
	}

	return errors
}

// ApplyValidationErrors adds multiple validation errors to a result
func ApplyValidationErrors[T any](result *core.Result[T], errors []*ValidationError) {
	for _, err := range errors {
		coreErr := core.NewError(err.Code, err.Message, core.ErrTypeValidation, core.SeverityMedium)
		if err.Field != "" {
			coreErr.WithField(err.Field)
		}
		if err.Value != nil {
			coreErr.WithContext("invalid_value", err.Value)
		}
		result.AddError(coreErr)
	}
}

// ValidateStruct performs reflection-based validation of struct fields (simplified version)
// In a full implementation, this would use struct tags for validation rules
func ValidateStruct(ctx context.Context, structValue interface{}) []*ValidationError {
	// This is a placeholder for struct validation
	// A full implementation would use reflection to inspect struct fields
	// and apply validation rules based on struct tags
	var errors []*ValidationError

	// For now, return empty slice
	// Real implementation would inspect the struct and validate based on tags like:
	// `validate:"required,min=1,max=100"`

	return errors
}

// StandardToolInputValidations provides common validations that most tools need
type StandardToolInputValidations struct {
	RequireRepositoryPath bool
	RequireDockerfile     bool
	RequireImageName      bool
	RequireNamespace      bool
	AllowedNamespaces     []string
}

// ValidateStandardToolInput performs common tool input validations
func ValidateStandardToolInput(input interface{}, config StandardToolInputValidations) []*ValidationError {
	var errors []*ValidationError

	// This would typically use reflection to extract common fields
	// For now, we'll provide a pattern that tools can follow

	// Example pattern for tools to implement:
	// if config.RequireRepositoryPath {
	//     if err := ValidateRepositoryPath(input.RepositoryPath); err != nil {
	//         errors = append(errors, err)
	//     }
	// }

	return errors
}
