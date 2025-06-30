package session

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// SessionAnalytics provides advanced analytics for session management
type SessionAnalytics struct {
	logger         zerolog.Logger
	sessionManager *SessionManager
	mutex          sync.RWMutex

	// Analytics data
	sessionMetrics map[string]*SessionMetrics
	teamMetrics    map[string]*TeamMetrics
	globalMetrics  *GlobalMetrics

	// Time-series data
	hourlyStats []HourlyStats
	dailyStats  []DailyStats

	// Health scoring
	healthScorer *HealthScorer

	// Configuration
	retentionDays  int
	analysisWindow time.Duration
}

// SessionMetrics tracks detailed metrics for individual sessions
type SessionMetrics struct {
	SessionID     string `json:"session_id"`
	TeamName      string `json:"team_name"`
	ComponentName string `json:"component_name"`

	// Time tracking
	CreatedAt      time.Time     `json:"created_at"`
	LastActivity   time.Time     `json:"last_activity"`
	TotalDuration  time.Duration `json:"total_duration"`
	ActiveDuration time.Duration `json:"active_duration"`

	// Operation metrics
	ToolExecutions int     `json:"tool_executions"`
	SuccessfulOps  int     `json:"successful_ops"`
	FailedOps      int     `json:"failed_ops"`
	SuccessRate    float64 `json:"success_rate"`

	// Resource metrics
	PeakMemoryUsage int64 `json:"peak_memory_usage"`
	DiskUsage       int64 `json:"disk_usage"`
	NetworkOps      int   `json:"network_ops"`

	// Performance metrics
	AvgOperationTime time.Duration `json:"avg_operation_time"`
	P95OperationTime time.Duration `json:"p95_operation_time"`
	SlowestOperation string        `json:"slowest_operation"`

	// Health indicators
	HealthScore float64 `json:"health_score"`
	HealthTrend string  `json:"health_trend"` // "IMPROVING", "STABLE", "DEGRADING"
	AlertLevel  string  `json:"alert_level"`  // "GREEN", "YELLOW", "RED"

	// Error analysis
	ErrorCategories map[string]int `json:"error_categories"`
	CriticalErrors  []ErrorEvent   `json:"critical_errors"`

	// Usage patterns
	PeakHours    []int  `json:"peak_hours"`
	UsagePattern string `json:"usage_pattern"` // "BURST", "STEADY", "IDLE"
}

// TeamMetrics aggregates metrics across all sessions for a team
type TeamMetrics struct {
	TeamName string `json:"team_name"`

	// Session statistics
	TotalSessions      int           `json:"total_sessions"`
	ActiveSessions     int           `json:"active_sessions"`
	AvgSessionDuration time.Duration `json:"avg_session_duration"`

	// Performance aggregates
	TotalOperations    int     `json:"total_operations"`
	OverallSuccessRate float64 `json:"overall_success_rate"`
	AvgHealthScore     float64 `json:"avg_health_score"`

	// Resource utilization
	TotalResourceUsage ResourceUsage `json:"total_resource_usage"`
	ResourceEfficiency float64       `json:"resource_efficiency"`

	// Trend analysis
	WeeklyTrend      TrendData `json:"weekly_trend"`
	PerformanceTrend string    `json:"performance_trend"`

	// Top issues
	TopErrors     []ErrorSummary `json:"top_errors"`
	BottleneckOps []string       `json:"bottleneck_ops"`

	// Quality indicators
	QualityScore     float64 `json:"quality_score"`
	ReliabilityScore float64 `json:"reliability_score"`
}

// GlobalMetrics provides system-wide analytics
type GlobalMetrics struct {
	Timestamp time.Time `json:"timestamp"`

	// System health
	SystemHealthScore float64 `json:"system_health_score"`
	OverallStatus     string  `json:"overall_status"`

	// Capacity metrics
	SessionCapacity  CapacityMetrics `json:"session_capacity"`
	ResourceCapacity CapacityMetrics `json:"resource_capacity"`

	// Performance metrics
	SystemThroughput float64       `json:"system_throughput"`
	P95ResponseTime  time.Duration `json:"p95_response_time"`
	ErrorRate        float64       `json:"error_rate"`

	// Trend indicators
	GrowthRate        float64        `json:"growth_rate"`
	PerformanceTrend  string         `json:"performance_trend"`
	PredictedCapacity PredictionData `json:"predicted_capacity"`
}

// Supporting types
type ErrorEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Category  string    `json:"category"`
	Message   string    `json:"message"`
	Severity  string    `json:"severity"`
	Context   string    `json:"context"`
}

type ErrorSummary struct {
	Category     string    `json:"category"`
	Count        int       `json:"count"`
	LastOccurred time.Time `json:"last_occurred"`
	Trend        string    `json:"trend"`
}

type ResourceUsage struct {
	Memory  int64   `json:"memory_bytes"`
	Disk    int64   `json:"disk_bytes"`
	Network int64   `json:"network_bytes"`
	CPU     float64 `json:"cpu_percent"`
}

type TrendData struct {
	Direction  string  `json:"direction"` // "UP", "DOWN", "STABLE"
	ChangeRate float64 `json:"change_rate"`
	Confidence float64 `json:"confidence"`
	DataPoints int     `json:"data_points"`
}

type CapacityMetrics struct {
	Current     int            `json:"current"`
	Maximum     int            `json:"maximum"`
	Utilization float64        `json:"utilization"`
	TimeToMax   *time.Duration `json:"time_to_max,omitempty"`
}

type PredictionData struct {
	NextWeek   float64 `json:"next_week"`
	NextMonth  float64 `json:"next_month"`
	Confidence float64 `json:"confidence"`
	TrendBasis string  `json:"trend_basis"`
}

type HourlyStats struct {
	Hour       int           `json:"hour"`
	Sessions   int           `json:"sessions"`
	Operations int           `json:"operations"`
	Errors     int           `json:"errors"`
	AvgLatency time.Duration `json:"avg_latency"`
}

type DailyStats struct {
	Date        time.Time `json:"date"`
	Sessions    int       `json:"sessions"`
	Operations  int       `json:"operations"`
	SuccessRate float64   `json:"success_rate"`
	HealthScore float64   `json:"health_score"`
}

// HealthScorer calculates health scores based on multiple factors
type HealthScorer struct {
	weights    map[string]float64
	thresholds map[string]ThresholdSet
}

type ThresholdSet struct {
	Excellent float64
	Good      float64
	Fair      float64
	Poor      float64
}

// NewSessionAnalytics creates a new session analytics engine
func NewSessionAnalytics(sessionManager *SessionManager, logger zerolog.Logger) *SessionAnalytics {
	analytics := &SessionAnalytics{
		logger:         logger.With().Str("component", "session_analytics").Logger(),
		sessionManager: sessionManager,
		sessionMetrics: make(map[string]*SessionMetrics),
		teamMetrics:    make(map[string]*TeamMetrics),
		globalMetrics:  &GlobalMetrics{},
		retentionDays:  30,
		analysisWindow: 24 * time.Hour,
		healthScorer:   newHealthScorer(),
	}

	// Initialize time-series data
	analytics.initializeTimeSeriesData()

	return analytics
}

func newHealthScorer() *HealthScorer {
	return &HealthScorer{
		weights: map[string]float64{
			"success_rate":    0.25,
			"performance":     0.20,
			"resource_usage":  0.15,
			"error_frequency": 0.15,
			"uptime":          0.10,
			"responsiveness":  0.10,
			"stability":       0.05,
		},
		thresholds: map[string]ThresholdSet{
			"success_rate": {
				Excellent: 99.0,
				Good:      95.0,
				Fair:      90.0,
				Poor:      80.0,
			},
			"performance": {
				Excellent: 100.0, // microseconds
				Good:      250.0,
				Fair:      500.0,
				Poor:      1000.0,
			},
			"resource_usage": {
				Excellent: 70.0, // percentage
				Good:      80.0,
				Fair:      90.0,
				Poor:      95.0,
			},
		},
	}
}

// UpdateSessionMetrics updates metrics for a specific session
func (sa *SessionAnalytics) UpdateSessionMetrics(sessionID string) error {
	sa.mutex.Lock()
	defer sa.mutex.Unlock()

	// Get session data from session manager
	sessionData, err := sa.sessionManager.GetSessionData(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session data: %w", err)
	}

	// Calculate or update session metrics
	metrics := sa.calculateSessionMetrics(sessionData)
	sa.sessionMetrics[sessionID] = metrics

	// Update team metrics
	sa.updateTeamMetrics(metrics.TeamName)

	// Update global metrics
	sa.updateGlobalMetrics()

	sa.logger.Debug().
		Str("session_id", sessionID).
		Float64("health_score", metrics.HealthScore).
		Str("alert_level", metrics.AlertLevel).
		Msg("Updated session metrics")

	return nil
}

// GetSessionAnalytics returns detailed analytics for a session
func (sa *SessionAnalytics) GetSessionAnalytics(sessionID string) (*SessionMetrics, error) {
	sa.mutex.RLock()
	defer sa.mutex.RUnlock()

	metrics, exists := sa.sessionMetrics[sessionID]
	if !exists {
		return nil, fmt.Errorf("session metrics not found: %s", sessionID)
	}

	return metrics, nil
}

// GetTeamAnalytics returns aggregated analytics for a team
func (sa *SessionAnalytics) GetTeamAnalytics(teamName string) (*TeamMetrics, error) {
	sa.mutex.RLock()
	defer sa.mutex.RUnlock()

	metrics, exists := sa.teamMetrics[teamName]
	if !exists {
		return nil, fmt.Errorf("team metrics not found: %s", teamName)
	}

	return metrics, nil
}

// GetGlobalAnalytics returns system-wide analytics
func (sa *SessionAnalytics) GetGlobalAnalytics() *GlobalMetrics {
	sa.mutex.RLock()
	defer sa.mutex.RUnlock()

	// Create a copy to avoid concurrent access
	globalCopy := *sa.globalMetrics
	return &globalCopy
}

// GenerateAnalyticsReport creates a comprehensive analytics report
func (sa *SessionAnalytics) GenerateAnalyticsReport() *AnalyticsReport {
	sa.mutex.RLock()
	defer sa.mutex.RUnlock()

	report := &AnalyticsReport{
		GeneratedAt:     time.Now(),
		GlobalMetrics:   *sa.globalMetrics,
		TeamSummaries:   make([]TeamSummary, 0, len(sa.teamMetrics)),
		TopSessions:     sa.getTopPerformingSessions(10),
		SystemInsights:  sa.generateSystemInsights(),
		Recommendations: sa.generateRecommendations(),
	}

	// Add team summaries
	for _, team := range sa.teamMetrics {
		summary := TeamSummary{
			TeamName:    team.TeamName,
			HealthScore: team.QualityScore,
			Sessions:    team.TotalSessions,
			SuccessRate: team.OverallSuccessRate,
			Trend:       team.PerformanceTrend,
			TopIssues:   team.TopErrors[:min(3, len(team.TopErrors))],
		}
		report.TeamSummaries = append(report.TeamSummaries, summary)
	}

	return report
}

// Private methods for calculations

func (sa *SessionAnalytics) calculateSessionMetrics(sessionData *SessionData) *SessionMetrics {
	metrics := &SessionMetrics{
		SessionID:       sessionData.ID,
		CreatedAt:       sessionData.CreatedAt,
		LastActivity:    sessionData.UpdatedAt,
		DiskUsage:       sessionData.DiskUsage,
		ErrorCategories: make(map[string]int),
		PeakHours:       make([]int, 0),
	}

	// Extract team and component info
	if teamName, ok := sessionData.Metadata["team_name"]; ok {
		metrics.TeamName = fmt.Sprintf("%v", teamName)
	}
	if componentName, ok := sessionData.Metadata["component_name"]; ok {
		metrics.ComponentName = fmt.Sprintf("%v", componentName)
	}

	// Calculate time metrics
	metrics.TotalDuration = time.Since(sessionData.CreatedAt)

	// Calculate operation metrics from session state
	if sessionData.State != nil {
		state := sessionData.State.(*SessionState)

		metrics.ToolExecutions = len(state.StageHistory)
		successCount := 0

		var operationTimes []time.Duration

		for _, execution := range state.StageHistory {
			if execution.Success {
				successCount++
			}

			if execution.Duration != nil {
				operationTimes = append(operationTimes, *execution.Duration)
			}

			// Categorize errors
			if execution.Error != nil {
				category := categorizeError(execution.Error.Message)
				metrics.ErrorCategories[category]++
			}
		}

		metrics.SuccessfulOps = successCount
		metrics.FailedOps = metrics.ToolExecutions - successCount

		if metrics.ToolExecutions > 0 {
			metrics.SuccessRate = float64(successCount) / float64(metrics.ToolExecutions) * 100
		}

		// Calculate performance metrics
		if len(operationTimes) > 0 {
			metrics.AvgOperationTime = calculateAverage(operationTimes)
			metrics.P95OperationTime = calculatePercentile(operationTimes, 95)
		}
	}

	// Calculate health score
	metrics.HealthScore = sa.healthScorer.calculateHealthScore(metrics)
	metrics.AlertLevel = sa.determineAlertLevel(metrics.HealthScore)
	metrics.HealthTrend = sa.calculateHealthTrend(metrics.SessionID, metrics.HealthScore)

	// Determine usage pattern
	metrics.UsagePattern = sa.determineUsagePattern(metrics)

	return metrics
}

func (sa *SessionAnalytics) updateTeamMetrics(teamName string) {
	if teamName == "" {
		return
	}

	teamMetrics := &TeamMetrics{
		TeamName:      teamName,
		TopErrors:     make([]ErrorSummary, 0),
		BottleneckOps: make([]string, 0),
	}

	// Aggregate metrics from all team sessions
	sessionCount := 0
	totalDuration := time.Duration(0)
	totalOps := 0
	successfulOps := 0
	totalHealthScore := 0.0

	for _, metrics := range sa.sessionMetrics {
		if metrics.TeamName == teamName {
			sessionCount++
			totalDuration += metrics.TotalDuration
			totalOps += metrics.ToolExecutions
			successfulOps += metrics.SuccessfulOps
			totalHealthScore += metrics.HealthScore
		}
	}

	if sessionCount > 0 {
		teamMetrics.TotalSessions = sessionCount
		teamMetrics.AvgSessionDuration = totalDuration / time.Duration(sessionCount)
		teamMetrics.TotalOperations = totalOps
		teamMetrics.AvgHealthScore = totalHealthScore / float64(sessionCount)

		if totalOps > 0 {
			teamMetrics.OverallSuccessRate = float64(successfulOps) / float64(totalOps) * 100
		}
	}

	// Calculate quality and reliability scores
	teamMetrics.QualityScore = sa.calculateQualityScore(teamMetrics)
	teamMetrics.ReliabilityScore = sa.calculateReliabilityScore(teamMetrics)

	sa.teamMetrics[teamName] = teamMetrics
}

func (sa *SessionAnalytics) updateGlobalMetrics() {
	globalMetrics := &GlobalMetrics{
		Timestamp: time.Now(),
	}

	// Calculate system-wide metrics
	totalSessions := len(sa.sessionMetrics)
	totalOps := 0
	successfulOps := 0
	totalHealthScore := 0.0

	for _, metrics := range sa.sessionMetrics {
		totalOps += metrics.ToolExecutions
		successfulOps += metrics.SuccessfulOps
		totalHealthScore += metrics.HealthScore
	}

	if totalSessions > 0 {
		globalMetrics.SystemHealthScore = totalHealthScore / float64(totalSessions)
		globalMetrics.OverallStatus = sa.determineSystemStatus(globalMetrics.SystemHealthScore)
	}

	if totalOps > 0 {
		globalMetrics.ErrorRate = float64(totalOps-successfulOps) / float64(totalOps) * 100
	}

	// Calculate throughput (operations per hour)
	if sa.analysisWindow > 0 {
		globalMetrics.SystemThroughput = float64(totalOps) / sa.analysisWindow.Hours()
	}

	sa.globalMetrics = globalMetrics
}

// Helper functions

func categorizeError(errorMessage string) string {
	message := fmt.Sprintf("%s", errorMessage)
	switch {
	case analyticsContains(message, "authentication", "auth", "login", "token"):
		return "AUTHENTICATION"
	case analyticsContains(message, "network", "connection", "timeout", "unreachable"):
		return "NETWORK"
	case analyticsContains(message, "permission", "access", "forbidden", "unauthorized"):
		return "PERMISSION"
	case analyticsContains(message, "resource", "memory", "disk", "cpu"):
		return "RESOURCE"
	case analyticsContains(message, "docker", "container", "image"):
		return "DOCKER"
	case analyticsContains(message, "validation", "invalid", "format"):
		return "VALIDATION"
	default:
		return "GENERAL"
	}
}

func analyticsContains(str string, substrings ...string) bool {
	for _, substr := range substrings {
		if len(str) >= len(substr) {
			for i := 0; i <= len(str)-len(substr); i++ {
				if str[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

func calculateAverage(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	total := time.Duration(0)
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}

func calculatePercentile(durations []time.Duration, percentile int) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	index := int(math.Ceil(float64(percentile)/100.0*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

func (hs *HealthScorer) calculateHealthScore(metrics *SessionMetrics) float64 {
	score := 0.0

	// Success rate component
	if weight, ok := hs.weights["success_rate"]; ok {
		successScore := math.Min(metrics.SuccessRate/100.0, 1.0)
		score += weight * successScore
	}

	// Performance component (inverse of latency)
	if weight, ok := hs.weights["performance"]; ok {
		perfScore := 1.0
		if metrics.AvgOperationTime > 0 {
			// Convert to microseconds and invert (lower is better)
			latencyUs := float64(metrics.AvgOperationTime.Microseconds())
			perfScore = math.Max(0, 1.0-(latencyUs/1000.0)) // Normalized against 1ms
		}
		score += weight * perfScore
	}

	// Resource usage component (assume reasonable usage)
	if weight, ok := hs.weights["resource_usage"]; ok {
		// This would need actual resource monitoring to be meaningful
		resourceScore := 0.8 // Default reasonable score
		score += weight * resourceScore
	}

	// Error frequency component
	if weight, ok := hs.weights["error_frequency"]; ok {
		errorScore := 1.0
		if metrics.ToolExecutions > 0 {
			errorRate := float64(metrics.FailedOps) / float64(metrics.ToolExecutions)
			errorScore = math.Max(0, 1.0-errorRate)
		}
		score += weight * errorScore
	}

	return math.Min(score*100, 100.0) // Convert to 0-100 scale
}

func (sa *SessionAnalytics) determineAlertLevel(healthScore float64) string {
	switch {
	case healthScore >= 90:
		return "GREEN"
	case healthScore >= 70:
		return "YELLOW"
	default:
		return "RED"
	}
}

func (sa *SessionAnalytics) calculateHealthTrend(sessionID string, currentScore float64) string {
	// This would require historical data to determine trend
	// For now, return stable as default
	return "STABLE"
}

func (sa *SessionAnalytics) determineUsagePattern(metrics *SessionMetrics) string {
	// Analyze operation frequency and timing
	if metrics.ToolExecutions == 0 {
		return "IDLE"
	}

	if metrics.TotalDuration > 0 {
		opsPerHour := float64(metrics.ToolExecutions) / metrics.TotalDuration.Hours()
		switch {
		case opsPerHour > 10:
			return "BURST"
		case opsPerHour > 1:
			return "STEADY"
		default:
			return "IDLE"
		}
	}

	return "STEADY"
}

func (sa *SessionAnalytics) calculateQualityScore(team *TeamMetrics) float64 {
	// Combine multiple quality factors
	score := 0.0
	factors := 0

	if team.OverallSuccessRate > 0 {
		score += team.OverallSuccessRate
		factors++
	}

	if team.AvgHealthScore > 0 {
		score += team.AvgHealthScore
		factors++
	}

	if factors > 0 {
		return score / float64(factors)
	}

	return 50.0 // Default score
}

func (sa *SessionAnalytics) calculateReliabilityScore(team *TeamMetrics) float64 {
	// Calculate reliability based on consistency and error rates
	if team.TotalOperations == 0 {
		return 100.0
	}

	return team.OverallSuccessRate // Simplified reliability score
}

func (sa *SessionAnalytics) determineSystemStatus(healthScore float64) string {
	switch {
	case healthScore >= 95:
		return "EXCELLENT"
	case healthScore >= 85:
		return "GOOD"
	case healthScore >= 70:
		return "FAIR"
	case healthScore >= 50:
		return "POOR"
	default:
		return "CRITICAL"
	}
}

func (sa *SessionAnalytics) initializeTimeSeriesData() {
	// Initialize with empty time series data
	sa.hourlyStats = make([]HourlyStats, 24)
	sa.dailyStats = make([]DailyStats, sa.retentionDays)

	// Initialize hourly stats
	for i := 0; i < 24; i++ {
		sa.hourlyStats[i] = HourlyStats{Hour: i}
	}

	// Initialize daily stats
	for i := 0; i < sa.retentionDays; i++ {
		date := time.Now().AddDate(0, 0, -i)
		sa.dailyStats[i] = DailyStats{Date: date}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Additional types for reporting

type AnalyticsReport struct {
	GeneratedAt     time.Time                 `json:"generated_at"`
	GlobalMetrics   GlobalMetrics             `json:"global_metrics"`
	TeamSummaries   []TeamSummary             `json:"team_summaries"`
	TopSessions     []AnalyticsSessionSummary `json:"top_sessions"`
	SystemInsights  []Insight                 `json:"system_insights"`
	Recommendations []Recommendation          `json:"recommendations"`
}

type TeamSummary struct {
	TeamName    string         `json:"team_name"`
	HealthScore float64        `json:"health_score"`
	Sessions    int            `json:"sessions"`
	SuccessRate float64        `json:"success_rate"`
	Trend       string         `json:"trend"`
	TopIssues   []ErrorSummary `json:"top_issues"`
}

type AnalyticsSessionSummary struct {
	SessionID   string        `json:"session_id"`
	TeamName    string        `json:"team_name"`
	HealthScore float64       `json:"health_score"`
	Operations  int           `json:"operations"`
	Duration    time.Duration `json:"duration"`
}

type Insight struct {
	Category    string `json:"category"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Impact      string `json:"impact"`
}

type Recommendation struct {
	Priority        string   `json:"priority"`
	Category        string   `json:"category"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Actions         []string `json:"actions"`
	EstimatedImpact string   `json:"estimated_impact"`
}

// Placeholder methods for future implementation
func (sa *SessionAnalytics) getTopPerformingSessions(limit int) []AnalyticsSessionSummary {
	// Return top performing sessions based on health score
	return []AnalyticsSessionSummary{}
}

func (sa *SessionAnalytics) generateSystemInsights() []Insight {
	// Generate insights based on analytics data
	return []Insight{}
}

func (sa *SessionAnalytics) generateRecommendations() []Recommendation {
	// Generate actionable recommendations
	return []Recommendation{}
}
