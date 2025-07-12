package workflow

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestElicitationClient tests the basic elicitation client functionality
func TestElicitationClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := NewElicitationClient(nil, logger) // nil context forces fallback mode

	t.Run("text elicitation with default", func(t *testing.T) {
		request := ElicitationRequest{
			Prompt:   "Enter application name:",
			Type:     ElicitationTypeText,
			Default:  "my-app",
			Required: true,
		}

		response, err := client.Elicit(context.Background(), request)
		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, "my-app", response.Value)
		assert.False(t, response.Cancelled)
		assert.True(t, response.Metadata["fallback_mode"].(bool))
	})

	t.Run("choice elicitation with options", func(t *testing.T) {
		request := ElicitationRequest{
			Prompt:  "Select environment:",
			Type:    ElicitationTypeChoice,
			Options: []string{"development", "staging", "production"},
		}

		response, err := client.Elicit(context.Background(), request)
		assert.NoError(t, err)
		assert.Equal(t, "development", response.Value) // Should pick first option
	})

	t.Run("boolean elicitation", func(t *testing.T) {
		request := ElicitationRequest{
			Prompt: "Enable security scanning?",
			Type:   ElicitationTypeBoolean,
		}

		response, err := client.Elicit(context.Background(), request)
		assert.NoError(t, err)
		assert.Equal(t, "true", response.Value)
	})

	t.Run("number elicitation", func(t *testing.T) {
		request := ElicitationRequest{
			Prompt:  "Enter port number:",
			Type:    ElicitationTypeNumber,
			Default: "3000",
		}

		response, err := client.Elicit(context.Background(), request)
		assert.NoError(t, err)
		assert.Equal(t, "3000", response.Value)
	})
}

// TestElicitationValidation tests validation rules
func TestElicitationValidation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	t.Run("required validation", func(t *testing.T) {
		// Test the validateResponse method directly since Elicit always provides defaults
		client := NewElicitationClient(nil, logger)

		request := ElicitationRequest{
			Prompt:   "Enter required value:",
			Type:     ElicitationTypeText,
			Required: true,
			Validation: &ValidationRules{
				Required: true,
			},
		}

		// Should fail validation with empty value
		err := client.ValidateResponse("", request)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "value is required")
	})

	t.Run("length validation", func(t *testing.T) {
		client := NewElicitationClient(nil, logger)

		// Test minimum length
		err := client.ValidateResponse("ab", ElicitationRequest{
			Validation: &ValidationRules{
				MinLength: 3,
			},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least 3 characters")

		// Test maximum length
		err = client.ValidateResponse("toolong", ElicitationRequest{
			Validation: &ValidationRules{
				MaxLength: 5,
			},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at most 5 characters")
	})

	t.Run("allowed keys validation", func(t *testing.T) {
		client := NewElicitationClient(nil, logger)

		err := client.ValidateResponse("invalid", ElicitationRequest{
			Validation: &ValidationRules{
				AllowedKeys: []string{"valid1", "valid2"},
			},
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be one of")

		// Valid key should pass
		err = client.ValidateResponse("valid1", ElicitationRequest{
			Validation: &ValidationRules{
				AllowedKeys: []string{"valid1", "valid2"},
			},
		})
		assert.NoError(t, err)
	})
}

// TestElicitMissingConfiguration tests configuration elicitation
func TestElicitMissingConfiguration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := NewElicitationClient(nil, logger)

	t.Run("elicit missing values", func(t *testing.T) {
		initialConfig := map[string]interface{}{
			"app_name": "existing-app",
			// port and registry are missing
		}

		config, err := client.ElicitMissingConfiguration(context.Background(), initialConfig)
		assert.NoError(t, err)
		assert.NotNil(t, config)

		// Should preserve existing values
		assert.Equal(t, "existing-app", config["app_name"])

		// Should have elicited missing values
		assert.Contains(t, config, "port")
		assert.Contains(t, config, "registry")
		assert.Contains(t, config, "environment")
		assert.Contains(t, config, "enable_security_scan")

		// Check default values
		assert.Equal(t, "8080", config["port"])
		assert.Equal(t, "docker.io", config["registry"])
	})

	t.Run("complete configuration unchanged", func(t *testing.T) {
		completeConfig := map[string]interface{}{
			"app_name":             "complete-app",
			"port":                 "3000",
			"registry":             "ghcr.io",
			"environment":          "production",
			"enable_security_scan": "false",
		}

		config, err := client.ElicitMissingConfiguration(context.Background(), completeConfig)
		assert.NoError(t, err)

		// Should be unchanged
		assert.Equal(t, completeConfig, config)
	})
}

// TestElicitDeploymentParameters tests deployment parameter elicitation
func TestElicitDeploymentParameters(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := NewElicitationClient(nil, logger)

	t.Run("elicit deployment parameters", func(t *testing.T) {
		initialParams := map[string]interface{}{
			"namespace": "custom-namespace",
			// Other parameters missing
		}

		params, err := client.ElicitDeploymentParameters(context.Background(), initialParams)
		assert.NoError(t, err)
		assert.NotNil(t, params)

		// Should preserve existing values
		assert.Equal(t, "custom-namespace", params["namespace"])

		// Should have elicited missing values with defaults
		assert.Equal(t, "2", params["replicas"])
		assert.Equal(t, "500m", params["cpu_limit"])
		assert.Equal(t, "512Mi", params["memory_limit"])
		assert.Equal(t, "ClusterIP", params["service_type"])
	})
}

// TestElicitationTypes tests different elicitation types
func TestElicitationTypes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := NewElicitationClient(nil, logger)

	testCases := []struct {
		name         string
		elicitType   ElicitationType
		expectedType string
	}{
		{"password", ElicitationTypePassword, ""},
		{"url", ElicitationTypeURL, "https://localhost:8080"},
		{"file", ElicitationTypeFile, "./Dockerfile"},
		{"directory", ElicitationTypeDirectory, "./"},
		{"multi_choice", ElicitationTypeMultiChoice, "default"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request := ElicitationRequest{
				Prompt: "Test prompt",
				Type:   tc.elicitType,
			}

			response, err := client.Elicit(context.Background(), request)
			assert.NoError(t, err)
			assert.NotNil(t, response)

			if tc.expectedType != "" {
				assert.Equal(t, tc.expectedType, response.Value)
			}
		})
	}
}

// TestElicitationWithContext tests context-aware elicitation
func TestElicitationWithContext(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := NewElicitationClient(nil, logger)

	t.Run("context provides workspace name", func(t *testing.T) {
		request := ElicitationRequest{
			Prompt: "Enter application name:",
			Type:   ElicitationTypeText,
			Context: map[string]interface{}{
				"workspace_name": "my-workspace",
			},
		}

		response, err := client.Elicit(context.Background(), request)
		assert.NoError(t, err)
		assert.Equal(t, "my-workspace", response.Value)
	})

	t.Run("context provides project name", func(t *testing.T) {
		request := ElicitationRequest{
			Prompt: "Enter application name:",
			Type:   ElicitationTypeText,
			Context: map[string]interface{}{
				"project_name": "my-project",
			},
		}

		response, err := client.Elicit(context.Background(), request)
		assert.NoError(t, err)
		assert.Equal(t, "my-project", response.Value)
	})
}

// TestElicitationTimeout tests timeout handling
func TestElicitationTimeout(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := NewElicitationClient(nil, logger)

	t.Run("custom timeout", func(t *testing.T) {
		request := ElicitationRequest{
			Prompt:  "Enter value:",
			Type:    ElicitationTypeText,
			Timeout: 1 * time.Millisecond, // Very short timeout
		}

		// In fallback mode, timeout is not enforced, so this should succeed
		response, err := client.Elicit(context.Background(), request)
		assert.NoError(t, err)
		assert.NotNil(t, response)
	})
}

// BenchmarkElicitationOperations benchmarks elicitation operations
func BenchmarkElicitationOperations(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := NewElicitationClient(nil, logger)

	b.Run("simple text elicitation", func(b *testing.B) {
		request := ElicitationRequest{
			Prompt:  "Enter value:",
			Type:    ElicitationTypeText,
			Default: "test",
		}

		for i := 0; i < b.N; i++ {
			_, err := client.Elicit(context.Background(), request)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("configuration elicitation", func(b *testing.B) {
		config := map[string]interface{}{
			"app_name": "test-app",
		}

		for i := 0; i < b.N; i++ {
			_, err := client.ElicitMissingConfiguration(context.Background(), config)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
