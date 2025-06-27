package state

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
)

// SessionStateMigrator handles session state migrations
type SessionStateMigrator struct {
	migrations map[string]func(interface{}) (interface{}, error)
}

// NewSessionStateMigrator creates a new session state migrator
func NewSessionStateMigrator() StateMigrator {
	m := &SessionStateMigrator{
		migrations: make(map[string]func(interface{}) (interface{}, error)),
	}

	// Register migrations
	m.migrations["v1_to_v2"] = m.migrateV1ToV2
	m.migrations["v2_to_v3"] = m.migrateV2ToV3

	return m
}

// MigrateState migrates session state between versions
func (m *SessionStateMigrator) MigrateState(ctx context.Context, stateType StateType, fromVersion, toVersion string, state interface{}) (interface{}, error) {
	migrationKey := fmt.Sprintf("%s_to_%s", fromVersion, toVersion)
	migrationFunc, exists := m.migrations[migrationKey]
	if !exists {
		return nil, fmt.Errorf("no migration path from %s to %s", fromVersion, toVersion)
	}

	return migrationFunc(state)
}

// migrateV1ToV2 migrates session state from v1 to v2
func (m *SessionStateMigrator) migrateV1ToV2(state interface{}) (interface{}, error) {
	// Example migration: Add new fields with defaults
	v1State := state.(map[string]interface{})

	// Add new fields
	v1State["version"] = "v2"
	v1State["features"] = map[string]bool{
		"ai_assistance": true,
		"auto_retry":    true,
	}

	return v1State, nil
}

// migrateV2ToV3 migrates session state from v2 to v3
func (m *SessionStateMigrator) migrateV2ToV3(state interface{}) (interface{}, error) {
	// Example migration: Restructure data
	v2State := state.(map[string]interface{})

	// Restructure fields
	v3State := map[string]interface{}{
		"version": "v3",
		"metadata": map[string]interface{}{
			"created_at": v2State["created_at"],
			"updated_at": v2State["updated_at"],
		},
		"data": v2State,
	}

	return v3State, nil
}

// GenericStateMigrator provides generic state migration capabilities
type GenericStateMigrator struct {
	transformers map[string]StateTransformer
}

// StateTransformer transforms state from one version to another
type StateTransformer interface {
	Transform(state interface{}) (interface{}, error)
	SourceVersion() string
	TargetVersion() string
}

// NewGenericStateMigrator creates a new generic state migrator
func NewGenericStateMigrator() *GenericStateMigrator {
	return &GenericStateMigrator{
		transformers: make(map[string]StateTransformer),
	}
}

// RegisterTransformer registers a state transformer
func (m *GenericStateMigrator) RegisterTransformer(transformer StateTransformer) {
	key := fmt.Sprintf("%s_to_%s", transformer.SourceVersion(), transformer.TargetVersion())
	m.transformers[key] = transformer
}

// MigrateState migrates state using registered transformers
func (m *GenericStateMigrator) MigrateState(ctx context.Context, stateType StateType, fromVersion, toVersion string, state interface{}) (interface{}, error) {
	// Find migration path
	path := m.findMigrationPath(fromVersion, toVersion)
	if len(path) == 0 {
		return nil, fmt.Errorf("no migration path from %s to %s", fromVersion, toVersion)
	}

	// Apply transformations in sequence
	currentState := state
	for _, transformer := range path {
		newState, err := transformer.Transform(currentState)
		if err != nil {
			return nil, fmt.Errorf("transformation failed at %s->%s: %w",
				transformer.SourceVersion(), transformer.TargetVersion(), err)
		}
		currentState = newState
	}

	return currentState, nil
}

// findMigrationPath finds a path of transformers from source to target version
func (m *GenericStateMigrator) findMigrationPath(fromVersion, toVersion string) []StateTransformer {
	// Simple direct path lookup for now
	key := fmt.Sprintf("%s_to_%s", fromVersion, toVersion)
	if transformer, exists := m.transformers[key]; exists {
		return []StateTransformer{transformer}
	}

	// TODO: Implement path finding for multi-step migrations
	return nil
}

// JSONStateTransformer transforms state using JSON manipulation
type JSONStateTransformer struct {
	sourceVersion string
	targetVersion string
	transform     func(map[string]interface{}) (map[string]interface{}, error)
}

// NewJSONStateTransformer creates a new JSON state transformer
func NewJSONStateTransformer(source, target string, transform func(map[string]interface{}) (map[string]interface{}, error)) StateTransformer {
	return &JSONStateTransformer{
		sourceVersion: source,
		targetVersion: target,
		transform:     transform,
	}
}

// Transform transforms the state
func (t *JSONStateTransformer) Transform(state interface{}) (interface{}, error) {
	// Convert to JSON map
	jsonData, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal state: %w", err)
	}

	var stateMap map[string]interface{}
	if err := json.Unmarshal(jsonData, &stateMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	// Apply transformation
	transformedMap, err := t.transform(stateMap)
	if err != nil {
		return nil, err
	}

	// Convert back to original type if possible
	transformedJSON, err := json.Marshal(transformedMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transformed state: %w", err)
	}

	// Try to unmarshal back to original type
	result := reflect.New(reflect.TypeOf(state)).Interface()
	if err := json.Unmarshal(transformedJSON, result); err != nil {
		// If unmarshal fails, return the map
		return transformedMap, nil
	}

	return result, nil
}

// SourceVersion returns the source version
func (t *JSONStateTransformer) SourceVersion() string {
	return t.sourceVersion
}

// TargetVersion returns the target version
func (t *JSONStateTransformer) TargetVersion() string {
	return t.targetVersion
}

// WorkflowStateMigrator handles workflow state migrations
type WorkflowStateMigrator struct {
	transformers []StateTransformer
}

// NewWorkflowStateMigrator creates a new workflow state migrator
func NewWorkflowStateMigrator() StateMigrator {
	m := &WorkflowStateMigrator{
		transformers: make([]StateTransformer, 0),
	}

	// Register default transformers
	m.transformers = append(m.transformers, NewJSONStateTransformer("1.0", "1.1", func(state map[string]interface{}) (map[string]interface{}, error) {
		// Add checkpoint support
		state["checkpoints"] = []interface{}{}
		state["checkpoint_enabled"] = true
		return state, nil
	}))

	return m
}

// MigrateState migrates workflow state
func (m *WorkflowStateMigrator) MigrateState(ctx context.Context, stateType StateType, fromVersion, toVersion string, state interface{}) (interface{}, error) {
	// Find appropriate transformer
	for _, transformer := range m.transformers {
		if transformer.SourceVersion() == fromVersion && transformer.TargetVersion() == toVersion {
			return transformer.Transform(state)
		}
	}

	return nil, fmt.Errorf("no transformer found for %s to %s", fromVersion, toVersion)
}
