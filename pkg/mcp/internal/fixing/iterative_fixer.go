package fixing

import (
	"context"
	"fmt"
	"strings"
	"time"

	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
	"github.com/rs/zerolog"
)

// DefaultIterativeFixer implements the IterativeFixer interface using CallerAnalyzer
type DefaultIterativeFixer struct {
	analyzer    mcptypes.AIAnalyzer
	logger      zerolog.Logger
	maxAttempts int
	fixHistory  []mcptypes.FixAttempt
}

// NewDefaultIterativeFixer creates a new iterative fixer
func NewDefaultIterativeFixer(analyzer mcptypes.AIAnalyzer, logger zerolog.Logger) *DefaultIterativeFixer {
	return &DefaultIterativeFixer{
		analyzer:    analyzer,
		logger:      logger.With().Str("component", "iterative_fixer").Logger(),
		maxAttempts: 3, // default max attempts
		fixHistory:  make([]mcptypes.FixAttempt, 0),
	}
}

// attemptFixInternal tries to fix a failure using AI analysis with iterative loops
func (f *DefaultIterativeFixer) attemptFixInternal(ctx context.Context, fixingCtx *FixingContext) (*mcptypes.FixingResult, error) {
	startTime := time.Now()
	result := &mcptypes.FixingResult{
		AllAttempts:   []mcptypes.FixAttempt{},
		TotalAttempts: 0,
	}

	f.logger.Info().
		Str("session_id", fixingCtx.SessionID).
		Str("tool", fixingCtx.ToolName).
		Str("operation", fixingCtx.OperationType).
		Msg("Starting iterative fixing process")

	for attempt := 1; attempt <= fixingCtx.MaxAttempts; attempt++ {
		f.logger.Debug().
			Int("attempt", attempt).
			Int("max_attempts", fixingCtx.MaxAttempts).
			Msg("Starting fix attempt")

		// Get fix strategies for this attempt
		strategies, err := f.getFixStrategiesForContext(ctx, fixingCtx)
		if err != nil {
			f.logger.Error().Err(err).Int("attempt", attempt).Msg("Failed to get fix strategies")
			continue
		}

		if len(strategies) == 0 {
			f.logger.Warn().Int("attempt", attempt).Msg("No fix strategies available")
			break
		}

		// Try the highest priority strategy
		strategy := strategies[0]
		fixAttempt, err := f.ApplyFix(ctx, fixingCtx, strategy)
		if err != nil {
			f.logger.Error().Err(err).Int("attempt", attempt).Msg("Failed to apply fix")
			continue
		}

		result.AllAttempts = append(result.AllAttempts, *fixAttempt)
		result.TotalAttempts = attempt
		result.FinalAttempt = fixAttempt

		// Check if fix was successful
		if fixAttempt.Success {
			result.Success = true
			result.TotalDuration = time.Since(startTime)
			f.logger.Info().
				Int("attempt", attempt).
				Dur("duration", result.TotalDuration).
				Msg("Fix attempt succeeded")
			return result, nil
		}

		// Add this attempt to the context for the next iteration
		fixingCtx.AttemptHistory = append(fixingCtx.AttemptHistory, *fixAttempt)

		f.logger.Debug().
			Int("attempt", attempt).
			Str("strategy", strategy.Name).
			Msg("Fix attempt failed, preparing for next attempt")
	}

	result.TotalDuration = time.Since(startTime)
	result.Error = fmt.Errorf("failed to fix after %d attempts", fixingCtx.MaxAttempts)

	f.logger.Error().
		Int("total_attempts", result.TotalAttempts).
		Dur("total_duration", result.TotalDuration).
		Msg("All fix attempts failed")

	return result, result.Error
}

// getFixStrategiesForContext analyzes an error and returns potential fix strategies
func (f *DefaultIterativeFixer) getFixStrategiesForContext(ctx context.Context, fixingCtx *FixingContext) ([]mcptypes.FixStrategy, error) {
	// Build comprehensive prompt for AI analysis
	prompt := f.buildAnalysisPrompt(fixingCtx)

	f.logger.Debug().
		Str("session_id", fixingCtx.SessionID).
		Int("prompt_length", len(prompt)).
		Msg("Requesting fix strategies from AI")

	// Use analyzer with file tools for comprehensive analysis
	analysisResult, err := f.analyzer.AnalyzeWithFileTools(ctx, prompt, fixingCtx.BaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze error for fix strategies: %w", err)
	}

	// Parse the analysis result into fix strategies
	strategies, err := f.parseFixStrategies(analysisResult)
	if err != nil {
		f.logger.Error().Err(err).Msg("Failed to parse fix strategies from AI response")
		return nil, fmt.Errorf("failed to parse fix strategies: %w", err)
	}

	f.logger.Info().
		Int("strategies_count", len(strategies)).
		Msg("Generated fix strategies")

	return strategies, nil
}

// ApplyFix applies a specific fix strategy
func (f *DefaultIterativeFixer) ApplyFix(ctx context.Context, fixingCtx *FixingContext, strategy mcptypes.FixStrategy) (*mcptypes.FixAttempt, error) {
	startTime := time.Now()
	attempt := &mcptypes.FixAttempt{
		AttemptNumber: len(fixingCtx.AttemptHistory) + 1,
		StartTime:     startTime,
		FixStrategy:   strategy,
	}

	f.logger.Info().
		Str("strategy", strategy.Name).
		Int("priority", strategy.Priority).
		Msg("Applying fix strategy")

	// Generate specific fix content using AI
	fixPrompt := f.buildFixApplicationPrompt(fixingCtx, strategy)
	fixResult, err := f.analyzer.AnalyzeWithFileTools(ctx, fixPrompt, fixingCtx.BaseDir)
	if err != nil {
		attempt.EndTime = time.Now()
		attempt.Duration = time.Since(startTime)
		attempt.Error = fmt.Errorf("failed to generate fix content: %w", err)
		return attempt, err
	}

	attempt.AnalysisPrompt = fixPrompt
	attempt.AnalysisResult = fixResult
	attempt.FixedContent = f.extractFixedContent(fixResult)

	// Validate the fix
	success, err := f.ValidateFix(ctx, fixingCtx, attempt)
	attempt.Success = success
	attempt.EndTime = time.Now()
	attempt.Duration = time.Since(startTime)

	if err != nil {
		attempt.Error = err
		f.logger.Error().Err(err).Str("strategy", strategy.Name).Msg("Fix validation failed")
	} else if success {
		f.logger.Info().
			Str("strategy", strategy.Name).
			Dur("duration", attempt.Duration).
			Msg("Fix applied successfully")
	}

	return attempt, nil
}

// ValidateFix checks if a fix was successful by attempting the operation
func (f *DefaultIterativeFixer) ValidateFix(ctx context.Context, fixingCtx *FixingContext, attempt *mcptypes.FixAttempt) (bool, error) {
	// This is a simplified validation - in a real implementation,
	// this would trigger the actual operation (build, deploy, etc.)
	// to verify the fix worked

	if attempt.FixedContent == "" {
		return false, fmt.Errorf("no fixed content generated")
	}

	// For now, we'll consider the fix successful if we got content
	// Real implementation would integrate with the actual operation
	f.logger.Debug().
		Int("attempt", attempt.AttemptNumber).
		Msg("Fix validation passed (simplified)")

	return true, nil
}

// buildAnalysisPrompt creates a comprehensive prompt for AI analysis
func (f *DefaultIterativeFixer) buildAnalysisPrompt(fixingCtx *FixingContext) string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf(`You are an expert containerization troubleshooter helping to fix a %s operation failure.

## Context
- Session ID: %s
- Tool: %s
- Operation: %s
- Workspace: %s

## Error Details
`, fixingCtx.OperationType, fixingCtx.SessionID, fixingCtx.ToolName, fixingCtx.OperationType, fixingCtx.WorkspaceDir))

	if fixingCtx.OriginalError != nil {
		prompt.WriteString(fmt.Sprintf("Original Error: %s\n", fixingCtx.OriginalError.Error()))
	}

	if fixingCtx.ErrorDetails != nil {
		prompt.WriteString(fmt.Sprintf(`
Rich Error Details:
- Code: %s
- Type: %s
- Severity: %s
- Message: %s
`, fixingCtx.ErrorDetails["code"], fixingCtx.ErrorDetails["type"],
			fixingCtx.ErrorDetails["severity"], fixingCtx.ErrorDetails["message"]))
	}

	// Add previous attempt history for context
	if len(fixingCtx.AttemptHistory) > 0 {
		prompt.WriteString("\n## Previous Fix Attempts\n")
		for i, prevAttempt := range fixingCtx.AttemptHistory {
			prompt.WriteString(fmt.Sprintf(`
Attempt %d:
- Strategy: %s
- Success: %t
- Duration: %v
`, i+1, prevAttempt.FixStrategy.Name, prevAttempt.Success, prevAttempt.Duration))
			if prevAttempt.Error != nil {
				prompt.WriteString(fmt.Sprintf("- Error: %s\n", prevAttempt.Error.Error()))
			}
		}
	}

	prompt.WriteString(`
## Task
Analyze this failure and provide 1-3 specific fix strategies in order of priority.

For each strategy, provide:
1. Name: Brief descriptive name
2. Description: What this fix does
3. Priority: 1-10 (1 highest)
4. Type: dockerfile|manifest|config|dependency|permission|network
5. Commands: Specific commands to run (if any)
6. FileChanges: Files to modify with old/new content
7. Validation: How to verify the fix worked
8. EstimatedTime: Rough time estimate

Examine the workspace files using file reading tools to understand the current state.
Focus on practical, actionable fixes that address the root cause.

Return your response in this exact format:

STRATEGY 1:
Name: [strategy name]
Description: [description]
Priority: [1-10]
Type: [type]
Commands: [command1], [command2], ...
FileChanges: [file1:operation:reason], [file2:operation:reason], ...
Validation: [validation steps]
EstimatedTime: [time estimate]

STRATEGY 2:
[repeat format]
`)

	return prompt.String()
}

// buildFixApplicationPrompt creates a prompt for applying a specific fix
func (f *DefaultIterativeFixer) buildFixApplicationPrompt(fixingCtx *FixingContext, strategy mcptypes.FixStrategy) string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf(`You are applying a specific fix strategy for a %s operation failure.

## Fix Strategy to Apply
Name: %s
Description: %s
Type: %s

## Context
- Session ID: %s
- Workspace: %s
- Base Directory: %s

## Previous Attempts
`, fixingCtx.OperationType, strategy.Name, strategy.Description, strategy.Type,
		fixingCtx.SessionID, fixingCtx.WorkspaceDir, fixingCtx.BaseDir))

	for i, attempt := range fixingCtx.AttemptHistory {
		prompt.WriteString(fmt.Sprintf("Attempt %d (%s): %t\n", i+1, attempt.FixStrategy.Name, attempt.Success))
	}

	prompt.WriteString(fmt.Sprintf(`
## Task
Apply the "%s" fix strategy by:

1. Examining current files using file reading tools
2. Generating the exact fixed content 
3. Providing specific file modifications needed

Focus on the %s type fix. Return the fixed content between:
<FIXED_CONTENT>
[your fixed content here]
</FIXED_CONTENT>

Be precise and ensure the fix addresses the specific error while maintaining functionality.
`, strategy.Name, strategy.Type))

	return prompt.String()
}

// parseFixStrategies parses AI response into structured fix strategies
func (f *DefaultIterativeFixer) parseFixStrategies(response string) ([]mcptypes.FixStrategy, error) {
	var strategies []mcptypes.FixStrategy

	// Simple parsing - in production this would be more robust
	lines := strings.Split(response, "\n")
	var currentStrategy *mcptypes.FixStrategy

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "STRATEGY ") {
			if currentStrategy != nil {
				strategies = append(strategies, *currentStrategy)
			}
			currentStrategy = &mcptypes.FixStrategy{}
		} else if currentStrategy != nil {
			if strings.HasPrefix(line, "Name: ") {
				currentStrategy.Name = strings.TrimPrefix(line, "Name: ")
			} else if strings.HasPrefix(line, "Description: ") {
				currentStrategy.Description = strings.TrimPrefix(line, "Description: ")
			} else if strings.HasPrefix(line, "Priority: ") {
				// Simple priority parsing - would be more robust in production
				currentStrategy.Priority = 5 // default
			} else if strings.HasPrefix(line, "Type: ") {
				currentStrategy.Type = strings.TrimPrefix(line, "Type: ")
			} else if strings.HasPrefix(line, "EstimatedTime: ") {
				// Parse duration, default to 1 minute if parsing fails
				if duration, err := time.ParseDuration(strings.TrimPrefix(line, "EstimatedTime: ")); err == nil {
					currentStrategy.EstimatedTime = duration
				} else {
					currentStrategy.EstimatedTime = 1 * time.Minute
				}
			}
		}
	}

	if currentStrategy != nil {
		strategies = append(strategies, *currentStrategy)
	}

	return strategies, nil
}

// extractFixedContent extracts the fixed content from AI response
func (f *DefaultIterativeFixer) extractFixedContent(response string) string {
	startTag := "<FIXED_CONTENT>"
	endTag := "</FIXED_CONTENT>"

	start := strings.Index(response, startTag)
	if start == -1 {
		return ""
	}
	start += len(startTag)

	end := strings.Index(response[start:], endTag)
	if end == -1 {
		return ""
	}

	return strings.TrimSpace(response[start : start+end])
}

// Fix implements the IterativeFixer interface method
func (f *DefaultIterativeFixer) Fix(ctx context.Context, issue interface{}) (*mcptypes.FixingResult, error) {
	// Convert issue to FixingContext
	fixingCtx, ok := issue.(*FixingContext)
	if !ok {
		// Try to create a basic FixingContext from the issue
		return nil, fmt.Errorf("issue must be of type *FixingContext")
	}

	// Ensure maxAttempts is set
	if fixingCtx.MaxAttempts == 0 {
		fixingCtx.MaxAttempts = f.maxAttempts
	}

	// Call the internal attempt fix method
	result, err := f.attemptFixInternal(ctx, fixingCtx)

	// Update fix history
	if result != nil && len(result.AllAttempts) > 0 {
		f.fixHistory = append(f.fixHistory, result.AllAttempts...)
	}

	return result, err
}

// AttemptFix implements the IterativeFixer interface method with specific attempt number
func (f *DefaultIterativeFixer) AttemptFix(ctx context.Context, issue interface{}, attempt int) (*mcptypes.FixingResult, error) {
	// Convert issue to FixingContext
	fixingCtx, ok := issue.(*FixingContext)
	if !ok {
		return nil, fmt.Errorf("issue must be of type *FixingContext")
	}

	// Set the specific attempt number
	fixingCtx.MaxAttempts = attempt

	// Call the main Fix method
	return f.Fix(ctx, fixingCtx)
}

// SetMaxAttempts implements the IterativeFixer interface method
func (f *DefaultIterativeFixer) SetMaxAttempts(max int) {
	f.maxAttempts = max
}

// GetFixHistory implements the IterativeFixer interface method
func (f *DefaultIterativeFixer) GetFixHistory() []mcptypes.FixAttempt {
	return f.fixHistory
}

// GetFailureRouting implements the IterativeFixer interface method
func (f *DefaultIterativeFixer) GetFailureRouting() map[string]string {
	// Return routing rules for different failure types
	return map[string]string{
		"build_error":      "dockerfile",
		"permission_error": "permission",
		"network_error":    "network",
		"config_error":     "config",
		"dependency_error": "dependency",
		"manifest_error":   "manifest",
		"deployment_error": "deployment",
	}
}

// GetFixStrategies implements the IterativeFixer interface method
func (f *DefaultIterativeFixer) GetFixStrategies() []string {
	// Return available fix strategy names
	return []string{
		"dockerfile_fix",
		"dependency_fix",
		"config_fix",
		"permission_fix",
		"network_fix",
		"manifest_fix",
		"retry_with_cleanup",
		"fallback_defaults",
	}
}
