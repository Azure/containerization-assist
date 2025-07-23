// Package steps contains individual workflow step implementations.
package steps

import (
	"context"
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/mark3labs/mcp-go/server"
)

func init() {
	Register(NewAnalyzeStep())
}

// AnalyzeStep implements repository analysis
type AnalyzeStep struct{}

// NewAnalyzeStep creates a new analyze step
func NewAnalyzeStep() workflow.Step {
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
func (s *AnalyzeStep) Execute(ctx context.Context, state *workflow.WorkflowState) error {
	var input, inputDesc string
	if state.Args.RepoURL != "" {
		input = state.Args.RepoURL
		inputDesc = fmt.Sprintf("repo_url=%s", state.Args.RepoURL)
	} else {
		input = state.Args.RepoPath
		inputDesc = fmt.Sprintf("repo_path=%s", state.Args.RepoPath)
	}

	state.Logger.Info("Step 1: Analyzing repository", "input", inputDesc)

	analyzeResult, err := AnalyzeRepository(input, state.Args.Branch, state.Logger)
	if err != nil {
		return fmt.Errorf("repository analysis failed: %v", err)
	}

	state.Logger.Info("Repository analysis completed",
		"language", analyzeResult.Language,
		"framework", analyzeResult.Framework,
		"port", analyzeResult.Port)

	// Enhance analysis with AI if available (skip in test mode)
	if server.ServerFromContext(ctx) != nil && !state.Args.TestMode {
		state.Logger.Info("Enhancing repository analysis with AI")
		enhancedResult, enhanceErr := EnhanceRepositoryAnalysis(ctx, analyzeResult, state.Logger)
		if enhanceErr == nil {
			analyzeResult = enhancedResult
			state.Logger.Info("Repository analysis enhanced by AI",
				"language", analyzeResult.Language,
				"framework", analyzeResult.Framework,
				"port", analyzeResult.Port)
		}
	}

	// Convert to workflow type
	state.AnalyzeResult = &workflow.AnalyzeResult{
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

	state.Result.RepoPath = analyzeResult.RepoPath

	return nil
}
