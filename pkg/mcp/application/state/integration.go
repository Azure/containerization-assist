package appstate

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/domain"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
)

// StateServiceContainer defines the interface for service container to avoid circular dependency
// StateServiceContainer - Use services.ServiceContainer for the canonical interface
// This version is simplified for state management use
// Deprecated: Use services.ServiceContainer for new code
type StateServiceContainer interface {
	SessionStore() StateSessionStore
	Logger() *slog.Logger
}

// StateManagementIntegration provides high-level state management integration
type StateManagementIntegration struct {
	manager          *UnifiedStateManager
	serviceContainer StateServiceContainer
	metricsObserver  *MetricsObserver
	auditObserver    *AuditObserver
	logger           *slog.Logger
}

// NewStateManagementIntegration creates a new state management integration
func NewStateManagementIntegration(
	sessionManager session.SessionManager,
	checkpointManager CheckpointManagerInterface,
	logger *slog.Logger,
) *StateManagementIntegration {
	manager := NewUnifiedStateManager(sessionManager, logger)

	manager.RegisterStateProvider(StateTypeSession, NewSessionStateProvider(sessionManager))
	manager.RegisterStateProvider(StateTypeConversation, NewConversationStateProvider())
	manager.RegisterStateProvider(StateTypeWorkflow, NewWorkflowStateProvider(checkpointManager))
	manager.RegisterStateProvider(StateTypeTool, NewToolStateProvider())
	manager.RegisterStateProvider(StateTypeGlobal, NewGlobalStateProvider())

	manager.RegisterValidator(StateTypeSession, NewSessionStateValidator())
	manager.RegisterValidator(StateTypeConversation, NewConversationStateValidator())
	manager.RegisterValidator(StateTypeWorkflow, NewWorkflowStateValidator())

	loggingObserver := NewLoggingObserver(logger)
	metricsObserver := NewMetricsObserver(5 * time.Minute)
	auditObserver := NewAuditObserver(10000, logger)

	manager.RegisterObserver(loggingObserver)
	manager.RegisterObserver(metricsObserver)
	manager.RegisterObserver(auditObserver)

	return &StateManagementIntegration{
		manager:         manager,
		metricsObserver: metricsObserver,
		auditObserver:   auditObserver,
		logger:          logger.With("component", "state_integration"),
	}
}

// NewStateManagementIntegrationWithContainer creates a new state management integration with service container
func NewStateManagementIntegrationWithContainer(
	serviceContainer StateServiceContainer,
	logger *slog.Logger,
) *StateManagementIntegration {
	// Create a simple unified state manager that uses the service container
	manager := NewUnifiedStateManager(nil, logger) // TODO: Convert SessionStore to SessionManager

	// Register state providers using service container
	manager.RegisterStateProvider(StateTypeSession, NewSessionStateProviderFromContainer(serviceContainer))
	manager.RegisterStateProvider(StateTypeConversation, NewConversationStateProvider())
	manager.RegisterStateProvider(StateTypeWorkflow, NewWorkflowStateProvider(nil)) // TODO: Get checkpoint manager from container
	manager.RegisterStateProvider(StateTypeTool, NewToolStateProvider())
	manager.RegisterStateProvider(StateTypeGlobal, NewGlobalStateProvider())

	manager.RegisterValidator(StateTypeSession, NewSessionStateValidator())
	manager.RegisterValidator(StateTypeConversation, NewConversationStateValidator())
	manager.RegisterValidator(StateTypeWorkflow, NewWorkflowStateValidator())

	loggingObserver := NewLoggingObserver(logger)
	metricsObserver := NewMetricsObserver(5 * time.Minute)
	auditObserver := NewAuditObserver(10000, logger)

	manager.RegisterObserver(loggingObserver)
	manager.RegisterObserver(metricsObserver)
	manager.RegisterObserver(auditObserver)

	return &StateManagementIntegration{
		manager:          manager,
		serviceContainer: serviceContainer,
		metricsObserver:  metricsObserver,
		auditObserver:    auditObserver,
		logger:           logger.With("component", "state_integration"),
	}
}

// GetManager returns the unified state manager
func (i *StateManagementIntegration) GetManager() *UnifiedStateManager {
	return i.manager
}

// StartSessionWorkflowSync starts synchronization between session and workflow states
func (i *StateManagementIntegration) StartSessionWorkflowSync(ctx context.Context) error {
	mapping := NewWorkflowToSessionMapping()
	sessionID, err := i.manager.syncCoordinator.StartContinuousSync(
		ctx,
		mcptypes.StateSyncConfig{
			Manager:    i.manager,
			SourceType: string(StateTypeWorkflow),
			TargetType: string(StateTypeSession),
			Mapping:    mapping,
			Interval:   30 * time.Second,
		},
	)
	if err != nil {
		return err
	}

	i.logger.Info("Started session-workflow synchronization", slog.String("sync_session", sessionID))
	return nil
}

// CreateConversationFromSession creates a conversation state from session state
func (i *StateManagementIntegration) CreateConversationFromSession(ctx context.Context, sessionID string) (string, error) {
	sessionState, err := i.manager.GetSessionState(ctx, sessionID)
	if err != nil {
		return "", err
	}

	mapping := NewSessionToConversationMapping()
	conversationState, err := mapping.MapState(sessionState)
	if err != nil {
		return "", err
	}

	conversationID := fmt.Sprintf("conv_%s_%d", sessionID, time.Now().UnixNano())
	if err := i.manager.SetState(ctx, StateTypeConversation, conversationID, conversationState); err != nil {
		return "", err
	}

	i.logger.Info("Created conversation from session",
		slog.String("session_id", sessionID),
		slog.String("conversation_id", conversationID))

	return conversationID, nil
}

// CreateToolStateTransaction creates a transaction for tool state updates
func (i *StateManagementIntegration) CreateToolStateTransaction(ctx context.Context, toolName string) *ToolStateTransaction {
	return &ToolStateTransaction{
		transaction: i.manager.CreateStateTransaction(ctx),
		toolName:    toolName,
		logger:      i.logger,
	}
}

// ToolStateTransaction provides tool-specific state transaction operations
type ToolStateTransaction struct {
	transaction *StateTransaction
	toolName    string
	logger      *slog.Logger
}

// SetToolConfig sets tool configuration in the transaction
func (t *ToolStateTransaction) SetToolConfig(config interface{}) *ToolStateTransaction {
	t.transaction.Set(StateTypeTool, fmt.Sprintf("%s_config", t.toolName), config)
	return t
}

// SetToolState sets tool state in the transaction
func (t *ToolStateTransaction) SetToolState(state interface{}) *ToolStateTransaction {
	t.transaction.Set(StateTypeTool, fmt.Sprintf("%s_state", t.toolName), state)
	return t
}

// SetToolMetrics sets tool metrics in the transaction
func (t *ToolStateTransaction) SetToolMetrics(metrics interface{}) *ToolStateTransaction {
	t.transaction.Set(StateTypeTool, fmt.Sprintf("%s_metrics", t.toolName), metrics)
	return t
}

// Commit commits the tool state transaction
func (t *ToolStateTransaction) Commit() error {
	if err := t.transaction.Commit(); err != nil {
		t.logger.Error("Tool state transaction failed",
			slog.String("error", err.Error()),
			slog.String("tool_name", t.toolName))
		return err
	}

	t.logger.Info("Tool state transaction committed",
		slog.String("tool_name", t.toolName))
	return nil
}

// GetStateMetrics returns current state metrics
func (i *StateManagementIntegration) GetStateMetrics() map[string]*StateMetrics {
	return i.metricsObserver.GetAllMetrics()
}

// GetAuditLog returns the state change audit log
func (i *StateManagementIntegration) GetAuditLog(limit int) []AuditEntry {
	return i.auditObserver.GetAuditLog(limit)
}

// RegisterStateChangeAlert registers an alert for specific state changes
func (i *StateManagementIntegration) RegisterStateChangeAlert(name string, handler AlertHandler) {
	alertingObserver := NewAlertingObserver(i.logger)
	alertingObserver.RegisterAlert(name, handler)
	i.manager.RegisterObserver(alertingObserver)
}

// EnableStateReplication enables state replication to a remote system
func (i *StateManagementIntegration) EnableStateReplication(ctx context.Context, config ReplicationConfig) error {
	i.logger.Info("State replication would be enabled",
		slog.String("target", config.TargetURL),
		slog.String("mode", string(config.Mode)))
	return nil
}

// GetServiceContainer returns the service container if available
func (i *StateManagementIntegration) GetServiceContainer() StateServiceContainer {
	return i.serviceContainer
}

// CreateSessionFromServices creates a session using the service container
func (i *StateManagementIntegration) CreateSessionFromServices(ctx context.Context, sessionID string) error {
	if i.serviceContainer == nil {
		return errors.NewError().
			Code(errors.CodeInvalidState).
			Type(errors.ErrTypeInternal).
			Severity(errors.SeverityHigh).
			Message("service container not available").
			WithLocation().
			Build()
	}

	sessionStore := i.serviceContainer.SessionStore()
	session := &api.Session{
		ID:        sessionID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
		State:     make(map[string]interface{}),
	}

	if err := sessionStore.Create(ctx, session); err != nil {
		return err
	}

	i.logger.Info("Created session using service container",
		slog.String("session_id", sessionID))
	return nil
}

// GetSessionFromServices retrieves a session using the service container
func (i *StateManagementIntegration) GetSessionFromServices(ctx context.Context, sessionID string) (interface{}, error) {
	if i.serviceContainer == nil {
		return nil, errors.NewError().
			Code(errors.CodeInvalidState).
			Type(errors.ErrTypeInternal).
			Severity(errors.SeverityHigh).
			Message("service container not available").
			WithLocation().
			Build()
	}

	sessionStore := i.serviceContainer.SessionStore()
	return sessionStore.Get(ctx, sessionID)
}

// ReplicationConfig configures state replication
type ReplicationConfig struct {
	TargetURL       string
	Mode            ReplicationMode
	StateTypes      []StateType
	SyncInterval    time.Duration
	Authentication  map[string]string
	ConflictHandler func(local, remote interface{}) (interface{}, error)
}

// ReplicationMode defines how states are replicated
type ReplicationMode string

const (
	ReplicationModePush          ReplicationMode = "push"
	ReplicationModePull          ReplicationMode = "pull"
	ReplicationModeBidirectional ReplicationMode = "bidirectional"
)
