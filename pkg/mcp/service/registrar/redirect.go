// Package registrar handles redirect mechanism for failed tools
package registrar

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

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
	// Response templates
	stepCompletedTemplate = `**{{.StepName}} completed successfully**

**Progress:** Step {{.CurrentStep}} of {{.TotalSteps}} completed
**Session ID:** {{.SessionID}}
{{.RepoAnalysisSection}}
**Next Step:** {{.NextStep}}

**Action:** Call tool "{{.NextStep}}" to continue the workflow.`

	workflowCompletedTemplate = `**{{.StepName}} completed successfully**

**Containerization workflow completed successfully!**

All {{.TotalSteps}} steps have been executed. Your application should now be containerized and deployed.`

	fallbackTemplate = `**{{.StepName}} completed successfully**

**Session ID:** {{.SessionID}}`

	// Redirect response templates
	redirectWithAITemplate = `Tool {{.FromTool}} failed with error:

{{.FormattedError}}

**Fixing Strategy**: {{.FixingStrategy}}

**AI Guidance for {{.RedirectTo}}**:

**System Context:**
{{.SystemPrompt}}

**User Prompt:**
{{.UserPrompt}}

**Expected Output:** {{.ExpectedOutput}}

**Parameters for next call:**
- session_id: {{.SessionID}}
- previous_error: {{.Error}}
- failed_tool: {{.FromTool}}
- fixing_mode: true

**Next Action:** 
1. Read the current files (Dockerfile, manifests, etc.) to understand existing configuration
2. Call tool "{{.RedirectTo}}" with the correct parameters and use the AI guidance to generate the corrected content.`

	redirectSimpleTemplate = `Tool {{.FromTool}} failed: {{.Error}}

**Next Action:** 
1. Read the current files to understand existing configuration
2. Call tool "{{.RedirectTo}}" with appropriate parameters to fix the issue.

**Parameters:**
- session_id: {{.SessionID}}
- previous_error: {{.Error}}
- failed_tool: {{.FromTool}}
- fixing_mode: true`

	errorDisplayThreshold = 200
)

// WorkflowTemplateData handles all workflow-related responses (progress, completion, fallback)
type WorkflowTemplateData struct {
	// Common fields
	StepName  string
	SessionID string

	// Progress-specific fields (optional)
	CurrentStep         int
	TotalSteps          int
	RepoAnalysisSection string
	NextStep            string
}

// RedirectTemplateData handles all redirect responses (with or without AI)
type RedirectTemplateData struct {
	// Common redirect fields
	FromTool   string
	Error      string
	RedirectTo string
	SessionID  string

	// AI-specific fields (optional)
	FormattedError string
	FixingStrategy string
	SystemPrompt   string
	UserPrompt     string
	ExpectedOutput string
}

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
	ShouldRedirect bool            `json:"should_redirect"`
	RedirectTo     string          `json:"redirect_to"`
	Reason         string          `json:"reason"`
	FailedTool     string          `json:"failed_tool"`
	FailureReason  string          `json:"failure_reason"`
	Parameters     map[string]any  `json:"parameters"`
	FixingMode     bool            `json:"fixing_mode"`
	AIPrompt       *AIFixingPrompt `json:"ai_prompt,omitempty"`
}

// AIFixingPrompt provides structured context for AI-powered fixing
type AIFixingPrompt struct {
	SystemPrompt   string         `json:"system_prompt"`
	UserPrompt     string         `json:"user_prompt"`
	Context        map[string]any `json:"context"`
	ExpectedOutput string         `json:"expected_output"`
	FixingStrategy string         `json:"fixing_strategy"`
}

// createRedirectResponse creates a response instructing the client to call a different tool
func (tr *ToolRegistrar) createRedirectResponse(fromTool, error string, sessionID string) (*mcp.CallToolResult, error) {
	config, hasRedirect := RedirectConfigs[fromTool]
	if !hasRedirect {
		return tr.createErrorResult(fmt.Sprintf("Tool %s failed: %s", fromTool, error))
	}

	aiPrompt := tr.generateAIFixingPrompt(fromTool, config.RedirectTo, error, sessionID)
	responseText := tr.buildRedirectResponseText(fromTool, error, sessionID, config.RedirectTo, aiPrompt)

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
	currentIndex := tr.findStepIndex(stepName)
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
func (tr *ToolRegistrar) formatAnalyzeResult(analyzeResult any) string {
	if analyzeResult != nil {
		return fmt.Sprintf("%v", analyzeResult)
	}
	return ""
}

// buildResponseText constructs the appropriate response text based on workflow progress
func (tr *ToolRegistrar) buildResponseText(stepName string, currentIndex int, analyzeResultStr, sessionID string) string {
	// Handle workflow completion
	if currentIndex == len(WorkflowSequence)-1 {
		return tr.buildCompletedResponse(stepName)
	}

	// Handle step in progress
	if currentIndex >= 0 && currentIndex < len(WorkflowSequence)-1 {
		return tr.buildProgressResponse(stepName, currentIndex, analyzeResultStr, sessionID)
	}

	// Handle unknown/invalid step
	return tr.buildFallbackResponse(stepName, sessionID)
}

// buildCompletedResponse builds response for completed workflow
func (tr *ToolRegistrar) buildCompletedResponse(stepName string) string {
	data := WorkflowTemplateData{
		StepName:   stepName,
		TotalSteps: len(WorkflowSequence),
	}
	return tr.executeTemplate(workflowCompletedTemplate, data)
}

// buildFallbackResponse builds response for unknown steps
func (tr *ToolRegistrar) buildFallbackResponse(stepName, sessionID string) string {
	data := WorkflowTemplateData{
		StepName:  stepName,
		SessionID: sessionID,
	}
	return tr.executeTemplate(fallbackTemplate, data)
}

// executeTemplate safely executes a template with given data
func (tr *ToolRegistrar) executeTemplate(templateStr string, data any) string {
	tmpl, err := template.New("response").Parse(templateStr)
	if err != nil {
		return fmt.Sprintf("Template error: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Sprintf("Template execution error: %v", err)
	}

	return buf.String()
}

// buildRedirectResponseText constructs redirect response text using templates
func (tr *ToolRegistrar) buildRedirectResponseText(fromTool, error, sessionID, redirectTo string, aiPrompt *AIFixingPrompt) string {
	if aiPrompt != nil {
		data := RedirectTemplateData{
			FromTool:       fromTool,
			Error:          error,
			RedirectTo:     redirectTo,
			SessionID:      sessionID,
			FormattedError: tr.formatErrorForDisplay(error),
			FixingStrategy: aiPrompt.FixingStrategy,
			SystemPrompt:   aiPrompt.SystemPrompt,
			UserPrompt:     aiPrompt.UserPrompt,
			ExpectedOutput: aiPrompt.ExpectedOutput,
		}
		return tr.executeTemplate(redirectWithAITemplate, data)
	}

	data := RedirectTemplateData{
		FromTool:   fromTool,
		Error:      error,
		RedirectTo: redirectTo,
		SessionID:  sessionID,
	}
	return tr.executeTemplate(redirectSimpleTemplate, data)
}

// buildProgressResponse builds response for workflow in progress
func (tr *ToolRegistrar) buildProgressResponse(stepName string, currentIndex int, analyzeResultStr, sessionID string) string {
	nextStep := WorkflowSequence[currentIndex+1]
	repoAnalysisSection := tr.formatRepoAnalysisSection(analyzeResultStr)

	data := WorkflowTemplateData{
		StepName:            stepName,
		SessionID:           sessionID,
		CurrentStep:         currentIndex + 1,
		TotalSteps:          len(WorkflowSequence),
		RepoAnalysisSection: repoAnalysisSection,
		NextStep:            nextStep,
	}

	return tr.executeTemplate(stepCompletedTemplate, data)
}

// formatRepoAnalysisSection formats repository analysis section for display
func (tr *ToolRegistrar) formatRepoAnalysisSection(analyzeResultStr string) string {
	if analyzeResultStr == "" {
		return ""
	}
	return fmt.Sprintf(`**Repo Analysis Result:**
%s

`, analyzeResultStr)
}

// formatErrorForDisplay formats error messages for better readability in tool responses
func (tr *ToolRegistrar) formatErrorForDisplay(error string) string {
	if tr.shouldFormatAsCodeBlock(error) {
		return fmt.Sprintf("```\n%s\n```", error)
	}
	return error
}

// shouldFormatAsCodeBlock determines if error should be formatted as code block
func (tr *ToolRegistrar) shouldFormatAsCodeBlock(error string) bool {
	return len(error) > errorDisplayThreshold || strings.Contains(error, "\n")
}
