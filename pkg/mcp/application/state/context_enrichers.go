package appstate

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/application/knowledge"
)

// StateSessionStore interface for session storage operations
// StateSessionStore - Use services.SessionStore for the canonical interface
// This version is simplified for state management operations
// Deprecated: Use services.SessionStore for new code
type StateSessionStore interface {
	// Create creates a new session
	Create(ctx context.Context, session *api.Session) error
	// Get retrieves a session by ID
	Get(ctx context.Context, sessionID string) (*api.Session, error)
	// Update updates an existing session
	Update(ctx context.Context, session *api.Session) error
	// Delete removes a session
	Delete(ctx context.Context, sessionID string) error
	// List returns all sessions
	List(ctx context.Context) ([]*api.Session, error)
}

// StateSessionState interface for session state management
// StateSessionState - Use services.SessionState for the canonical interface
// This version is simplified for state management operations
// Deprecated: Use services.SessionState for new code
type StateSessionState interface {
	// SaveState saves the current state for a session
	SaveState(ctx context.Context, sessionID string, state map[string]interface{}) error
	// GetState retrieves the state for a session
	GetState(ctx context.Context, sessionID string) (map[string]interface{}, error)
}

// RelationshipEnricher enriches context with additional relationship information
type RelationshipEnricher struct {
	logger *slog.Logger
}

// NewRelationshipEnricher creates a new relationship enricher
func NewRelationshipEnricher(logger *slog.Logger) ContextEnricher {
	return &RelationshipEnricher{
		logger: logger.With(slog.String("enricher", "relationship")),
	}
}

// GetName returns the enricher name
func (e *RelationshipEnricher) GetName() string {
	return "relationship_enricher"
}

// Enrich enriches context with relationship information
func (e *RelationshipEnricher) Enrich(ctx context.Context, data *ComprehensiveContext) error {
	e.logger.Debug("Enriching context with relationships")

	// Add temporal relationships
	temporalRelationships := e.analyzeTemporalRelationships(data)
	data.Relationships = append(data.Relationships, temporalRelationships...)

	// Add causal relationships
	causalRelationships := e.analyzeCausalRelationships(data)
	data.Relationships = append(data.Relationships, causalRelationships...)

	// Update metadata
	if data.Metadata == nil {
		data.Metadata = make(map[string]interface{})
	}
	data.Metadata["relationship_count"] = len(data.Relationships)
	data.Metadata["relationship_types"] = e.getRelationshipTypes(data.Relationships)

	e.logger.Info("Added relationships to context", slog.Int("count", len(data.Relationships)))

	return nil
}

// analyzeTemporalRelationships finds temporal relationships
func (e *RelationshipEnricher) analyzeTemporalRelationships(data *ComprehensiveContext) []*ContextRelationship {
	relationships := make([]*ContextRelationship, 0)

	events := data.RecentEvents
	if len(events) < 2 {
		return relationships
	}

	for i := 0; i < len(events)-1; i++ {
		event1 := events[i]
		event2 := events[i+1]

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

	for toolName, toolContext := range data.ToolContexts {
		if toolName == "build" && toolContext.Type == ContextTypeBuild {
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
	knowledgeBase *knowledge.CrossToolKnowledgeBase
	logger        *slog.Logger
}

// NewInsightEnricher creates a new insight enricher
func NewInsightEnricher(knowledgeBase *knowledge.CrossToolKnowledgeBase, logger *slog.Logger) ContextEnricher {
	return &InsightEnricher{
		knowledgeBase: knowledgeBase,
		logger:        logger.With(slog.String("enricher", "insight")),
	}
}

// GetName returns the enricher name
func (e *InsightEnricher) GetName() string {
	return "insight_enricher"
}

// Enrich enriches context with insights
func (e *InsightEnricher) Enrich(ctx context.Context, data *ComprehensiveContext) error {
	e.logger.Debug("Enriching context with insights")
	//
	// Generate insights based on current data
	insights := e.generateInsights(data)
	//
	// Enhance existing insights if present
	if data.AnalysisInsights != nil {
		e.enhancePatterns(data.AnalysisInsights.Patterns)
		e.enhanceAnomalies(data.AnalysisInsights.Anomalies)
		e.enhancePredictions(data.AnalysisInsights.PredictedIssues)
	} else {
		data.AnalysisInsights = insights
	}
	//
	// Generate recommendations based on insights
	recommendations := e.generateRecommendations(data)
	data.Recommendations = append(data.Recommendations, recommendations...)
	//
	// Add cross-tool insights to metadata
	crossToolInsights := e.generateCrossToolInsights(data)
	if len(crossToolInsights) > 0 {
		if data.Metadata == nil {
			data.Metadata = make(map[string]interface{})
		}
		data.Metadata["cross_tool_insights"] = crossToolInsights
	}
	//
	e.logger.Info("Generated insights and recommendations", slog.Int("recommendations", len(recommendations)))
	//
	return nil
}

// enhancePatterns enhances patterns with additional information
func (e *InsightEnricher) enhancePatterns(patterns []*Pattern) {
	for _, pattern := range patterns {
		pattern.Type = e.categorizePattern(pattern)
		//
		if pattern.Occurrences > 10 {
			pattern.Confidence = min(pattern.Confidence*1.2, 1.0)
		}
	}
}

// enhanceAnomalies enhances anomalies with severity assessment
func (e *InsightEnricher) enhanceAnomalies(anomalies []*Anomaly) {
	for _, anomaly := range anomalies {
		anomaly.Severity = e.assessAnomalySeverity(anomaly)
	}
}

// enhancePredictions enhances predictions with mitigation strategies
func (e *InsightEnricher) enhancePredictions(predictions []*PredictedIssue) {
	for _, prediction := range predictions {
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
	//
	if m, exists := mitigations[issueType]; exists {
		return m
	}
	return []string{"Review logs", "Contact support", "Check documentation"}
}

// generateInsights generates initial insights from context data
func (e *InsightEnricher) generateInsights(data *ComprehensiveContext) *AnalysisInsights {
	insights := &AnalysisInsights{
		Patterns:        make([]*Pattern, 0),
		Anomalies:       make([]*Anomaly, 0),
		PredictedIssues: make([]*PredictedIssue, 0),
	}
	//
	// Analyze patterns in recent events
	if len(data.RecentEvents) > 5 {
		patterns := e.detectPatterns(data.RecentEvents)
		insights.Patterns = append(insights.Patterns, patterns...)
	}
	//
	// Detect anomalies
	anomalies := e.detectAnomalies(data)
	insights.Anomalies = append(insights.Anomalies, anomalies...)
	//
	// Predict potential issues
	predictions := e.predictIssues(data)
	insights.PredictedIssues = append(insights.PredictedIssues, predictions...)
	//
	return insights
}

// generateRecommendations generates recommendations based on insights
func (e *InsightEnricher) generateRecommendations(data *ComprehensiveContext) []*Recommendation {
	recommendations := make([]*Recommendation, 0)
	//
	// Generate recommendations from analysis insights
	if data.AnalysisInsights != nil {
		// From patterns
		for _, pattern := range data.AnalysisInsights.Patterns {
			if pattern.Confidence > 0.7 {
				rec := &Recommendation{
					ID:          fmt.Sprintf("pattern-%s-%d", pattern.Type, time.Now().UnixNano()),
					Title:       fmt.Sprintf("Pattern Detected: %s", pattern.Type),
					Description: pattern.Description,
					Priority:    e.calculatePatternPriority(pattern),
					Category:    "pattern",
					Actions:     e.getPatternActions(pattern.Type),
					Confidence:  pattern.Confidence,
					CreatedAt:   time.Now(),
				}
				recommendations = append(recommendations, rec)
			}
		}
		//
		// From anomalies
		for _, anomaly := range data.AnalysisInsights.Anomalies {
			if anomaly.Severity == "high" || anomaly.Severity == "critical" {
				rec := &Recommendation{
					ID:          fmt.Sprintf("anomaly-%s-%d", anomaly.Type, time.Now().UnixNano()),
					Title:       fmt.Sprintf("Anomaly: %s", anomaly.Type),
					Description: anomaly.Description,
					Priority:    e.anomalySeverityToPriority(anomaly.Severity),
					Category:    "anomaly",
					Actions:     e.getAnomalyActions(anomaly.Type),
					Confidence:  0.85,
					CreatedAt:   time.Now(),
				}
				recommendations = append(recommendations, rec)
			}
		}
		//
		// From predicted issues
		for _, prediction := range data.AnalysisInsights.PredictedIssues {
			if prediction.Probability > 0.6 {
				rec := &Recommendation{
					ID:          fmt.Sprintf("predict-%s-%d", prediction.Type, time.Now().UnixNano()),
					Title:       fmt.Sprintf("Potential Issue: %s", prediction.Type),
					Description: prediction.Description,
					Priority:    e.calculatePredictionPriority(prediction),
					Category:    "prediction",
					Actions:     prediction.Mitigations,
					Confidence:  prediction.Probability,
					CreatedAt:   time.Now(),
				}
				recommendations = append(recommendations, rec)
			}
		}
	}
	//
	return recommendations
}

// Helper methods for InsightEnricher
func (e *InsightEnricher) detectPatterns(events []*Event) []*Pattern {
	patterns := make([]*Pattern, 0)
	// Simple pattern detection logic
	typeCount := make(map[string]int)
	for _, event := range events {
		typeCount[event.Type]++
	}
	//
	for eventType, count := range typeCount {
		if count > 3 {
			patterns = append(patterns, &Pattern{
				Type:        eventType,
				Description: fmt.Sprintf("Repeated %s events detected", eventType),
				Occurrences: count,
				Confidence:  float64(count) / float64(len(events)),
			})
		}
	}
	//
	return patterns
}

func (e *InsightEnricher) detectAnomalies(data *ComprehensiveContext) []*Anomaly {
	anomalies := make([]*Anomaly, 0)
	// Simple anomaly detection
	return anomalies
}

func (e *InsightEnricher) predictIssues(data *ComprehensiveContext) []*PredictedIssue {
	predictions := make([]*PredictedIssue, 0)
	// Simple prediction logic
	return predictions
}

func (e *InsightEnricher) calculatePatternPriority(pattern *Pattern) int {
	if pattern.Occurrences > 10 {
		return 1
	} else if pattern.Occurrences > 5 {
		return 2
	}
	return 3
}

func (e *InsightEnricher) anomalySeverityToPriority(severity string) int {
	switch severity {
	case "critical":
		return 1
	case "high":
		return 2
	case "medium":
		return 3
	default:
		return 4
	}
}

func (e *InsightEnricher) calculatePredictionPriority(prediction *PredictedIssue) int {
	if prediction.Probability > 0.8 {
		return 1
	} else if prediction.Probability > 0.6 {
		return 2
	}
	return 3
}

func (e *InsightEnricher) getPatternActions(patternType string) []string {
	// Return appropriate actions based on pattern type
	return []string{"Review pattern occurrences", "Monitor for escalation", "Consider automation"}
}

func (e *InsightEnricher) getAnomalyActions(anomalyType string) []string {
	// Return appropriate actions based on anomaly type
	return []string{"Investigate anomaly", "Review system logs", "Check for security issues"}
}

// generateCrossToolInsights generates insights across tools
func (e *InsightEnricher) generateCrossToolInsights(data *ComprehensiveContext) []map[string]interface{} {
	insights := make([]map[string]interface{}, 0)
	//
	if buildCtx, hasBuild := data.ToolContexts["build"]; hasBuild {
		if deployCtx, hasDeploy := data.ToolContexts["deployment"]; hasDeploy {
			buildData := buildCtx.Data
			deployData := deployCtx.Data
			//
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
	//
	return insights
}

// SecurityEnricher enriches context with security information
type SecurityEnricher struct {
	sessionStore StateSessionStore
	sessionState StateSessionState
	logger       *slog.Logger
}

// NewSecurityEnricher creates a new security enricher
func NewSecurityEnricher(sessionStore StateSessionStore, sessionState StateSessionState, logger *slog.Logger) ContextEnricher {
	return &SecurityEnricher{
		sessionStore: sessionStore,
		sessionState: sessionState,
		logger:       logger.With(slog.String("enricher", "security")),
	}
}

// GetName returns the enricher name
func (e *SecurityEnricher) GetName() string {
	return "security_enricher"
}

// Enrich enriches context with security information
func (e *SecurityEnricher) Enrich(ctx context.Context, data *ComprehensiveContext) error {
	e.logger.Debug("Enriching context with security analysis")
	//
	// Perform security assessment
	riskAssessment := e.assessSecurityRisks(data)
	//
	// Create security recommendations
	for _, risk := range riskAssessment.HighRisks {
		rec := &Recommendation{
			ID:          fmt.Sprintf("sec-%s", risk.ID),
			Title:       fmt.Sprintf("Security: %s", risk.Title),
			Description: risk.Description,
			Priority:    1, // High priority (int, not string)
			Category:    "security",
			Actions:     risk.Mitigations,
			Confidence:  0.9,
			CreatedAt:   time.Now(),
		}
		data.Recommendations = append(data.Recommendations, rec)
	}
	//
	// Add security metadata
	if data.Metadata == nil {
		data.Metadata = make(map[string]interface{})
	}
	data.Metadata["security_score"] = riskAssessment.Score
	data.Metadata["security_risks"] = len(riskAssessment.HighRisks)
	//
	return nil
}

// SecurityRiskAssessment represents a security risk assessment
type SecurityRiskAssessment struct {
	Level           string
	Score           float64
	Factors         []string
	Recommendations []string
	HighRisks       []*SecurityRisk
}

// SecurityRisk represents a specific security risk
type SecurityRisk struct {
	ID          string
	Title       string
	Description string
	Severity    string
	Mitigations []string
}

// assessSecurityRisks assesses security risks from context
func (e *SecurityEnricher) assessSecurityRisks(data *ComprehensiveContext) *SecurityRiskAssessment {
	assessment := &SecurityRiskAssessment{
		Level:           "low",
		Score:           0.0,
		Factors:         make([]string, 0),
		Recommendations: make([]string, 0),
		HighRisks:       make([]*SecurityRisk, 0),
	}

	if secContext, exists := data.ToolContexts["security"]; exists {
		if secData, ok := secContext.Data["security_scans"].(map[string]interface{}); ok {
			if critical, ok := secData["critical_issues"].(int); ok && critical > 0 {
				assessment.Score += float64(critical) * 0.3
				assessment.Factors = append(assessment.Factors, fmt.Sprintf("%d critical vulnerabilities", critical))
				assessment.Recommendations = append(assessment.Recommendations, "Address critical vulnerabilities immediately")

				// Add high risk for critical issues
				risk := &SecurityRisk{
					ID:          fmt.Sprintf("vuln-critical-%d", time.Now().UnixNano()),
					Title:       "Critical Vulnerabilities Detected",
					Description: fmt.Sprintf("Found %d critical security vulnerabilities", critical),
					Severity:    "critical",
					Mitigations: []string{
						"Update vulnerable dependencies",
						"Apply security patches",
						"Review security configuration",
					},
				}
				assessment.HighRisks = append(assessment.HighRisks, risk)
			}

			if high, ok := secData["high_issues"].(int); ok && high > 0 {
				assessment.Score += float64(high) * 0.1
				assessment.Factors = append(assessment.Factors, fmt.Sprintf("%d high severity issues", high))
				assessment.Recommendations = append(assessment.Recommendations, "Review and fix high severity issues")

				// Add high risk for high severity issues
				risk := &SecurityRisk{
					ID:          fmt.Sprintf("vuln-high-%d", time.Now().UnixNano()),
					Title:       "High Severity Issues Found",
					Description: fmt.Sprintf("Found %d high severity security issues", high),
					Severity:    "high",
					Mitigations: []string{
						"Review security findings",
						"Implement security best practices",
						"Enable security monitoring",
					},
				}
				assessment.HighRisks = append(assessment.HighRisks, risk)
			}
		}
	}

	if data.AnalysisInsights != nil {
		for _, anomaly := range data.AnalysisInsights.Anomalies {
			if anomaly.Type == "security_breach" || anomaly.Type == "unauthorized_access" {
				assessment.Score += 0.5
				assessment.Factors = append(assessment.Factors, anomaly.Description)
				assessment.Recommendations = append(assessment.Recommendations, "Investigate security anomaly: "+anomaly.Description)
			}
		}
	}

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

	if assessment.Score > 1.0 {
		assessment.Score = 1.0
	}

	return assessment
}

// PerformanceEnricher enriches context with performance insights
type PerformanceEnricher struct {
	logger *slog.Logger
}

// NewPerformanceEnricher creates a new performance enricher
func NewPerformanceEnricher(logger *slog.Logger) ContextEnricher {
	return &PerformanceEnricher{
		logger: logger.With("enricher", "performance"),
	}
}

// GetName returns the enricher name
func (e *PerformanceEnricher) GetName() string {
	return "performance_enricher"
}

// Enrich enriches context with performance insights
func (e *PerformanceEnricher) Enrich(ctx context.Context, data *ComprehensiveContext) error {
	e.logger.Debug("Enriching context with performance analysis")

	// Analyze performance bottlenecks
	bottlenecks := e.identifyBottlenecks(data)

	// Create performance recommendations
	for _, bottleneck := range bottlenecks {
		rec := &Recommendation{
			ID:          fmt.Sprintf("perf-%s", bottleneck.ID),
			Title:       bottleneck.Title,
			Description: bottleneck.Description,
			Priority:    e.calculatePriority(bottleneck),
			Category:    "performance",
			Actions:     bottleneck.Recommendations,
			Confidence:  bottleneck.Confidence,
			CreatedAt:   time.Now(),
		}
		data.Recommendations = append(data.Recommendations, rec)
	}

	// Add performance metrics
	metrics := e.calculateMetrics(data)
	if data.Metrics == nil {
		data.Metrics = make(map[string]float64)
	}
	for k, v := range metrics {
		data.Metrics[k] = v
	}

	return nil
}

// PerformanceAnalysis represents performance analysis results
type PerformanceAnalysis struct {
	Score       float64
	Bottlenecks []*PerformanceBottleneck
}

// PerformanceBottleneck represents a performance bottleneck
type PerformanceBottleneck struct {
	ID              string
	Type            string
	Title           string
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

	if perfContext, exists := data.ToolContexts["performance"]; exists {
		if perfData, ok := perfContext.Data["performance_metrics"].(map[string]interface{}); ok {
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

	if analysis.Score < 0 {
		analysis.Score = 0
	}

	return analysis
}

// identifyBottlenecks identifies performance bottlenecks from context
func (e *PerformanceEnricher) identifyBottlenecks(data *ComprehensiveContext) []*PerformanceBottleneck {
	bottlenecks := make([]*PerformanceBottleneck, 0)

	// Analyze performance context
	if perfContext, exists := data.ToolContexts["performance"]; exists {
		if perfData, ok := perfContext.Data["performance_metrics"].(map[string]interface{}); ok {
			// Check CPU usage
			if cpu, ok := perfData["cpu_usage"].(float64); ok && cpu > 80 {
				bottleneck := &PerformanceBottleneck{
					ID:          fmt.Sprintf("cpu-%d", time.Now().UnixNano()),
					Type:        "cpu",
					Title:       "High CPU Usage",
					Name:        "CPU Bottleneck",
					Description: fmt.Sprintf("CPU usage at %.1f%% exceeds threshold", cpu),
					Priority:    "high",
					Impact:      0.8,
					Confidence:  0.9,
					Recommendations: []string{
						"Profile CPU usage to identify hot spots",
						"Optimize compute-intensive operations",
						"Consider horizontal scaling",
					},
				}
				bottlenecks = append(bottlenecks, bottleneck)
			}

			// Check memory usage
			if memory, ok := perfData["memory_usage"].(float64); ok && memory > 85 {
				bottleneck := &PerformanceBottleneck{
					ID:          fmt.Sprintf("memory-%d", time.Now().UnixNano()),
					Type:        "memory",
					Title:       "High Memory Usage",
					Name:        "Memory Pressure",
					Description: fmt.Sprintf("Memory usage at %.1f%% indicates pressure", memory),
					Priority:    "high",
					Impact:      0.7,
					Confidence:  0.9,
					Recommendations: []string{
						"Analyze memory allocation patterns",
						"Check for memory leaks",
						"Optimize data structures",
					},
				}
				bottlenecks = append(bottlenecks, bottleneck)
			}
		}
	}

	return bottlenecks
}

// calculatePriority converts string priority to int
func (e *PerformanceEnricher) calculatePriority(bottleneck *PerformanceBottleneck) int {
	switch bottleneck.Priority {
	case "critical":
		return 1
	case "high":
		return 2
	case "medium":
		return 3
	case "low":
		return 4
	default:
		return 5
	}
}

// calculateMetrics calculates performance metrics from context
func (e *PerformanceEnricher) calculateMetrics(data *ComprehensiveContext) map[string]float64 {
	metrics := make(map[string]float64)

	// Extract metrics from performance context
	if perfContext, exists := data.ToolContexts["performance"]; exists {
		if perfData, ok := perfContext.Data["performance_metrics"].(map[string]interface{}); ok {
			// Add CPU metrics
			if cpu, ok := perfData["cpu_usage"].(float64); ok {
				metrics["cpu_usage_percent"] = cpu
			}

			// Add memory metrics
			if memory, ok := perfData["memory_usage"].(float64); ok {
				metrics["memory_usage_percent"] = memory
			}

			// Add error rate metrics
			if errorRate, ok := perfData["error_rate"].(float64); ok {
				metrics["error_rate"] = errorRate
			}

			// Add response time metrics
			if responseTime, ok := perfData["response_time_ms"].(float64); ok {
				metrics["response_time_ms"] = responseTime
			}
		}
	}

	// Calculate derived metrics
	if cpuUsage, hasCPU := metrics["cpu_usage_percent"]; hasCPU {
		if memUsage, hasMem := metrics["memory_usage_percent"]; hasMem {
			// Overall resource utilization
			metrics["resource_utilization"] = (cpuUsage + memUsage) / 2.0
		}
	}

	return metrics
}
