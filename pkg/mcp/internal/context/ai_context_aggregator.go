package context

import (
	"context"
	"fmt"
	"sync"
	"time"

	mcptypes "github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/state"
	"github.com/rs/zerolog"
)

type AIContextAggregator struct {
	stateManager     *state.UnifiedStateManager
	sessionManager   *session.SessionManager
	contextProviders map[string]ContextProvider
	contextEnrichers []ContextEnricher
	contextCache     *ContextCache
	mu               sync.RWMutex
	logger           zerolog.Logger
}

type ContextProvider interface {
	GetContextData(ctx context.Context, request *ContextRequest) (*ContextData, error)
	GetCapabilities() *ContextProviderCapabilities
}

type ContextEnricher interface {
	EnrichContext(ctx context.Context, data *ComprehensiveContext) error
	Name() string
}

type ContextRequest struct {
	SessionID      string
	ToolName       string
	ContextType    ContextType
	TimeRange      *TimeRange
	IncludeHistory bool
	MaxItems       int
	Filters        map[string]interface{}
}

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

type TimeRange struct {
	Start time.Time
	End   time.Time
}

type ContextData struct {
	Provider   string                 `json:"provider"`
	Type       ContextType            `json:"type"`
	Timestamp  time.Time              `json:"timestamp"`
	Data       map[string]interface{} `json:"data"`
	Metadata   map[string]interface{} `json:"metadata"`
	Relevance  float64                `json:"relevance"`
	Confidence float64                `json:"confidence"`
}

type ContextProviderCapabilities struct {
	SupportedTypes  []ContextType
	SupportsHistory bool
	MaxHistoryDays  int
	RealTimeUpdates bool
}

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

type StateSnapshot struct {
	SessionState   interface{}            `json:"session_state"`
	WorkflowStates map[string]interface{} `json:"workflow_states"`
	ToolStates     map[string]interface{} `json:"tool_states"`
	GlobalState    map[string]interface{} `json:"global_state"`
	Timestamp      time.Time              `json:"timestamp"`
}

type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Severity  string                 `json:"severity"`
	Impact    float64                `json:"impact"`
}

type ContextRelationship struct {
	Source      string  `json:"source"`
	Target      string  `json:"target"`
	Type        string  `json:"type"`
	Strength    float64 `json:"strength"`
	Description string  `json:"description"`
}

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

type AnalysisInsights struct {
	Patterns        []*Pattern        `json:"patterns"`
	Anomalies       []*Anomaly        `json:"anomalies"`
	Trends          []*Trend          `json:"trends"`
	Correlations    []*Correlation    `json:"correlations"`
	PredictedIssues []*PredictedIssue `json:"predicted_issues"`
}

type Pattern struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Occurrences int       `json:"occurrences"`
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	Confidence  float64   `json:"confidence"`
}

type Anomaly struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Severity    string                 `json:"severity"`
	DetectedAt  time.Time              `json:"detected_at"`
	Data        map[string]interface{} `json:"data"`
	Confidence  float64                `json:"confidence"`
}

type Trend struct {
	ID         string    `json:"id"`
	Metric     string    `json:"metric"`
	Direction  string    `json:"direction"`
	Rate       float64   `json:"rate"`
	StartTime  time.Time `json:"start_time"`
	Confidence float64   `json:"confidence"`
}

type Correlation struct {
	Metric1     string  `json:"metric1"`
	Metric2     string  `json:"metric2"`
	Coefficient float64 `json:"coefficient"`
	Confidence  float64 `json:"confidence"`
}

type PredictedIssue struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"`
	Description   string    `json:"description"`
	Probability   float64   `json:"probability"`
	EstimatedTime time.Time `json:"estimated_time"`
	Impact        string    `json:"impact"`
	Mitigations   []string  `json:"mitigations"`
}

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

func (a *AIContextAggregator) RegisterContextProvider(name string, provider ContextProvider) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.contextProviders[name] = provider
	a.logger.Info().Str("provider", name).Msg("Registered context provider")
}

func (a *AIContextAggregator) RegisterContextEnricher(enricher ContextEnricher) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.contextEnrichers = append(a.contextEnrichers, enricher)
	a.logger.Info().Str("enricher", enricher.Name()).Msg("Registered context enricher")
}

func (a *AIContextAggregator) GetComprehensiveContext(ctx context.Context, sessionID string) (*ComprehensiveContext, error) {
	if cached := a.contextCache.Get(sessionID); cached != nil {
		return cached, nil
	}

	startTime := time.Now()

	compContext := &ComprehensiveContext{
		SessionID:     sessionID,
		Timestamp:     time.Now(),
		RequestID:     fmt.Sprintf("ctx_%d", time.Now().UnixNano()),
		ToolContexts:  make(map[string]*ContextData),
		RecentEvents:  make([]*Event, 0),
		Relationships: make([]*ContextRelationship, 0),
		Metadata:      make(map[string]interface{}),
	}

	snapshot, err := a.getStateSnapshot(ctx, sessionID)
	if err != nil {
		a.logger.Error().Err(err).Msg("Failed to get state snapshot")
	} else {
		compContext.StateSnapshot = snapshot
	}

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

	go func() {
		wg.Wait()
		close(contextChan)
	}()

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

	compContext.RecentEvents = a.getRecentEvents(ctx, sessionID)

	compContext.Relationships = a.analyzeRelationships(compContext)

	compContext.AnalysisInsights = a.generateAnalysisInsights(compContext)

	compContext.Recommendations = a.generateRecommendations(compContext)

	for _, enricher := range a.contextEnrichers {
		if err := enricher.EnrichContext(ctx, compContext); err != nil {
			a.logger.Error().
				Err(err).
				Str("enricher", enricher.Name()).
				Msg("Context enrichment failed")
		}
	}

	compContext.Metadata["aggregation_time_ms"] = time.Since(startTime).Milliseconds()
	compContext.Metadata["provider_count"] = len(compContext.ToolContexts)
	compContext.Metadata["event_count"] = len(compContext.RecentEvents)

	a.contextCache.Set(sessionID, compContext)

	a.logger.Info().
		Str("session_id", sessionID).
		Int("providers", len(compContext.ToolContexts)).
		Dur("duration", time.Since(startTime)).
		Msg("Generated comprehensive context")

	return compContext, nil
}

func (a *AIContextAggregator) getStateSnapshot(ctx context.Context, sessionID string) (*StateSnapshot, error) {
	snapshot := &StateSnapshot{
		WorkflowStates: make(map[string]interface{}),
		ToolStates:     make(map[string]interface{}),
		GlobalState:    make(map[string]interface{}),
		Timestamp:      time.Now(),
	}

	sessionState, err := a.stateManager.GetSessionState(ctx, sessionID)
	if err == nil {
		snapshot.SessionState = sessionState
	}

	workflowIDs, _ := a.stateManager.GetState(ctx, state.StateTypeWorkflow, sessionID)
	if workflowList, ok := workflowIDs.([]string); ok {
		for _, wfID := range workflowList {
			if wfState, err := a.stateManager.GetState(ctx, state.StateTypeWorkflow, wfID); err == nil {
				snapshot.WorkflowStates[wfID] = wfState
			}
		}
	}

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

func (a *AIContextAggregator) getRecentEvents(ctx context.Context, sessionID string) []*Event {
	events := make([]*Event, 0)

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

func (a *AIContextAggregator) analyzeRelationships(ctx *ComprehensiveContext) []*ContextRelationship {
	relationships := make([]*ContextRelationship, 0)

	for tool1, context1 := range ctx.ToolContexts {
		for tool2, context2 := range ctx.ToolContexts {
			if tool1 != tool2 {
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

func (a *AIContextAggregator) hasDataDependency(context1, context2 *ContextData) bool {
	return context1.Timestamp.After(context2.Timestamp) && context1.Relevance > 0.5
}

func (a *AIContextAggregator) generateAnalysisInsights(ctx *ComprehensiveContext) *AnalysisInsights {
	return &AnalysisInsights{
		Patterns:        a.detectPatterns(ctx),
		Anomalies:       a.detectAnomalies(ctx),
		Trends:          a.detectTrends(ctx),
		Correlations:    a.findCorrelations(ctx),
		PredictedIssues: a.predictIssues(ctx),
	}
}

func (a *AIContextAggregator) detectPatterns(ctx *ComprehensiveContext) []*Pattern {
	patterns := make([]*Pattern, 0)

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

func (a *AIContextAggregator) detectAnomalies(ctx *ComprehensiveContext) []*Anomaly {
	return []*Anomaly{}
}

func (a *AIContextAggregator) detectTrends(ctx *ComprehensiveContext) []*Trend {
	return []*Trend{}
}

func (a *AIContextAggregator) findCorrelations(ctx *ComprehensiveContext) []*Correlation {
	return []*Correlation{}
}

func (a *AIContextAggregator) predictIssues(ctx *ComprehensiveContext) []*PredictedIssue {
	issues := make([]*PredictedIssue, 0)

	if ctx.StateSnapshot != nil && ctx.StateSnapshot.SessionState != nil {
		if sessionState, ok := ctx.StateSnapshot.SessionState.(*session.SessionState); ok {
			if sessionState.DiskUsage > 0 && sessionState.MaxDiskUsage > 0 {
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

func (a *AIContextAggregator) generateRecommendations(ctx *ComprehensiveContext) []*Recommendation {
	recommendations := make([]*Recommendation, 0)

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

func (a *AIContextAggregator) GetAIContext(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	compContext, err := a.GetComprehensiveContext(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	adapter := &AIContextAdapter{
		comprehensiveContext: compContext,
	}
	return adapter.GetContextData(), nil
}

type AIContextAdapter struct {
	comprehensiveContext *ComprehensiveContext
}

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

func (a *AIContextAdapter) GetRelevance() float64 {
	if len(a.comprehensiveContext.ToolContexts) == 0 {
		return 0.0
	}

	total := 0.0
	for _, ctx := range a.comprehensiveContext.ToolContexts {
		total += ctx.Relevance
	}

	return total / float64(len(a.comprehensiveContext.ToolContexts))
}

func (a *AIContextAdapter) GetConfidence() float64 {
	if len(a.comprehensiveContext.ToolContexts) == 0 {
		return 0.0
	}

	total := 0.0
	for _, ctx := range a.comprehensiveContext.ToolContexts {
		total += ctx.Confidence
	}

	return total / float64(len(a.comprehensiveContext.ToolContexts))
}

func (a *AIContextAdapter) GenerateRecommendations() []mcptypes.Recommendation {
	recommendations := make([]mcptypes.Recommendation, 0)

	if a.comprehensiveContext.Recommendations != nil {
		for range a.comprehensiveContext.Recommendations {
			recommendations = append(recommendations, mcptypes.Recommendation{})
		}
	}

	return recommendations
}

func (a *AIContextAdapter) GetAssessment() map[string]interface{} {
	return map[string]interface{}{}
}

func (a *AIContextAdapter) GetToolContext() map[string]interface{} {
	return map[string]interface{}{}
}

func (a *AIContextAdapter) GetMetadata() map[string]interface{} {
	if a.comprehensiveContext.Metadata != nil {
		return a.comprehensiveContext.Metadata
	}
	return make(map[string]interface{})
}
