// Package registrar handles redirect mechanism for failed tools
package registrar

import (
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// Workflow sequence constants
var WorkflowSequence = []string{
	"analyze_repository",
	"generate_dockerfile",
	"build_image",
	"scan_image",
	"tag_image",
	"push_image",
	"generate_k8s_manifests",
	"prepare_cluster",
	"deploy_application",
	"verify_deployment",
}

const (
	// Response template constants
	completedTemplate = `**%s completed successfully**

**Progress:** Step %d of %d completed
%s**Next Step:** %s
**Parameters:**
- session_id: %s

**Action:** Call tool "%s" to continue the workflow.`

	workflowCompletedTemplate = `**%s completed successfully**

**Containerization workflow completed successfully!**

All %d steps have been executed. Your application should now be containerized and deployed.`

	fallbackTemplate = `**%s completed successfully**

**Session ID:** %s`

	errorDisplayThreshold = 200
)

// RedirectConfig defines where to redirect when a tool fails
type RedirectConfig struct {
	RedirectTo   string `json:"redirect_to"`   // Tool to redirect to
	MaxRedirects int    `json:"max_redirects"` // Maximum redirects to prevent loops
	Reason       string `json:"reason"`        // Why this redirect makes sense
}

// RedirectConfigs maps tool names to their redirect configurations
var RedirectConfigs = map[string]RedirectConfig{
	"build_image": {
		RedirectTo:   "generate_dockerfile",
		MaxRedirects: 1,
		Reason:       "Build failures often indicate Dockerfile issues - regenerate with AI fixing",
	},
	"deploy_application": {
		RedirectTo:   "generate_k8s_manifests",
		MaxRedirects: 1,
		Reason:       "Deployment failures often indicate manifest issues - regenerate with AI fixing",
	},
	"push_image": {
		RedirectTo:   "build_image",
		MaxRedirects: 1,
		Reason:       "Push failures may indicate image issues - rebuild with different settings",
	},
	"scan_image": {
		RedirectTo:   "generate_dockerfile",
		MaxRedirects: 1,
		Reason:       "Scan failures may indicate Dockerfile issues - regenerate with AI fixing",
	},
	"verify_deployment": {
		RedirectTo:   "deploy_application",
		MaxRedirects: 1,
		Reason:       "Verification failures may indicate deployment issues - retry deployment",
	},
}

// RedirectInstruction contains the instruction for the client to call the next tool
type RedirectInstruction struct {
	ShouldRedirect bool                   `json:"should_redirect"`
	RedirectTo     string                 `json:"redirect_to"`
	Reason         string                 `json:"reason"`
	FailedTool     string                 `json:"failed_tool"`
	FailureReason  string                 `json:"failure_reason"`
	Parameters     map[string]interface{} `json:"parameters"`
	FixingMode     bool                   `json:"fixing_mode"`
	AIPrompt       *AIFixingPrompt        `json:"ai_prompt,omitempty"`
}

// AIFixingPrompt provides structured context for AI-powered fixing
type AIFixingPrompt struct {
	SystemPrompt   string                 `json:"system_prompt"`
	UserPrompt     string                 `json:"user_prompt"`
	Context        map[string]interface{} `json:"context"`
	ExpectedOutput string                 `json:"expected_output"`
	FixingStrategy string                 `json:"fixing_strategy"`
}

// createRedirectResponse creates a response instructing the client to call a different tool
func (tr *ToolRegistrar) createRedirectResponse(fromTool, error string, sessionID string) (*mcp.CallToolResult, error) {
	config, hasRedirect := RedirectConfigs[fromTool]
	if !hasRedirect {
		// No redirect configured - return normal error
		return tr.createErrorResult(fmt.Sprintf("Tool %s failed: %s", fromTool, error))
	}

	// Generate AI fixing prompt
	aiPrompt := tr.generateAIFixingPrompt(fromTool, config.RedirectTo, error, sessionID)

	// Create redirect instruction (only AIPrompt is used for text response)
	instruction := RedirectInstruction{
		AIPrompt: aiPrompt,
	}

	// Create text-based response with AI prompt
	var responseText string
	if instruction.AIPrompt != nil {
		// Format error for better readability if it contains build output
		formattedError := tr.formatErrorForDisplay(error)

		responseText = fmt.Sprintf(`Tool %s failed with error:

%s

**Fixing Strategy**: %s

**AI Guidance for %s**:

**System Context:**
%s

**User Prompt:**
%s

**Expected Output:** %s

**Parameters for next call:**
- session_id: %s
- previous_error: %s
- failed_tool: %s
- fixing_mode: true

**Next Action:** 
1. Read the current files (Dockerfile, manifests, etc.) to understand existing configuration
2. Call tool "%s" with the above parameters and use the AI guidance to generate the corrected content.`,
			fromTool,
			formattedError,
			instruction.AIPrompt.FixingStrategy,
			config.RedirectTo,
			instruction.AIPrompt.SystemPrompt,
			instruction.AIPrompt.UserPrompt,
			instruction.AIPrompt.ExpectedOutput,
			sessionID,
			error,
			fromTool,
			config.RedirectTo)
	} else {
		responseText = fmt.Sprintf(`Tool %s failed: %s

**Next Action:** 
1. Read the current files to understand existing configuration
2. Call tool "%s" with appropriate parameters to fix the issue.

**Parameters:**
- session_id: %s
- previous_error: %s  
- failed_tool: %s
- fixing_mode: true`,
			fromTool,
			error,
			config.RedirectTo,
			sessionID,
			error,
			fromTool)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: responseText,
			},
		},
	}, nil
}

// createProgressResponse creates a response for successful tool execution with next step hint
func (tr *ToolRegistrar) createProgressResponse(stepName string, responseData map[string]any, sessionID string) (*mcp.CallToolResult, error) {

	// Find current step index
	currentIndex := tr.findStepIndex(stepName)

	// Build text-based response
	analyzeRepoResultStr := tr.formatAnalyzeResult(responseData["analyze_result"])
	responseText := tr.buildResponseText(stepName, currentIndex, analyzeRepoResultStr, sessionID)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: responseText,
			},
		},
	}, nil
}

// Helper functions for cleaner code organization

// findStepIndex returns the index of a step in the workflow sequence
func (tr *ToolRegistrar) findStepIndex(stepName string) int {
	for i, step := range WorkflowSequence {
		if step == stepName {
			return i
		}
	}
	return -1
}

// formatAnalyzeResult formats the analyze_result for display
func (tr *ToolRegistrar) formatAnalyzeResult(analyzeResult interface{}) string {
	if analyzeResult != nil {
		return fmt.Sprintf("%v", analyzeResult)
	}
	return ""
}

// buildResponseText constructs the appropriate response text based on workflow progress
func (tr *ToolRegistrar) buildResponseText(stepName string, currentIndex int, analyzeResultStr, sessionID string) string {
	switch {
	case currentIndex >= 0 && currentIndex < len(WorkflowSequence)-1:
		return tr.buildProgressResponse(stepName, currentIndex, analyzeResultStr, sessionID)
	case currentIndex == len(WorkflowSequence)-1:
		return fmt.Sprintf(workflowCompletedTemplate, stepName, len(WorkflowSequence))
	default:
		return fmt.Sprintf(fallbackTemplate, stepName, sessionID)
	}
}

// buildProgressResponse builds response for workflow in progress
func (tr *ToolRegistrar) buildProgressResponse(stepName string, currentIndex int, analyzeResultStr, sessionID string) string {
	nextStep := WorkflowSequence[currentIndex+1]
	repoAnalysisSection := ""
	if analyzeResultStr != "" {
		repoAnalysisSection = fmt.Sprintf(`**Repo Analysis Result:**
%s

`, analyzeResultStr)
	}
	return fmt.Sprintf(completedTemplate, stepName, currentIndex+1, len(WorkflowSequence), repoAnalysisSection, nextStep, sessionID, nextStep)
}

// formatErrorForDisplay formats error messages for better readability in tool responses
func (tr *ToolRegistrar) formatErrorForDisplay(error string) string {
	// If error contains build output or multi-line content, format it properly
	if len(error) > errorDisplayThreshold || strings.Contains(error, "\n") {
		return fmt.Sprintf("```\n%s\n```", error)
	}
	return error
}
