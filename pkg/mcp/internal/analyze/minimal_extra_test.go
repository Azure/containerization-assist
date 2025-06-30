package analyze

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
)

// Targeted test to reach 15% coverage - test generateAnalysisContext function indirectly
func TestGenerateAnalysisContext_Minimal(t *testing.T) {
	tempDir := t.TempDir()

	// Create minimal test files to trigger different detection logic
	files := map[string]string{
		"test.go":                    "package main\nfunc main() {}\n",
		"package.json":               `{"name":"test"}`,
		"go.mod":                     "module test\n",
		"Dockerfile":                 "FROM alpine\n",
		"README.md":                  "# Test\n",
		"LICENSE":                    "MIT\n",
		".gitignore":                 "*.log\n",
		".github/workflows/test.yml": "name: test\n",
	}

	for name, content := range files {
		fullPath := filepath.Join(tempDir, name)
		dir := filepath.Dir(fullPath)
		if dir != tempDir {
			os.MkdirAll(dir, 0755)
		}
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	logger := zerolog.Nop()
	analyzer := NewAnalyzer(logger)
	ctx := context.Background()

	// This should trigger the internal generateAnalysisContext function
	result, err := analyzer.Analyze(ctx, AnalysisOptions{
		RepoPath:     tempDir,
		Context:      "test",
		LanguageHint: "go",
		SessionID:    "test",
	})

	if err != nil {
		t.Errorf("Analysis failed: %v", err)
	}
	if result == nil || result.Context == nil {
		t.Error("Should return valid result with context")
	}
}

// Test to trigger validation and utility functions
func TestValidationAndUtilityFunctions(t *testing.T) {
	logger := zerolog.Nop()
	analyzer := NewAnalyzer(logger)
	ctx := context.Background()

	// Test various invalid scenarios to trigger different validation code paths
	testCases := []AnalysisOptions{
		{RepoPath: "", Context: "test", LanguageHint: "go", SessionID: "test"},
		{RepoPath: "/nonexistent", Context: "", LanguageHint: "go", SessionID: "test"},
		{RepoPath: "/nonexistent", Context: "test", LanguageHint: "", SessionID: "test"},
		{RepoPath: "/nonexistent", Context: "test", LanguageHint: "go", SessionID: ""},
	}

	for i, opts := range testCases {
		_, err := analyzer.Analyze(ctx, opts)
		if err == nil {
			t.Errorf("Test case %d should have failed validation", i)
		}
	}
}

// Test more utility functions indirectly through successful analysis
func TestUtilityFunctionsIndirect(t *testing.T) {
	tempDir := t.TempDir()

	// Create files that will trigger different utility function calls
	testFiles := []string{
		"main.go", "main_test.go", "Makefile", "build.sh",
		"schema.sql", "config.yaml", "docker-compose.yml",
		"deployment.yml", "service.yaml",
	}

	for _, fileName := range testFiles {
		filePath := filepath.Join(tempDir, fileName)
		os.WriteFile(filePath, []byte("test content"), 0644)
	}

	logger := zerolog.Nop()
	analyzer := NewAnalyzer(logger)
	ctx := context.Background()

	result, err := analyzer.Analyze(ctx, AnalysisOptions{
		RepoPath:     tempDir,
		Context:      "containerization",
		LanguageHint: "go",
		SessionID:    "utility-test",
	})

	if err != nil {
		t.Errorf("Analysis should succeed: %v", err)
	}
	if result == nil {
		t.Error("Should return non-nil result")
	}

	// This exercises multiple utility functions:
	// - isConfigFile, isTestFile, isBuildFile, isK8sFile, isDatabaseFile
	// - fileExists, hasReadmeFile, hasLicenseFile, hasCIConfig
	// - calculateDirectorySize
	// - generateContainerizationSuggestions, generateNextStepSuggestions
}
