package tools

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnsureWithinWorkspace guards against path escapes
func EnsureWithinWorkspace(repoRoot, rel string) (string, error) {
	// Reject absolute paths
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("path escapes workspace: absolute paths not allowed")
	}

	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", fmt.Errorf("abs root: %w", err)
	}

	// Handle the relative path
	cleanRel := filepath.Clean(filepath.FromSlash(rel))
	target := filepath.Join(root, cleanRel)
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("abs target: %w", err)
	}

	// Security check: ensure path is within workspace
	if !strings.HasPrefix(abs, root+string(filepath.Separator)) && abs != root {
		return "", fmt.Errorf("path escapes workspace: %s is outside %s", abs, root)
	}

	return abs, nil
}

// WriteFileAtomic writes atomically and returns hash info for idempotency
func WriteFileAtomic(dest string, content []byte, mode os.FileMode) (changed bool, oldHash, newHash string, err error) {
	// Compute new content hash
	sum := sha256.Sum256(content)
	newHash = hex.EncodeToString(sum[:])

	// Check if file exists with same content (idempotency check)
	if existingContent, readErr := os.ReadFile(dest); readErr == nil {
		oldSum := sha256.Sum256(existingContent)
		oldHash = hex.EncodeToString(oldSum[:])

		// If content is identical, skip write (idempotent)
		if oldHash == newHash {
			return false, oldHash, newHash, nil // No change needed
		}
	} else if !os.IsNotExist(readErr) {
		// Error reading file (not just missing)
		return false, "", "", fmt.Errorf("read existing file: %w", readErr)
	}

	// Ensure directory exists
	dir := filepath.Dir(dest)
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return false, "", "", fmt.Errorf("create directory: %w", err)
	}

	// Write to temporary file first (atomic write pattern)
	tmpFile := dest + ".tmp"
	if err = os.WriteFile(tmpFile, content, mode); err != nil {
		return false, "", "", fmt.Errorf("write temp file: %w", err)
	}

	// Atomic rename (this is atomic on POSIX systems)
	if err = os.Rename(tmpFile, dest); err != nil {
		// Clean up temp file on error
		_ = os.Remove(tmpFile)
		return false, "", "", fmt.Errorf("rename temp to final: %w", err)
	}

	return true, oldHash, newHash, nil
}

// ComputeFileHash computes SHA256 hash of a file
func ComputeFileHash(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]), nil
}

// ExtractBaseImage extracts the base image from Dockerfile content
func ExtractBaseImage(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), "FROM ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				// Handle multi-stage builds (FROM image AS stage)
				return parts[1]
			}
		}
	}
	return ""
}

// ExtractExposedPort extracts the exposed port from Dockerfile content
func ExtractExposedPort(content string) int {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), "EXPOSE ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				// Try to parse the port number
				var port int
				if _, err := fmt.Sscanf(parts[1], "%d", &port); err == nil {
					return port
				}
			}
		}
	}
	return 0
}

// DiffSummary represents a summary of file changes
type DiffSummary struct {
	Path         string `json:"path"`
	Action       string `json:"action"` // created, modified, unchanged
	OldHash      string `json:"old_hash,omitempty"`
	NewHash      string `json:"new_hash"`
	SizeBytes    int    `json:"size_bytes"`
	LinesAdded   int    `json:"lines_added,omitempty"`
	LinesRemoved int    `json:"lines_removed,omitempty"`
}

// CreateDiffSummary creates a summary of file changes
func CreateDiffSummary(path string, changed bool, oldHash, newHash string, content []byte) DiffSummary {
	action := "unchanged"
	if changed {
		if oldHash == "" {
			action = "created"
		} else {
			action = "modified"
		}
	}

	summary := DiffSummary{
		Path:      path,
		Action:    action,
		OldHash:   oldHash,
		NewHash:   newHash,
		SizeBytes: len(content),
	}

	// For text files, count line changes (simplified)
	if changed && oldHash != "" {
		// This is a simplified line count - in production you'd use a proper diff algorithm
		newLines := strings.Count(string(content), "\n")
		summary.LinesAdded = newLines // Simplified - would need old content for accurate diff
	}

	return summary
}

// ValidatePath validates that a path is safe to use
func ValidatePath(path string) error {
	// Check for dangerous patterns
	dangerous := []string{
		"..", // Path traversal
		"~",  // Home directory
		"$",  // Environment variable
		"|",  // Pipe
		">",  // Redirect
		"<",  // Redirect
		"&",  // Background
		";",  // Command separator
		"`",  // Command substitution
		"\\", // Escape (on Unix)
	}

	for _, pattern := range dangerous {
		if strings.Contains(path, pattern) {
			return fmt.Errorf("path contains dangerous pattern '%s'", pattern)
		}
	}

	// Check for absolute paths (we want relative paths)
	if filepath.IsAbs(path) {
		return fmt.Errorf("absolute paths not allowed")
	}

	return nil
}
