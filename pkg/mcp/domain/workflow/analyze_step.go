// Package workflow contains individual workflow step implementations.
package workflow

import (
	"context"
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/infrastructure/steps"
	"github.com/mark3labs/mcp-go/server"
)

// AnalyzeStep implements repository analysis
type AnalyzeStep struct{}

// NewAnalyzeStep creates a new analyze step
func NewAnalyzeStep() Step {
	return &AnalyzeStep{}
}

// Name returns the step name
func (s *AnalyzeStep) Name() string {
	return "analyze_repository"
}

// MaxRetries returns the maximum number of retries for this step
func (s *AnalyzeStep) MaxRetries() int {
	return 2
}

// Execute performs repository analysis
func (s *AnalyzeStep) Execute(ctx context.Context, state *WorkflowState) error {
	state.Logger.Info("Step 1: Analyzing repository", "repo_url", state.Args.RepoURL)

	// Perform basic repository analysis
	analyzeResult, err := steps.AnalyzeRepository(state.Args.RepoURL, state.Args.Branch, state.Logger)
	if err != nil {
		return fmt.Errorf("repository analysis failed: %v", err)
	}

	state.Logger.Info("Repository analysis completed",
		"language", analyzeResult.Language,
		"framework", analyzeResult.Framework,
		"port", analyzeResult.Port)

	// Enhance analysis with AI if available
	if server.ServerFromContext(ctx) != nil {
		state.Logger.Info("Enhancing repository analysis with AI")
		enhancedResult, enhanceErr := steps.EnhanceRepositoryAnalysis(ctx, analyzeResult, state.Logger)
		if enhanceErr == nil {
			analyzeResult = enhancedResult
			state.Logger.Info("Repository analysis enhanced by AI",
				"language", analyzeResult.Language,
				"framework", analyzeResult.Framework,
				"port", analyzeResult.Port)
		}
	}

	// Convert to workflow type
	state.AnalyzeResult = &AnalyzeResult{
		Language:  analyzeResult.Language,
		Framework: analyzeResult.Framework,
		Port:      analyzeResult.Port,
		Metadata:  analyzeResult.Analysis, // Use Analysis field as metadata
		RepoPath:  analyzeResult.RepoPath,
		// Set reasonable defaults for missing fields
		BuildCommand:    "",
		StartCommand:    "",
		Dependencies:    []string{},
		DevDependencies: []string{},
	}

	return nil
}
