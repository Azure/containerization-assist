package tools

import (
	"encoding/json"
	"testing"
)

func TestRemoveCopilotIncompatible(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected map[string]any
	}{
		{
			name: "removes $schema field",
			input: map[string]any{
				"$schema": "https://json-schema.org/draft/2020-12/schema",
				"type":    "object",
			},
			expected: map[string]any{
				"type": "object",
			},
		},
		{
			name: "removes $id field",
			input: map[string]any{
				"$id":  "https://example.com/schema",
				"type": "object",
			},
			expected: map[string]any{
				"type": "object",
			},
		},
		{
			name: "removes $dynamicRef field",
			input: map[string]any{
				"$dynamicRef": "#meta",
				"type":        "object",
			},
			expected: map[string]any{
				"type": "object",
			},
		},
		{
			name: "removes $dynamicAnchor field",
			input: map[string]any{
				"$dynamicAnchor": "meta",
				"type":           "object",
			},
			expected: map[string]any{
				"type": "object",
			},
		},
		{
			name: "removes fields recursively in properties",
			input: map[string]any{
				"$schema": "https://json-schema.org/draft/2020-12/schema",
				"type":    "object",
				"properties": map[string]any{
					"field1": map[string]any{
						"$id":  "field1",
						"type": "string",
					},
					"field2": map[string]any{
						"$dynamicRef": "#ref",
						"type":        "number",
					},
				},
			},
			expected: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"field1": map[string]any{
						"type": "string",
					},
					"field2": map[string]any{
						"type": "number",
					},
				},
			},
		},
		{
			name: "removes fields recursively in arrays",
			input: map[string]any{
				"type": "array",
				"items": []any{
					map[string]any{
						"$schema": "https://json-schema.org/draft/2020-12/schema",
						"type":    "string",
					},
					map[string]any{
						"$id":  "item2",
						"type": "number",
					},
				},
			},
			expected: map[string]any{
				"type": "array",
				"items": []any{
					map[string]any{
						"type": "string",
					},
					map[string]any{
						"type": "number",
					},
				},
			},
		},
		{
			name: "handles deeply nested structures",
			input: map[string]any{
				"$schema": "https://json-schema.org/draft/2020-12/schema",
				"type":    "object",
				"properties": map[string]any{
					"level1": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"level2": map[string]any{
								"$id":  "nested",
								"type": "array",
								"items": map[string]any{
									"$dynamicRef": "#ref",
									"type":        "string",
								},
							},
						},
					},
				},
			},
			expected: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"level1": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"level2": map[string]any{
								"type": "array",
								"items": map[string]any{
									"type": "string",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "preserves valid fields",
			input: map[string]any{
				"type":        "object",
				"description": "A test object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "The name field",
						"minLength":   1,
						"maxLength":   100,
					},
				},
				"required": []any{"name"},
			},
			expected: map[string]any{
				"type":        "object",
				"description": "A test object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "The name field",
						"minLength":   1,
						"maxLength":   100,
					},
				},
				"required": []any{"name"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Clone the input to avoid modifying test data
			input := cloneMap(tt.input)
			removeCopilotIncompatible(input)

			// Compare the result
			if !mapsEqual(input, tt.expected) {
				inputJSON, err := json.MarshalIndent(input, "", "  ")
				if err != nil {
					inputJSON = []byte("error marshaling input")
				}
				expectedJSON, err := json.MarshalIndent(tt.expected, "", "  ")
				if err != nil {
					expectedJSON = []byte("error marshaling expected")
				}
				t.Errorf("removeCopilotIncompatible() = %s, want %s", inputJSON, expectedJSON)
			}
		})
	}
}

// Helper function to clone a map
func cloneMap(m map[string]any) map[string]any {
	clone := make(map[string]any)
	for k, v := range m {
		switch val := v.(type) {
		case map[string]any:
			clone[k] = cloneMap(val)
		case []any:
			clone[k] = cloneSlice(val)
		default:
			clone[k] = v
		}
	}
	return clone
}

// Helper function to clone a slice
func cloneSlice(s []any) []any {
	clone := make([]any, len(s))
	for i, v := range s {
		switch val := v.(type) {
		case map[string]any:
			clone[i] = cloneMap(val)
		case []any:
			clone[i] = cloneSlice(val)
		default:
			clone[i] = v
		}
	}
	return clone
}

// Helper function to compare maps
func mapsEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v1 := range a {
		v2, ok := b[k]
		if !ok {
			return false
		}
		switch val1 := v1.(type) {
		case map[string]any:
			val2, ok := v2.(map[string]any)
			if !ok || !mapsEqual(val1, val2) {
				return false
			}
		case []any:
			val2, ok := v2.([]any)
			if !ok || !slicesEqual(val1, val2) {
				return false
			}
		default:
			if v1 != v2 {
				return false
			}
		}
	}
	return true
}

// Helper function to compare slices
func slicesEqual(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		switch val1 := a[i].(type) {
		case map[string]any:
			val2, ok := b[i].(map[string]any)
			if !ok || !mapsEqual(val1, val2) {
				return false
			}
		case []any:
			val2, ok := b[i].([]any)
			if !ok || !slicesEqual(val1, val2) {
				return false
			}
		default:
			if a[i] != b[i] {
				return false
			}
		}
	}
	return true
}
