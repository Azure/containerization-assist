//go:build test
// +build test

// Package wire provides test utilities for Wire dependency injection
package wire

import (
	"context"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/session"
	"github.com/Azure/container-kit/pkg/mcp/infrastructure/sampling"
)

// MockSessionManager for testing
type MockSessionManager struct{}

func (m *MockSessionManager) GetSession(ctx context.Context, sessionID string) (*session.SessionState, error) {
	return &session.SessionState{
		SessionID: sessionID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
		Status:    "active",
		Stage:     "test",
		UserID:    "test-user",
		Labels:    map[string]string{"test": "true"},
		Metadata:  map[string]interface{}{"repo_url": "https://github.com/example/test"},
	}, nil
}

func (m *MockSessionManager) GetOrCreateSession(ctx context.Context, sessionID string) (*session.SessionState, error) {
	return m.GetSession(ctx, sessionID)
}

func (m *MockSessionManager) UpdateSession(ctx context.Context, sessionID string, updateFunc func(*session.SessionState) error) error {
	state, err := m.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	return updateFunc(state)
}

func (m *MockSessionManager) Stop(ctx context.Context) error {
	return nil
}

func (m *MockSessionManager) GetStats() (*session.SessionStats, error) {
	return &session.SessionStats{
		ActiveSessions: 1,
		TotalSessions:  1,
		MaxSessions:    10,
	}, nil
}

// Legacy methods (implemented for compatibility)
func (m *MockSessionManager) GetSessionTyped(ctx context.Context, sessionID string) (*session.SessionState, error) {
	return m.GetSession(ctx, sessionID)
}

func (m *MockSessionManager) GetSessionConcrete(ctx context.Context, sessionID string) (*session.SessionState, error) {
	return m.GetSession(ctx, sessionID)
}

func (m *MockSessionManager) GetOrCreateSessionTyped(ctx context.Context, sessionID string) (*session.SessionState, error) {
	return m.GetOrCreateSession(ctx, sessionID)
}

func (m *MockSessionManager) ListSessionsTyped(ctx context.Context) ([]*session.SessionState, error) {
	state, err := m.GetSession(ctx, "test-session")
	if err != nil {
		return nil, err
	}
	return []*session.SessionState{state}, nil
}

func (m *MockSessionManager) ListSessionSummaries(ctx context.Context) ([]*session.SessionSummary, error) {
	return []*session.SessionSummary{{
		ID:     "test-session",
		Labels: map[string]string{"test": "true"},
	}}, nil
}

func (m *MockSessionManager) UpdateJobStatus(ctx context.Context, sessionID, jobID string, status session.JobStatus, result interface{}, err error) error {
	return nil
}

func (m *MockSessionManager) StartCleanupRoutine(ctx context.Context) error {
	return nil
}

// MockSamplingClient for testing
type MockSamplingClient struct{}

func (m *MockSamplingClient) Sample(ctx context.Context, req sampling.SamplingRequest) (*sampling.SamplingResponse, error) {
	return &sampling.SamplingResponse{
		Content:    "Mock AI response for testing",
		TokensUsed: 10,
		Model:      "test-model",
		StopReason: "stop",
		Error:      nil,
	}, nil
}

// TestConfigSet removed to avoid circular imports - tests can use main ConfigSet

// NewMockSessionManager creates a mock session manager for testing
func NewMockSessionManager() session.SessionManager {
	return &MockSessionManager{}
}

// NewMockSamplingClient creates a mock sampling client for testing
func NewMockSamplingClient() *MockSamplingClient {
	return &MockSamplingClient{}
}

// Note: For full test injection, tests can use the main ProviderSet
// and substitute individual dependencies as needed, or use the wire_entry.go
// public API with test configurations.
