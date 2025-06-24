package tools

import (
	"context"
	"fmt"
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

// testPushPipelineAdapter extends testPipelineAdapter for push-specific tests
type testPushPipelineAdapter struct {
	*testPipelineAdapter
	pushResult     *coredocker.RegistryPushResult
	pushError      error
	shouldFailPush bool
	pushCallCount  int
	pushCallArgs   []string
}

func (t *testPushPipelineAdapter) PushDockerImage(sessionID, imageName, registryURL string) (*coredocker.RegistryPushResult, error) {
	t.pushCallCount++
	t.pushCallArgs = []string{sessionID, imageName, registryURL}

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
		Output:   "Push complete: sha256:abc123def456",
	}, nil
}

// Context management methods for testing
func (t *testPushPipelineAdapter) SetContext(sessionID string, ctx context.Context) {
	// No-op for tests
}

func (t *testPushPipelineAdapter) GetContext(sessionID string) context.Context {
	return context.Background()
}

func (t *testPushPipelineAdapter) ClearContext(sessionID string) {
	// No-op for tests
}

func setupPushTest(t *testing.T, sessionID string, setupFunc func(*testPushPipelineAdapter, *testSessionManager)) (*AtomicPushImageTool, *testPushPipelineAdapter, *testSessionManager) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	adapter := &testPushPipelineAdapter{
		testPipelineAdapter: &testPipelineAdapter{
			workspaceDir: t.TempDir(),
		},
	}
	sessionMgr := newTestSessionManager()

	// Create default session if sessionID provided
	if sessionID != "" {
		session := &sessiontypes.SessionState{
			SessionID: sessionID,
			Metadata: map[string]interface{}{
				"last_built_image": "myapp:latest",
				"build_success":    true,
				"image_built":      true,
				"image_name":       "myapp:latest",
			},
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

	tool := NewAtomicPushImageTool(adapter, sessionMgr, logger)
	return tool, adapter, sessionMgr
}

func TestAtomicPushImageTool_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		args           AtomicPushImageArgs
		setupTest      func(*testPushPipelineAdapter, *testSessionManager)
		expectedError  bool
		validateResult func(*testing.T, *AtomicPushImageResult, *testPushPipelineAdapter)
	}{
		{
			name: "successful push to registry",
			args: AtomicPushImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "test-session",
					DryRun:    false,
				},
				ImageRef:    "myregistry.azurecr.io/myapp:v1.0.0",
				RegistryURL: "myregistry.azurecr.io",
			},
			setupTest: func(adapter *testPushPipelineAdapter, sessionMgr *testSessionManager) {
				// Default setup is sufficient
			},
			expectedError: false,
			validateResult: func(t *testing.T, result *AtomicPushImageResult, adapter *testPushPipelineAdapter) {
				assert.True(t, result.Success)
				assert.Equal(t, "test-session", result.SessionID)
				assert.Equal(t, "myregistry.azurecr.io/myapp:v1.0.0", result.ImageRef)
				assert.Equal(t, "myregistry.azurecr.io", result.RegistryURL)
				assert.NotNil(t, result.PushResult)
				assert.True(t, result.PushResult.Success)
				assert.NotZero(t, result.PushDuration)

				// Verify adapter was called correctly
				assert.Equal(t, 1, adapter.pushCallCount)
				assert.Equal(t, "myregistry.azurecr.io/myapp:v1.0.0", adapter.pushCallArgs[1])
				assert.Equal(t, "myregistry.azurecr.io", adapter.pushCallArgs[2])
			},
		},
		{
			name: "successful push with inferred registry URL",
			args: AtomicPushImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "infer-registry-session",
					DryRun:    false,
				},
				ImageRef: "docker.io/myuser/myapp:latest",
				// RegistryURL not provided - should be extracted from ImageRef
			},
			setupTest: func(adapter *testPushPipelineAdapter, sessionMgr *testSessionManager) {
				// Default setup
			},
			expectedError: false,
			validateResult: func(t *testing.T, result *AtomicPushImageResult, adapter *testPushPipelineAdapter) {
				assert.True(t, result.Success)
				assert.Equal(t, "docker.io/myuser/myapp:latest", result.ImageRef)
				assert.Equal(t, "docker.io", result.RegistryURL)
				assert.NotNil(t, result.PushContext)
				assert.Equal(t, "docker.io", result.PushContext.RegistryEndpoint)
			},
		},
		{
			name: "dry run mode",
			args: AtomicPushImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "dry-run-session",
					DryRun:    true,
				},
				ImageRef: "myregistry.azurecr.io/test:dry-run",
			},
			setupTest: func(adapter *testPushPipelineAdapter, sessionMgr *testSessionManager) {
				// No setup needed for dry run
			},
			expectedError: false,
			validateResult: func(t *testing.T, result *AtomicPushImageResult, adapter *testPushPipelineAdapter) {
				assert.True(t, result.Success)
				assert.Equal(t, "dry-run-session", result.SessionID)
				assert.True(t, result.DryRun)
				assert.Equal(t, "dry-run", result.PushContext.PushStatus)
				assert.Contains(t, result.PushContext.NextStepSuggestions[0], "dry-run")
				assert.Equal(t, 0, adapter.pushCallCount) // No actual push
			},
		},
		{
			name: "authentication failure",
			args: AtomicPushImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "auth-fail-session",
					DryRun:    false,
				},
				ImageRef:    "private.registry.io/app:v1",
				RegistryURL: "private.registry.io",
			},
			setupTest: func(adapter *testPushPipelineAdapter, sessionMgr *testSessionManager) {
				adapter.shouldFailPush = true
				adapter.pushError = fmt.Errorf("authentication required")
				adapter.pushResult = &coredocker.RegistryPushResult{
					Success: false,
					Error: &coredocker.RegistryError{
						Type:     "auth_error",
						Message:  "authentication required",
						ImageRef: "private.registry.io/app:v1",
						Registry: "private.registry.io",
						Output:   "unauthorized: authentication required",
						Context: map[string]interface{}{
							"auth_guidance": []string{
								"Run: docker login private.registry.io",
								"Or use cloud provider CLI for authentication",
							},
						},
					},
				}
			},
			expectedError: false,
			validateResult: func(t *testing.T, result *AtomicPushImageResult, adapter *testPushPipelineAdapter) {
				assert.False(t, result.Success)
				assert.NotNil(t, result.PushResult)
				assert.NotNil(t, result.PushResult.Error)
				assert.Contains(t, result.PushResult.Error.Message, "authentication")
				assert.Equal(t, "authentication", result.PushContext.ErrorCategory)
				assert.NotEmpty(t, result.PushContext.AuthenticationGuide)
				assert.Contains(t, result.PushContext.AuthenticationGuide[0], "docker login")
			},
		},
		{
			name: "network failure with retry suggestion",
			args: AtomicPushImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "network-fail-session",
					DryRun:    false,
				},
				ImageRef: "myregistry.io/app:latest",
			},
			setupTest: func(adapter *testPushPipelineAdapter, sessionMgr *testSessionManager) {
				adapter.shouldFailPush = true
				adapter.pushError = fmt.Errorf("network timeout")
				adapter.pushResult = &coredocker.RegistryPushResult{
					Success: false,
					Error: &coredocker.RegistryError{
						Type:     "network_error",
						Message:  "network timeout while pushing layers",
						ImageRef: "myregistry.io/app:latest",
						Registry: "myregistry.io",
						Output:   "Get https://myregistry.io/v2/: net/http: request canceled",
					},
				}
			},
			expectedError: false,
			validateResult: func(t *testing.T, result *AtomicPushImageResult, adapter *testPushPipelineAdapter) {
				assert.False(t, result.Success)
				assert.NotNil(t, result.PushResult)
				assert.NotNil(t, result.PushResult.Error)
				assert.Contains(t, result.PushResult.Error.Message, "network")
				assert.Equal(t, "connectivity", result.PushContext.ErrorCategory)
				assert.True(t, result.PushContext.IsRetryable)
				assert.NotEmpty(t, result.PushContext.TroubleshootingTips)
			},
		},
		{
			name: "session not found",
			args: AtomicPushImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "nonexistent-session",
					DryRun:    false,
				},
				ImageRef: "test:latest",
			},
			setupTest: func(adapter *testPushPipelineAdapter, sessionMgr *testSessionManager) {
				// Remove the default session that was created
				delete(sessionMgr.sessions, "nonexistent-session")
			},
			expectedError: true,
			validateResult: func(t *testing.T, result *AtomicPushImageResult, adapter *testPushPipelineAdapter) {
				t.Logf("Result: success=%v", result.Success)
				if result.PushResult != nil && result.PushResult.Error != nil {
					t.Logf("Error message: %s", result.PushResult.Error.Message)
				}
				assert.False(t, result.Success)
			},
		},
		{
			name: "push failure due to network error",
			args: AtomicPushImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "network-fail-session",
					DryRun:    false,
				},
				ImageRef: "registry.io/app:v1.0.0",
			},
			setupTest: func(adapter *testPushPipelineAdapter, sessionMgr *testSessionManager) {
				adapter.pushError = fmt.Errorf("network timeout")
				adapter.shouldFailPush = true
			},
			expectedError: false,
			validateResult: func(t *testing.T, result *AtomicPushImageResult, adapter *testPushPipelineAdapter) {
				assert.False(t, result.Success)
				if result.PushResult != nil && result.PushResult.Error != nil {
					assert.Contains(t, result.PushResult.Error.Message, "network timeout")
				}
				assert.Greater(t, len(result.PushContext.TroubleshootingTips), 0)
			},
		},
		{
			name: "push with detailed progress tracking",
			args: AtomicPushImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "progress-session",
					DryRun:    false,
				},
				ImageRef: "registry.io/app:v2.0.0",
				Timeout:  300,
			},
			setupTest: func(adapter *testPushPipelineAdapter, sessionMgr *testSessionManager) {
				adapter.pushResult = &coredocker.RegistryPushResult{
					Success:  true,
					Registry: "registry.io",
					ImageRef: "registry.io/app:v2.0.0",
					Duration: 5 * time.Second,
					Output:   "The push refers to repository [registry.io/app]\n5 layers pushed, 3 layers cached",
					Context: map[string]interface{}{
						"layers_pushed": 5,
						"layers_cached": 3,
						"size_mb":       45.2,
					},
				}
			},
			expectedError: false,
			validateResult: func(t *testing.T, result *AtomicPushImageResult, adapter *testPushPipelineAdapter) {
				assert.True(t, result.Success)
				assert.NotNil(t, result.PushContext)
				// The tool should parse push output
				assert.Contains(t, result.PushContext.PushStatus, "success")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, adapter, _ := setupPushTest(t, tt.args.SessionID, tt.setupTest)

			// Execute
			resultInterface, err := tool.Execute(ctx, tt.args)

			// Verify
			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if tt.validateResult != nil {
				result, ok := resultInterface.(*AtomicPushImageResult)
				require.True(t, ok, "Result should be *AtomicPushImageResult")
				tt.validateResult(t, result, adapter)
			}
		})
	}
}

func TestAtomicPushImageTool_RegistryDetection(t *testing.T) {
	logger := zerolog.New(nil).Level(zerolog.Disabled)
	adapter := &testPipelineAdapter{}
	sessionMgr := newTestSessionManager()
	tool := NewAtomicPushImageTool(adapter, sessionMgr, logger)

	tests := []struct {
		name             string
		imageRef         string
		providedRegistry string
		expectedRegistry string
		expectedType     string
	}{
		{
			name:             "docker hub with explicit docker.io",
			imageRef:         "docker.io/library/nginx:latest",
			providedRegistry: "",
			expectedRegistry: "docker.io",
			expectedType:     "Docker Hub",
		},
		{
			name:             "docker hub implicit",
			imageRef:         "nginx:latest",
			providedRegistry: "",
			expectedRegistry: "docker.io",
			expectedType:     "Docker Hub",
		},
		{
			name:             "azure container registry",
			imageRef:         "myregistry.azurecr.io/app:v1",
			providedRegistry: "",
			expectedRegistry: "myregistry.azurecr.io",
			expectedType:     "Azure Container Registry",
		},
		{
			name:             "aws ecr",
			imageRef:         "123456789.dkr.ecr.us-east-1.amazonaws.com/app:latest",
			providedRegistry: "",
			expectedRegistry: "123456789.dkr.ecr.us-east-1.amazonaws.com",
			expectedType:     "Amazon ECR",
		},
		{
			name:             "google container registry",
			imageRef:         "gcr.io/project/app:latest",
			providedRegistry: "",
			expectedRegistry: "gcr.io",
			expectedType:     "Google Container Registry",
		},
		{
			name:             "override registry URL",
			imageRef:         "myapp:latest",
			providedRegistry: "custom.registry.io",
			expectedRegistry: "custom.registry.io",
			expectedType:     "Private Registry",
		},
		{
			name:             "localhost registry",
			imageRef:         "localhost:5000/app:latest",
			providedRegistry: "",
			expectedRegistry: "localhost:5000",
			expectedType:     "Local Registry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := AtomicPushImageArgs{
				ImageRef:    tt.imageRef,
				RegistryURL: tt.providedRegistry,
			}

			registry := tool.extractRegistryURL(args)
			assert.Equal(t, tt.expectedRegistry, registry)

			registryType := tool.detectRegistryType(registry)
			assert.Equal(t, tt.expectedType, registryType)
		})
	}
}

func TestAtomicPushImageTool_ErrorCategorization(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name              string
		errorMessage      string
		errorType         string
		expectedCategory  string
		expectedRetryable bool
		expectedGuides    []string
	}{
		{
			name:              "authentication error",
			errorMessage:      "unauthorized: authentication required",
			errorType:         "auth_error",
			expectedCategory:  "authentication",
			expectedRetryable: false,
			expectedGuides: []string{
				"docker login",
				"Check credentials",
			},
		},
		{
			name:              "network timeout",
			errorMessage:      "net/http: request canceled (Client.Timeout exceeded)",
			errorType:         "network_error",
			expectedCategory:  "connectivity",
			expectedRetryable: true,
			expectedGuides: []string{
				"Check network connectivity",
				"Retry with increased timeout",
			},
		},
		{
			name:              "registry not found",
			errorMessage:      "no such host",
			errorType:         "network_error",
			expectedCategory:  "connectivity",
			expectedRetryable: false,
			expectedGuides: []string{
				"Verify registry URL",
				"Check DNS resolution",
			},
		},
		{
			name:              "permission denied",
			errorMessage:      "denied: requested access to the resource is denied",
			errorType:         "auth_error",
			expectedCategory:  "authentication",
			expectedRetryable: false,
			expectedGuides: []string{
				"Verify you have push permissions",
				"Check registry access policies",
			},
		},
		{
			name:              "quota exceeded",
			errorMessage:      "toomanyrequests: rate limit exceeded",
			errorType:         "rate_limit",
			expectedCategory:  "rate_limit",
			expectedRetryable: true,
			expectedGuides: []string{
				"Wait before retrying",
				"Consider upgrading plan",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, _, _ := setupPushTest(t, "error-test-session", func(adapter *testPushPipelineAdapter, sessionMgr *testSessionManager) {
				adapter.shouldFailPush = true
				adapter.pushError = fmt.Errorf("%s", tt.errorMessage)
				adapter.pushResult = &coredocker.RegistryPushResult{
					Success: false,
					Error: &coredocker.RegistryError{
						Type:     tt.errorType,
						Message:  tt.errorMessage,
						ImageRef: "test:latest",
						Registry: "test.registry.io",
					},
				}
			})

			args := AtomicPushImageArgs{
				BaseToolArgs: types.BaseToolArgs{
					SessionID: "error-test-session",
					DryRun:    false,
				},
				ImageRef: "test:latest",
			}

			resultInterface, err := tool.Execute(ctx, args)
			require.NoError(t, err)
			result, ok := resultInterface.(*AtomicPushImageResult)
			require.True(t, ok, "Result should be *AtomicPushImageResult")
			assert.False(t, result.Success)

			// Check error categorization
			assert.Equal(t, tt.expectedCategory, result.PushContext.ErrorCategory)
			assert.Equal(t, tt.expectedRetryable, result.PushContext.IsRetryable)

			// Check that appropriate guides are present
			allGuides := append(result.PushContext.AuthenticationGuide, result.PushContext.TroubleshootingTips...)
			for _, expectedGuide := range tt.expectedGuides {
				found := false
				for _, guide := range allGuides {
					if strings.Contains(guide, expectedGuide) {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected guide containing '%s' not found", expectedGuide)
			}
		})
	}
}

func TestAtomicPushImageTool_SessionStateUpdate(t *testing.T) {
	ctx := context.Background()

	tool, _, sessionMgr := setupPushTest(t, "state-test-session", func(adapter *testPushPipelineAdapter, sessionMgr *testSessionManager) {
		// Setup successful push result
		adapter.pushResult = &coredocker.RegistryPushResult{
			Success:  true,
			Registry: "test.registry.io",
			ImageRef: "test.registry.io/app:v1.0.0",
			Duration: 4 * time.Second,
			Output:   "Push complete",
		}
	})

	args := AtomicPushImageArgs{
		BaseToolArgs: types.BaseToolArgs{
			SessionID: "state-test-session",
			DryRun:    false,
		},
		ImageRef:    "test.registry.io/app:v1.0.0",
		RegistryURL: "test.registry.io",
	}

	resultInterface, err := tool.Execute(ctx, args)
	require.NoError(t, err)
	result, ok := resultInterface.(*AtomicPushImageResult)
	require.True(t, ok, "Result should be *AtomicPushImageResult")
	assert.True(t, result.Success)

	// Verify session state was updated
	session, err := sessionMgr.GetSession("state-test-session")
	require.NoError(t, err)

	// Verify image push state is correctly tracked
	assert.True(t, session.Dockerfile.Pushed) // Check modern field directly
	assert.True(t, session.Dockerfile.Pushed) // Verify modern field is set
	// Check the image ref is stored in metadata instead
	assert.Equal(t, "test.registry.io/app:v1.0.0", session.Metadata["pushed_image_ref"])
	assert.Equal(t, "test.registry.io", session.Metadata["registry_url"])
	assert.Equal(t, true, session.Metadata["push_success"])
	assert.NotNil(t, session.Metadata["push_duration"])
}
