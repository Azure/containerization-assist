// Package steps contains individual workflow step implementations.
package steps

import (
	"context"
	"fmt"

	"github.com/Azure/containerization-assist/pkg/domain/workflow"
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
func (s *AnalyzeStep) Execute(ctx context.Context, state *workflow.WorkflowState) (*workflow.StepResult, error) {
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
		return nil, fmt.Errorf("repository analysis failed: %v", err)
	}

	state.Logger.Info("Repository analysis completed")

	// Enhance analysis with AI if available (skip in test mode)
	// Determine test mode via centralized helper to avoid duplication across steps
	testMode := state.IsTestMode()

	if server.ServerFromContext(ctx) != nil && !testMode {
		state.Logger.Info("Enhancing repository analysis with AI")
		enhancedResult, enhanceErr := EnhanceRepositoryAnalysis(ctx, analyzeResult, state.Logger)
		if enhanceErr == nil {
			analyzeResult = enhancedResult
			state.Logger.Info("Repository analysis enhanced by AI")
		}
	}

	// Convert to workflow type
	state.AnalyzeResult = &workflow.AnalyzeResult{
		Language:        analyzeResult.Language,
		LanguageVersion: analyzeResult.LanguageVersion,
		Framework:       analyzeResult.Framework,
		Port:            analyzeResult.Port,
		Metadata:        analyzeResult.Analysis, // Use Analysis field as metadata
		RepoPath:        analyzeResult.RepoPath,
		// Set reasonable defaults for missing fields
		BuildCommand:    "",
		StartCommand:    "",
		Dependencies:    analyzeResult.Dependencies,
		DevDependencies: []string{},
	}

	state.Result.RepoPath = analyzeResult.RepoPath

	// Return StepResult with minimal data and metadata
	return &workflow.StepResult{
		Success: true,
		Data: map[string]interface{}{
			"language":         analyzeResult.Language,
			"language_version": analyzeResult.LanguageVersion,
			"framework":        analyzeResult.Framework,
			"port":             analyzeResult.Port,
			"repo_path":        analyzeResult.RepoPath,
		},
		Metadata: map[string]interface{}{
			"analysis": analyzeResult.Analysis,
		},
	}, nil
}
