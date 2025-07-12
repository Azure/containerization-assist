package prompts

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPromptRegistryIntegration tests the complete prompt registry system
func TestPromptRegistryIntegration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	registry := NewRegistry(nil, logger) // nil server for testing

	t.Run("registry initialization", func(t *testing.T) {
		assert.NotNil(t, registry)
		assert.NotNil(t, registry.loader)

		err := registry.RegisterAll()
		assert.NoError(t, err)
	})

	t.Run("list all prompts", func(t *testing.T) {
		prompts := registry.ListPrompts()
		assert.NotEmpty(t, prompts)

		expectedPrompts := []string{
			"containerKit.quickDockerfile",
			"containerKit.deploy",
			"containerKit.troubleshoot",
			"containerKit.analyze",
			"containerKit.k8sManifest",
		}

		for _, expected := range expectedPrompts {
			assert.Contains(t, prompts, expected)
			versions := prompts[expected]
			assert.NotEmpty(t, versions)
			assert.Contains(t, versions, "1.0.0")
		}
	})

	t.Run("get prompt info", func(t *testing.T) {
		info, err := registry.GetPromptInfo("containerKit.quickDockerfile", "1.0.0")
		assert.NoError(t, err)
		assert.NotNil(t, info)

		assert.Equal(t, "containerKit.quickDockerfile", info.Name)
		assert.Equal(t, "1.0.0", info.Version)
		assert.Equal(t, "generation", info.Category)
		assert.Contains(t, info.Metadata.Tags, "dockerfile")
		assert.Greater(t, info.Metadata.EstimatedTokens, 0)
	})

	t.Run("render dockerfile prompt", func(t *testing.T) {
		params := map[string]interface{}{
			"language":  "python",
			"framework": "fastapi",
			"port":      8000,
		}

		result, err := registry.RenderPrompt("containerKit.quickDockerfile", "1.0.0", params)
		assert.NoError(t, err)
		assert.NotEmpty(t, result)

		// Verify template rendering
		assert.Contains(t, result, "python")
		assert.Contains(t, result, "fastapi")
		assert.Contains(t, result, "8000")
		assert.Contains(t, result, "multi-stage")
		assert.Contains(t, result, "security")
	})

	t.Run("render deploy prompt", func(t *testing.T) {
		params := map[string]interface{}{
			"repo_url":    "https://github.com/test/app.git",
			"branch":      "develop",
			"registry":    "docker.io/test/app",
			"scan":        true,
			"environment": "staging",
		}

		result, err := registry.RenderPrompt("containerKit.deploy", "1.0.0", params)
		assert.NoError(t, err)
		assert.NotEmpty(t, result)

		assert.Contains(t, result, "test/app.git")
		assert.Contains(t, result, "develop")
		assert.Contains(t, result, "staging")
		assert.Contains(t, result, "10-step")
	})

	t.Run("get latest version", func(t *testing.T) {
		version := registry.GetLatestVersion("containerKit.quickDockerfile")
		assert.Equal(t, "1.0.0", version)
	})
}

// TestWorkflowPromptGeneration tests generating prompts for actual workflows
func TestWorkflowPromptGeneration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	registry := NewRegistry(nil, logger)

	// Test realistic workflow scenarios
	scenarios := []struct {
		name         string
		promptName   string
		params       map[string]interface{}
		requirements []string
	}{
		{
			name:       "go_microservice_production",
			promptName: "containerKit.quickDockerfile",
			params: map[string]interface{}{
				"language":  "go",
				"framework": "gin",
				"port":      8080,
			},
			requirements: []string{
				"go", "gin", "8080",
				"multi-stage", "security", "alpine",
			},
		},
		{
			name:       "node_webapp_development",
			promptName: "containerKit.quickDockerfile",
			params: map[string]interface{}{
				"language":  "node",
				"framework": "express",
				"port":      3000,
			},
			requirements: []string{
				"node", "express", "3000",
			},
		},
		{
			name:       "production_deployment",
			promptName: "containerKit.deploy",
			params: map[string]interface{}{
				"repo_url":    "https://github.com/company/api.git",
				"branch":      "main",
				"registry":    "gcr.io/company/api",
				"scan":        true,
				"environment": "production",
			},
			requirements: []string{
				"production", "company/api", "Security Scan", "10-step",
				"Health Verification", "rolling update",
			},
		},
		{
			name:       "kubernetes_production_manifest",
			promptName: "containerKit.k8sManifest",
			params: map[string]interface{}{
				"app_name":     "payment-service",
				"image":        "gcr.io/company/payment:v2.1.0",
				"namespace":    "production",
				"replicas":     5,
				"port":         8080,
				"service_type": "LoadBalancer",
				"environment":  "production",
				"resources": map[string]interface{}{
					"requests": map[string]interface{}{
						"cpu":    "200m",
						"memory": "256Mi",
					},
					"limits": map[string]interface{}{
						"cpu":    "1000m",
						"memory": "1Gi",
					},
				},
			},
			requirements: []string{
				"payment-service", "production", "replicas: 5",
				"LoadBalancer", "HorizontalPodAutoscaler",
				"securityContext", "NetworkPolicy", "200m", "256Mi",
			},
		},
		{
			name:       "error_troubleshooting",
			promptName: "containerKit.troubleshoot",
			params: map[string]interface{}{
				"error_message": "Unable to pull image: repository does not exist",
				"operation":     "deploy",
				"context":       "Deploying to Kubernetes cluster",
				"severity":      "high",
			},
			requirements: []string{
				"repository does not exist", "deploy", "high",
				"Root Cause Analysis", "Prevention Strategies",
				"Alternative Approaches",
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			result, err := registry.RenderPrompt(scenario.promptName, "1.0.0", scenario.params)
			require.NoError(t, err)
			require.NotEmpty(t, result)

			// Verify all requirements are present
			for _, req := range scenario.requirements {
				assert.Contains(t, result, req,
					"Scenario %s missing requirement: %s", scenario.name, req)
			}

			// Log result for manual inspection
			t.Logf("Scenario %s result length: %d characters", scenario.name, len(result))
		})
	}
}

// TestPromptVersioning tests version management
func TestPromptVersioning(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	registry := NewRegistry(nil, logger)

	t.Run("version consistency", func(t *testing.T) {
		prompts := registry.ListPrompts()

		for promptName := range prompts {
			latestVersion := registry.GetLatestVersion(promptName)
			assert.NotEmpty(t, latestVersion, "Template %s has no latest version", promptName)

			// Should be able to get info for latest version
			info, err := registry.GetPromptInfo(promptName, latestVersion)
			assert.NoError(t, err)
			assert.Equal(t, latestVersion, info.Version)

			// Should be able to render with latest version
			_, err = registry.RenderPrompt(promptName, latestVersion, map[string]interface{}{})
			// May fail due to required params, but should not fail due to version issues
			if err != nil {
				assert.Contains(t, err.Error(), "required parameter",
					"Unexpected error for %s: %v", promptName, err)
			}
		}
	})

	t.Run("semantic versioning", func(t *testing.T) {
		// All current templates should be version 1.0.0
		expectedVersion := "1.0.0"
		prompts := registry.ListPrompts()

		for promptName := range prompts {
			latestVersion := registry.GetLatestVersion(promptName)
			assert.Equal(t, expectedVersion, latestVersion,
				"Template %s has unexpected version", promptName)
		}
	})
}

// TestPromptComplexityAndTokens tests complexity and token estimation
func TestPromptComplexityAndTokens(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	registry := NewRegistry(nil, logger)

	complexityLevels := map[string][]string{
		"intermediate": {"containerKit.quickDockerfile"},
		"advanced": {
			"containerKit.deploy",
			"containerKit.troubleshoot",
			"containerKit.analyze",
			"containerKit.k8sManifest",
		},
	}

	for complexity, templateNames := range complexityLevels {
		for _, templateName := range templateNames {
			t.Run(templateName+"_complexity", func(t *testing.T) {
				info, err := registry.GetPromptInfo(templateName, "")
				assert.NoError(t, err)
				assert.Equal(t, complexity, info.Metadata.Complexity)
				assert.Greater(t, info.Metadata.EstimatedTokens, 0)

				// Advanced templates should have higher token estimates
				if complexity == "advanced" {
					assert.Greater(t, info.Metadata.EstimatedTokens, 500)
				}
			})
		}
	}
}

// TestPromptParameterValidation tests comprehensive parameter validation
func TestPromptParameterValidation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	registry := NewRegistry(nil, logger)

	t.Run("k8s_manifest_parameter_validation", func(t *testing.T) {
		// Test valid parameters
		validParams := map[string]interface{}{
			"app_name":     "valid-name",
			"image":        "nginx:latest",
			"namespace":    "production",
			"replicas":     3,
			"port":         8080,
			"service_type": "ClusterIP",
			"environment":  "production",
		}

		_, err := registry.RenderPrompt("containerKit.k8sManifest", "1.0.0", validParams)
		assert.NoError(t, err)

		// Test invalid app_name (with uppercase)
		invalidParams := map[string]interface{}{
			"app_name": "Invalid-Name", // Should be lowercase
			"image":    "nginx:latest",
		}

		_, err = registry.RenderPrompt("containerKit.k8sManifest", "1.0.0", invalidParams)
		// Should validate pattern ^[a-z0-9-]+$
		assert.Error(t, err)

		// Test replicas out of range
		outOfRangeParams := map[string]interface{}{
			"app_name": "test-app",
			"image":    "nginx:latest",
			"replicas": 500, // Out of range [1, 100]
		}

		_, err = registry.RenderPrompt("containerKit.k8sManifest", "1.0.0", outOfRangeParams)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "out of range")
	})

	t.Run("troubleshoot_required_parameters", func(t *testing.T) {
		// Missing required error_message
		incompleteParams := map[string]interface{}{
			"operation": "build",
		}

		_, err := registry.RenderPrompt("containerKit.troubleshoot", "1.0.0", incompleteParams)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "required parameter")
	})
}

// BenchmarkPromptRendering benchmarks prompt rendering performance
func BenchmarkPromptRendering(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	registry := NewRegistry(nil, logger)

	params := map[string]interface{}{
		"language":  "go",
		"framework": "gin",
		"port":      8080,
	}

	b.Run("quickDockerfile", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := registry.RenderPrompt("containerKit.quickDockerfile", "1.0.0", params)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	k8sParams := map[string]interface{}{
		"app_name": "test-app",
		"image":    "nginx:latest",
	}

	b.Run("k8sManifest", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := registry.RenderPrompt("containerKit.k8sManifest", "1.0.0", k8sParams)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
