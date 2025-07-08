package core

import (
	"encoding/json"
	"time"
)

// TypedErrorDetails represents structured error details instead of interface{}
type TypedErrorDetails struct {
	Field      string             `json:"field,omitempty"`
	Value      string             `json:"value,omitempty"`
	Constraint string             `json:"constraint,omitempty"`
	Context    map[string]string  `json:"context,omitempty"`
	StackTrace []string           `json:"stack_trace,omitempty"`
	InnerError *TypedErrorDetails `json:"inner_error,omitempty"`
	Metadata   map[string]string  `json:"metadata,omitempty"`
}

// TypedToolExample represents a typed tool example instead of interface{}
type TypedToolExample struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Input       json.RawMessage `json:"input"`
	Output      json.RawMessage `json:"output"`
	Scenario    string          `json:"scenario,omitempty"`
	Notes       []string        `json:"notes,omitempty"`
}

// TypedSessionMetadata represents typed session metadata instead of map[string]string
type TypedSessionMetadata struct {
	UserID       string            `json:"user_id,omitempty"`
	Source       string            `json:"source,omitempty"`
	Environment  string            `json:"environment,omitempty"`
	LastTool     string            `json:"last_tool,omitempty"`
	ToolCount    int               `json:"tool_count,omitempty"`
	LastActivity time.Time         `json:"last_activity,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	CustomFields map[string]string `json:"custom_fields,omitempty"`
}

// ToMap converts TypedErrorDetails to map[string]interface{} for backward compatibility
func (d *TypedErrorDetails) ToMap() map[string]interface{} {
	if d == nil {
		return nil
	}

	result := make(map[string]interface{})

	if d.Field != "" {
		result["field"] = d.Field
	}
	if d.Value != "" {
		result["value"] = d.Value
	}
	if d.Constraint != "" {
		result["constraint"] = d.Constraint
	}
	if len(d.Context) > 0 {
		result["context"] = d.Context
	}
	if len(d.StackTrace) > 0 {
		result["stack_trace"] = d.StackTrace
	}
	if d.InnerError != nil {
		result["inner_error"] = d.InnerError.ToMap()
	}
	if len(d.Metadata) > 0 {
		result["metadata"] = d.Metadata
	}

	return result
}

// ToInterface converts TypedToolExample to interface{} for backward compatibility
func (e *TypedToolExample) ToInterface() interface{} {
	if e == nil {
		return nil
	}

	result := map[string]interface{}{
		"name":        e.Name,
		"description": e.Description,
		"input":       json.RawMessage(e.Input),
		"output":      json.RawMessage(e.Output),
	}

	if e.Scenario != "" {
		result["scenario"] = e.Scenario
	}
	if len(e.Notes) > 0 {
		result["notes"] = e.Notes
	}

	return result
}

// ToMap converts TypedSessionMetadata to map[string]string for backward compatibility
func (m *TypedSessionMetadata) ToMap() map[string]string {
	if m == nil {
		return nil
	}

	result := make(map[string]string)

	if m.UserID != "" {
		result["user_id"] = m.UserID
	}
	if m.Source != "" {
		result["source"] = m.Source
	}
	if m.Environment != "" {
		result["environment"] = m.Environment
	}
	if m.LastTool != "" {
		result["last_tool"] = m.LastTool
	}
	if m.ToolCount > 0 {
		result["tool_count"] = string(rune(m.ToolCount))
	}
	if !m.LastActivity.IsZero() {
		result["last_activity"] = m.LastActivity.Format(time.RFC3339)
	}
	if len(m.Tags) > 0 {
		// Convert tags slice to comma-separated string for map compatibility
		result["tags"] = ""
		for i, tag := range m.Tags {
			if i > 0 {
				result["tags"] += ","
			}
			result["tags"] += tag
		}
	}

	// Include custom fields
	for k, v := range m.CustomFields {
		result[k] = v
	}

	return result
}
