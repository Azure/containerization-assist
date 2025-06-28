package context

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/state"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// AIContextAggregator aggregates context from tools
type AIContextAggregator struct {
	stateManager     *state.UnifiedStateManager
	sessionManager   *session.SessionManager
	contextProviders map[string]ContextProvider
	contextEnrichers []ContextEnricher
	contextCache     *ContextCache
	mu               sync.RWMutex
	logger           zerolog.Logger
}

// ContextProvider provides tool context
type ContextProvider interface {
	GetContextData(ctx context.Context, request *ContextRequest) (*ContextData, error)
	GetCapabilities() *ContextProviderCapabilities
}

// ContextEnricher enriches context
type ContextEnricher interface {
	EnrichContext(ctx context.Context, data *ComprehensiveContext) error
	Name() string
}

// ContextRequest represents context request
type ContextRequest struct {
	SessionID      string
	ToolName       string
	ContextType    ContextType
	TimeRange      *TimeRange
	IncludeHistory bool
	MaxItems       int
	Filters        map[string]interface{}
}

// ContextType represents context type
type ContextType string

const (
	ContextTypeBuild       ContextType = "build"
	ContextTypeDeployment  ContextType = "deployment"
	ContextTypeAnalysis    ContextType = "analysis"
	ContextTypeState       ContextType = "state"
	ContextTypePerformance ContextType = "performance"
	ContextTypeSecurity    ContextType = "security"
	ContextTypeAll         ContextType = "all"
)

// TimeRange represents time range
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// ContextData represents provider context data
type ContextData struct {
	Provider   string                 `json:"provider"`
	Type       ContextType            `json:"type"`
	Timestamp  time.Time              `json:"timestamp"`
	Data       map[string]interface{} `json:"data"`
	Metadata   map[string]interface{} `json:"metadata"`
	Relevance  float64                `json:"relevance"`
	Confidence float64                `json:"confidence"`
}

// ContextProviderCapabilities represents provider capabilities
type ContextProviderCapabilities struct {
	SupportedTypes  []ContextType
	SupportsHistory bool
	MaxHistoryDays  int
	RealTimeUpdates bool
}

// ComprehensiveContext represents aggregated context
type ComprehensiveContext struct {
	SessionID        string                  `json:"session_id"`
	Timestamp        time.Time               `json:"timestamp"`
	RequestID        string                  `json:"request_id"`
	ToolContexts     map[string]*ContextData `json:"tool_contexts"`
	StateSnapshot    *StateSnapshot          `json:"state_snapshot"`
	RecentEvents     []*Event                `json:"recent_events"`
	Relationships    []*ContextRelationship  `json:"relationships"`
	Recommendations  []*Recommendation       `json:"recommendations"`
	AnalysisInsights *AnalysisInsights       `json:"analysis_insights"`
	Metadata         map[string]interface{}  `json:"metadata"`
}

// StateSnapshot represents system state
type StateSnapshot struct {
	SessionState   interface{}            `json:"session_state"`
	WorkflowStates map[string]interface{} `json:"workflow_states"`
	ToolStates     map[string]interface{} `json:"tool_states"`
	GlobalState    map[string]interface{} `json:"global_state"`
	Timestamp      time.Time              `json:"timestamp"`
}

// Event represents system event
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Severity  string                 `json:"severity"`
	Impact    float64                `json:"impact"`
}

// ContextRelationship represents context relationships
type ContextRelationship struct {
	Source      string  `json:"source"`
	Target      string  `json:"target"`
	Type        string  `json:"type"`
	Strength    float64 `json:"strength"`
	Description string  `json:"description"`
}

// Recommendation represents system recommendation
type Recommendation struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Priority    string                 `json:"priority"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Actions     []string               `json:"actions"`
	Confidence  float64                `json:"confidence"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// AnalysisInsights represents analysis insights
type AnalysisInsights struct {
	Patterns        []*Pattern        `json:"patterns"`
	Anomalies       []*Anomaly        `json:"anomalies"`
	Trends          []*Trend          `json:"trends"`
	Correlations    []*Correlation    `json:"correlations"`
	PredictedIssues []*PredictedIssue `json:"predicted_issues"`
}

// Pattern represents detected pattern
type Pattern struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Occurrences int       `json:"occurrences"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	Confidence  float64   `json:"confidence"`
}

// Anomaly represents detected anomaly
type Anomaly struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Severity    string                 `json:"severity"`
	DetectedAt  time.Time              `json:"detected_at"`
	Data        map[string]interface{} `json:"data"`
	Confidence  float64                `json:"confidence"`
}

// Trend represents detected trend
type Trend struct {
	ID         string    `json:"id"`
	Metric     string    `json:"metric"`
	Direction  string    `json:"direction"`
	Rate       float64   `json:"rate"`
	StartTime  time.Time `json:"start_time"`
	Confidence float64   `json:"confidence"`
}

// Correlation represents metric correlation
type Correlation struct {
	Metric1     string  `json:"metric1"`
	Metric2     string  `json:"metric2"`
	Coefficient float64 `json:"coefficient"`
	Confidence  float64 `json:"confidence"`
}

// PredictedIssue represents predicted issue
type PredictedIssue struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"`
	Description   string    `json:"description"`
	Probability   float64   `json:"probability"`
	EstimatedTime time.Time `json:"estimated_time"`
	Impact        string    `json:"impact"`
	Mitigations   []string  `json:"mitigations"`
}

// NewAIContextAggregator creates a new AI context aggregator
func NewAIContextAggregator(
	stateManager *state.UnifiedStateManager,
	sessionManager *session.SessionManager,
	logger zerolog.Logger,
) *AIContextAggregator {
	return &AIContextAggregator{
		stateManager:     stateManager,
		sessionManager:   sessionManager,
		contextProviders: make(map[string]ContextProvider),
		contextEnrichers: make([]ContextEnricher, 0),
		contextCache:     NewContextCache(5 * time.Minute),
		logger:           logger.With().Str("component", "ai_context_aggregator").Logger(),
	}
}

// RegisterContextProvider registers a context provider
func (a *AIContextAggregator) RegisterContextProvider(name string, provider ContextProvider) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.contextProviders[name] = provider
	a.logger.Info().Str("provider", name).Msg("Registered context provider")
}

// RegisterContextEnricher registers a context enricher
func (a *AIContextAggregator) RegisterContextEnricher(enricher ContextEnricher) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.contextEnrichers = append(a.contextEnrichers, enricher)
	a.logger.Info().Str("enricher", enricher.Name()).Msg("Registered context enricher")
}

// GetComprehensiveContext aggregates context from all sources
func (a *AIContextAggregator) GetComprehensiveContext(ctx context.Context, sessionID string) (*ComprehensiveContext, error) {
	// Check cache first
	if cached := a.contextCache.Get(sessionID); cached != nil {
		return cached, nil
	}

	startTime := time.Now()

	// Create comprehensive context
	compContext := &ComprehensiveContext{
		SessionID:     sessionID,
		Timestamp:     time.Now(),
		RequestID:     fmt.Sprintf("ctx_%d", time.Now().UnixNano()),
		ToolContexts:  make(map[string]*ContextData),
		RecentEvents:  make([]*Event, 0),
		Relationships: make([]*ContextRelationship, 0),
		Metadata:      make(map[string]interface{}),
	}

	// Get state snapshot
	snapshot, err := a.getStateSnapshot(ctx, sessionID)
	if err != nil {
		a.logger.Error().Err(err).Msg("Failed to get state snapshot")
	} else {
		compContext.StateSnapshot = snapshot
	}

	// Gather context from all providers
	var wg sync.WaitGroup
	contextChan := make(chan struct {
		name string
		data *ContextData
		err  error
	}, len(a.contextProviders))

	a.mu.RLock()
	providers := make(map[string]ContextProvider)
	for k, v := range a.contextProviders {
		providers[k] = v
	}
	a.mu.RUnlock()

	// Collect context from each provider concurrently
	for name, provider := range providers {
		wg.Add(1)
		go func(n string, p ContextProvider) {
			defer wg.Done()

			request := &ContextRequest{
				SessionID:      sessionID,
				ContextType:    ContextTypeAll,
				IncludeHistory: true,
				MaxItems:       100,
			}

			data, err := p.GetContextData(ctx, request)
			contextChan <- struct {
				name string
				data *ContextData
				err  error
			}{n, data, err}
		}(name, provider)
	}

	// Wait for all providers
	go func() {
		wg.Wait()
		close(contextChan)
	}()

	// Collect results
	for result := range contextChan {
		if result.err != nil {
			a.logger.Error().
				Err(result.err).
				Str("provider", result.name).
				Msg("Failed to get context from provider")
		} else if result.data != nil {
			compContext.ToolContexts[result.name] = result.data
		}
	}

	// Get recent events
	compContext.RecentEvents = a.getRecentEvents(ctx, sessionID)

	// Analyze relationships
	compContext.Relationships = a.analyzeRelationships(compContext)

	// Generate analysis insights
	compContext.AnalysisInsights = a.generateAnalysisInsights(compContext)

	// Generate recommendations
	compContext.Recommendations = a.generateRecommendations(compContext)

	// Apply enrichers
	for _, enricher := range a.contextEnrichers {
		if err := enricher.EnrichContext(ctx, compContext); err != nil {
			a.logger.Error().
				Err(err).
				Str("enricher", enricher.Name()).
				Msg("Context enrichment failed")
		}
	}

	// Add metadata
	compContext.Metadata["aggregation_time_ms"] = time.Since(startTime).Milliseconds()
	compContext.Metadata["provider_count"] = len(compContext.ToolContexts)
	compContext.Metadata["event_count"] = len(compContext.RecentEvents)

	// Cache the result
	a.contextCache.Set(sessionID, compContext)

	a.logger.Info().
		Str("session_id", sessionID).
		Int("providers", len(compContext.ToolContexts)).
		Dur("duration", time.Since(startTime)).
		Msg("Generated comprehensive context")

	return compContext, nil
}

// getStateSnapshot retrieves current state snapshot
func (a *AIContextAggregator) getStateSnapshot(ctx context.Context, sessionID string) (*StateSnapshot, error) {
	snapshot := &StateSnapshot{
		WorkflowStates: make(map[string]interface{}),
		ToolStates:     make(map[string]interface{}),
		GlobalState:    make(map[string]interface{}),
		Timestamp:      time.Now(),
	}

	// Get session state
	sessionState, err := a.stateManager.GetSessionState(ctx, sessionID)
	if err == nil {
		snapshot.SessionState = sessionState
	}

	// Get workflow states
	workflowIDs, _ := a.stateManager.GetState(ctx, state.StateTypeWorkflow, sessionID)
	if workflowList, ok := workflowIDs.([]string); ok {
		for _, wfID := range workflowList {
			if wfState, err := a.stateManager.GetState(ctx, state.StateTypeWorkflow, wfID); err == nil {
				snapshot.WorkflowStates[wfID] = wfState
			}
		}
	}

	// Get tool states
	toolStateIDs, _ := a.stateManager.GetState(ctx, state.StateTypeTool, sessionID)
	if toolList, ok := toolStateIDs.([]string); ok {
		for _, toolID := range toolList {
			if toolState, err := a.stateManager.GetState(ctx, state.StateTypeTool, toolID); err == nil {
				snapshot.ToolStates[toolID] = toolState
			}
		}
	}

	return snapshot, nil
}

// getRecentEvents retrieves recent system events
func (a *AIContextAggregator) getRecentEvents(ctx context.Context, sessionID string) []*Event {
	events := make([]*Event, 0)

	// Get state change events
	stateEvents, err := a.stateManager.GetStateHistory(ctx, state.StateTypeSession, sessionID, 50)
	if err == nil {
		for _, se := range stateEvents {
			event := &Event{
				ID:        se.ID,
				Type:      "state_change",
				Source:    string(se.StateType),
				Timestamp: se.Timestamp,
				Data: map[string]interface{}{
					"state_type": se.StateType,
					"event_type": se.Type,
				},
				Severity: "info",
				Impact:   0.3,
			}
			events = append(events, event)
		}
	}

	return events
}

// analyzeRelationships analyzes relationships between context elements
func (a *AIContextAggregator) analyzeRelationships(ctx *ComprehensiveContext) []*ContextRelationship {
	relationships := make([]*ContextRelationship, 0)

	// Analyze tool dependencies
	for tool1, context1 := range ctx.ToolContexts {
		for tool2, context2 := range ctx.ToolContexts {
			if tool1 != tool2 {
				// Check for data dependencies
				if a.hasDataDependency(context1, context2) {
					relationships = append(relationships, &ContextRelationship{
						Source:      tool1,
						Target:      tool2,
						Type:        "data_dependency",
						Strength:    0.8,
						Description: fmt.Sprintf("%s depends on data from %s", tool1, tool2),
					})
				}
			}
		}
	}

	return relationships
}

// hasDataDependency checks if context1 depends on data from context2
func (a *AIContextAggregator) hasDataDependency(context1, context2 *ContextData) bool {
	// Simple implementation - could be enhanced
	return context1.Timestamp.After(context2.Timestamp) && context1.Relevance > 0.5
}

// generateAnalysisInsights generates analysis insights from context
func (a *AIContextAggregator) generateAnalysisInsights(ctx *ComprehensiveContext) *AnalysisInsights {
	return &AnalysisInsights{
		Patterns:        a.detectPatterns(ctx),
		Anomalies:       a.detectAnomalies(ctx),
		Trends:          a.detectTrends(ctx),
		Correlations:    a.findCorrelations(ctx),
		PredictedIssues: a.predictIssues(ctx),
	}
}

// detectPatterns detects patterns in the context
func (a *AIContextAggregator) detectPatterns(ctx *ComprehensiveContext) []*Pattern {
	patterns := make([]*Pattern, 0)

	// Example: Detect repeated build failures
	if len(ctx.RecentEvents) > 5 {
		failureCount := 0
		for _, event := range ctx.RecentEvents {
			if event.Type == "build_failure" {
				failureCount++
			}
		}

		if failureCount > 3 {
			patterns = append(patterns, &Pattern{
				ID:          fmt.Sprintf("pattern_%d", time.Now().UnixNano()),
				Type:        "repeated_failure",
				Description: "Multiple build failures detected",
				Occurrences: failureCount,
				FirstSeen:   ctx.RecentEvents[0].Timestamp,
				LastSeen:    time.Now(),
				Confidence:  0.9,
			})
		}
	}

	return patterns
}

// detectAnomalies detects anomalies in the context
func (a *AIContextAggregator) detectAnomalies(ctx *ComprehensiveContext) []*Anomaly {
	// Placeholder implementation
	return []*Anomaly{}
}

// detectTrends detects trends in the context
func (a *AIContextAggregator) detectTrends(ctx *ComprehensiveContext) []*Trend {
	// Placeholder implementation
	return []*Trend{}
}

// findCorrelations finds correlations in the context
func (a *AIContextAggregator) findCorrelations(ctx *ComprehensiveContext) []*Correlation {
	// Placeholder implementation
	return []*Correlation{}
}

// predictIssues predicts potential future issues
func (a *AIContextAggregator) predictIssues(ctx *ComprehensiveContext) []*PredictedIssue {
	issues := make([]*PredictedIssue, 0)

	// Example: Predict resource exhaustion
	if ctx.StateSnapshot != nil && ctx.StateSnapshot.SessionState != nil {
		if sessionState, ok := ctx.StateSnapshot.SessionState.(*session.SessionState); ok {
			if sessionState.DiskUsage > 0 && sessionState.MaxDiskUsage > 0 {
				// Check disk space usage
				usagePercent := float64(sessionState.DiskUsage) / float64(sessionState.MaxDiskUsage)
				if usagePercent > 0.8 {
					issues = append(issues, &PredictedIssue{
						ID:            fmt.Sprintf("issue_%d", time.Now().UnixNano()),
						Type:          "resource_exhaustion",
						Description:   "Disk space approaching limit",
						Probability:   usagePercent,
						EstimatedTime: time.Now().Add(2 * time.Hour),
						Impact:        "high",
						Mitigations: []string{
							"Clean up temporary files",
							"Remove old build artifacts",
							"Increase disk quota",
						},
					})
				}
			}
		}
	}

	return issues
}

// generateRecommendations generates recommendations based on context
func (a *AIContextAggregator) generateRecommendations(ctx *ComprehensiveContext) []*Recommendation {
	recommendations := make([]*Recommendation, 0)

	// Generate recommendations based on patterns and issues
	if ctx.AnalysisInsights != nil {
		for _, pattern := range ctx.AnalysisInsights.Patterns {
			if pattern.Type == "repeated_failure" {
				recommendations = append(recommendations, &Recommendation{
					ID:          fmt.Sprintf("rec_%d", time.Now().UnixNano()),
					Type:        "build_optimization",
					Priority:    "high",
					Title:       "Address Repeated Build Failures",
					Description: "Multiple build failures detected. Consider reviewing build configuration and dependencies.",
					Actions: []string{
						"Review recent build logs",
						"Check for dependency conflicts",
						"Validate build configuration",
						"Enable incremental builds",
					},
					Confidence: 0.85,
				})
			}
		}
	}

	return recommendations
}

// GetAIContext implements the mcptypes.AIContextProvider interface
func (a *AIContextAggregator) GetAIContext(ctx context.Context, sessionID string) (mcptypes.AIContext, error) {
	compContext, err := a.GetComprehensiveContext(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Convert to AIContext interface
	return &AIContextAdapter{
		comprehensiveContext: compContext,
	}, nil
}

// AIContextAdapter adapts ComprehensiveContext to mcptypes.AIContext interface
type AIContextAdapter struct {
	comprehensiveContext *ComprehensiveContext
}

// GetContextData returns the context data
func (a *AIContextAdapter) GetContextData() map[string]interface{} {
	return map[string]interface{}{
		"session_id":        a.comprehensiveContext.SessionID,
		"timestamp":         a.comprehensiveContext.Timestamp,
		"tool_contexts":     a.comprehensiveContext.ToolContexts,
		"state_snapshot":    a.comprehensiveContext.StateSnapshot,
		"recent_events":     a.comprehensiveContext.RecentEvents,
		"relationships":     a.comprehensiveContext.Relationships,
		"recommendations":   a.comprehensiveContext.Recommendations,
		"analysis_insights": a.comprehensiveContext.AnalysisInsights,
		"metadata":          a.comprehensiveContext.Metadata,
	}
}

// GetRelevance returns the relevance score
func (a *AIContextAdapter) GetRelevance() float64 {
	// Calculate average relevance from tool contexts
	if len(a.comprehensiveContext.ToolContexts) == 0 {
		return 0.0
	}

	total := 0.0
	for _, ctx := range a.comprehensiveContext.ToolContexts {
		total += ctx.Relevance
	}

	return total / float64(len(a.comprehensiveContext.ToolContexts))
}

// GetConfidence returns the confidence score
func (a *AIContextAdapter) GetConfidence() float64 {
	// Calculate average confidence
	if len(a.comprehensiveContext.ToolContexts) == 0 {
		return 0.0
	}

	total := 0.0
	for _, ctx := range a.comprehensiveContext.ToolContexts {
		total += ctx.Confidence
	}

	return total / float64(len(a.comprehensiveContext.ToolContexts))
}

// GenerateRecommendations generates recommendations based on the current context
func (a *AIContextAdapter) GenerateRecommendations() []mcptypes.Recommendation {
	recommendations := make([]mcptypes.Recommendation, 0)

	if a.comprehensiveContext.Recommendations != nil {
		for range a.comprehensiveContext.Recommendations {
			recommendations = append(recommendations, mcptypes.Recommendation{
				// Note: mcptypes.Recommendation is an empty struct in the current version
				// The actual recommendation data would need to be mapped when the type is properly defined
			})
		}
	}

	return recommendations
}

// GetAssessment returns the unified assessment
func (a *AIContextAdapter) GetAssessment() *mcptypes.UnifiedAssessment {
	// Return an empty assessment for now
	return &mcptypes.UnifiedAssessment{}
}

// GetToolContext returns the tool context
func (a *AIContextAdapter) GetToolContext() *mcptypes.ToolContext {
	// Return an empty tool context for now
	return &mcptypes.ToolContext{}
}

// GetMetadata returns the metadata
func (a *AIContextAdapter) GetMetadata() map[string]interface{} {
	if a.comprehensiveContext.Metadata != nil {
		return a.comprehensiveContext.Metadata
	}
	return make(map[string]interface{})
}
