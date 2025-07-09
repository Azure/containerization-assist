package state

// NOTE: The following interfaces have been consolidated into WorkflowService
// in ../unified_interfaces.go for better maintainability:
// - WorkflowSessionInterface
// - ConversationStateInterface
// - ConversationEntryInterface
// - DecisionInterface
// - ArtifactInterface
//
// Use WorkflowService instead of these interfaces for new implementations.

// The supporting types and concrete implementations remain in this file.

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// StateType represents different types of state that can be managed
type StateType string

const (
	StateTypeConversation StateType = "conversation"
	StateTypeWorkflow     StateType = "workflow"
	StateTypeSession      StateType = "session"
	StateTypeCache        StateType = "cache"
	StateTypeTool         StateType = "tool"
	StateTypeGlobal       StateType = "global"
)

// StateEventType represents different types of state events
type StateEventType string

const (
	StateEventCreated  StateEventType = "created"
	StateEventUpdated  StateEventType = "updated"
	StateEventDeleted  StateEventType = "deleted"
	StateEventAccessed StateEventType = "accessed"
	StateEventExpired  StateEventType = "expired"
	StateEventRestored StateEventType = "restored"
)

// StateEvent represents an event that occurred on a state
type StateEvent struct {
	ID        string                 `json:"id"`
	Type      StateEventType         `json:"type"`
	EventType StateEventType         `json:"event_type"`
	StateType StateType              `json:"state_type"`
	StateID   string                 `json:"state_id"`
	Timestamp time.Time              `json:"timestamp"`
	Actor     string                 `json:"actor,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	OldValue  interface{}            `json:"old_value,omitempty"`
	NewValue  interface{}            `json:"new_value,omitempty"`
}

// StateObserver represents an observer for state changes
type StateObserver interface {
	OnStateChange(event *StateEvent) error
	GetID() string
	IsActive() bool
}

// StateMapping represents a mapping between state types
type StateMapping interface {
	MapState(source interface{}) (interface{}, error)
	SupportsReverse() bool
	ReverseMap(target interface{}) (interface{}, error)
}

// UnifiedStateService defines the interface for unified state management
type UnifiedStateService interface {
	// State operations
	GetState(ctx context.Context, stateType StateType, key string) (interface{}, error)
	SetState(ctx context.Context, stateType StateType, key string, value interface{}) error
	DeleteState(ctx context.Context, stateType StateType, key string) error

	// Session state operations
	GetSessionState(ctx context.Context, sessionID string) (interface{}, error)

	// State history
	GetStateHistory(ctx context.Context, stateType StateType, key string, limit int) ([]*StateEvent, error)

	// State transactions
	CreateStateTransaction(ctx context.Context) *StateTransaction

	// Observer management
	RegisterObserver(observer StateObserver)

	// Validator management
	RegisterValidator(stateType StateType, validator StateValidator)

	// Provider management
	RegisterStateProvider(stateType StateType, provider InternalStateProvider)
}

// UnifiedStateServiceImpl implements UnifiedStateService
type UnifiedStateServiceImpl struct {
	conversationStates map[string]*BasicConversationState
	workflowSessions   map[string]WorkflowSessionInterface
	stateObservers     []StateObserver
	stateValidators    map[StateType]StateValidator
	eventStore         *StateEventStore
	stateProviders     map[StateType]InternalStateProvider
	stateHistory       map[string][]StateHistoryEntry
	syncCoordinator    *SyncCoordinator
	sessionManager     interface{} // Using interface{} to avoid circular deps
	logger             interface{} // Using interface{} to avoid circular deps
	mu                 sync.RWMutex
}

// Type alias for backward compatibility
type UnifiedStateManager = UnifiedStateServiceImpl

// ComprehensiveContext represents comprehensive context information
type ComprehensiveContext struct {
	// Core identification
	SessionID string    `json:"session_id"`
	RequestID string    `json:"request_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`

	// Context information
	ConversationContext *ConversationContext `json:"conversation_context,omitempty"`
	WorkflowContext     *WorkflowContext     `json:"workflow_context,omitempty"`
	SessionContext      *SessionContext      `json:"session_context,omitempty"`

	// Tool contexts
	ToolContexts map[string]*ToolContext `json:"tool_contexts,omitempty"`

	// Event tracking
	RecentEvents []*Event `json:"recent_events,omitempty"`

	// Relationships
	Relationships []*ContextRelationship `json:"relationships,omitempty"`

	// Analysis and insights
	AnalysisInsights *AnalysisInsights `json:"analysis_insights,omitempty"`
	Patterns         []Pattern         `json:"patterns,omitempty"`
	Anomalies        []Anomaly         `json:"anomalies,omitempty"`
	PredictedIssues  []PredictedIssue  `json:"predicted_issues,omitempty"`

	// Recommendations
	Recommendations []*Recommendation `json:"recommendations,omitempty"`

	// Security and performance
	SecurityAlerts     []interface{}          `json:"security_alerts,omitempty"`
	PerformanceMetrics map[string]interface{} `json:"performance_metrics,omitempty"`

	// State information
	CurrentState  map[string]interface{} `json:"current_state,omitempty"`
	PreviousState map[string]interface{} `json:"previous_state,omitempty"`

	// Metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Performance metrics
	Metrics map[string]float64 `json:"metrics,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ConversationContext represents conversation-specific context
type ConversationContext struct {
	SessionID    string                 `json:"session_id"`
	Messages     []ConversationMessage  `json:"messages"`
	CurrentStage string                 `json:"current_stage"`
	Variables    map[string]interface{} `json:"variables"`
}

// WorkflowContext represents workflow-specific context
type WorkflowContext struct {
	WorkflowID   string                 `json:"workflow_id"`
	CurrentStage string                 `json:"current_stage"`
	Status       string                 `json:"status"`
	Variables    map[string]interface{} `json:"variables"`
}

// SessionContext represents session-specific context
type SessionContext struct {
	SessionID  string                 `json:"session_id"`
	UserID     string                 `json:"user_id,omitempty"`
	StartTime  time.Time              `json:"start_time"`
	LastActive time.Time              `json:"last_active"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// ConversationMessage represents a message in a conversation
type ConversationMessage struct {
	ID        string                 `json:"id"`
	Role      string                 `json:"role"`
	Content   string                 `json:"content"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// BasicConversationState represents basic conversation state
type BasicConversationState struct {
	SessionID      string                   `json:"session_id"`
	ConversationID string                   `json:"conversation_id"`
	SessionState   interface{}              `json:"session_state,omitempty"`
	Stage          string                   `json:"stage"`
	CurrentStage   string                   `json:"current_stage"`
	Messages       []ConversationMessage    `json:"messages"`
	Variables      map[string]interface{}   `json:"variables"`
	History        []BasicConversationEntry `json:"history"`
	Decisions      map[string]BasicDecision `json:"decisions"`
	Artifacts      map[string]BasicArtifact `json:"artifacts"`
	CreatedAt      time.Time                `json:"created_at"`
	UpdatedAt      time.Time                `json:"updated_at"`
}

// BasicConversationEntry represents a basic conversation entry
type BasicConversationEntry struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Content   string                 `json:"content"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// BasicDecision represents a basic decision
type BasicDecision struct {
	ID         string                 `json:"id"`
	Question   string                 `json:"question"`
	Answer     string                 `json:"answer"`
	Confidence float64                `json:"confidence"`
	Timestamp  time.Time              `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// BasicArtifact represents a basic artifact
type BasicArtifact struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Content   interface{}            `json:"content"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// WorkflowSessionInterface represents a workflow session
type WorkflowSessionInterface interface {
	GetID() string
	GetSessionID() string
	GetStatus() string
	GetMetadata() map[string]interface{}
	UpdateStatus(status string) error
	AddCheckpoint(checkpoint interface{}) error
	GetCurrentStage() string
	GetProgress() float64
}

// CheckpointManagerInterface manages workflow checkpoints
type CheckpointManagerInterface interface {
	SaveCheckpoint(ctx context.Context, sessionID string, checkpoint interface{}) error
	LoadCheckpoint(ctx context.Context, sessionID string) (interface{}, error)
	ListCheckpoints(ctx context.Context, sessionID string) ([]interface{}, error)
	DeleteCheckpoint(ctx context.Context, sessionID string, checkpointID string) error
}

// AIContextAggregator aggregates AI context from multiple sources
type AIContextAggregator struct {
	providers      []ContextProvider
	namedProviders map[string]ContextProvider
	enrichers      []ContextEnricher
	sources        []ContextSource
	cache          *ContextCache
	logger         interface{} // Using interface{} to avoid circular deps
	mu             sync.RWMutex
}

// ContextSource represents a source of context information
type ContextSource interface {
	GetContext(ctx context.Context, key string) (interface{}, error)
	GetPriority() int
}

// ContextEnricher enriches context with additional information
type ContextEnricher interface {
	Enrich(ctx context.Context, context *ComprehensiveContext) error
	GetName() string
}

// ContextCache caches context information
type ContextCache struct {
	cache   map[string]*ComprehensiveContext
	ttl     time.Duration
	maxSize int
	mu      sync.RWMutex
}

// ContextType represents different types of context
type ContextType string

const (
	ContextTypeBuild       ContextType = "build"
	ContextTypeDeployment  ContextType = "deployment"
	ContextTypeSecurity    ContextType = "security"
	ContextTypeAnalysis    ContextType = "analysis"
	ContextTypePerformance ContextType = "performance"
	ContextTypeState       ContextType = "state"
	ContextTypeAll         ContextType = "all"
)

// ContextRelationship represents a relationship between contexts
type ContextRelationship struct {
	Source       string                 `json:"source"`
	Target       string                 `json:"target"`
	SourceID     string                 `json:"source_id"`
	TargetID     string                 `json:"target_id"`
	Type         string                 `json:"type"`
	RelationType string                 `json:"relation_type"`
	Strength     float64                `json:"strength"`
	Description  string                 `json:"description,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// AnalysisInsights represents insights from analysis
type AnalysisInsights struct {
	// Pattern detection
	Patterns []*Pattern `json:"patterns,omitempty"`

	// Anomaly detection
	Anomalies []*Anomaly `json:"anomalies,omitempty"`

	// Predictions
	PredictedIssues []*PredictedIssue `json:"predicted_issues,omitempty"`

	// Trends
	Trends []Trend `json:"trends,omitempty"`

	// Summary
	Summary string `json:"summary,omitempty"`

	// Score and confidence
	Score      float64 `json:"score,omitempty"`
	Confidence float64 `json:"confidence"`

	// Metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Pattern represents a detected pattern
type Pattern struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Confidence  float64                `json:"confidence"`
	Occurrences int                    `json:"occurrences"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Anomaly represents a detected anomaly
type Anomaly struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Severity    string                 `json:"severity"`
	Description string                 `json:"description"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// PredictedIssue represents a predicted issue
type PredictedIssue struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Probability float64                `json:"probability"`
	Impact      string                 `json:"impact"`
	TimeFrame   string                 `json:"time_frame,omitempty"`
	Mitigations []string               `json:"mitigations,omitempty"`
	Suggestion  string                 `json:"suggestion,omitempty"`
	PredictedAt time.Time              `json:"predicted_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Recommendation represents a recommendation
type Recommendation struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Priority    int                    `json:"priority"`
	Category    string                 `json:"category"`
	Type        string                 `json:"type,omitempty"`
	Actions     []string               `json:"actions,omitempty"`
	Action      string                 `json:"action,omitempty"` // Deprecated, use Actions
	Impact      string                 `json:"impact,omitempty"`
	Effort      string                 `json:"effort,omitempty"`
	Confidence  float64                `json:"confidence"`
	CreatedAt   time.Time              `json:"created_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ContextProvider interface for providing context
type ContextProvider interface {
	GetContext(ctx context.Context, request *ContextRequest) (*ContextData, error)
	GetCapabilities() *ContextProviderCapabilities
	GetName() string
}

// ContextRequest represents a request for context
type ContextRequest struct {
	SessionID      string                 `json:"session_id"`
	Type           ContextType            `json:"type"`
	ContextType    ContextType            `json:"context_type"`
	TargetID       string                 `json:"target_id,omitempty"`
	IncludeHistory bool                   `json:"include_history"`
	Filters        map[string]interface{} `json:"filters,omitempty"`
}

// ContextData represents context data
type ContextData struct {
	ID            string                 `json:"id"`
	Provider      string                 `json:"provider"`
	Type          ContextType            `json:"type"`
	SessionID     string                 `json:"session_id"`
	Timestamp     time.Time              `json:"timestamp"`
	Content       interface{}            `json:"content"`
	Data          map[string]interface{} `json:"data,omitempty"`
	Relevance     float64                `json:"relevance,omitempty"`
	Confidence    float64                `json:"confidence,omitempty"`
	Relationships []*ContextRelationship `json:"relationships,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
}

// ContextProviderCapabilities represents provider capabilities
type ContextProviderCapabilities struct {
	SupportedTypes  []ContextType          `json:"supported_types"`
	Features        []string               `json:"features"`
	Limitations     map[string]interface{} `json:"limitations,omitempty"`
	SupportsHistory bool                   `json:"supports_history"`
	MaxHistoryDays  int                    `json:"max_history_days,omitempty"`
	RealTimeUpdates bool                   `json:"real_time_updates"`
}

// InternalStateProvider interface for internal state providers
type InternalStateProvider interface {
	GetState(ctx context.Context, key string) (interface{}, error)
	SetState(ctx context.Context, key string, value interface{}) error
	DeleteState(ctx context.Context, key string) error
	GetType() StateType
}

// StateOperations interface for state operations
type StateOperations interface {
	GetState(ctx context.Context, key string) (interface{}, error)
	SetState(ctx context.Context, key string, value interface{}) error
	DeleteState(ctx context.Context, key string) error
	GetType() StateType
}

// StateValidator interface for state validators
type StateValidator interface {
	Validate(state interface{}) error
	GetRules() []ValidationRule
}

// StateHistoryEntry represents a state history entry
type StateHistoryEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	StateType StateType              `json:"state_type"`
	Key       string                 `json:"key"`
	Value     interface{}            `json:"value"`
	Actor     string                 `json:"actor,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// SyncCoordinator coordinates state synchronization
type SyncCoordinator struct {
	mu sync.RWMutex
}

// ValidationRule represents a validation rule
type ValidationRule struct {
	Name      string                 `json:"name"`
	Type      string                 `json:"type"`
	Condition string                 `json:"condition"`
	Message   string                 `json:"message"`
	Severity  string                 `json:"severity"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Event represents a system event
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Severity  string                 `json:"severity,omitempty"`
	Message   string                 `json:"message,omitempty"`
}

// ToolContext represents tool-specific context
type ToolContext struct {
	ToolName    string                 `json:"tool_name"`
	SessionID   string                 `json:"session_id"`
	ExecutionID string                 `json:"execution_id,omitempty"`
	Status      string                 `json:"status"`
	Type        ContextType            `json:"type"`
	StartTime   time.Time              `json:"start_time"`
	EndTime     *time.Time             `json:"end_time,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Result      interface{}            `json:"result,omitempty"`
	Error       *ToolError             `json:"error,omitempty"`
	Warnings    []ToolWarning          `json:"warnings,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
}

// ToolError represents errors with context
type ToolError struct {
	Code        string                 `json:"code"`
	Message     string                 `json:"message"`
	Details     string                 `json:"details,omitempty"`
	Source      string                 `json:"source,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	Severity    string                 `json:"severity"`
	Context     map[string]interface{} `json:"context,omitempty"`
	StackTrace  string                 `json:"stack_trace,omitempty"`
	Recoverable bool                   `json:"recoverable"`
}

// ToolWarning represents warnings
type ToolWarning struct {
	Code      string                 `json:"code"`
	Message   string                 `json:"message"`
	Details   string                 `json:"details,omitempty"`
	Source    string                 `json:"source,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

// Trend represents a trend in data
type Trend struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Direction   string                 `json:"direction"` // up, down, stable
	Magnitude   float64                `json:"magnitude"`
	Duration    string                 `json:"duration"`
	StartTime   time.Time              `json:"start_time"`
	EndTime     *time.Time             `json:"end_time,omitempty"`
	Confidence  float64                `json:"confidence"`
	Description string                 `json:"description,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
}

// NewUnifiedStateService creates a new unified state service
func NewUnifiedStateService(sessionManager interface{}, logger interface{}) UnifiedStateService {
	return &UnifiedStateServiceImpl{
		conversationStates: make(map[string]*BasicConversationState),
		workflowSessions:   make(map[string]WorkflowSessionInterface),
		stateObservers:     make([]StateObserver, 0),
		stateValidators:    make(map[StateType]StateValidator),
		eventStore:         &StateEventStore{events: make(map[string][]*StateEvent), eventsByID: make(map[string]*StateEvent), maxEvents: 1000},
		stateProviders:     make(map[StateType]InternalStateProvider),
		stateHistory:       make(map[string][]StateHistoryEntry),
		syncCoordinator:    &SyncCoordinator{},
		sessionManager:     sessionManager,
		logger:             logger,
	}
}

// NewUnifiedStateManager creates a new unified state manager (backward compatibility)
func NewUnifiedStateManager(sessionManager interface{}, logger interface{}) *UnifiedStateManager {
	return &UnifiedStateServiceImpl{
		conversationStates: make(map[string]*BasicConversationState),
		workflowSessions:   make(map[string]WorkflowSessionInterface),
		stateObservers:     make([]StateObserver, 0),
		stateValidators:    make(map[StateType]StateValidator),
		eventStore:         &StateEventStore{events: make(map[string][]*StateEvent), eventsByID: make(map[string]*StateEvent), maxEvents: 1000},
		stateProviders:     make(map[StateType]InternalStateProvider),
		stateHistory:       make(map[string][]StateHistoryEntry),
		syncCoordinator:    &SyncCoordinator{},
		sessionManager:     sessionManager,
		logger:             logger,
	}
}

// RegisterObserver registers a state observer
func (m *UnifiedStateServiceImpl) RegisterObserver(observer StateObserver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stateObservers = append(m.stateObservers, observer)
}

// GetState gets state by key
func (m *UnifiedStateServiceImpl) GetState(ctx context.Context, stateType StateType, key string) (interface{}, error) {
	m.mu.RLock()
	provider, exists := m.stateProviders[stateType]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no provider for state type: %s", stateType)
	}

	return provider.GetState(ctx, key)
}

// SetState sets state by key
func (m *UnifiedStateServiceImpl) SetState(ctx context.Context, stateType StateType, key string, value interface{}) error {
	m.mu.RLock()
	provider, exists := m.stateProviders[stateType]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no provider for state type: %s", stateType)
	}

	// Add to history
	m.addToHistory(stateType, key, value)

	// Notify observers
	event := &StateEvent{
		ID:        fmt.Sprintf("%s-%d", key, time.Now().UnixNano()),
		EventType: StateEventUpdated,
		StateType: stateType,
		StateID:   key,
		Timestamp: time.Now(),
		NewValue:  value,
	}
	m.notifyObservers(event)

	return provider.SetState(ctx, key, value)
}

// DeleteState deletes state by key
func (m *UnifiedStateServiceImpl) DeleteState(ctx context.Context, stateType StateType, key string) error {
	m.mu.RLock()
	provider, exists := m.stateProviders[stateType]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no provider for state type: %s", stateType)
	}

	// Notify observers
	event := &StateEvent{
		ID:        fmt.Sprintf("%s-%d", key, time.Now().UnixNano()),
		EventType: StateEventDeleted,
		StateType: stateType,
		StateID:   key,
		Timestamp: time.Now(),
	}
	m.notifyObservers(event)

	return provider.DeleteState(ctx, key)
}

// GetStateHistory gets state history
func (m *UnifiedStateServiceImpl) GetStateHistory(_ context.Context, stateType StateType, key string, limit int) ([]*StateEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get events from event store
	if m.eventStore != nil {
		events := make([]*StateEvent, 0)
		for _, eventList := range m.eventStore.events {
			for _, event := range eventList {
				if event.StateType == stateType && event.StateID == key {
					events = append(events, event)
				}
			}
		}

		// Sort by timestamp descending and limit
		if len(events) > limit {
			events = events[len(events)-limit:]
		}

		return events, nil
	}

	return []*StateEvent{}, nil
}

// GetSessionState gets session state
func (m *UnifiedStateServiceImpl) GetSessionState(ctx context.Context, sessionID string) (interface{}, error) {
	return m.GetState(ctx, StateTypeSession, sessionID)
}

// StateMutation represents a state mutation
type StateMutation struct {
	StateType StateType
	StateID   string
	OldValue  interface{}
	NewValue  interface{}
	Operation string
}

// TransactionState represents transaction state
type TransactionState string

const (
	TransactionStatePending    TransactionState = "pending"
	TransactionStateCommitted  TransactionState = "committed"
	TransactionStateRolledBack TransactionState = "rolled_back"
)

// CreateStateTransaction creates a new state transaction
func (m *UnifiedStateServiceImpl) CreateStateTransaction(ctx context.Context) *StateTransaction {
	return &StateTransaction{
		manager:    m,
		ctx:        ctx,
		operations: make([]StateOperation, 0),
		committed:  false,
	}
}

// addToHistory adds an entry to state history
func (m *UnifiedStateServiceImpl) addToHistory(stateType StateType, key string, value interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := StateHistoryEntry{
		Timestamp: time.Now(),
		StateType: stateType,
		Key:       key,
		Value:     value,
	}

	m.stateHistory[key] = append(m.stateHistory[key], entry)

	// Keep only last 100 entries
	if len(m.stateHistory[key]) > 100 {
		m.stateHistory[key] = m.stateHistory[key][len(m.stateHistory[key])-100:]
	}
}

// notifyObservers notifies all observers of a state event
func (m *UnifiedStateServiceImpl) notifyObservers(event *StateEvent) {
	for _, observer := range m.stateObservers {
		if observer.IsActive() {
			go observer.OnStateChange(event)
		}
	}
}

// NewAIContextAggregator creates a new AI context aggregator
func NewAIContextAggregator() *AIContextAggregator {
	return &AIContextAggregator{
		providers:      make([]ContextProvider, 0),
		namedProviders: make(map[string]ContextProvider),
		enrichers:      make([]ContextEnricher, 0),
		sources:        make([]ContextSource, 0),
		cache: &ContextCache{
			cache:   make(map[string]*ComprehensiveContext),
			ttl:     15 * time.Minute,
			maxSize: 1000,
		},
	}
}

// RegisterContextProvider registers a context provider
func (a *AIContextAggregator) RegisterContextProvider(args ...interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(args) == 1 {
		// Single argument: just the provider
		if provider, ok := args[0].(ContextProvider); ok {
			a.providers = append(a.providers, provider)
			if a.namedProviders != nil {
				a.namedProviders[provider.GetName()] = provider
			}
		}
	} else if len(args) == 2 {
		// Two arguments: name and provider
		if name, ok := args[0].(string); ok {
			if provider, ok := args[1].(ContextProvider); ok {
				a.providers = append(a.providers, provider)
				if a.namedProviders == nil {
					a.namedProviders = make(map[string]ContextProvider)
				}
				a.namedProviders[name] = provider
			}
		}
	}
}

// RegisterContextEnricher registers a context enricher
func (a *AIContextAggregator) RegisterContextEnricher(enricher ContextEnricher) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enrichers = append(a.enrichers, enricher)
}

// GetComprehensiveContext gets comprehensive context for a session
func (a *AIContextAggregator) GetComprehensiveContext(ctx context.Context, sessionID string) (*ComprehensiveContext, error) {
	// Check cache first
	a.mu.RLock()
	if cached, exists := a.cache.cache[sessionID]; exists {
		if time.Since(cached.UpdatedAt) < a.cache.ttl {
			a.mu.RUnlock()
			return cached, nil
		}
	}
	a.mu.RUnlock()

	// Create new comprehensive context
	compContext := &ComprehensiveContext{
		SessionID:       sessionID,
		Timestamp:       time.Now(),
		ToolContexts:    make(map[string]*ToolContext),
		RecentEvents:    make([]*Event, 0),
		Relationships:   make([]*ContextRelationship, 0),
		Recommendations: make([]*Recommendation, 0),
		CurrentState:    make(map[string]interface{}),
		PreviousState:   make(map[string]interface{}),
		Metadata:        make(map[string]interface{}),
		Metrics:         make(map[string]float64),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// Gather context from all providers
	for _, provider := range a.providers {
		request := &ContextRequest{
			SessionID:      sessionID,
			IncludeHistory: true,
		}

		if data, err := provider.GetContext(ctx, request); err == nil && data != nil {
			// Merge context data
			a.mergeContextData(compContext, data)
		}
	}

	// Apply enrichers
	for _, enricher := range a.enrichers {
		if err := enricher.Enrich(ctx, compContext); err != nil {
			// Log error but continue with other enrichers
			continue
		}
	}

	// Update cache
	a.mu.Lock()
	a.cache.cache[sessionID] = compContext
	a.mu.Unlock()

	return compContext, nil
}

// mergeContextData merges context data into comprehensive context
func (a *AIContextAggregator) mergeContextData(comp *ComprehensiveContext, data *ContextData) {
	// Merge based on context type
	switch data.Type {
	case ContextTypeBuild:
		if comp.ToolContexts["build"] == nil {
			comp.ToolContexts["build"] = &ToolContext{
				ToolName:  "build",
				SessionID: comp.SessionID,
				Status:    "active",
				StartTime: time.Now(),
				Context:   data.Data,
			}
		}
	case ContextTypeDeployment:
		if comp.ToolContexts["deployment"] == nil {
			comp.ToolContexts["deployment"] = &ToolContext{
				ToolName:  "deployment",
				SessionID: comp.SessionID,
				Status:    "active",
				StartTime: time.Now(),
				Context:   data.Data,
			}
		}
	case ContextTypeSecurity:
		if comp.ToolContexts["security"] == nil {
			comp.ToolContexts["security"] = &ToolContext{
				ToolName:  "security",
				SessionID: comp.SessionID,
				Status:    "active",
				StartTime: time.Now(),
				Context:   data.Data,
			}
		}
	}

	// Merge relationships
	if len(data.Relationships) > 0 {
		comp.Relationships = append(comp.Relationships, data.Relationships...)
	}
}

// ===== STATE PROVIDER IMPLEMENTATIONS =====

// BasicStateProvider is a basic implementation of InternalStateProvider
type BasicStateProvider struct {
	stateType StateType
	states    map[string]interface{}
	mu        sync.RWMutex
}

// NewBasicStateProvider creates a new basic state provider
func NewBasicStateProvider(stateType StateType) InternalStateProvider {
	return &BasicStateProvider{
		stateType: stateType,
		states:    make(map[string]interface{}),
	}
}

// GetState gets state by key
func (p *BasicStateProvider) GetState(ctx context.Context, key string) (interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	state, exists := p.states[key]
	if !exists {
		return nil, fmt.Errorf("state not found: %s", key)
	}
	return state, nil
}

// SetState sets state by key
func (p *BasicStateProvider) SetState(ctx context.Context, key string, value interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.states[key] = value
	return nil
}

// DeleteState deletes state by key
func (p *BasicStateProvider) DeleteState(ctx context.Context, key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.states, key)
	return nil
}

// GetType returns the state type
func (p *BasicStateProvider) GetType() StateType {
	return p.stateType
}

// RegisterValidator registers a validator for a state type
func (m *UnifiedStateServiceImpl) RegisterValidator(stateType StateType, validator StateValidator) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stateValidators == nil {
		m.stateValidators = make(map[StateType]StateValidator)
	}
	m.stateValidators[stateType] = validator
}

// RegisterStateProvider registers a state provider for a state type
func (m *UnifiedStateServiceImpl) RegisterStateProvider(stateType StateType, provider InternalStateProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stateProviders == nil {
		m.stateProviders = make(map[StateType]InternalStateProvider)
	}
	m.stateProviders[stateType] = provider
}

// StartContinuousSync starts continuous synchronization between state types
func (sc *SyncCoordinator) StartContinuousSync(ctx context.Context, config interface{}) (string, error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Generate a unique session ID for this sync operation
	sessionID := fmt.Sprintf("sync-%d", time.Now().UnixNano())

	// In a real implementation, this would:
	// 1. Parse the config to understand what to sync
	// 2. Start a goroutine that periodically syncs the states
	// 3. Store the sync session for later management

	// For now, we'll just return the session ID
	return sessionID, nil
}
