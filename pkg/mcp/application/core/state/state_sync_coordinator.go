package state

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors/codes"
	mcptypes "github.com/Azure/container-kit/pkg/mcp/domain/types"
	"github.com/rs/zerolog"
)

// StateMapping defines how to map state between different types
type StateMapping interface {
	MapState(source interface{}) (target interface{}, err error)
	SupportsReverse() bool
	ReverseMap(target interface{}) (source interface{}, err error)
}

// SyncStrategy defines how states should be synchronized
type SyncStrategy string

const (
	SyncStrategyOverwrite     SyncStrategy = "overwrite"
	SyncStrategyMerge         SyncStrategy = "merge"
	SyncStrategyIncremental   SyncStrategy = "incremental"
	SyncStrategyBidirectional SyncStrategy = "bidirectional"
)

// StateSyncCoordinator coordinates state synchronization between providers
type StateSyncCoordinator struct {
	activeSyncs map[string]*SyncSession
	mu          sync.RWMutex
	logger      zerolog.Logger
}

// SyncSession represents an active synchronization session
type SyncSession struct {
	ID         string
	SourceType StateType
	TargetType StateType
	Strategy   SyncStrategy
	StartTime  time.Time
	LastSync   time.Time
	SyncCount  int
	Errors     []error
	Active     bool
	mu         sync.Mutex
}

// NewStateSyncCoordinator creates a new sync coordinator
func NewStateSyncCoordinator(logger zerolog.Logger) *StateSyncCoordinator {
	return &StateSyncCoordinator{
		activeSyncs: make(map[string]*SyncSession),
		logger:      logger.With().Str("component", "state_sync_coordinator").Logger(),
	}
}

// SyncStates synchronizes states between two providers
func (c *StateSyncCoordinator) SyncStates(ctx context.Context, manager *UnifiedStateManager, sourceType, targetType StateType, mapping StateMapping) error {
	session := &SyncSession{
		ID:         c.generateSyncID(sourceType, targetType),
		SourceType: sourceType,
		TargetType: targetType,
		Strategy:   SyncStrategyOverwrite,
		StartTime:  time.Now(),
		Active:     true,
	}

	c.mu.Lock()
	c.activeSyncs[session.ID] = session
	c.mu.Unlock()

	defer func() {
		session.mu.Lock()
		session.Active = false
		session.mu.Unlock()
	}()

	sourceProvider, exists := manager.stateProviders[sourceType]
	if !exists {
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
			fmt.Sprintf("No provider for source type: %s", sourceType),
			nil,
		)
		systemErr.Context["source_type"] = string(sourceType)
		systemErr.Context["component"] = "state_sync_coordinator"
		systemErr.Suggestions = append(systemErr.Suggestions, "Register a state provider for the source type")
		return systemErr
	}

	targetProvider, exists := manager.stateProviders[targetType]
	if !exists {
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
			fmt.Sprintf("No provider for target type: %s", targetType),
			nil,
		)
		systemErr.Context["target_type"] = string(targetType)
		systemErr.Context["component"] = "state_sync_coordinator"
		systemErr.Suggestions = append(systemErr.Suggestions, "Register a state provider for the target type")
		return systemErr
	}

	sourceIDs, err := sourceProvider.ListStates(ctx)
	if err != nil {
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
			"Failed to list source states",
			err,
		)
		systemErr.Context["source_type"] = string(sourceType)
		systemErr.Context["component"] = "state_sync_coordinator"
		systemErr.Suggestions = append(systemErr.Suggestions, "Check source state provider availability")
		return systemErr
	}

	var syncErrors []error
	for _, id := range sourceIDs {
		if err := c.syncSingleState(ctx, manager, sourceProvider, targetProvider, id, mapping, session); err != nil {
			systemErr := errors.SystemError(
				codes.SYSTEM_ERROR,
				fmt.Sprintf("Failed to sync state %s", id),
				err,
			)
			systemErr.Context["state_id"] = id
			systemErr.Context["session_id"] = session.ID
			systemErr.Context["component"] = "state_sync_coordinator"
			syncErrors = append(syncErrors, systemErr)
			session.mu.Lock()
			session.Errors = append(session.Errors, err)
			session.mu.Unlock()
		} else {
			session.mu.Lock()
			session.SyncCount++
			session.mu.Unlock()
		}
	}

	session.mu.Lock()
	session.LastSync = time.Now()
	session.mu.Unlock()

	if len(syncErrors) > 0 {
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
			fmt.Sprintf("Sync completed with %d errors", len(syncErrors)),
			nil,
		)
		systemErr.Context["error_count"] = len(syncErrors)
		systemErr.Context["github.com/Azure/container-kit/pkg/mcp/domain/errors"] = syncErrors
		systemErr.Context["session_id"] = session.ID
		systemErr.Context["component"] = "state_sync_coordinator"
		systemErr.Suggestions = append(systemErr.Suggestions, "Check individual sync errors for specific issues")
		return systemErr
	}

	c.logger.Info().
		Str("session_id", session.ID).
		Str("source_type", string(sourceType)).
		Str("target_type", string(targetType)).
		Int("synced_count", session.SyncCount).
		Msg("State synchronization completed")

	return nil
}

// syncSingleState syncs a single state between providers
func (c *StateSyncCoordinator) syncSingleState(
	ctx context.Context,
	manager *UnifiedStateManager,
	sourceProvider, targetProvider StateProvider,
	id string,
	mapping StateMapping,
	session *SyncSession,
) error {
	sourceState, err := sourceProvider.GetState(ctx, id)
	if err != nil {
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
			"Failed to get source state",
			err,
		)
		systemErr.Context["state_id"] = id
		systemErr.Context["component"] = "state_sync_coordinator"
		systemErr.Suggestions = append(systemErr.Suggestions, "Check source state provider and state ID")
		return systemErr
	}

	targetState, err := mapping.MapState(sourceState)
	if err != nil {
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
			"Failed to map state",
			err,
		)
		systemErr.Context["state_id"] = id
		systemErr.Context["component"] = "state_sync_coordinator"
		systemErr.Suggestions = append(systemErr.Suggestions, "Check state mapping implementation")
		return systemErr
	}

	if err := targetProvider.SetState(ctx, id, targetState); err != nil {
		systemErr := errors.SystemError(
			codes.SYSTEM_ERROR,
			"Failed to set target state",
			err,
		)
		systemErr.Context["state_id"] = id
		systemErr.Context["component"] = "state_sync_coordinator"
		systemErr.Suggestions = append(systemErr.Suggestions, "Check target state provider availability")
		return systemErr
	}

	event := &StateEvent{
		ID:        generateEventID(),
		Type:      StateEventSynced,
		StateType: session.TargetType,
		StateID:   id,
		NewValue:  targetState,
		Metadata: map[string]interface{}{
			"sync_session": session.ID,
			"source_type":  session.SourceType,
		},
		Timestamp: time.Now(),
	}

	manager.notifyObservers(event)
	manager.eventStore.StoreEvent(event)

	return nil
}

// StartContinuousSync starts continuous synchronization between state types
func (c *StateSyncCoordinator) StartContinuousSync(
	ctx context.Context,
	config mcptypes.StateSyncConfig,
) (string, error) {
	manager, ok := config.Manager.(*UnifiedStateManager)
	if !ok {
		return "", errors.NewError().
			Code("TYPE_ASSERTION_FAILED").
			Message("Failed to convert manager to UnifiedStateManager").
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Context("manager_type", fmt.Sprintf("%T", config.Manager)).
			Suggestion("Ensure the manager is of type UnifiedStateManager").
			WithLocation().
			Build()
	}

	sourceType := StateType(config.SourceType)
	targetType := StateType(config.TargetType)

	mapping, ok := config.Mapping.(StateMapping)
	if !ok {
		return "", errors.NewError().
			Code("TYPE_ASSERTION_FAILED").
			Message("Failed to convert mapping to StateMapping").
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Context("mapping_type", fmt.Sprintf("%T", config.Mapping)).
			Suggestion("Ensure the mapping implements StateMapping interface").
			WithLocation().
			Build()
	}

	interval := config.Interval

	sessionID := c.generateSyncID(sourceType, targetType)

	c.mu.RLock()
	if existing, exists := c.activeSyncs[sessionID]; exists && existing.Active {
		c.mu.RUnlock()
		return "", errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Message(fmt.Sprintf("Sync already active for %s -> %s", sourceType, targetType)).
			Context("source_type", string(sourceType)).
			Context("target_type", string(targetType)).
			Context("existing_session", sessionID).
			Context("component", "state_sync_coordinator").
			Suggestion("Stop the existing sync session before starting a new one").
			Build()
	}
	c.mu.RUnlock()

	session := &SyncSession{
		ID:         sessionID,
		SourceType: sourceType,
		TargetType: targetType,
		Strategy:   SyncStrategyIncremental,
		StartTime:  time.Now(),
		Active:     true,
	}

	c.mu.Lock()
	c.activeSyncs[sessionID] = session
	c.mu.Unlock()

	go c.continuousSyncRoutine(ctx, manager, session, mapping, interval)

	c.logger.Info().
		Str("session_id", sessionID).
		Str("source_type", string(sourceType)).
		Str("target_type", string(targetType)).
		Dur("interval", interval).
		Msg("Started continuous state synchronization")

	return sessionID, nil
}

// continuousSyncRoutine runs continuous synchronization
func (c *StateSyncCoordinator) continuousSyncRoutine(
	ctx context.Context,
	manager *UnifiedStateManager,
	session *SyncSession,
	mapping StateMapping,
	interval time.Duration,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			session.mu.Lock()
			session.Active = false
			session.mu.Unlock()
			return

		case <-ticker.C:
			if err := c.SyncStates(ctx, manager, session.SourceType, session.TargetType, mapping); err != nil {
				c.logger.Error().
					Err(err).
					Str("session_id", session.ID).
					Msg("Continuous sync iteration failed")
			}
		}
	}
}

// StopContinuousSync stops a continuous synchronization session
func (c *StateSyncCoordinator) StopContinuousSync(sessionID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	session, exists := c.activeSyncs[sessionID]
	if !exists {
		return errors.NewError().
			Code(codes.VALIDATION_FAILED).
			Message(fmt.Sprintf("Sync session not found: %s", sessionID)).
			Context("session_id", sessionID).
			Context("component", "state_sync_coordinator").
			Suggestion("Check session ID or start a new sync session").
			Build()
	}

	session.mu.Lock()
	session.Active = false
	session.mu.Unlock()

	c.logger.Info().
		Str("session_id", sessionID).
		Msg("Stopped continuous state synchronization")

	return nil
}

// GetActiveSyncs returns all active sync sessions
func (c *StateSyncCoordinator) GetActiveSyncs() []*SyncSession {
	c.mu.RLock()
	defer c.mu.RUnlock()

	sessions := make([]*SyncSession, 0, len(c.activeSyncs))
	for _, session := range c.activeSyncs {
		if session.Active {
			sessions = append(sessions, session)
		}
	}

	return sessions
}

// generateSyncID generates a unique sync session ID
func (c *StateSyncCoordinator) generateSyncID(sourceType, targetType StateType) string {
	return fmt.Sprintf("sync_%s_to_%s", sourceType, targetType)
}
