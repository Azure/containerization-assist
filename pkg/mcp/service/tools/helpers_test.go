package tools

import (
"context"
"log/slog"
"testing"

"github.com/mark3labs/mcp-go/mcp"
"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"
)

func TestGenerateSessionID(t *testing.T) {
	// Generate multiple IDs
	id1 := GenerateSessionID()
	id2 := GenerateSessionID()

	// Verify format
	assert.True(t, len(id1) > 3)
	assert.True(t, len(id2) > 3)
	assert.True(t, id1[:3] == "wf_")
	assert.True(t, id2[:3] == "wf_")

	// Verify uniqueness
	assert.NotEqual(t, id1, id2)
}

func TestExtractStringParam(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]interface{}
		key     string
		want    string
		wantErr bool
	}{
		{
			name: "valid string param",
			args: map[string]interface{}{
				"param1": "value1",
			},
			key:     "param1",
			want:    "value1",
			wantErr: false,
		},
		{
			name: "missing param",
			args: map[string]interface{}{
				"other": "value",
			},
			key:     "param1",
			want:    "",
			wantErr: true,
		},
		{
			name: "non-string param",
			args: map[string]interface{}{
				"param1": 123,
			},
			key:     "param1",
			want:    "",
			wantErr: true,
		},
		{
			name: "empty string param",
			args: map[string]interface{}{
				"param1": "",
			},
			key:     "param1",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractStringParam(tt.args, tt.key)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestExtractOptionalStringParam(t *testing.T) {
	tests := []struct {
		name         string
		args         map[string]interface{}
		key          string
		defaultValue string
		want         string
	}{
		{
			name: "param exists",
			args: map[string]interface{}{
				"param1": "value1",
			},
			key:          "param1",
			defaultValue: "default",
			want:         "value1",
		},
		{
			name:         "param missing",
			args:         map[string]interface{}{},
			key:          "param1",
			defaultValue: "default",
			want:         "default",
		},
		{
			name: "param empty",
			args: map[string]interface{}{
				"param1": "",
			},
			key:          "param1",
			defaultValue: "default",
			want:         "default",
		},
		{
			name: "param not string",
			args: map[string]interface{}{
				"param1": 123,
			},
			key:          "param1",
			defaultValue: "default",
			want:         "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractOptionalStringParam(tt.args, tt.key, tt.defaultValue)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractStringArrayParam(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]interface{}
		key     string
		want    []string
		wantErr bool
	}{
		{
			name: "string array",
			args: map[string]interface{}{
				"param1": []string{"a", "b", "c"},
			},
			key:     "param1",
			want:    []string{"a", "b", "c"},
			wantErr: false,
		},
		{
			name: "interface array",
			args: map[string]interface{}{
				"param1": []interface{}{"a", "b", "c"},
			},
			key:     "param1",
			want:    []string{"a", "b", "c"},
			wantErr: false,
		},
		{
			name:    "missing param",
			args:    map[string]interface{}{},
			key:     "param1",
			want:    nil,
			wantErr: false,
		},
		{
			name: "non-array param",
			args: map[string]interface{}{
				"param1": "not an array",
			},
			key:     "param1",
			want:    nil,
			wantErr: true,
		},
		{
			name: "mixed type array",
			args: map[string]interface{}{
				"param1": []interface{}{"a", 123, "c"},
			},
			key:     "param1",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractStringArrayParam(tt.args, tt.key)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestCreateProgressEmitter(t *testing.T) {
	t.Run("creates progress emitter", func(t *testing.T) {
		ctx := context.Background()
		req := &mcp.CallToolRequest{}
		totalSteps := 5
		logger := slog.Default()

		emitter := CreateProgressEmitter(ctx, req, totalSteps, logger)
		require.NotNil(t, emitter)
	})
}

func TestMarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		data interface{}
		want string
	}{
		{
			name: "simple map",
			data: map[string]interface{}{
				"key": "value",
			},
			want: `{"key":"value"}`,
		},
		{
			name: "nil data",
			data: nil,
			want: "null",
		},
		{
			name: "complex structure",
			data: struct {
				Name  string `json:"name"`
				Count int    `json:"count"`
			}{
				Name:  "test",
				Count: 42,
			},
			want: `{"name":"test","count":42}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MarshalJSON(tt.data)
			assert.Equal(t, tt.want, got)
		})
	}
}
