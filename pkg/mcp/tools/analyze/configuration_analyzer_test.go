package analyze

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
)

// Test ConfigurationAnalyzer constructor and basic methods
func TestConfigurationAnalyzer(t *testing.T) {
	logger := zerolog.Nop()

	// Test constructor
	analyzer := NewConfigurationAnalyzer(logger)
	if analyzer == nil {
		t.Error("NewConfigurationAnalyzer should not return nil")
	}

	// Test GetName
	name := analyzer.GetName()
	if name != "configuration_analyzer" {
		t.Errorf("Expected name to be 'configuration_analyzer', got '%s'", name)
	}

	// Test GetCapabilities
	capabilities := analyzer.GetCapabilities()
	if len(capabilities) == 0 {
		t.Error("Expected at least one capability")
	}

	expectedCapabilities := []string{
		"configuration_files",
		"environment_variables",
		"port_detection",
		"secrets_detection",
		"logging_configuration",
		"monitoring_setup",
	}

	if len(capabilities) != len(expectedCapabilities) {
		t.Errorf("Expected %d capabilities, got %d", len(expectedCapabilities), len(capabilities))
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
			t.Errorf("Expected capability '%s' not found", expected)
		}
	}
}

// Test IsApplicable method
func TestConfigurationAnalyzerIsApplicable(t *testing.T) {
	logger := zerolog.Nop()
	analyzer := NewConfigurationAnalyzer(logger)
	ctx := context.Background()

	// Test with nil RepoData
	applicable := analyzer.IsApplicable(ctx, nil)
	if !applicable {
		t.Error("Configuration analyzer should always be applicable, even with nil RepoData")
	}

	// Test with empty RepoData
	emptyRepoData := &RepoData{
		Path:      "",
		Files:     []FileData{},
		Languages: map[string]float64{},
		Structure: map[string]interface{}{},
	}
	applicable = analyzer.IsApplicable(ctx, emptyRepoData)
	if !applicable {
		t.Error("Configuration analyzer should always be applicable with empty RepoData")
	}

	// Test with populated RepoData
	repoData := &RepoData{
		Path: "/tmp/test-repo",
		Files: []FileData{
			{Path: "config.yaml", Content: "key: value", Size: 100},
			{Path: "main.go", Content: "package main", Size: 200},
		},
		Languages: map[string]float64{
			"Go":   80.5,
			"YAML": 19.5,
		},
		Structure: map[string]interface{}{
			"type": "application",
		},
	}
	applicable = analyzer.IsApplicable(ctx, repoData)
	if !applicable {
		t.Error("Configuration analyzer should always be applicable with populated RepoData")
	}
}

// Test ConfigurationAnalyzer interface compliance
func TestConfigurationAnalyzerInterface(t *testing.T) {
	logger := zerolog.Nop()
	var engine AnalysisEngine = NewConfigurationAnalyzer(logger)

	// Test interface methods
	name := engine.GetName()
	if name == "" {
		t.Error("GetName should return non-empty string")
	}

	capabilities := engine.GetCapabilities()
	if len(capabilities) == 0 {
		t.Error("GetCapabilities should return at least one capability")
	}

	ctx := context.Background()
	repoData := &RepoData{Path: "/test"}
	applicable := engine.IsApplicable(ctx, repoData)
	if !applicable {
		t.Error("IsApplicable should return true for configuration analyzer")
	}
}

// Test RepoData and FileData types
func TestRepoDataTypes(t *testing.T) {
	// Test FileData
	fileData := FileData{
		Path:    "config.yaml",
		Content: "test: value",
		Size:    100,
	}

	if fileData.Path != "config.yaml" {
		t.Errorf("Expected Path to be 'config.yaml', got '%s'", fileData.Path)
	}
	if fileData.Content != "test: value" {
		t.Errorf("Expected Content to be 'test: value', got '%s'", fileData.Content)
	}
	if fileData.Size != 100 {
		t.Errorf("Expected Size to be 100, got %d", fileData.Size)
	}

	// Test RepoData
	repoData := RepoData{
		Path: "/tmp/test-repo",
		Files: []FileData{
			fileData,
			{Path: "main.go", Content: "package main", Size: 50},
		},
		Languages: map[string]float64{
			"YAML": 66.7,
			"Go":   33.3,
		},
		Structure: map[string]interface{}{
			"type":       "application",
			"has_config": true,
			"file_count": 2,
		},
	}

	if repoData.Path != "/tmp/test-repo" {
		t.Errorf("Expected Path to be '/tmp/test-repo', got '%s'", repoData.Path)
	}
	if len(repoData.Files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(repoData.Files))
	}
	if len(repoData.Languages) != 2 {
		t.Errorf("Expected 2 languages, got %d", len(repoData.Languages))
	}
	if repoData.Languages["YAML"] != 66.7 {
		t.Errorf("Expected YAML percentage to be 66.7, got %f", repoData.Languages["YAML"])
	}
	if len(repoData.Structure) != 3 {
		t.Errorf("Expected 3 structure entries, got %d", len(repoData.Structure))
	}
	if repoData.Structure["type"] != "application" {
		t.Errorf("Expected structure type to be 'application', got %v", repoData.Structure["type"])
	}
}
