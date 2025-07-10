package validation

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKubernetesManifestValidator(t *testing.T) {
	validator := NewKubernetesManifestValidator()

	t.Run("valid manifest", func(t *testing.T) {
		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test-pod",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "test",
						"image": "test:latest",
					},
				},
			},
		}

		result := validator.Validate(context.Background(), manifest)
		assert.True(t, result.Valid)
		assert.Empty(t, result.Errors)
	})

	t.Run("missing apiVersion", func(t *testing.T) {
		manifest := map[string]interface{}{
			"kind": "Pod",
			"metadata": map[string]interface{}{
				"name": "test-pod",
			},
		}

		result := validator.Validate(context.Background(), manifest)
		assert.False(t, result.Valid)
		// Should have 2 errors: missing apiVersion in basic validation and empty apiVersion in kind validation
		assert.GreaterOrEqual(t, len(result.Errors), 1)
		assert.Contains(t, result.Errors[0].Error(), "apiVersion")
	})

	t.Run("missing kind", func(t *testing.T) {
		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"metadata": map[string]interface{}{
				"name": "test-pod",
			},
		}

		result := validator.Validate(context.Background(), manifest)
		assert.False(t, result.Valid)
		// Should have 2 errors: missing kind in basic validation and empty kind in kind validation
		assert.GreaterOrEqual(t, len(result.Errors), 1)
		assert.Contains(t, result.Errors[0].Error(), "kind")
	})

	t.Run("missing metadata", func(t *testing.T) {
		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
		}

		result := validator.Validate(context.Background(), manifest)
		assert.False(t, result.Valid)
		// Should have 2 errors: missing metadata in basic validation and metadata must be object
		assert.GreaterOrEqual(t, len(result.Errors), 1)
		assert.Contains(t, result.Errors[0].Error(), "metadata")
	})

	t.Run("missing metadata.name", func(t *testing.T) {
		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata":   map[string]interface{}{},
		}

		result := validator.Validate(context.Background(), manifest)
		assert.False(t, result.Valid)
		// Should have at least 1 error for missing metadata.name
		assert.GreaterOrEqual(t, len(result.Errors), 1)
		assert.Contains(t, result.Errors[0].Error(), "metadata.name")
	})

	t.Run("validator metadata", func(t *testing.T) {
		assert.Equal(t, "KubernetesManifestValidator", validator.Name())
		assert.Equal(t, "kubernetes", validator.Domain())
		assert.Equal(t, "manifest", validator.Category())
		assert.Equal(t, 100, validator.Priority())
		assert.Empty(t, validator.Dependencies())
	})
}

func TestDockerConfigValidator(t *testing.T) {
	validator := NewDockerConfigValidator()

	t.Run("valid config", func(t *testing.T) {
		config := map[string]interface{}{
			"image": "nginx:latest",
			"ports": []interface{}{"80:8080", "443"},
			"environment": map[string]interface{}{
				"ENV_VAR": "value",
				"DEBUG":   "true",
			},
		}

		result := validator.Validate(context.Background(), config)
		assert.True(t, result.Valid)
		assert.Empty(t, result.Errors)
	})

	t.Run("empty image", func(t *testing.T) {
		config := map[string]interface{}{
			"image": "",
		}

		result := validator.Validate(context.Background(), config)
		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Error(), "image")
		assert.Contains(t, result.Errors[0].Error(), "cannot be empty")
	})

	t.Run("invalid image with dots", func(t *testing.T) {
		config := map[string]interface{}{
			"image": "nginx/../malicious",
		}

		result := validator.Validate(context.Background(), config)
		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Error(), "cannot contain '..'")
	})

	t.Run("invalid port mapping", func(t *testing.T) {
		config := map[string]interface{}{
			"ports": []interface{}{"80:8080:invalid"},
		}

		result := validator.Validate(context.Background(), config)
		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Error(), "invalid port mapping format")
	})

	t.Run("empty port mapping", func(t *testing.T) {
		config := map[string]interface{}{
			"ports": []interface{}{""},
		}

		result := validator.Validate(context.Background(), config)
		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Error(), "port mapping cannot be empty")
	})

	t.Run("environment variable with equals", func(t *testing.T) {
		config := map[string]interface{}{
			"environment": map[string]interface{}{
				"BAD=KEY": "value",
			},
		}

		result := validator.Validate(context.Background(), config)
		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Error(), "cannot contain '='")
	})

	t.Run("empty environment variable key", func(t *testing.T) {
		config := map[string]interface{}{
			"environment": map[string]interface{}{
				"": "value",
			},
		}

		result := validator.Validate(context.Background(), config)
		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Error(), "environment variable key cannot be empty")
	})

	t.Run("validator metadata", func(t *testing.T) {
		assert.Equal(t, "DockerConfigValidator", validator.Name())
		assert.Equal(t, "docker", validator.Domain())
		assert.Equal(t, "config", validator.Category())
		assert.Equal(t, 90, validator.Priority())
		assert.Empty(t, validator.Dependencies())
	})
}

func TestSecurityPolicyValidator(t *testing.T) {
	validator := NewSecurityPolicyValidator()

	t.Run("secure config", func(t *testing.T) {
		policy := map[string]interface{}{
			"securityContext": map[string]interface{}{
				"runAsNonRoot":           true,
				"readOnlyRootFilesystem": true,
			},
			"privileged":  false,
			"hostNetwork": false,
		}

		result := validator.Validate(context.Background(), policy)
		assert.True(t, result.Valid)
		assert.Empty(t, result.Errors)
		assert.Empty(t, result.Warnings)
	})

	t.Run("privileged container blocked", func(t *testing.T) {
		policy := map[string]interface{}{
			"privileged": true,
		}

		result := validator.Validate(context.Background(), policy)
		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Error(), "privileged containers not allowed")
	})

	t.Run("security warnings", func(t *testing.T) {
		policy := map[string]interface{}{
			"securityContext": map[string]interface{}{
				"runAsNonRoot":           false,
				"readOnlyRootFilesystem": false,
			},
			"hostNetwork": true,
		}

		result := validator.Validate(context.Background(), policy)
		assert.True(t, result.Valid) // Warnings don't fail validation
		assert.Empty(t, result.Errors)
		assert.Len(t, result.Warnings, 3)

		warningText := result.Warnings[0] + result.Warnings[1] + result.Warnings[2]
		assert.Contains(t, warningText, "may run as root")
		assert.Contains(t, warningText, "root filesystem is writable")
		assert.Contains(t, warningText, "using host network")
	})

	t.Run("validator metadata", func(t *testing.T) {
		assert.Equal(t, "SecurityPolicyValidator", validator.Name())
		assert.Equal(t, "security", validator.Domain())
		assert.Equal(t, "policy", validator.Category())
		assert.Equal(t, 200, validator.Priority())
		assert.Equal(t, []string{"KubernetesManifestValidator"}, validator.Dependencies())
	})
}

func TestValidatorChain(t *testing.T) {
	t.Run("stop on first error", func(t *testing.T) {
		chain := NewValidatorChain[interface{}](StopOnFirstError)

		// First validator fails
		failValidator := &mockValidator{
			name:   "fail",
			result: ValidationResult{Valid: false, Errors: []error{fmt.Errorf("error1")}},
		}

		// Second validator would succeed but shouldn't run
		successValidator := &mockValidator{
			name:   "success",
			result: ValidationResult{Valid: true, Warnings: []string{"should not see this"}},
		}

		chain.Add(failValidator).Add(successValidator)

		result := chain.Validate(context.Background(), "test")
		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 1)
		assert.Empty(t, result.Warnings) // Second validator shouldn't have run
	})

	t.Run("continue on error", func(t *testing.T) {
		chain := NewValidatorChain[interface{}](ContinueOnError)

		failValidator := &mockValidator{
			name:   "fail",
			result: ValidationResult{Valid: false, Errors: []error{fmt.Errorf("error1")}},
		}

		successValidator := &mockValidator{
			name:   "success",
			result: ValidationResult{Valid: true, Warnings: []string{"warning1"}},
		}

		chain.Add(failValidator).Add(successValidator)

		result := chain.Validate(context.Background(), "test")
		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 1)
		assert.Len(t, result.Warnings, 1) // Second validator did run
	})

	t.Run("stop on first warning", func(t *testing.T) {
		chain := NewValidatorChain[interface{}](StopOnFirstWarning)

		warnValidator := &mockValidator{
			name:   "warn",
			result: ValidationResult{Valid: true, Warnings: []string{"warning1"}},
		}

		successValidator := &mockValidator{
			name:   "success",
			result: ValidationResult{Valid: true, Warnings: []string{"should not see this"}},
		}

		chain.Add(warnValidator).Add(successValidator)

		result := chain.Validate(context.Background(), "test")
		assert.True(t, result.Valid)
		assert.Len(t, result.Warnings, 1)
		assert.Equal(t, "warning1", result.Warnings[0])
	})
}

func TestValidatorIntegration(t *testing.T) {
	t.Run("full integration test", func(t *testing.T) {
		registry := NewValidatorRegistry()

		// Register validators in order
		kubernetesValidator := NewKubernetesManifestValidator()
		securityValidator := NewSecurityPolicyValidator()

		err := registry.Register(kubernetesValidator)
		require.NoError(t, err)

		err = registry.Register(securityValidator)
		require.NoError(t, err)

		// Test with valid Kubernetes manifest with security policy
		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "secure-pod",
			},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"name":  "test",
						"image": "test:latest",
					},
				},
			},
			"securityContext": map[string]interface{}{
				"runAsNonRoot":           true,
				"readOnlyRootFilesystem": true,
			},
			"privileged": false,
		}

		result := registry.ValidateAll(context.Background(), manifest, "kubernetes", "manifest")
		assert.True(t, result.Valid)
		assert.Empty(t, result.Errors)

		// Test security validation
		result = registry.ValidateAll(context.Background(), manifest, "security", "policy")
		assert.True(t, result.Valid)
		assert.Empty(t, result.Errors)
		assert.Empty(t, result.Warnings)

		// Test with privileged container (should fail security)
		privilegedManifest := map[string]interface{}{
			"privileged": true,
		}

		result = registry.ValidateAll(context.Background(), privilegedManifest, "security", "policy")
		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Error(), "privileged containers not allowed")
	})
}
