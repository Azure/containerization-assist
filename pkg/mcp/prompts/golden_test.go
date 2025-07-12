package prompts

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var updateGolden = flag.Bool("update-golden", false, "Update golden test files")

// TestGoldenTemplates tests template outputs against golden files
func TestGoldenTemplates(t *testing.T) {
	loader, err := NewTemplateLoader()
	require.NoError(t, err)

	testCases := []struct {
		name     string
		template string
		version  string
		params   map[string]interface{}
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
		},
		{
			name:     "dockerfile_python_fastapi",
			template: "containerKit.quickDockerfile",
			version:  "1.0.0",
			params: map[string]interface{}{
				"language":  "python",
				"framework": "fastapi",
				"port":      8000,
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
		},
		{
			name:     "deploy_staging",
			template: "containerKit.deploy",
			version:  "1.0.0",
			params: map[string]interface{}{
				"repo_url":    "https://github.com/org/app.git",
				"branch":      "develop",
				"registry":    "ghcr.io/org/app",
				"scan":        true,
				"environment": "staging",
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
		},
		{
			name:     "troubleshoot_deployment_error",
			template: "containerKit.troubleshoot",
			version:  "1.0.0",
			params: map[string]interface{}{
				"error_message": "kubectl apply failed: connection refused",
				"operation":     "deploy",
				"context":       "Deploying to kubernetes cluster",
				"severity":      "critical",
			},
		},
		{
			name:     "analyze_quick",
			template: "containerKit.analyze",
			version:  "1.0.0",
			params: map[string]interface{}{
				"repo_path":               "./my-app",
				"depth":                   "quick",
				"focus_areas":             []string{"dependencies"},
				"include_recommendations": false,
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
		},
		{
			name:     "k8s_manifest_simple",
			template: "containerKit.k8sManifest",
			version:  "1.0.0",
			params: map[string]interface{}{
				"app_name":     "my-app",
				"image":        "nginx:latest",
				"namespace":    "default",
				"replicas":     1,
				"port":         80,
				"service_type": "ClusterIP",
				"environment":  "development",
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
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Render the template
			result, err := loader.RenderTemplate(tc.template, tc.version, tc.params)
			require.NoError(t, err)
			require.NotEmpty(t, result)

			// Golden file path
			goldenPath := filepath.Join("testdata", "golden", tc.name+".golden")

			if *updateGolden {
				// Update mode: write the current output as the new golden file
				err := os.MkdirAll(filepath.Dir(goldenPath), 0755)
				require.NoError(t, err)

				err = os.WriteFile(goldenPath, []byte(result), 0644)
				require.NoError(t, err)

				t.Logf("Updated golden file: %s", goldenPath)
			} else {
				// Test mode: compare against the golden file
				golden, err := os.ReadFile(goldenPath)
				if os.IsNotExist(err) {
					t.Fatalf("Golden file does not exist: %s\nRun with -update-golden to create it", goldenPath)
				}
				require.NoError(t, err)

				// Normalize line endings for cross-platform compatibility
				expected := normalizeLineEndings(string(golden))
				actual := normalizeLineEndings(result)

				if expected != actual {
					// If they don't match, write the actual output to a .actual file for debugging
					actualPath := goldenPath + ".actual"
					os.WriteFile(actualPath, []byte(result), 0644)

					t.Errorf("Template output does not match golden file.\nGolden: %s\nActual: %s\n\nDiff:\n%s",
						goldenPath, actualPath, generateDiff(expected, actual))
				}
			}
		})
	}
}

// TestGoldenTemplateStability ensures templates produce consistent output
func TestGoldenTemplateStability(t *testing.T) {
	loader, err := NewTemplateLoader()
	require.NoError(t, err)

	// Test each template multiple times to ensure stability
	iterations := 5

	testCases := []struct {
		template string
		version  string
		params   map[string]interface{}
	}{
		{
			template: "containerKit.quickDockerfile",
			version:  "1.0.0",
			params: map[string]interface{}{
				"language":  "go",
				"framework": "gin",
				"port":      8080,
			},
		},
		{
			template: "containerKit.deploy",
			version:  "1.0.0",
			params: map[string]interface{}{
				"repo_url":    "https://github.com/org/app.git",
				"branch":      "main",
				"registry":    "ghcr.io/org/app",
				"environment": "production",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.template, func(t *testing.T) {
			var firstResult string

			for i := 0; i < iterations; i++ {
				result, err := loader.RenderTemplate(tc.template, tc.version, tc.params)
				require.NoError(t, err)

				if i == 0 {
					firstResult = result
				} else {
					assert.Equal(t, firstResult, result,
						"Template produced different output on iteration %d", i+1)
				}
			}
		})
	}
}

// TestGoldenTemplateParameterVariations tests templates with various parameter combinations
func TestGoldenTemplateParameterVariations(t *testing.T) {
	loader, err := NewTemplateLoader()
	require.NoError(t, err)

	// Test dockerfile template with different languages and frameworks
	languages := []string{"go", "node", "python", "java", "rust"}
	frameworks := map[string][]string{
		"go":     {"gin", "echo", "fiber", "chi"},
		"node":   {"express", "fastify", "koa", "hapi"},
		"python": {"fastapi", "flask", "django", "pyramid"},
		"java":   {"spring", "quarkus", "micronaut", "helidon"},
		"rust":   {"actix", "rocket", "warp", "axum"},
	}

	for _, lang := range languages {
		for _, fw := range frameworks[lang] {
			testName := fmt.Sprintf("dockerfile_%s_%s", lang, fw)
			t.Run(testName, func(t *testing.T) {
				params := map[string]interface{}{
					"language":  lang,
					"framework": fw,
					"port":      8080,
				}

				result, err := loader.RenderTemplate("containerKit.quickDockerfile", "1.0.0", params)
				require.NoError(t, err)
				require.NotEmpty(t, result)

				// Verify language-specific content
				assert.Contains(t, result, lang)
				assert.Contains(t, result, fw)

				// Check for common best practices
				assert.Contains(t, result, "multi-stage")
				assert.Contains(t, result, "security")
			})
		}
	}
}

// normalizeLineEndings converts all line endings to \n for consistent comparison
func normalizeLineEndings(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimSpace(s)
}

// generateDiff generates a simple diff between two strings
func generateDiff(expected, actual string) string {
	expectedLines := strings.Split(expected, "\n")
	actualLines := strings.Split(actual, "\n")

	var diff bytes.Buffer
	maxLines := len(expectedLines)
	if len(actualLines) > maxLines {
		maxLines = len(actualLines)
	}

	for i := 0; i < maxLines; i++ {
		if i >= len(expectedLines) {
			diff.WriteString(fmt.Sprintf("+%d: %s\n", i+1, actualLines[i]))
		} else if i >= len(actualLines) {
			diff.WriteString(fmt.Sprintf("-%d: %s\n", i+1, expectedLines[i]))
		} else if expectedLines[i] != actualLines[i] {
			diff.WriteString(fmt.Sprintf("-%d: %s\n", i+1, expectedLines[i]))
			diff.WriteString(fmt.Sprintf("+%d: %s\n", i+1, actualLines[i]))
		}
	}

	return diff.String()
}