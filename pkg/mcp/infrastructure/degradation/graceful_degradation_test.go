package degradation

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDegradationManager_BasicOperations(t *testing.T) {
	logger := slog.Default()
	dm := NewDegradationManager(logger)
	defer dm.Stop()

	// Initially all features should be enabled
	assert.True(t, dm.IsFeatureEnabled(FeatureAIAnalysis))
	assert.True(t, dm.IsFeatureEnabled(FeatureSecurityScan))
	assert.True(t, dm.IsFeatureEnabled(FeatureMLOptimization))

	// Register a healthy service
	dm.RegisterService("test-service", func(ctx context.Context) error {
		return nil
	})

	// Service should be healthy
	assert.Equal(t, HealthHealthy, dm.GetServiceHealth("test-service"))

	// Degradation level should be 0
	assert.Equal(t, 0, dm.GetDegradationLevel())
}

func TestDegradationManager_ServiceDegradation(t *testing.T) {
	logger := slog.Default()
	dm := NewDegradationManager(logger)
	defer dm.Stop()

	errorCount := 0
	dm.RegisterService("failing-service", func(ctx context.Context) error {
		errorCount++
		if errorCount < 3 {
			return errors.New("service error")
		}
		return nil
	})

	// Manually trigger health checks
	dm.checkAllServices()
	assert.Equal(t, HealthHealthy, dm.GetServiceHealth("failing-service"))

	// After second failure
	dm.checkAllServices()
	assert.Equal(t, HealthDegraded, dm.GetServiceHealth("failing-service"))

	// Service recovers
	dm.checkAllServices()
	dm.checkAllServices()
	assert.Equal(t, HealthDegraded, dm.GetServiceHealth("failing-service"))

	// Eventually healthy again
	dm.checkAllServices()
	assert.Equal(t, HealthHealthy, dm.GetServiceHealth("failing-service"))
}

func TestDegradationManager_FeatureToggling(t *testing.T) {
	logger := slog.Default()
	dm := NewDegradationManager(logger)
	defer dm.Stop()

	// Register multiple unhealthy services
	dm.RegisterService("service1", func(ctx context.Context) error {
		return errors.New("error")
	})
	dm.RegisterService("service2", func(ctx context.Context) error {
		return errors.New("error")
	})

	// Make services unhealthy
	for i := 0; i < 6; i++ {
		dm.checkAllServices()
	}

	// Apply degradation policies
	dm.applyDegradationPolicies()

	// High degradation should disable features
	degradation := dm.GetDegradationLevel()
	assert.Greater(t, degradation, 0)

	if degradation >= 80 {
		assert.False(t, dm.IsFeatureEnabled(FeatureAIAnalysis))
		assert.False(t, dm.IsFeatureEnabled(FeatureMLOptimization))
	}
}

func TestDegradableService_ExecuteWithFallback(t *testing.T) {
	logger := slog.Default()
	dm := NewDegradationManager(logger)
	defer dm.Stop()

	service := NewDegradableService(dm, "test-service")

	fallbackExecuted := false
	service.RegisterFallback(FeatureAIAnalysis, func() error {
		fallbackExecuted = true
		return nil
	})

	// Feature is enabled, normal operation should run
	normalExecuted := false
	err := service.ExecuteWithFallback(context.Background(), FeatureAIAnalysis, func() error {
		normalExecuted = true
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, normalExecuted)
	assert.False(t, fallbackExecuted)

	// Disable feature
	dm.mu.Lock()
	dm.featureToggles[FeatureAIAnalysis] = false
	dm.mu.Unlock()

	// Now fallback should run
	normalExecuted = false
	fallbackExecuted = false
	err = service.ExecuteWithFallback(context.Background(), FeatureAIAnalysis, func() error {
		normalExecuted = true
		return nil
	})

	assert.NoError(t, err)
	assert.False(t, normalExecuted)
	assert.True(t, fallbackExecuted)
}

func TestDegradableService_NoFallback(t *testing.T) {
	logger := slog.Default()
	dm := NewDegradationManager(logger)
	defer dm.Stop()

	service := NewDegradableService(dm, "test-service")

	// Disable feature without fallback
	dm.mu.Lock()
	dm.featureToggles[FeatureSecurityScan] = false
	dm.mu.Unlock()

	err := service.ExecuteWithFallback(context.Background(), FeatureSecurityScan, func() error {
		return nil
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled and no fallback available")
}

func TestHealthStatus(t *testing.T) {
	logger := slog.Default()
	dm := NewDegradationManager(logger)
	defer dm.Stop()

	// Register services
	dm.RegisterService("healthy-service", func(ctx context.Context) error {
		return nil
	})
	dm.RegisterService("unhealthy-service", func(ctx context.Context) error {
		return errors.New("error")
	})

	// Run health checks
	dm.checkAllServices()

	// Get health status
	status := dm.GetHealthStatus()

	assert.NotNil(t, status.Services)
	assert.NotNil(t, status.Features)
	assert.GreaterOrEqual(t, status.DegradationLevel, 0)
	assert.NotZero(t, status.Timestamp)

	// Check specific service statuses
	assert.Contains(t, status.Services, "healthy-service")
	assert.Contains(t, status.Services, "unhealthy-service")

	// Check features
	assert.Contains(t, status.Features, string(FeatureAIAnalysis))
	assert.Contains(t, status.Features, string(FeatureSecurityScan))
}

func TestGracefulOrchestrator(t *testing.T) {
	logger := slog.Default()
	dm := NewDegradationManager(logger)
	defer dm.Stop()

	// Mock orchestrator
	mockOrch := &mockOrchestrator{
		result: &workflow.ContainerizeAndDeployResult{
			Success:  true,
			ImageRef: "test/image:latest",
		},
	}

	gracefulOrch := NewGracefulOrchestrator(mockOrch, dm)

	// Execute with no degradation
	req := &mcp.CallToolRequest{}
	args := &workflow.ContainerizeAndDeployArgs{}
	result, err := gracefulOrch.Execute(context.Background(), req, args)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)

	// Disable some features
	dm.mu.Lock()
	dm.featureToggles[FeatureAIAnalysis] = false
	dm.featureToggles[FeatureMLOptimization] = false
	dm.mu.Unlock()

	// Execute with degradation - verify it still works
	result, err = gracefulOrch.Execute(context.Background(), req, args)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)

	// Verify features are disabled
	assert.False(t, dm.IsFeatureEnabled(FeatureAIAnalysis))
	assert.False(t, dm.IsFeatureEnabled(FeatureMLOptimization))
}

func TestServiceResponseTimeDegradation(t *testing.T) {
	logger := slog.Default()
	dm := NewDegradationManager(logger)
	defer dm.Stop()

	// Register a slow service
	dm.RegisterService("slow-service", func(ctx context.Context) error {
		time.Sleep(6 * time.Second)
		return nil
	})

	// Update status with slow response time
	dm.updateServiceStatus("slow-service", nil, 6*time.Second)

	// Service should be degraded due to slow response
	assert.Equal(t, HealthDegraded, dm.GetServiceHealth("slow-service"))
}

// Mock implementations for testing

type mockOrchestrator struct {
	result *workflow.ContainerizeAndDeployResult
	err    error
}

func (m *mockOrchestrator) Execute(ctx context.Context, req *mcp.CallToolRequest, args *workflow.ContainerizeAndDeployArgs) (*workflow.ContainerizeAndDeployResult, error) {
	return m.result, m.err
}

func BenchmarkDegradationManager_IsFeatureEnabled(b *testing.B) {
	logger := slog.Default()
	dm := NewDegradationManager(logger)
	defer dm.Stop()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			dm.IsFeatureEnabled(FeatureAIAnalysis)
		}
	})
}

func BenchmarkDegradationManager_GetServiceHealth(b *testing.B) {
	logger := slog.Default()
	dm := NewDegradationManager(logger)
	defer dm.Stop()

	dm.RegisterService("test-service", func(ctx context.Context) error {
		return nil
	})

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			dm.GetServiceHealth("test-service")
		}
	})
}
