package security

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockHealthChecker for testing
type MockHealthChecker struct {
	name         string
	dependencies []string
	status       HealthStatus
	message      string
	shouldFail   bool
}

func (m *MockHealthChecker) GetName() string {
	return m.name
}

func (m *MockHealthChecker) GetDependencies() []string {
	return m.dependencies
}

func (m *MockHealthChecker) CheckHealth(_ context.Context) ComponentHealth {
	if m.shouldFail {
		return ComponentHealth{
			Name:         m.name,
			Status:       HealthStatusUnhealthy,
			Message:      "Mock failure",
			LastChecked:  time.Now(),
			Dependencies: m.dependencies,
		}
	}

	return ComponentHealth{
		Name:         m.name,
		Status:       m.status,
		Message:      m.message,
		LastChecked:  time.Now(),
		Dependencies: m.dependencies,
	}
}

func TestNewHealthMonitor(t *testing.T) {
	logger := zerolog.Nop()
	monitor := NewHealthMonitor(logger)

	assert.NotNil(t, monitor)
	assert.Equal(t, 30*time.Second, monitor.checkInterval)
	assert.Equal(t, 10*time.Second, monitor.timeout)
	assert.False(t, monitor.running)
}

func TestHealthMonitor_RegisterChecker(t *testing.T) {
	logger := zerolog.Nop()
	monitor := NewHealthMonitor(logger)

	checker := &MockHealthChecker{
		name:         "test_checker",
		dependencies: []string{"dependency1"},
		status:       HealthStatusHealthy,
		message:      "Test message",
	}

	monitor.RegisterChecker(checker)

	assert.Contains(t, monitor.checkers, "test_checker")
	assert.Contains(t, monitor.results, "test_checker")

	result := monitor.results["test_checker"]
	assert.Equal(t, "test_checker", result.Name)
	assert.Equal(t, HealthStatusUnhealthy, result.Status) // Initial state
	assert.Equal(t, "Not yet checked", result.Message)
	assert.Equal(t, []string{"dependency1"}, result.Dependencies)
}

func TestHealthMonitor_UnregisterChecker(t *testing.T) {
	logger := zerolog.Nop()
	monitor := NewHealthMonitor(logger)

	checker := &MockHealthChecker{name: "test_checker"}
	monitor.RegisterChecker(checker)

	assert.Contains(t, monitor.checkers, "test_checker")

	monitor.UnregisterChecker("test_checker")

	assert.NotContains(t, monitor.checkers, "test_checker")
	assert.NotContains(t, monitor.results, "test_checker")
}

func TestHealthMonitor_CheckAllHealth(t *testing.T) {
	logger := zerolog.Nop()
	monitor := NewHealthMonitor(logger)

	healthyChecker := &MockHealthChecker{
		name:    "healthy_checker",
		status:  HealthStatusHealthy,
		message: "All good",
	}

	unhealthyChecker := &MockHealthChecker{
		name:       "unhealthy_checker",
		shouldFail: true,
	}

	monitor.RegisterChecker(healthyChecker)
	monitor.RegisterChecker(unhealthyChecker)

	ctx := context.Background()
	monitor.checkAllHealth(ctx)

	// Check healthy component
	healthyResult := monitor.results["healthy_checker"]
	assert.Equal(t, HealthStatusHealthy, healthyResult.Status)
	assert.Equal(t, "All good", healthyResult.Message)
	assert.Equal(t, int64(1), healthyResult.CheckCount)
	assert.Equal(t, int64(0), healthyResult.FailureCount)

	// Check unhealthy component
	unhealthyResult := monitor.results["unhealthy_checker"]
	assert.Equal(t, HealthStatusUnhealthy, unhealthyResult.Status)
	assert.Equal(t, "Mock failure", unhealthyResult.Message)
	assert.Equal(t, int64(1), unhealthyResult.CheckCount)
	assert.Equal(t, int64(1), unhealthyResult.FailureCount)
}

func TestHealthMonitor_GetHealth(t *testing.T) {
	logger := zerolog.Nop()
	monitor := NewHealthMonitor(logger)
	monitor.SetVersion("1.0.0")

	healthyChecker := &MockHealthChecker{
		name:    "healthy_checker",
		status:  HealthStatusHealthy,
		message: "All good",
	}

	degradedChecker := &MockHealthChecker{
		name:    "degraded_checker",
		status:  HealthStatusDegraded,
		message: "Partially working",
	}

	monitor.RegisterChecker(healthyChecker)
	monitor.RegisterChecker(degradedChecker)

	ctx := context.Background()
	monitor.checkAllHealth(ctx)

	health := monitor.GetHealth()

	assert.Equal(t, HealthStatusDegraded, health.Status) // Overall degraded due to one degraded component
	assert.Equal(t, "1.0.0", health.Version)
	assert.Contains(t, health.Components, "healthy_checker")
	assert.Contains(t, health.Components, "degraded_checker")

	// Check summary
	assert.Equal(t, 2, health.Summary.TotalComponents)
	assert.Equal(t, 1, health.Summary.HealthyComponents)
	assert.Equal(t, 1, health.Summary.DegradedComponents)
	assert.Equal(t, 0, health.Summary.UnhealthyComponents)
}

func TestHealthMonitor_GetReadiness(t *testing.T) {
	logger := zerolog.Nop()
	monitor := NewHealthMonitor(logger)

	healthyChecker := &MockHealthChecker{
		name:    "healthy_checker",
		status:  HealthStatusHealthy,
		message: "Ready",
	}

	unhealthyChecker := &MockHealthChecker{
		name:       "unhealthy_checker",
		shouldFail: true,
	}

	monitor.RegisterChecker(healthyChecker)
	monitor.RegisterChecker(unhealthyChecker)

	ctx := context.Background()
	monitor.checkAllHealth(ctx)

	readiness := monitor.GetReadiness()

	assert.False(t, readiness.Ready) // Not ready due to unhealthy component
	assert.Len(t, readiness.Checks, 2)
	assert.Contains(t, readiness.Message, "Not ready")

	// Check individual readiness checks
	for _, check := range readiness.Checks {
		if check.Component == "healthy_checker" {
			assert.True(t, check.Ready)
		} else if check.Component == "unhealthy_checker" {
			assert.False(t, check.Ready)
		}
	}
}

func TestHealthMonitor_StartStop(t *testing.T) {
	logger := zerolog.Nop()
	monitor := NewHealthMonitor(logger)
	monitor.SetCheckInterval(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Test starting
	err := monitor.Start(ctx)
	assert.NoError(t, err)
	assert.True(t, monitor.running)

	// Test starting again (should fail)
	err = monitor.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	// Test stopping
	err = monitor.Stop()
	assert.NoError(t, err)
	assert.False(t, monitor.running)

	// Test stopping again (should fail)
	err = monitor.Stop()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestHealthEndpointHandler_HealthzHandler(t *testing.T) {
	logger := zerolog.Nop()
	monitor := NewHealthMonitor(logger)
	handler := NewHealthEndpointHandler(monitor, logger)

	// Add a healthy checker
	healthyChecker := &MockHealthChecker{
		name:    "test_checker",
		status:  HealthStatusHealthy,
		message: "All good",
	}
	monitor.RegisterChecker(healthyChecker)
	monitor.checkAllHealth(context.Background())

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()

	handler.HealthzHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var health OverallHealth
	err := json.Unmarshal(w.Body.Bytes(), &health)
	assert.NoError(t, err)
	assert.Equal(t, HealthStatusHealthy, health.Status)
	assert.Contains(t, health.Components, "test_checker")
}

func TestHealthEndpointHandler_HealthzHandler_Unhealthy(t *testing.T) {
	logger := zerolog.Nop()
	monitor := NewHealthMonitor(logger)
	handler := NewHealthEndpointHandler(monitor, logger)

	// Add an unhealthy checker
	unhealthyChecker := &MockHealthChecker{
		name:       "test_checker",
		shouldFail: true,
	}
	monitor.RegisterChecker(unhealthyChecker)
	monitor.checkAllHealth(context.Background())

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()

	handler.HealthzHandler(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var health OverallHealth
	err := json.Unmarshal(w.Body.Bytes(), &health)
	assert.NoError(t, err)
	assert.Equal(t, HealthStatusUnhealthy, health.Status)
}

func TestHealthEndpointHandler_ReadyzHandler(t *testing.T) {
	logger := zerolog.Nop()
	monitor := NewHealthMonitor(logger)
	handler := NewHealthEndpointHandler(monitor, logger)

	// Add a healthy checker
	healthyChecker := &MockHealthChecker{
		name:    "test_checker",
		status:  HealthStatusHealthy,
		message: "Ready",
	}
	monitor.RegisterChecker(healthyChecker)
	monitor.checkAllHealth(context.Background())

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()

	handler.ReadyzHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var readiness ReadinessStatus
	err := json.Unmarshal(w.Body.Bytes(), &readiness)
	assert.NoError(t, err)
	assert.True(t, readiness.Ready)
	assert.Len(t, readiness.Checks, 1)
}

func TestHealthEndpointHandler_ReadyzHandler_NotReady(t *testing.T) {
	logger := zerolog.Nop()
	monitor := NewHealthMonitor(logger)
	handler := NewHealthEndpointHandler(monitor, logger)

	// Add an unhealthy checker
	unhealthyChecker := &MockHealthChecker{
		name:       "test_checker",
		shouldFail: true,
	}
	monitor.RegisterChecker(unhealthyChecker)
	monitor.checkAllHealth(context.Background())

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()

	handler.ReadyzHandler(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var readiness ReadinessStatus
	err := json.Unmarshal(w.Body.Bytes(), &readiness)
	assert.NoError(t, err)
	assert.False(t, readiness.Ready)
	assert.Contains(t, readiness.Message, "Not ready")
}

func TestHealthEndpointHandler_LivezHandler(t *testing.T) {
	logger := zerolog.Nop()
	monitor := NewHealthMonitor(logger)
	handler := NewHealthEndpointHandler(monitor, logger)

	req := httptest.NewRequest("GET", "/livez", nil)
	w := httptest.NewRecorder()

	handler.LivezHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var liveness map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &liveness)
	assert.NoError(t, err)
	assert.Equal(t, "alive", liveness["status"])
	assert.Contains(t, liveness, "timestamp")
	assert.Contains(t, liveness, "uptime")
}

func TestHealthEndpointHandler_RegisterRoutes(t *testing.T) {
	logger := zerolog.Nop()
	monitor := NewHealthMonitor(logger)
	handler := NewHealthEndpointHandler(monitor, logger)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Test that routes are registered by making requests
	endpoints := []string{"/healthz", "/readyz", "/livez"}

	for _, endpoint := range endpoints {
		req := httptest.NewRequest("GET", endpoint, nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		// Should not return 404
		assert.NotEqual(t, http.StatusNotFound, w.Code)
	}
}

func TestTrivyScannerHealthChecker(t *testing.T) {
	logger := zerolog.Nop()
	checker := NewTrivyScannerHealthChecker(logger)

	assert.Equal(t, "trivy_scanner", checker.GetName())
	assert.Equal(t, []string{"filesystem", "network"}, checker.GetDependencies())

	// Note: This test will fail if Trivy is not installed, which is expected
	health := checker.CheckHealth(context.Background())
	assert.Equal(t, "trivy_scanner", health.Name)
	assert.Contains(t, []HealthStatus{HealthStatusHealthy, HealthStatusUnhealthy}, health.Status)
}

func TestGrypeScannerHealthChecker(t *testing.T) {
	logger := zerolog.Nop()
	checker := NewGrypeScannerHealthChecker(logger)

	assert.Equal(t, "grype_scanner", checker.GetName())
	assert.Equal(t, []string{"filesystem", "network"}, checker.GetDependencies())

	// Note: This test will fail if Grype is not installed, which is expected
	health := checker.CheckHealth(context.Background())
	assert.Equal(t, "grype_scanner", health.Name)
	assert.Contains(t, []HealthStatus{HealthStatusHealthy, HealthStatusUnhealthy}, health.Status)
}

func TestPolicyEngineHealthChecker(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("nil policy engine", func(t *testing.T) {
		checker := NewPolicyEngineHealthChecker(logger, nil)
		health := checker.CheckHealth(context.Background())

		assert.Equal(t, "policy_engine", health.Name)
		assert.Equal(t, HealthStatusUnhealthy, health.Status)
		assert.Contains(t, health.Message, "not initialized")
	})

	t.Run("policy engine with no policies", func(t *testing.T) {
		policyEngine := NewPolicyEngine(logger)
		checker := NewPolicyEngineHealthChecker(logger, policyEngine)
		health := checker.CheckHealth(context.Background())

		assert.Equal(t, "policy_engine", health.Name)
		assert.Equal(t, HealthStatusDegraded, health.Status)
		assert.Contains(t, health.Message, "No security policies loaded")
	})

	t.Run("policy engine with policies", func(t *testing.T) {
		policyEngine := NewPolicyEngine(logger)
		err := policyEngine.LoadDefaultPolicies()
		require.NoError(t, err)

		checker := NewPolicyEngineHealthChecker(logger, policyEngine)
		health := checker.CheckHealth(context.Background())

		assert.Equal(t, "policy_engine", health.Name)
		assert.Equal(t, HealthStatusHealthy, health.Status)
		assert.Contains(t, health.Message, "operational with")
		assert.Greater(t, health.Metadata["enabled_policies"], 0)
	})
}

func TestSecretDiscoveryHealthChecker(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("nil secret discovery", func(t *testing.T) {
		checker := NewSecretDiscoveryHealthChecker(logger, nil)
		health := checker.CheckHealth(context.Background())

		assert.Equal(t, "secret_discovery", health.Name)
		assert.Equal(t, HealthStatusUnhealthy, health.Status)
		assert.Contains(t, health.Message, "not initialized")
	})

	t.Run("working secret discovery", func(t *testing.T) {
		secretDiscovery := NewSecretDiscovery(logger)
		checker := NewSecretDiscoveryHealthChecker(logger, secretDiscovery)
		health := checker.CheckHealth(context.Background())

		assert.Equal(t, "secret_discovery", health.Name)
		assert.Equal(t, HealthStatusHealthy, health.Status)
		assert.Contains(t, health.Message, "operational")
		assert.Greater(t, health.Metadata["test_findings"], 0)
	})
}

func TestCalculateOverallStatus(t *testing.T) {
	logger := zerolog.Nop()
	monitor := NewHealthMonitor(logger)

	tests := []struct {
		name     string
		summary  HealthSummary
		expected HealthStatus
	}{
		{
			name: "all healthy",
			summary: HealthSummary{
				TotalComponents:     3,
				HealthyComponents:   3,
				DegradedComponents:  0,
				UnhealthyComponents: 0,
			},
			expected: HealthStatusHealthy,
		},
		{
			name: "some degraded",
			summary: HealthSummary{
				TotalComponents:     3,
				HealthyComponents:   2,
				DegradedComponents:  1,
				UnhealthyComponents: 0,
			},
			expected: HealthStatusDegraded,
		},
		{
			name: "some unhealthy",
			summary: HealthSummary{
				TotalComponents:     3,
				HealthyComponents:   1,
				DegradedComponents:  1,
				UnhealthyComponents: 1,
			},
			expected: HealthStatusUnhealthy,
		},
		{
			name: "no components",
			summary: HealthSummary{
				TotalComponents:     0,
				HealthyComponents:   0,
				DegradedComponents:  0,
				UnhealthyComponents: 0,
			},
			expected: HealthStatusUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := monitor.calculateOverallStatus(tt.summary)
			assert.Equal(t, tt.expected, status)
		})
	}
}
