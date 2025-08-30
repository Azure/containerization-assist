package sampling

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractJSONCandidate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain JSON object",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON with surrounding text",
			input:    `Here is the JSON: {"key": "value"} and some more text`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON array",
			input:    `[1, 2, 3]`,
			expected: `[1, 2, 3]`,
		},
		{
			name:     "nested JSON object",
			input:    `{"outer": {"inner": "value"}}`,
			expected: `{"outer": {"inner": "value"}}`,
		},
		{
			name:     "JSON with code fence",
			input:    "```json\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON with generic code fence",
			input:    "```\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON with escaped quotes",
			input:    `{"message": "He said \"hello\""}`,
			expected: `{"message": "He said \"hello\""}`,
		},
		{
			name: "multiline JSON",
			input: `{
				"key1": "value1",
				"key2": "value2"
			}`,
			expected: `{
				"key1": "value1",
				"key2": "value2"
			}`,
		},
		{
			name:     "no JSON found",
			input:    `This is just plain text`,
			expected: `This is just plain text`,
		},
		{
			name:     "JSON with prefix text",
			input:    `The result is: {"status": "success", "data": {"items": [1, 2, 3]}}`,
			expected: `{"status": "success", "data": {"items": [1, 2, 3]}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSONCandidate(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStripCodeFences(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no code fence",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "json code fence",
			input:    "```json\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON uppercase fence",
			input:    "```JSON\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "javascript fence",
			input:    "```javascript\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "generic fence",
			input:    "```\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "no closing fence",
			input:    "```json\n{\"key\": \"value\"}",
			expected: `{"key": "value"}`,
		},
		{
			name:     "multiple fences (nested not supported)",
			input:    "```json\n{\"outer\": \"```inner```\"}\n```",
			expected: `{"outer": "`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripCodeFences(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildJSONRepairPrompt(t *testing.T) {
	tests := []struct {
		name        string
		schema      string
		invalidJSON string
		lastErr     error
		contains    []string
	}{
		{
			name:        "basic repair prompt",
			schema:      "",
			invalidJSON: `{"key": "value"`,
			lastErr:     fmt.Errorf("unexpected end of JSON input"),
			contains: []string{
				"should be valid JSON",
				"Error: unexpected end of JSON input",
				"corrected valid JSON",
			},
		},
		{
			name:        "with schema",
			schema:      `{"type": "object", "required": ["key"]}`,
			invalidJSON: `{"wrong": "value"}`,
			lastErr:     fmt.Errorf("missing required field: key"),
			contains: []string{
				"must conform to this schema",
				`"type": "object"`,
				"Error: missing required field",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildJSONRepairPrompt(tt.schema, tt.invalidJSON, tt.lastErr)
			for _, expected := range tt.contains {
				assert.Contains(t, result, expected)
			}
		})
	}
}

func TestValidateJSONSchema(t *testing.T) {
	tests := []struct {
		name      string
		jsonStr   string
		schemaStr string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid object against schema",
			jsonStr:   `{"name": "test", "age": 25}`,
			schemaStr: `{"type": "object", "properties": {"name": {"type": "string"}, "age": {"type": "number"}}}`,
			wantErr:   false,
		},
		{
			name:      "missing required field",
			jsonStr:   `{"age": 25}`,
			schemaStr: `{"type": "object", "required": ["name"], "properties": {"name": {"type": "string"}, "age": {"type": "number"}}}`,
			wantErr:   true,
			errMsg:    "missing properties",
		},
		{
			name:      "wrong type",
			jsonStr:   `{"name": "test", "age": "25"}`,
			schemaStr: `{"type": "object", "properties": {"name": {"type": "string"}, "age": {"type": "number"}}}`,
			wantErr:   true,
			errMsg:    "expected",
		},
		{
			name:      "empty schema allows anything",
			jsonStr:   `{"any": "thing"}`,
			schemaStr: ``,
			wantErr:   false,
		},
		{
			name:      "array schema",
			jsonStr:   `[1, 2, 3]`,
			schemaStr: `{"type": "array", "items": {"type": "number"}}`,
			wantErr:   false,
		},
		{
			name:      "invalid JSON",
			jsonStr:   `{"invalid": }`,
			schemaStr: `{"type": "object"}`,
			wantErr:   true,
			errMsg:    "invalid JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateJSONSchema(tt.jsonStr, tt.schemaStr)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMinNonZero(t *testing.T) {
	tests := []struct {
		a, b     int32
		expected int32
	}{
		{0, 0, 0},
		{0, 5, 5},
		{5, 0, 5},
		{3, 5, 3},
		{5, 3, 3},
		{5, 5, 5},
	}

	for _, tt := range tests {
		result := minNonZero(tt.a, tt.b)
		assert.Equal(t, tt.expected, result)
	}
}
