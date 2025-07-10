package validation

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLegacyValidatorAdapter(t *testing.T) {
	validator := NewKubernetesManifestValidator()
	adapter := NewLegacyValidatorAdapter(validator)

	t.Run("valid manifest returns nil", func(t *testing.T) {
		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test-pod",
			},
		}

		err := adapter.ValidateOldInterface(manifest)
		assert.NoError(t, err)
	})

	t.Run("invalid manifest returns error", func(t *testing.T) {
		manifest := map[string]interface{}{
			"kind": "Pod", // Missing apiVersion
		}

		err := adapter.ValidateOldInterface(manifest)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "apiVersion")
	})

	t.Run("validate with context", func(t *testing.T) {
		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test-pod",
			},
		}

		ctx := context.Background()
		err := adapter.ValidateWithContext(ctx, manifest)
		assert.NoError(t, err)
	})
}

func TestSimpleFunctionAdapter(t *testing.T) {
	// Create a simple validation function
	validateNotEmpty := func(data interface{}) error {
		str, ok := data.(string)
		if !ok {
			return fmt.Errorf("expected string")
		}
		if str == "" {
			return fmt.Errorf("string cannot be empty")
		}
		return nil
	}

	adapter := NewSimpleFunctionAdapter("not-empty", "common", "string", 50, validateNotEmpty)

	t.Run("valid string passes", func(t *testing.T) {
		result := adapter.Validate(context.Background(), "hello")
		assert.True(t, result.Valid)
		assert.Empty(t, result.Errors)
	})

	t.Run("empty string fails", func(t *testing.T) {
		result := adapter.Validate(context.Background(), "")
		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Error(), "cannot be empty")
	})

	t.Run("adapter metadata", func(t *testing.T) {
		assert.Equal(t, "not-empty", adapter.Name())
		assert.Equal(t, "common", adapter.Domain())
		assert.Equal(t, "string", adapter.Category())
		assert.Equal(t, 50, adapter.Priority())
		assert.Empty(t, adapter.Dependencies())
	})
}

func TestRegistryAdapter(t *testing.T) {
	registry := NewValidatorRegistry()
	kubernetesValidator := NewKubernetesManifestValidator()
	dockerValidator := NewDockerConfigValidator()
	securityValidator := NewSecurityPolicyValidator()

	err := registry.Register(kubernetesValidator)
	require.NoError(t, err)
	err = registry.Register(dockerValidator)
	require.NoError(t, err)
	err = registry.Register(securityValidator)
	require.NoError(t, err)

	adapter := NewRegistryAdapter(registry)

	t.Run("validate kubernetes manifest", func(t *testing.T) {
		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test-pod",
			},
		}

		err := adapter.ValidateKubernetesManifest(manifest)
		assert.NoError(t, err)

		// Test with invalid manifest
		invalidManifest := map[string]interface{}{
			"kind": "Pod", // Missing apiVersion
		}
		err = adapter.ValidateKubernetesManifest(invalidManifest)
		assert.Error(t, err)
	})

	t.Run("validate docker config", func(t *testing.T) {
		config := map[string]interface{}{
			"image": "nginx:latest",
		}

		err := adapter.ValidateDockerConfig(config)
		assert.NoError(t, err)

		// Test with invalid config
		invalidConfig := map[string]interface{}{
			"image": "",
		}
		err = adapter.ValidateDockerConfig(invalidConfig)
		assert.Error(t, err)
	})

	t.Run("validate security policy", func(t *testing.T) {
		policy := map[string]interface{}{
			"privileged": false,
		}

		err := adapter.ValidateSecurityPolicy(policy)
		assert.NoError(t, err)

		// Test with invalid policy
		invalidPolicy := map[string]interface{}{
			"privileged": true,
		}
		err = adapter.ValidateSecurityPolicy(invalidPolicy)
		assert.Error(t, err)
	})

	t.Run("validate with domain", func(t *testing.T) {
		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test-pod",
			},
		}

		err := adapter.ValidateWithDomain(manifest, "kubernetes", "manifest")
		assert.NoError(t, err)
	})
}

func TestStringValidationAdapter(t *testing.T) {
	validateNotEmpty := func(s string) error {
		if s == "" {
			return fmt.Errorf("string cannot be empty")
		}
		return nil
	}

	adapter := NewStringValidationAdapter("string-not-empty", validateNotEmpty)

	t.Run("valid string", func(t *testing.T) {
		result := adapter.Validate(context.Background(), "hello")
		assert.True(t, result.Valid)
		assert.Empty(t, result.Errors)
	})

	t.Run("empty string fails", func(t *testing.T) {
		result := adapter.Validate(context.Background(), "")
		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 1)
	})

	t.Run("non-string input fails", func(t *testing.T) {
		result := adapter.Validate(context.Background(), 123)
		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Error(), "expected string")
	})

	t.Run("adapter metadata", func(t *testing.T) {
		assert.Equal(t, "string-not-empty", adapter.Name())
		assert.Equal(t, "common", adapter.Domain())
		assert.Equal(t, "string", adapter.Category())
		assert.Equal(t, 50, adapter.Priority())
	})
}

func TestNetworkValidationAdapter(t *testing.T) {
	validatePort := func(s string) error {
		if s == "" {
			return fmt.Errorf("port cannot be empty")
		}
		// Simple port validation
		if len(s) > 5 {
			return fmt.Errorf("port too long")
		}
		return nil
	}

	adapter := NewNetworkValidationAdapter("port-validator", validatePort)

	t.Run("valid port", func(t *testing.T) {
		result := adapter.Validate(context.Background(), "8080")
		assert.True(t, result.Valid)
		assert.Empty(t, result.Errors)
	})

	t.Run("empty port fails", func(t *testing.T) {
		result := adapter.Validate(context.Background(), "")
		assert.False(t, result.Valid)
		assert.Contains(t, result.Errors[0].Error(), "cannot be empty")
	})

	t.Run("adapter metadata", func(t *testing.T) {
		assert.Equal(t, "port-validator", adapter.Name())
		assert.Equal(t, "network", adapter.Domain())
		assert.Equal(t, "basic", adapter.Category())
		assert.Equal(t, 75, adapter.Priority())
	})
}

func TestValidationResultAdapter(t *testing.T) {
	t.Run("valid result", func(t *testing.T) {
		result := ValidationResult{
			Valid:  true,
			Errors: make([]error, 0),
		}
		adapter := NewValidationResultAdapter(result)

		assert.NoError(t, adapter.AsError())
		assert.NoError(t, adapter.AsMultiError())
		
		valid, err := adapter.AsBoolError()
		assert.True(t, valid)
		assert.NoError(t, err)
	})

	t.Run("invalid result with single error", func(t *testing.T) {
		result := ValidationResult{
			Valid:  false,
			Errors: []error{fmt.Errorf("test error")},
		}
		adapter := NewValidationResultAdapter(result)

		assert.Error(t, adapter.AsError())
		assert.Equal(t, "test error", adapter.AsError().Error())
		
		multiErr := adapter.AsMultiError()
		assert.Error(t, multiErr)
		assert.Equal(t, "test error", multiErr.Error())

		valid, err := adapter.AsBoolError()
		assert.False(t, valid)
		assert.Error(t, err)
	})

	t.Run("invalid result with multiple errors", func(t *testing.T) {
		result := ValidationResult{
			Valid: false,
			Errors: []error{
				fmt.Errorf("error 1"),
				fmt.Errorf("error 2"),
			},
		}
		adapter := NewValidationResultAdapter(result)

		assert.Error(t, adapter.AsError())
		assert.Equal(t, "error 1", adapter.AsError().Error()) // First error

		multiErr := adapter.AsMultiError()
		assert.Error(t, multiErr)
		assert.Contains(t, multiErr.Error(), "multiple validation errors")

		// Test unwrap
		if multiValidationError, ok := IsMultiValidationError(multiErr); ok {
			assert.Len(t, multiValidationError.Errors, 2)
		} else {
			t.Error("Expected MultiValidationError")
		}
	})
}

func TestMultiValidationError(t *testing.T) {
	errors := []error{
		fmt.Errorf("error 1"),
		fmt.Errorf("error 2"),
	}
	multiErr := &MultiValidationError{Errors: errors}

	t.Run("error message", func(t *testing.T) {
		assert.Contains(t, multiErr.Error(), "multiple validation errors")
		assert.Contains(t, multiErr.Error(), "2 errors")
	})

	t.Run("unwrap", func(t *testing.T) {
		unwrapped := multiErr.Unwrap()
		assert.Len(t, unwrapped, 2)
		assert.Equal(t, "error 1", unwrapped[0].Error())
		assert.Equal(t, "error 2", unwrapped[1].Error())
	})

	t.Run("single error", func(t *testing.T) {
		singleErr := &MultiValidationError{Errors: []error{fmt.Errorf("single error")}}
		assert.Equal(t, "single error", singleErr.Error())
	})

	t.Run("is multi validation error", func(t *testing.T) {
		multiErr, ok := IsMultiValidationError(multiErr)
		assert.True(t, ok)
		assert.NotNil(t, multiErr)

		regularErr := fmt.Errorf("regular error")
		_, ok = IsMultiValidationError(regularErr)
		assert.False(t, ok)
	})
}

func TestMigrationHelper(t *testing.T) {
	registry := NewValidatorRegistry()
	helper := NewMigrationHelper(registry)

	t.Run("register legacy function", func(t *testing.T) {
		validateFunc := func(data interface{}) error {
			str, ok := data.(string)
			if !ok {
				return fmt.Errorf("expected string")
			}
			if str == "invalid" {
				return fmt.Errorf("invalid value")
			}
			return nil
		}

		err := helper.RegisterLegacyFunction("legacy-validator", "test", "basic", 100, validateFunc)
		assert.NoError(t, err)

		// Test validation
		result := registry.ValidateAll(context.Background(), "valid", "test", "basic")
		assert.True(t, result.Valid)

		result = registry.ValidateAll(context.Background(), "invalid", "test", "basic")
		assert.False(t, result.Valid)
	})

	t.Run("register string validator", func(t *testing.T) {
		validateString := func(s string) error {
			if len(s) < 3 {
				return fmt.Errorf("string too short")
			}
			return nil
		}

		err := helper.RegisterStringValidator("min-length", validateString)
		assert.NoError(t, err)

		// Test validation
		result := registry.ValidateAll(context.Background(), "hello", "common", "string")
		assert.True(t, result.Valid)

		result = registry.ValidateAll(context.Background(), "hi", "common", "string")
		assert.False(t, result.Valid)
	})

	t.Run("register network validator", func(t *testing.T) {
		validateNetwork := func(s string) error {
			if s == "invalid-ip" {
				return fmt.Errorf("invalid IP address")
			}
			return nil
		}

		err := helper.RegisterNetworkValidator("ip-validator", validateNetwork)
		assert.NoError(t, err)

		// Test validation
		result := registry.ValidateAll(context.Background(), "192.168.1.1", "network", "basic")
		assert.True(t, result.Valid)

		result = registry.ValidateAll(context.Background(), "invalid-ip", "network", "basic")
		assert.False(t, result.Valid)
	})

	t.Run("validate and adapt", func(t *testing.T) {
		// Test different result formats
		result := helper.ValidateAndAdapt("hello", "common", "string", "error")
		if result != nil {
			assert.NoError(t, result.(error))
		} else {
			assert.Nil(t, result) // No error means nil for "error" format
		}

		result = helper.ValidateAndAdapt("hi", "common", "string", "error")
		if result != nil {
			assert.Error(t, result.(error))
		}

		result = helper.ValidateAndAdapt("hello", "common", "string", "bool-error")
		resultMap := result.(map[string]interface{})
		assert.True(t, resultMap["valid"].(bool))
		if resultMap["error"] != nil {
			assert.NoError(t, resultMap["error"].(error))
		}

		result = helper.ValidateAndAdapt("hello", "common", "string", "full")
		validationResult := result.(ValidationResult)
		assert.True(t, validationResult.Valid)
	})
}