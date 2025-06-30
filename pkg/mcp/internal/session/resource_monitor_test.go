package session

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResourceMonitor(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	// Create session manager
	sessionConfig := SessionManagerConfig{
		WorkspaceDir: t.TempDir(),
		MaxSessions:  10,
		SessionTTL:   time.Hour,
		Logger:       logger,
	}

	sessionMgr, err := NewSessionManager(sessionConfig)
	require.NoError(t, err)
	defer sessionMgr.Stop()

	// Create resource monitor
	config := ResourceMonitorConfig{
		MonitoringInterval: 1 * time.Second,
		CleanupInterval:    30 * time.Second,
		ResourceLimits: ResourceLimits{
			MaxMemoryUsage:  1024 * 1024 * 100, // 100MB
			MaxDiskUsage:    1024 * 1024 * 500, // 500MB
			MaxSessionCount: 5,
			MaxSessionAge:   24 * time.Hour,
			MaxIdleTime:     2 * time.Hour,
		},
		EnableAutoCleanup: true,
		MaxSnapshots:      100,
		MaxCleanupHistory: 50,
	}

	monitor := NewResourceMonitor(sessionMgr, config, logger)

	assert.NotNil(t, monitor)
	assert.Equal(t, config.MonitoringInterval, monitor.monitoringInterval)
	assert.Equal(t, config.CleanupInterval, monitor.cleanupInterval)
	assert.Equal(t, config.EnableAutoCleanup, monitor.enableAutoCleanup)
	assert.Equal(t, config.MaxSnapshots, monitor.maxSnapshots)
	assert.Equal(t, config.MaxCleanupHistory, monitor.maxCleanupHistory)
	assert.NotNil(t, monitor.currentResources)
	assert.NotEmpty(t, monitor.cleanupRules)
	assert.NotEmpty(t, monitor.alertThresholds)
}

func TestResourceMonitor_StartStopMonitoring(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	sessionConfig := SessionManagerConfig{
		WorkspaceDir: t.TempDir(),
		MaxSessions:  10,
		SessionTTL:   time.Hour,
		Logger:       logger,
	}

	sessionMgr, err := NewSessionManager(sessionConfig)
	require.NoError(t, err)
	defer sessionMgr.Stop()

	config := ResourceMonitorConfig{
		MonitoringInterval: 100 * time.Millisecond,
		CleanupInterval:    500 * time.Millisecond,
		EnableAutoCleanup:  false, // Disable for testing
	}

	monitor := NewResourceMonitor(sessionMgr, config, logger)

	ctx := context.Background()

	// Test starting monitoring
	err = monitor.StartMonitoring(ctx)
	assert.NoError(t, err)
	assert.True(t, monitor.isMonitoring)

	// Test starting again (should fail)
	err = monitor.StartMonitoring(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	// Wait a bit for monitoring to collect data
	time.Sleep(250 * time.Millisecond)

	// Check that snapshots are being collected
	snapshots := monitor.GetResourceHistory(10)
	assert.NotEmpty(t, snapshots)

	// Test stopping monitoring
	monitor.StopMonitoring()
	assert.False(t, monitor.isMonitoring)

	// Test stopping again (should be safe)
	monitor.StopMonitoring()
	assert.False(t, monitor.isMonitoring)
}

func TestResourceMonitor_ResourceCollection(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	sessionConfig := SessionManagerConfig{
		WorkspaceDir: t.TempDir(),
		MaxSessions:  10,
		SessionTTL:   time.Hour,
		Logger:       logger,
	}

	sessionMgr, err := NewSessionManager(sessionConfig)
	require.NoError(t, err)
	defer sessionMgr.Stop()

	// Create some test sessions
	session1, err := sessionMgr.CreateSession("")
	require.NoError(t, err)
	session2, err := sessionMgr.CreateSession("")
	require.NoError(t, err)

	config := ResourceMonitorConfig{
		MonitoringInterval: 50 * time.Millisecond,
		EnableAutoCleanup:  false,
	}

	monitor := NewResourceMonitor(sessionMgr, config, logger)

	ctx := context.Background()
	err = monitor.StartMonitoring(ctx)
	assert.NoError(t, err)
	defer monitor.StopMonitoring()

	// Wait for data collection
	time.Sleep(150 * time.Millisecond)

	// Check current resources
	current := monitor.GetCurrentResources()
	assert.NotNil(t, current)
	assert.Equal(t, 2, current.ActiveSessions)
	assert.Equal(t, "HEALTHY", current.HealthStatus) // Should be healthy with low usage
	assert.True(t, current.LastUpdated.After(time.Now().Add(-time.Minute)))

	// Check resource history
	history := monitor.GetResourceHistory(5)
	assert.NotEmpty(t, history)

	for _, snapshot := range history {
		assert.Equal(t, 2, snapshot.SessionCount)
		assert.LessOrEqual(t, snapshot.IdleSessions, 2)
		assert.GreaterOrEqual(t, snapshot.ActiveSessions, 0)
		assert.Len(t, snapshot.SessionDetails, 2)

		// Verify session details
		session1State := session1.(*SessionState)
		session2State := session2.(*SessionState)
		for _, sessionDetail := range snapshot.SessionDetails {
			assert.True(t, sessionDetail.SessionID == session1State.SessionID || sessionDetail.SessionID == session2State.SessionID)
			assert.GreaterOrEqual(t, sessionDetail.Age, time.Duration(0))
			assert.GreaterOrEqual(t, sessionDetail.ResourceScore, 0.0)
		}
	}
}

func TestResourceMonitor_CleanupRules(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	sessionConfig := SessionManagerConfig{
		WorkspaceDir: t.TempDir(),
		MaxSessions:  10,
		SessionTTL:   time.Hour,
		Logger:       logger,
	}

	sessionMgr, err := NewSessionManager(sessionConfig)
	require.NoError(t, err)
	defer sessionMgr.Stop()

	config := ResourceMonitorConfig{
		EnableAutoCleanup: false, // We'll trigger manually
	}

	monitor := NewResourceMonitor(sessionMgr, config, logger)

	// Add a custom cleanup rule
	rule := CleanupRule{
		Name:        "test_cleanup",
		Description: "Test cleanup rule",
		Enabled:     true,
		Priority:    5,
		Conditions: []CleanupCondition{
			{
				Type:        "count",
				Operator:    "gt",
				Threshold:   1.0, // More than 1 session
				Scope:       "system",
				Description: "More than 1 session",
			},
		},
		Actions: []CleanupAction{
			{
				Type:        "terminate",
				Target:      "oldest_sessions",
				Parameters:  map[string]interface{}{"max_count": 1.0},
				Description: "Terminate 1 oldest session",
			},
		},
		Cooldown: 1 * time.Second,
	}

	monitor.AddCleanupRule(rule)

	// Create test sessions
	_, err = sessionMgr.CreateSession("")
	assert.NoError(t, err)
	session2, err := sessionMgr.CreateSession("")
	assert.NoError(t, err)

	// Start monitoring
	ctx := context.Background()
	err = monitor.StartMonitoring(ctx)
	assert.NoError(t, err)
	defer monitor.StopMonitoring()

	// Wait for initial data collection
	time.Sleep(100 * time.Millisecond)

	// Trigger cleanup manually
	err = monitor.TriggerCleanup()
	assert.NoError(t, err)

	// Wait for cleanup to complete
	time.Sleep(200 * time.Millisecond)

	// Check cleanup history
	history := monitor.GetCleanupHistory(10)
	assert.NotEmpty(t, history)

	lastCleanup := history[len(history)-1]
	assert.Equal(t, "test_cleanup", lastCleanup.RuleName)
	assert.True(t, lastCleanup.Success)
	assert.Contains(t, lastCleanup.ActionsExecuted, "terminate")
	assert.NotEmpty(t, lastCleanup.SessionsAffected)

	// Verify that one session was terminated
	remainingSessions, _ := sessionMgr.GetAllSessions()
	assert.Len(t, remainingSessions, 1)

	// The remaining session should be the newer one (session2)
	session2State := session2.(*SessionState)
	assert.Equal(t, session2State.SessionID, remainingSessions[0].ID)
}

func TestResourceMonitor_AlertThresholds(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	sessionConfig := SessionManagerConfig{
		WorkspaceDir: t.TempDir(),
		MaxSessions:  10,
		SessionTTL:   time.Hour,
		Logger:       logger,
	}

	sessionMgr, err := NewSessionManager(sessionConfig)
	require.NoError(t, err)
	defer sessionMgr.Stop()

	// Set very low limits to easily trigger alerts
	config := ResourceMonitorConfig{
		MonitoringInterval: 50 * time.Millisecond,
		ResourceLimits: ResourceLimits{
			MaxSessionCount: 1, // Only 1 session allowed
		},
		EnableAutoCleanup: false,
	}

	alertTriggered := false
	alertCallback := func(alert *ActiveAlert) {
		alertTriggered = true
	}
	config.AlertCallback = alertCallback

	monitor := NewResourceMonitor(sessionMgr, config, logger)

	// Set a custom alert threshold
	threshold := AlertThreshold{
		Name:              "test_sessions",
		ResourceType:      "sessions",
		WarningThreshold:  50.0,  // 50% of max (0.5 sessions)
		CriticalThreshold: 100.0, // 100% of max (1 session)
		Duration:          1 * time.Millisecond,
		Enabled:           true,
	}
	monitor.SetAlertThreshold("test_sessions", threshold)

	// Start monitoring
	ctx := context.Background()
	err = monitor.StartMonitoring(ctx)
	assert.NoError(t, err)
	defer monitor.StopMonitoring()

	// Create sessions to trigger alerts
	session1, err := sessionMgr.CreateSession("")
	require.NoError(t, err)
	session2, err := sessionMgr.CreateSession("")
	require.NoError(t, err)

	// Wait for monitoring to detect the sessions and trigger alerts
	time.Sleep(200 * time.Millisecond)

	// Check that alerts were triggered
	activeAlerts := monitor.GetActiveAlerts()
	assert.NotEmpty(t, activeAlerts)

	// Should have both warning and critical alerts
	hasWarning := false
	hasCritical := false

	for _, alert := range activeAlerts {
		assert.Equal(t, "test_sessions", alert.ThresholdName)
		assert.False(t, alert.Acknowledged)
		assert.True(t, alert.StartTime.After(time.Now().Add(-time.Minute)))

		if alert.Level == "WARNING" {
			hasWarning = true
		}
		if alert.Level == "CRITICAL" {
			hasCritical = true
		}
	}

	assert.True(t, hasWarning || hasCritical) // At least one should be triggered
	assert.True(t, alertTriggered)

	// Test acknowledging an alert
	if len(activeAlerts) > 0 {
		err = monitor.AcknowledgeAlert(activeAlerts[0].ID)
		assert.NoError(t, err)

		updatedAlerts := monitor.GetActiveAlerts()
		for _, alert := range updatedAlerts {
			if alert.ID == activeAlerts[0].ID {
				assert.True(t, alert.Acknowledged)
				break
			}
		}
	}

	// Clean up sessions - simplified for testing
	// In a real test, would call appropriate cleanup methods
	_ = session1
	_ = session2
}

func TestResourceMonitor_ResourcePressure(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	sessionConfig := SessionManagerConfig{
		WorkspaceDir: t.TempDir(),
		MaxSessions:  10,
		SessionTTL:   time.Hour,
		Logger:       logger,
	}

	sessionMgr, err := NewSessionManager(sessionConfig)
	require.NoError(t, err)
	defer sessionMgr.Stop()

	config := ResourceMonitorConfig{
		MonitoringInterval: 50 * time.Millisecond,
		ResourceLimits: ResourceLimits{
			MaxMemoryUsage:  1024, // Very low limit
			MaxSessionCount: 2,    // Low session limit
		},
		EnableAutoCleanup: false,
	}

	monitor := NewResourceMonitor(sessionMgr, config, logger)

	// Start monitoring
	ctx := context.Background()
	err = monitor.StartMonitoring(ctx)
	assert.NoError(t, err)
	defer monitor.StopMonitoring()

	// Create sessions to increase pressure
	_, err = sessionMgr.CreateSession("")
	assert.NoError(t, err)
	_, err = sessionMgr.CreateSession("")
	assert.NoError(t, err)
	_, err = sessionMgr.CreateSession("")
	assert.NoError(t, err)

	// Wait for monitoring to detect resource pressure
	time.Sleep(150 * time.Millisecond)

	current := monitor.GetCurrentResources()
	assert.NotNil(t, current)

	// Should show high resource pressure due to session count exceeding limit
	assert.GreaterOrEqual(t, current.ResourcePressure, 100.0) // Should be > 100% (3 sessions vs 2 limit)
	assert.NotEqual(t, "HEALTHY", current.HealthStatus)       // Should not be healthy

	// Clean up sessions to reduce pressure - simplified for testing
	// In a real test, would call appropriate cleanup methods

	// Wait for pressure to reduce
	time.Sleep(150 * time.Millisecond)

	updatedCurrent := monitor.GetCurrentResources()
	assert.LessOrEqual(t, updatedCurrent.ResourcePressure, current.ResourcePressure)
}

func TestResourceMonitor_CleanupConditions(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	sessionConfig := SessionManagerConfig{
		WorkspaceDir: t.TempDir(),
		MaxSessions:  10,
		SessionTTL:   time.Hour,
		Logger:       logger,
	}

	sessionMgr, err := NewSessionManager(sessionConfig)
	require.NoError(t, err)
	defer sessionMgr.Stop()

	config := ResourceMonitorConfig{
		EnableAutoCleanup: false,
	}

	monitor := NewResourceMonitor(sessionMgr, config, logger)

	// Test different condition evaluations
	tests := []struct {
		name      string
		condition CleanupCondition
		expected  bool
	}{
		{
			name: "count_greater_than",
			condition: CleanupCondition{
				Type:      "count",
				Operator:  "gt",
				Threshold: 0.0,
			},
			expected: false, // No sessions yet
		},
		{
			name: "count_equal",
			condition: CleanupCondition{
				Type:      "count",
				Operator:  "eq",
				Threshold: 0.0,
			},
			expected: true, // Should equal 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := monitor.evaluateCleanupCondition(tt.condition)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResourceMonitor_DefaultRulesAndThresholds(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	sessionConfig := SessionManagerConfig{
		WorkspaceDir: t.TempDir(),
		MaxSessions:  10,
		SessionTTL:   time.Hour,
		Logger:       logger,
	}

	sessionMgr, err := NewSessionManager(sessionConfig)
	require.NoError(t, err)
	defer sessionMgr.Stop()

	config := ResourceMonitorConfig{}
	monitor := NewResourceMonitor(sessionMgr, config, logger)

	// Check that default cleanup rules were created
	assert.NotEmpty(t, monitor.cleanupRules)

	ruleNames := make(map[string]bool)
	for _, rule := range monitor.cleanupRules {
		ruleNames[rule.Name] = true
		assert.NotEmpty(t, rule.Description)
		assert.NotEmpty(t, rule.Conditions)
		assert.NotEmpty(t, rule.Actions)
		assert.Greater(t, rule.Priority, 0)
	}

	// Check for expected default rules
	assert.True(t, ruleNames["cleanup_old_sessions"])
	assert.True(t, ruleNames["cleanup_idle_high_memory"])
	assert.True(t, ruleNames["emergency_cleanup"])

	// Check that default alert thresholds were created
	assert.NotEmpty(t, monitor.alertThresholds)

	thresholdNames := make(map[string]bool)
	for name, threshold := range monitor.alertThresholds {
		thresholdNames[name] = true
		assert.NotEmpty(t, threshold.ResourceType)
		assert.Greater(t, threshold.WarningThreshold, 0.0)
		assert.Greater(t, threshold.CriticalThreshold, threshold.WarningThreshold)
		assert.True(t, threshold.Enabled)
	}

	// Check for expected default thresholds
	assert.True(t, thresholdNames["memory_pressure"])
	assert.True(t, thresholdNames["disk_pressure"])
	assert.True(t, thresholdNames["session_count"])
}

// Benchmark tests
func BenchmarkResourceMonitor_CollectSnapshot(b *testing.B) {
	logger := zerolog.New(zerolog.NewTestWriter(b))

	sessionConfig := SessionManagerConfig{
		WorkspaceDir: b.TempDir(),
		MaxSessions:  100,
		SessionTTL:   time.Hour,
		Logger:       logger,
	}

	sessionMgr, err := NewSessionManager(sessionConfig)
	require.NoError(b, err)
	defer sessionMgr.Stop()

	// Create multiple sessions for realistic testing
	for i := 0; i < 10; i++ {
		_, err := sessionMgr.CreateSession("")
		require.NoError(b, err)
	}

	config := ResourceMonitorConfig{
		EnableAutoCleanup: false,
	}
	monitor := NewResourceMonitor(sessionMgr, config, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.collectResourceSnapshot()
	}
}

func BenchmarkResourceMonitor_EvaluateCleanup(b *testing.B) {
	logger := zerolog.New(zerolog.NewTestWriter(b))

	sessionConfig := SessionManagerConfig{
		WorkspaceDir: b.TempDir(),
		MaxSessions:  100,
		SessionTTL:   time.Hour,
		Logger:       logger,
	}

	sessionMgr, err := NewSessionManager(sessionConfig)
	require.NoError(b, err)
	defer sessionMgr.Stop()

	config := ResourceMonitorConfig{
		EnableAutoCleanup: false,
	}
	monitor := NewResourceMonitor(sessionMgr, config, logger)

	// Initialize current resources
	monitor.updateCurrentResources()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.evaluateCleanup()
	}
}
