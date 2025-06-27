package utils

import (
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test structs
type BaseArgs struct {
	SessionID string `json:"sessionId"`
	DryRun    bool   `json:"dryRun"`
}

type SimpleArgs struct {
	BaseArgs
	ImageName string `json:"imageName"`
	Tag       string `json:"tag"`
}

type ComplexArgs struct {
	BaseArgs
	BuildArgs map[string]string `json:"buildArgs"`
	Platform  string            `json:"platform"`
	NoCache   bool              `json:"noCache"`
	VulnTypes []string          `json:"vulnTypes"`
	Port      int               `json:"port"`
	Replicas  int32             `json:"replicas"`
}

type LegacyArgs struct {
	SessionID          string            `mapkey:"session_id"`
	IncludeHealthCheck bool              `mapkey:"include_health_check"`
	BuildArgs          map[string]string `mapkey:"build_args"`
	VulnTypes          []string          `mapkey:"vuln_types"`
}

type NoTagsArgs struct {
	SessionID string
	ImageName string
	DryRun    bool
}

func TestBuildArgsMap_SimpleStruct(t *testing.T) {
	args := &SimpleArgs{
		BaseArgs: BaseArgs{
			SessionID: "test-session-123",
			DryRun:    false,
		},
		ImageName: "myapp",
		Tag:       "v1.0.0",
	}

	result, err := BuildArgsMap(args)
	require.NoError(t, err)

	expected := map[string]interface{}{
		"sessionId": "test-session-123",
		"dryRun":    false,
		"imageName": "myapp",
		"tag":       "v1.0.0",
	}

	assert.Equal(t, expected, result)
}

func TestBuildArgsMap_ComplexStruct(t *testing.T) {
	args := &ComplexArgs{
		BaseArgs: BaseArgs{
			SessionID: "test-session-456",
			DryRun:    true,
		},
		BuildArgs: map[string]string{"ENV": "production", "VERSION": "1.0.0"},
		Platform:  "linux/amd64",
		NoCache:   true,
		VulnTypes: []string{"os", "library"},
		Port:      8080,
		Replicas:  3,
	}

	result, err := BuildArgsMap(args)
	require.NoError(t, err)

	expected := map[string]interface{}{
		"sessionId": "test-session-456",
		"dryRun":    true,
		"buildArgs": map[string]string{"ENV": "production", "VERSION": "1.0.0"},
		"platform":  "linux/amd64",
		"noCache":   true,
		"vulnTypes": []string{"os", "library"},
		"port":      8080,
		"replicas":  int32(3),
	}

	assert.Equal(t, expected, result)
}

func TestBuildArgsMap_LegacyMapkeyTags(t *testing.T) {
	args := &LegacyArgs{
		SessionID:          "test-session-789",
		IncludeHealthCheck: true,
		BuildArgs:          map[string]string{"DEBUG": "true"},
		VulnTypes:          []string{"config"},
	}

	result, err := BuildArgsMap(args)
	require.NoError(t, err)

	expected := map[string]interface{}{
		"session_id":           "test-session-789",
		"include_health_check": true,
		"build_args":           map[string]string{"DEBUG": "true"},
		"vuln_types":           []string{"config"},
	}

	assert.Equal(t, expected, result)
}

func TestBuildArgsMap_NoTags(t *testing.T) {
	args := &NoTagsArgs{
		SessionID: "test-session-notags",
		ImageName: "myapp",
		DryRun:    false,
	}

	result, err := BuildArgsMap(args)
	require.NoError(t, err)

	expected := map[string]interface{}{
		"session_id": "test-session-notags",
		"image_name": "myapp",
		"dry_run":    false,
	}

	assert.Equal(t, expected, result)
}

func TestBuildArgsMap_NilInput(t *testing.T) {
	result, err := BuildArgsMap(nil)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "args cannot be nil")
}

func TestBuildArgsMap_NilPointer(t *testing.T) {
	var args *SimpleArgs
	result, err := BuildArgsMap(args)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "args cannot be nil pointer")
}

func TestBuildArgsMap_NonStruct(t *testing.T) {
	result, err := BuildArgsMap("not a struct")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "args must be a struct")
}

func TestBuildArgsMap_EmptyStruct(t *testing.T) {
	type EmptyStruct struct{}
	args := &EmptyStruct{}

	result, err := BuildArgsMap(args)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestBuildArgsMap_DefaultValues(t *testing.T) {
	args := &ComplexArgs{
		BaseArgs: BaseArgs{
			SessionID: "test-defaults",
			// DryRun defaults to false
		},
		// BuildArgs defaults to nil
		// Platform defaults to empty string
		// NoCache defaults to false
		// VulnTypes defaults to nil
		Port:     8080,
		Replicas: 0,
	}

	result, err := BuildArgsMap(args)
	require.NoError(t, err)

	expected := map[string]interface{}{
		"sessionId": "test-defaults",
		"dryRun":    false,
		"buildArgs": map[string]string(nil),
		"platform":  "",
		"noCache":   false,
		"vulnTypes": []string(nil),
		"port":      8080,
		"replicas":  int32(0),
	}

	assert.Equal(t, expected, result)
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"SessionID", "session_id"},
		{"ImageName", "image_name"},
		{"DryRun", "dry_run"},
		{"IncludeHealthCheck", "include_health_check"},
		{"VulnTypes", "vuln_types"},
		{"CPULimit", "cpu_limit"},
		{"HTTPPort", "http_port"},
		{"simple", "simple"},
		{"A", "a"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := utils.ToSnakeCase(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertSliceToInterfaceSlice(t *testing.T) {
	t.Run("string slice", func(t *testing.T) {
		input := []string{"a", "b", "c"}
		result := ConvertSliceToInterfaceSlice(input)
		expected := []interface{}{"a", "b", "c"}
		assert.Equal(t, expected, result)
	})

	t.Run("int slice", func(t *testing.T) {
		input := []int{1, 2, 3}
		result := ConvertSliceToInterfaceSlice(input)
		expected := []interface{}{1, 2, 3}
		assert.Equal(t, expected, result)
	})

	t.Run("empty slice", func(t *testing.T) {
		input := []string{}
		result := ConvertSliceToInterfaceSlice(input)
		expected := []interface{}{}
		assert.Equal(t, expected, result)
	})

	t.Run("nil slice", func(t *testing.T) {
		var input []string
		result := ConvertSliceToInterfaceSlice(input)
		assert.Nil(t, result)
	})
}

// Benchmark to ensure the reflection-based approach is performant
func BenchmarkBuildArgsMap(b *testing.B) {
	args := &ComplexArgs{
		BaseArgs: BaseArgs{
			SessionID: "test-session-bench",
			DryRun:    true,
		},
		BuildArgs: map[string]string{"ENV": "production"},
		Platform:  "linux/amd64",
		NoCache:   true,
		VulnTypes: []string{"os", "library"},
		Port:      8080,
		Replicas:  3,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := BuildArgsMap(args)
		if err != nil {
			b.Fatal(err)
		}
		// Prevent compiler optimization
		_ = result
	}
}

// Benchmark comparison with manual mapping
func BenchmarkManualMapping(b *testing.B) {
	args := &ComplexArgs{
		BaseArgs: BaseArgs{
			SessionID: "test-session-bench",
			DryRun:    true,
		},
		BuildArgs: map[string]string{"ENV": "production"},
		Platform:  "linux/amd64",
		NoCache:   true,
		VulnTypes: []string{"os", "library"},
		Port:      8080,
		Replicas:  3,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := map[string]interface{}{
			"sessionId": args.SessionID,
			"dryRun":    args.DryRun,
			"buildArgs": args.BuildArgs,
			"platform":  args.Platform,
			"noCache":   args.NoCache,
			"vulnTypes": args.VulnTypes,
			"port":      args.Port,
			"replicas":  args.Replicas,
		}
		// Prevent compiler optimization
		_ = result
	}
}
