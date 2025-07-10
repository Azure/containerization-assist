package pipeline

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application"
	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
)

// TestOperations creates a properly initialized Operations instance for testing
func TestOperations(t *testing.T) *Operations {
	// Create temporary workspace for tests
	tempDir := filepath.Join(os.TempDir(), "pipeline-test-"+t.Name())
	if err := os.MkdirAll(tempDir, 0750); err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	// Create mock session manager
	sessionManager := NewMockSessionManager()

	// Create mock Docker client
	dockerClient := &mockDockerClient{}

	// Create MCP clients
	clients := &application.MCPClients{
		Docker: dockerClient,
	}

	// Create logger
	logger := slog.Default()

	return NewOperations(sessionManager, clients, logger)
}

type MockSessionManager struct {
	sessions map[string]*session.SessionState
}

// NewMockSessionManager creates a new mock session manager
func NewMockSessionManager() *MockSessionManager {
	return &MockSessionManager{
		sessions: make(map[string]*session.SessionState),
	}
}

// GetSession retrieves a session by ID
func (m *MockSessionManager) GetSession(sessionID string) (*session.SessionState, error) {
	if sess, exists := m.sessions[sessionID]; exists {
		return sess, nil
	}
	return nil, errors.NewError().
		Code(errors.CodeNotFound).
		Type(errors.ErrTypeNotFound).
		Messagef("session not found: %s", sessionID).
		WithLocation().
		Build()
}

// GetSessionTyped retrieves a session with type safety
func (m *MockSessionManager) GetSessionTyped(sessionID string) (*session.SessionState, error) {
	return m.GetSession(sessionID)
}

// GetSessionConcrete retrieves a concrete session
func (m *MockSessionManager) GetSessionConcrete(sessionID string) (*session.SessionState, error) {
	return m.GetSession(sessionID)
}

// GetSessionData retrieves session data
func (m *MockSessionManager) GetSessionData(_ context.Context, sessionID string) (map[string]interface{}, error) {
	sess, err := m.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	return sess.Metadata, nil
}

// GetOrCreateSession gets or creates a session
func (m *MockSessionManager) GetOrCreateSession(sessionID string) (*session.SessionState, error) {
	if sess, exists := m.sessions[sessionID]; exists {
		return sess, nil
	}
	sess := &session.SessionState{
		SessionID:    sessionID,
		WorkspaceDir: "/tmp/test",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		Metadata:     make(map[string]interface{}),
	}
	m.sessions[sessionID] = sess
	return sess, nil
}

// GetOrCreateSessionTyped gets or creates a session with type safety
func (m *MockSessionManager) GetOrCreateSessionTyped(sessionID string) (*session.SessionState, error) {
	return m.GetOrCreateSession(sessionID)
}

// UpdateSession updates session state
func (m *MockSessionManager) UpdateSession(_ context.Context, sessionID string, updateFunc func(*session.SessionState) error) error {
	if sess, exists := m.sessions[sessionID]; exists {
		return updateFunc(sess)
	}
	return errors.NewError().
		Code(errors.CodeNotFound).
		Type(errors.ErrTypeNotFound).
		Messagef("session not found: %s", sessionID).
		WithLocation().
		Build()
}

// DeleteSession deletes a session
func (m *MockSessionManager) DeleteSession(sessionID string) error {
	delete(m.sessions, sessionID)
	return nil
}

// ListSessionsTyped lists sessions with type safety
func (m *MockSessionManager) ListSessionsTyped() ([]*session.SessionState, error) {
	var sessions []*session.SessionState
	for _, sess := range m.sessions {
		sessions = append(sessions, sess)
	}
	return sessions, nil
}

// ListSessionSummaries lists session summaries
func (m *MockSessionManager) ListSessionSummaries() ([]*session.SessionSummary, error) {
	return []*session.SessionSummary{}, nil
}

// StartJob starts a new job in the session
func (m *MockSessionManager) StartJob(_ string, _ string) (string, error) {
	return "job-123", nil
}

// UpdateJobStatus updates job status
func (m *MockSessionManager) UpdateJobStatus(_ string, _ string, _ session.JobStatus, _ interface{}, _ error) error {
	return nil
}

// CompleteJob completes a job
func (m *MockSessionManager) CompleteJob(_ string, _ string, _ interface{}) error {
	return nil
}

// TrackToolExecution tracks tool execution
func (m *MockSessionManager) TrackToolExecution(_ string, _ string, _ interface{}) error {
	return nil
}

// CompleteToolExecution completes tool execution
func (m *MockSessionManager) CompleteToolExecution(_ string, _ string, _ bool, _ error, _ int) error {
	return nil
}

// TrackError tracks an error
func (m *MockSessionManager) TrackError(_ string, _ error, _ interface{}) error {
	return nil
}

// StartCleanupRoutine starts cleanup routine
func (m *MockSessionManager) StartCleanupRoutine() {
	// No-op
}

// Stop stops the session manager
func (m *MockSessionManager) Stop() error {
	return nil
}

// mockDockerClient provides a minimal mock implementation for testing
type mockDockerClient struct{}

func (m *mockDockerClient) Version(_ context.Context) (string, error) {
	return "Docker version 20.10.0", nil
}

func (m *mockDockerClient) Info(_ context.Context) (string, error) {
	return "Docker info mock", nil
}

func (m *mockDockerClient) Build(_ context.Context, _, imageTag, _ string) (string, error) {
	// Mock successful build
	return "Successfully built " + imageTag, nil
}

func (m *mockDockerClient) Push(_ context.Context, imageTag string) (string, error) {
	// Mock successful push
	return "Successfully pushed " + imageTag, nil
}

func (m *mockDockerClient) Pull(_ context.Context, imageRef string) (string, error) {
	// Mock successful pull
	return "Successfully pulled " + imageRef, nil
}

func (m *mockDockerClient) Tag(_ context.Context, sourceRef, targetRef string) (string, error) {
	// Mock successful tag
	return "Successfully tagged " + sourceRef + " as " + targetRef, nil
}

func (m *mockDockerClient) Login(_ context.Context, _, _, _ string) (string, error) {
	return "Login succeeded", nil
}

func (m *mockDockerClient) LoginWithToken(_ context.Context, _, _ string) (string, error) {
	return "Login succeeded", nil
}

func (m *mockDockerClient) Logout(_ context.Context, _ string) (string, error) {
	return "Logout succeeded", nil
}

func (m *mockDockerClient) IsLoggedIn(_ context.Context, _ string) (bool, error) {
	return true, nil
}

// createBenchmarkOperations creates an Operations instance suitable for benchmarking
func createBenchmarkOperations(b *testing.B) *Operations {
	// Create temporary workspace for benchmarks
	tempDir := filepath.Join(os.TempDir(), "pipeline-bench-"+b.Name())
	if err := os.MkdirAll(tempDir, 0750); err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	b.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	// Create mock session manager
	sessionManager := NewMockSessionManager()

	// Create mock Docker client
	dockerClient := &mockDockerClient{}

	// Create MCP clients
	clients := &application.MCPClients{
		Docker: dockerClient,
	}

	// Create logger
	logger := slog.Default()

	return NewOperations(sessionManager, clients, logger)
}
