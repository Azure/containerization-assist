// Package util provides common utility functions for the MCP package
package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// ID generates a cryptographically secure random ID of the specified length.
// The returned string will be hex-encoded and thus twice the length of n.
func ID(n int) string {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		// This should never happen with crypto/rand
		panic("failed to generate random ID: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// IDWithPrefix generates a random ID with a given prefix.
func IDWithPrefix(prefix string, n int) string {
	return prefix + ID(n)
}

// ShortID generates a short random ID suitable for user-facing identifiers.
// Returns a hex string of 8 characters (4 bytes).
func ShortID() string {
	return ID(4)
}

// LongID generates a long random ID suitable for internal identifiers.
// Returns a hex string of 32 characters (16 bytes).
func LongID() string {
	return ID(16)
}

// WorkflowID generates a unique workflow identifier based on repository URL
func WorkflowID(repoURL string) string {
	// Extract repo name from URL
	parts := strings.Split(repoURL, "/")
	repoName := "unknown"
	if len(parts) > 0 {
		repoName = strings.TrimSuffix(parts[len(parts)-1], ".git")
	}

	// Generate unique workflow ID
	timestamp := time.Now().Unix()
	return fmt.Sprintf("workflow-%s-%d", repoName, timestamp)
}

// EventID generates a unique event identifier
func EventID() string {
	return time.Now().Format("20060102150405") + "-" + ShortID()
}
