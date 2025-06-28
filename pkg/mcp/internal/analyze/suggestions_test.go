package analyze

import (
	"strings"
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
			if strings.Contains(suggestion, "go") && strings.Contains(suggestion, "base image") {
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
			if strings.Contains(suggestion, "gin") && strings.Contains(suggestion, "framework") {
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
			if strings.Contains(suggestion, "Dependencies") && strings.Contains(suggestion, "installed") {
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
			if strings.Contains(suggestion, "Configuration") && strings.Contains(suggestion, "environment variables") {
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

		if !strings.Contains(suggestions[0], "python") {
			t.Errorf("Expected python-specific suggestion, got: %s", suggestions[0])
		}
	})
}

// Helper function to check if suggestions contain expected text
func assertSuggestionContains(t *testing.T, suggestions []string, expected string) {
	t.Helper()
	for _, suggestion := range suggestions {
		if strings.Contains(suggestion, expected) {
			return
		}
	}
	t.Errorf("Expected suggestion containing '%s'", expected)
}

// Helper function to check if suggestions do NOT contain expected text
func assertSuggestionNotContains(t *testing.T, suggestions []string, notExpected string) {
	t.Helper()
	for _, suggestion := range suggestions {
		if strings.Contains(suggestion, notExpected) {
			t.Errorf("Should not have suggestion containing '%s'", notExpected)
			return
		}
	}
}

// Test generateNextStepSuggestions function
func TestGenerateNextStepSuggestions(t *testing.T) {
	logger := zerolog.Nop()
	analyzer := NewAnalyzer(logger)

	t.Run("no docker or k8s files", func(t *testing.T) {
		testGenerateNextStepSuggestionsNoFiles(t, analyzer)
	})

	t.Run("with existing docker and k8s files", func(t *testing.T) {
		testGenerateNextStepSuggestionsWithFiles(t, analyzer)
	})
}

func testGenerateNextStepSuggestionsNoFiles(t *testing.T, analyzer *Analyzer) {
	analysisResult := &analysis.AnalysisResult{Language: "go"}
	context := &AnalysisContext{
		DockerFiles: []string{}, // No docker files
		K8sFiles:    []string{}, // No k8s files
	}

	suggestions := analyzer.generateNextStepSuggestions(analysisResult, context)

	if len(suggestions) == 0 {
		t.Error("Expected at least one suggestion")
	}

	// Check expected suggestions
	expectedSuggestions := []string{
		"Generate a Dockerfile",
		"Build container image",
		"security vulnerabilities",
		"Generate Kubernetes manifests",
		"Scan for secrets",
	}

	for _, expected := range expectedSuggestions {
		assertSuggestionContains(t, suggestions, expected)
	}
}

func testGenerateNextStepSuggestionsWithFiles(t *testing.T, analyzer *Analyzer) {
	analysisResult := &analysis.AnalysisResult{Language: "go"}
	context := &AnalysisContext{
		DockerFiles: []string{"Dockerfile"},      // Has docker file
		K8sFiles:    []string{"deployment.yaml"}, // Has k8s files
	}

	suggestions := analyzer.generateNextStepSuggestions(analysisResult, context)

	if len(suggestions) == 0 {
		t.Error("Expected at least one suggestion")
	}

	// Should suggest reviewing existing Dockerfile
	assertSuggestionContains(t, suggestions, "Review and optimize")

	// Should NOT suggest generating K8s manifests (since they exist)
	assertSuggestionNotContains(t, suggestions, "Generate Kubernetes manifests")

	// Should still suggest build, security, and secrets scanning
	stillExpected := []string{
		"Build container image",
		"security vulnerabilities",
		"Scan for secrets",
	}

	for _, expected := range stillExpected {
		assertSuggestionContains(t, suggestions, expected)
	}
}

// Note: contains function is defined in helper_functions_test.go
