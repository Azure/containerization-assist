package tools

import (
	// "context"
	"os"
	"path/filepath"
	"testing"

	// "log/slog"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TODO: Fix test when CallToolRequest fields are accessible
func TestApplyDockerfileHandler(t *testing.T) {
	t.Skip("Skipping until CallToolRequest fields are accessible")
	/*
		// Create temp directory for testing
		tempDir := t.TempDir()

		deps := ToolDependencies{
			Logger: slog.Default(),
		}

		handler := createApplyDockerfileHandler(deps)

		tests := []struct {
			name        string
			args        map[string]interface{}
			setupFiles  map[string]string
			wantSuccess bool
			wantError   string
			checkFile   string
			checkContent string
		}{
			{
				name: "successful create",
				args: map[string]interface{}{
					"session_id": "test-session",
					"repo_path":  tempDir,
					"content":    "FROM node:18\nWORKDIR /app\nCOPY . .\nCMD [\"npm\", \"start\"]",
					"path":       "Dockerfile",
				},
				wantSuccess: true,
				checkFile:   "Dockerfile",
				checkContent: "FROM node:18",
			},
			{
				name: "successful update with idempotency",
				args: map[string]interface{}{
					"session_id": "test-session",
					"repo_path":  tempDir,
					"content":    "FROM node:18-alpine\nWORKDIR /app",
					"path":       "Dockerfile.test",
				},
				setupFiles: map[string]string{
					"Dockerfile.test": "FROM node:16\nWORKDIR /app",
				},
				wantSuccess: true,
				checkFile:   "Dockerfile.test",
				checkContent: "FROM node:18-alpine",
			},
			{
				name: "dry run mode",
				args: map[string]interface{}{
					"session_id": "test-session",
					"repo_path":  tempDir,
					"content":    "FROM python:3.9",
					"path":       "Dockerfile.dry",
					"dry_run":    true,
				},
				wantSuccess: true,
				checkFile:   "Dockerfile.dry",
				checkContent: "", // File should not be created
			},
			{
				name: "path traversal prevention",
				args: map[string]interface{}{
					"session_id": "test-session",
					"repo_path":  tempDir,
					"content":    "FROM malicious",
					"path":       "../../../etc/passwd",
				},
				wantSuccess: false,
				wantError:   "invalid path",
			},
			{
				name: "missing required parameters",
				args: map[string]interface{}{
					"repo_path": tempDir,
					"content":   "FROM node:18",
				},
				wantSuccess: false,
				wantError:   "missing required parameters",
			},
			{
				name: "empty content",
				args: map[string]interface{}{
					"session_id": "test-session",
					"repo_path":  tempDir,
					"content":    "",
					"path":       "Dockerfile.empty",
				},
				wantSuccess: false,
				wantError:   "missing required parameters",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Setup files if needed
				for path, content := range tt.setupFiles {
					fullPath := filepath.Join(tempDir, path)
					err := os.WriteFile(fullPath, []byte(content), 0o644)
					require.NoError(t, err)
				}

				// Create request
				req := mcp.CallToolRequest{
					Name: "apply_dockerfile",
					Arguments: tt.args,
				}

				// Execute handler
				result, err := handler(context.Background(), req)

				// Check error
				if tt.wantError != "" {
					require.NotNil(t, result)
					require.Contains(t, getResultText(result), tt.wantError)
				} else {
					require.NoError(t, err)
					require.NotNil(t, result)
				}

				// Check file content if applicable
				if tt.checkFile != "" && !tt.args["dry_run"].(bool) {
					fullPath := filepath.Join(tempDir, tt.checkFile)
					if tt.checkContent != "" {
						content, err := os.ReadFile(fullPath)
						if tt.wantSuccess {
							require.NoError(t, err)
							assert.Contains(t, string(content), tt.checkContent)
						}
					} else {
						// File should not exist (dry run)
						_, err := os.Stat(fullPath)
						assert.True(t, os.IsNotExist(err))
					}
				}
			})
		}
	*/
}

func TestAtomicWrite(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")

	// Test atomic write
	content1 := []byte("content version 1")
	changed1, oldHash1, newHash1, err := WriteFileAtomic(testFile, content1, 0o644)

	require.NoError(t, err)
	assert.True(t, changed1)
	assert.Empty(t, oldHash1)
	assert.NotEmpty(t, newHash1)

	// Verify file content
	readContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, content1, readContent)

	// Test idempotency - writing same content
	changed2, oldHash2, newHash2, err := WriteFileAtomic(testFile, content1, 0o644)

	require.NoError(t, err)
	assert.False(t, changed2) // Should not change
	assert.Equal(t, newHash1, oldHash2)
	assert.Equal(t, newHash1, newHash2)

	// Test update with different content
	content3 := []byte("content version 3")
	changed3, oldHash3, newHash3, err := WriteFileAtomic(testFile, content3, 0o644)

	require.NoError(t, err)
	assert.True(t, changed3)
	assert.Equal(t, newHash1, oldHash3)
	assert.NotEqual(t, newHash1, newHash3)

	// Verify updated content
	readContent3, err := os.ReadFile(testFile)
	require.NoError(t, err)
	assert.Equal(t, content3, readContent3)
}

func TestPathEscapePrevention(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name      string
		repoRoot  string
		relPath   string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid relative path",
			repoRoot:  tempDir,
			relPath:   "subdir/file.txt",
			wantError: false,
		},
		{
			name:      "parent directory traversal",
			repoRoot:  tempDir,
			relPath:   "../../../etc/passwd",
			wantError: true,
			errorMsg:  "escapes workspace",
		},
		{
			name:      "absolute path attempt",
			repoRoot:  tempDir,
			relPath:   "/etc/passwd",
			wantError: true,
			errorMsg:  "escapes workspace",
		},
		{
			name:      "hidden parent traversal",
			repoRoot:  tempDir,
			relPath:   "subdir/../../etc/passwd",
			wantError: true,
			errorMsg:  "escapes workspace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := EnsureWithinWorkspace(tt.repoRoot, tt.relPath)

			if tt.wantError {
				assert.Error(t, err)
				if err != nil {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, result)
				// Result should be within the workspace
				assert.Contains(t, result, tempDir)
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantError bool
	}{
		{
			name:      "valid path",
			path:      "k8s/deployment.yaml",
			wantError: false,
		},
		{
			name:      "path with parent traversal",
			path:      "../etc/passwd",
			wantError: true,
		},
		{
			name:      "path with home directory",
			path:      "~/secret",
			wantError: true,
		},
		{
			name:      "path with environment variable",
			path:      "$HOME/secret",
			wantError: true,
		},
		{
			name:      "path with command injection",
			path:      "file; rm -rf /",
			wantError: true,
		},
		{
			name:      "absolute path",
			path:      "/etc/passwd",
			wantError: true,
		},
		{
			name:      "path with pipe",
			path:      "file | cat",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExtractors(t *testing.T) {
	t.Run("ExtractBaseImage", func(t *testing.T) {
		tests := []struct {
			content  string
			expected string
		}{
			{
				content:  "FROM node:18-alpine\nWORKDIR /app",
				expected: "node:18-alpine",
			},
			{
				content:  "FROM golang:1.20 AS builder\nFROM alpine:latest",
				expected: "golang:1.20",
			},
			{
				content:  "# Comment\nFROM python:3.9",
				expected: "python:3.9",
			},
			{
				content:  "WORKDIR /app\nCOPY . .",
				expected: "",
			},
		}

		for _, tt := range tests {
			result := ExtractBaseImage(tt.content)
			assert.Equal(t, tt.expected, result)
		}
	})

	t.Run("ExtractExposedPort", func(t *testing.T) {
		tests := []struct {
			content  string
			expected int
		}{
			{
				content:  "FROM node:18\nEXPOSE 3000",
				expected: 3000,
			},
			{
				content:  "FROM python:3.9\nEXPOSE 8080\nEXPOSE 9090",
				expected: 8080, // First port
			},
			{
				content:  "FROM alpine\nCMD [\"sh\"]",
				expected: 0,
			},
		}

		for _, tt := range tests {
			result := ExtractExposedPort(tt.content)
			assert.Equal(t, tt.expected, result)
		}
	})
}

func TestDiffSummary(t *testing.T) {
	content := []byte("test content")

	t.Run("created file", func(t *testing.T) {
		summary := CreateDiffSummary("/path/to/file", true, "", "newhash", content)
		assert.Equal(t, "created", summary.Action)
		assert.Empty(t, summary.OldHash)
		assert.Equal(t, "newhash", summary.NewHash)
		assert.Equal(t, len(content), summary.SizeBytes)
	})

	t.Run("modified file", func(t *testing.T) {
		summary := CreateDiffSummary("/path/to/file", true, "oldhash", "newhash", content)
		assert.Equal(t, "modified", summary.Action)
		assert.Equal(t, "oldhash", summary.OldHash)
		assert.Equal(t, "newhash", summary.NewHash)
	})

	t.Run("unchanged file", func(t *testing.T) {
		summary := CreateDiffSummary("/path/to/file", false, "samehash", "samehash", content)
		assert.Equal(t, "unchanged", summary.Action)
		assert.Equal(t, "samehash", summary.OldHash)
		assert.Equal(t, "samehash", summary.NewHash)
	})
}

// Helper function to extract text from tool result
func getResultText(result *mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}

	if textContent, ok := result.Content[0].(mcp.TextContent); ok {
		return textContent.Text
	}

	return ""
}
