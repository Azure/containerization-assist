package utils

import (
	"testing"
	"time"

	coredocker "github.com/Azure/container-copilot/pkg/core/docker"
	"github.com/Azure/container-copilot/pkg/core/kubernetes"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
)

// MockBuildResult simulates a build tool result for testing
type MockBuildResult struct {
	Success       bool
	SessionID     string
	ImageName     string
	ImageTag      string
	FullImageRef  string
	BuildDuration time.Duration
	BuildResult   *coredocker.BuildResult
	SecurityScan  *coredocker.ScanResult
	Error         *types.RichError
}

// MockDeployResult simulates a deploy tool result for testing
type MockDeployResult struct {
	Success            bool
	SessionID          string
	AppName            string
	Namespace          string
	ImageRef           string
	Replicas           int
	DeploymentDuration time.Duration
	HealthResult       *kubernetes.HealthCheckResult
	DeploymentResult   *kubernetes.DeploymentResult
	Error              *types.RichError
}

// Implement unified AI context interfaces for MockBuildResult

func (r *MockBuildResult) CalculateScore() int {
	if !r.Success {
		return 20
	}
	score := 70
	if r.BuildDuration < 2*time.Minute {
		score += 15
	}
	if r.SecurityScan != nil && r.SecurityScan.Summary.Critical == 0 {
		score += 15
	}
	return score
}

func (r *MockBuildResult) DetermineRiskLevel() string {
	if !r.Success {
		return types.SeverityHigh
	}
	if r.SecurityScan != nil && r.SecurityScan.Summary.Critical > 0 {
		return types.SeverityCritical
	}
	return types.SeverityLow
}

func (r *MockBuildResult) GetStrengths() []string {
	strengths := make([]string, 0)
	if r.Success {
		strengths = append(strengths, "Build completed successfully")
	}
	if r.BuildDuration < 2*time.Minute {
		strengths = append(strengths, "Fast build time")
	}
	return strengths
}

func (r *MockBuildResult) GetChallenges() []string {
	challenges := make([]string, 0)
	if !r.Success {
		challenges = append(challenges, "Build failed")
	}
	if r.SecurityScan != nil && r.SecurityScan.Summary.Critical > 0 {
		challenges = append(challenges, "Critical vulnerabilities found")
	}
	return challenges
}

func (r *MockBuildResult) GetAssessment() *UnifiedAssessment {
	return &UnifiedAssessment{
		ReadinessScore:      r.CalculateScore(),
		RiskLevel:           r.DetermineRiskLevel(),
		ConfidenceLevel:     80,
		OverallHealth:       "good",
		StrengthAreas:       make([]AssessmentArea, 0),
		ChallengeAreas:      make([]AssessmentArea, 0),
		RiskFactors:         make([]RiskFactor, 0),
		DecisionFactors:     make([]DecisionFactor, 0),
		AssessmentBasis:     make([]EvidenceItem, 0),
		QualityIndicators:   make(map[string]interface{}),
		RecommendedApproach: "Continue with deployment",
		NextSteps:           []string{"Deploy to staging"},
		ConsiderationsNote:  "Build successful",
	}
}

func (r *MockBuildResult) GenerateRecommendations() []Recommendation {
	recommendations := make([]Recommendation, 0)
	if !r.Success {
		recommendations = append(recommendations, Recommendation{
			RecommendationID: "fix-build",
			Title:            "Fix Build Failure",
			Category:         "operational",
			Priority:         types.SeverityCritical,
			Type:             "fix",
		})
	}
	return recommendations
}

func (r *MockBuildResult) CreateRemediationPlan() *RemediationPlan {
	return &RemediationPlan{
		PlanID:      "test-plan",
		Title:       "Test Remediation Plan",
		Description: "Test plan for mock build result",
		Priority:    types.SeverityMedium,
		Category:    "operational",
		Steps:       make([]RemediationStep, 0),
	}
}

func (r *MockBuildResult) GetAlternativeStrategies() []AlternativeStrategy {
	return make([]AlternativeStrategy, 0)
}

func (r *MockBuildResult) GetAIContext() *ToolContext {
	enricher := NewContextEnricher()
	context, _ := enricher.EnrichToolResponse(r, "mock_build")
	return context
}

func (r *MockBuildResult) EnrichWithInsights(insights []ContextualInsight) {
	// Mock implementation
}

func (r *MockBuildResult) GetMetadataForAI() map[string]interface{} {
	return map[string]interface{}{
		"tool_name":  "mock_build",
		"success":    r.Success,
		"session_id": r.SessionID,
		"image_name": r.ImageName,
	}
}

// Implement unified AI context interfaces for MockDeployResult

func (r *MockDeployResult) CalculateScore() int {
	if !r.Success {
		return 25
	}
	score := 70
	if r.HealthResult != nil && r.HealthResult.Summary.ReadyPods == r.HealthResult.Summary.TotalPods {
		score += 20
	}
	if r.DeploymentDuration < 30*time.Second {
		score += 5
	}
	return score
}

func (r *MockDeployResult) DetermineRiskLevel() string {
	if !r.Success {
		return types.SeverityHigh
	}
	if r.HealthResult != nil && r.HealthResult.Summary.ReadyPods < r.HealthResult.Summary.TotalPods {
		return types.SeverityMedium
	}
	return types.SeverityLow
}

func (r *MockDeployResult) GetStrengths() []string {
	strengths := make([]string, 0)
	if r.Success {
		strengths = append(strengths, "Deployment completed successfully")
	}
	if r.HealthResult != nil && r.HealthResult.Summary.ReadyPods == r.HealthResult.Summary.TotalPods {
		strengths = append(strengths, "All pods are healthy")
	}
	return strengths
}

func (r *MockDeployResult) GetChallenges() []string {
	challenges := make([]string, 0)
	if !r.Success {
		challenges = append(challenges, "Deployment failed")
	}
	if r.HealthResult != nil && r.HealthResult.Summary.ReadyPods < r.HealthResult.Summary.TotalPods {
		challenges = append(challenges, "Some pods are not ready")
	}
	return challenges
}

func (r *MockDeployResult) GetAssessment() *UnifiedAssessment {
	return &UnifiedAssessment{
		ReadinessScore:      r.CalculateScore(),
		RiskLevel:           r.DetermineRiskLevel(),
		ConfidenceLevel:     85,
		OverallHealth:       "good",
		StrengthAreas:       make([]AssessmentArea, 0),
		ChallengeAreas:      make([]AssessmentArea, 0),
		RiskFactors:         make([]RiskFactor, 0),
		DecisionFactors:     make([]DecisionFactor, 0),
		AssessmentBasis:     make([]EvidenceItem, 0),
		QualityIndicators:   make(map[string]interface{}),
		RecommendedApproach: "Monitor deployment health",
		NextSteps:           []string{"Set up monitoring"},
		ConsiderationsNote:  "Deployment successful",
	}
}

func (r *MockDeployResult) GenerateRecommendations() []Recommendation {
	recommendations := make([]Recommendation, 0)
	if !r.Success {
		recommendations = append(recommendations, Recommendation{
			RecommendationID: "fix-deploy",
			Title:            "Fix Deployment Failure",
			Category:         "operational",
			Priority:         types.SeverityCritical,
			Type:             "fix",
		})
	}
	return recommendations
}

func (r *MockDeployResult) CreateRemediationPlan() *RemediationPlan {
	priority := types.SeverityMedium
	if !r.Success {
		priority = types.SeverityCritical
	}

	return &RemediationPlan{
		PlanID:      "test-deploy-plan",
		Title:       "Test Deploy Remediation Plan",
		Description: "Test plan for mock deploy result",
		Priority:    priority,
		Category:    "operational",
		Steps:       make([]RemediationStep, 0),
	}
}

func (r *MockDeployResult) GetAlternativeStrategies() []AlternativeStrategy {
	return make([]AlternativeStrategy, 0)
}

func (r *MockDeployResult) GetAIContext() *ToolContext {
	enricher := NewContextEnricher()
	context, _ := enricher.EnrichToolResponse(r, "mock_deploy")
	return context
}

func (r *MockDeployResult) EnrichWithInsights(insights []ContextualInsight) {
	// Mock implementation
}

func (r *MockDeployResult) GetMetadataForAI() map[string]interface{} {
	return map[string]interface{}{
		"tool_name":  "mock_deploy",
		"success":    r.Success,
		"session_id": r.SessionID,
		"app_name":   r.AppName,
		"namespace":  r.Namespace,
	}
}

// Test unified AI context integration

func TestUnifiedAIContextIntegration(t *testing.T) {
	t.Run("Successful Build Tool Context", func(t *testing.T) {
		mockBuild := &MockBuildResult{
			Success:       true,
			SessionID:     "test-session-1",
			ImageName:     "test-app",
			ImageTag:      "latest",
			FullImageRef:  "test-app:latest",
			BuildDuration: 1 * time.Minute,
			SecurityScan: &coredocker.ScanResult{
				Success: true,
				Summary: coredocker.VulnerabilitySummary{
					Critical: 0,
					High:     0,
					Total:    0,
				},
			},
		}

		// Test Assessable interface
		score := mockBuild.CalculateScore()
		if score < 80 {
			t.Errorf("Expected high score for successful build, got %d", score)
		}

		riskLevel := mockBuild.DetermineRiskLevel()
		if riskLevel != types.SeverityLow {
			t.Errorf("Expected low risk for successful build, got %s", riskLevel)
		}

		strengths := mockBuild.GetStrengths()
		if len(strengths) == 0 {
			t.Error("Expected strengths for successful build")
		}

		challenges := mockBuild.GetChallenges()
		if len(challenges) > 0 {
			t.Error("Expected no challenges for successful build")
		}

		// Test Recommendable interface
		recommendations := mockBuild.GenerateRecommendations()
		if len(recommendations) > 1 {
			t.Error("Expected minimal recommendations for successful build")
		}

		plan := mockBuild.CreateRemediationPlan()
		if plan == nil {
			t.Error("Expected remediation plan to be created")
		}

		// Test ContextEnriched interface
		context := mockBuild.GetAIContext()
		if context == nil {
			t.Error("Expected AI context to be created")
		}
		if context.ToolName != "mock_build" {
			t.Errorf("Expected tool name 'mock_build', got %s", context.ToolName)
		}

		metadata := mockBuild.GetMetadataForAI()
		if metadata["success"] != true {
			t.Error("Expected success in metadata")
		}
	})

	t.Run("Failed Build Tool Context", func(t *testing.T) {
		mockBuild := &MockBuildResult{
			Success:      false,
			SessionID:    "test-session-2",
			ImageName:    "failed-app",
			ImageTag:     "latest",
			FullImageRef: "failed-app:latest",
			Error: &types.RichError{
				Code:     "BUILD_FAILED",
				Type:     "build_error",
				Severity: types.SeverityHigh,
				Message:  "Docker build failed",
			},
		}

		// Test Assessable interface
		score := mockBuild.CalculateScore()
		if score > 30 {
			t.Errorf("Expected low score for failed build, got %d", score)
		}

		riskLevel := mockBuild.DetermineRiskLevel()
		if riskLevel != types.SeverityHigh {
			t.Errorf("Expected high risk for failed build, got %s", riskLevel)
		}

		challenges := mockBuild.GetChallenges()
		if len(challenges) == 0 {
			t.Error("Expected challenges for failed build")
		}

		// Test Recommendable interface
		recommendations := mockBuild.GenerateRecommendations()
		if len(recommendations) == 0 {
			t.Error("Expected recommendations for failed build")
		}

		// Verify recommendation is for fixing the failure
		fixRecommendation := false
		for _, rec := range recommendations {
			if rec.Type == "fix" && rec.Priority == types.SeverityCritical {
				fixRecommendation = true
			}
		}
		if !fixRecommendation {
			t.Error("Expected critical fix recommendation for failed build")
		}
	})

	t.Run("Successful Deploy Tool Context", func(t *testing.T) {
		mockDeploy := &MockDeployResult{
			Success:            true,
			SessionID:          "test-session-3",
			AppName:            "test-app",
			Namespace:          "default",
			ImageRef:           "test-app:latest",
			Replicas:           3,
			DeploymentDuration: 45 * time.Second,
			HealthResult: &kubernetes.HealthCheckResult{
				Success: true,
				Summary: kubernetes.HealthSummary{
					ReadyPods: 3,
					TotalPods: 3,
				},
			},
		}

		// Test Assessable interface
		score := mockDeploy.CalculateScore()
		if score < 85 {
			t.Errorf("Expected high score for successful deployment, got %d", score)
		}

		riskLevel := mockDeploy.DetermineRiskLevel()
		if riskLevel != types.SeverityLow {
			t.Errorf("Expected low risk for successful deployment, got %s", riskLevel)
		}

		strengths := mockDeploy.GetStrengths()
		if len(strengths) == 0 {
			t.Error("Expected strengths for successful deployment")
		}

		// Test ContextEnriched interface
		context := mockDeploy.GetAIContext()
		if context == nil {
			t.Error("Expected AI context to be created")
		}
		if context.Assessment.ReadinessScore < 80 {
			t.Errorf("Expected high readiness score, got %d", context.Assessment.ReadinessScore)
		}
	})

	t.Run("Failed Deploy Tool Context", func(t *testing.T) {
		mockDeploy := &MockDeployResult{
			Success:   false,
			SessionID: "test-session-4",
			AppName:   "failed-app",
			Namespace: "default",
			ImageRef:  "failed-app:latest",
			Replicas:  3,
			HealthResult: &kubernetes.HealthCheckResult{
				Success: false,
				Summary: kubernetes.HealthSummary{
					ReadyPods: 0,
					TotalPods: 3,
				},
			},
			Error: &types.RichError{
				Code:     "DEPLOY_FAILED",
				Type:     "deployment_error",
				Severity: types.SeverityHigh,
				Message:  "Deployment failed",
			},
		}

		// Test Assessable interface
		score := mockDeploy.CalculateScore()
		if score > 40 {
			t.Errorf("Expected low score for failed deployment, got %d", score)
		}

		riskLevel := mockDeploy.DetermineRiskLevel()
		if riskLevel != types.SeverityHigh {
			t.Errorf("Expected high risk for failed deployment, got %s", riskLevel)
		}

		// Test Recommendable interface
		recommendations := mockDeploy.GenerateRecommendations()
		if len(recommendations) == 0 {
			t.Error("Expected recommendations for failed deployment")
		}

		plan := mockDeploy.CreateRemediationPlan()
		if plan.Priority != types.SeverityCritical {
			t.Error("Expected critical priority for failed deployment remediation plan")
		}
	})
}

func TestContextEnricher(t *testing.T) {
	enricher := NewContextEnricher()

	t.Run("Enrich Simple Response", func(t *testing.T) {
		simpleResponse := struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}{
			Success: true,
			Message: "Operation completed",
		}

		context, err := enricher.EnrichToolResponse(simpleResponse, "test_tool")
		if err != nil {
			t.Fatalf("Failed to enrich response: %v", err)
		}

		if context.ToolName != "test_tool" {
			t.Errorf("Expected tool name 'test_tool', got %s", context.ToolName)
		}

		if context.Assessment == nil {
			t.Error("Expected assessment to be generated")
		}

		if len(context.Recommendations) == 0 {
			t.Error("Expected recommendations to be generated")
		}
	})

	t.Run("Enrich Complex Response", func(t *testing.T) {
		complexResponse := &MockBuildResult{
			Success:       true,
			SessionID:     "test-session",
			ImageName:     "complex-app",
			BuildDuration: 2 * time.Minute,
		}

		context, err := enricher.EnrichToolResponse(complexResponse, "complex_build")
		if err != nil {
			t.Fatalf("Failed to enrich complex response: %v", err)
		}

		if context.Assessment.ReadinessScore <= 0 {
			t.Error("Expected positive readiness score")
		}

		if len(context.Insights) == 0 {
			t.Error("Expected insights to be generated")
		}

		if len(context.QualityMetrics) == 0 {
			t.Error("Expected quality metrics to be extracted")
		}

		if len(context.ReasoningContext) == 0 {
			t.Error("Expected reasoning context to be built")
		}
	})
}

func TestAssessmentCalculations(t *testing.T) {
	t.Run("Score Calculation", func(t *testing.T) {
		calc := &DefaultScoreCalculator{}

		// Test successful response
		successResponse := struct{ Success bool }{Success: true}
		score := calc.CalculateScore(successResponse)
		if score != 85 {
			t.Errorf("Expected score 85 for success, got %d", score)
		}

		// Test failed response
		failResponse := struct{ Success bool }{Success: false}
		score = calc.CalculateScore(failResponse)
		if score != 30 {
			t.Errorf("Expected score 30 for failure, got %d", score)
		}

		// Test response without success field
		neutralResponse := struct{ Message string }{Message: "test"}
		score = calc.CalculateScore(neutralResponse)
		if score != 60 {
			t.Errorf("Expected score 60 for neutral, got %d", score)
		}
	})

	t.Run("Risk Level Determination", func(t *testing.T) {
		calc := &DefaultScoreCalculator{}

		highScore := calc.DetermineRiskLevel(90, map[string]interface{}{})
		if highScore != types.SeverityLow {
			t.Errorf("Expected low risk for high score, got %s", highScore)
		}

		lowScore := calc.DetermineRiskLevel(30, map[string]interface{}{})
		if lowScore != types.SeverityCritical {
			t.Errorf("Expected critical risk for low score, got %s", lowScore)
		}
	})

	t.Run("Confidence Calculation", func(t *testing.T) {
		calc := &DefaultScoreCalculator{}

		confidence := calc.CalculateConfidence([]string{"evidence1", "evidence2", "evidence3"})
		if confidence != 80 {
			t.Errorf("Expected confidence 80, got %d", confidence)
		}

		confidence = calc.CalculateConfidence([]string{})
		if confidence != 50 {
			t.Errorf("Expected confidence 50 for no evidence, got %d", confidence)
		}
	})
}

func TestTradeoffAnalysis(t *testing.T) {
	analyzer := &DefaultTradeoffAnalyzer{}

	t.Run("Analyze Tradeoffs", func(t *testing.T) {
		options := []string{"option1", "option2"}
		context := map[string]interface{}{"factor": "value"}

		analyses := analyzer.AnalyzeTradeoffs(options, context)
		if len(analyses) != 2 {
			t.Errorf("Expected 2 analyses, got %d", len(analyses))
		}

		for _, analysis := range analyses {
			if analysis.TotalBenefit != 70 {
				t.Errorf("Expected total benefit 70, got %d", analysis.TotalBenefit)
			}
			if analysis.Complexity != "moderate" {
				t.Errorf("Expected complexity 'moderate', got %s", analysis.Complexity)
			}
		}
	})

	t.Run("Recommend Best Option", func(t *testing.T) {
		analyses := []TradeoffAnalysis{
			{Option: "option1", TotalBenefit: 60},
			{Option: "option2", TotalBenefit: 80},
		}

		recommendation := analyzer.RecommendBestOption(analyses)
		if recommendation.RecommendedOption != "option2" {
			t.Errorf("Expected 'option2' as best option, got %s", recommendation.RecommendedOption)
		}
		if recommendation.Confidence != 80 {
			t.Errorf("Expected confidence 80, got %d", recommendation.Confidence)
		}
	})
}

func TestUnifiedStructures(t *testing.T) {
	t.Run("UnifiedAssessment", func(t *testing.T) {
		assessment := &UnifiedAssessment{
			ReadinessScore:      85,
			RiskLevel:           types.SeverityLow,
			ConfidenceLevel:     90,
			OverallHealth:       types.SeverityGood,
			RecommendedApproach: "Deploy to production",
			NextSteps:           []string{"Monitor", "Scale"},
		}

		if assessment.ReadinessScore != 85 {
			t.Errorf("Expected readiness score 85, got %d", assessment.ReadinessScore)
		}
		if len(assessment.NextSteps) != 2 {
			t.Errorf("Expected 2 next steps, got %d", len(assessment.NextSteps))
		}
	})

	t.Run("RemediationPlan", func(t *testing.T) {
		plan := &RemediationPlan{
			PlanID:      "test-plan",
			Title:       "Test Plan",
			Description: "Test remediation plan",
			Priority:    types.SeverityHigh,
			Category:    "operational",
			Steps: []RemediationStep{
				{
					StepID:      "step1",
					Order:       1,
					Title:       "First Step",
					Description: "Do something",
					Action:      "fix",
					Target:      "issue",
				},
			},
		}

		if plan.PlanID != "test-plan" {
			t.Errorf("Expected plan ID 'test-plan', got %s", plan.PlanID)
		}
		if len(plan.Steps) != 1 {
			t.Errorf("Expected 1 step, got %d", len(plan.Steps))
		}
		if plan.Steps[0].Order != 1 {
			t.Errorf("Expected step order 1, got %d", plan.Steps[0].Order)
		}
	})

	t.Run("ToolContext", func(t *testing.T) {
		context := &ToolContext{
			ToolName:    "test_tool",
			OperationID: "op-123",
			Timestamp:   time.Now(),
			Assessment: &UnifiedAssessment{
				ReadinessScore: 75,
			},
			Recommendations: []Recommendation{
				{
					RecommendationID: "rec-1",
					Title:            "Test Recommendation",
					Category:         "performance",
				},
			},
		}

		if context.ToolName != "test_tool" {
			t.Errorf("Expected tool name 'test_tool', got %s", context.ToolName)
		}
		if context.Assessment.ReadinessScore != 75 {
			t.Errorf("Expected readiness score 75, got %d", context.Assessment.ReadinessScore)
		}
		if len(context.Recommendations) != 1 {
			t.Errorf("Expected 1 recommendation, got %d", len(context.Recommendations))
		}
	})
}
