package analyze

import (
	"testing"

	"github.com/Azure/container-kit/pkg/core/analysis"
)

// Test ContainerizationAssessment structure
func TestContainerizationAssessment_Structure(t *testing.T) {
	assessment := ContainerizationAssessment{
		ReadinessScore:      85,
		StrengthAreas:       []string{"Well-structured code", "Good test coverage"},
		ChallengeAreas:      []string{"Large dependencies", "Complex configuration"},
		RecommendedApproach: "multi-stage-build",
		TechnologyStack: TechnologyStackAssessment{
			Language:               "go",
			Framework:              "gin",
			BaseImageOptions:       []string{"golang:1.21-alpine", "distroless/base"},
			BuildStrategy:          "multi-stage",
			SecurityConsiderations: []string{"Run as non-root", "Scan for vulnerabilities"},
		},
		RiskAnalysis: []ContainerizationRisk{
			{
				Area:       "dependencies",
				Risk:       "Large dependency tree",
				Impact:     "medium",
				Mitigation: "Use dependency scanning",
			},
		},
		DeploymentOptions: []DeploymentRecommendation{
			{
				Strategy:   "kubernetes",
				Pros:       []string{"Scalability", "High availability"},
				Cons:       []string{"Complexity", "Learning curve"},
				Complexity: "moderate",
			},
		},
	}

	if assessment.ReadinessScore != 85 {
		t.Errorf("Expected ReadinessScore to be 85, got %d", assessment.ReadinessScore)
	}
	if len(assessment.StrengthAreas) != 2 {
		t.Errorf("Expected 2 strength areas, got %d", len(assessment.StrengthAreas))
	}
	if assessment.RecommendedApproach != "multi-stage-build" {
		t.Errorf("Expected RecommendedApproach to be 'multi-stage-build', got '%s'", assessment.RecommendedApproach)
	}
	if assessment.TechnologyStack.Language != "go" {
		t.Errorf("Expected Language to be 'go', got '%s'", assessment.TechnologyStack.Language)
	}
	if len(assessment.RiskAnalysis) != 1 {
		t.Errorf("Expected 1 risk analysis item, got %d", len(assessment.RiskAnalysis))
	}
	if len(assessment.DeploymentOptions) != 1 {
		t.Errorf("Expected 1 deployment option, got %d", len(assessment.DeploymentOptions))
	}
}

// Test TechnologyStackAssessment structure
func TestTechnologyStackAssessment_Structure(t *testing.T) {
	stack := TechnologyStackAssessment{
		Language:               "python",
		Framework:              "flask",
		BaseImageOptions:       []string{"python:3.11-slim", "python:3.11-alpine"},
		BuildStrategy:          "single-stage",
		SecurityConsiderations: []string{"Use virtual environment", "Pin dependencies"},
	}

	if stack.Language != "python" {
		t.Errorf("Expected Language to be 'python', got '%s'", stack.Language)
	}
	if stack.Framework != "flask" {
		t.Errorf("Expected Framework to be 'flask', got '%s'", stack.Framework)
	}
	if len(stack.BaseImageOptions) != 2 {
		t.Errorf("Expected 2 base image options, got %d", len(stack.BaseImageOptions))
	}
	if stack.BuildStrategy != "single-stage" {
		t.Errorf("Expected BuildStrategy to be 'single-stage', got '%s'", stack.BuildStrategy)
	}
	if len(stack.SecurityConsiderations) != 2 {
		t.Errorf("Expected 2 security considerations, got %d", len(stack.SecurityConsiderations))
	}
}

// Test ContainerizationRisk structure
func TestContainerizationRisk_Structure(t *testing.T) {
	risk := ContainerizationRisk{
		Area:       "configuration",
		Risk:       "Complex environment variables",
		Impact:     "high",
		Mitigation: "Use configuration management tools",
	}

	if risk.Area != "configuration" {
		t.Errorf("Expected Area to be 'configuration', got '%s'", risk.Area)
	}
	if risk.Risk != "Complex environment variables" {
		t.Errorf("Expected Risk to be 'Complex environment variables', got '%s'", risk.Risk)
	}
	if risk.Impact != "high" {
		t.Errorf("Expected Impact to be 'high', got '%s'", risk.Impact)
	}
	if risk.Mitigation != "Use configuration management tools" {
		t.Errorf("Expected specific mitigation, got '%s'", risk.Mitigation)
	}
}

// Test DeploymentRecommendation structure
func TestDeploymentRecommendation_Structure(t *testing.T) {
	recommendation := DeploymentRecommendation{
		Strategy:   "docker-compose",
		Pros:       []string{"Simple setup", "Local development friendly"},
		Cons:       []string{"Limited scalability", "Single machine"},
		Complexity: "simple",
	}

	if recommendation.Strategy != "docker-compose" {
		t.Errorf("Expected Strategy to be 'docker-compose', got '%s'", recommendation.Strategy)
	}
	if len(recommendation.Pros) != 2 {
		t.Errorf("Expected 2 pros, got %d", len(recommendation.Pros))
	}
	if len(recommendation.Cons) != 2 {
		t.Errorf("Expected 2 cons, got %d", len(recommendation.Cons))
	}
	if recommendation.Complexity != "simple" {
		t.Errorf("Expected Complexity to be 'simple', got '%s'", recommendation.Complexity)
	}
}

// Test AnalysisContext with containerization suggestions
func TestAnalysisContext_ContainerizationSuggestions(t *testing.T) {
	context := AnalysisContext{
		FilesAnalyzed:   100,
		PackageManagers: []string{"npm", "pip"},
		DockerFiles:     []string{"Dockerfile"},
		HasGitIgnore:    true,
		HasReadme:       true,
		RepositorySize:  500000,
		ContainerizationSuggestions: []string{
			"Add .dockerignore file",
			"Use multi-stage build",
			"Consider smaller base image",
			"Add health checks",
		},
		NextStepSuggestions: []string{
			"Set up CI/CD",
			"Add monitoring",
			"Configure logging",
		},
	}

	if len(context.ContainerizationSuggestions) != 4 {
		t.Errorf("Expected 4 containerization suggestions, got %d", len(context.ContainerizationSuggestions))
	}
	if len(context.NextStepSuggestions) != 3 {
		t.Errorf("Expected 3 next step suggestions, got %d", len(context.NextStepSuggestions))
	}

	// Check specific suggestions
	found := false
	for _, suggestion := range context.ContainerizationSuggestions {
		if suggestion == "Add .dockerignore file" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find 'Add .dockerignore file' in containerization suggestions")
	}
}

// Test analysis.AnalysisResult embedding
func TestAnalysisResult_Embedding(t *testing.T) {
	// Create a core analysis result
	coreResult := &analysis.AnalysisResult{
		// We don't need to populate this for the test, just verify embedding works
	}

	result := AnalysisResult{
		AnalysisResult: coreResult,
		Duration:       30 * 1000000000, // 30 seconds in nanoseconds
		Context: &AnalysisContext{
			FilesAnalyzed: 50,
			HasReadme:     true,
		},
	}

	// Test that we can access the embedded field
	if result.AnalysisResult != coreResult {
		t.Error("Expected embedded AnalysisResult to be accessible")
	}
	if result.Duration != 30*1000000000 {
		t.Errorf("Expected Duration to be 30s in nanoseconds, got %d", result.Duration)
	}
	if result.Context.FilesAnalyzed != 50 {
		t.Errorf("Expected FilesAnalyzed to be 50, got %d", result.Context.FilesAnalyzed)
	}
}
