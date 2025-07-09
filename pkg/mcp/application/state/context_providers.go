package state

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/knowledge"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors/codes"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
)

// BuildContextProvider provides build-related context
type BuildContextProvider struct {
	stateManager   *UnifiedStateManager
	sessionManager *session.SessionManager
	knowledgeBase  *knowledge.CrossToolKnowledgeBase
	logger         *slog.Logger
}

// NewBuildContextProvider creates a new build context provider
func NewBuildContextProvider(
	stateManager *UnifiedStateManager,
	sessionManager *session.SessionManager,
	knowledgeBase *knowledge.CrossToolKnowledgeBase,
	logger *slog.Logger,
) ContextProvider {
	return &BuildContextProvider{
		stateManager:   stateManager,
		sessionManager: sessionManager,
		knowledgeBase:  knowledgeBase,
		logger:         logger.With(slog.String("provider", "build_context")),
	}
}

// GetContext retrieves build context data
func (p *BuildContextProvider) GetContext(ctx context.Context, request *ContextRequest) (*ContextData, error) {
	p.logger.Debug("Getting build context",
		slog.String("session_id", request.SessionID),
		slog.String("request_type", string(request.Type)))

	data := &ContextData{
		Provider:   "build",
		Type:       ContextTypeBuild,
		Timestamp:  time.Now(),
		Data:       make(map[string]interface{}),
		Metadata:   make(map[string]interface{}),
		Relevance:  0.8,
		Confidence: 0.9,
	}

	sessionState, err := p.sessionManager.GetSessionConcrete(request.SessionID)
	if err != nil {
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
			"Failed to get session state",
			err,
		)
		systemErr.Context["session_id"] = request.SessionID
		systemErr.Context["component"] = "build_context_provider"
		systemErr.Suggestions = append(systemErr.Suggestions, "Check session ID and session manager availability")
		return nil, systemErr
	}

	if sessionState != nil {
		if sessionState.Dockerfile.Content != "" {
			data.Data["docker_build"] = map[string]interface{}{
				"dockerfile_generated": true,
				"dockerfile_path":      sessionState.Dockerfile.Path,
				"image_built":          sessionState.Dockerfile.Built,
				"image_ref":            sessionState.ImageRef.String(),
			}
		}
	}

	if p.knowledgeBase != nil {
		buildRequest := &knowledge.AnalysisRequest{
			Error: errors.NewError().Messagef("build analysis request").Build(),
		}

		relatedFailures, err := p.knowledgeBase.GetRelatedFailures(ctx, buildRequest)
		if err == nil && len(relatedFailures) > 0 {
			data.Data["related_failures"] = relatedFailures
			data.Metadata["failure_count"] = len(relatedFailures)
		}
	}

	events, err := p.stateManager.GetStateHistory(ctx, StateTypeTool, "docker_build", 20)
	if err == nil {
		recentBuilds := make([]map[string]interface{}, 0)
		for _, event := range events {
			if event.Type == StateEventUpdated {
				recentBuilds = append(recentBuilds, map[string]interface{}{
					"timestamp": event.Timestamp,
					"metadata":  event.Metadata,
				})
			}
		}
		data.Data["recent_builds"] = recentBuilds
	}

	p.logger.Info("Build context retrieved successfully",
		slog.String("session_id", request.SessionID),
		slog.Int("data_keys", len(data.Data)))

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

// GetName returns the provider name
func (p *BuildContextProvider) GetName() string {
	return "build"
}

// DeploymentContextProvider provides deployment-related context
type DeploymentContextProvider struct {
	stateManager   *UnifiedStateManager
	sessionManager *session.SessionManager
	logger         *slog.Logger
}

// NewDeploymentContextProvider creates a new deployment context provider
func NewDeploymentContextProvider(
	stateManager *UnifiedStateManager,
	sessionManager *session.SessionManager,
	logger *slog.Logger,
) ContextProvider {
	return &DeploymentContextProvider{
		stateManager:   stateManager,
		sessionManager: sessionManager,
		logger:         logger.With(slog.String("provider", "deployment_context")),
	}
}

// GetContext retrieves deployment context data
func (p *DeploymentContextProvider) GetContext(ctx context.Context, request *ContextRequest) (*ContextData, error) {
	p.logger.Debug("Getting deployment context",
		slog.String("session_id", request.SessionID),
		slog.String("request_type", string(request.Type)))

	data := &ContextData{
		Provider:   "deployment",
		Type:       ContextTypeDeployment,
		Timestamp:  time.Now(),
		Data:       make(map[string]interface{}),
		Metadata:   make(map[string]interface{}),
		Relevance:  0.7,
		Confidence: 0.85,
	}

	sessionState, err := p.sessionManager.GetSessionConcrete(request.SessionID)
	if err != nil {
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
			"Failed to get session state",
			err,
		)
		systemErr.Context["session_id"] = request.SessionID
		systemErr.Context["component"] = "build_context_provider"
		systemErr.Suggestions = append(systemErr.Suggestions, "Check session ID and session manager availability")
		return nil, systemErr
	}

	if sessionState != nil {
		if len(sessionState.K8sManifests) > 0 {
			manifestNames := make([]string, 0, len(sessionState.K8sManifests))
			for name := range sessionState.K8sManifests {
				manifestNames = append(manifestNames, name)
			}
			data.Data["kubernetes"] = map[string]interface{}{
				"manifests_count": len(sessionState.K8sManifests),
				"namespaces":      p.extractNamespaces(manifestNames),
				"resource_types":  p.extractResourceTypes(manifestNames),
			}
		}
	}

	events, err := p.stateManager.GetStateHistory(ctx, StateTypeTool, "k8s_deploy", 10)
	if err == nil {
		deployments := make([]map[string]interface{}, 0)
		for _, event := range events {
			if event.Type == StateEventUpdated {
				deployments = append(deployments, map[string]interface{}{
					"timestamp": event.Timestamp,
					"metadata":  event.Metadata,
				})
			}
		}
		data.Data["recent_deployments"] = deployments
	}

	p.logger.Info("Deployment context retrieved successfully",
		slog.String("session_id", request.SessionID),
		slog.Int("manifest_count", 0))

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

// GetName returns the provider name
func (p *DeploymentContextProvider) GetName() string {
	return "deployment"
}

// extractNamespaces extracts unique namespaces from manifests
func (p *DeploymentContextProvider) extractNamespaces(manifests []string) []string {
	return []string{"default"}
}

// extractResourceTypes extracts resource types from manifests
func (p *DeploymentContextProvider) extractResourceTypes(manifests []string) []string {
	return []string{"Deployment", "Service", "ConfigMap"}
}

// SecurityContextProvider provides security-related context
type SecurityContextProvider struct {
	stateManager   *UnifiedStateManager
	sessionManager *session.SessionManager
	logger         *slog.Logger
}

// NewSecurityContextProvider creates a new security context provider
func NewSecurityContextProvider(
	stateManager *UnifiedStateManager,
	sessionManager *session.SessionManager,
	logger *slog.Logger,
) ContextProvider {
	return &SecurityContextProvider{
		stateManager:   stateManager,
		sessionManager: sessionManager,
		logger:         logger.With(slog.String("provider", "security_context")),
	}
}

// GetContext retrieves security context data
func (p *SecurityContextProvider) GetContext(ctx context.Context, request *ContextRequest) (*ContextData, error) {
	p.logger.Debug("Getting security context",
		slog.String("session_id", request.SessionID),
		slog.String("request_type", string(request.Type)))

	data := &ContextData{
		Provider:   "security",
		Type:       ContextTypeSecurity,
		Timestamp:  time.Now(),
		Data:       make(map[string]interface{}),
		Metadata:   make(map[string]interface{}),
		Relevance:  0.9,
		Confidence: 0.8,
	}

	sessionState, err := p.sessionManager.GetSessionConcrete(request.SessionID)
	if err != nil {
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
			"Failed to get session state",
			err,
		)
		systemErr.Context["session_id"] = request.SessionID
		systemErr.Context["component"] = "build_context_provider"
		systemErr.Suggestions = append(systemErr.Suggestions, "Check session ID and session manager availability")
		return nil, systemErr
	}

	if sessionState != nil {
		if sessionState.SecurityScan != nil {
			data.Data["security_scans"] = map[string]interface{}{
				"success":         sessionState.SecurityScan.Success,
				"scanned_at":      sessionState.SecurityScan.ScannedAt,
				"scanner":         sessionState.SecurityScan.Scanner,
				"critical_issues": sessionState.SecurityScan.Summary.Critical,
				"high_issues":     sessionState.SecurityScan.Summary.High,
				"total_issues":    sessionState.SecurityScan.Summary.Total,
				"fixable_count":   sessionState.SecurityScan.Fixable,
			}

			if sessionState.SecurityScan.Summary.Critical > 0 {
				data.Relevance = 1.0
			} else if sessionState.SecurityScan.Summary.High > 0 {
				data.Relevance = 0.9
			}
		}
	}

	p.logger.Info("Security context retrieved successfully",
		slog.String("session_id", request.SessionID),
		slog.Float64("relevance", data.Relevance))

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

// GetName returns the provider name
func (p *SecurityContextProvider) GetName() string {
	return "security"
}

// PerformanceContextProvider provides performance-related context
type PerformanceContextProvider struct {
	stateManager     *UnifiedStateManager
	sessionManager   *session.SessionManager
	metricsCollector *MetricsCollector
	logger           *slog.Logger
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
	stateManager *UnifiedStateManager,
	sessionManager *session.SessionManager,
	logger *slog.Logger,
) ContextProvider {
	return &PerformanceContextProvider{
		stateManager:     stateManager,
		sessionManager:   sessionManager,
		metricsCollector: &MetricsCollector{metrics: make(map[string]*PerformanceMetrics)},
		logger:           logger.With(slog.String("provider", "performance_context")),
	}
}

// GetContext retrieves performance context data
func (p *PerformanceContextProvider) GetContext(ctx context.Context, request *ContextRequest) (*ContextData, error) {
	p.logger.Debug("Getting performance context",
		slog.String("session_id", request.SessionID),
		slog.String("request_type", string(request.Type)))

	data := &ContextData{
		Provider:   "performance",
		Type:       ContextTypePerformance,
		Timestamp:  time.Now(),
		Data:       make(map[string]interface{}),
		Metadata:   make(map[string]interface{}),
		Relevance:  0.7,
		Confidence: 0.9,
	}

	sessionState, err := p.sessionManager.GetSessionConcrete(request.SessionID)
	if err != nil {
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
			"Failed to get session state",
			err,
		)
		systemErr.Context["session_id"] = request.SessionID
		systemErr.Context["component"] = "build_context_provider"
		systemErr.Suggestions = append(systemErr.Suggestions, "Check session ID and session manager availability")
		return nil, systemErr
	}

	if sessionState != nil {
		data.Data["resource_usage"] = map[string]interface{}{
			"token_usage":      sessionState.TokenUsage,
			"disk_space_bytes": sessionState.DiskUsage,
			"max_disk_usage":   sessionState.MaxDiskUsage,
			"jobs_active":      sessionState.GetActiveJobCount(),
		}

		if sessionState.MaxDiskUsage > 0 {
			diskUsagePercent := float64(sessionState.DiskUsage) / float64(sessionState.MaxDiskUsage)
			data.Data["usage_percentages"] = map[string]float64{
				"disk_usage": diskUsagePercent,
			}

			if diskUsagePercent > 0.8 {
				data.Relevance = 0.95
			}
		}
	}

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

	p.logger.Info("Performance context retrieved successfully",
		slog.String("session_id", request.SessionID),
		slog.Float64("relevance", data.Relevance))

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

// GetName returns the provider name
func (p *PerformanceContextProvider) GetName() string {
	return "performance"
}

// StateContextProvider provides state-related context
type StateContextProvider struct {
	stateManager *UnifiedStateManager
	logger       *slog.Logger
}

// NewStateContextProvider creates a new state context provider
func NewStateContextProvider(
	stateManager *UnifiedStateManager,
	logger *slog.Logger,
) ContextProvider {
	return &StateContextProvider{
		stateManager: stateManager,
		logger:       logger.With(slog.String("provider", "state_context")),
	}
}

// GetContext retrieves state context data
func (p *StateContextProvider) GetContext(ctx context.Context, request *ContextRequest) (*ContextData, error) {
	p.logger.Debug("Getting state context",
		slog.String("session_id", request.SessionID),
		slog.String("request_type", string(request.Type)))

	data := &ContextData{
		Provider:   "state",
		Type:       ContextTypeState,
		Timestamp:  time.Now(),
		Data:       make(map[string]interface{}),
		Metadata:   make(map[string]interface{}),
		Relevance:  0.6,
		Confidence: 1.0,
	}

	stateIntegration := NewStateManagementIntegration(nil, nil, p.logger)
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

	recentChanges := make([]map[string]interface{}, 0)
	for _, stateType := range []StateType{
		StateTypeSession,
		StateTypeWorkflow,
		StateTypeTool,
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

	p.logger.Info("State context retrieved successfully",
		slog.String("session_id", request.SessionID),
		slog.Int("recent_changes", len(recentChanges)))

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

// GetName returns the provider name
func (p *StateContextProvider) GetName() string {
	return "state"
}
