package analyze

import (
	"testing"

	"github.com/Azure/container-kit/pkg/core/analysis"
	"github.com/rs/zerolog"
)

// Test generateContainerizationSuggestions function
func TestGenerateContainerizationSuggestions(t *testing.T) {
	logger := zerolog.Nop()
	analyzer := NewAnalyzer(logger)

	t.Run("with language and framework", func(t *testing.T) {
		analysisResult := &analysis.AnalysisResult{
			Language:  "go",
			Framework: "gin",
			Dependencies: []analysis.Dependency{
				{Name: "github.com/gin-gonic/gin", Type: "runtime", Manager: "go"},
			},
			ConfigFiles: []analysis.ConfigFile{
				{Path: "config.yaml", Type: "env", Relevant: true},
				{Path: "app.env", Type: "env", Relevant: true},
			},
		}

		suggestions := analyzer.generateContainerizationSuggestions(analysisResult)

		if len(suggestions) == 0 {
			t.Error("Expected at least one suggestion")
		}

		// Check for language-specific suggestion
		found := false
		for _, suggestion := range suggestions {
			if contains(suggestion, "go") && contains(suggestion, "base image") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected language-specific base image suggestion")
		}

		// Check for framework suggestion
		found = false
		for _, suggestion := range suggestions {
			if contains(suggestion, "gin") && contains(suggestion, "framework") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected framework-specific suggestion")
		}

		// Check for dependencies suggestion
		found = false
		for _, suggestion := range suggestions {
			if contains(suggestion, "Dependencies") && contains(suggestion, "installed") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected dependencies suggestion")
		}

		// Check for config files suggestion
		found = false
		for _, suggestion := range suggestions {
			if contains(suggestion, "Configuration") && contains(suggestion, "environment variables") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected configuration files suggestion")
		}
	})

	t.Run("minimal analysis result", func(t *testing.T) {
		analysisResult := &analysis.AnalysisResult{
			Language:     "",
			Framework:    "",
			Dependencies: []analysis.Dependency{},
			ConfigFiles:  []analysis.ConfigFile{},
		}

		suggestions := analyzer.generateContainerizationSuggestions(analysisResult)

		// Should have no suggestions for empty analysis
		if len(suggestions) != 0 {
			t.Errorf("Expected no suggestions for minimal analysis, got %d", len(suggestions))
		}
	})

	t.Run("only language", func(t *testing.T) {
		analysisResult := &analysis.AnalysisResult{
			Language:     "python",
			Framework:    "",
			Dependencies: []analysis.Dependency{},
			ConfigFiles:  []analysis.ConfigFile{},
		}

		suggestions := analyzer.generateContainerizationSuggestions(analysisResult)

		if len(suggestions) != 1 {
			t.Errorf("Expected 1 suggestion for language-only analysis, got %d", len(suggestions))
		}

		if !contains(suggestions[0], "python") {
			t.Errorf("Expected python-specific suggestion, got: %s", suggestions[0])
		}
	})
}

// Test generateNextStepSuggestions function
func TestGenerateNextStepSuggestions(t *testing.T) {
	logger := zerolog.Nop()
	analyzer := NewAnalyzer(logger)

	t.Run("no docker or k8s files", func(t *testing.T) {
		analysisResult := &analysis.AnalysisResult{
			Language: "go",
		}

		context := &AnalysisContext{
			DockerFiles: []string{}, // No docker files
			K8sFiles:    []string{}, // No k8s files
		}

		suggestions := analyzer.generateNextStepSuggestions(analysisResult, context)

		if len(suggestions) == 0 {
			t.Error("Expected at least one suggestion")
		}

		// Should suggest generating Dockerfile
		found := false
		for _, suggestion := range suggestions {
			if contains(suggestion, "Generate a Dockerfile") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected Dockerfile generation suggestion")
		}

		// Should suggest building image
		found = false
		for _, suggestion := range suggestions {
			if contains(suggestion, "Build container image") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected build image suggestion")
		}

		// Should suggest security scanning
		found = false
		for _, suggestion := range suggestions {
			if contains(suggestion, "security vulnerabilities") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected security scanning suggestion")
		}

		// Should suggest generating K8s manifests
		found = false
		for _, suggestion := range suggestions {
			if contains(suggestion, "Generate Kubernetes manifests") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected Kubernetes manifests generation suggestion")
		}

		// Should suggest secrets scanning
		found = false
		for _, suggestion := range suggestions {
			if contains(suggestion, "Scan for secrets") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected secrets scanning suggestion")
		}
	})

	t.Run("with existing docker and k8s files", func(t *testing.T) {
		analysisResult := &analysis.AnalysisResult{
			Language: "go",
		}

		context := &AnalysisContext{
			DockerFiles: []string{"Dockerfile"},      // Has docker file
			K8sFiles:    []string{"deployment.yaml"}, // Has k8s files
		}

		suggestions := analyzer.generateNextStepSuggestions(analysisResult, context)

		if len(suggestions) == 0 {
			t.Error("Expected at least one suggestion")
		}

		// Should suggest reviewing existing Dockerfile
		found := false
		for _, suggestion := range suggestions {
			if contains(suggestion, "Review and optimize") && contains(suggestion, "Dockerfile") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected Dockerfile review suggestion")
		}

		// Should NOT suggest generating K8s manifests (since they exist)
		found = false
		for _, suggestion := range suggestions {
			if contains(suggestion, "Generate Kubernetes manifests") {
				found = true
				break
			}
		}
		if found {
			t.Error("Should not suggest generating K8s manifests when they already exist")
		}

		// Should still suggest build, security, and secrets scanning
		expectedSuggestions := []string{
			"Build container image",
			"security vulnerabilities",
			"Scan for secrets",
		}

		for _, expected := range expectedSuggestions {
			found = false
			for _, suggestion := range suggestions {
				if contains(suggestion, expected) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected suggestion containing '%s'", expected)
			}
		}
	})
}

// Note: contains function is defined in helper_functions_test.go
