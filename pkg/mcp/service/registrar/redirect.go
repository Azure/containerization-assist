// Package registrar handles redirect mechanism for failed tools
package registrar

import (
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
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
func (tr *ToolRegistrar) createRedirectResponse(failedTool, error string, sessionID string) (*mcp.CallToolResult, error) {
	config, hasRedirect := RedirectConfigs[failedTool]
	if !hasRedirect {
		// No redirect configured - return normal error
		return tr.createErrorResult(fmt.Sprintf("Tool %s failed: %s", failedTool, error))
	}

	// Generate AI fixing prompt
	aiPrompt := tr.generateAIFixingPrompt(failedTool, config.RedirectTo, error, sessionID)

	// Create redirect instruction (only AIPrompt is used for text response)
	instruction := RedirectInstruction{
		AIPrompt: aiPrompt,
	}

	// Create text-based response with AI prompt
	var responseText string
	if instruction.AIPrompt != nil {
		responseText = fmt.Sprintf(`Tool %s failed: %s

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

**Next Action:** Call tool "%s" with the above parameters and use the AI guidance to generate the corrected content.`,
			failedTool,
			error,
			instruction.AIPrompt.FixingStrategy,
			config.RedirectTo,
			instruction.AIPrompt.SystemPrompt,
			instruction.AIPrompt.UserPrompt,
			instruction.AIPrompt.ExpectedOutput,
			sessionID,
			error,
			failedTool,
			config.RedirectTo)
	} else {
		responseText = fmt.Sprintf(`Tool %s failed: %s

**Next Action:** Call tool "%s" to fix the issue.

**Parameters:**
- session_id: %s
- previous_error: %s  
- failed_tool: %s
- fixing_mode: true`,
			failedTool,
			error,
			config.RedirectTo,
			sessionID,
			error,
			failedTool)
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
func (tr *ToolRegistrar) createProgressResponse(stepName, message string, _ map[string]interface{}, sessionID string) (*mcp.CallToolResult, error) {
	// Workflow sequence for determining next step
	workflowSequence := []string{
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

	// Find current step index
	currentIndex := -1
	for i, step := range workflowSequence {
		if step == stepName {
			currentIndex = i
			break
		}
	}

	// Build text-based response
	var responseText string

	// Add next step instruction
	if currentIndex >= 0 && currentIndex < len(workflowSequence)-1 {
		nextStep := workflowSequence[currentIndex+1]
		responseText = fmt.Sprintf(`**%s completed successfully**

%s

**Progress:** Step %d of %d completed

**Next Step:** %s
**Parameters:**
- session_id: %s

**Action:** Call tool "%s" to continue the workflow.`,
			stepName,
			message,
			currentIndex+1,
			len(workflowSequence),
			nextStep,
			sessionID,
			nextStep)
	} else if currentIndex == len(workflowSequence)-1 {
		// Last step completed
		responseText = fmt.Sprintf(`**%s completed successfully**

%s

**Containerization workflow completed successfully!**

All %d steps have been executed. Your application should now be containerized and deployed.`,
			stepName,
			message,
			len(workflowSequence))
	} else {
		// Fallback for unknown step
		responseText = fmt.Sprintf(`**%s completed successfully**

%s

**Session ID:** %s`,
			stepName,
			message,
			sessionID)
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
