package state

import (
	"context"
	"fmt"
	"sync"

	"github.com/Azure/container-kit/pkg/mcp/internal/session"
)

// SessionStateProvider provides access to session state
type SessionStateProvider struct {
	sessionManager *session.SessionManager
}

// NewSessionStateProvider creates a new session state provider
func NewSessionStateProvider(sessionManager *session.SessionManager) StateProvider {
	return &SessionStateProvider{
		sessionManager: sessionManager,
	}
}

// GetState retrieves session state
func (p *SessionStateProvider) GetState(ctx context.Context, id string) (interface{}, error) {
	return p.sessionManager.GetSession(id)
}

// SetState updates session state
func (p *SessionStateProvider) SetState(ctx context.Context, id string, state interface{}) error {
	sessionState, ok := state.(*session.SessionState)
	if !ok {
		return fmt.Errorf("invalid state type for session provider")
	}
	return p.sessionManager.UpdateSession(id, func(current interface{}) {
		// Update the session state
		if currentState, ok := current.(*session.SessionState); ok {
			*currentState = *sessionState
		}
	})
}

// DeleteState removes session state
func (p *SessionStateProvider) DeleteState(ctx context.Context, id string) error {
	return p.sessionManager.DeleteSession(ctx, id)
}

// ListStates lists all session IDs
func (p *SessionStateProvider) ListStates(ctx context.Context) ([]string, error) {
	sessions, err := p.sessionManager.ListSessions(ctx, map[string]interface{}{})
	if err != nil {
		return nil, err
	}

	ids := make([]string, len(sessions))
	for i, s := range sessions {
		if sessionState, ok := s.(*session.SessionState); ok {
			ids[i] = sessionState.SessionID
		}
	}
	return ids, nil
}

// ConversationStateProvider provides access to conversation state
type ConversationStateProvider struct {
	states map[string]*BasicConversationState
	mu     sync.RWMutex
}

// NewConversationStateProvider creates a new conversation state provider
func NewConversationStateProvider() StateProvider {
	return &ConversationStateProvider{
		states: make(map[string]*BasicConversationState),
	}
}

// GetState retrieves conversation state
func (p *ConversationStateProvider) GetState(ctx context.Context, id string) (interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	state, exists := p.states[id]
	if !exists {
		return nil, fmt.Errorf("conversation state not found: %s", id)
	}
	return state, nil
}

// SetState updates conversation state
func (p *ConversationStateProvider) SetState(ctx context.Context, id string, state interface{}) error {
	conversationState, ok := state.(*BasicConversationState)
	if !ok {
		return fmt.Errorf("invalid state type for conversation provider")
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.states[id] = conversationState
	return nil
}

// DeleteState removes conversation state
func (p *ConversationStateProvider) DeleteState(ctx context.Context, id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.states, id)
	return nil
}

// ListStates lists all conversation IDs
func (p *ConversationStateProvider) ListStates(ctx context.Context) ([]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ids := make([]string, 0, len(p.states))
	for id := range p.states {
		ids = append(ids, id)
	}
	return ids, nil
}

// WorkflowStateProvider provides access to workflow state
type WorkflowStateProvider struct {
	checkpointManager CheckpointManagerInterface
	sessions          map[string]WorkflowSessionInterface
	mu                sync.RWMutex
}

// NewWorkflowStateProvider creates a new workflow state provider
func NewWorkflowStateProvider(checkpointManager CheckpointManagerInterface) StateProvider {
	return &WorkflowStateProvider{
		checkpointManager: checkpointManager,
		sessions:          make(map[string]WorkflowSessionInterface),
	}
}

// GetState retrieves workflow state
func (p *WorkflowStateProvider) GetState(ctx context.Context, id string) (interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	session, exists := p.sessions[id]
	if !exists {
		return nil, fmt.Errorf("workflow state not found: %s", id)
	}
	return session, nil
}

// SetState updates workflow state
func (p *WorkflowStateProvider) SetState(ctx context.Context, id string, state interface{}) error {
	workflowSession, ok := state.(WorkflowSessionInterface)
	if !ok {
		return fmt.Errorf("invalid state type for workflow provider")
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.sessions[id] = workflowSession

	// Also create a checkpoint
	if p.checkpointManager != nil {
		return p.checkpointManager.SaveCheckpoint(ctx, id, workflowSession)
	}

	return nil
}

// DeleteState removes workflow state
func (p *WorkflowStateProvider) DeleteState(ctx context.Context, id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.sessions, id)
	return nil
}

// ListStates lists all workflow session IDs
func (p *WorkflowStateProvider) ListStates(ctx context.Context) ([]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ids := make([]string, 0, len(p.sessions))
	for id := range p.sessions {
		ids = append(ids, id)
	}
	return ids, nil
}

// ToolStateProvider provides access to tool-specific state
type ToolStateProvider struct {
	states map[string]interface{}
	mu     sync.RWMutex
}

// NewToolStateProvider creates a new tool state provider
func NewToolStateProvider() StateProvider {
	return &ToolStateProvider{
		states: make(map[string]interface{}),
	}
}

// GetState retrieves tool state
func (p *ToolStateProvider) GetState(ctx context.Context, id string) (interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	state, exists := p.states[id]
	if !exists {
		return nil, fmt.Errorf("tool state not found: %s", id)
	}
	return state, nil
}

// SetState updates tool state
func (p *ToolStateProvider) SetState(ctx context.Context, id string, state interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.states[id] = state
	return nil
}

// DeleteState removes tool state
func (p *ToolStateProvider) DeleteState(ctx context.Context, id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.states, id)
	return nil
}

// ListStates lists all tool state IDs
func (p *ToolStateProvider) ListStates(ctx context.Context) ([]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ids := make([]string, 0, len(p.states))
	for id := range p.states {
		ids = append(ids, id)
	}
	return ids, nil
}

// GlobalStateProvider provides access to global application state
type GlobalStateProvider struct {
	states map[string]interface{}
	mu     sync.RWMutex
}

// NewGlobalStateProvider creates a new global state provider
func NewGlobalStateProvider() StateProvider {
	return &GlobalStateProvider{
		states: make(map[string]interface{}),
	}
}

// GetState retrieves global state
func (p *GlobalStateProvider) GetState(ctx context.Context, id string) (interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	state, exists := p.states[id]
	if !exists {
		return nil, fmt.Errorf("global state not found: %s", id)
	}
	return state, nil
}

// SetState updates global state
func (p *GlobalStateProvider) SetState(ctx context.Context, id string, state interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.states[id] = state
	return nil
}

// DeleteState removes global state
func (p *GlobalStateProvider) DeleteState(ctx context.Context, id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.states, id)
	return nil
}

// ListStates lists all global state IDs
func (p *GlobalStateProvider) ListStates(ctx context.Context) ([]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ids := make([]string, 0, len(p.states))
	for id := range p.states {
		ids = append(ids, id)
	}
	return ids, nil
}
