package state

import (
	"context"
	"fmt"
	"sync"
	"time"

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
	SyncStrategyOverwrite     SyncStrategy = "overwrite"     // Target is completely replaced
	SyncStrategyMerge         SyncStrategy = "merge"         // States are merged
	SyncStrategyIncremental   SyncStrategy = "incremental"   // Only changes are synced
	SyncStrategyBidirectional SyncStrategy = "bidirectional" // Two-way sync
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
	// Create sync session
	session := &SyncSession{
		ID:         c.generateSyncID(sourceType, targetType),
		SourceType: sourceType,
		TargetType: targetType,
		Strategy:   SyncStrategyOverwrite,
		StartTime:  time.Now(),
		Active:     true,
	}

	// Register session
	c.mu.Lock()
	c.activeSyncs[session.ID] = session
	c.mu.Unlock()

	defer func() {
		session.mu.Lock()
		session.Active = false
		session.mu.Unlock()
	}()

	// Get source provider
	sourceProvider, exists := manager.stateProviders[sourceType]
	if !exists {
		return fmt.Errorf("no provider for source type: %s", sourceType)
	}

	// Get target provider
	targetProvider, exists := manager.stateProviders[targetType]
	if !exists {
		return fmt.Errorf("no provider for target type: %s", targetType)
	}

	// List all source states
	sourceIDs, err := sourceProvider.ListStates(ctx)
	if err != nil {
		return fmt.Errorf("failed to list source states: %w", err)
	}

	// Sync each state
	var syncErrors []error
	for _, id := range sourceIDs {
		if err := c.syncSingleState(ctx, manager, sourceProvider, targetProvider, id, mapping, session); err != nil {
			syncErrors = append(syncErrors, fmt.Errorf("failed to sync state %s: %w", id, err))
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
		return fmt.Errorf("sync completed with %d errors", len(syncErrors))
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
	// Get source state
	sourceState, err := sourceProvider.GetState(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get source state: %w", err)
	}

	// Map state
	targetState, err := mapping.MapState(sourceState)
	if err != nil {
		return fmt.Errorf("failed to map state: %w", err)
	}

	// Set target state
	if err := targetProvider.SetState(ctx, id, targetState); err != nil {
		return fmt.Errorf("failed to set target state: %w", err)
	}

	// Notify observers
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
	manager *UnifiedStateManager,
	sourceType, targetType StateType,
	mapping StateMapping,
	interval time.Duration,
) (string, error) {
	sessionID := c.generateSyncID(sourceType, targetType)

	// Check if sync already exists
	c.mu.RLock()
	if existing, exists := c.activeSyncs[sessionID]; exists && existing.Active {
		c.mu.RUnlock()
		return "", fmt.Errorf("sync already active for %s -> %s", sourceType, targetType)
	}
	c.mu.RUnlock()

	// Create sync session
	session := &SyncSession{
		ID:         sessionID,
		SourceType: sourceType,
		TargetType: targetType,
		Strategy:   SyncStrategyIncremental,
		StartTime:  time.Now(),
		Active:     true,
	}

	// Register session
	c.mu.Lock()
	c.activeSyncs[sessionID] = session
	c.mu.Unlock()

	// Start sync routine
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
		return fmt.Errorf("sync session not found: %s", sessionID)
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
