package testutil

import (
	"sync"
	"testing"
)

// TestServiceContainer provides isolated services for testing
type TestServiceContainer struct {
	registryService *RegistryService
	sessionService  *SessionService
	stateManager    *StateManager
	mu              sync.RWMutex
}

// RegistryService provides test registry functionality
type RegistryService struct {
	tools map[string]interface{}
	mu    sync.RWMutex
}

// SessionService provides test session management
type SessionService struct {
	sessions map[string]interface{}
	mu       sync.RWMutex
}

// StateManager provides test state management
type StateManager struct {
	state map[string]interface{}
	mu    sync.RWMutex
}

// NewTestServiceContainer creates isolated services for a test
func NewTestServiceContainer(t *testing.T) *TestServiceContainer {
	return &TestServiceContainer{
		registryService: &RegistryService{
			tools: make(map[string]interface{}),
		},
		sessionService: &SessionService{
			sessions: make(map[string]interface{}),
		},
		stateManager: &StateManager{
			state: make(map[string]interface{}),
		},
	}
}

// Registry returns the test registry service
func (c *TestServiceContainer) Registry() *RegistryService {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.registryService
}

// Session returns the test session service
func (c *TestServiceContainer) Session() *SessionService {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessionService
}

// State returns the test state manager
func (c *TestServiceContainer) State() *StateManager {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stateManager
}

// Cleanup cleans up test resources
func (c *TestServiceContainer) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clean up registry
	c.registryService.mu.Lock()
	c.registryService.tools = nil
	c.registryService.mu.Unlock()

	// Clean up sessions
	c.sessionService.mu.Lock()
	c.sessionService.sessions = nil
	c.sessionService.mu.Unlock()

	// Clean up state
	c.stateManager.mu.Lock()
	c.stateManager.state = nil
	c.stateManager.mu.Unlock()
}

// Registry service methods

// Register adds a tool to the test registry
func (r *RegistryService) Register(name string, tool interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[name] = tool
}

// Get retrieves a tool from the test registry
func (r *RegistryService) Get(name string) (interface{}, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, exists := r.tools[name]
	return tool, exists
}

// Session service methods

// Create creates a new test session
func (s *SessionService) Create(id string, data interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[id] = data
}

// Get retrieves a test session
func (s *SessionService) Get(id string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, exists := s.sessions[id]
	return session, exists
}

// State manager methods

// Set sets a state value
func (m *StateManager) Set(key string, value interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state[key] = value
}

// Get retrieves a state value
func (m *StateManager) Get(key string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, exists := m.state[key]
	return value, exists
}
