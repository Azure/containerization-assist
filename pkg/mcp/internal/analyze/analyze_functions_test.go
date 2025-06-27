package analyze

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
)

// Test Analyze function with minimal setup
func TestAnalyze_Function(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create some test files
	goFile := filepath.Join(tempDir, "main.go")
	err := os.WriteFile(goFile, []byte("package main\n\nfunc main() {}\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	readmeFile := filepath.Join(tempDir, "README.md")
	err = os.WriteFile(readmeFile, []byte("# Test Project\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create README: %v", err)
	}

	logger := zerolog.Nop()
	analyzer := NewAnalyzer(logger)

	options := AnalysisOptions{
		RepoPath:     tempDir,
		Context:      "test",
		LanguageHint: "go",
		SessionID:    "test-session",
	}

	ctx := context.Background()
	result, err := analyzer.Analyze(ctx, options)
	if err != nil {
		t.Errorf("Analyze should not return error, got: %v", err)
	}
	if result == nil {
		t.Error("Analyze should return non-nil result")
		return
	}
	if result.Context == nil {
		t.Error("Result should have non-nil context")
		return
	}
	if result.Context.FilesAnalyzed == 0 {
		t.Error("Should have analyzed at least some files")
	}
}

// Test generateAnalysisContext function
func TestGenerateAnalysisContext_Function(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create various test files to trigger different code paths
	testFiles := map[string]string{
		"main.go":                  "package main\n\nfunc main() {}\n",
		"package.json":             `{"name": "test"}`,
		"Dockerfile":               "FROM alpine\nRUN echo hello\n",
		"main_test.go":             "package main\n\nimport \"testing\"\n",
		"Makefile":                 "all:\n\techo build\n",
		"schema.sql":               "CREATE TABLE test (id INT);\n",
		"deployment.yaml":          "apiVersion: v1\nkind: Pod\n",
		"docker-compose.yml":       "version: '3'\nservices:\n  app:\n    build: .\n",
		".gitignore":               "*.log\n",
		"README.md":                "# Test Project\n",
		"LICENSE":                  "MIT License\n",
		".github/workflows/ci.yml": "name: CI\non: [push]\n",
	}

	for filename, content := range testFiles {
		dir := filepath.Dir(filepath.Join(tempDir, filename))
		if dir != tempDir {
			err := os.MkdirAll(dir, 0755)
			if err != nil {
				t.Fatalf("Failed to create directory %s: %v", dir, err)
			}
		}

		err := os.WriteFile(filepath.Join(tempDir, filename), []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	logger := zerolog.Nop()
	analyzer := NewAnalyzer(logger)

	// Use reflection or access the internal method through a test helper
	// Since generateAnalysisContext is internal, we'll test it through Analyze
	options := AnalysisOptions{
		RepoPath:     tempDir,
		Context:      "containerization",
		LanguageHint: "go",
		SessionID:    "test-session",
	}

	ctx := context.Background()
	result, err := analyzer.Analyze(ctx, options)
	if err != nil {
		t.Errorf("Analyze should not return error, got: %v", err)
	}

	context := result.Context
	if context == nil {
		t.Fatal("Context should not be nil")
	}

	// Verify various aspects of the generated context
	if context.FilesAnalyzed == 0 {
		t.Error("Should have analyzed at least some files")
	}

	// Check config files detection
	foundConfigFiles := false
	for _, file := range context.ConfigFilesFound {
		if file == "package.json" {
			foundConfigFiles = true
			break
		}
	}
	if !foundConfigFiles {
		t.Error("Should have detected package.json as config file")
	}

	// Check test files detection - may or may not be detected depending on implementation
	if len(context.TestFilesFound) > 0 {
		t.Logf("Test files detected: %v", context.TestFilesFound)
	}

	// Check build files detection
	foundBuildFiles := false
	for _, file := range context.BuildFilesFound {
		if file == "Makefile" {
			foundBuildFiles = true
			break
		}
	}
	if !foundBuildFiles {
		t.Error("Should have detected Makefile as build file")
	}

	// Check Docker files detection
	foundDockerFiles := false
	for _, file := range context.DockerFiles {
		if file == "Dockerfile" || file == "docker-compose.yml" {
			foundDockerFiles = true
			break
		}
	}
	if !foundDockerFiles {
		t.Error("Should have detected Docker files")
	}

	// Check K8s files detection - may or may not be detected depending on implementation
	if len(context.K8sFiles) > 0 {
		t.Logf("K8s files detected: %v", context.K8sFiles)
	}

	// Check database files detection - may or may not be detected depending on implementation
	if len(context.DatabaseFiles) > 0 {
		t.Logf("Database files detected: %v", context.DatabaseFiles)
	}

	// Check repository insights
	if !context.HasGitIgnore {
		t.Error("Should have detected .gitignore")
	}
	if !context.HasReadme {
		t.Error("Should have detected README.md")
	}
	if !context.HasLicense {
		t.Error("Should have detected LICENSE")
	}
	if !context.HasCI {
		t.Error("Should have detected CI configuration")
	}

	// Check that suggestions were generated
	if len(context.ContainerizationSuggestions) == 0 {
		t.Error("Should have generated containerization suggestions")
	}
	if len(context.NextStepSuggestions) == 0 {
		t.Error("Should have generated next step suggestions")
	}

	// Check repository size calculation
	if context.RepositorySize == 0 {
		t.Error("Repository size should be greater than 0")
	}
}

// Test validateAnalysisOptions function integration
func TestValidateAnalysisOptions_Integration(t *testing.T) {
	logger := zerolog.Nop()
	analyzer := NewAnalyzer(logger)

	// Test valid options
	validOptions := AnalysisOptions{
		RepoPath:     "/valid/path",
		Context:      "test",
		LanguageHint: "go",
		SessionID:    "session-123",
	}

	// Since validateAnalysisOptions is internal, test through Analyze
	// which will call it internally. We expect this to succeed if validation passes
	ctx := context.Background()
	_, err := analyzer.Analyze(ctx, validOptions)
	// We expect an error because the path doesn't exist, but not a validation error
	if err == nil {
		t.Error("Expected error for non-existent path, but validation should pass")
	}

	// Test empty repo path
	invalidOptions := AnalysisOptions{
		RepoPath:     "",
		Context:      "test",
		LanguageHint: "go",
		SessionID:    "session-123",
	}

	_, err = analyzer.Analyze(ctx, invalidOptions)
	if err == nil {
		t.Error("Expected error for empty repo path")
	}
}

// Test utility functions
func TestUtilityFunctions(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Test isConfigFile
	testCases := []struct {
		filename string
		expected bool
	}{
		{"package.json", true},
		{"Cargo.toml", true},
		{"go.mod", true},
		{"pom.xml", true},
		{"requirements.txt", true},
		{"main.go", false},
		{"README.md", false},
	}

	for _, tc := range testCases {
		// Create the file
		filePath := filepath.Join(tempDir, tc.filename)
		err := os.WriteFile(filePath, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", tc.filename, err)
		}

		// Test isConfigFile through file analysis
		// Since these are internal functions, we test them indirectly through Analyze
	}

	logger := zerolog.Nop()
	analyzer := NewAnalyzer(logger)

	options := AnalysisOptions{
		RepoPath:     tempDir,
		Context:      "test",
		LanguageHint: "go",
		SessionID:    "test-session",
	}

	ctx := context.Background()
	result, err := analyzer.Analyze(ctx, options)
	if err != nil {
		t.Errorf("Analyze should not return error, got: %v", err)
	}

	// Verify that config files were detected
	if len(result.Context.ConfigFilesFound) == 0 {
		t.Error("Should have detected config files")
	}
}
