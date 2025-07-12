package prompts

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTemplateLoader tests the template loading system
func TestTemplateLoader(t *testing.T) {
	loader, err := NewTemplateLoader()
	require.NoError(t, err)
	assert.NotNil(t, loader)

	t.Run("list all templates", func(t *testing.T) {
		templates := loader.ListTemplates()
		assert.NotEmpty(t, templates)

		// Verify expected templates exist
		expectedTemplates := []string{
			"containerKit.quickDockerfile",
			"containerKit.deploy",
			"containerKit.troubleshoot",
			"containerKit.analyze",
			"containerKit.k8sManifest",
		}

		for _, expected := range expectedTemplates {
			assert.Contains(t, templates, expected, "Expected template %s not found", expected)
			assert.NotEmpty(t, templates[expected], "Template %s has no versions", expected)
		}
	})

	t.Run("get template by name and version", func(t *testing.T) {
		template, err := loader.GetTemplate("containerKit.quickDockerfile", "1.0.0")
		assert.NoError(t, err)
		assert.NotNil(t, template)
		assert.Equal(t, "containerKit.quickDockerfile", template.Name)
		assert.Equal(t, "1.0.0", template.Version)
		assert.NotEmpty(t, template.Description)
		assert.NotEmpty(t, template.Template)
	})

	t.Run("get latest version", func(t *testing.T) {
		version := loader.GetLatestVersion("containerKit.quickDockerfile")
		assert.Equal(t, "1.0.0", version)
	})

	t.Run("template not found", func(t *testing.T) {
		_, err := loader.GetTemplate("nonexistent", "1.0.0")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// TestTemplateRendering tests template rendering with various parameters
func TestTemplateRendering(t *testing.T) {
	loader, err := NewTemplateLoader()
	require.NoError(t, err)

	t.Run("render dockerfile template", func(t *testing.T) {
		params := map[string]interface{}{
			"language":  "go",
			"framework": "gin",
			"port":      8080,
		}

		result, err := loader.RenderTemplate("containerKit.quickDockerfile", "1.0.0", params)
		assert.NoError(t, err)
		assert.NotEmpty(t, result)

		// Verify parameter substitution
		assert.Contains(t, result, "go")
		assert.Contains(t, result, "gin")
		assert.Contains(t, result, "8080")
		assert.Contains(t, result, "multi-stage")
		assert.Contains(t, result, "security")
	})

	t.Run("render deploy template", func(t *testing.T) {
		params := map[string]interface{}{
			"repo_url":    "https://github.com/org/app.git",
			"branch":      "main",
			"registry":    "ghcr.io/org/app",
			"scan":        true,
			"environment": "production",
		}

		result, err := loader.RenderTemplate("containerKit.deploy", "1.0.0", params)
		assert.NoError(t, err)
		assert.NotEmpty(t, result)

		// Verify parameter substitution
		assert.Contains(t, result, "https://github.com/org/app.git")
		assert.Contains(t, result, "main")
		assert.Contains(t, result, "ghcr.io/org/app")
		assert.Contains(t, result, "production")
		assert.Contains(t, result, "workflow")
	})

	t.Run("render with defaults", func(t *testing.T) {
		// Minimal parameters, should use defaults
		params := map[string]interface{}{}

		result, err := loader.RenderTemplate("containerKit.quickDockerfile", "1.0.0", params)
		assert.NoError(t, err)
		assert.NotEmpty(t, result)

		// Should contain default values
		assert.Contains(t, result, "auto-detect")
		assert.Contains(t, result, "8080")
	})
}

// TestParameterValidation tests parameter validation
func TestParameterValidation(t *testing.T) {
	loader, err := NewTemplateLoader()
	require.NoError(t, err)

	t.Run("required parameter missing", func(t *testing.T) {
		params := map[string]interface{}{
			// Missing required parameters
		}

		_, err := loader.RenderTemplate("containerKit.troubleshoot", "1.0.0", params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "required parameter")
	})

	t.Run("invalid parameter type", func(t *testing.T) {
		params := map[string]interface{}{
			"error_message": "Test error",
			"operation":     "build",
			"context":       "",
			"severity":      123, // Should be string
		}

		_, err := loader.RenderTemplate("containerKit.troubleshoot", "1.0.0", params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be a string")
	})

	t.Run("parameter out of range", func(t *testing.T) {
		params := map[string]interface{}{
			"app_name": "test-app",
			"image":    "nginx:latest",
			"replicas": 1000, // Out of range [1, 100]
		}

		_, err := loader.RenderTemplate("containerKit.k8sManifest", "1.0.0", params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "out of range")
	})

	t.Run("invalid option", func(t *testing.T) {
		params := map[string]interface{}{
			"app_name":     "test-app",
			"image":        "nginx:latest",
			"service_type": "InvalidType", // Not in options
		}

		_, err := loader.RenderTemplate("containerKit.k8sManifest", "1.0.0", params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not in allowed options")
	})

	t.Run("string too long", func(t *testing.T) {
		longString := strings.Repeat("a", 3000) // Exceeds max_length

		params := map[string]interface{}{
			"error_message": longString,
		}

		_, err := loader.RenderTemplate("containerKit.troubleshoot", "1.0.0", params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "too long")
	})
}

// TestTemplateSnapshots tests template outputs for regression
func TestTemplateSnapshots(t *testing.T) {
	loader, err := NewTemplateLoader()
	require.NoError(t, err)

	testCases := []struct {
		name       string
		template   string
		version    string
		params     map[string]interface{}
		assertions []string // Strings that must be present in output
	}{
		{
			name:     "dockerfile_go_gin",
			template: "containerKit.quickDockerfile",
			version:  "1.0.0",
			params: map[string]interface{}{
				"language":  "go",
				"framework": "gin",
				"port":      8080,
			},
			assertions: []string{
				"multi-stage",
				"security",
				"go",
				"gin",
				"8080",
				"alpine",
				"distroless",
				"scratch",
			},
		},
		{
			name:     "dockerfile_node_express",
			template: "containerKit.quickDockerfile",
			version:  "1.0.0",
			params: map[string]interface{}{
				"language":  "node",
				"framework": "express",
				"port":      3000,
			},
			assertions: []string{
				"node",
				"express",
				"3000",
				"production-ready",
				"security",
			},
		},
		{
			name:     "deploy_production",
			template: "containerKit.deploy",
			version:  "1.0.0",
			params: map[string]interface{}{
				"repo_url":    "https://github.com/org/app.git",
				"branch":      "main",
				"registry":    "ghcr.io/org/app",
				"scan":        true,
				"environment": "production",
			},
			assertions: []string{
				"production",
				"github.com/org/app.git",
				"ghcr.io/org/app",
				"10-step workflow",
				"Security Scan",
			},
		},
		{
			name:     "troubleshoot_build_error",
			template: "containerKit.troubleshoot",
			version:  "1.0.0",
			params: map[string]interface{}{
				"error_message": "Docker build failed: no such file or directory",
				"operation":     "build",
				"context":       "Building go application",
				"severity":      "high",
			},
			assertions: []string{
				"build",
				"high",
				"Root Cause Analysis",
				"Step-by-Step Resolution",
				"Alternative Approaches",
				"Prevention Strategies",
			},
		},
		{
			name:     "analyze_comprehensive",
			template: "containerKit.analyze",
			version:  "1.0.0",
			params: map[string]interface{}{
				"repo_path":               "./my-app",
				"depth":                   "comprehensive",
				"focus_areas":             []string{"dependencies", "security", "performance", "deployment"},
				"include_recommendations": true,
			},
			assertions: []string{
				"comprehensive",
				"dependencies",
				"security",
				"performance",
				"deployment",
				"Technology Stack Assessment",
				"Dependency Assessment",
				"Security Analysis",
				"Performance Considerations",
				"Deployment Readiness",
				"Recommendations",
			},
		},
		{
			name:     "k8s_manifest_production",
			template: "containerKit.k8sManifest",
			version:  "1.0.0",
			params: map[string]interface{}{
				"app_name":     "my-app",
				"image":        "ghcr.io/org/my-app:v1.0.0",
				"namespace":    "production",
				"replicas":     3,
				"port":         8080,
				"service_type": "LoadBalancer",
				"environment":  "production",
			},
			assertions: []string{
				"my-app",
				"ghcr.io/org/my-app:v1.0.0",
				"production",
				"replicas: 3",
				"LoadBalancer",
				"HorizontalPodAutoscaler",
				"NetworkPolicy",
				"securityContext",
				"livenessProbe",
				"readinessProbe",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := loader.RenderTemplate(tc.template, tc.version, tc.params)
			assert.NoError(t, err)
			assert.NotEmpty(t, result)

			// Verify all required assertions
			for _, assertion := range tc.assertions {
				assert.Contains(t, result, assertion,
					"Template output missing expected content: %s", assertion)
			}

			// Store snapshot for manual review (in real testing, you'd compare against golden files)
			t.Logf("Template %s snapshot:\n%s", tc.name, result)
		})
	}
}

// TestTemplateInfo tests template information retrieval
func TestTemplateInfo(t *testing.T) {
	loader, err := NewTemplateLoader()
	require.NoError(t, err)

	t.Run("get template info", func(t *testing.T) {
		info, err := loader.GetTemplateInfo("containerKit.quickDockerfile", "1.0.0")
		assert.NoError(t, err)
		assert.NotNil(t, info)

		assert.Equal(t, "containerKit.quickDockerfile", info.Name)
		assert.Equal(t, "1.0.0", info.Version)
		assert.NotEmpty(t, info.Description)
		assert.Equal(t, "generation", info.Category)
		assert.NotEmpty(t, info.Parameters)
		assert.Contains(t, info.Parameters, "language")
		assert.Contains(t, info.Parameters, "framework")
		assert.Contains(t, info.Parameters, "port")
		assert.NotEmpty(t, info.Metadata.Tags)
		assert.Equal(t, "intermediate", info.Metadata.Complexity)
		assert.Greater(t, info.Metadata.EstimatedTokens, 0)
	})

	t.Run("template info not found", func(t *testing.T) {
		_, err := loader.GetTemplateInfo("nonexistent", "1.0.0")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// TestTemplateCategories tests template categorization
func TestTemplateCategories(t *testing.T) {
	loader, err := NewTemplateLoader()
	require.NoError(t, err)

	expectedCategories := map[string]string{
		"containerKit.quickDockerfile": "generation",
		"containerKit.deploy":          "workflow",
		"containerKit.troubleshoot":    "debugging",
		"containerKit.analyze":         "analysis",
		"containerKit.k8sManifest":     "generation",
	}

	for templateName, expectedCategory := range expectedCategories {
		t.Run(templateName, func(t *testing.T) {
			info, err := loader.GetTemplateInfo(templateName, "")
			assert.NoError(t, err)
			assert.Equal(t, expectedCategory, info.Category,
				"Template %s has incorrect category", templateName)
		})
	}
}

// TestTemplateComplexity tests complexity ratings
func TestTemplateComplexity(t *testing.T) {
	loader, err := NewTemplateLoader()
	require.NoError(t, err)

	expectedComplexity := map[string]string{
		"containerKit.quickDockerfile": "intermediate",
		"containerKit.deploy":          "advanced",
		"containerKit.troubleshoot":    "advanced",
		"containerKit.analyze":         "advanced",
		"containerKit.k8sManifest":     "advanced",
	}

	for templateName, expectedLevel := range expectedComplexity {
		t.Run(templateName, func(t *testing.T) {
			info, err := loader.GetTemplateInfo(templateName, "")
			assert.NoError(t, err)
			assert.Equal(t, expectedLevel, info.Metadata.Complexity,
				"Template %s has incorrect complexity", templateName)
		})
	}
}

// BenchmarkTemplateOperations benchmarks template operations
func BenchmarkTemplateOperations(b *testing.B) {
	loader, err := NewTemplateLoader()
	if err != nil {
		b.Fatal(err)
	}

	params := map[string]interface{}{
		"language":  "go",
		"framework": "gin",
		"port":      8080,
	}

	b.Run("GetTemplate", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := loader.GetTemplate("containerKit.quickDockerfile", "1.0.0")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("RenderTemplate", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := loader.RenderTemplate("containerKit.quickDockerfile", "1.0.0", params)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("GetTemplateInfo", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := loader.GetTemplateInfo("containerKit.quickDockerfile", "1.0.0")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
