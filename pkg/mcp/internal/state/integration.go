package state

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/internal/session"
	"github.com/rs/zerolog"
)

// StateManagementIntegration provides high-level state management integration
type StateManagementIntegration struct {
	manager         *UnifiedStateManager
	metricsObserver *MetricsObserver
	auditObserver   *AuditObserver
	logger          zerolog.Logger
}

// NewStateManagementIntegration creates a new state management integration
func NewStateManagementIntegration(
	sessionManager *session.SessionManager,
	checkpointManager CheckpointManagerInterface,
	logger zerolog.Logger,
) *StateManagementIntegration {
	// Create unified state manager
	manager := NewUnifiedStateManager(sessionManager, logger)

	// Register state providers
	manager.RegisterStateProvider(StateTypeSession, NewSessionStateProvider(sessionManager))
	manager.RegisterStateProvider(StateTypeConversation, NewConversationStateProvider())
	manager.RegisterStateProvider(StateTypeWorkflow, NewWorkflowStateProvider(checkpointManager))
	manager.RegisterStateProvider(StateTypeTool, NewToolStateProvider())
	manager.RegisterStateProvider(StateTypeGlobal, NewGlobalStateProvider())

	// Register validators
	manager.RegisterValidator(StateTypeSession, NewSessionStateValidator())
	manager.RegisterValidator(StateTypeConversation, NewConversationStateValidator())
	manager.RegisterValidator(StateTypeWorkflow, NewWorkflowStateValidator())

	// Register migrators
	manager.RegisterMigrator(StateTypeSession, NewSessionStateMigrator())
	manager.RegisterMigrator(StateTypeWorkflow, NewWorkflowStateMigrator())

	// Create observers
	loggingObserver := NewLoggingObserver(logger)
	metricsObserver := NewMetricsObserver(5 * time.Minute)
	auditObserver := NewAuditObserver(10000, logger)

	// Register observers
	manager.RegisterObserver(loggingObserver)
	manager.RegisterObserver(metricsObserver)
	manager.RegisterObserver(auditObserver)

	return &StateManagementIntegration{
		manager:         manager,
		metricsObserver: metricsObserver,
		auditObserver:   auditObserver,
		logger:          logger.With().Str("component", "state_integration").Logger(),
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
		i.manager,
		StateTypeWorkflow,
		StateTypeSession,
		mapping,
		30*time.Second,
	)
	if err != nil {
		return err
	}

	i.logger.Info().Str("sync_session", sessionID).Msg("Started session-workflow synchronization")
	return nil
}

// CreateConversationFromSession creates a conversation state from session state
func (i *StateManagementIntegration) CreateConversationFromSession(ctx context.Context, sessionID string) (string, error) {
	// Get session state
	sessionState, err := i.manager.GetSessionState(ctx, sessionID)
	if err != nil {
		return "", err
	}

	// Map to conversation state
	mapping := NewSessionToConversationMapping()
	conversationState, err := mapping.MapState(sessionState)
	if err != nil {
		return "", err
	}

	// Save conversation state
	conversationID := fmt.Sprintf("conv_%s_%d", sessionID, time.Now().UnixNano())
	if err := i.manager.SetState(ctx, StateTypeConversation, conversationID, conversationState); err != nil {
		return "", err
	}

	i.logger.Info().
		Str("session_id", sessionID).
		Str("conversation_id", conversationID).
		Msg("Created conversation from session")

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
	logger      zerolog.Logger
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
		t.logger.Error().
			Err(err).
			Str("tool_name", t.toolName).
			Msg("Tool state transaction failed")
		return err
	}

	t.logger.Info().
		Str("tool_name", t.toolName).
		Msg("Tool state transaction committed")
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

// MigrateAllStates migrates all states of a given type to a new version
func (i *StateManagementIntegration) MigrateAllStates(ctx context.Context, stateType StateType, fromVersion, toVersion string) error {
	provider, exists := i.manager.stateProviders[stateType]
	if !exists {
		return fmt.Errorf("no provider for state type: %s", stateType)
	}

	// List all states
	stateIDs, err := provider.ListStates(ctx)
	if err != nil {
		return err
	}

	// Migrate each state
	successCount := 0
	errorCount := 0
	for _, id := range stateIDs {
		if err := i.manager.MigrateState(ctx, stateType, id, fromVersion, toVersion); err != nil {
			i.logger.Error().
				Err(err).
				Str("state_type", string(stateType)).
				Str("state_id", id).
				Msg("State migration failed")
			errorCount++
		} else {
			successCount++
		}
	}

	i.logger.Info().
		Str("state_type", string(stateType)).
		Str("from_version", fromVersion).
		Str("to_version", toVersion).
		Int("success_count", successCount).
		Int("error_count", errorCount).
		Msg("Bulk state migration completed")

	if errorCount > 0 {
		return fmt.Errorf("migration completed with %d errors", errorCount)
	}

	return nil
}

// EnableStateReplication enables state replication to a remote system
func (i *StateManagementIntegration) EnableStateReplication(ctx context.Context, config ReplicationConfig) error {
	// This would implement state replication to remote systems
	// For now, just log the intent
	i.logger.Info().
		Str("target", config.TargetURL).
		Str("mode", string(config.Mode)).
		Msg("State replication would be enabled")
	return nil
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
