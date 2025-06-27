package analyze

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// Test the file classification helper functions
func TestHelperFunctions(t *testing.T) {
	logger := zerolog.Nop()
	analyzer := NewAnalyzer(logger)

	// Test isConfigFile
	t.Run("isConfigFile", func(t *testing.T) {
		tests := []struct {
			path     string
			expected bool
		}{
			{"config.yaml", true},
			{"settings.json", true},
			{".env", true},
			{"app.properties", true},
			{"data.toml", true},
			{"config.ini", true},
			{"main.go", false},
			{"README.md", false},
			{"", false},
		}

		for _, test := range tests {
			result := analyzer.isConfigFile(test.path)
			if result != test.expected {
				t.Errorf("isConfigFile(%s) = %v, expected %v", test.path, result, test.expected)
			}
		}
	})

	// Test isTestFile
	t.Run("isTestFile", func(t *testing.T) {
		tests := []struct {
			path     string
			expected bool
		}{
			{"main_test.go", true},
			{"test_helper.py", true},
			{"spec/user_spec.rb", true},
			{"integration.test.js", true},
			{"main.go", false},
			{"config.yaml", false},
			{"", false},
		}

		for _, test := range tests {
			result := analyzer.isTestFile(test.path)
			if result != test.expected {
				t.Errorf("isTestFile(%s) = %v, expected %v", test.path, result, test.expected)
			}
		}
	})

	// Test isBuildFile
	t.Run("isBuildFile", func(t *testing.T) {
		tests := []struct {
			path     string
			expected bool
		}{
			{"Makefile", true},
			{"makefile", true},
			{"build.gradle", true},
			{"pom.xml", true},
			{"package.json", true},
			{"Cargo.toml", true},
			{"go.mod", true},
			{"requirements.txt", true},
			{"Gemfile", true},
			{"build.sbt", true},
			{"project.clj", true},
			{"src/main.go", false},
			{"config.yaml", false},
			{"", false},
		}

		for _, test := range tests {
			result := analyzer.isBuildFile(test.path)
			if result != test.expected {
				t.Errorf("isBuildFile(%s) = %v, expected %v", test.path, result, test.expected)
			}
		}
	})

	// Test isK8sFile
	t.Run("isK8sFile", func(t *testing.T) {
		tests := []struct {
			path     string
			expected bool
		}{
			{"deployment.yaml", true},
			{"service.yml", true},
			{"ingress.yaml", true},
			{"configmap.yml", true},
			{"secret.yaml", true},
			{"app-k8s.yaml", true},
			{"deployment.json", false}, // Must be yaml/yml
			{"main.go", false},
			{"config.properties", false},
			{"", false},
		}

		for _, test := range tests {
			result := analyzer.isK8sFile(test.path)
			if result != test.expected {
				t.Errorf("isK8sFile(%s) = %v, expected %v", test.path, result, test.expected)
			}
		}
	})

	// Test isDatabaseFile
	t.Run("isDatabaseFile", func(t *testing.T) {
		tests := []struct {
			path     string
			expected bool
		}{
			{"schema.sql", true},
			{"migration_001.sql", true},
			{"database.db", true},
			{"data.sqlite", true},
			{"schema_migration.rb", true},
			{"main.go", false},
			{"config.yaml", false},
			{"", false},
		}

		for _, test := range tests {
			result := analyzer.isDatabaseFile(test.path)
			if result != test.expected {
				t.Errorf("isDatabaseFile(%s) = %v, expected %v", test.path, result, test.expected)
			}
		}
	})
}

// Test fileExists function
func TestFileExists(t *testing.T) {
	logger := zerolog.Nop()
	analyzer := NewAnalyzer(logger)

	// Create a temporary file
	tmpFile, err := ioutil.TempFile("", "test_file_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Test existing file
	if !analyzer.fileExists(tmpFile.Name()) {
		t.Error("fileExists should return true for existing file")
	}

	// Test non-existing file
	if analyzer.fileExists("/non/existent/file.txt") {
		t.Error("fileExists should return false for non-existing file")
	}

	// Test empty path
	if analyzer.fileExists("") {
		t.Error("fileExists should return false for empty path")
	}
}

// Test validateAnalysisOptions function
func TestValidateAnalysisOptions(t *testing.T) {
	logger := zerolog.Nop()
	analyzer := NewAnalyzer(logger)

	// Create a temporary directory
	tmpDir, err := ioutil.TempDir("", "test_repo_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("valid options", func(t *testing.T) {
		opts := AnalysisOptions{
			RepoPath:     tmpDir,
			Context:      "test",
			LanguageHint: "go",
			SessionID:    "session-123",
		}

		err := analyzer.validateAnalysisOptions(opts)
		if err != nil {
			t.Errorf("validateAnalysisOptions should not return error for valid options: %v", err)
		}
	})

	t.Run("empty repo path", func(t *testing.T) {
		opts := AnalysisOptions{
			RepoPath:  "",
			Context:   "test",
			SessionID: "session-123",
		}

		err := analyzer.validateAnalysisOptions(opts)
		if err == nil {
			t.Error("validateAnalysisOptions should return error for empty repo path")
		}
		if err.Error() != "repository path is required" {
			t.Errorf("Expected 'repository path is required', got %s", err.Error())
		}
	})

	t.Run("non-existent repo path", func(t *testing.T) {
		opts := AnalysisOptions{
			RepoPath:  "/non/existent/path",
			Context:   "test",
			SessionID: "session-123",
		}

		err := analyzer.validateAnalysisOptions(opts)
		if err == nil {
			t.Error("validateAnalysisOptions should return error for non-existent repo path")
		}
		if !contains(err.Error(), "repository path does not exist") {
			t.Errorf("Error should mention path doesn't exist, got: %s", err.Error())
		}
	})
}

// Test additional file helper functions
func TestAdditionalFileHelpers(t *testing.T) {
	logger := zerolog.Nop()
	analyzer := NewAnalyzer(logger)

	// Create a temporary directory structure
	tmpDir, err := ioutil.TempDir("", "test_repo_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	testFiles := map[string]string{
		"README.md":                "# Test Repository",
		"readme.txt":               "Test readme",
		"LICENSE":                  "MIT License",
		"license.txt":              "License content",
		".github/workflows/ci.yml": "name: CI",
		".gitlab-ci.yml":           "stages: [test]",
		"jenkins.file":             "pipeline {}",
	}

	for fileName, content := range testFiles {
		filePath := filepath.Join(tmpDir, fileName)
		err := os.MkdirAll(filepath.Dir(filePath), 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		err = ioutil.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", fileName, err)
		}
	}

	// Test hasReadmeFile
	if !analyzer.hasReadmeFile(tmpDir) {
		t.Error("hasReadmeFile should return true when README files exist")
	}

	// Test hasLicenseFile
	if !analyzer.hasLicenseFile(tmpDir) {
		t.Error("hasLicenseFile should return true when LICENSE files exist")
	}

	// Test hasCIConfig
	if !analyzer.hasCIConfig(tmpDir) {
		t.Error("hasCIConfig should return true when CI config files exist")
	}

	// Test with directory that has no such files
	emptyDir, err := ioutil.TempDir("", "empty_repo_*")
	if err != nil {
		t.Fatalf("Failed to create empty temp dir: %v", err)
	}
	defer os.RemoveAll(emptyDir)

	if analyzer.hasReadmeFile(emptyDir) {
		t.Error("hasReadmeFile should return false for empty directory")
	}
	if analyzer.hasLicenseFile(emptyDir) {
		t.Error("hasLicenseFile should return false for empty directory")
	}
	if analyzer.hasCIConfig(emptyDir) {
		t.Error("hasCIConfig should return false for empty directory")
	}
}

// Test calculateDirectorySize function
func TestCalculateDirectorySize(t *testing.T) {
	logger := zerolog.Nop()
	analyzer := NewAnalyzer(logger)

	// Create a temporary directory with known files
	tmpDir, err := ioutil.TempDir("", "test_size_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files with known sizes
	testFiles := map[string]string{
		"file1.txt":      "Hello World", // 11 bytes
		"file2.txt":      "Test",        // 4 bytes
		"dir1/file3.txt": "Content",     // 7 bytes
	}

	expectedSize := int64(0)
	for fileName, content := range testFiles {
		filePath := filepath.Join(tmpDir, fileName)
		err := os.MkdirAll(filepath.Dir(filePath), 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		err = ioutil.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", fileName, err)
		}
		expectedSize += int64(len(content))
	}

	// Test directory size calculation
	size, err := analyzer.calculateDirectorySize(tmpDir)
	if err != nil {
		t.Errorf("calculateDirectorySize should not return error: %v", err)
	}
	if size != expectedSize {
		t.Errorf("Expected directory size to be %d, got %d", expectedSize, size)
	}

	// Test with non-existent directory
	_, err = analyzer.calculateDirectorySize("/non/existent/path")
	if err == nil {
		t.Error("calculateDirectorySize should return error for non-existent directory")
	}

	// Test with empty directory
	emptyDir, err := ioutil.TempDir("", "empty_*")
	if err != nil {
		t.Fatalf("Failed to create empty temp dir: %v", err)
	}
	defer os.RemoveAll(emptyDir)

	size, err = analyzer.calculateDirectorySize(emptyDir)
	if err != nil {
		t.Errorf("calculateDirectorySize should not return error for empty directory: %v", err)
	}
	if size != 0 {
		t.Errorf("Expected empty directory size to be 0, got %d", size)
	}
}

// Test AnalyzeRepositoryRedirectTool metadata, constructor, and validation
func TestAnalyzeRepositoryRedirectTool(t *testing.T) {
	logger := zerolog.Nop()

	// Create an atomic tool instance (can be nil for metadata test)
	var atomicTool *AtomicAnalyzeRepositoryTool

	// Test constructor
	tool := NewAnalyzeRepositoryRedirectTool(atomicTool, logger)
	if tool == nil {
		t.Error("NewAnalyzeRepositoryRedirectTool should not return nil")
	}
	if tool.atomicTool != atomicTool {
		t.Error("atomicTool field not set correctly")
	}

	// Test GetMetadata
	metadata := tool.GetMetadata()
	if metadata.Name != "analyze_repository" {
		t.Errorf("Expected Name to be 'analyze_repository', got %s", metadata.Name)
	}
	if metadata.Version != "1.0.0" {
		t.Errorf("Expected Version to be '1.0.0', got %s", metadata.Version)
	}
	if metadata.Category != "analysis" {
		t.Errorf("Expected Category to be 'analysis', got %s", metadata.Category)
	}
	if len(metadata.Dependencies) == 0 {
		t.Error("Expected Dependencies to be non-empty")
	}
	if len(metadata.Capabilities) == 0 {
		t.Error("Expected Capabilities to be non-empty")
	}
	if len(metadata.Requirements) == 0 {
		t.Error("Expected Requirements to be non-empty")
	}

	// Test specific capabilities
	expectedCapabilities := []string{
		"language_detection",
		"framework_analysis",
		"dependency_scanning",
		"structure_analysis",
		"containerization_assessment",
	}
	for _, capability := range expectedCapabilities {
		found := false
		for _, c := range metadata.Capabilities {
			if c == capability {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected capability '%s' not found", capability)
		}
	}

	// Test Validate function
	t.Run("Validate", func(t *testing.T) {
		ctx := context.Background()

		// Test valid arguments with repo_path
		validArgs := map[string]interface{}{
			"session_id": "session-123",
			"repo_path":  "/tmp/test-repo",
		}
		err := tool.Validate(ctx, validArgs)
		if err != nil {
			t.Errorf("Validate should not return error for valid args: %v", err)
		}

		// Test valid arguments with path instead of repo_path
		validArgsPath := map[string]interface{}{
			"session_id": "session-123",
			"path":       "/tmp/test-repo",
		}
		err = tool.Validate(ctx, validArgsPath)
		if err != nil {
			t.Errorf("Validate should not return error for valid args with path: %v", err)
		}

		// Test valid arguments without session_id (should be optional)
		validArgsNoSession := map[string]interface{}{
			"repo_path": "/tmp/test-repo",
		}
		err = tool.Validate(ctx, validArgsNoSession)
		if err != nil {
			t.Errorf("Validate should not return error when session_id is missing: %v", err)
		}

		// Test invalid argument type
		invalidArgs := "not a map"
		err = tool.Validate(ctx, invalidArgs)
		if err == nil {
			t.Error("Validate should return error for invalid argument type")
		}
		if !contains(err.Error(), "invalid argument type") {
			t.Errorf("Error should mention invalid argument type, got: %s", err.Error())
		}

		// Test missing repo_path and path
		missingPathArgs := map[string]interface{}{
			"session_id": "session-123",
		}
		err = tool.Validate(ctx, missingPathArgs)
		if err == nil {
			t.Error("Validate should return error when both repo_path and path are missing")
		}
		if !contains(err.Error(), "repo_path or path is required") {
			t.Errorf("Error should mention required path, got: %s", err.Error())
		}

		// Test empty repo_path and path
		emptyPathArgs := map[string]interface{}{
			"session_id": "session-123",
			"repo_path":  "",
			"path":       "",
		}
		err = tool.Validate(ctx, emptyPathArgs)
		if err == nil {
			t.Error("Validate should return error when both repo_path and path are empty")
		}
		if !contains(err.Error(), "repo_path or path is required") {
			t.Errorf("Error should mention required path, got: %s", err.Error())
		}
	})
}

// Test types and constructors
func TestAnalysisTypes(t *testing.T) {
	// Test AnalysisOptions
	opts := AnalysisOptions{
		RepoPath:     "/tmp/test",
		Context:      "test context",
		LanguageHint: "go",
		SessionID:    "session-123",
	}

	if opts.RepoPath != "/tmp/test" {
		t.Errorf("Expected RepoPath to be '/tmp/test', got %s", opts.RepoPath)
	}
	if opts.Context != "test context" {
		t.Errorf("Expected Context to be 'test context', got %s", opts.Context)
	}
	if opts.LanguageHint != "go" {
		t.Errorf("Expected LanguageHint to be 'go', got %s", opts.LanguageHint)
	}
	if opts.SessionID != "session-123" {
		t.Errorf("Expected SessionID to be 'session-123', got %s", opts.SessionID)
	}

	// Test AnalysisContext
	context := &AnalysisContext{
		FilesAnalyzed:    10,
		ConfigFilesFound: []string{"config.yaml"},
		EntryPointsFound: []string{"main.go"},
		TestFilesFound:   []string{"main_test.go"},
		BuildFilesFound:  []string{"go.mod"},
		PackageManagers:  []string{"go"},
	}

	if context.FilesAnalyzed != 10 {
		t.Errorf("Expected FilesAnalyzed to be 10, got %d", context.FilesAnalyzed)
	}
	if len(context.ConfigFilesFound) != 1 || context.ConfigFilesFound[0] != "config.yaml" {
		t.Errorf("ConfigFilesFound not set correctly: %v", context.ConfigFilesFound)
	}
	if len(context.PackageManagers) != 1 || context.PackageManagers[0] != "go" {
		t.Errorf("PackageManagers not set correctly: %v", context.PackageManagers)
	}
}

// Test clone-related functions
func TestCloner(t *testing.T) {
	logger := zerolog.Nop()
	cloner := NewCloner(logger)

	// Test constructor
	if cloner == nil {
		t.Error("NewCloner should not return nil")
	}

	// Test isURL function
	t.Run("isURL", func(t *testing.T) {
		tests := []struct {
			path     string
			expected bool
		}{
			{"https://github.com/user/repo.git", true},
			{"http://gitlab.com/user/repo.git", true},
			{"git@github.com:user/repo.git", true},
			{"anything.github.com/path", true},
			{"anything.gitlab.com/path", true},
			{"/local/path", false},
			{"./relative/path", false},
			{"file.txt", false},
			{"", false},
		}

		for _, test := range tests {
			result := cloner.isURL(test.path)
			if result != test.expected {
				t.Errorf("isURL(%s) = %v, expected %v", test.path, result, test.expected)
			}
		}
	})

	// Test validateCloneOptions function
	t.Run("validateCloneOptions", func(t *testing.T) {
		// Test valid local path options
		validLocalOpts := CloneOptions{
			RepoURL:   "/local/path",
			Branch:    "main",
			SessionID: "session-123",
		}
		err := cloner.validateCloneOptions(validLocalOpts)
		if err != nil {
			t.Errorf("validateCloneOptions should not return error for valid local options: %v", err)
		}

		// Test valid URL options
		validURLOpts := CloneOptions{
			RepoURL:   "https://github.com/user/repo.git",
			Branch:    "main",
			TargetDir: "/tmp/target",
			SessionID: "session-123",
		}
		err = cloner.validateCloneOptions(validURLOpts)
		if err != nil {
			t.Errorf("validateCloneOptions should not return error for valid URL options: %v", err)
		}

		// Test empty repo URL
		emptyRepoOpts := CloneOptions{
			RepoURL:   "",
			TargetDir: "/tmp/target",
		}
		err = cloner.validateCloneOptions(emptyRepoOpts)
		if err == nil {
			t.Error("validateCloneOptions should return error for empty repo URL")
		}
		if !contains(err.Error(), "repository URL or path is required") {
			t.Errorf("Error should mention required repo URL, got: %s", err.Error())
		}

		// Test URL without target directory
		urlNoTargetOpts := CloneOptions{
			RepoURL: "https://github.com/user/repo.git",
			Branch:  "main",
		}
		err = cloner.validateCloneOptions(urlNoTargetOpts)
		if err == nil {
			t.Error("validateCloneOptions should return error for URL without target directory")
		}
		if !contains(err.Error(), "target directory is required") {
			t.Errorf("Error should mention required target directory, got: %s", err.Error())
		}
	})
}

// Test CloneOptions and CloneResult types
func TestCloneTypes(t *testing.T) {
	// Test CloneOptions
	opts := CloneOptions{
		RepoURL:   "https://github.com/user/repo.git",
		Branch:    "main",
		Shallow:   true,
		TargetDir: "/tmp/target",
		SessionID: "session-123",
	}

	if opts.RepoURL != "https://github.com/user/repo.git" {
		t.Errorf("Expected RepoURL to be 'https://github.com/user/repo.git', got %s", opts.RepoURL)
	}
	if opts.Branch != "main" {
		t.Errorf("Expected Branch to be 'main', got %s", opts.Branch)
	}
	if !opts.Shallow {
		t.Error("Expected Shallow to be true")
	}
	if opts.TargetDir != "/tmp/target" {
		t.Errorf("Expected TargetDir to be '/tmp/target', got %s", opts.TargetDir)
	}
	if opts.SessionID != "session-123" {
		t.Errorf("Expected SessionID to be 'session-123', got %s", opts.SessionID)
	}

	// Test CloneResult
	duration := time.Second * 5
	result := &CloneResult{
		CloneResult: nil, // git.CloneResult would be nil in test
		Duration:    duration,
	}

	if result.Duration != duration {
		t.Errorf("Expected Duration to be %v, got %v", duration, result.Duration)
	}
}

// Helper function for string contains check (reused from other tests)
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
