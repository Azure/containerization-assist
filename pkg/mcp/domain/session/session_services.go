package session

import "context"

// Services provides access to all session-related services
type Services interface {
	// Operations returns the session operations service
	Operations() Operations

	// Query returns the session query service
	Query() Query

	// JobTracker returns the job tracking service
	JobTracker() JobTracker

	// ToolTracker returns the tool tracking service
	ToolTracker() ToolTracker

	// ErrorTracker returns the error tracking service
	ErrorTracker() ErrorTracker

	// Lifecycle returns the lifecycle management service
	Lifecycle() Lifecycle
}

// sessionServices implements Services
type sessionServices struct {
	operations   Operations
	query        Query
	jobTracker   JobTracker
	toolTracker  ToolTracker
	errorTracker ErrorTracker
	lifecycle    Lifecycle
}

// NewSessionServices creates a new Services container from a Manager
// This allows gradual migration from the old Manager interface
func NewSessionServices(manager Manager) Services {
	// Create adapters that wrap the manager
	return &sessionServices{
		operations:   &sessionOperationsAdapter{manager: manager},
		query:        &sessionQueryAdapter{manager: manager},
		jobTracker:   &sessionJobTrackerAdapter{manager: manager},
		toolTracker:  &sessionToolTrackerAdapter{manager: manager},
		errorTracker: &sessionErrorTrackerAdapter{manager: manager},
		lifecycle:    &sessionLifecycleAdapter{manager: manager},
	}
}

func (s *sessionServices) Operations() Operations {
	return s.operations
}

func (s *sessionServices) Query() Query {
	return s.query
}

func (s *sessionServices) JobTracker() JobTracker {
	return s.jobTracker
}

func (s *sessionServices) ToolTracker() ToolTracker {
	return s.toolTracker
}

func (s *sessionServices) ErrorTracker() ErrorTracker {
	return s.errorTracker
}

func (s *sessionServices) Lifecycle() Lifecycle {
	return s.lifecycle
}

// Adapter implementations to wrap the old Manager

type sessionOperationsAdapter struct {
	manager Manager
}

func (a *sessionOperationsAdapter) GetSession(sessionID string) (*SessionState, error) {
	return a.manager.GetSession(sessionID)
}

func (a *sessionOperationsAdapter) GetOrCreateSession(sessionID string) (*SessionState, error) {
	return a.manager.GetOrCreateSession(sessionID)
}

func (a *sessionOperationsAdapter) UpdateSession(ctx context.Context, sessionID string, updateFunc func(*SessionState) error) error {
	return a.manager.UpdateSession(ctx, sessionID, updateFunc)
}

func (a *sessionOperationsAdapter) DeleteSession(sessionID string) error {
	return a.manager.DeleteSession(sessionID)
}

type sessionQueryAdapter struct {
	manager Manager
}

func (a *sessionQueryAdapter) ListSessions() ([]*SessionState, error) {
	return a.manager.ListSessionsTyped()
}

func (a *sessionQueryAdapter) ListSessionSummaries() ([]*SessionSummary, error) {
	return a.manager.ListSessionSummaries()
}

func (a *sessionQueryAdapter) GetSessionData(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	return a.manager.GetSessionData(ctx, sessionID)
}

type sessionJobTrackerAdapter struct {
	manager Manager
}

func (a *sessionJobTrackerAdapter) StartJob(sessionID string, jobType string) (string, error) {
	return a.manager.StartJob(sessionID, jobType)
}

func (a *sessionJobTrackerAdapter) UpdateJobStatus(sessionID string, jobID string, status JobStatus, result interface{}, err error) error {
	return a.manager.UpdateJobStatus(sessionID, jobID, status, result, err)
}

func (a *sessionJobTrackerAdapter) CompleteJob(sessionID string, jobID string, result interface{}) error {
	return a.manager.CompleteJob(sessionID, jobID, result)
}

type sessionToolTrackerAdapter struct {
	manager Manager
}

func (a *sessionToolTrackerAdapter) TrackToolExecution(sessionID string, toolName string, args interface{}) error {
	return a.manager.TrackToolExecution(sessionID, toolName, args)
}

func (a *sessionToolTrackerAdapter) CompleteToolExecution(sessionID string, toolName string, success bool, err error, tokensUsed int) error {
	return a.manager.CompleteToolExecution(sessionID, toolName, success, err, tokensUsed)
}

type sessionErrorTrackerAdapter struct {
	manager Manager
}

func (a *sessionErrorTrackerAdapter) TrackError(sessionID string, err error, context interface{}) error {
	return a.manager.TrackError(sessionID, err, context)
}

type sessionLifecycleAdapter struct {
	manager Manager
}

func (a *sessionLifecycleAdapter) StartCleanupRoutine() {
	a.manager.StartCleanupRoutine()
}

func (a *sessionLifecycleAdapter) Stop() error {
	return a.manager.Stop()
}
