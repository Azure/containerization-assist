package validation

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors/codes"
	"sigs.k8s.io/yaml"
)

// ConfigValidators consolidates all Configuration validation logic
// Replaces: config validation scattered across multiple files, unified_validator.go parts
type ConfigValidators struct{}

// NewConfigValidators creates a new Config validator
func NewConfigValidators() *ConfigValidators {
	return &ConfigValidators{}
}

// ValidateFilePath validates file paths and extensions
func (cv *ConfigValidators) ValidateFilePath(path string) error {
	if path == "" {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("file path cannot be empty").
			Build()
	}

	// Check for path traversal attempts
	if strings.Contains(path, "..") {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeSecurity).
			Messagef("path traversal detected in file path: %s", path).
			Build()
	}

	// Validate file extension for known config types
	ext := strings.ToLower(filepath.Ext(path))
	validExtensions := map[string]bool{
		".yaml": true, ".yml": true, ".json": true, ".toml": true,
		".env": true, ".conf": true, ".config": true, ".ini": true,
	}

	if ext != "" && !validExtensions[ext] {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("unsupported config file extension: %s", ext).
			Build()
	}

	return nil
}

// ValidateJSON validates JSON format and structure
func (cv *ConfigValidators) ValidateJSON(jsonStr string) error {
	if jsonStr == "" {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("JSON content cannot be empty").
			Build()
	}

	var jsonData interface{}
	if err := json.Unmarshal([]byte(jsonStr), &jsonData); err != nil {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("invalid JSON format: %s", err.Error()).
			Build()
	}

	return nil
}

// ValidateYAML validates YAML format and structure
func (cv *ConfigValidators) ValidateYAML(yamlStr string) error {
	if yamlStr == "" {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("YAML content cannot be empty").
			Build()
	}

	var yamlData interface{}
	if err := yaml.Unmarshal([]byte(yamlStr), &yamlData); err != nil {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("invalid YAML format: %s", err.Error()).
			Build()
	}

	return nil
}

// ValidateEnvironmentVariable validates environment variable names and values
func (cv *ConfigValidators) ValidateEnvironmentVariable(name, value string) error {
	if name == "" {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("environment variable name cannot be empty").
			Build()
	}

	// Environment variable name validation (POSIX compliant)
	validEnvName := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	if !validEnvName.MatchString(name) {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("invalid environment variable name format: %s", name).
			Build()
	}

	// Check for sensitive variable patterns
	sensitivePatterns := []string{
		"PASSWORD", "SECRET", "KEY", "TOKEN", "CREDENTIAL",
		"API_KEY", "AUTH", "PRIVATE", "CERTIFICATE",
	}

	upperName := strings.ToUpper(name)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(upperName, pattern) {
			// Don't validate the actual value for sensitive vars
			return nil
		}
	}

	return nil
}

// ValidateConfigMap validates Kubernetes ConfigMap structures
func (cv *ConfigValidators) ValidateConfigMap(configMap map[string]interface{}) error {
	// Validate metadata
	metadata, ok := configMap["metadata"].(map[string]interface{})
	if !ok {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("configMap metadata is required").
			Build()
	}

	// Validate name
	if name, exists := metadata["name"]; exists {
		if nameStr, ok := name.(string); ok {
			if err := cv.validateConfigMapName(nameStr); err != nil {
				return err
			}
		}
	} else {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("configMap name is required").
			Build()
	}

	// Validate data section
	if data, exists := configMap["data"]; exists {
		if dataMap, ok := data.(map[string]interface{}); ok {
			for key, value := range dataMap {
				if err := cv.validateConfigMapData(key, value); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// ValidateSecret validates Kubernetes Secret structures
func (cv *ConfigValidators) ValidateSecret(secret map[string]interface{}) error {
	// Validate metadata
	metadata, ok := secret["metadata"].(map[string]interface{})
	if !ok {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("secret metadata is required").
			Build()
	}

	// Validate name
	if name, exists := metadata["name"]; exists {
		if nameStr, ok := name.(string); ok {
			if err := cv.validateSecretName(nameStr); err != nil {
				return err
			}
		}
	} else {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("secret name is required").
			Build()
	}

	// Validate type
	if secretType, exists := secret["type"]; exists {
		if typeStr, ok := secretType.(string); ok {
			if err := cv.validateSecretType(typeStr); err != nil {
				return err
			}
		}
	}

	return nil
}

// ValidateResourceLimits validates resource limit configurations
func (cv *ConfigValidators) ValidateResourceLimits(limits map[string]interface{}) error {
	validResources := map[string]bool{
		"cpu": true, "memory": true, "storage": true,
		"nvidia.com/gpu": true, "ephemeral-storage": true,
	}

	for resource, limit := range limits {
		if !validResources[resource] {
			return errors.NewError().
				Code(codes.VALIDATION_FAILED).
				Type(errors.ErrTypeValidation).
				Messagef("invalid resource type: %s", resource).
				Build()
		}

		if err := cv.validateResourceValue(resource, limit); err != nil {
			return err
		}
	}

	return nil
}

// validateConfigMapName validates ConfigMap names
func (cv *ConfigValidators) validateConfigMapName(name string) error {
	if len(name) > 253 {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("configMap name too long (max 253 chars): %s", name).
			Build()
	}

	validName := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	if !validName.MatchString(name) {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("invalid configMap name format: %s", name).
			Build()
	}

	return nil
}

// validateConfigMapData validates ConfigMap data entries
func (cv *ConfigValidators) validateConfigMapData(key string, value interface{}) error {
	// Key validation
	if key == "" {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("configMap data key cannot be empty").
			Build()
	}

	// Value should be string
	if _, ok := value.(string); !ok {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("configMap data value must be string for key: %s", key).
			Build()
	}

	return nil
}

// validateSecretName validates Secret names
func (cv *ConfigValidators) validateSecretName(name string) error {
	if len(name) > 253 {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("secret name too long (max 253 chars): %s", name).
			Build()
	}

	validName := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	if !validName.MatchString(name) {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("invalid secret name format: %s", name).
			Build()
	}

	return nil
}

// validateSecretType validates Secret types
func (cv *ConfigValidators) validateSecretType(secretType string) error {
	validTypes := []string{
		"Opaque",
		"kubernetes.io/service-account-token",
		"kubernetes.io/dockercfg",
		"kubernetes.io/dockerconfigjson",
		"kubernetes.io/basic-auth",
		"kubernetes.io/ssh-auth",
		"kubernetes.io/tls",
		"bootstrap.kubernetes.io/token",
	}

	for _, validType := range validTypes {
		if secretType == validType {
			return nil
		}
	}

	return errors.NewError().
		Code(codes.VALIDATION_FAILED).
		Type(errors.ErrTypeValidation).
		Messagef("invalid secret type: %s", secretType).
		Build()
}

// validateResourceValue validates resource limit values
func (cv *ConfigValidators) validateResourceValue(resource string, value interface{}) error {
	valueStr, ok := value.(string)
	if !ok {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Type(errors.ErrTypeValidation).
			Messagef("resource limit value must be string for %s", resource).
			Build()
	}

	// Basic validation for resource formats
	switch resource {
	case "cpu":
		// CPU can be in millicores (100m) or cores (0.1, 1)
		cpuPattern := regexp.MustCompile(`^(\d+(\.\d+)?|\d+m)$`)
		if !cpuPattern.MatchString(valueStr) {
			return errors.NewError().
				Code(codes.VALIDATION_FAILED).
				Type(errors.ErrTypeValidation).
				Messagef("invalid CPU resource format: %s", valueStr).
				Build()
		}
	case "memory":
		// Memory can be in bytes, Ki, Mi, Gi, etc.
		memoryPattern := regexp.MustCompile(`^(\d+(\.\d+)?([KMGTPE]i?)?|\d+[kmgtpe])$`)
		if !memoryPattern.MatchString(valueStr) {
			return errors.NewError().
				Code(codes.VALIDATION_FAILED).
				Type(errors.ErrTypeValidation).
				Messagef("invalid memory resource format: %s", valueStr).
				Build()
		}
	}

	return nil
}
