package ai_context

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
)

// ContextEnricher provides utilities to enrich tool responses with unified AI context
type ContextEnricher struct {
	calculator ScoreCalculator
	analyzer   TradeoffAnalyzer
}

// NewContextEnricher creates a new context enricher with default implementations
func NewContextEnricher() *ContextEnricher {
	return &ContextEnricher{
		calculator: &DefaultScoreCalculator{},
		analyzer:   &DefaultTradeoffAnalyzer{},
	}
}

// EnrichToolResponse enriches any tool response with unified AI context
func (e *ContextEnricher) EnrichToolResponse(response interface{}, toolName string) (*ToolContext, error) {
	context := &ToolContext{
		ToolName:         toolName,
		OperationID:      generateOperationID(),
		Timestamp:        time.Now(),
		Insights:         make([]ContextualInsight, 0),
		QualityMetrics:   make(map[string]interface{}),
		PerformanceData:  make(map[string]interface{}),
		ReasoningContext: make(map[string]interface{}),
		Metadata:         make(map[string]interface{}),
	}

	// Extract assessment if response implements AIContext
	if aiContext, ok := response.(AIContext); ok {
		context.Assessment = aiContext.GetAssessment()
		context.Recommendations = aiContext.GenerateRecommendations()
	} else {
		// Generate assessment and recommendations from response data
		context.Assessment = e.generateAssessment(response, toolName)
		context.Recommendations = e.generateRecommendations(response, toolName)
	}

	// Generate decision points and trade-offs
	context.DecisionPoints = e.extractDecisionPoints(response)
	context.TradeOffs = e.generateTradeoffs(response)

	// Generate insights
	context.Insights = e.generateInsights(response, toolName)

	// Extract performance data
	context.PerformanceData = e.extractPerformanceData(response)
	context.QualityMetrics = e.extractQualityMetrics(response)

	// Build reasoning context
	context.ReasoningContext = e.buildReasoningContext(response, toolName)

	return context, nil
}

// generateAssessment creates a unified assessment from response data
func (e *ContextEnricher) generateAssessment(response interface{}, toolName string) *UnifiedAssessment {
	assessment := &UnifiedAssessment{
		StrengthAreas:     make([]AssessmentArea, 0),
		ChallengeAreas:    make([]AssessmentArea, 0),
		RiskFactors:       make([]RiskFactor, 0),
		DecisionFactors:   make([]DecisionFactor, 0),
		AssessmentBasis:   make([]EvidenceItem, 0),
		QualityIndicators: make(map[string]interface{}),
	}

	// Extract success indicators
	successFound := e.extractBooleanField(response, "success", "Success")
	if successFound {
		if success, _ := e.getBooleanField(response, "success", "Success"); success {
			assessment.ReadinessScore = 85
			assessment.RiskLevel = types.SeverityLow
			assessment.OverallHealth = "good"
			assessment.ConfidenceLevel = 90
		} else {
			assessment.ReadinessScore = 30
			assessment.RiskLevel = types.SeverityHigh
			assessment.OverallHealth = "poor"
			assessment.ConfidenceLevel = 70
		}
	} else {
		// Default moderate assessment
		assessment.ReadinessScore = 60
		assessment.RiskLevel = types.SeverityMedium
		assessment.OverallHealth = "fair"
		assessment.ConfidenceLevel = 75
	}

	// Extract error information to build challenge areas
	if errorField := e.getErrorField(response); errorField != nil {
		assessment.ChallengeAreas = append(assessment.ChallengeAreas, AssessmentArea{
			Area:        "error_handling",
			Category:    "operational",
			Description: fmt.Sprintf("Operation encountered error: %s", errorField.Error()),
			Impact:      types.SeverityHigh,
			Evidence:    []string{errorField.Error()},
			Score:       20,
		})
	}

	// Build evidence from response fields
	assessment.AssessmentBasis = e.buildEvidence(response, toolName)

	return assessment
}

// generateRecommendations creates recommendations from response data
func (e *ContextEnricher) generateRecommendations(response interface{}, toolName string) []Recommendation {
	recommendations := make([]Recommendation, 0)

	// Check for errors and generate fix recommendations
	if errorField := e.getErrorField(response); errorField != nil {
		rec := Recommendation{
			RecommendationID: fmt.Sprintf("%s-error-fix-%d", toolName, time.Now().Unix()),
			Title:            "Address Operation Error",
			Description:      fmt.Sprintf("The %s operation encountered an error that should be addressed", toolName),
			Category:         "operational",
			Priority:         types.SeverityHigh,
			Type:             "fix",
			Tags:             []string{"error", "operational", "immediate"},
			ActionType:       "immediate",
			Benefits:         []string{"Restore operation functionality", "Prevent cascading failures"},
			Risks:            []string{"Continued operation failures"},
			Urgency:          "immediate",
			Effort:           "medium",
			Impact:           types.SeverityHigh,
			Confidence:       85,
		}

		// Add basic remediation plan
		rec.Implementation = RemediationPlan{
			PlanID:      fmt.Sprintf("%s-fix-plan-%d", toolName, time.Now().Unix()),
			Title:       "Fix Operation Error",
			Description: "Address the error encountered during operation",
			Priority:    types.SeverityHigh,
			Category:    "operational",
			Steps: []RemediationStep{
				{
					StepID:         "analyze-error",
					Order:          1,
					Title:          "Analyze Error",
					Description:    "Examine error details and context",
					Action:         "analyze",
					Target:         "error_context",
					ExpectedResult: "Understanding of root cause",
				},
				{
					StepID:         "apply-fix",
					Order:          2,
					Title:          "Apply Fix",
					Description:    "Implement solution based on error analysis",
					Action:         "fix",
					Target:         "root_cause",
					ExpectedResult: "Operation completes successfully",
				},
			},
		}

		recommendations = append(recommendations, rec)
	}

	// Generate optimization recommendations for successful operations
	if success, found := e.getBooleanField(response, "success", "Success"); found && success {
		rec := Recommendation{
			RecommendationID: fmt.Sprintf("%s-optimize-%d", toolName, time.Now().Unix()),
			Title:            "Optimize Operation Performance",
			Description:      fmt.Sprintf("Consider optimizations for %s operation", toolName),
			Category:         "performance",
			Priority:         types.SeverityMedium,
			Type:             "optimization",
			Tags:             []string{"performance", "optimization", "enhancement"},
			ActionType:       "planned",
			Benefits:         []string{"Improved performance", "Better resource utilization"},
			Urgency:          "eventually",
			Effort:           "low",
			Impact:           types.SeverityMedium,
			Confidence:       70,
		}
		recommendations = append(recommendations, rec)
	}

	return recommendations
}

// extractDecisionPoints identifies decision points from response data
func (e *ContextEnricher) extractDecisionPoints(response interface{}) []DecisionPoint {
	decisions := make([]DecisionPoint, 0)

	// Look for configuration choices in response
	v := reflect.ValueOf(response)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() == reflect.Struct {
		for i := 0; i < v.NumField(); i++ {
			field := v.Type().Field(i)
			value := v.Field(i)

			// Look for fields that represent choices
			if strings.Contains(strings.ToLower(field.Name), "config") ||
				strings.Contains(strings.ToLower(field.Name), "option") ||
				strings.Contains(strings.ToLower(field.Name), "strategy") {

				decision := DecisionPoint{
					DecisionID:  fmt.Sprintf("config-%s", strings.ToLower(field.Name)),
					Title:       fmt.Sprintf("Configuration: %s", field.Name),
					Description: fmt.Sprintf("Configuration choice for %s", field.Name),
					Chosen:      fmt.Sprintf("%v", value.Interface()),
					Confidence:  80,
					Impact:      types.SeverityMedium,
					Reversible:  true,
					Metadata:    map[string]interface{}{"field": field.Name},
				}
				decisions = append(decisions, decision)
			}
		}
	}

	return decisions
}

// generateTradeoffs creates trade-off analysis from response data
func (e *ContextEnricher) generateTradeoffs(response interface{}) []TradeoffAnalysis {
	return e.analyzer.AnalyzeTradeoffs([]string{"current_approach"}, e.extractTradeoffContext(response))
}

// generateInsights creates contextual insights from response data
func (e *ContextEnricher) generateInsights(response interface{}, toolName string) []ContextualInsight {
	insights := make([]ContextualInsight, 0)

	// Performance insight
	if duration := e.extractDurationField(response); duration > 0 {
		insight := ContextualInsight{
			InsightID:   fmt.Sprintf("%s-performance-%d", toolName, time.Now().Unix()),
			Type:        "performance",
			Title:       "Operation Duration Analysis",
			Description: fmt.Sprintf("Operation completed in %v", duration),
			Observation: fmt.Sprintf("Total execution time: %v", duration),
			Relevance:   types.SeverityMedium,
			Confidence:  95,
			Source:      "timing_analysis",
			Actionable:  true,
		}

		if duration > 5*time.Minute {
			insight.Implications = []string{"Long execution time may indicate optimization opportunities"}
		} else {
			insight.Implications = []string{"Reasonable execution time for this operation"}
		}

		insights = append(insights, insight)
	}

	// Success pattern insight
	if success, found := e.getBooleanField(response, "success", "Success"); found {
		insight := ContextualInsight{
			InsightID:   fmt.Sprintf("%s-success-pattern-%d", toolName, time.Now().Unix()),
			Type:        "pattern",
			Title:       "Operation Success Pattern",
			Description: fmt.Sprintf("Operation success status: %v", success),
			Observation: fmt.Sprintf("Operation completed with success=%v", success),
			Relevance:   types.SeverityHigh,
			Confidence:  100,
			Source:      "result_analysis",
			Actionable:  !success,
		}

		if success {
			insight.Implications = []string{"Operation completed successfully, consider optimizations"}
		} else {
			insight.Implications = []string{"Operation failed, requires immediate attention"}
		}

		insights = append(insights, insight)
	}

	return insights
}

// Helper methods for extracting data from responses

func (e *ContextEnricher) extractBooleanField(response interface{}, fieldNames ...string) bool {
	_, found := e.getBooleanField(response, fieldNames...)
	return found
}

func (e *ContextEnricher) getBooleanField(response interface{}, fieldNames ...string) (bool, bool) {
	v := reflect.ValueOf(response)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return false, false
	}

	for _, fieldName := range fieldNames {
		if field := v.FieldByName(fieldName); field.IsValid() && field.Kind() == reflect.Bool {
			return field.Bool(), true
		}
	}
	return false, false
}

func (e *ContextEnricher) getErrorField(response interface{}) error {
	v := reflect.ValueOf(response)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil
	}

	// Look for Error or Err fields
	if field := v.FieldByName("Error"); field.IsValid() && !field.IsNil() {
		if err, ok := field.Interface().(error); ok {
			return err
		}
	}

	if field := v.FieldByName("Err"); field.IsValid() && !field.IsNil() {
		if err, ok := field.Interface().(error); ok {
			return err
		}
	}

	return nil
}

func (e *ContextEnricher) extractDurationField(response interface{}) time.Duration {
	v := reflect.ValueOf(response)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return 0
	}

	// Look for duration fields
	durationFields := []string{"Duration", "TotalDuration", "ExecutionTime", "BuildDuration"}
	for _, fieldName := range durationFields {
		if field := v.FieldByName(fieldName); field.IsValid() {
			if duration, ok := field.Interface().(time.Duration); ok && duration > 0 {
				return duration
			}
		}
	}

	return 0
}

func (e *ContextEnricher) extractPerformanceData(response interface{}) map[string]interface{} {
	data := make(map[string]interface{})

	// Extract timing information
	if duration := e.extractDurationField(response); duration > 0 {
		data["total_duration"] = duration
		data["duration_seconds"] = duration.Seconds()
	}

	// Extract resource usage if available
	v := reflect.ValueOf(response)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() == reflect.Struct {
		// Look for size/count fields
		sizeFields := []string{"Size", "Count", "Lines", "Files"}
		for _, fieldName := range sizeFields {
			if field := v.FieldByName(fieldName); field.IsValid() && field.CanInterface() {
				data[strings.ToLower(fieldName)] = field.Interface()
			}
		}
	}

	return data
}

func (e *ContextEnricher) extractQualityMetrics(response interface{}) map[string]interface{} {
	metrics := make(map[string]interface{})

	// Extract success rate
	if success, found := e.getBooleanField(response, "success", "Success"); found {
		if success {
			metrics["success_rate"] = 1.0
		} else {
			metrics["success_rate"] = 0.0
		}
	}

	// Extract error information
	if errorField := e.getErrorField(response); errorField != nil {
		metrics["error_count"] = 1
		metrics["error_present"] = true
	} else {
		metrics["error_count"] = 0
		metrics["error_present"] = false
	}

	return metrics
}

func (e *ContextEnricher) buildEvidence(response interface{}, toolName string) []EvidenceItem {
	evidence := make([]EvidenceItem, 0)

	// Add operation evidence
	evidence = append(evidence, EvidenceItem{
		Type:        "operation",
		Source:      toolName,
		Description: fmt.Sprintf("Result from %s operation", toolName),
		Weight:      1.0,
		Details: map[string]interface{}{
			"tool_name": toolName,
			"timestamp": time.Now(),
		},
	})

	// Add success/failure evidence
	if success, found := e.getBooleanField(response, "success", "Success"); found {
		evidence = append(evidence, EvidenceItem{
			Type:        "result",
			Source:      "operation_result",
			Description: fmt.Sprintf("Operation success status: %v", success),
			Weight:      0.9,
			Details: map[string]interface{}{
				"success": success,
			},
		})
	}

	return evidence
}

func (e *ContextEnricher) buildReasoningContext(response interface{}, toolName string) map[string]interface{} {
	context := make(map[string]interface{})

	context["tool_name"] = toolName
	context["operation_timestamp"] = time.Now()
	context["response_type"] = reflect.TypeOf(response).String()

	// Add response summary
	if data, err := json.Marshal(response); err == nil {
		context["response_size"] = len(data)
		context["has_structured_data"] = true
	}

	// Add success context
	if success, found := e.getBooleanField(response, "success", "Success"); found {
		context["operation_successful"] = success
		if success {
			context["reasoning_focus"] = "optimization_opportunities"
		} else {
			context["reasoning_focus"] = "error_resolution"
		}
	}

	return context
}

func (e *ContextEnricher) extractTradeoffContext(response interface{}) map[string]interface{} {
	context := make(map[string]interface{})

	// Extract basic context from response structure
	v := reflect.ValueOf(response)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() == reflect.Struct {
		context["response_fields"] = v.NumField()
		context["response_type"] = v.Type().String()
	}

	return context
}

// generateOperationID creates a unique operation ID
func generateOperationID() string {
	return fmt.Sprintf("op-%d", time.Now().UnixNano())
}

// Default implementations

// DefaultScoreCalculator provides basic scoring functionality
type DefaultScoreCalculator struct{}

func (c *DefaultScoreCalculator) CalculateScore(data interface{}) int {
	// Basic scoring based on success status
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() == reflect.Struct {
		if field := v.FieldByName("Success"); field.IsValid() && field.Kind() == reflect.Bool {
			if field.Bool() {
				return 85 // Good score for success
			}
			return 30 // Poor score for failure
		}
	}

	return 60 // Default neutral score
}

func (c *DefaultScoreCalculator) DetermineRiskLevel(score int, factors map[string]interface{}) string {
	if score >= 80 {
		return types.SeverityLow
	} else if score >= 60 {
		return types.SeverityMedium
	} else if score >= 40 {
		return types.SeverityHigh
	}
	return types.SeverityCritical
}

func (c *DefaultScoreCalculator) CalculateConfidence(evidence []string) int {
	// More evidence = higher confidence
	confidence := 50 + len(evidence)*10
	if confidence > 100 {
		confidence = 100
	}
	return confidence
}

// DefaultTradeoffAnalyzer provides basic trade-off analysis
type DefaultTradeoffAnalyzer struct{}

func (a *DefaultTradeoffAnalyzer) AnalyzeTradeoffs(options []string, context map[string]interface{}) []TradeoffAnalysis {
	analyses := make([]TradeoffAnalysis, 0)

	for _, option := range options {
		analysis := TradeoffAnalysis{
			Option:       option,
			Category:     "general",
			Benefits:     []Benefit{{Description: "Standard approach", Value: 70}},
			Costs:        []Cost{{Description: "Standard cost", Value: 30}},
			Risks:        []Risk{{Description: "Standard risk", Value: 20}},
			TotalBenefit: 70,
			TotalCost:    30,
			TotalRisk:    20,
			Complexity:   "moderate",
			TimeToValue:  "medium",
			Metadata:     make(map[string]interface{}),
		}
		analyses = append(analyses, analysis)
	}

	return analyses
}

func (a *DefaultTradeoffAnalyzer) CompareAlternatives(alternatives []AlternativeStrategy) *ComparisonMatrix {
	matrix := &ComparisonMatrix{
		Criteria:     []ComparisonCriterion{{Name: "effectiveness", Weight: 1.0}},
		Alternatives: make([]string, len(alternatives)),
		Scores:       make(map[string]map[string]int),
		Weights:      map[string]float64{"effectiveness": 1.0},
		Totals:       make(map[string]float64),
		Confidence:   75,
	}

	for i, alt := range alternatives {
		matrix.Alternatives[i] = alt.Name
		matrix.Scores[alt.Name] = map[string]int{"effectiveness": 70}
		matrix.Totals[alt.Name] = 70.0
	}

	if len(alternatives) > 0 {
		matrix.Winner = alternatives[0].Name
	}

	return matrix
}

func (a *DefaultTradeoffAnalyzer) RecommendBestOption(analysis []TradeoffAnalysis) *DecisionRecommendation {
	if len(analysis) == 0 {
		return &DecisionRecommendation{
			RecommendedOption: "default",
			Confidence:        50,
			Reasoning:         []string{"No alternatives analyzed"},
		}
	}

	best := analysis[0]
	for _, option := range analysis {
		if option.TotalBenefit > best.TotalBenefit {
			best = option
		}
	}

	return &DecisionRecommendation{
		RecommendedOption: best.Option,
		Confidence:        80,
		Reasoning:         []string{fmt.Sprintf("Highest benefit score: %d", best.TotalBenefit)},
		Assumptions:       []string{"Benefits weighted equally"},
	}
}
