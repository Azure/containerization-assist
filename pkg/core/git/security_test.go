package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilesystemJail_ValidatePath(t *testing.T) {
	// Create temporary workspace
	tempDir := t.TempDir()
	workspaceRoot := filepath.Join(tempDir, "workspace")
	if err := os.MkdirAll(workspaceRoot, 0755); err != nil {
		t.Fatal(err)
	}

	opts := &SecurityOptions{
		WorkspaceRoot: workspaceRoot,
		BlockedPaths:  DefaultSecurityOptions().BlockedPaths,
		AllowSymlinks: false,
	}

	jail, err := NewFilesystemJail(opts)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid path within workspace",
			path:    filepath.Join(workspaceRoot, "repo", "file.txt"),
			wantErr: false,
		},
		{
			name:    "path outside workspace",
			path:    "/etc/passwd",
			wantErr: true,
			errMsg:  "restricted location",
		},
		{
			name:    "path with traversal attempt",
			path:    filepath.Join(workspaceRoot, "..", "..", "etc", "passwd"),
			wantErr: true,
			errMsg:  "outside workspace root",
		},
		{
			name:    "blocked path pattern",
			path:    filepath.Join(workspaceRoot, "test", "...", "config"),
			wantErr: true,
			errMsg:  "blocked pattern",
		},
		{
			name:    "path with triple dots",
			path:    filepath.Join(workspaceRoot, "...", "file"),
			wantErr: true,
			errMsg:  "blocked pattern",
		},
		{
			name:    "absolute path outside workspace",
			path:    "/root/malicious",
			wantErr: true,
			errMsg:  "restricted location",
		},
		{
			name:    "relative path resolved to workspace",
			path:    filepath.Join(workspaceRoot, "repo/file.txt"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := jail.ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidatePath() error = %v, want error containing %v", err, tt.errMsg)
			}
		})
	}
}

func TestFilesystemJail_ValidateURL(t *testing.T) {
	jail, err := NewFilesystemJail(&SecurityOptions{
		WorkspaceRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "valid https URL",
			url:     "https://github.com/user/repo.git",
			wantErr: false,
		},
		{
			name:    "valid http URL",
			url:     "http://example.com/repo.git",
			wantErr: false,
		},
		{
			name:    "valid SSH URL",
			url:     "git@github.com:user/repo.git",
			wantErr: false,
		},
		{
			name:    "file URL blocked",
			url:     "file:///etc/passwd",
			wantErr: true,
		},
		{
			name:    "URL with path traversal",
			url:     "https://github.com/../../etc/passwd",
			wantErr: true,
		},
		{
			name:    "URL with command injection attempt",
			url:     "https://github.com/user/repo.git; rm -rf /",
			wantErr: true,
		},
		{
			name:    "URL with backticks",
			url:     "https://github.com/user/repo.git`whoami`",
			wantErr: true,
		},
		{
			name:    "URL with pipe",
			url:     "https://github.com/user/repo.git | cat /etc/passwd",
			wantErr: true,
		},
		{
			name:    "URL with variable expansion",
			url:     "https://github.com/user/repo-${HOME}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := jail.ValidateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFilesystemJail_RestrictGitCommand(t *testing.T) {
	jail, err := NewFilesystemJail(&SecurityOptions{
		WorkspaceRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name              string
		args              []string
		wantErr           bool
		checkRestrictions bool
	}{
		{
			name:              "basic clone command",
			args:              []string{"clone", "https://github.com/user/repo.git", "target"},
			wantErr:           false,
			checkRestrictions: true,
		},
		{
			name:    "command with path traversal",
			args:    []string{"clone", "https://github.com/user/repo.git", "../../../etc"},
			wantErr: true,
		},
		{
			name:    "command with home directory",
			args:    []string{"clone", "https://github.com/user/repo.git", "~/target"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restricted, err := jail.RestrictGitCommand(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("RestrictGitCommand() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && tt.checkRestrictions {
				// Check that security restrictions were added
				foundHooksPath := false
				foundProtocol := false

				for i := 0; i < len(restricted)-1; i++ {
					if restricted[i] == "-c" {
						if restricted[i+1] == "core.hooksPath=/dev/null" {
							foundHooksPath = true
						}
						if restricted[i+1] == "protocol.file.allow=never" {
							foundProtocol = true
						}
					}
				}

				if !foundHooksPath {
					t.Error("Expected core.hooksPath restriction not found")
				}
				if !foundProtocol {
					t.Error("Expected protocol.file.allow restriction not found")
				}
			}
		})
	}
}

func TestFilesystemJail_SecureTargetPath(t *testing.T) {
	tempDir := t.TempDir()
	workspaceRoot := filepath.Join(tempDir, "workspace")
	if err := os.MkdirAll(workspaceRoot, 0755); err != nil {
		t.Fatal(err)
	}

	jail, err := NewFilesystemJail(&SecurityOptions{
		WorkspaceRoot: workspaceRoot,
		BlockedPaths:  DefaultSecurityOptions().BlockedPaths,
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		targetDir  string
		wantErr    bool
		wantPrefix string
	}{
		{
			name:       "relative path becomes absolute within workspace",
			targetDir:  "repo",
			wantErr:    false,
			wantPrefix: workspaceRoot,
		},
		{
			name:      "absolute path within workspace",
			targetDir: filepath.Join(workspaceRoot, "repo"),
			wantErr:   false,
		},
		{
			name:      "path outside workspace",
			targetDir: "/tmp/outside",
			wantErr:   true,
		},
		{
			name:      "path with traversal",
			targetDir: "../outside",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secured, err := jail.SecureTargetPath(tt.targetDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("SecureTargetPath() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && tt.wantPrefix != "" {
				if !strings.Contains(secured, tt.wantPrefix) {
					t.Errorf("SecureTargetPath() = %v, want path containing %v", secured, tt.wantPrefix)
				}
			}
		})
	}
}
