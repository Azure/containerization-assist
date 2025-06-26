package core

import (
	"reflect"
	"testing"
)

type TestArgs struct {
	SessionID    string            `json:"sessionId"`
	RepoURL      string            `json:"repoUrl"`
	Branch       string            `json:"branch"`
	Context      string            `json:"context"`
	LanguageHint string            `json:"languageHint"`
	Shallow      bool              `json:"shallow"`
	DryRun       bool              `json:"dryRun"`
	BuildArgs    map[string]string `json:"buildArgs"`
	Tags         []string          `json:"tags"`
	OptionalPtr  *string           `json:"optionalPtr,omitempty"`
}

type TestArgsSnakeCase struct {
	SessionID          string `json:"session_id"`
	IncludeHealthCheck bool   `json:"include_health_check"`
	PushAfterBuild     bool   `json:"push_after_build"`
}

func TestBuildArgsMapDebug(t *testing.T) {
	args := TestArgs{
		SessionID: "test",
		RepoURL:   "https://example.com",
		Tags:      []string{"tag1", "tag2"},
	}

	result, err := BuildArgsMap(args)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	t.Logf("Result: %+v", result)
	for k, v := range result {
		t.Logf("  %s: %T = %v", k, v, v)
	}
}

func TestBuildArgsMap(t *testing.T) {
	tests := []struct {
		name     string
		args     interface{}
		expected map[string]interface{}
		wantErr  bool
	}{
		{
			name: "basic struct with JSON tags",
			args: TestArgs{
				SessionID:    "test-session",
				RepoURL:      "https://github.com/test/repo",
				Branch:       "main",
				Context:      "test-context",
				LanguageHint: "go",
				Shallow:      true,
				DryRun:       false,
				BuildArgs:    map[string]string{"KEY": "value"},
				Tags:         []string{"tag1", "tag2"},
			},
			expected: map[string]interface{}{
				"sessionId":    "test-session",
				"repoUrl":      "https://github.com/test/repo",
				"branch":       "main",
				"context":      "test-context",
				"languageHint": "go",
				"shallow":      true,
				"dryRun":       false,
				"buildArgs":    map[string]string{"KEY": "value"},
				"tags":         []interface{}{"tag1", "tag2"},
				"optionalPtr":  (*string)(nil),
			},
			wantErr: false,
		},
		{
			name: "struct with snake_case JSON tags",
			args: TestArgsSnakeCase{
				SessionID:          "test-session",
				IncludeHealthCheck: true,
				PushAfterBuild:     false,
			},
			expected: map[string]interface{}{
				"session_id":           "test-session",
				"include_health_check": true,
				"push_after_build":     false,
			},
			wantErr: false,
		},
		{
			name: "pointer to struct",
			args: &TestArgs{
				SessionID: "test-session",
				RepoURL:   "https://github.com/test/repo",
			},
			expected: map[string]interface{}{
				"sessionId":    "test-session",
				"repoUrl":      "https://github.com/test/repo",
				"branch":       "",
				"context":      "",
				"languageHint": "",
				"shallow":      false,
				"dryRun":       false,
				"buildArgs":    map[string]string(nil),
				"tags":         []string(nil),
				"optionalPtr":  (*string)(nil),
			},
			wantErr: false,
		},
		{
			name:    "nil pointer",
			args:    (*TestArgs)(nil),
			wantErr: true,
		},
		{
			name:    "nil interface",
			args:    nil,
			wantErr: true,
		},
		{
			name:    "non-struct type",
			args:    "not a struct",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := BuildArgsMap(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Errorf("BuildArgsMap() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("BuildArgsMap() unexpected error: %v", err)
				return
			}

			// Check each field individually for better error messages
			for k, expectedVal := range tt.expected {
				actualVal, exists := result[k]
				if !exists {
					t.Errorf("BuildArgsMap() missing key %s", k)
					continue
				}
				if !reflect.DeepEqual(actualVal, expectedVal) {
					t.Errorf("BuildArgsMap() key %s mismatch\nGot:      %+v (%T)\nExpected: %+v (%T)",
						k, actualVal, actualVal, expectedVal, expectedVal)
				}
			}

			// Check for extra keys
			for k := range result {
				if _, exists := tt.expected[k]; !exists {
					t.Errorf("BuildArgsMap() unexpected key %s with value %+v", k, result[k])
				}
			}
		})
	}
}

func TestGetFieldName(t *testing.T) {
	tests := []struct {
		name     string
		field    reflect.StructField
		expected string
	}{
		{
			name: "JSON tag with camelCase",
			field: reflect.StructField{
				Name: "SessionID",
				Tag:  `json:"sessionId"`,
			},
			expected: "sessionId",
		},
		{
			name: "JSON tag with snake_case",
			field: reflect.StructField{
				Name: "IncludeHealthCheck",
				Tag:  `json:"include_health_check"`,
			},
			expected: "include_health_check",
		},
		{
			name: "JSON tag with omitempty",
			field: reflect.StructField{
				Name: "OptionalField",
				Tag:  `json:"optionalField,omitempty"`,
			},
			expected: "optionalField",
		},
		{
			name: "No JSON tag - convert to camelCase",
			field: reflect.StructField{
				Name: "SessionID",
			},
			expected: "sessionID",
		},
		{
			name: "Empty JSON tag - convert to camelCase",
			field: reflect.StructField{
				Name: "RepoURL",
				Tag:  `json:""`,
			},
			expected: "repoURL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getFieldName(tt.field)
			if result != tt.expected {
				t.Errorf("getFieldName() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestConvertSliceToInterfaceSlice(t *testing.T) {
	tests := []struct {
		name     string
		slice    interface{}
		expected []interface{}
	}{
		{
			name:     "string slice",
			slice:    []string{"a", "b", "c"},
			expected: []interface{}{"a", "b", "c"},
		},
		{
			name:     "int slice",
			slice:    []int{1, 2, 3},
			expected: []interface{}{1, 2, 3},
		},
		{
			name:     "empty slice",
			slice:    []string{},
			expected: []interface{}{},
		},
		{
			name:     "nil slice",
			slice:    []string(nil),
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := reflect.ValueOf(tt.slice)
			result := convertSliceToInterfaceSlice(v)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("convertSliceToInterfaceSlice() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
