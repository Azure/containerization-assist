package analyze

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/application/api"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"github.com/rs/zerolog"
)

// TypeSafeAnalyzeRepositoryTool implements the new type-safe api.TypedAnalyzeTool interface
type TypeSafeAnalyzeRepositoryTool struct {
	atomicTool     *AtomicAnalyzeRepositoryTool
	sessionManager session.UnifiedSessionManager
	logger         zerolog.Logger
	timeout        time.Duration
}

// NewTypeSafeAnalyzeRepositoryTool creates a new type-safe analyze repository tool
func NewTypeSafeAnalyzeRepositoryTool(
	atomicTool *AtomicAnalyzeRepositoryTool,
	sessionManager session.UnifiedSessionManager,
	logger zerolog.Logger,
) api.TypedAnalyzeTool {
	return &TypeSafeAnalyzeRepositoryTool{
		atomicTool:     atomicTool,
		sessionManager: sessionManager,
		logger:         logger.With().Str("tool", "typesafe_analyze_repository").Logger(),
		timeout:        5 * time.Minute,
	}
}

// Name implements api.TypedTool
func (t *TypeSafeAnalyzeRepositoryTool) Name() string {
	return "analyze_repository"
}

// Description implements api.TypedTool
func (t *TypeSafeAnalyzeRepositoryTool) Description() string {
	return "Analyzes a repository to detect language, framework, and generate containerization recommendations"
}

// Execute implements api.TypedTool with type-safe input and output
func (t *TypeSafeAnalyzeRepositoryTool) Execute(
	ctx context.Context,
	input api.TypedToolInput[api.TypedAnalyzeInput, api.AnalysisContext],
) (api.TypedToolOutput[api.TypedAnalyzeOutput, api.AnalysisDetails], error) {
	// Telemetry execution removed
	return t.executeInternal(ctx, input)
}

// executeInternal contains the core execution logic
func (t *TypeSafeAnalyzeRepositoryTool) executeInternal(
	ctx context.Context,
	input api.TypedToolInput[api.TypedAnalyzeInput, api.AnalysisContext],
) (api.TypedToolOutput[api.TypedAnalyzeOutput, api.AnalysisDetails], error) {
	startTime := time.Now()

	// Validation is now handled by tag-based validation in the atomic tool

	t.logger.Info().
		Str("session_id", input.SessionID).
		Str("repo_url", input.Data.RepoURL).
		Str("branch", input.Data.Branch).
		Msg("Starting repository analysis")

	// Create or get session
	sess, err := t.sessionManager.GetOrCreateSession(ctx, input.SessionID)
	if err != nil {
		return t.errorOutput(input.SessionID, "Failed to get or create session", err), err
	}

	// Update session state
	sess.AddLabel("analyzing")
	sess.UpdateLastAccessed()

	// Perform the atomic analysis - convert to proper tool input
	toolInput := api.ToolInput{
		SessionID: input.SessionID,
		Data: map[string]interface{}{
			"repo_url": input.Data.RepoURL,
			"branch":   input.Data.Branch,
		},
	}
	rawResult, err := t.atomicTool.Execute(ctx, toolInput)
	if err != nil {
		return t.errorOutput(input.SessionID, "Analysis failed", err), err
	}

	// Store analysis results in session
	sess.RemoveLabel("analyzing")
	sess.AddLabel("analysis_completed")

	// Add execution record
	endTime := time.Now()
	sess.AddToolExecution(session.ToolExecution{
		Tool:      "analyze_repository",
		StartTime: startTime,
		EndTime:   &endTime,
		Success:   err == nil,
	})

	// Extract data from the raw result
	language, _ := rawResult.Data["language"].(string)
	framework, _ := rawResult.Data["framework"].(string)
	dependencies, _ := rawResult.Data["dependencies"].([]interface{})

	// Build output
	output := api.TypedAnalyzeOutput{
		Success:              rawResult.Success,
		SessionID:            input.SessionID,
		Language:             language,
		Framework:            framework,
		Dependencies:         t.convertDependencies(dependencies),
		SecurityIssues:       []api.SecurityIssue{}, // TODO: Extract from result
		BuildRecommendations: []string{},            // Using string slice for now
		AnalysisMetrics: api.AnalysisMetrics{
			FilesAnalyzed:  0, // TODO: Extract from rawResult.Data
			LinesOfCode:    0, // TODO: Extract from rawResult.Data
			AnalysisTime:   time.Since(startTime),
			CodeComplexity: 0, // TODO: Extract from rawResult.Data
			TestCoverage:   0, // TODO: Extract from rawResult.Data
		},
	}

	// Build details
	details := api.AnalysisDetails{
		ExecutionDetails: api.ExecutionDetails{
			Duration:  time.Since(startTime),
			StartTime: startTime,
			EndTime:   time.Now(),
			ResourcesUsed: api.ResourceUsage{
				CPUTime:    int64(time.Since(startTime).Milliseconds()),
				MemoryPeak: 0, // TODO: Implement memory tracking
				NetworkIO:  0, // TODO: Implement network tracking
				DiskIO:     0, // TODO: Implement disk tracking
			},
		},
		FilesScanned: 0, // TODO: Extract from rawResult.Data
		IssuesFound:  0, // TODO: Extract from rawResult.Data
		CodeCoverage: 0, // TODO: Extract from rawResult.Data
	}

	t.logger.Info().
		Str("session_id", input.SessionID).
		Dur("duration", time.Since(startTime)).
		Str("language", language).
		Str("framework", framework).
		Int("files_analyzed", 0). // TODO: Extract from rawResult.Data
		Msg("Repository analysis completed")

	return api.TypedToolOutput[api.TypedAnalyzeOutput, api.AnalysisDetails]{
		Success: true,
		Data:    output,
		Details: details,
	}, nil
}

// Schema implements api.TypedTool
func (t *TypeSafeAnalyzeRepositoryTool) Schema() api.TypedToolSchema[api.TypedAnalyzeInput, api.AnalysisContext, api.TypedAnalyzeOutput, api.AnalysisDetails] {
	return api.TypedToolSchema[api.TypedAnalyzeInput, api.AnalysisContext, api.TypedAnalyzeOutput, api.AnalysisDetails]{
		Name:        t.Name(),
		Description: t.Description(),
		Version:     "2.0.0",
		InputExample: api.TypedToolInput[api.TypedAnalyzeInput, api.AnalysisContext]{
			SessionID: "example-session-123",
			Data: api.TypedAnalyzeInput{
				SessionID:            "example-session-123",
				RepoURL:              "https://github.com/example/repo",
				Branch:               "main",
				IncludeDependencies:  true,
				IncludeSecurityScan:  true,
				IncludeBuildAnalysis: true,
			},
			Context: api.AnalysisContext{
				ExecutionContext: api.ExecutionContext{
					RequestID: "req-123",
					TraceID:   "trace-456",
					Timeout:   5 * time.Minute,
				},
				Branch:        "main",
				AnalysisDepth: 3,
			},
		},
		OutputExample: api.TypedToolOutput[api.TypedAnalyzeOutput, api.AnalysisDetails]{
			Success: true,
			Data: api.TypedAnalyzeOutput{
				Success:   true,
				SessionID: "example-session-123",
				Language:  "go",
				Framework: "gin",
			},
		},
		Tags:     []string{"analysis", "repository", "security"},
		Category: api.CategoryAnalyze,
	}
}

// validateInput validates the typed input
func (t *TypeSafeAnalyzeRepositoryTool) validateInput(input api.TypedToolInput[api.TypedAnalyzeInput, api.AnalysisContext]) error {
	if input.SessionID == "" {
		return errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("Session ID is required").
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Build()
	}

	if input.Data.RepoURL == "" {
		return errors.NewError().
			Code(errors.CodeInvalidParameter).
			Message("Repository URL is required").
			Type(errors.ErrTypeValidation).
			Severity(errors.SeverityMedium).
			Build()
	}

	return nil
}

// errorOutput creates an error output
func (t *TypeSafeAnalyzeRepositoryTool) errorOutput(sessionID, message string, err error) api.TypedToolOutput[api.TypedAnalyzeOutput, api.AnalysisDetails] {
	return api.TypedToolOutput[api.TypedAnalyzeOutput, api.AnalysisDetails]{
		Success: false,
		Data: api.TypedAnalyzeOutput{
			Success:   false,
			SessionID: sessionID,
			ErrorMsg:  fmt.Sprintf("%s: %v", message, err),
		},
		Error: err.Error(),
	}
}

// convertDependencies converts internal dependencies to API format
func (t *TypeSafeAnalyzeRepositoryTool) convertDependencies(deps []interface{}) []api.Dependency {
	result := make([]api.Dependency, 0, len(deps))
	// TODO: Implement proper conversion based on actual dependency structure
	return result
}

// convertSecurityIssues converts internal security findings to API format
func (t *TypeSafeAnalyzeRepositoryTool) convertSecurityIssues(findings []interface{}) []api.SecurityIssue {
	result := make([]api.SecurityIssue, 0, len(findings))
	// TODO: Implement proper conversion based on actual security finding structure
	return result
}
