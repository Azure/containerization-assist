package analyze

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// Test BuildAnalyzer constructor
func TestNewBuildAnalyzer(t *testing.T) {
	logger := zerolog.Nop()
	analyzer := NewBuildAnalyzer(logger)

	if analyzer == nil {
		t.Error("NewBuildAnalyzer should not return nil")
	}
	if analyzer.GetName() != "build_analyzer" {
		t.Errorf("Expected name to be 'build_analyzer', got '%s'", analyzer.GetName())
	}
}

// Test BuildAnalyzer capabilities
func TestBuildAnalyzer_GetCapabilities(t *testing.T) {
	logger := zerolog.Nop()
	analyzer := NewBuildAnalyzer(logger)

	capabilities := analyzer.GetCapabilities()
	if len(capabilities) == 0 {
		t.Error("Expected capabilities to be non-empty")
	}

	expectedCapabilities := []string{
		"build_systems",
		"entry_points",
		"build_scripts",
		"ci_cd_configuration",
		"containerization_readiness",
		"deployment_artifacts",
	}

	for _, expected := range expectedCapabilities {
		found := false
		for _, capability := range capabilities {
			if capability == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected capability '%s' not found in capabilities", expected)
		}
	}
}

// Test BuildAnalyzer IsApplicable
func TestBuildAnalyzer_IsApplicable(t *testing.T) {
	logger := zerolog.Nop()
	analyzer := NewBuildAnalyzer(logger)
	ctx := context.Background()

	// Build analysis should always be applicable
	applicable := analyzer.IsApplicable(ctx, nil)
	if !applicable {
		t.Error("Build analyzer should always be applicable")
	}

	// Test with empty repo data
	repoData := &RepoData{}
	applicable = analyzer.IsApplicable(ctx, repoData)
	if !applicable {
		t.Error("Build analyzer should be applicable even with empty repo data")
	}
}

// Test CloneOptions structure
func TestCloneOptions_Structure(t *testing.T) {
	opts := CloneOptions{
		RepoURL:   "https://github.com/example/repo.git",
		Branch:    "main",
		Shallow:   true,
		TargetDir: "/tmp/test-repo",
		SessionID: "session-123",
	}

	if opts.RepoURL != "https://github.com/example/repo.git" {
		t.Errorf("Expected RepoURL to be 'https://github.com/example/repo.git', got '%s'", opts.RepoURL)
	}
	if opts.Branch != "main" {
		t.Errorf("Expected Branch to be 'main', got '%s'", opts.Branch)
	}
	if !opts.Shallow {
		t.Error("Expected Shallow to be true")
	}
	if opts.TargetDir != "/tmp/test-repo" {
		t.Errorf("Expected TargetDir to be '/tmp/test-repo', got '%s'", opts.TargetDir)
	}
	if opts.SessionID != "session-123" {
		t.Errorf("Expected SessionID to be 'session-123', got '%s'", opts.SessionID)
	}
}

// Test CloneResult structure
func TestCloneResult_Structure(t *testing.T) {
	result := CloneResult{
		Duration: 30 * time.Second,
	}

	if result.Duration != 30*time.Second {
		t.Errorf("Expected Duration to be 30s, got %v", result.Duration)
	}
}

// Test AnalysisOptions structure
func TestAnalysisOptions_Structure(t *testing.T) {
	opts := AnalysisOptions{
		RepoPath:     "/path/to/repo",
		Context:      "containerization",
		LanguageHint: "go",
		SessionID:    "analysis-session-456",
	}

	if opts.RepoPath != "/path/to/repo" {
		t.Errorf("Expected RepoPath to be '/path/to/repo', got '%s'", opts.RepoPath)
	}
	if opts.Context != "containerization" {
		t.Errorf("Expected Context to be 'containerization', got '%s'", opts.Context)
	}
	if opts.LanguageHint != "go" {
		t.Errorf("Expected LanguageHint to be 'go', got '%s'", opts.LanguageHint)
	}
	if opts.SessionID != "analysis-session-456" {
		t.Errorf("Expected SessionID to be 'analysis-session-456', got '%s'", opts.SessionID)
	}
}

// Test AnalysisResult structure
func TestAnalysisResult_Structure(t *testing.T) {
	result := AnalysisResult{
		Duration: 2 * time.Minute,
		Context: &AnalysisContext{
			FilesAnalyzed:    100,
			ConfigFilesFound: []string{"package.json", "Dockerfile"},
			EntryPointsFound: []string{"main.go", "app.js"},
		},
	}

	if result.Duration != 2*time.Minute {
		t.Errorf("Expected Duration to be 2m, got %v", result.Duration)
	}
	if result.Context == nil {
		t.Error("Expected Context to not be nil")
	}
	if result.Context.FilesAnalyzed != 100 {
		t.Errorf("Expected FilesAnalyzed to be 100, got %d", result.Context.FilesAnalyzed)
	}
}

// Test AnalysisContext structure
func TestAnalysisContext_Structure(t *testing.T) {
	context := AnalysisContext{
		FilesAnalyzed:    250,
		ConfigFilesFound: []string{"package.json", "Cargo.toml", "go.mod"},
		EntryPointsFound: []string{"main.go", "index.js"},
		TestFilesFound:   []string{"main_test.go", "app.test.js"},
		BuildFilesFound:  []string{"Makefile", "build.sh"},
		PackageManagers:  []string{"npm", "cargo", "go"},
		DatabaseFiles:    []string{"schema.sql", "migrations/"},
		DockerFiles:      []string{"Dockerfile", "docker-compose.yml"},
		K8sFiles:         []string{"deployment.yaml", "service.yaml"},
		HasGitIgnore:     true,
		HasReadme:        true,
		HasLicense:       false,
		HasCI:            true,
		RepositorySize:   1024000,
		ContainerizationSuggestions: []string{
			"Use multi-stage Docker build",
			"Consider distroless base image",
		},
		NextStepSuggestions: []string{
			"Set up CI/CD pipeline",
			"Add health checks",
		},
	}

	if context.FilesAnalyzed != 250 {
		t.Errorf("Expected FilesAnalyzed to be 250, got %d", context.FilesAnalyzed)
	}
	if len(context.ConfigFilesFound) != 3 {
		t.Errorf("Expected 3 config files, got %d", len(context.ConfigFilesFound))
	}
	if len(context.ContainerizationSuggestions) != 2 {
		t.Errorf("Expected 2 containerization suggestions, got %d", len(context.ContainerizationSuggestions))
	}
	if !context.HasGitIgnore {
		t.Error("Expected HasGitIgnore to be true")
	}
	if context.HasLicense {
		t.Error("Expected HasLicense to be false")
	}
	if context.RepositorySize != 1024000 {
		t.Errorf("Expected RepositorySize to be 1024000, got %d", context.RepositorySize)
	}
}
