package context

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/build"
	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/Azure/container-kit/pkg/mcp/internal/state"
	"github.com/rs/zerolog"
)

// BuildContextProvider provides build-related context
type BuildContextProvider struct {
	stateManager   *state.UnifiedStateManager
	sessionManager *session.SessionManager
	knowledgeBase  *build.CrossToolKnowledgeBase
	logger         zerolog.Logger
}

// NewBuildContextProvider creates a new build context provider
func NewBuildContextProvider(
	stateManager *state.UnifiedStateManager,
	sessionManager *session.SessionManager,
	knowledgeBase *build.CrossToolKnowledgeBase,
	logger zerolog.Logger,
) ContextProvider {
	return &BuildContextProvider{
		stateManager:   stateManager,
		sessionManager: sessionManager,
		knowledgeBase:  knowledgeBase,
		logger:         logger.With().Str("provider", "build_context").Logger(),
	}
}

// GetContextData retrieves build context data
func (p *BuildContextProvider) GetContextData(ctx context.Context, request *ContextRequest) (*ContextData, error) {
	data := &ContextData{
		Provider:   "build",
		Type:       ContextTypeBuild,
		Timestamp:  time.Now(),
		Data:       make(map[string]interface{}),
		Metadata:   make(map[string]interface{}),
		Relevance:  0.8,
		Confidence: 0.9,
	}

	// Get session state
	sessionState, err := p.sessionManager.GetSession(request.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session state: %w", err)
	}

	// Extract build-related data
	// Note: sessionState is interface{}, need to handle accordingly
	if ss, ok := sessionState.(*session.SessionState); ok {
		if ss.Dockerfile.Content != "" {
			data.Data["docker_build"] = map[string]interface{}{
				"dockerfile_generated": true,
				"dockerfile_path":      ss.Dockerfile.Path,
				"image_built":          ss.Dockerfile.Built,
				"image_ref":            ss.ImageRef.String(),
			}
		}
	}

	// Get build insights from knowledge base
	if p.knowledgeBase != nil {
		buildRequest := &build.AnalysisRequest{
			Error: fmt.Errorf("build analysis request"),
		}

		relatedFailures, err := p.knowledgeBase.GetRelatedFailures(ctx, buildRequest)
		if err == nil && len(relatedFailures) > 0 {
			data.Data["related_failures"] = relatedFailures
			data.Metadata["failure_count"] = len(relatedFailures)
		}
	}

	// Get recent build events
	events, err := p.stateManager.GetStateHistory(ctx, state.StateTypeTool, "docker_build", 20)
	if err == nil {
		recentBuilds := make([]map[string]interface{}, 0)
		for _, event := range events {
			if event.Type == state.StateEventUpdated {
				recentBuilds = append(recentBuilds, map[string]interface{}{
					"timestamp": event.Timestamp,
					"metadata":  event.Metadata,
				})
			}
		}
		data.Data["recent_builds"] = recentBuilds
	}

	return data, nil
}

// GetCapabilities returns provider capabilities
func (p *BuildContextProvider) GetCapabilities() *ContextProviderCapabilities {
	return &ContextProviderCapabilities{
		SupportedTypes:  []ContextType{ContextTypeBuild, ContextTypeAnalysis},
		SupportsHistory: true,
		MaxHistoryDays:  7,
		RealTimeUpdates: false,
	}
}

// DeploymentContextProvider provides deployment-related context
type DeploymentContextProvider struct {
	stateManager   *state.UnifiedStateManager
	sessionManager *session.SessionManager
	logger         zerolog.Logger
}

// NewDeploymentContextProvider creates a new deployment context provider
func NewDeploymentContextProvider(
	stateManager *state.UnifiedStateManager,
	sessionManager *session.SessionManager,
	logger zerolog.Logger,
) ContextProvider {
	return &DeploymentContextProvider{
		stateManager:   stateManager,
		sessionManager: sessionManager,
		logger:         logger.With().Str("provider", "deployment_context").Logger(),
	}
}

// GetContextData retrieves deployment context data
func (p *DeploymentContextProvider) GetContextData(ctx context.Context, request *ContextRequest) (*ContextData, error) {
	data := &ContextData{
		Provider:   "deployment",
		Type:       ContextTypeDeployment,
		Timestamp:  time.Now(),
		Data:       make(map[string]interface{}),
		Metadata:   make(map[string]interface{}),
		Relevance:  0.7,
		Confidence: 0.85,
	}

	// Get session state
	sessionState, err := p.sessionManager.GetSession(request.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session state: %w", err)
	}

	// Extract deployment-related data
	if ss, ok := sessionState.(*session.SessionState); ok {
		if len(ss.K8sManifests) > 0 {
			manifestNames := make([]string, 0, len(ss.K8sManifests))
			for name := range ss.K8sManifests {
				manifestNames = append(manifestNames, name)
			}
			data.Data["kubernetes"] = map[string]interface{}{
				"manifests_count": len(ss.K8sManifests),
				"namespaces":      p.extractNamespaces(manifestNames),
				"resource_types":  p.extractResourceTypes(manifestNames),
			}
		}
	}

	// Get deployment history
	events, err := p.stateManager.GetStateHistory(ctx, state.StateTypeTool, "k8s_deploy", 10)
	if err == nil {
		deployments := make([]map[string]interface{}, 0)
		for _, event := range events {
			if event.Type == state.StateEventUpdated {
				deployments = append(deployments, map[string]interface{}{
					"timestamp": event.Timestamp,
					"metadata":  event.Metadata,
				})
			}
		}
		data.Data["recent_deployments"] = deployments
	}

	return data, nil
}

// GetCapabilities returns provider capabilities
func (p *DeploymentContextProvider) GetCapabilities() *ContextProviderCapabilities {
	return &ContextProviderCapabilities{
		SupportedTypes:  []ContextType{ContextTypeDeployment},
		SupportsHistory: true,
		MaxHistoryDays:  30,
		RealTimeUpdates: false,
	}
}

// extractNamespaces extracts unique namespaces from manifests
func (p *DeploymentContextProvider) extractNamespaces(manifests []string) []string {
	// Implementation would parse manifests and extract namespaces
	return []string{"default"} // Placeholder
}

// extractResourceTypes extracts resource types from manifests
func (p *DeploymentContextProvider) extractResourceTypes(manifests []string) []string {
	// Implementation would parse manifests and extract resource types
	return []string{"Deployment", "Service", "ConfigMap"} // Placeholder
}

// SecurityContextProvider provides security-related context
type SecurityContextProvider struct {
	stateManager   *state.UnifiedStateManager
	sessionManager *session.SessionManager
	logger         zerolog.Logger
}

// NewSecurityContextProvider creates a new security context provider
func NewSecurityContextProvider(
	stateManager *state.UnifiedStateManager,
	sessionManager *session.SessionManager,
	logger zerolog.Logger,
) ContextProvider {
	return &SecurityContextProvider{
		stateManager:   stateManager,
		sessionManager: sessionManager,
		logger:         logger.With().Str("provider", "security_context").Logger(),
	}
}

// GetContextData retrieves security context data
func (p *SecurityContextProvider) GetContextData(ctx context.Context, request *ContextRequest) (*ContextData, error) {
	data := &ContextData{
		Provider:   "security",
		Type:       ContextTypeSecurity,
		Timestamp:  time.Now(),
		Data:       make(map[string]interface{}),
		Metadata:   make(map[string]interface{}),
		Relevance:  0.9,
		Confidence: 0.8,
	}

	// Get session state
	sessionState, err := p.sessionManager.GetSession(request.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session state: %w", err)
	}

	// Extract security scan results
	if ss, ok := sessionState.(*session.SessionState); ok {
		if ss.SecurityScan != nil {
			data.Data["security_scans"] = map[string]interface{}{
				"success":         ss.SecurityScan.Success,
				"scanned_at":      ss.SecurityScan.ScannedAt,
				"scanner":         ss.SecurityScan.Scanner,
				"critical_issues": ss.SecurityScan.Summary.Critical,
				"high_issues":     ss.SecurityScan.Summary.High,
				"total_issues":    ss.SecurityScan.Summary.Total,
				"fixable_count":   ss.SecurityScan.Fixable,
			}

			// Set relevance based on severity
			if ss.SecurityScan.Summary.Critical > 0 {
				data.Relevance = 1.0
			} else if ss.SecurityScan.Summary.High > 0 {
				data.Relevance = 0.9
			}
		}
	}

	return data, nil
}

// GetCapabilities returns provider capabilities
func (p *SecurityContextProvider) GetCapabilities() *ContextProviderCapabilities {
	return &ContextProviderCapabilities{
		SupportedTypes:  []ContextType{ContextTypeSecurity},
		SupportsHistory: true,
		MaxHistoryDays:  90,
		RealTimeUpdates: false,
	}
}

// PerformanceContextProvider provides performance-related context
type PerformanceContextProvider struct {
	stateManager     *state.UnifiedStateManager
	sessionManager   *session.SessionManager
	metricsCollector *MetricsCollector
	logger           zerolog.Logger
}

// MetricsCollector collects performance metrics
type MetricsCollector struct {
	metrics map[string]*PerformanceMetrics
	mu      sync.RWMutex
}

// PerformanceMetrics represents performance metrics
type PerformanceMetrics struct {
	CPU         float64
	Memory      float64
	Disk        float64
	Network     float64
	Latency     time.Duration
	Throughput  float64
	ErrorRate   float64
	LastUpdated time.Time
}

// NewPerformanceContextProvider creates a new performance context provider
func NewPerformanceContextProvider(
	stateManager *state.UnifiedStateManager,
	sessionManager *session.SessionManager,
	logger zerolog.Logger,
) ContextProvider {
	return &PerformanceContextProvider{
		stateManager:     stateManager,
		sessionManager:   sessionManager,
		metricsCollector: &MetricsCollector{metrics: make(map[string]*PerformanceMetrics)},
		logger:           logger.With().Str("provider", "performance_context").Logger(),
	}
}

// GetContextData retrieves performance context data
func (p *PerformanceContextProvider) GetContextData(ctx context.Context, request *ContextRequest) (*ContextData, error) {
	data := &ContextData{
		Provider:   "performance",
		Type:       ContextTypePerformance,
		Timestamp:  time.Now(),
		Data:       make(map[string]interface{}),
		Metadata:   make(map[string]interface{}),
		Relevance:  0.7,
		Confidence: 0.9,
	}

	// Get session state for resource usage
	sessionState, err := p.sessionManager.GetSession(request.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session state: %w", err)
	}

	// Extract resource usage
	if ss, ok := sessionState.(*session.SessionState); ok {
		data.Data["resource_usage"] = map[string]interface{}{
			"token_usage":      ss.TokenUsage,
			"disk_space_bytes": ss.DiskUsage,
			"max_disk_usage":   ss.MaxDiskUsage,
			"jobs_active":      ss.GetActiveJobCount(),
		}

		// Calculate usage percentages
		if ss.MaxDiskUsage > 0 {
			diskUsagePercent := float64(ss.DiskUsage) / float64(ss.MaxDiskUsage)
			data.Data["usage_percentages"] = map[string]float64{
				"disk_usage": diskUsagePercent,
			}

			// Increase relevance if approaching limits
			if diskUsagePercent > 0.8 {
				data.Relevance = 0.95
			}
		}
	}

	// Get performance metrics from collector
	p.metricsCollector.mu.RLock()
	if metrics, exists := p.metricsCollector.metrics[request.SessionID]; exists {
		data.Data["performance_metrics"] = map[string]interface{}{
			"cpu_usage":     metrics.CPU,
			"memory_usage":  metrics.Memory,
			"disk_usage":    metrics.Disk,
			"network_usage": metrics.Network,
			"latency_ms":    metrics.Latency.Milliseconds(),
			"throughput":    metrics.Throughput,
			"error_rate":    metrics.ErrorRate,
			"last_updated":  metrics.LastUpdated,
		}
	}
	p.metricsCollector.mu.RUnlock()

	return data, nil
}

// GetCapabilities returns provider capabilities
func (p *PerformanceContextProvider) GetCapabilities() *ContextProviderCapabilities {
	return &ContextProviderCapabilities{
		SupportedTypes:  []ContextType{ContextTypePerformance},
		SupportsHistory: true,
		MaxHistoryDays:  1,
		RealTimeUpdates: true,
	}
}

// StateContextProvider provides state-related context
type StateContextProvider struct {
	stateManager *state.UnifiedStateManager
	logger       zerolog.Logger
}

// NewStateContextProvider creates a new state context provider
func NewStateContextProvider(
	stateManager *state.UnifiedStateManager,
	logger zerolog.Logger,
) ContextProvider {
	return &StateContextProvider{
		stateManager: stateManager,
		logger:       logger.With().Str("provider", "state_context").Logger(),
	}
}

// GetContextData retrieves state context data
func (p *StateContextProvider) GetContextData(ctx context.Context, request *ContextRequest) (*ContextData, error) {
	data := &ContextData{
		Provider:   "state",
		Type:       ContextTypeState,
		Timestamp:  time.Now(),
		Data:       make(map[string]interface{}),
		Metadata:   make(map[string]interface{}),
		Relevance:  0.6,
		Confidence: 1.0,
	}

	// Get state metrics
	stateIntegration := state.NewStateManagementIntegration(nil, nil, p.logger)
	metrics := stateIntegration.GetStateMetrics()

	metricsData := make(map[string]interface{})
	for stateType, m := range metrics {
		metricsData[stateType] = map[string]interface{}{
			"total_changes": m.TotalChanges,
			"create_count":  m.CreateCount,
			"update_count":  m.UpdateCount,
			"delete_count":  m.DeleteCount,
			"change_rate":   m.ChangeRate,
			"last_change":   m.LastChangeTime,
		}
	}
	data.Data["state_metrics"] = metricsData

	// Get recent state changes
	recentChanges := make([]map[string]interface{}, 0)
	for _, stateType := range []state.StateType{
		state.StateTypeSession,
		state.StateTypeWorkflow,
		state.StateTypeTool,
	} {
		events, err := p.stateManager.GetStateHistory(ctx, stateType, request.SessionID, 5)
		if err == nil {
			for _, event := range events {
				recentChanges = append(recentChanges, map[string]interface{}{
					"state_type": event.StateType,
					"event_type": event.Type,
					"timestamp":  event.Timestamp,
					"metadata":   event.Metadata,
				})
			}
		}
	}
	data.Data["recent_changes"] = recentChanges

	return data, nil
}

// GetCapabilities returns provider capabilities
func (p *StateContextProvider) GetCapabilities() *ContextProviderCapabilities {
	return &ContextProviderCapabilities{
		SupportedTypes:  []ContextType{ContextTypeState, ContextTypeAll},
		SupportsHistory: true,
		MaxHistoryDays:  7,
		RealTimeUpdates: true,
	}
}
