package conversation

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

type IntelligentRetrySystem struct {
	promptBuilder *ErrorRecoveryPromptBuilder
	logger        *slog.Logger
}
type RetryContext struct {
	SessionID        string
	OriginalError    string
	AttemptCount     int
	PreviousAttempts []RetryAttempt
	LastResponse     string
	ProjectContext   *RepositoryContext
	TimeSpent        time.Duration
}
type RetryAttempt struct {
	AttemptNumber  int
	Approach       string
	FilesExamined  []string
	RootCauseFound bool
	FixApplied     string
	Result         string
	LessonLearned  string
	TimeSpent      time.Duration
}
type RetryGuidance struct {
	ProgressAssessment string
	NextSteps          []string
	SpecificTools      []string
	FocusAreas         []string
	SuccessIndicators  []string
	AvoidRepeating     []string
}

func NewIntelligentRetrySystem(logger *slog.Logger) *IntelligentRetrySystem {
	return &IntelligentRetrySystem{
		promptBuilder: NewErrorRecoveryPromptBuilder(logger),
		logger:        logger.With("component", "intelligent_retry"),
	}
}
func (irs *IntelligentRetrySystem) GenerateRetryGuidance(ctx context.Context, retryCtx *RetryContext) *RetryGuidance {
	guidance := &RetryGuidance{}
	guidance.ProgressAssessment = irs.assessProgress(retryCtx)
	guidance.FocusAreas = irs.determineFocusAreas(retryCtx)
	guidance.SpecificTools = irs.recommendTools(retryCtx)
	guidance.NextSteps = irs.generateNextSteps(retryCtx)
	guidance.SuccessIndicators = irs.defineSuccessIndicators(retryCtx)
	guidance.AvoidRepeating = irs.identifyThingsToAvoid(retryCtx)

	return guidance
}
func (irs *IntelligentRetrySystem) BuildProgressiveRetryPrompt(ctx context.Context, retryCtx *RetryContext) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔄 **BUILD ERROR RECOVERY - ATTEMPT %d**\n\n", retryCtx.AttemptCount+1))

	if retryCtx.AttemptCount == 1 {
		sb.WriteString("The first fix attempt didn't work. Let's take a more systematic approach.\n\n")
	} else if retryCtx.AttemptCount >= 2 {
		sb.WriteString("Multiple attempts have been made. Time for a deeper, more methodical investigation.\n\n")
	}
	sb.WriteString("## Original Error\n\n")
	sb.WriteString("```\n")
	sb.WriteString(retryCtx.OriginalError)
	sb.WriteString("\n```\n\n")
	guidance := irs.GenerateRetryGuidance(ctx, retryCtx)
	sb.WriteString("## Progress Assessment\n\n")
	sb.WriteString(guidance.ProgressAssessment)
	sb.WriteString("\n\n")
	if len(retryCtx.PreviousAttempts) > 0 {
		sb.WriteString("## Previous Attempts Analysis\n\n")
		for _, attempt := range retryCtx.PreviousAttempts {
			sb.WriteString(fmt.Sprintf("### Attempt %d: %s\n", attempt.AttemptNumber, attempt.Approach))
			sb.WriteString(fmt.Sprintf("- **Files Examined**: %s\n", strings.Join(attempt.FilesExamined, ", ")))
			sb.WriteString(fmt.Sprintf("- **Root Cause Found**: %t\n", attempt.RootCauseFound))
			sb.WriteString(fmt.Sprintf("- **Result**: %s\n", attempt.Result))
			if attempt.LessonLearned != "" {
				sb.WriteString(fmt.Sprintf("- **Lesson**: %s\n", attempt.LessonLearned))
			}
			sb.WriteString("\n")
		}
	}
	sb.WriteString("## Next Steps - Progressive Guidance\n\n")
	for i, step := range guidance.NextSteps {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
	}
	sb.WriteString("\n")
	sb.WriteString("## Focus Areas for This Attempt\n\n")
	for _, area := range guidance.FocusAreas {
		sb.WriteString(fmt.Sprintf("🎯 %s\n", area))
	}
	sb.WriteString("\n")
	sb.WriteString("## Recommended Tools for Investigation\n\n")
	for _, tool := range guidance.SpecificTools {
		sb.WriteString(fmt.Sprintf("🔧 %s\n", tool))
	}
	sb.WriteString("\n")
	sb.WriteString("## Success Indicators - Know When You're On Track\n\n")
	for _, indicator := range guidance.SuccessIndicators {
		sb.WriteString(fmt.Sprintf("✅ %s\n", indicator))
	}
	sb.WriteString("\n")
	if len(guidance.AvoidRepeating) > 0 {
		sb.WriteString("## ❌ Avoid Repeating These Approaches\n\n")
		for _, avoid := range guidance.AvoidRepeating {
			sb.WriteString(fmt.Sprintf("- %s\n", avoid))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("## Required Response Format\n\n")
	sb.WriteString("Structure your response as:\n\n")
	sb.WriteString("1. **Investigation Plan**: What you will investigate and why\n")
	sb.WriteString("2. **Findings**: Detailed findings from your investigation\n")
	sb.WriteString("3. **Root Cause Analysis**: Specific explanation of why the build fails\n")
	sb.WriteString("4. **Solution Strategy**: How your fix addresses the root cause\n")
	sb.WriteString("5. **Fixed Dockerfile**: Complete corrected Dockerfile\n\n")

	sb.WriteString("**Remember**: Take time to thoroughly investigate. A methodical approach is better than quick guesses.\n")

	return sb.String()
}
func (irs *IntelligentRetrySystem) assessProgress(retryCtx *RetryContext) string {
	if len(retryCtx.PreviousAttempts) == 0 {
		return "This is the first retry attempt. Time to be more systematic in the investigation."
	}

	var assessments []string
	foundRootCause := false
	examinedFiles := make(map[string]bool)

	for _, attempt := range retryCtx.PreviousAttempts {
		if attempt.RootCauseFound {
			foundRootCause = true
		}
		for _, file := range attempt.FilesExamined {
			examinedFiles[file] = true
		}
	}

	if foundRootCause {
		assessments = append(assessments, "✅ **Root cause has been identified** - focus on implementing the correct fix")
	} else {
		assessments = append(assessments, "❌ **Root cause not yet identified** - need deeper investigation")
	}

	if len(examinedFiles) > 0 {
		assessments = append(assessments, fmt.Sprintf("📋 **Files examined**: %d files investigated so far", len(examinedFiles)))
	} else {
		assessments = append(assessments, "❌ **Insufficient investigation** - need to examine project files")
	}
	if retryCtx.TimeSpent > 5*time.Minute {
		assessments = append(assessments, "⏰ **Significant time spent** - let's ensure the next attempt is comprehensive")
	}

	return strings.Join(assessments, "\n") + "\n"
}
func (irs *IntelligentRetrySystem) determineFocusAreas(retryCtx *RetryContext) []string {
	areas := []string{}
	examinedFiles := make(map[string]bool)
	for _, attempt := range retryCtx.PreviousAttempts {
		for _, file := range attempt.FilesExamined {
			examinedFiles[file] = true
		}
	}
	errorLower := strings.ToLower(retryCtx.OriginalError)

	if strings.Contains(errorLower, "copy failed") || strings.Contains(errorLower, "no such file") {
		if !examinedFiles["project_structure"] {
			areas = append(areas, "**Project Structure Analysis** - Map out exactly what files exist and where")
		}
		if !examinedFiles["dockerfile_paths"] {
			areas = append(areas, "**Dockerfile Path Verification** - Check every COPY/ADD command against actual file locations")
		}
	} else if strings.Contains(errorLower, "command failed") || strings.Contains(errorLower, "non-zero code") {
		if !examinedFiles["dependency_files"] {
			areas = append(areas, "**Dependency Analysis** - Examine package.json, requirements.txt, etc. for build requirements")
		}
		if !examinedFiles["build_scripts"] {
			areas = append(areas, "**Build Script Analysis** - Look for existing build scripts or documentation")
		}
	}
	if retryCtx.AttemptCount >= 2 {
		areas = append(areas, "**Deep File Investigation** - Read key files line by line, don't just list them")
		areas = append(areas, "**Assumption Validation** - Question every assumption about how the project should build")
	}

	return areas
}
func (irs *IntelligentRetrySystem) recommendTools(retryCtx *RetryContext) []string {
	tools := []string{}
	tools = append(tools, "`scan_repository` - Get overview of project structure if not done thoroughly")
	errorLower := strings.ToLower(retryCtx.OriginalError)

	if strings.Contains(errorLower, "copy failed") {
		tools = append(tools, "`list_directory` - Check exact paths mentioned in error")
		tools = append(tools, "`read_file` - Examine .dockerignore if it exists")
	} else if strings.Contains(errorLower, "package not found") || strings.Contains(errorLower, "module not found") {
		tools = append(tools, "`read_file` - Read dependency files (package.json, requirements.txt, go.mod)")
		tools = append(tools, "`read_file` - Check for lock files (package-lock.json, poetry.lock, go.sum)")
	}

	tools = append(tools, "`read_file` - Read README.md or docs for build instructions")

	return tools
}
func (irs *IntelligentRetrySystem) generateNextSteps(retryCtx *RetryContext) []string {
	steps := []string{}

	if retryCtx.AttemptCount == 1 {
		steps = append(steps, "**Start with systematic repository analysis** - Don't assume anything about the project structure")
		steps = append(steps, "**Map the error to specific files** - Identify exactly which files or commands are mentioned in the error")
		steps = append(steps, "**Verify every assumption** - Check that files exist where the Dockerfile expects them")
	} else {
		steps = append(steps, "**Take a step back** - Re-examine the fundamental assumptions about this project")
		steps = append(steps, "**Deep-dive investigation** - Read the actual content of key files, don't just list them")
		steps = append(steps, "**Cross-reference everything** - Compare error details with actual project structure line by line")
	}

	steps = append(steps, "**Document your findings** - Keep track of what you discover to build a complete picture")
	steps = append(steps, "**Identify the disconnect** - Find the specific gap between what the Dockerfile expects and what actually exists")

	return steps
}
func (irs *IntelligentRetrySystem) defineSuccessIndicators(retryCtx *RetryContext) []string {
	indicators := []string{
		"You can explain exactly why the original command failed",
		"You've verified the existence and location of all files mentioned in the error",
		"You understand the project's actual build requirements and dependencies",
		"You can map the error to a specific mismatch between Dockerfile and project reality",
		"Your proposed fix addresses the root cause, not just the symptoms",
	}
	if retryCtx.AttemptCount >= 2 {
		indicators = append(indicators, "You've identified something new that previous attempts missed")
		indicators = append(indicators, "You can explain why previous fixes didn't work")
	}

	return indicators
}
func (irs *IntelligentRetrySystem) identifyThingsToAvoid(retryCtx *RetryContext) []string {
	avoid := []string{}
	for _, attempt := range retryCtx.PreviousAttempts {
		if strings.Contains(strings.ToLower(attempt.Result), "failed") {
			avoid = append(avoid, fmt.Sprintf("Repeating the approach from attempt %d: %s", attempt.AttemptNumber, attempt.Approach))
		}
	}
	avoid = append(avoid, "Making changes without understanding the root cause")
	avoid = append(avoid, "Assuming file locations without verification")
	avoid = append(avoid, "Guessing at fixes instead of investigating systematically")

	return avoid
}
