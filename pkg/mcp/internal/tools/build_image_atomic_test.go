package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	coredocker "github.com/Azure/container-copilot/pkg/core/docker"
	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	sessiontypes "github.com/Azure/container-copilot/pkg/mcp/internal/types/session"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper functions
func mustMkdirAll(t testing.TB, path string, perm os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(path, perm); err != nil {
		t.Fatalf("Failed to create directory %s: %v", path, err)
	}
}

func mustWriteFile(t testing.TB, name string, data []byte, perm os.FileMode) {
	t.Helper()
	if err := os.WriteFile(name, data, perm); err != nil {
		t.Fatalf("Failed to write file %s: %v", name, err)
	}
}

// testBuildPipelineAdapter extends testPipelineAdapter for build-specific tests
type testBuildPipelineAdapter struct {
	*testPipelineAdapter
	buildResult     *coredocker.BuildResult
	buildError      error
	pushResult      *coredocker.RegistryPushResult
	pushError       error
	shouldFailBuild bool
	shouldFailPush  bool
	buildCallCount  int
	buildCallArgs   []string
}

func (t *testBuildPipelineAdapter) BuildDockerImage(sessionID, imageName, dockerfilePath string) (*coredocker.BuildResult, error) {
	t.buildCallCount++
	t.buildCallArgs = []string{sessionID, imageName, dockerfilePath}

	if t.shouldFailBuild {
		return t.buildResult, t.buildError
	}
	if t.buildResult != nil {
		return t.buildResult, nil
	}
	// Default result
	return &coredocker.BuildResult{
		Success:  true,
		ImageID:  "sha256:abc123def456",
		ImageRef: imageName,
		Duration: 5 * time.Second,
		Logs:     []string{"Step 1/5 : FROM golang:1.21-alpine", "... Build complete"},
	}, nil
}

func (t *testBuildPipelineAdapter) PushDockerImage(sessionID, imageName, registryURL string) (*coredocker.RegistryPushResult, error) {
	if t.shouldFailPush {
		return t.pushResult, t.pushError
	}
	if t.pushResult != nil {
		return t.pushResult, nil
	}
	// Default result
	return &coredocker.RegistryPushResult{
		Success:  true,
		Registry: registryURL,
		ImageRef: imageName,
		Duration: 3 * time.Second,
		Output:   "Push complete",
	}, nil
}

// Context management methods for testing
func (t *testBuildPipelineAdapter) SetContext(sessionID string, ctx context.Context) {
	// No-op for tests
}

func (t *testBuildPipelineAdapter) GetContext(sessionID string) context.Context {
	return context.Background()
}

func (t *testBuildPipelineAdapter) ClearContext(sessionID string) {
	// No-op for tests
}

func setupBuildTest(t *testing.T, sessionID string, setupFunc func(*testBuildPipelineAdapter, *testSessionManager)) (*AtomicBuildImageTool, *testBuildPipelineAdapter, *testSessionManager) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	adapter := &testBuildPipelineAdapter{
		testPipelineAdapter: &testPipelineAdapter{
			workspaceDir: t.TempDir(),
		},
	}
	sessionMgr := newTestSessionManager()

	// Create default session if sessionID provided
	if sessionID != "" {
		session := &sessiontypes.SessionState{
			SessionID: sessionID,
			ScanSummary: &types.RepositoryScanSummary{
				Language:      "Go",
				Framework:     "gin",
				Port:          8080,
				FilesAnalyzed: 10,
			},
			Metadata:     make(map[string]interface{}),
			CreatedAt:    time.Now(),
			LastAccessed: time.Now(),
			ExpiresAt:    time.Now().Add(24 * time.Hour),
		}
		sessionMgr.sessions[sessionID] = session
	}

	// Run custom setup
	if setupFunc != nil {
		setupFunc(adapter, sessionMgr)
	}

	tool := NewAtomicBuildImageTool(adapter, sessionMgr, logger)
	return tool, adapter, sessionMgr
}

func TestAtomicBuildImageTool_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		args           AtomicBuildImageArgs
		setupTest      func(*testBuildPipelineAdapter, *testSessionManager)
		expectedError  bool
		validateResult func(*testing.T, *AtomicBuildImageResult, *testBuildPipelineAdapter)
	}{
		{
			name: "successful build with defaults",
			args: AtomicBuildImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "test-session",
					DryRun:    false,
				},
				ImageName: "my-app",
			},
			setupTest: func(adapter *testBuildPipelineAdapter, sessionMgr *testSessionManager) {
				// Create Dockerfile in workspace
				workspaceDir := adapter.GetSessionWorkspace("test-session")
				repoDir := filepath.Join(workspaceDir, "repo")
				mustMkdirAll(t, repoDir, 0o755)

				dockerfilePath := filepath.Join(repoDir, "Dockerfile")
				dockerfileContent := `FROM golang:1.21-alpine
WORKDIR /app
COPY . .
RUN go build -o main .
EXPOSE 8080
CMD ["./main"]`
				mustWriteFile(t, dockerfilePath, []byte(dockerfileContent), 0o644)
			},
			expectedError: false,
			validateResult: func(t *testing.T, result *AtomicBuildImageResult, adapter *testBuildPipelineAdapter) {
				assert.True(t, result.Success)
				assert.Equal(t, "test-session", result.SessionID)
				assert.Equal(t, "my-app", result.ImageName)
				assert.Equal(t, "latest", result.ImageTag)
				assert.Equal(t, "my-app:latest", result.FullImageRef)
				assert.NotEmpty(t, result.DockerfilePath)
				assert.NotEmpty(t, result.BuildContext)
				assert.NotNil(t, result.BuildResult)
				assert.True(t, result.BuildResult.Success)
				assert.NotZero(t, result.BuildDuration)
				assert.True(t, result.Success)

				// Verify build context info
				assert.True(t, result.BuildContext_Info.DockerfileExists)
				assert.Greater(t, result.BuildContext_Info.DockerfileLines, 0)
				assert.Equal(t, "golang:1.21-alpine", result.BuildContext_Info.BaseImage)
				assert.Contains(t, result.BuildContext_Info.ExposedPorts, "8080")
				assert.Equal(t, 1, result.BuildContext_Info.BuildStages)

				// Verify adapter was called correctly
				assert.Equal(t, 1, adapter.buildCallCount)
				assert.Equal(t, "my-app:latest", adapter.buildCallArgs[1])
			},
		},
		{
			name: "successful build with custom tag and push",
			args: AtomicBuildImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "push-session",
					DryRun:    false,
				},
				ImageName:      "my-app",
				ImageTag:       "v1.0.0",
				PushAfterBuild: true,
				RegistryURL:    "myregistry.azurecr.io",
			},
			setupTest: func(adapter *testBuildPipelineAdapter, sessionMgr *testSessionManager) {
				// Create Dockerfile
				workspaceDir := adapter.GetSessionWorkspace("push-session")
				repoDir := filepath.Join(workspaceDir, "repo")
				mustMkdirAll(t, repoDir, 0o755)

				dockerfilePath := filepath.Join(repoDir, "Dockerfile")
				dockerfileContent := `FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
EXPOSE 3000
CMD ["npm", "start"]`
				mustWriteFile(t, dockerfilePath, []byte(dockerfileContent), 0o644)
			},
			expectedError: false,
			validateResult: func(t *testing.T, result *AtomicBuildImageResult, adapter *testBuildPipelineAdapter) {
				assert.True(t, result.Success)
				assert.Equal(t, "v1.0.0", result.ImageTag)
				assert.Equal(t, "my-app:v1.0.0", result.FullImageRef)
				assert.NotNil(t, result.PushResult)
				assert.True(t, result.PushResult.Success)
				assert.Equal(t, "myregistry.azurecr.io", result.PushResult.Registry)
				assert.NotZero(t, result.PushDuration)
			},
		},
		{
			name: "dry run mode",
			args: AtomicBuildImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "dry-run-session",
					DryRun:    true,
				},
				ImageName: "test-app",
				ImageTag:  "dry-run",
			},
			setupTest: func(adapter *testBuildPipelineAdapter, sessionMgr *testSessionManager) {
				// No setup needed for dry run
			},
			expectedError: false,
			validateResult: func(t *testing.T, result *AtomicBuildImageResult, adapter *testBuildPipelineAdapter) {
				assert.Equal(t, "dry-run-session", result.SessionID)
				assert.True(t, result.DryRun)
				assert.Contains(t, result.BuildContext_Info.NextStepSuggestions[0], "dry-run")
				assert.Contains(t, result.BuildContext_Info.NextStepSuggestions[1], "test-app:dry-run")
				assert.Equal(t, 0, adapter.buildCallCount) // No actual build
			},
		},
		{
			name: "dockerfile not found",
			args: AtomicBuildImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "no-dockerfile-session",
					DryRun:    false,
				},
				ImageName: "missing-dockerfile",
			},
			setupTest: func(adapter *testBuildPipelineAdapter, sessionMgr *testSessionManager) {
				// Create workspace but no Dockerfile
				workspaceDir := adapter.GetSessionWorkspace("no-dockerfile-session")
				repoDir := filepath.Join(workspaceDir, "repo")
				mustMkdirAll(t, repoDir, 0o755)
			},
			expectedError: true,
			validateResult: func(t *testing.T, result *AtomicBuildImageResult, adapter *testBuildPipelineAdapter) {
				assert.False(t, result.Success)
				// Error details are now logged instead of stored in result
				assert.False(t, result.BuildContext_Info.DockerfileExists)
			},
		},
		{
			name: "build failure",
			args: AtomicBuildImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "build-fail-session",
					DryRun:    false,
				},
				ImageName: "failing-app",
			},
			setupTest: func(adapter *testBuildPipelineAdapter, sessionMgr *testSessionManager) {
				// Create Dockerfile
				workspaceDir := adapter.GetSessionWorkspace("build-fail-session")
				repoDir := filepath.Join(workspaceDir, "repo")
				mustMkdirAll(t, repoDir, 0o755)

				dockerfilePath := filepath.Join(repoDir, "Dockerfile")
				dockerfileContent := `FROM alpine:latest
RUN exit 1`
				mustWriteFile(t, dockerfilePath, []byte(dockerfileContent), 0o644)

				// Configure adapter to fail
				adapter.shouldFailBuild = true
				adapter.buildError = fmt.Errorf("build command failed: exit status 1")
				adapter.buildResult = &coredocker.BuildResult{
					Success: false,
					Error: &coredocker.BuildError{
						Type:      "build_error",
						Message:   "build command failed: exit status 1",
						ExitCode:  1,
						BuildLogs: "Step 1/2 : FROM alpine:latest\nStep 2/2 : RUN exit 1\n ---> Running in abc123\nThe command '/bin/sh -c exit 1' returned a non-zero code: 1",
						Context: map[string]interface{}{
							"stage": "RUN exit 1",
						},
					},
					Logs: []string{"Step 1/2 : FROM alpine:latest", "Step 2/2 : RUN exit 1", "The command '/bin/sh -c exit 1' returned a non-zero code: 1"},
				}
			},
			expectedError: true,
			validateResult: func(t *testing.T, result *AtomicBuildImageResult, adapter *testBuildPipelineAdapter) {
				assert.False(t, result.Success)
				// Error details are now logged instead of stored in result
				assert.NotEmpty(t, result.BuildContext_Info.TroubleshootingTips)
			},
		},
		{
			name: "session not found",
			args: AtomicBuildImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "nonexistent-session",
					DryRun:    false,
				},
				ImageName: "test-app",
			},
			setupTest: func(adapter *testBuildPipelineAdapter, sessionMgr *testSessionManager) {
				// Don't create session - this will cause tool to return early
			},
			expectedError: true,
			validateResult: func(t *testing.T, result *AtomicBuildImageResult, adapter *testBuildPipelineAdapter) {
				assert.False(t, result.Success)
				// The tool now fails at the Dockerfile check stage since we don't create any files
				// when session is not found. This is expected behavior.
				// Error details are now logged instead of stored in result
			},
		},
		{
			name: "multi-stage dockerfile analysis",
			args: AtomicBuildImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "multi-stage-session",
					DryRun:    false,
				},
				ImageName: "multi-stage-app",
			},
			setupTest: func(adapter *testBuildPipelineAdapter, sessionMgr *testSessionManager) {
				// Create multi-stage Dockerfile
				workspaceDir := adapter.GetSessionWorkspace("multi-stage-session")
				repoDir := filepath.Join(workspaceDir, "repo")
				mustMkdirAll(t, repoDir, 0o755)

				dockerfilePath := filepath.Join(repoDir, "Dockerfile")
				dockerfileContent := `# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o app .

# Runtime stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /build/app .
EXPOSE 8080
EXPOSE 8081
CMD ["./app"]`
				mustWriteFile(t, dockerfilePath, []byte(dockerfileContent), 0o644)

				// Add .dockerignore
				dockerignorePath := filepath.Join(repoDir, ".dockerignore")
				mustWriteFile(t, dockerignorePath, []byte("*.log\n.git\n"), 0o644)
			},
			expectedError: false,
			validateResult: func(t *testing.T, result *AtomicBuildImageResult, adapter *testBuildPipelineAdapter) {
				assert.True(t, result.Success)
				assert.Equal(t, 2, result.BuildContext_Info.BuildStages)
				assert.Equal(t, "golang:1.21-alpine", result.BuildContext_Info.BaseImage)
				assert.Contains(t, result.BuildContext_Info.ExposedPorts, "8080")
				assert.Contains(t, result.BuildContext_Info.ExposedPorts, "8081")
				assert.True(t, result.BuildContext_Info.HasDockerIgnore)
				assert.Contains(t, result.BuildContext_Info.BuildOptimizations[0], "Multi-stage build")
			},
		},
		{
			name: "large build context warning",
			args: AtomicBuildImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "large-context-session",
					DryRun:    false,
				},
				ImageName: "large-context-app",
			},
			setupTest: func(adapter *testBuildPipelineAdapter, sessionMgr *testSessionManager) {
				// Create Dockerfile and large files
				workspaceDir := adapter.GetSessionWorkspace("large-context-session")
				repoDir := filepath.Join(workspaceDir, "repo")
				mustMkdirAll(t, repoDir, 0o755)

				dockerfilePath := filepath.Join(repoDir, "Dockerfile")
				dockerfileContent := `FROM alpine:latest
COPY . /app
CMD ["echo", "done"]`
				mustWriteFile(t, dockerfilePath, []byte(dockerfileContent), 0o644)

				// Create a large file (60MB) - this would trigger large file warning
				largePath := filepath.Join(repoDir, "large.bin")
				largeData := make([]byte, 60*1024*1024)
				mustWriteFile(t, largePath, largeData, 0o644)
			},
			expectedError: false,
			validateResult: func(t *testing.T, result *AtomicBuildImageResult, adapter *testBuildPipelineAdapter) {
				assert.True(t, result.Success)
				assert.NotEmpty(t, result.BuildContext_Info.LargeFilesFound)
				assert.Contains(t, result.BuildContext_Info.LargeFilesFound[0], "large.bin")
				assert.Greater(t, result.BuildContext_Info.ContextSize, int64(60*1024*1024))
				// Should have optimization suggestion about large files
				found := false
				for _, opt := range result.BuildContext_Info.BuildOptimizations {
					if strings.Contains(opt, "Large files detected") {
						found = true
						break
					}
				}
				assert.True(t, found, "Should have large file optimization suggestion")
			},
		},
		{
			name: "push failure after successful build",
			args: AtomicBuildImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "push-fail-session",
					DryRun:    false,
				},
				ImageName:      "push-fail-app",
				PushAfterBuild: true,
				RegistryURL:    "failing.registry.io",
			},
			setupTest: func(adapter *testBuildPipelineAdapter, sessionMgr *testSessionManager) {
				// Create Dockerfile
				workspaceDir := adapter.GetSessionWorkspace("push-fail-session")
				repoDir := filepath.Join(workspaceDir, "repo")
				mustMkdirAll(t, repoDir, 0o755)

				dockerfilePath := filepath.Join(repoDir, "Dockerfile")
				dockerfileContent := `FROM alpine:latest`
				mustWriteFile(t, dockerfilePath, []byte(dockerfileContent), 0o644)

				// Build succeeds but push fails
				adapter.shouldFailPush = true
				adapter.pushError = fmt.Errorf("authentication failed")
				adapter.pushResult = &coredocker.RegistryPushResult{
					Success: false,
					Error: &coredocker.RegistryError{
						Type:     "auth_error",
						Message:  "authentication failed",
						ImageRef: "push-fail-app:latest",
						Registry: "failing.registry.io",
						Context: map[string]interface{}{
							"auth_guidance": []string{
								"Run: docker login failing.registry.io",
								"Or use: az acr login --name failing",
							},
						},
					},
				}
			},
			expectedError: false,
			validateResult: func(t *testing.T, result *AtomicBuildImageResult, adapter *testBuildPipelineAdapter) {
				assert.True(t, result.Success) // Build succeeded even though push failed
				assert.NotNil(t, result.BuildResult)
				assert.True(t, result.BuildResult.Success)
				assert.NotNil(t, result.PushResult)
				assert.False(t, result.PushResult.Success)
				// Should have auth guidance in troubleshooting tips
				// The auth_guidance is correctly passed to troubleshooting tips
				tips := strings.Join(result.BuildContext_Info.TroubleshootingTips, " ")
				t.Logf("Troubleshooting tips: %v", result.BuildContext_Info.TroubleshootingTips)
				assert.Contains(t, tips, "docker login")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, adapter, _ := setupBuildTest(t, tt.args.SessionID, tt.setupTest)

			// Execute
			resultInterface, err := tool.Execute(ctx, tt.args)

			// Verify
			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if tt.validateResult != nil && resultInterface != nil {
				result, ok := resultInterface.(*AtomicBuildImageResult)
				require.True(t, ok, "Result should be *AtomicBuildImageResult")
				tt.validateResult(t, result, adapter)
			}
		})
	}
}

func TestAtomicBuildImageTool_HelperMethods(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	adapter := &testPipelineAdapter{workspaceDir: "/workspace/test"}
	sessionMgr := newTestSessionManager()
	tool := NewAtomicBuildImageTool(adapter, sessionMgr, logger)

	t.Run("getImageTag", func(t *testing.T) {
		assert.Equal(t, "latest", tool.getImageTag(""))
		assert.Equal(t, "v1.0.0", tool.getImageTag("v1.0.0"))
		assert.Equal(t, "dev", tool.getImageTag("dev"))
	})

	t.Run("getPlatform", func(t *testing.T) {
		assert.Equal(t, "linux/amd64", tool.getPlatform(""))
		assert.Equal(t, "linux/arm64", tool.getPlatform("linux/arm64"))
		assert.Equal(t, "linux/amd64,linux/arm64", tool.getPlatform("linux/amd64,linux/arm64"))
	})

	t.Run("getBuildContext", func(t *testing.T) {
		workspaceDir := "/workspace/session123"

		// Default to repo directory
		assert.Equal(t, "/workspace/session123/repo", tool.getBuildContext("", workspaceDir))

		// Relative path
		assert.Equal(t, "/workspace/session123/src", tool.getBuildContext("src", workspaceDir))

		// Absolute path
		assert.Equal(t, "/custom/path", tool.getBuildContext("/custom/path", workspaceDir))
	})

	t.Run("getDockerfilePath", func(t *testing.T) {
		buildContext := "/workspace/repo"

		// Default Dockerfile
		assert.Equal(t, "/workspace/repo/Dockerfile", tool.getDockerfilePath("", buildContext))

		// Relative path
		assert.Equal(t, "/workspace/repo/docker/Dockerfile.prod", tool.getDockerfilePath("docker/Dockerfile.prod", buildContext))

		// Absolute path
		assert.Equal(t, "/custom/Dockerfile", tool.getDockerfilePath("/custom/Dockerfile", buildContext))
	})
}

func TestAtomicBuildImageTool_SecurityRecommendations(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name              string
		dockerfileContent string
		expectedRecs      []string
	}{
		{
			name: "latest tag warning",
			dockerfileContent: `FROM ubuntu:latest
RUN apt-get update`,
			expectedRecs: []string{"specific image tags"},
		},
		{
			name: "non-alpine base image",
			dockerfileContent: `FROM ubuntu:22.04
RUN apt-get update`,
			expectedRecs: []string{"alpine or distroless"},
		},
		{
			name: "secure alpine image",
			dockerfileContent: `FROM alpine:3.18
RUN apk add --no-cache curl`,
			expectedRecs: []string{}, // No security warnings
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, _, _ := setupBuildTest(t, "sec-test-session", func(adapter *testBuildPipelineAdapter, sessionMgr *testSessionManager) {
				// Create Dockerfile with test content
				workspaceDir := adapter.GetSessionWorkspace("sec-test-session")
				repoDir := filepath.Join(workspaceDir, "repo")
				mustMkdirAll(t, repoDir, 0o755)

				dockerfilePath := filepath.Join(repoDir, "Dockerfile")
				mustWriteFile(t, dockerfilePath, []byte(tt.dockerfileContent), 0o644)
			})

			args := AtomicBuildImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "sec-test-session",
					DryRun:    false,
				},
				ImageName: "security-test",
			}

			resultInterface, err := tool.Execute(ctx, args)
			require.NoError(t, err)
			result, ok := resultInterface.(*AtomicBuildImageResult)
			require.True(t, ok, "Result should be *AtomicBuildImageResult")

			// Check security recommendations
			for _, expectedRec := range tt.expectedRecs {
				found := false
				for _, rec := range result.BuildContext_Info.SecurityRecommendations {
					if strings.Contains(rec, expectedRec) {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected security recommendation containing '%s'", expectedRec)
			}
		})
	}
}

func TestAtomicBuildImageTool_SessionStateUpdate(t *testing.T) {
	ctx := context.Background()

	tool, _, sessionMgr := setupBuildTest(t, "state-test-session", func(adapter *testBuildPipelineAdapter, sessionMgr *testSessionManager) {
		// Create Dockerfile
		workspaceDir := adapter.GetSessionWorkspace("state-test-session")
		repoDir := filepath.Join(workspaceDir, "repo")
		mustMkdirAll(t, repoDir, 0o755)

		dockerfilePath := filepath.Join(repoDir, "Dockerfile")
		dockerfileContent := `FROM alpine:latest`
		mustWriteFile(t, dockerfilePath, []byte(dockerfileContent), 0o644)
	})

	args := AtomicBuildImageArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "state-test-session",
			DryRun:    false,
		},
		ImageName:      "state-test-app",
		ImageTag:       "v1.2.3",
		PushAfterBuild: true,
		RegistryURL:    "test.registry.io",
	}

	resultInterface, err := tool.Execute(ctx, args)
	require.NoError(t, err)
	result, ok := resultInterface.(*AtomicBuildImageResult)
	require.True(t, ok, "Result should be *AtomicBuildImageResult")
	assert.True(t, result.Success)

	// Verify session state was updated
	session, err := sessionMgr.GetSession("state-test-session")
	require.NoError(t, err)

	// Check current stage derivation from StageHistory
	expectedStage := "image_built"
	if len(session.StageHistory) == 0 {
		expectedStage = "initialized"
	} else {
		lastExecution := session.StageHistory[len(session.StageHistory)-1]
		if lastExecution.Success && lastExecution.EndTime != nil {
			expectedStage = sessiontypes.DeriveNextStage(lastExecution.Tool)
		} else {
			expectedStage = lastExecution.Tool
		}
	}
	assert.Equal(t, "image_built", expectedStage)
	assert.Equal(t, "state-test-app:v1.2.3", session.Metadata["last_built_image"])
	assert.Equal(t, true, session.Metadata["build_success"])
	assert.Equal(t, "sha256:abc123def456", session.Metadata["image_id"])
	assert.Equal(t, true, session.Metadata["push_success"])
	assert.Equal(t, "test.registry.io", session.Metadata["registry_url"])
	assert.NotNil(t, session.Metadata["build_duration"])
	assert.NotNil(t, session.Metadata["dockerfile_path"])
	assert.NotNil(t, session.Metadata["build_context"])
}
