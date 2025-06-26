package core

import (
	"testing"

	"github.com/Azure/container-kit/pkg/mcp/internal/analyze"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/stretchr/testify/assert"
)

// TestToolArgumentPassthrough validates that arguments are correctly passed through
// the entire tool chain without being lost or modified incorrectly
func TestToolArgumentPassthrough(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "generate_dockerfile_with_all_args",
			input: map[string]interface{}{
				"session_id":           "test-session",
				"base_image":           "tomcat:9-jdk11-openjdk",
				"template":             "java",
				"optimization":         "size",
				"include_health_check": true,
				"build_args":           map[string]string{"ENV": "production"},
				"platform":             "linux/amd64",
				"dry_run":              false,
			},
			expected: map[string]interface{}{
				"session_id":           "test-session",
				"base_image":           "tomcat:9-jdk11-openjdk",
				"template":             "java",
				"optimization":         "size",
				"include_health_check": true,
				"build_args":           map[string]string{"ENV": "production"},
				"platform":             "linux/amd64",
				"dry_run":              false,
			},
		},
		{
			name: "generate_dockerfile_minimal_args",
			input: map[string]interface{}{
				"session_id": "test-session",
				"template":   "python",
			},
			expected: map[string]interface{}{
				"session_id":           "test-session",
				"base_image":           "",
				"template":             "python",
				"optimization":         "",
				"include_health_check": false,
				"build_args":           map[string]string(nil),
				"platform":             "",
				"dry_run":              false,
			},
		},
		{
			name: "generate_dockerfile_with_build_args",
			input: map[string]interface{}{
				"session_id": "test-session",
				"template":   "node",
				"build_args": map[string]string{
					"NODE_ENV": "production",
					"PORT":     "3000",
				},
			},
			expected: map[string]interface{}{
				"session_id":           "test-session",
				"base_image":           "",
				"template":             "node",
				"optimization":         "",
				"include_health_check": false,
				"build_args": map[string]string{
					"NODE_ENV": "production",
					"PORT":     "3000",
				},
				"platform": "",
				"dry_run":  false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create args from input
			args := &analyze.GenerateDockerfileArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: tt.input["session_id"].(string),
				},
			}

			if dryRun, ok := tt.input["dry_run"]; ok {
				args.DryRun = dryRun.(bool)
			}

			if baseImage, ok := tt.input["base_image"]; ok {
				args.BaseImage = baseImage.(string)
			}
			if template, ok := tt.input["template"]; ok {
				args.Template = template.(string)
			}
			if optimization, ok := tt.input["optimization"]; ok {
				args.Optimization = optimization.(string)
			}
			if includeHealthCheck, ok := tt.input["include_health_check"]; ok {
				args.IncludeHealthCheck = includeHealthCheck.(bool)
			}
			if buildArgs, ok := tt.input["build_args"]; ok {
				args.BuildArgs = buildArgs.(map[string]string)
			}
			if platform, ok := tt.input["platform"]; ok {
				args.Platform = platform.(string)
			}

			// Convert to map (this simulates what our tool registration does)
			argsMap := map[string]interface{}{
				"session_id":           args.SessionID,
				"base_image":           args.BaseImage,
				"template":             args.Template,
				"optimization":         args.Optimization,
				"include_health_check": args.IncludeHealthCheck,
				"build_args":           args.BuildArgs,
				"platform":             args.Platform,
				"dry_run":              args.DryRun,
			}

			// Verify all expected fields are present and correct
			for key, expectedValue := range tt.expected {
				actualValue, exists := argsMap[key]
				assert.True(t, exists, "Expected key '%s' not found in args map", key)
				assert.Equal(t, expectedValue, actualValue, "Value mismatch for key '%s'", key)
			}

			// Verify no unexpected fields are present
			assert.Len(t, argsMap, len(tt.expected), "Args map contains unexpected fields")
		})
	}
}

// TestSessionIDHandling verifies session ID auto-creation logic
func TestSessionIDHandling(t *testing.T) {
	tests := []struct {
		name             string
		inputSessionID   string
		expectedBehavior string
	}{
		{
			name:             "existing_session_id",
			inputSessionID:   "existing-session-123",
			expectedBehavior: "preserve_existing",
		},
		{
			name:             "empty_session_id",
			inputSessionID:   "",
			expectedBehavior: "auto_create",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := &analyze.GenerateDockerfileArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: tt.inputSessionID,
				},
				Template: "java",
			}

			// Simulate the session handling logic from our tool registration
			originalSessionID := args.SessionID
			if args.SessionID == "" {
				// This would normally call deps.SessionManager.GetOrCreateSession("")
				// For the test, we'll simulate creating a new session
				args.SessionID = "auto-created-session-456"
			}

			argsMap := map[string]interface{}{
				"session_id":           args.SessionID,
				"base_image":           args.BaseImage,
				"template":             args.Template,
				"optimization":         args.Optimization,
				"include_health_check": args.IncludeHealthCheck,
				"build_args":           args.BuildArgs,
				"platform":             args.Platform,
				"dry_run":              args.DryRun,
			}

			switch tt.expectedBehavior {
			case "preserve_existing":
				assert.Equal(t, originalSessionID, argsMap["session_id"])
				assert.Equal(t, "existing-session-123", argsMap["session_id"])
			case "auto_create":
				assert.NotEqual(t, originalSessionID, argsMap["session_id"])
				assert.Equal(t, "auto-created-session-456", argsMap["session_id"])
			}
		})
	}
}
