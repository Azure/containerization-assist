package session

import (
	"context"
	"strings"
	"time"

	errors "github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/rs/zerolog"
)

// SessionQuery defines criteria for querying sessions
type SessionQuery struct {
	// Label-based filters
	Labels    []string          // Sessions that have ALL these labels
	AnyLabels []string          // Sessions that have ANY of these labels
	K8sLabels map[string]string // Sessions with these K8s labels

	// Time-based filters
	CreatedAfter   *time.Time
	CreatedBefore  *time.Time
	AccessedAfter  *time.Time
	AccessedBefore *time.Time
	ExpiresAfter   *time.Time
	ExpiresBefore  *time.Time

	// State-based filters
	LastErrorExists bool // Sessions that have a last error
	ActiveJobsOnly  bool // Sessions with active jobs
	HasRepoAnalysis bool // Sessions with repository analysis

	// Pagination
	Limit  int
	Offset int

	// Sorting
	SortBy    string // "created", "accessed", "expires"
	SortOrder string // "asc", "desc"
}

// QueryResult contains the results of a session query
type QueryResult struct {
	Sessions   []*SessionState
	TotalCount int
	HasMore    bool
	Query      SessionQuery
	ExecutedAt time.Time
	Duration   time.Duration
}

// SessionQueryManager provides session querying capabilities
type SessionQueryManager struct {
	sessionManager *SessionManager
	labelIndex     *LabelIndex
	logger         zerolog.Logger
}

// NewSessionQueryManager creates a new session query manager
func NewSessionQueryManager(sessionManager *SessionManager, logger zerolog.Logger) *SessionQueryManager {
	return &SessionQueryManager{
		sessionManager: sessionManager,
		labelIndex:     NewLabelIndex(logger),
		logger:         logger.With().Str("component", "query_manager").Logger(),
	}
}

// QuerySessions executes a query and returns matching sessions
func (qm *SessionQueryManager) QuerySessions(query SessionQuery) ([]*SessionState, error) {
	startTime := time.Now()

	qm.logger.Debug().
		Interface("query", query).
		Msg("Executing session query")

	// Get all sessions to filter
	allSessions, err := qm.getAllSessions()
	if err != nil {
		return nil, errors.NewError().Message("failed to get sessions").Cause(err).WithLocation(

		// Apply filters
		).Build()
	}

	var matchingSessions []*SessionState
	for _, session := range allSessions {
		if qm.sessionMatchesQuery(session, query) {
			matchingSessions = append(matchingSessions, session)
		}
	}

	// Apply sorting
	qm.sortSessions(matchingSessions, query.SortBy, query.SortOrder)

	// Apply pagination
	start := query.Offset
	if start < 0 {
		start = 0
	}
	if start > len(matchingSessions) {
		start = len(matchingSessions)
	}

	end := start + query.Limit
	if query.Limit <= 0 || end > len(matchingSessions) {
		end = len(matchingSessions)
	}

	result := matchingSessions[start:end]

	qm.logger.Info().
		Int("total_sessions", len(allSessions)).
		Int("matching_sessions", len(matchingSessions)).
		Int("returned_sessions", len(result)).
		Dur("duration", time.Since(startTime)).
		Msg("Session query completed")

	return result, nil
}

// CountSessions returns the count of sessions matching the query
func (qm *SessionQueryManager) CountSessions(query SessionQuery) (int, error) {
	qm.logger.Debug().
		Interface("query", query).
		Msg("Counting sessions for query")

	allSessions, err := qm.getAllSessions()
	if err != nil {
		return 0, errors.NewError().Message("failed to get sessions").Cause(err).Build()
	}

	count := 0
	for _, session := range allSessions {
		if qm.sessionMatchesQuery(session, query) {
			count++
		}
	}

	return count, nil
}

// QuerySessionIDs returns only the session IDs matching the query
func (qm *SessionQueryManager) QuerySessionIDs(query SessionQuery) ([]string, error) {
	sessions, err := qm.QuerySessions(query)
	if err != nil {
		return nil, err
	}

	sessionIDs := make([]string, len(sessions))
	for i, session := range sessions {
		sessionIDs[i] = session.SessionID
	}

	return sessionIDs, nil
}

// GetSessionsByLabelPrefix returns sessions that have labels with the specified prefix
func (qm *SessionQueryManager) GetSessionsByLabelPrefix(prefix string) ([]*SessionState, error) {
	qm.logger.Debug().
		Str("prefix", prefix).
		Msg("Getting sessions by label prefix")

	allSessions, err := qm.getAllSessions()
	if err != nil {
		return nil, errors.NewError().Message("failed to get sessions").Cause(err).Build()
	}

	var matchingSessions []*SessionState
	for _, session := range allSessions {
		for _, label := range session.Labels {
			if strings.HasPrefix(label, prefix) {
				matchingSessions = append(matchingSessions, session)
				break // Found one match, no need to check other labels
			}
		}
	}

	return matchingSessions, nil
}

// GetSessionsWithAnyLabel returns sessions that have any of the specified labels
func (qm *SessionQueryManager) GetSessionsWithAnyLabel(labels []string) ([]*SessionState, error) {
	query := SessionQuery{
		AnyLabels: labels,
	}
	return qm.QuerySessions(query)
}

// GetSessionsWithAllLabels returns sessions that have all of the specified labels
func (qm *SessionQueryManager) GetSessionsWithAllLabels(labels []string) ([]*SessionState, error) {
	query := SessionQuery{
		Labels: labels,
	}
	return qm.QuerySessions(query)
}

// sessionMatchesQuery checks if a session matches the given query criteria
// Predicate represents a function that tests if a session matches certain criteria
type Predicate func(session *SessionState, query SessionQuery) bool

// sessionMatchesQuery checks if a session matches the query using predicate functions
func (qm *SessionQueryManager) sessionMatchesQuery(session *SessionState, query SessionQuery) bool {
	predicates := []Predicate{
		qm.checkRequiredLabels,
		qm.checkAnyLabels,
		qm.checkK8sLabels,
		qm.checkTimeFilters,
		qm.checkStateFilters,
	}

	for _, predicate := range predicates {
		if !predicate(session, query) {
			return false
		}
	}

	return true
}

// checkRequiredLabels verifies that all required labels are present
func (qm *SessionQueryManager) checkRequiredLabels(session *SessionState, query SessionQuery) bool {
	if len(query.Labels) == 0 {
		return true
	}

	sessionLabels := qm.buildLabelMap(session.Labels)
	for _, requiredLabel := range query.Labels {
		if !sessionLabels[requiredLabel] {
			return false
		}
	}
	return true
}

// checkAnyLabels verifies that at least one of the specified labels is present
func (qm *SessionQueryManager) checkAnyLabels(session *SessionState, query SessionQuery) bool {
	if len(query.AnyLabels) == 0 {
		return true
	}

	sessionLabels := qm.buildLabelMap(session.Labels)
	for _, anyLabel := range query.AnyLabels {
		if sessionLabels[anyLabel] {
			return true
		}
	}
	return false
}

// checkK8sLabels verifies that all K8s labels match
func (qm *SessionQueryManager) checkK8sLabels(session *SessionState, query SessionQuery) bool {
	if len(query.K8sLabels) == 0 {
		return true
	}

	if session.K8sLabels == nil {
		return false
	}

	for key, value := range query.K8sLabels {
		if sessionValue, exists := session.K8sLabels[key]; !exists || sessionValue != value {
			return false
		}
	}
	return true
}

// checkTimeFilters verifies all time-based filters
func (qm *SessionQueryManager) checkTimeFilters(session *SessionState, query SessionQuery) bool {
	timeChecks := []func() bool{
		func() bool {
			return query.CreatedAfter == nil || !session.CreatedAt.Before(*query.CreatedAfter)
		},
		func() bool {
			return query.CreatedBefore == nil || !session.CreatedAt.After(*query.CreatedBefore)
		},
		func() bool {
			return query.AccessedAfter == nil || !session.LastAccessed.Before(*query.AccessedAfter)
		},
		func() bool {
			return query.AccessedBefore == nil || !session.LastAccessed.After(*query.AccessedBefore)
		},
		func() bool {
			return query.ExpiresAfter == nil || !session.ExpiresAt.Before(*query.ExpiresAfter)
		},
		func() bool {
			return query.ExpiresBefore == nil || !session.ExpiresAt.After(*query.ExpiresBefore)
		},
	}

	for _, check := range timeChecks {
		if !check() {
			return false
		}
	}
	return true
}

// checkStateFilters verifies all state-based filters
func (qm *SessionQueryManager) checkStateFilters(session *SessionState, query SessionQuery) bool {
	stateChecks := []func() bool{
		func() bool {
			return !query.LastErrorExists || session.LastError != nil
		},
		func() bool {
			return !query.ActiveJobsOnly || len(session.ActiveJobs) > 0
		},
		func() bool {
			return !query.HasRepoAnalysis || len(session.RepoAnalysis) > 0
		},
	}

	for _, check := range stateChecks {
		if !check() {
			return false
		}
	}
	return true
}

// buildLabelMap creates a map for efficient label lookup
func (qm *SessionQueryManager) buildLabelMap(labels []string) map[string]bool {
	labelMap := make(map[string]bool)
	for _, label := range labels {
		labelMap[label] = true
	}
	return labelMap
}

// sortSessions sorts sessions based on the specified criteria
func (qm *SessionQueryManager) sortSessions(sessions []*SessionState, sortBy, sortOrder string) {
	if len(sessions) <= 1 {
		return
	}

	// Default sorting
	if sortBy == "" {
		sortBy = "created"
	}
	if sortOrder == "" {
		sortOrder = "desc"
	}

	// Simple bubble sort for small datasets (can be optimized later)
	n := len(sessions)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			var shouldSwap bool

			switch sortBy {
			case "created":
				if sortOrder == "asc" {
					shouldSwap = sessions[j].CreatedAt.After(sessions[j+1].CreatedAt)
				} else {
					shouldSwap = sessions[j].CreatedAt.Before(sessions[j+1].CreatedAt)
				}
			case "accessed":
				if sortOrder == "asc" {
					shouldSwap = sessions[j].LastAccessed.After(sessions[j+1].LastAccessed)
				} else {
					shouldSwap = sessions[j].LastAccessed.Before(sessions[j+1].LastAccessed)
				}
			case "expires":
				if sortOrder == "asc" {
					shouldSwap = sessions[j].ExpiresAt.After(sessions[j+1].ExpiresAt)
				} else {
					shouldSwap = sessions[j].ExpiresAt.Before(sessions[j+1].ExpiresAt)
				}
			default:
				// Default to created time
				if sortOrder == "asc" {
					shouldSwap = sessions[j].CreatedAt.After(sessions[j+1].CreatedAt)
				} else {
					shouldSwap = sessions[j].CreatedAt.Before(sessions[j+1].CreatedAt)
				}
			}

			if shouldSwap {
				sessions[j], sessions[j+1] = sessions[j+1], sessions[j]
			}
		}
	}
}

// getAllSessions gets all sessions from the session manager
func (qm *SessionQueryManager) getAllSessions() ([]*SessionState, error) {
	// This is a simplified implementation - in a production system,
	// we would want to optimize this to avoid loading all sessions into memory
	sessionSummaries, err := qm.sessionManager.ListSessionSummaries(context.Background())
	if err != nil {
		return nil, err
	}

	var sessions []*SessionState
	for _, summary := range sessionSummaries {
		session, err := qm.sessionManager.GetSessionConcrete(summary.ID)
		if err != nil {
			qm.logger.Warn().
				Str("session_id", summary.ID).
				Err(err).
				Msg("Failed to load session, skipping")
			continue
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// BuildWorkflowQuery creates a query for common workflow patterns
func BuildWorkflowQuery(stage string, env string) SessionQuery {
	var labels []string

	if stage != "" {
		labels = append(labels, "workflow.stage/"+stage)
	}

	if env != "" {
		labels = append(labels, "env:"+env)
	}

	return SessionQuery{
		Labels:    labels,
		SortBy:    "accessed",
		SortOrder: "desc",
		Limit:     50,
	}
}

// BuildFailedSessionsQuery creates a query for failed sessions
func BuildFailedSessionsQuery() SessionQuery {
	return SessionQuery{
		AnyLabels:       []string{"workflow.stage/failed", "status:error"},
		LastErrorExists: true,
		SortBy:          "accessed",
		SortOrder:       "desc",
		Limit:           20,
	}
}

// BuildActiveSessionsQuery creates a query for active sessions
func BuildActiveSessionsQuery() SessionQuery {
	return SessionQuery{
		ActiveJobsOnly: true,
		SortBy:         "accessed",
		SortOrder:      "desc",
		Limit:          100,
	}
}
