// Package workflow provides shared utilities for workflow orchestration
package workflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/containerization-assist/pkg/domain/progress"
)

// NoOpSink is a no-operation progress sink for fallback cases
type NoOpSink struct{}

func (n *NoOpSink) Publish(ctx context.Context, u progress.Update) error { return nil }

func (n *NoOpSink) Close() error { return nil }

// GenerateWorkflowID creates a unique workflow identifier based on repository URL or path
func GenerateWorkflowID(repoInput string) string {
	// Extract repo name from URL or path
	parts := strings.Split(repoInput, "/")
	repoName := "unknown"
	if len(parts) > 0 {
		repoName = strings.TrimSuffix(parts[len(parts)-1], ".git")
		// Handle empty names or special cases
		if repoName == "" || repoName == "." {
			if len(parts) > 1 {
				repoName = parts[len(parts)-2]
			}
		}
	}

	// Generate unique workflow ID
	timestamp := time.Now().Unix()
	return fmt.Sprintf("workflow-%s-%d", repoName, timestamp)
}

// GetRepositoryIdentifier returns the repository identifier from workflow args
func GetRepositoryIdentifier(args *ContainerizeAndDeployArgs) string {
	if args.RepoPath != "" {
		return args.RepoPath
	}
	return args.RepoURL
}
