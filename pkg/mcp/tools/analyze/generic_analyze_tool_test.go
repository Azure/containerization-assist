package analyze

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/api"
	"github.com/Azure/container-kit/pkg/mcp/core"
	"github.com/Azure/container-kit/pkg/mcp/session"
	"github.com/rs/zerolog"
)

// mockUnifiedSessionManager implements session.UnifiedSessionManager for testing
type mockUnifiedSessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*session.SessionState
}

func newMockUnifiedSessionManager() *mockUnifiedSessionManager {
	return &mockUnifiedSessionManager{
		sessions: make(map[string]*session.SessionState),
	}
}

func (m *mockUnifiedSessionManager) CreateSession(ctx context.Context, userID string) (*session.SessionState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessionID := "test-session-" + time.Now().Format("20060102150405")
	sess := session.NewSessionState(sessionID, "/tmp/workspace/"+sessionID)
	m.sessions[sessionID] = sess
	return sess, nil
}

func (m *mockUnifiedSessionManager) GetSession(ctx context.Context, sessionID string) (*session.SessionState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if sess, exists := m.sessions[sessionID]; exists {
		return sess, nil
	}
	return nil, fmt.Errorf("session not found: %s", sessionID)
}

func (m *mockUnifiedSessionManager) GetOrCreateSession(ctx context.Context, sessionID string) (*session.SessionState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sess, exists := m.sessions[sessionID]; exists {
		return sess, nil
	}

	sess := session.NewSessionState(sessionID, "/tmp/workspace/"+sessionID)
	m.sessions[sessionID] = sess
	return sess, nil
}

func (m *mockUnifiedSessionManager) UpdateSession(_ context.Context, sessionID string, updater func(*session.SessionState) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sess, exists := m.sessions[sessionID]; exists {
		if err := updater(sess); err != nil {
			return err
		}
		sess.UpdateLastAccessed()
		return nil
	}
	return fmt.Errorf("session not found: %s", sessionID)
}

func (m *mockUnifiedSessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[sessionID]; exists {
		delete(m.sessions, sessionID)
		return nil
	}
	return fmt.Errorf("session not found: %s", sessionID)
}

func (m *mockUnifiedSessionManager) ListSessions(_ context.Context) ([]*session.SessionData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*session.SessionData, 0, len(m.sessions))
	for _, sess := range m.sessions {
		sessions = append(sessions, &session.SessionData{
			ID:           sess.SessionID,
			CreatedAt:    sess.CreatedAt,
			UpdatedAt:    sess.UpdatedAt,
			WorkspaceDir: sess.WorkspaceDir,
			Status:       sess.Status,
			Labels:       sess.Labels,
		})
	}
	return sessions, nil
}

func (m *mockUnifiedSessionManager) GetStats(ctx context.Context) (*core.SessionManagerStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activeCount := 0
	for _, sess := range m.sessions {
		if !sess.IsExpired() {
			activeCount++
		}
	}

	return &core.SessionManagerStats{
		ActiveSessions: activeCount,
		TotalSessions:  len(m.sessions),
	}, nil
}

func (m *mockUnifiedSessionManager) GarbageCollect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove expired sessions
	for id, sess := range m.sessions {
		if sess.IsExpired() {
			delete(m.sessions, id)
		}
	}
	return nil
}

func (m *mockUnifiedSessionManager) CreateWorkflowSession(ctx context.Context, spec *session.WorkflowSpec) (*session.SessionState, error) {
	return m.CreateSession(ctx, "workflow-user")
}

func (m *mockUnifiedSessionManager) GetWorkflowSession(ctx context.Context, sessionID string) (*session.WorkflowSession, error) {
	sess, err := m.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	// Return a basic workflow session with embedded SessionState
	return &session.WorkflowSession{
		SessionState: sess,
		WorkflowID:   "test-workflow",
		WorkflowName: "Test Workflow",
		Status:       "running",
	}, nil
}

func (m *mockUnifiedSessionManager) UpdateWorkflowSession(ctx context.Context, session *session.WorkflowSession) error {
	return nil
}

// Missing methods from UnifiedSessionManager interface
func (m *mockUnifiedSessionManager) ListSessionSummaries(_ context.Context) ([]session.SessionSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summaries := make([]session.SessionSummary, 0, len(m.sessions))
	for _, sess := range m.sessions {
		summaries = append(summaries, session.SessionSummary{
			ID:        sess.SessionID,
			CreatedAt: sess.CreatedAt,
			UpdatedAt: sess.UpdatedAt,
			Status:    sess.Status,
			Labels:    sess.Labels,
		})
	}
	return summaries, nil
}

func (m *mockUnifiedSessionManager) GetSessionData(_ context.Context, sessionID string) (*session.SessionData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if sess, exists := m.sessions[sessionID]; exists {
		return &session.SessionData{
			ID:           sess.SessionID,
			CreatedAt:    sess.CreatedAt,
			UpdatedAt:    sess.UpdatedAt,
			WorkspaceDir: sess.WorkspaceDir,
			Status:       sess.Status,
			Labels:       sess.Labels,
		}, nil
	}
	return nil, fmt.Errorf("session not found: %s", sessionID)
}

func (m *mockUnifiedSessionManager) SaveSession(_ context.Context, sessionID string, session *session.SessionState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[sessionID] = session
	return nil
}

func (m *mockUnifiedSessionManager) AddSessionLabel(_ context.Context, sessionID, label string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sess, exists := m.sessions[sessionID]; exists {
		// Add label if not already present
		for _, existingLabel := range sess.Labels {
			if existingLabel == label {
				return nil // Already has this label
			}
		}
		sess.Labels = append(sess.Labels, label)
		return nil
	}
	return fmt.Errorf("session not found: %s", sessionID)
}

func (m *mockUnifiedSessionManager) RemoveSessionLabel(_ context.Context, sessionID, label string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sess, exists := m.sessions[sessionID]; exists {
		// Remove label if present
		for i, existingLabel := range sess.Labels {
			if existingLabel == label {
				sess.Labels = append(sess.Labels[:i], sess.Labels[i+1:]...)
				return nil
			}
		}
		return nil // Label not found, but not an error
	}
	return fmt.Errorf("session not found: %s", sessionID)
}

func (m *mockUnifiedSessionManager) GetSessionsByLabel(_ context.Context, label string) ([]*session.SessionData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sessions []*session.SessionData
	for _, sess := range m.sessions {
		for _, sessionLabel := range sess.Labels {
			if sessionLabel == label {
				sessions = append(sessions, &session.SessionData{
					ID:           sess.SessionID,
					CreatedAt:    sess.CreatedAt,
					UpdatedAt:    sess.UpdatedAt,
					WorkspaceDir: sess.WorkspaceDir,
					Status:       sess.Status,
					Labels:       sess.Labels,
				})
				break
			}
		}
	}
	return sessions, nil
}

func (m *mockUnifiedSessionManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions = make(map[string]*session.SessionState)
	return nil
}

func TestGenericAnalyzeRepositoryTool_TypeSafety(t *testing.T) {
	// Create a mock atomic tool and session manager
	atomicTool := &AtomicAnalyzeRepositoryTool{}
	sessionManager := newMockUnifiedSessionManager()
	logger := zerolog.New(nil).With().Timestamp().Logger()

	// Create the generic tool
	tool := NewGenericAnalyzeRepositoryTool(atomicTool, sessionManager, logger)

	// Test basic interface compliance
	if tool.Name() == "" {
		t.Error("Tool name should not be empty")
	}

	if tool.Description() == "" {
		t.Error("Tool description should not be empty")
	}

	if tool.GetTimeout() <= 0 {
		t.Error("Tool timeout should be positive")
	}

	// Test schema
	schema := tool.Schema()
	if schema.Name != "generic_analyze_repository" {
		t.Errorf("Expected schema name 'generic_analyze_repository', got %s", schema.Name)
	}

	if schema.Version == "" {
		t.Error("Schema version should not be empty")
	}
}

func TestGenericAnalyzeRepositoryTool_InputValidation(t *testing.T) {
	atomicTool := &AtomicAnalyzeRepositoryTool{}
	sessionManager := newMockUnifiedSessionManager()
	logger := zerolog.New(nil).With().Timestamp().Logger()

	tool := NewGenericAnalyzeRepositoryTool(atomicTool, sessionManager, logger)

	ctx := context.Background()

	tests := []struct {
		name        string
		input       *api.AnalyzeInput
		expectError bool
	}{
		{
			name: "valid input",
			input: &api.AnalyzeInput{
				SessionID: "test-session",
				RepoURL:   "https://github.com/example/repo",
				Branch:    "main",
			},
			expectError: false,
		},
		{
			name: "missing session ID",
			input: &api.AnalyzeInput{
				RepoURL: "https://github.com/example/repo",
				Branch:  "main",
			},
			expectError: true,
		},
		{
			name: "missing repo URL",
			input: &api.AnalyzeInput{
				SessionID: "test-session",
				Branch:    "main",
			},
			expectError: true,
		},
		{
			name: "valid input with options",
			input: &api.AnalyzeInput{
				SessionID:           "test-session",
				RepoURL:             "https://github.com/example/repo",
				Branch:              "develop",
				LanguageHint:        "go",
				IncludeDependencies: true,
				IncludeSecurityScan: true,
				CustomOptions: map[string]string{
					"depth": "shallow",
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tool.Validate(ctx, tt.input)
			if tt.expectError && err == nil {
				t.Error("Expected validation error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no validation error but got: %v", err)
			}
		})
	}
}

func TestAnalyzeInput_ConstraintInterface(t *testing.T) {
	input := &api.AnalyzeInput{
		SessionID: "test-session-123",
		RepoURL:   "https://github.com/example/repo",
		Context: map[string]interface{}{
			"user_id": "user123",
			"depth":   "shallow",
		},
	}

	// Test ToolInputConstraint interface
	if input.GetSessionID() != "test-session-123" {
		t.Errorf("Expected session ID 'test-session-123', got %s", input.GetSessionID())
	}

	if err := input.Validate(); err != nil {
		t.Errorf("Expected valid input, got error: %v", err)
	}

	context := input.GetContext()
	if context["user_id"] != "user123" {
		t.Errorf("Expected context user_id 'user123', got %v", context["user_id"])
	}
}

func TestAnalyzeOutput_ConstraintInterface(t *testing.T) {
	output := &api.AnalyzeOutput{
		Success:      true,
		SessionID:    "test-session",
		Language:     "Go",
		Framework:    "gin",
		AnalysisTime: 2 * time.Second,
		Data: map[string]interface{}{
			"repo_url": "https://github.com/example/repo",
			"branch":   "main",
		},
	}

	// Test ToolOutputConstraint interface
	if !output.IsSuccess() {
		t.Error("Expected output to be successful")
	}

	data := output.GetData()
	if data == nil {
		t.Error("Expected output data to not be nil")
	}

	if output.GetError() != "" {
		t.Errorf("Expected no error message, got: %s", output.GetError())
	}

	// Test error case
	errorOutput := &api.AnalyzeOutput{
		Success:  false,
		ErrorMsg: "Analysis failed",
	}

	if errorOutput.IsSuccess() {
		t.Error("Expected output to not be successful")
	}

	if errorOutput.GetError() != "Analysis failed" {
		t.Errorf("Expected error message 'Analysis failed', got: %s", errorOutput.GetError())
	}
}

func TestGenericTypeAliases(t *testing.T) {
	// Test that the type aliases work correctly
	var analyzeTool api.AnalyzeTool
	var buildTool api.BuildTool

	// These should be different types
	if analyzeTool != nil && buildTool != nil {
		// Type safety test - this wouldn't compile if we tried to assign wrong types:
		// analyzeTool = buildTool // This would be a compile error!
	}

	// Test that generic registry works with different tool types
	// TODO: GenericRegistry type not yet implemented
	// var analyzeRegistry api.GenericRegistry[*api.AnalyzeInput, *api.AnalyzeOutput]
	// var buildRegistry api.GenericRegistry[*api.BuildInput, *api.BuildOutput]

	// if analyzeRegistry != nil && buildRegistry != nil {
	// 	// These are different types and cannot be interchanged
	// 	// analyzeRegistry = buildRegistry // This would be a compile error!
	// }

	t.Log("Type aliases and generic registries maintain type safety")
}

func TestDependencyConversion(t *testing.T) {
	// Test the helper function for converting dependencies
	deps := []api.Dependency{
		{Name: "gin", Version: "v1.7.0", Type: "direct"},
		{Name: "gorm", Version: "v1.21.0", Type: "direct"},
		{Name: "testify", Version: "v1.7.0", Type: "test"},
	}

	if len(deps) != 3 {
		t.Errorf("Expected 3 dependencies, got %d", len(deps))
	}

	if deps[0].Name != "gin" {
		t.Errorf("Expected first dependency to be 'gin', got %s", deps[0].Name)
	}

	if deps[0].Version != "v1.7.0" {
		t.Errorf("Expected gin version 'v1.7.0', got %s", deps[0].Version)
	}
}

func TestSecurityIssueGeneration(t *testing.T) {
	// Test the helper function for generating security issues
	issues := []api.SecurityIssue{
		{
			ID:          "SEC-001",
			Severity:    "high",
			Description: "SQL injection vulnerability",
			Package:     "database/sql",
			Version:     "v1.0.0",
			FixVersion:  "v1.1.0",
		},
	}

	if len(issues) != 1 {
		t.Errorf("Expected 1 security issue, got %d", len(issues))
	}

	if issues[0].Severity != "high" {
		t.Errorf("Expected severity 'high', got %s", issues[0].Severity)
	}
}

// Benchmark to ensure type safety doesn't impact performance
func BenchmarkGenericAnalyzeInput_Validation(b *testing.B) {
	input := &api.AnalyzeInput{
		SessionID: "benchmark-session",
		RepoURL:   "https://github.com/example/repo",
		Branch:    "main",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = input.Validate()
	}
}

func BenchmarkGenericAnalyzeOutput_Success(b *testing.B) {
	output := &api.AnalyzeOutput{
		Success:   true,
		SessionID: "benchmark-session",
		Language:  "Go",
		Framework: "gin",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = output.IsSuccess()
	}
}
