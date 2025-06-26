package utils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceManager_CheckQuota(t *testing.T) {
	tests := []struct {
		name            string
		sessionID       string
		currentUsage    int64
		additionalBytes int64
		maxPerSession   int64
		totalMax        int64
		otherSessions   map[string]int64 // Other session disk usage
		expectError     bool
		errorCode       string
		errorContains   string
	}{
		{
			name:            "within all quotas",
			sessionID:       "session1",
			currentUsage:    1000,
			additionalBytes: 500,
			maxPerSession:   10000,
			totalMax:        50000,
			expectError:     false,
		},
		{
			name:            "exceeds session quota",
			sessionID:       "session1",
			currentUsage:    8000,
			additionalBytes: 3000,
			maxPerSession:   10000,
			totalMax:        50000,
			expectError:     true,
			errorCode:       "SESSION_QUOTA_EXCEEDED",
			errorContains:   "session disk quota would be exceeded: 8000 + 3000 > 10000",
		},
		{
			name:            "within session but exceeds global quota",
			sessionID:       "session1",
			currentUsage:    5000,
			additionalBytes: 4000,
			maxPerSession:   10000,
			totalMax:        20000,
			otherSessions: map[string]int64{
				"session2": 8000,
				"session3": 5000,
			},
			expectError:   true,
			errorCode:     "GLOBAL_QUOTA_EXCEEDED",
			errorContains: "global disk quota would be exceeded",
		},
		{
			name:            "exactly at session limit",
			sessionID:       "session1",
			currentUsage:    5000,
			additionalBytes: 5000,
			maxPerSession:   10000,
			totalMax:        50000,
			expectError:     false,
		},
		{
			name:            "exactly at global limit",
			sessionID:       "session1",
			currentUsage:    5000,
			additionalBytes: 5000,
			maxPerSession:   20000,
			totalMax:        15000,
			otherSessions: map[string]int64{
				"session2": 5000,
			},
			expectError: false,
		},
		{
			name:            "zero additional bytes",
			sessionID:       "session1",
			currentUsage:    10000,
			additionalBytes: 0,
			maxPerSession:   10000,
			totalMax:        50000,
			expectError:     false,
		},
		{
			name:            "negative additional bytes",
			sessionID:       "session1",
			currentUsage:    8000,
			additionalBytes: -1000,
			maxPerSession:   10000,
			totalMax:        50000,
			expectError:     false, // Should handle gracefully
		},
		{
			name:            "new session",
			sessionID:       "newsession",
			currentUsage:    0,
			additionalBytes: 5000,
			maxPerSession:   10000,
			totalMax:        50000,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create workspace manager
			tempDir := t.TempDir()
			logger := zerolog.Nop()

			wm := &WorkspaceManager{
				baseDir:           tempDir,
				logger:            logger,
				maxSizePerSession: tt.maxPerSession,
				totalMaxSize:      tt.totalMax,
				diskUsage:         make(map[string]int64),
			}

			// Set up current usage
			wm.diskUsage[tt.sessionID] = tt.currentUsage

			// Set up other sessions
			for sid, usage := range tt.otherSessions {
				wm.diskUsage[sid] = usage
			}

			// Run the test
			err := wm.CheckQuota(tt.sessionID, tt.additionalBytes)

			if tt.expectError {
				assert.Error(t, err)

				richErr, ok := err.(*types.RichError)
				if ok {
					assert.Equal(t, tt.errorCode, richErr.Code)
					if tt.errorContains != "" {
						assert.Contains(t, richErr.Message, tt.errorContains)
					}
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWorkspaceManager_ValidateLocalPath(t *testing.T) {
	// Create temp directory for testing
	tempDir := t.TempDir()

	// Create some test paths
	validFile := filepath.Join(tempDir, "valid.txt")
	require.NoError(t, os.WriteFile(validFile, []byte("test"), 0644))

	validDir := filepath.Join(tempDir, "validdir")
	require.NoError(t, os.Mkdir(validDir, 0755))

	tests := []struct {
		name          string
		path          string
		expectError   bool
		errorContains string
	}{
		{
			name:        "valid file path",
			path:        validFile,
			expectError: false,
		},
		{
			name:        "valid directory path",
			path:        validDir,
			expectError: false,
		},
		{
			name:          "empty path",
			path:          "",
			expectError:   true,
			errorContains: "path cannot be empty",
		},
		{
			name:          "path traversal attempt",
			path:          "../../../etc/passwd",
			expectError:   true,
			errorContains: "path traversal",
		},
		{
			name:          "absolute path outside workspace",
			path:          "/etc/passwd",
			expectError:   true,
			errorContains: "absolute paths not allowed",
		},
		{
			name:          "hidden file",
			path:          ".git/config",
			expectError:   true,
			errorContains: "hidden files",
		},
		{
			name:          "non-existent path",
			path:          filepath.Join(tempDir, "nonexistent.txt"),
			expectError:   true,
			errorContains: "does not exist",
		},
		{
			name:          "path with spaces",
			path:          "path with spaces/file.txt",
			expectError:   true,
			errorContains: "does not exist",
		},
		{
			name:        "relative path within workspace",
			path:        "validdir",
			expectError: false, // Assuming it resolves to validDir
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create workspace manager for testing
			wm := &WorkspaceManager{
				baseDir:   tempDir,
				logger:    zerolog.Nop(),
				diskUsage: make(map[string]int64),
			}

			err := wm.ValidateLocalPath(context.Background(), tt.path)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Helper function to simulate path validation
func validateLocalPath(path, baseDir string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	// Check for path traversal
	if filepath.IsAbs(path) && !strings.HasPrefix(path, baseDir) {
		return fmt.Errorf("absolute paths not allowed outside workspace")
	}

	if strings.Contains(path, "..") {
		return fmt.Errorf("path traversal detected")
	}

	// Check for hidden files
	if strings.HasPrefix(filepath.Base(path), ".") || strings.Contains(path, "/.") {
		return fmt.Errorf("hidden files not allowed")
	}

	// Resolve path
	fullPath := path
	if !filepath.IsAbs(path) {
		fullPath = filepath.Join(baseDir, path)
	}

	// Check existence
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", path)
	}

	return nil
}

func TestWorkspaceManager_UpdateDiskUsage(t *testing.T) {
	tests := []struct {
		name          string
		sessionID     string
		files         map[string]int64 // filename -> size
		expectError   bool
		expectedUsage int64
	}{
		{
			name:      "single file",
			sessionID: "session1",
			files: map[string]int64{
				"file1.txt": 1024,
			},
			expectError:   false,
			expectedUsage: 1024,
		},
		{
			name:      "multiple files",
			sessionID: "session2",
			files: map[string]int64{
				"file1.txt":         1024,
				"file2.txt":         2048,
				"dir/file3.txt":     512,
				"dir/subdir/f4.txt": 256,
			},
			expectError:   false,
			expectedUsage: 3840,
		},
		{
			name:          "empty directory",
			sessionID:     "session3",
			files:         map[string]int64{},
			expectError:   false,
			expectedUsage: 0,
		},
		{
			name:      "large files",
			sessionID: "session4",
			files: map[string]int64{
				"large1.bin": 1024 * 1024 * 100, // 100MB
				"large2.bin": 1024 * 1024 * 50,  // 50MB
			},
			expectError:   false,
			expectedUsage: 1024 * 1024 * 150,
		},
		{
			name:          "non-existent session",
			sessionID:     "nonexistent",
			files:         nil,
			expectError:   false, // Directory doesn't exist but Walk handles it
			expectedUsage: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create workspace manager
			tempDir := t.TempDir()
			logger := zerolog.Nop()

			wm := &WorkspaceManager{
				baseDir:   tempDir,
				logger:    logger,
				diskUsage: make(map[string]int64),
			}

			// Create session directory and files
			if tt.files != nil {
				sessionDir := filepath.Join(tempDir, tt.sessionID)
				require.NoError(t, os.MkdirAll(sessionDir, 0755))

				for filename, size := range tt.files {
					filePath := filepath.Join(sessionDir, filename)
					dir := filepath.Dir(filePath)
					if dir != sessionDir {
						require.NoError(t, os.MkdirAll(dir, 0755))
					}

					// Create file with specified size
					data := make([]byte, size)
					require.NoError(t, os.WriteFile(filePath, data, 0644))
				}
			}

			// Run the test
			err := wm.UpdateDiskUsage(context.Background(), tt.sessionID)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Check the recorded usage
				usage := wm.GetDiskUsage(tt.sessionID)
				assert.Equal(t, tt.expectedUsage, usage)
			}
		})
	}
}

func TestWorkspaceManager_EnforceGlobalQuota(t *testing.T) {
	tests := []struct {
		name         string
		totalMaxSize int64
		sessions     map[string]int64
		expectError  bool
		errorCode    string
	}{
		{
			name:         "within global quota",
			totalMaxSize: 10000,
			sessions: map[string]int64{
				"session1": 2000,
				"session2": 3000,
				"session3": 1000,
			},
			expectError: false,
		},
		{
			name:         "exactly at global quota",
			totalMaxSize: 10000,
			sessions: map[string]int64{
				"session1": 3000,
				"session2": 4000,
				"session3": 3000,
			},
			expectError: false,
		},
		{
			name:         "exceeds global quota",
			totalMaxSize: 10000,
			sessions: map[string]int64{
				"session1": 4000,
				"session2": 4000,
				"session3": 3000,
			},
			expectError: true,
			errorCode:   "GLOBAL_QUOTA_EXCEEDED",
		},
		{
			name:         "no sessions",
			totalMaxSize: 10000,
			sessions:     map[string]int64{},
			expectError:  false,
		},
		{
			name:         "single large session",
			totalMaxSize: 5000,
			sessions: map[string]int64{
				"session1": 6000,
			},
			expectError: true,
			errorCode:   "GLOBAL_QUOTA_EXCEEDED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wm := &WorkspaceManager{
				totalMaxSize: tt.totalMaxSize,
				diskUsage:    tt.sessions,
			}

			err := wm.EnforceGlobalQuota()

			if tt.expectError {
				assert.Error(t, err)

				richErr, ok := err.(*types.RichError)
				if ok {
					assert.Equal(t, tt.errorCode, richErr.Code)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
