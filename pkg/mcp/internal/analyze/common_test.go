package analyze

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
)

// Test RepoData structure with languages
func TestRepoData_WithLanguages(t *testing.T) {
	repoData := RepoData{
		Path: "/test/repo",
		Files: []FileData{
			{
				Path:    "main.go",
				Content: "package main\n\nfunc main() {}\n",
				Size:    23,
			},
			{
				Path:    "README.md",
				Content: "# Test Repository\n",
				Size:    18,
			},
		},
		Languages: map[string]float64{
			"Go":       80.5,
			"Markdown": 19.5,
		},
		Structure: map[string]interface{}{
			"has_dockerfile": true,
			"has_tests":      false,
			"depth":          2,
		},
	}

	if repoData.Path != "/test/repo" {
		t.Errorf("Expected Path to be '/test/repo', got '%s'", repoData.Path)
	}
	if len(repoData.Files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(repoData.Files))
	}
	if repoData.Files[0].Path != "main.go" {
		t.Errorf("Expected first file to be 'main.go', got '%s'", repoData.Files[0].Path)
	}
	if repoData.Files[0].Size != 23 {
		t.Errorf("Expected first file size to be 23, got %d", repoData.Files[0].Size)
	}
	if len(repoData.Languages) != 2 {
		t.Errorf("Expected 2 languages, got %d", len(repoData.Languages))
	}
	if repoData.Languages["Go"] != 80.5 {
		t.Errorf("Expected Go percentage to be 80.5, got %f", repoData.Languages["Go"])
	}
	if repoData.Structure["has_dockerfile"] != true {
		t.Error("Expected has_dockerfile to be true")
	}
}

// Test FileData content handling
func TestFileData_ContentHandling(t *testing.T) {
	content := "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}\n"
	fileData := FileData{
		Path:    "pkg/main.go",
		Content: content,
		Size:    int64(len(content)),
	}

	if fileData.Path != "pkg/main.go" {
		t.Errorf("Expected Path to be 'pkg/main.go', got '%s'", fileData.Path)
	}
	if len(fileData.Content) != int(fileData.Size) {
		t.Errorf("Expected Content length to match Size, got %d vs %d", len(fileData.Content), fileData.Size)
	}
	if fileData.Size != int64(len(content)) {
		t.Errorf("Expected Size to be %d, got %d", len(content), fileData.Size)
	}
}

// Test AnalysisConfig with options
func TestAnalysisConfig_WithOptions(t *testing.T) {
	logger := zerolog.Nop()
	repoData := &RepoData{
		Path: "/test/repo",
		Files: []FileData{
			{Path: "main.go", Content: "package main", Size: 12},
		},
	}
	options := AnalysisOptions{
		RepoPath:     "/test/repo",
		Context:      "analysis",
		LanguageHint: "go",
		SessionID:    "test-session",
	}

	config := AnalysisConfig{
		RepositoryPath: "/test/repo",
		RepoData:       repoData,
		Options:        options,
		Logger:         logger,
	}

	if config.RepositoryPath != "/test/repo" {
		t.Errorf("Expected RepositoryPath to be '/test/repo', got '%s'", config.RepositoryPath)
	}
	if config.RepoData != repoData {
		t.Error("Expected RepoData to match the provided data")
	}
	if config.Options.RepoPath != "/test/repo" {
		t.Errorf("Expected Options.RepoPath to be '/test/repo', got '%s'", config.Options.RepoPath)
	}
	if config.Options.LanguageHint != "go" {
		t.Errorf("Expected Options.LanguageHint to be 'go', got '%s'", config.Options.LanguageHint)
	}
}

// Test EngineAnalysisOptions structure
func TestEngineAnalysisOptions_Structure(t *testing.T) {
	options := EngineAnalysisOptions{
		IncludeFrameworks: true,
	}

	if !options.IncludeFrameworks {
		t.Error("Expected IncludeFrameworks to be true")
	}
}

// Mock implementation of AnalysisEngine for testing
type MockAnalysisEngine struct {
	name         string
	capabilities []string
	applicable   bool
}

func (m *MockAnalysisEngine) GetName() string {
	return m.name
}

func (m *MockAnalysisEngine) Analyze(ctx context.Context, config AnalysisConfig) (*EngineAnalysisResult, error) {
	// Return a minimal result for testing
	return &EngineAnalysisResult{}, nil
}

func (m *MockAnalysisEngine) GetCapabilities() []string {
	return m.capabilities
}

func (m *MockAnalysisEngine) IsApplicable(ctx context.Context, repoData *RepoData) bool {
	return m.applicable
}

// Test AnalysisEngine interface implementation
func TestAnalysisEngine_Interface(t *testing.T) {
	engine := &MockAnalysisEngine{
		name:         "test-engine",
		capabilities: []string{"test-capability", "another-capability"},
		applicable:   true,
	}

	// Test GetName
	if engine.GetName() != "test-engine" {
		t.Errorf("Expected name to be 'test-engine', got '%s'", engine.GetName())
	}

	// Test GetCapabilities
	capabilities := engine.GetCapabilities()
	if len(capabilities) != 2 {
		t.Errorf("Expected 2 capabilities, got %d", len(capabilities))
	}
	if capabilities[0] != "test-capability" {
		t.Errorf("Expected first capability to be 'test-capability', got '%s'", capabilities[0])
	}

	// Test IsApplicable
	ctx := context.Background()
	repoData := &RepoData{Path: "/test"}
	if !engine.IsApplicable(ctx, repoData) {
		t.Error("Expected engine to be applicable")
	}

	// Test Analyze
	config := AnalysisConfig{
		RepositoryPath: "/test",
		RepoData:       repoData,
		Logger:         zerolog.Nop(),
	}
	result, err := engine.Analyze(ctx, config)
	if err != nil {
		t.Errorf("Expected no error from Analyze, got %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result from Analyze")
	}
}

// Test AnalysisEngine with non-applicable scenario
func TestAnalysisEngine_NonApplicable(t *testing.T) {
	engine := &MockAnalysisEngine{
		name:         "non-applicable-engine",
		capabilities: []string{"special-capability"},
		applicable:   false,
	}

	ctx := context.Background()
	repoData := &RepoData{Path: "/different/repo"}

	if engine.IsApplicable(ctx, repoData) {
		t.Error("Expected engine to not be applicable")
	}
}

// Test empty RepoData scenarios
func TestRepoData_Empty(t *testing.T) {
	emptyRepo := RepoData{}

	if emptyRepo.Path != "" {
		t.Errorf("Expected empty Path, got '%s'", emptyRepo.Path)
	}
	if len(emptyRepo.Files) != 0 {
		t.Errorf("Expected 0 files, got %d", len(emptyRepo.Files))
	}
	if len(emptyRepo.Languages) != 0 {
		t.Errorf("Expected 0 languages, got %d", len(emptyRepo.Languages))
	}
	if len(emptyRepo.Structure) != 0 {
		t.Errorf("Expected 0 structure items, got %d", len(emptyRepo.Structure))
	}
}
