package context

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/rs/zerolog"
)

// RelationshipEnricher enriches context with additional relationship information
type RelationshipEnricher struct {
	logger zerolog.Logger
}

// NewRelationshipEnricher creates a new relationship enricher
func NewRelationshipEnricher(logger zerolog.Logger) ContextEnricher {
	return &RelationshipEnricher{
		logger: logger.With().Str("enricher", "relationship").Logger(),
	}
}

// Name returns the enricher name
func (e *RelationshipEnricher) Name() string {
	return "relationship_enricher"
}

// EnrichContext enriches context with relationship information
func (e *RelationshipEnricher) EnrichContext(ctx context.Context, data *ComprehensiveContext) error {
	// Analyze temporal relationships
	temporalRelationships := e.analyzeTemporalRelationships(data)
	data.Relationships = append(data.Relationships, temporalRelationships...)

	// Analyze causal relationships
	causalRelationships := e.analyzeCausalRelationships(data)
	data.Relationships = append(data.Relationships, causalRelationships...)

	// Add relationship metadata
	data.Metadata["relationship_count"] = len(data.Relationships)
	data.Metadata["relationship_types"] = e.getRelationshipTypes(data.Relationships)

	e.logger.Debug().
		Int("relationships_added", len(temporalRelationships)+len(causalRelationships)).
		Msg("Context enriched with relationships")

	return nil
}

// analyzeTemporalRelationships finds temporal relationships
func (e *RelationshipEnricher) analyzeTemporalRelationships(data *ComprehensiveContext) []*ContextRelationship {
	relationships := make([]*ContextRelationship, 0)

	// Sort events by timestamp
	events := data.RecentEvents
	if len(events) < 2 {
		return relationships
	}

	// Find sequential relationships
	for i := 0; i < len(events)-1; i++ {
		event1 := events[i]
		event2 := events[i+1]

		// Check if events are closely related in time
		timeDiff := event2.Timestamp.Sub(event1.Timestamp)
		if timeDiff < 5*time.Minute {
			relationships = append(relationships, &ContextRelationship{
				Source:      event1.ID,
				Target:      event2.ID,
				Type:        "temporal_sequence",
				Strength:    1.0 - (timeDiff.Minutes() / 5.0),
				Description: fmt.Sprintf("%s followed by %s", event1.Type, event2.Type),
			})
		}
	}

	return relationships
}

// analyzeCausalRelationships finds causal relationships
func (e *RelationshipEnricher) analyzeCausalRelationships(data *ComprehensiveContext) []*ContextRelationship {
	relationships := make([]*ContextRelationship, 0)

	// Analyze build-deploy relationships
	for toolName, toolContext := range data.ToolContexts {
		if toolName == "build" && toolContext.Type == ContextTypeBuild {
			// Check for deployment after build
			if deployContext, exists := data.ToolContexts["deployment"]; exists {
				if deployContext.Timestamp.After(toolContext.Timestamp) {
					relationships = append(relationships, &ContextRelationship{
						Source:      "build",
						Target:      "deployment",
						Type:        "causal",
						Strength:    0.9,
						Description: "Build triggers deployment",
					})
				}
			}
		}
	}

	return relationships
}

// getRelationshipTypes extracts unique relationship types
func (e *RelationshipEnricher) getRelationshipTypes(relationships []*ContextRelationship) []string {
	types := make(map[string]bool)
	for _, rel := range relationships {
		types[rel.Type] = true
	}

	result := make([]string, 0, len(types))
	for t := range types {
		result = append(result, t)
	}
	return result
}

// InsightEnricher enriches context with additional insights
type InsightEnricher struct {
	knowledgeBase *build.CrossToolKnowledgeBase
	logger        zerolog.Logger
}

// NewInsightEnricher creates a new insight enricher
func NewInsightEnricher(knowledgeBase *build.CrossToolKnowledgeBase, logger zerolog.Logger) ContextEnricher {
	return &InsightEnricher{
		knowledgeBase: knowledgeBase,
		logger:        logger.With().Str("enricher", "insight").Logger(),
	}
}

// Name returns the enricher name
func (e *InsightEnricher) Name() string {
	return "insight_enricher"
}

// EnrichContext enriches context with insights
func (e *InsightEnricher) EnrichContext(ctx context.Context, data *ComprehensiveContext) error {
	// Enhance patterns with historical data
	if data.AnalysisInsights != nil {
		e.enhancePatterns(data.AnalysisInsights.Patterns)
		e.enhanceAnomalies(data.AnalysisInsights.Anomalies)
		e.enhancePredictions(data.AnalysisInsights.PredictedIssues)
	}

	// Add cross-tool insights
	crossToolInsights := e.generateCrossToolInsights(data)
	if len(crossToolInsights) > 0 {
		data.Metadata["cross_tool_insights"] = crossToolInsights
	}

	e.logger.Debug().Msg("Context enriched with insights")
	return nil
}

// enhancePatterns enhances patterns with additional information
func (e *InsightEnricher) enhancePatterns(patterns []*Pattern) {
	for _, pattern := range patterns {
		// Add pattern categorization
		pattern.Type = e.categorizePattern(pattern)

		// Adjust confidence based on occurrences
		if pattern.Occurrences > 10 {
			pattern.Confidence = min(pattern.Confidence*1.2, 1.0)
		}
	}
}

// enhanceAnomalies enhances anomalies with severity assessment
func (e *InsightEnricher) enhanceAnomalies(anomalies []*Anomaly) {
	for _, anomaly := range anomalies {
		// Assess severity based on type
		anomaly.Severity = e.assessAnomalySeverity(anomaly)
	}
}

// enhancePredictions enhances predictions with mitigation strategies
func (e *InsightEnricher) enhancePredictions(predictions []*PredictedIssue) {
	for _, prediction := range predictions {
		// Add detailed mitigations based on issue type
		prediction.Mitigations = e.generateMitigations(prediction.Type)
	}
}

// categorizePattern categorizes a pattern
func (e *InsightEnricher) categorizePattern(pattern *Pattern) string {
	switch {
	case pattern.Type == "repeated_failure":
		return "reliability_issue"
	case pattern.Type == "performance_degradation":
		return "performance_issue"
	case pattern.Type == "resource_spike":
		return "resource_issue"
	default:
		return "general_pattern"
	}
}

// assessAnomalySeverity assesses anomaly severity
func (e *InsightEnricher) assessAnomalySeverity(anomaly *Anomaly) string {
	// Simple severity assessment based on type
	switch anomaly.Type {
	case "security_breach", "data_loss":
		return "critical"
	case "performance_degradation", "service_disruption":
		return "high"
	case "resource_anomaly", "config_drift":
		return "medium"
	default:
		return "low"
	}
}

// generateMitigations generates mitigation strategies
func (e *InsightEnricher) generateMitigations(issueType string) []string {
	mitigations := map[string][]string{
		"resource_exhaustion": {
			"Increase resource quotas",
			"Implement resource cleanup policies",
			"Enable auto-scaling",
			"Optimize resource usage",
		},
		"build_failure": {
			"Review build configuration",
			"Check dependency versions",
			"Enable incremental builds",
			"Implement build caching",
		},
		"deployment_failure": {
			"Validate manifests",
			"Check cluster connectivity",
			"Review resource requirements",
			"Implement rollback strategy",
		},
	}

	if m, exists := mitigations[issueType]; exists {
		return m
	}
	return []string{"Review logs", "Contact support", "Check documentation"}
}

// generateCrossToolInsights generates insights across tools
func (e *InsightEnricher) generateCrossToolInsights(data *ComprehensiveContext) []map[string]interface{} {
	insights := make([]map[string]interface{}, 0)

	// Check for build-deploy misalignment
	if buildCtx, hasBuild := data.ToolContexts["build"]; hasBuild {
		if deployCtx, hasDeploy := data.ToolContexts["deployment"]; hasDeploy {
			buildData := buildCtx.Data
			deployData := deployCtx.Data

			// Check if deployed image matches built image
			if dockerBuild, ok := buildData["docker_build"].(map[string]interface{}); ok {
				if buildImages, ok := dockerBuild["images_built"].(int); ok {
					if kubernetesData, ok := deployData["kubernetes"].(map[string]interface{}); ok {
						if deployManifests, ok := kubernetesData["manifests_count"].(int); ok {
							if buildImages > 0 && deployManifests == 0 {
								insights = append(insights, map[string]interface{}{
									"type":        "build_deploy_gap",
									"description": "Images built but not deployed",
									"severity":    "medium",
									"action":      "Consider deploying built images",
								})
							}
						}
					}
				}
			}
		}
	}

	return insights
}

// min returns the minimum of two float64 values
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// SecurityEnricher enriches context with security information
type SecurityEnricher struct {
	sessionManager *session.SessionManager
	logger         zerolog.Logger
}

// NewSecurityEnricher creates a new security enricher
func NewSecurityEnricher(sessionManager *session.SessionManager, logger zerolog.Logger) ContextEnricher {
	return &SecurityEnricher{
		sessionManager: sessionManager,
		logger:         logger.With().Str("enricher", "security").Logger(),
	}
}

// Name returns the enricher name
func (e *SecurityEnricher) Name() string {
	return "security_enricher"
}

// EnrichContext enriches context with security information
func (e *SecurityEnricher) EnrichContext(ctx context.Context, data *ComprehensiveContext) error {
	// Add security risk assessment
	riskAssessment := e.assessSecurityRisk(data)
	data.Metadata["security_risk_level"] = riskAssessment.Level
	data.Metadata["security_risk_score"] = riskAssessment.Score

	// Add security recommendations if high risk
	if riskAssessment.Level == "high" || riskAssessment.Level == "critical" {
		securityRec := &Recommendation{
			ID:          fmt.Sprintf("sec_rec_%d", time.Now().UnixNano()),
			Type:        "security",
			Priority:    "high",
			Title:       "Security Risk Detected",
			Description: fmt.Sprintf("Security risk level: %s (score: %.2f)", riskAssessment.Level, riskAssessment.Score),
			Actions:     riskAssessment.Recommendations,
			Confidence:  0.9,
		}
		data.Recommendations = append(data.Recommendations, securityRec)
	}

	e.logger.Debug().
		Str("risk_level", riskAssessment.Level).
		Float64("risk_score", riskAssessment.Score).
		Msg("Context enriched with security assessment")

	return nil
}

// SecurityRiskAssessment represents a security risk assessment
type SecurityRiskAssessment struct {
	Level           string
	Score           float64
	Factors         []string
	Recommendations []string
}

// assessSecurityRisk assesses security risk from context
func (e *SecurityEnricher) assessSecurityRisk(data *ComprehensiveContext) *SecurityRiskAssessment {
	assessment := &SecurityRiskAssessment{
		Level:           "low",
		Score:           0.0,
		Factors:         make([]string, 0),
		Recommendations: make([]string, 0),
	}

	// Check security context
	if secContext, exists := data.ToolContexts["security"]; exists {
		if secData, ok := secContext.Data["security_scans"].(map[string]interface{}); ok {
			// Assess based on vulnerability counts
			if critical, ok := secData["critical_issues"].(int); ok && critical > 0 {
				assessment.Score += float64(critical) * 0.3
				assessment.Factors = append(assessment.Factors, fmt.Sprintf("%d critical vulnerabilities", critical))
				assessment.Recommendations = append(assessment.Recommendations, "Address critical vulnerabilities immediately")
			}

			if high, ok := secData["high_issues"].(int); ok && high > 0 {
				assessment.Score += float64(high) * 0.1
				assessment.Factors = append(assessment.Factors, fmt.Sprintf("%d high severity issues", high))
				assessment.Recommendations = append(assessment.Recommendations, "Review and fix high severity issues")
			}
		}
	}

	// Check for security anomalies
	if data.AnalysisInsights != nil {
		for _, anomaly := range data.AnalysisInsights.Anomalies {
			if anomaly.Type == "security_breach" || anomaly.Type == "unauthorized_access" {
				assessment.Score += 0.5
				assessment.Factors = append(assessment.Factors, anomaly.Description)
				assessment.Recommendations = append(assessment.Recommendations, "Investigate security anomaly: "+anomaly.Description)
			}
		}
	}

	// Determine risk level based on score
	switch {
	case assessment.Score >= 1.0:
		assessment.Level = "critical"
	case assessment.Score >= 0.7:
		assessment.Level = "high"
	case assessment.Score >= 0.4:
		assessment.Level = "medium"
	default:
		assessment.Level = "low"
	}

	// Cap score at 1.0
	if assessment.Score > 1.0 {
		assessment.Score = 1.0
	}

	return assessment
}

// PerformanceEnricher enriches context with performance insights
type PerformanceEnricher struct {
	logger zerolog.Logger
}

// NewPerformanceEnricher creates a new performance enricher
func NewPerformanceEnricher(logger zerolog.Logger) ContextEnricher {
	return &PerformanceEnricher{
		logger: logger.With().Str("enricher", "performance").Logger(),
	}
}

// Name returns the enricher name
func (e *PerformanceEnricher) Name() string {
	return "performance_enricher"
}

// EnrichContext enriches context with performance insights
func (e *PerformanceEnricher) EnrichContext(ctx context.Context, data *ComprehensiveContext) error {
	// Analyze performance trends
	perfAnalysis := e.analyzePerformance(data)

	// Add performance metadata
	data.Metadata["performance_score"] = perfAnalysis.Score
	data.Metadata["performance_bottlenecks"] = perfAnalysis.Bottlenecks

	// Add performance recommendations
	if perfAnalysis.Score < 0.7 {
		for _, bottleneck := range perfAnalysis.Bottlenecks {
			rec := &Recommendation{
				ID:          fmt.Sprintf("perf_rec_%s_%d", bottleneck.Type, time.Now().UnixNano()),
				Type:        "performance",
				Priority:    bottleneck.Priority,
				Title:       fmt.Sprintf("Performance Bottleneck: %s", bottleneck.Name),
				Description: bottleneck.Description,
				Actions:     bottleneck.Recommendations,
				Confidence:  bottleneck.Confidence,
			}
			data.Recommendations = append(data.Recommendations, rec)
		}
	}

	e.logger.Debug().
		Float64("performance_score", perfAnalysis.Score).
		Int("bottlenecks", len(perfAnalysis.Bottlenecks)).
		Msg("Context enriched with performance analysis")

	return nil
}

// PerformanceAnalysis represents performance analysis results
type PerformanceAnalysis struct {
	Score       float64
	Bottlenecks []*PerformanceBottleneck
}

// PerformanceBottleneck represents a performance bottleneck
type PerformanceBottleneck struct {
	Type            string
	Name            string
	Description     string
	Priority        string
	Impact          float64
	Confidence      float64
	Recommendations []string
}

// analyzePerformance analyzes performance from context
func (e *PerformanceEnricher) analyzePerformance(data *ComprehensiveContext) *PerformanceAnalysis {
	analysis := &PerformanceAnalysis{
		Score:       1.0,
		Bottlenecks: make([]*PerformanceBottleneck, 0),
	}

	// Check performance context
	if perfContext, exists := data.ToolContexts["performance"]; exists {
		if perfData, ok := perfContext.Data["performance_metrics"].(map[string]interface{}); ok {
			// Check CPU usage
			if cpu, ok := perfData["cpu_usage"].(float64); ok && cpu > 80 {
				analysis.Score -= 0.2
				analysis.Bottlenecks = append(analysis.Bottlenecks, &PerformanceBottleneck{
					Type:        "cpu",
					Name:        "High CPU Usage",
					Description: fmt.Sprintf("CPU usage at %.1f%%", cpu),
					Priority:    "high",
					Impact:      0.8,
					Confidence:  0.9,
					Recommendations: []string{
						"Optimize CPU-intensive operations",
						"Consider scaling horizontally",
						"Profile application for CPU hotspots",
					},
				})
			}

			// Check memory usage
			if memory, ok := perfData["memory_usage"].(float64); ok && memory > 85 {
				analysis.Score -= 0.15
				analysis.Bottlenecks = append(analysis.Bottlenecks, &PerformanceBottleneck{
					Type:        "memory",
					Name:        "High Memory Usage",
					Description: fmt.Sprintf("Memory usage at %.1f%%", memory),
					Priority:    "high",
					Impact:      0.7,
					Confidence:  0.9,
					Recommendations: []string{
						"Investigate memory leaks",
						"Optimize memory allocation",
						"Increase memory limits if needed",
					},
				})
			}

			// Check error rate
			if errorRate, ok := perfData["error_rate"].(float64); ok && errorRate > 0.05 {
				analysis.Score -= 0.3
				analysis.Bottlenecks = append(analysis.Bottlenecks, &PerformanceBottleneck{
					Type:        "reliability",
					Name:        "High Error Rate",
					Description: fmt.Sprintf("Error rate at %.1f%%", errorRate*100),
					Priority:    "critical",
					Impact:      0.9,
					Confidence:  0.95,
					Recommendations: []string{
						"Investigate error patterns",
						"Implement better error handling",
						"Add retry mechanisms",
					},
				})
			}
		}
	}

	// Ensure score doesn't go below 0
	if analysis.Score < 0 {
		analysis.Score = 0
	}

	return analysis
}
