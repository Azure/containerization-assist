package state

import (
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/internal/session"
)

// SessionToConversationMapping maps session state to conversation state
type SessionToConversationMapping struct{}

// NewSessionToConversationMapping creates a new session to conversation mapping
func NewSessionToConversationMapping() StateMapping {
	return &SessionToConversationMapping{}
}

// MapState maps session state to conversation state
func (m *SessionToConversationMapping) MapState(source interface{}) (interface{}, error) {
	sessionState, ok := source.(*session.SessionState)
	if !ok {
		return nil, fmt.Errorf("expected *session.SessionState, got %T", source)
	}

	// Create conversation state from session state
	conversationState := &BasicConversationState{
		SessionState:   *sessionState,
		ConversationID: fmt.Sprintf("conv_%s", sessionState.SessionID),
		CurrentStage:   "planning", // Default stage
		History:        make([]BasicConversationEntry, 0),
		Decisions:      make(map[string]BasicDecision),
		Artifacts:      make(map[string]BasicArtifact),
	}

	return conversationState, nil
}

// SupportsReverse indicates if reverse mapping is supported
func (m *SessionToConversationMapping) SupportsReverse() bool {
	return true
}

// ReverseMap maps conversation state back to session state
func (m *SessionToConversationMapping) ReverseMap(target interface{}) (interface{}, error) {
	conversationState, ok := target.(*BasicConversationState)
	if !ok {
		return nil, fmt.Errorf("expected *BasicConversationState, got %T", target)
	}

	// Extract session state from conversation state
	return &conversationState.SessionState, nil
}

// WorkflowToSessionMapping maps workflow state to session state
type WorkflowToSessionMapping struct{}

// NewWorkflowToSessionMapping creates a new workflow to session mapping
func NewWorkflowToSessionMapping() StateMapping {
	return &WorkflowToSessionMapping{}
}

// MapState maps workflow state to session state updates
func (m *WorkflowToSessionMapping) MapState(source interface{}) (interface{}, error) {
	workflowSession, ok := source.(WorkflowSessionInterface)
	if !ok {
		return nil, fmt.Errorf("expected WorkflowSessionInterface, got %T", source)
	}

	// Create a session state update that reflects workflow progress
	updates := map[string]interface{}{
		"workflow_id":     workflowSession.GetSessionID(),
		"workflow_status": "active", // simplified status
		"current_stage":   workflowSession.GetCurrentStage(),
		"progress":        workflowSession.GetProgress(),
	}

	return updates, nil
}

// SupportsReverse indicates if reverse mapping is supported
func (m *WorkflowToSessionMapping) SupportsReverse() bool {
	return false // One-way mapping only
}

// ReverseMap is not supported for this mapping
func (m *WorkflowToSessionMapping) ReverseMap(target interface{}) (interface{}, error) {
	return nil, fmt.Errorf("reverse mapping not supported for WorkflowToSessionMapping")
}

// Removed unused helper methods - interface methods are called directly

// ToolStateMapping provides generic tool state mapping
type ToolStateMapping struct {
	sourceToolName string
	targetToolName string
	fieldMappings  map[string]string
}

// NewToolStateMapping creates a new tool state mapping
func NewToolStateMapping(sourceToolName, targetToolName string) *ToolStateMapping {
	return &ToolStateMapping{
		sourceToolName: sourceToolName,
		targetToolName: targetToolName,
		fieldMappings:  make(map[string]string),
	}
}

// AddFieldMapping adds a field mapping
func (m *ToolStateMapping) AddFieldMapping(sourceField, targetField string) {
	m.fieldMappings[sourceField] = targetField
}

// MapState maps tool state using field mappings
func (m *ToolStateMapping) MapState(source interface{}) (interface{}, error) {
	// Use reflection to map fields
	sourceMap, ok := source.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("tool state must be a map, got %T", source)
	}

	targetMap := make(map[string]interface{})
	targetMap["tool_name"] = m.targetToolName

	// Map fields according to field mappings
	for sourceField, targetField := range m.fieldMappings {
		if value, exists := sourceMap[sourceField]; exists {
			targetMap[targetField] = value
		}
	}

	return targetMap, nil
}

// SupportsReverse indicates if reverse mapping is supported
func (m *ToolStateMapping) SupportsReverse() bool {
	return true
}

// ReverseMap reverses the mapping
func (m *ToolStateMapping) ReverseMap(target interface{}) (interface{}, error) {
	targetMap, ok := target.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("tool state must be a map, got %T", target)
	}

	sourceMap := make(map[string]interface{})
	sourceMap["tool_name"] = m.sourceToolName

	// Reverse map fields
	for sourceField, targetField := range m.fieldMappings {
		if value, exists := targetMap[targetField]; exists {
			sourceMap[sourceField] = value
		}
	}

	return sourceMap, nil
}

// CompositeMapping combines multiple mappings
type CompositeMapping struct {
	mappings []StateMapping
}

// NewCompositeMapping creates a new composite mapping
func NewCompositeMapping(mappings ...StateMapping) StateMapping {
	return &CompositeMapping{
		mappings: mappings,
	}
}

// MapState applies all mappings in sequence
func (m *CompositeMapping) MapState(source interface{}) (interface{}, error) {
	current := source
	for i, mapping := range m.mappings {
		result, err := mapping.MapState(current)
		if err != nil {
			return nil, fmt.Errorf("mapping %d failed: %w", i, err)
		}
		current = result
	}
	return current, nil
}

// SupportsReverse checks if all mappings support reverse
func (m *CompositeMapping) SupportsReverse() bool {
	for _, mapping := range m.mappings {
		if !mapping.SupportsReverse() {
			return false
		}
	}
	return true
}

// ReverseMap applies all mappings in reverse order
func (m *CompositeMapping) ReverseMap(target interface{}) (interface{}, error) {
	if !m.SupportsReverse() {
		return nil, fmt.Errorf("reverse mapping not supported")
	}

	current := target
	// Apply mappings in reverse order
	for i := len(m.mappings) - 1; i >= 0; i-- {
		result, err := m.mappings[i].ReverseMap(current)
		if err != nil {
			return nil, fmt.Errorf("reverse mapping %d failed: %w", i, err)
		}
		current = result
	}
	return current, nil
}

// FilteredMapping applies mapping with filtering
type FilteredMapping struct {
	baseMapping StateMapping
	filter      func(interface{}) bool
}

// NewFilteredMapping creates a new filtered mapping
func NewFilteredMapping(baseMapping StateMapping, filter func(interface{}) bool) StateMapping {
	return &FilteredMapping{
		baseMapping: baseMapping,
		filter:      filter,
	}
}

// MapState applies mapping only if filter passes
func (m *FilteredMapping) MapState(source interface{}) (interface{}, error) {
	if !m.filter(source) {
		return nil, fmt.Errorf("state filtered out")
	}
	return m.baseMapping.MapState(source)
}

// SupportsReverse delegates to base mapping
func (m *FilteredMapping) SupportsReverse() bool {
	return m.baseMapping.SupportsReverse()
}

// ReverseMap applies reverse mapping only if filter passes
func (m *FilteredMapping) ReverseMap(target interface{}) (interface{}, error) {
	if !m.filter(target) {
		return nil, fmt.Errorf("state filtered out")
	}
	return m.baseMapping.ReverseMap(target)
}
