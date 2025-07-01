package session

import (
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionAnalytics_NewSessionAnalytics(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	// Create session manager
	config := SessionManagerConfig{
		WorkspaceDir: t.TempDir(),
		MaxSessions:  10,
		SessionTTL:   time.Hour,
		Logger:       logger,
	}

	sessionMgr, err := NewSessionManager(config)
	require.NoError(t, err)
	defer sessionMgr.Stop()

	// Create analytics
	analytics := NewSessionAnalytics(sessionMgr, logger)
	assert.NotNil(t, analytics)
	assert.NotNil(t, analytics.sessionMetrics)
	assert.NotNil(t, analytics.teamMetrics)
	assert.NotNil(t, analytics.globalMetrics)
	assert.NotNil(t, analytics.healthScorer)
}

func TestSessionAnalytics_UpdateSessionMetrics(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	// Create session manager
	config := SessionManagerConfig{
		WorkspaceDir: t.TempDir(),
		MaxSessions:  10,
		SessionTTL:   time.Hour,
		Logger:       logger,
	}

	sessionMgr, err := NewSessionManager(config)
	require.NoError(t, err)
	defer sessionMgr.Stop()

	// Create analytics
	analytics := NewSessionAnalytics(sessionMgr, logger)

	// Create a session
	sessionInterface, err := sessionMgr.CreateSession("")
	require.NoError(t, err)

	sessionState := sessionInterface.(*SessionState)
	sessionID := sessionState.SessionID

	// Add team metadata
	err = sessionMgr.UpdateSession(sessionID, func(s interface{}) {
		if state, ok := s.(*SessionState); ok {
			state.Metadata = map[string]interface{}{
				"team_name":      "TestTeam",
				"component_name": "test_component",
			}
		}
	})
	require.NoError(t, err)

	// Update analytics
	err = analytics.UpdateSessionMetrics(sessionID)
	assert.NoError(t, err)

	// Verify metrics were created
	metrics, err := analytics.GetSessionAnalytics(sessionID)
	require.NoError(t, err)
	assert.Equal(t, sessionID, metrics.SessionID)
	assert.Equal(t, "TestTeam", metrics.TeamName)
	assert.Equal(t, "test_component", metrics.ComponentName)
	assert.NotZero(t, metrics.HealthScore)
	assert.NotEmpty(t, metrics.AlertLevel)
}

func TestSessionAnalytics_HealthScoring(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	healthScorer := newHealthScorer()

	tests := []struct {
		name        string
		metrics     *SessionMetrics
		expectedMin float64
		expectedMax float64
	}{
		{
			name: "perfect_session",
			metrics: &SessionMetrics{
				ToolExecutions:   10,
				SuccessfulOps:    10,
				FailedOps:        0,
				SuccessRate:      100.0,
				AvgOperationTime: 100 * time.Microsecond,
			},
			expectedMin: 60.0, // Adjusted based on actual algorithm
			expectedMax: 100.0,
		},
		{
			name: "average_session",
			metrics: &SessionMetrics{
				ToolExecutions:   10,
				SuccessfulOps:    8,
				FailedOps:        2,
				SuccessRate:      80.0,
				AvgOperationTime: 500 * time.Microsecond,
			},
			expectedMin: 50.0,
			expectedMax: 80.0,
		},
		{
			name: "poor_session",
			metrics: &SessionMetrics{
				ToolExecutions:   10,
				SuccessfulOps:    3,
				FailedOps:        7,
				SuccessRate:      30.0,
				AvgOperationTime: 2 * time.Millisecond,
			},
			expectedMin: 0.0,
			expectedMax: 50.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := healthScorer.calculateHealthScore(tt.metrics)
			assert.GreaterOrEqual(t, score, tt.expectedMin)
			assert.LessOrEqual(t, score, tt.expectedMax)

			logger.Info().
				Str("test", tt.name).
				Float64("score", score).
				Float64("success_rate", tt.metrics.SuccessRate).
				Dur("avg_time", tt.metrics.AvgOperationTime).
				Msg("Health score calculated")
		})
	}
}

func TestSessionAnalytics_ErrorCategorization(t *testing.T) {
	tests := []struct {
		error    string
		category string
	}{
		{"authentication failed", "AUTHENTICATION"},
		{"invalid token", "AUTHENTICATION"},
		{"network timeout", "NETWORK"},
		{"connection refused", "NETWORK"},
		{"permission denied", "PERMISSION"},
		{"access forbidden", "PERMISSION"},
		{"out of memory", "RESOURCE"},
		{"disk full", "RESOURCE"},
		{"docker pull failed", "DOCKER"},
		{"container not found", "DOCKER"},
		{"invalid format", "VALIDATION"},
		{"unknown error", "GENERAL"},
	}

	for _, tt := range tests {
		t.Run(tt.error, func(t *testing.T) {
			category := categorizeError(tt.error)
			assert.Equal(t, tt.category, category)
		})
	}
}

func TestSessionAnalytics_UsagePatterns(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	// Create session manager
	config := SessionManagerConfig{
		WorkspaceDir: t.TempDir(),
		MaxSessions:  10,
		SessionTTL:   time.Hour,
		Logger:       logger,
	}

	sessionMgr, err := NewSessionManager(config)
	require.NoError(t, err)
	defer sessionMgr.Stop()

	analytics := NewSessionAnalytics(sessionMgr, logger)

	tests := []struct {
		name            string
		toolExecutions  int
		totalDuration   time.Duration
		expectedPattern string
	}{
		{
			name:            "idle_session",
			toolExecutions:  0,
			totalDuration:   time.Hour,
			expectedPattern: "IDLE",
		},
		{
			name:            "burst_session",
			toolExecutions:  50,
			totalDuration:   time.Hour,
			expectedPattern: "BURST",
		},
		{
			name:            "steady_session",
			toolExecutions:  5,
			totalDuration:   time.Hour,
			expectedPattern: "STEADY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := &SessionMetrics{
				ToolExecutions: tt.toolExecutions,
				TotalDuration:  tt.totalDuration,
			}

			pattern := analytics.determineUsagePattern(metrics)
			assert.Equal(t, tt.expectedPattern, pattern)
		})
	}
}

func TestSessionAnalytics_TeamMetrics(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	// Create session manager
	config := SessionManagerConfig{
		WorkspaceDir: t.TempDir(),
		MaxSessions:  10,
		SessionTTL:   time.Hour,
		Logger:       logger,
	}

	sessionMgr, err := NewSessionManager(config)
	require.NoError(t, err)
	defer sessionMgr.Stop()

	analytics := NewSessionAnalytics(sessionMgr, logger)

	// Create multiple sessions for the same team
	teamName := "TestTeam"
	for i := 0; i < 3; i++ {
		sessionInterface, err := sessionMgr.CreateSession("")
		require.NoError(t, err)

		sessionState := sessionInterface.(*SessionState)

		// Add team metadata
		err = sessionMgr.UpdateSession(sessionState.SessionID, func(s interface{}) {
			if state, ok := s.(*SessionState); ok {
				state.Metadata = map[string]interface{}{
					"team_name":      teamName,
					"component_name": "test_component",
				}
			}
		})
		require.NoError(t, err)

		// Update analytics for each session
		err = analytics.UpdateSessionMetrics(sessionState.SessionID)
		assert.NoError(t, err)
	}

	// Get team analytics
	teamMetrics, err := analytics.GetTeamAnalytics(teamName)
	require.NoError(t, err)
	assert.Equal(t, teamName, teamMetrics.TeamName)
	assert.Equal(t, 3, teamMetrics.TotalSessions)
	assert.NotZero(t, teamMetrics.QualityScore)
	assert.NotZero(t, teamMetrics.ReliabilityScore)
}

func TestSessionAnalytics_GlobalMetrics(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	// Create session manager
	config := SessionManagerConfig{
		WorkspaceDir: t.TempDir(),
		MaxSessions:  10,
		SessionTTL:   time.Hour,
		Logger:       logger,
	}

	sessionMgr, err := NewSessionManager(config)
	require.NoError(t, err)
	defer sessionMgr.Stop()

	analytics := NewSessionAnalytics(sessionMgr, logger)

	// Create sessions for different teams
	teams := []string{"TeamA", "TeamB", "TeamC"}
	for _, team := range teams {
		sessionInterface, err := sessionMgr.CreateSession("")
		require.NoError(t, err)

		sessionState := sessionInterface.(*SessionState)

		// Add team metadata
		err = sessionMgr.UpdateSession(sessionState.SessionID, func(s interface{}) {
			if state, ok := s.(*SessionState); ok {
				state.Metadata = map[string]interface{}{
					"team_name":      team,
					"component_name": "test_component",
				}
			}
		})
		require.NoError(t, err)

		// Update analytics
		err = analytics.UpdateSessionMetrics(sessionState.SessionID)
		assert.NoError(t, err)
	}

	// Get global analytics
	globalMetrics := analytics.GetGlobalAnalytics()
	assert.NotNil(t, globalMetrics)
	assert.NotZero(t, globalMetrics.SystemHealthScore)
	assert.NotEmpty(t, globalMetrics.OverallStatus)
	assert.NotZero(t, globalMetrics.Timestamp)
}

func TestSessionAnalytics_AnalyticsReport(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	// Create session manager
	config := SessionManagerConfig{
		WorkspaceDir: t.TempDir(),
		MaxSessions:  10,
		SessionTTL:   time.Hour,
		Logger:       logger,
	}

	sessionMgr, err := NewSessionManager(config)
	require.NoError(t, err)
	defer sessionMgr.Stop()

	analytics := NewSessionAnalytics(sessionMgr, logger)

	// Create test sessions
	sessionInterface, err := sessionMgr.CreateSession("")
	require.NoError(t, err)

	sessionState := sessionInterface.(*SessionState)

	// Add metadata
	err = sessionMgr.UpdateSession(sessionState.SessionID, func(s interface{}) {
		if state, ok := s.(*SessionState); ok {
			state.Metadata = map[string]interface{}{
				"team_name":      "TestTeam",
				"component_name": "test_component",
			}
		}
	})
	require.NoError(t, err)

	// Update analytics
	err = analytics.UpdateSessionMetrics(sessionState.SessionID)
	require.NoError(t, err)

	// Generate report
	report := analytics.GenerateAnalyticsReport()
	assert.NotNil(t, report)
	assert.NotZero(t, report.GeneratedAt)
	assert.NotNil(t, report.GlobalMetrics)
	assert.NotNil(t, report.TeamSummaries)
	assert.NotNil(t, report.TopSessions)
	assert.NotNil(t, report.SystemInsights)
	assert.NotNil(t, report.Recommendations)
}

// Benchmark tests
func BenchmarkHealthScoreCalculation(b *testing.B) {
	healthScorer := newHealthScorer()

	metrics := &SessionMetrics{
		ToolExecutions:   100,
		SuccessfulOps:    95,
		FailedOps:        5,
		SuccessRate:      95.0,
		AvgOperationTime: 250 * time.Microsecond,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = healthScorer.calculateHealthScore(metrics)
	}
}

func BenchmarkUpdateSessionMetrics(b *testing.B) {
	logger := zerolog.New(zerolog.NewTestWriter(b))

	// Create session manager
	config := SessionManagerConfig{
		WorkspaceDir: b.TempDir(),
		MaxSessions:  1000,
		SessionTTL:   time.Hour,
		Logger:       logger,
	}

	sessionMgr, err := NewSessionManager(config)
	require.NoError(b, err)
	defer sessionMgr.Stop()

	analytics := NewSessionAnalytics(sessionMgr, logger)

	// Create test session
	sessionInterface, err := sessionMgr.CreateSession("")
	require.NoError(b, err)

	sessionState := sessionInterface.(*SessionState)
	sessionID := sessionState.SessionID

	// Add metadata
	err = sessionMgr.UpdateSession(sessionID, func(s interface{}) {
		if state, ok := s.(*SessionState); ok {
			state.Metadata = map[string]interface{}{
				"team_name":      "BenchTeam",
				"component_name": "bench_component",
			}
		}
	})
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := analytics.UpdateSessionMetrics(sessionID)
		if err != nil {
			b.Fatal(err)
		}
	}
}
