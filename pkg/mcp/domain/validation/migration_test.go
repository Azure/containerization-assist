package validation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasicValidatorMigration(t *testing.T) {
	registry := NewValidatorRegistry()

	t.Run("register common validators", func(t *testing.T) {
		// Register various basic validators with unified interface
		validators := []DomainValidator[interface{}]{
			NewStringLengthValidator("username-length", "username", 3, 20),
			NewEmailValidator("user-email", "email"),
			NewURLValidator("website-url", "website"),
			NewRequiredValidator("name-required", "name"),
			NewNetworkPortValidator("service-port", "port", false),
			NewIPAddressValidator("server-ip", "ip", true, true, false),
		}

		for _, validator := range validators {
			err := registry.Register(validator)
			require.NoError(t, err)
		}

		assert.Equal(t, 6, registry.(*ValidatorRegistryImpl).Count())
	})

	t.Run("validate string fields", func(t *testing.T) {
		// Test string length validation - this will run ALL string validators in common domain
		result := registry.ValidateAll(context.Background(), "john", "common", "string")
		// "john" passes length validation but fails email and URL validation
		assert.False(t, result.Valid)
		assert.True(t, len(result.Errors) >= 1) // Should have validation errors

		result = registry.ValidateAll(context.Background(), "ab", "common", "string") // Too short
		assert.False(t, result.Valid)
		assert.True(t, len(result.Errors) >= 2) // Should have multiple validation errors

		// Test with valid email that is also a valid URL format - this should pass
		result = registry.ValidateAll(context.Background(), "https://example.com", "common", "string")
		// This passes URL validation but fails email validation - let's check specific validators
		assert.False(t, result.Valid) // Will fail email validation
	})

	t.Run("validate network fields", func(t *testing.T) {
		// Test port validation
		result := registry.ValidateAll(context.Background(), 8080, "network", "port")
		assert.True(t, result.Valid)

		result = registry.ValidateAll(context.Background(), 0, "network", "port") // Invalid port
		assert.False(t, result.Valid)

		// Test IP validation
		result = registry.ValidateAll(context.Background(), "192.168.1.1", "network", "address")
		assert.True(t, result.Valid)

		result = registry.ValidateAll(context.Background(), "invalid-ip", "network", "address")
		assert.False(t, result.Valid)
	})

	t.Run("required field validation", func(t *testing.T) {
		result := registry.ValidateAll(context.Background(), "John Doe", "common", "required")
		assert.True(t, result.Valid)

		result = registry.ValidateAll(context.Background(), "", "common", "required")
		assert.False(t, result.Valid)

		result = registry.ValidateAll(context.Background(), "   ", "common", "required") // Whitespace only
		assert.False(t, result.Valid)
	})

	t.Run("pattern validation", func(t *testing.T) {
		// Create pattern validator for phone numbers
		phoneValidator, err := NewPatternValidator("phone-pattern", "phone", `^\+?[1-9]\d{1,14}$`)
		require.NoError(t, err)

		err = registry.Register(phoneValidator)
		require.NoError(t, err)

		result := registry.ValidateAll(context.Background(), "+1234567890", "common", "pattern")
		assert.True(t, result.Valid)

		result = registry.ValidateAll(context.Background(), "invalid-phone", "common", "pattern")
		assert.False(t, result.Valid)
	})
}

func TestValidatorIntegrationWithExistingSystem(t *testing.T) {
	t.Run("combine new and existing validators", func(t *testing.T) {
		registry := NewValidatorRegistry()

		// Register existing domain validators
		kubernetesValidator := NewKubernetesManifestValidator()
		err := registry.Register(kubernetesValidator)
		require.NoError(t, err)

		// Register migrated common validators
		emailValidator := NewEmailValidator("contact-email", "email")
		err = registry.Register(emailValidator)
		require.NoError(t, err)

		portValidator := NewNetworkPortValidator("service-port", "port", false)
		err = registry.Register(portValidator)
		require.NoError(t, err)

		// Test Kubernetes validation
		manifest := map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name": "my-service",
			},
			"spec": map[string]interface{}{
				"type": "ClusterIP",
				"ports": []interface{}{
					map[string]interface{}{
						"port":     80,
						"protocol": "TCP",
					},
				},
				"selector": map[string]interface{}{
					"app": "my-app",
				},
			},
		}

		result := registry.ValidateAll(context.Background(), manifest, "kubernetes", "manifest")
		assert.True(t, result.Valid)

		// Test email validation
		result = registry.ValidateAll(context.Background(), "admin@example.com", "common", "string")
		assert.True(t, result.Valid)

		// Test port validation
		result = registry.ValidateAll(context.Background(), 80, "network", "port")
		assert.True(t, result.Valid)

		// Verify all validators are registered
		validators := registry.ListValidators()
		assert.Len(t, validators, 3)

		// Check domains are properly categorized
		domainCount := make(map[string]int)
		for _, v := range validators {
			domainCount[v.Domain]++
		}

		assert.Equal(t, 1, domainCount["kubernetes"])
		assert.Equal(t, 1, domainCount["common"])
		assert.Equal(t, 1, domainCount["network"])
	})
}
