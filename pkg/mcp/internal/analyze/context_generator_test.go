package analyze

import (
	"testing"

	"github.com/Azure/container-kit/pkg/core/analysis"
	"github.com/rs/zerolog"
)

func TestNewContextGenerator(t *testing.T) {
	logger := zerolog.Nop()
	generator := NewContextGenerator(logger)

	if generator == nil {
		t.Fatal("NewContextGenerator returned nil")
	}
}

func TestGenerateContainerizationAssessment_NilInputs(t *testing.T) {
	logger := zerolog.Nop()
	generator := NewContextGenerator(logger)

	// Test with nil analysis result
	_, err := generator.GenerateContainerizationAssessment(nil, &AnalysisContext{})
	if err == nil {
		t.Error("Expected error with nil analysis result")
	}

	// Test with nil context
	_, err = generator.GenerateContainerizationAssessment(&analysis.AnalysisResult{}, nil)
	if err == nil {
		t.Error("Expected error with nil context")
	}

	// Test with both nil
	_, err = generator.GenerateContainerizationAssessment(nil, nil)
	if err == nil {
		t.Error("Expected error with both nil")
	}
}

func TestGenerateContainerizationAssessment_ValidInputs(t *testing.T) {
	logger := zerolog.Nop()
	generator := NewContextGenerator(logger)

	analysisResult := &analysis.AnalysisResult{
		Success:  true,
		Language: "Go",
		Dependencies: []analysis.Dependency{
			{Name: "github.com/gin-gonic/gin", Type: "runtime"},
		},
	}

	analysisContext := &AnalysisContext{
		FilesAnalyzed:    10,
		ConfigFilesFound: []string{"config.yaml"},
		EntryPointsFound: []string{"main.go"},
	}

	assessment, err := generator.GenerateContainerizationAssessment(analysisResult, analysisContext)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if assessment == nil {
		t.Fatal("Assessment should not be nil")
	}

	// Check that assessment has been populated
	if assessment.ReadinessScore < 0 || assessment.ReadinessScore > 100 {
		t.Errorf("ReadinessScore out of range: %d", assessment.ReadinessScore)
	}
}
