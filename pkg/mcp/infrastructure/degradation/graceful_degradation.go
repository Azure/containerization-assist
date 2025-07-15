// Package degradation provides graceful degradation capabilities for Container Kit
package degradation

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/mark3labs/mcp-go/mcp"
)

// ServiceHealth represents the health status of a service
type ServiceHealth int

const (
	HealthHealthy ServiceHealth = iota
	HealthDegraded
	HealthUnhealthy
)

// Feature represents a toggleable feature
type Feature string

const (
	FeatureAIAnalysis      Feature = "ai_analysis"
	FeatureSecurityScan    Feature = "security_scan"
	FeatureMLOptimization  Feature = "ml_optimization"
	FeatureAdvancedMetrics Feature = "advanced_metrics"
	FeatureAutoRetry       Feature = "auto_retry"
	FeatureCaching         Feature = "caching"
)

// ServiceStatus tracks the status of a service
type ServiceStatus struct {
	Name          string
	Health        ServiceHealth
	LastCheckTime time.Time
	ErrorCount    int
	ResponseTime  time.Duration
}

// DegradationManager manages graceful degradation of services
type DegradationManager struct {
	mu              sync.RWMutex
	serviceStatuses map[string]*ServiceStatus
	featureToggles  map[Feature]bool
	healthCheckers  map[string]HealthChecker
	logger          *slog.Logger
	checkInterval   time.Duration
	stopChan        chan struct{}
}

// HealthChecker checks the health of a service
type HealthChecker func(ctx context.Context) error

// NewDegradationManager creates a new degradation manager
func NewDegradationManager(logger *slog.Logger) *DegradationManager {
	dm := &DegradationManager{
		serviceStatuses: make(map[string]*ServiceStatus),
		featureToggles:  make(map[Feature]bool),
		healthCheckers:  make(map[string]HealthChecker),
		logger:          logger,
		checkInterval:   30 * time.Second,
		stopChan:        make(chan struct{}),
	}

	// Enable all features by default
	dm.enableAllFeatures()

	// Start health monitoring
	go dm.monitorHealth()

	return dm
}

// RegisterService registers a service for health monitoring
func (dm *DegradationManager) RegisterService(name string, checker HealthChecker) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.healthCheckers[name] = checker
	dm.serviceStatuses[name] = &ServiceStatus{
		Name:          name,
		Health:        HealthHealthy,
		LastCheckTime: time.Now(),
	}
}

// IsFeatureEnabled checks if a feature is enabled
func (dm *DegradationManager) IsFeatureEnabled(feature Feature) bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	enabled, exists := dm.featureToggles[feature]
	if !exists {
		return false
	}
	return enabled
}

// GetServiceHealth returns the health of a service
func (dm *DegradationManager) GetServiceHealth(service string) ServiceHealth {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	status, exists := dm.serviceStatuses[service]
	if !exists {
		return HealthUnhealthy
	}
	return status.Health
}

// GetDegradationLevel returns the overall degradation level (0-100)
func (dm *DegradationManager) GetDegradationLevel() int {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if len(dm.serviceStatuses) == 0 {
		return 0
	}

	unhealthyCount := 0
	degradedCount := 0

	for _, status := range dm.serviceStatuses {
		switch status.Health {
		case HealthUnhealthy:
			unhealthyCount++
		case HealthDegraded:
			degradedCount++
		}
	}

	// Calculate degradation percentage
	total := len(dm.serviceStatuses)
	degradation := (unhealthyCount*100 + degradedCount*50) / total

	return degradation
}

// Stop stops the degradation manager
func (dm *DegradationManager) Stop() {
	close(dm.stopChan)
}

// monitorHealth continuously monitors service health
func (dm *DegradationManager) monitorHealth() {
	ticker := time.NewTicker(dm.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			dm.checkAllServices()
			dm.applyDegradationPolicies()
		case <-dm.stopChan:
			return
		}
	}
}

// checkAllServices checks the health of all registered services
func (dm *DegradationManager) checkAllServices() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for name, checker := range dm.healthCheckers {
		start := time.Now()
		err := checker(ctx)
		responseTime := time.Since(start)

		dm.updateServiceStatus(name, err, responseTime)
	}
}

// updateServiceStatus updates the status of a service based on health check
func (dm *DegradationManager) updateServiceStatus(name string, err error, responseTime time.Duration) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	status, exists := dm.serviceStatuses[name]
	if !exists {
		return
	}

	status.LastCheckTime = time.Now()
	status.ResponseTime = responseTime

	if err != nil {
		status.ErrorCount++

		// Determine health based on error count
		if status.ErrorCount >= 5 {
			status.Health = HealthUnhealthy
		} else if status.ErrorCount >= 2 {
			status.Health = HealthDegraded
		}

		dm.logger.Warn("Service health check failed",
			"service", name,
			"error", err,
			"error_count", status.ErrorCount,
			"health", status.Health)
	} else {
		// Reset error count on success, but recovery is slower than degradation
		// Allow error count to go negative to implement gradual recovery
		status.ErrorCount--

		// Restore health gradually - need error count to go below zero to become healthy
		// This ensures recovery takes longer than degradation
		if status.ErrorCount <= -1 {
			status.Health = HealthHealthy
		} else if status.ErrorCount < 2 {
			status.Health = HealthDegraded
		}

		// Check response time
		if responseTime > 5*time.Second && status.Health == HealthHealthy {
			status.Health = HealthDegraded
			dm.logger.Warn("Service response time degraded",
				"service", name,
				"response_time", responseTime)
		}
	}
}

// applyDegradationPolicies applies degradation policies based on service health
func (dm *DegradationManager) applyDegradationPolicies() {
	degradationLevel := dm.GetDegradationLevel()

	dm.mu.Lock()
	defer dm.mu.Unlock()

	// Apply progressive degradation
	if degradationLevel >= 80 {
		// Severe degradation - disable non-essential features
		dm.featureToggles[FeatureAIAnalysis] = false
		dm.featureToggles[FeatureMLOptimization] = false
		dm.featureToggles[FeatureAdvancedMetrics] = false
		dm.featureToggles[FeatureSecurityScan] = false
		dm.logger.Error("Severe degradation - disabled non-essential features", "level", degradationLevel)
	} else if degradationLevel >= 50 {
		// Moderate degradation - disable expensive features
		dm.featureToggles[FeatureMLOptimization] = false
		dm.featureToggles[FeatureAdvancedMetrics] = false
		dm.logger.Warn("Moderate degradation - disabled expensive features", "level", degradationLevel)
	} else if degradationLevel >= 20 {
		// Light degradation - disable luxury features
		dm.featureToggles[FeatureAdvancedMetrics] = false
		dm.logger.Info("Light degradation - disabled luxury features", "level", degradationLevel)
	} else {
		// Healthy - enable all features
		dm.enableAllFeatures()
	}
}

// enableAllFeatures enables all features
func (dm *DegradationManager) enableAllFeatures() {
	dm.featureToggles[FeatureAIAnalysis] = true
	dm.featureToggles[FeatureSecurityScan] = true
	dm.featureToggles[FeatureMLOptimization] = true
	dm.featureToggles[FeatureAdvancedMetrics] = true
	dm.featureToggles[FeatureAutoRetry] = true
	dm.featureToggles[FeatureCaching] = true
}

// FallbackStrategy defines a fallback strategy for a feature
type FallbackStrategy struct {
	Feature  Feature
	Fallback func() error
}

// DegradableService wraps a service with degradation capabilities
type DegradableService struct {
	manager    *DegradationManager
	name       string
	strategies map[Feature]FallbackStrategy
}

// NewDegradableService creates a new degradable service
func NewDegradableService(manager *DegradationManager, name string) *DegradableService {
	return &DegradableService{
		manager:    manager,
		name:       name,
		strategies: make(map[Feature]FallbackStrategy),
	}
}

// RegisterFallback registers a fallback strategy for a feature
func (ds *DegradableService) RegisterFallback(feature Feature, fallback func() error) {
	ds.strategies[feature] = FallbackStrategy{
		Feature:  feature,
		Fallback: fallback,
	}
}

// ExecuteWithFallback executes an operation with fallback
func (ds *DegradableService) ExecuteWithFallback(ctx context.Context, feature Feature, operation func() error) error {
	// Check if feature is enabled
	if !ds.manager.IsFeatureEnabled(feature) {
		// Use fallback if available
		if strategy, exists := ds.strategies[feature]; exists {
			ds.manager.logger.Info("Using fallback for disabled feature",
				"service", ds.name,
				"feature", feature)
			return strategy.Fallback()
		}
		return fmt.Errorf("feature %s is disabled and no fallback available", feature)
	}

	// Execute normal operation
	return operation()
}

// GracefulOrchestrator wraps a workflow orchestrator with graceful degradation
type GracefulOrchestrator struct {
	base    workflow.WorkflowOrchestrator
	manager *DegradationManager
	service *DegradableService
}

// NewGracefulOrchestrator creates a new orchestrator with graceful degradation
func NewGracefulOrchestrator(base workflow.WorkflowOrchestrator, manager *DegradationManager) *GracefulOrchestrator {
	service := NewDegradableService(manager, "workflow_orchestrator")

	// Register fallbacks
	service.RegisterFallback(FeatureAIAnalysis, func() error {
		// Simple analysis without AI
		return nil
	})

	service.RegisterFallback(FeatureSecurityScan, func() error {
		// Skip security scan
		return nil
	})

	service.RegisterFallback(FeatureMLOptimization, func() error {
		// Use default optimization
		return nil
	})

	return &GracefulOrchestrator{
		base:    base,
		manager: manager,
		service: service,
	}
}

// Execute executes the workflow with graceful degradation
func (g *GracefulOrchestrator) Execute(ctx context.Context, req *mcp.CallToolRequest, args *workflow.ContainerizeAndDeployArgs) (*workflow.ContainerizeAndDeployResult, error) {
	// Log degradation level
	degradationLevel := g.manager.GetDegradationLevel()
	if degradationLevel > 0 {
		g.manager.logger.Info("Executing workflow with degradation",
			"level", degradationLevel,
			"disabled_features", g.getDisabledFeatures())
	}

	// Execute base workflow
	// In a real implementation, individual steps would check feature toggles
	// and use fallback strategies when features are disabled
	result, err := g.base.Execute(ctx, req, args)

	// Log degradation info with result
	if result != nil && degradationLevel > 0 {
		g.manager.logger.Info("Workflow completed with degradation",
			"success", result.Success,
			"degradation_level", degradationLevel,
			"disabled_features", g.getDisabledFeatures())
	}

	return result, err
}

// getDisabledFeatures returns a list of disabled features
func (g *GracefulOrchestrator) getDisabledFeatures() []string {
	var disabled []string

	allFeatures := []Feature{
		FeatureAIAnalysis,
		FeatureSecurityScan,
		FeatureMLOptimization,
		FeatureAdvancedMetrics,
		FeatureAutoRetry,
		FeatureCaching,
	}

	for _, feature := range allFeatures {
		if !g.manager.IsFeatureEnabled(feature) {
			disabled = append(disabled, string(feature))
		}
	}

	return disabled
}

// HealthStatus represents the overall health status
type HealthStatus struct {
	Services         map[string]ServiceStatus `json:"services"`
	Features         map[string]bool          `json:"features"`
	DegradationLevel int                      `json:"degradation_level"`
	Timestamp        time.Time                `json:"timestamp"`
}

// GetHealthStatus returns the current health status
func (dm *DegradationManager) GetHealthStatus() HealthStatus {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	// Copy service statuses
	services := make(map[string]ServiceStatus)
	for name, status := range dm.serviceStatuses {
		services[name] = *status
	}

	// Copy feature toggles
	features := make(map[string]bool)
	for feature, enabled := range dm.featureToggles {
		features[string(feature)] = enabled
	}

	return HealthStatus{
		Services:         services,
		Features:         features,
		DegradationLevel: dm.GetDegradationLevel(),
		Timestamp:        time.Now(),
	}
}
