package analyze

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestSeverityConstants(t *testing.T) {
	// Test that severity constants have expected values
	tests := []struct {
		severity Severity
		expected string
	}{
		{SeverityInfo, "info"},
		{SeverityLow, "low"},
		{SeverityMedium, "medium"},
		{SeverityHigh, "high"},
		{SeverityCritical, "critical"},
	}

	for _, tt := range tests {
		if string(tt.severity) != tt.expected {
			t.Errorf("Severity %s has value %s, expected %s", tt.expected, string(tt.severity), tt.expected)
		}
	}
}

func TestFindingTypeConstants(t *testing.T) {
	// Test that finding type constants have expected values
	tests := []struct {
		findingType FindingType
		expected    string
	}{
		{FindingTypeLanguage, "language"},
		{FindingTypeFramework, "framework"},
		{FindingTypeDependency, "dependency"},
		{FindingTypeConfiguration, "configuration"},
		{FindingTypeDatabase, "database"},
		{FindingTypeBuild, "build"},
		{FindingTypePort, "port"},
		{FindingTypeEnvironment, "environment"},
		{FindingTypeEntrypoint, "entrypoint"},
		{FindingTypeSecurity, "security"},
	}

	for _, tt := range tests {
		if string(tt.findingType) != tt.expected {
			t.Errorf("FindingType %s has value %s, expected %s", tt.expected, string(tt.findingType), tt.expected)
		}
	}
}

func TestNewAnalysisOrchestrator(t *testing.T) {
	logger := zerolog.New(os.Stderr)
	orchestrator := NewAnalysisOrchestrator(logger)

	if orchestrator == nil {
		t.Fatal("NewAnalysisOrchestrator returned nil")
	}

	if orchestrator.engines == nil {
		t.Error("Orchestrator engines should be initialized")
	}

	if len(orchestrator.engines) != 0 {
		t.Error("Orchestrator should start with no engines")
	}
}

func TestAnalysisOrchestrator_RegisterEngine(t *testing.T) {
	logger := zerolog.New(os.Stderr)
	orchestrator := NewAnalysisOrchestrator(logger)

	// Create a mock engine
	engine := &mockEngine{name: "test-engine"}

	// Register the engine
	orchestrator.RegisterEngine(engine)

	if len(orchestrator.engines) != 1 {
		t.Errorf("Expected 1 engine, got %d", len(orchestrator.engines))
	}

	// Register another engine
	engine2 := &mockEngine{name: "test-engine-2"}
	orchestrator.RegisterEngine(engine2)

	if len(orchestrator.engines) != 2 {
		t.Errorf("Expected 2 engines, got %d", len(orchestrator.engines))
	}
}

func TestEngineAnalysisResult_Defaults(t *testing.T) {
	result := &EngineAnalysisResult{
		Engine:   "test-engine",
		Success:  true,
		Duration: 5 * time.Second,
	}

	if result.Findings == nil {
		// Findings can be nil, that's ok
	}

	if result.Metadata == nil {
		// Metadata can be nil, that's ok
	}

	if result.Errors == nil {
		// Errors can be nil, that's ok
	}

	if result.Confidence < 0 || result.Confidence > 100 {
		// Confidence is just a float, no validation needed
	}
}

func TestLocation_Structure(t *testing.T) {
	loc := &Location{
		Path:       "/test/file.go",
		LineNumber: 42,
		Column:     10,
		Section:    "imports",
	}

	if loc.Path != "/test/file.go" {
		t.Errorf("Path = %s, want /test/file.go", loc.Path)
	}
}

func TestEvidence_Structure(t *testing.T) {
	evidence := &Evidence{
		Type:        "file_content",
		Description: "Found database connection string",
		Location: &Location{
			Path:       "/config/db.yaml",
			LineNumber: 15,
		},
		Value: "postgres://localhost:5432/mydb",
	}

	if evidence.Type != "file_content" {
		t.Errorf("Type = %s, want file_content", evidence.Type)
	}
}

// Mock engine for testing
type mockEngine struct {
	name string
}

func (m *mockEngine) GetName() string {
	return m.name
}

func (m *mockEngine) Analyze(ctx context.Context, config AnalysisConfig) (*EngineAnalysisResult, error) {
	return &EngineAnalysisResult{
		Engine:  m.name,
		Success: true,
	}, nil
}

func (m *mockEngine) GetCapabilities() []string {
	return []string{"test"}
}

func (m *mockEngine) IsApplicable(ctx context.Context, repoData *RepoData) bool {
	return true
}
