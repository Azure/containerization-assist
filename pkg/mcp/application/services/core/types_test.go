package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestImageReference_String(t *testing.T) {
	tests := []struct {
		name     string
		ref      ImageReference
		expected string
	}{
		{
			name: "basic repository with tag",
			ref: ImageReference{
				Repository: "myapp",
				Tag:        "v1.0.0",
			},
			expected: "myapp:v1.0.0",
		},
		{
			name: "repository with registry and tag",
			ref: ImageReference{
				Registry:   "myregistry.azurecr.io",
				Repository: "myapp",
				Tag:        "latest",
			},
			expected: "myregistry.azurecr.io/myapp:latest",
		},
		{
			name: "repository with digest",
			ref: ImageReference{
				Repository: "myapp",
				Digest:     "sha256:abc123",
			},
			expected: "myapp@sha256:abc123",
		},
		{
			name: "full reference with registry, tag, and digest",
			ref: ImageReference{
				Registry:   "docker.io",
				Repository: "library/nginx",
				Tag:        "alpine",
				Digest:     "sha256:def456",
			},
			expected: "docker.io/library/nginx:alpine@sha256:def456",
		},
		{
			name: "repository only",
			ref: ImageReference{
				Repository: "myapp",
			},
			expected: "myapp",
		},
		{
			name: "empty repository",
			ref: ImageReference{
				Registry: "myregistry.com",
				Tag:      "latest",
			},
			expected: "myregistry.com/:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ref.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewBaseResponse(t *testing.T) {
	tool := "test_tool"
	sessionID := "test-session-123"
	dryRun := true

	beforeTime := time.Now()

	response := NewBaseResponse(tool, sessionID, dryRun)

	afterTime := time.Now()

	assert.Equal(t, CurrentSchemaVersion, response.Version)
	assert.Equal(t, tool, response.Tool)
	assert.Equal(t, sessionID, response.SessionID)
	assert.Equal(t, dryRun, response.DryRun)

	assert.True(t, response.Timestamp.After(beforeTime) || response.Timestamp.Equal(beforeTime))
	assert.True(t, response.Timestamp.Before(afterTime) || response.Timestamp.Equal(afterTime))
}

func TestNewBaseResponse_VariousInputs(t *testing.T) {
	tests := []struct {
		name      string
		tool      string
		sessionID string
		dryRun    bool
	}{
		{
			name:      "normal case",
			tool:      "analyze_repository",
			sessionID: "session-abc123",
			dryRun:    false,
		},
		{
			name:      "dry run case",
			tool:      "build_image",
			sessionID: "session-def456",
			dryRun:    true,
		},
		{
			name:      "empty session ID",
			tool:      "deploy_kubernetes",
			sessionID: "",
			dryRun:    false,
		},
		{
			name:      "empty tool name",
			tool:      "",
			sessionID: "session-ghi789",
			dryRun:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := NewBaseResponse(tt.tool, tt.sessionID, tt.dryRun)

			assert.Equal(t, CurrentSchemaVersion, response.Version)
			assert.Equal(t, tt.tool, response.Tool)
			assert.Equal(t, tt.sessionID, response.SessionID)
			assert.Equal(t, tt.dryRun, response.DryRun)
			assert.WithinDuration(t, time.Now(), response.Timestamp, time.Second)
		})
	}
}

func TestConstants(t *testing.T) {
	assert.NotEmpty(t, CurrentSchemaVersion)
	assert.NotEmpty(t, ToolAPIVersion)

	assert.Equal(t, "v1.0.0", CurrentSchemaVersion)
	assert.Equal(t, "2024.12.17", ToolAPIVersion)
}

func TestBaseToolResponse_Structure(t *testing.T) {
	response := BaseToolResponse{
		Version:   "v1.0.0",
		Tool:      "test_tool",
		Timestamp: time.Now(),
		SessionID: "test-session",
		DryRun:    true,
	}

	assert.Equal(t, "v1.0.0", response.Version)
	assert.Equal(t, "test_tool", response.Tool)
	assert.Equal(t, "test-session", response.SessionID)
	assert.True(t, response.DryRun)
	assert.NotZero(t, response.Timestamp)
}

func TestBaseToolArgs_Structure(t *testing.T) {
	args := BaseToolArgs{
		DryRun:    true,
		SessionID: "test-session",
	}

	assert.True(t, args.DryRun)
	assert.Equal(t, "test-session", args.SessionID)
}

func TestResourceRequests_Structure(t *testing.T) {
	resources := ResourceRequests{
		CPURequest:    "100m",
		MemoryRequest: "256Mi",
		CPULimit:      "500m",
		MemoryLimit:   "512Mi",
	}

	assert.Equal(t, "100m", resources.CPURequest)
	assert.Equal(t, "256Mi", resources.MemoryRequest)
	assert.Equal(t, "500m", resources.CPULimit)
	assert.Equal(t, "512Mi", resources.MemoryLimit)
}

func TestSecretRef_Structure(t *testing.T) {
	secret := SecretRef{
		Name: "my-secret",
		Key:  "password",
		Env:  "DB_PASSWORD",
	}

	assert.Equal(t, "my-secret", secret.Name)
	assert.Equal(t, "password", secret.Key)
	assert.Equal(t, "DB_PASSWORD", secret.Env)
}

func TestPortForward_Structure(t *testing.T) {
	portForward := PortForward{
		LocalPort:  8080,
		RemotePort: 80,
		Service:    "web-service",
		Pod:        "web-pod-123",
	}

	assert.Equal(t, 8080, portForward.LocalPort)
	assert.Equal(t, 80, portForward.RemotePort)
	assert.Equal(t, "web-service", portForward.Service)
	assert.Equal(t, "web-pod-123", portForward.Pod)
}

func TestResourceUtilization_Structure(t *testing.T) {
	utilization := ResourceUtilization{
		CPU:         45.6,
		Memory:      78.2,
		Disk:        23.1,
		DiskFree:    1024 * 1024 * 1024,
		LoadAverage: 1.5,
	}

	assert.Equal(t, 45.6, utilization.CPU)
	assert.Equal(t, 78.2, utilization.Memory)
	assert.Equal(t, 23.1, utilization.Disk)
	assert.Equal(t, int64(1024*1024*1024), utilization.DiskFree)
	assert.Equal(t, 1.5, utilization.LoadAverage)
}

func TestServiceHealth_Structure(t *testing.T) {
	now := time.Now()
	health := ServiceHealth{
		Status:       "healthy",
		LastCheck:    now,
		ResponseTime: 150 * time.Millisecond,
		Error:        "",
	}

	assert.Equal(t, "healthy", health.Status)
	assert.Equal(t, now, health.LastCheck)
	assert.Equal(t, 150*time.Millisecond, health.ResponseTime)
	assert.Empty(t, health.Error)
}

func TestServiceHealth_WithError(t *testing.T) {
	now := time.Now()
	health := ServiceHealth{
		Status:    "unhealthy",
		LastCheck: now,
		Error:     "connection timeout",
	}

	assert.Equal(t, "unhealthy", health.Status)
	assert.Equal(t, now, health.LastCheck)
	assert.Equal(t, "connection timeout", health.Error)
	assert.Zero(t, health.ResponseTime)
}
