// Package registrar handles redirect mechanism for failed tools
package registrar

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/service/session"
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
	// Response templates with generic step-specific sections
	stepCompletedTemplate = `**{{.StepName}} completed successfully**

**Progress:** Step {{.CurrentStep}} of {{.TotalSteps}} completed
**Session ID:** {{.SessionID}}
{{range .StepSpecificSections}}{{.}}{{end}}
**Next Step:** {{.NextStep}}

**Action:** Call tool "{{.NextStep}}" to continue the workflow.`

	workflowCompletedTemplate = `**{{.StepName}} completed successfully**

**Containerization workflow completed successfully!**

All {{.TotalSteps}} steps have been executed. Your application should now be containerized and deployed.
{{range .StepSpecificSections}}{{.}}{{end}}`

	fallbackTemplate = `**{{.StepName}} completed successfully**

**Session ID:** {{.SessionID}}
{{range .StepSpecificSections}}{{.}}{{end}}`

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

	redirectDockerfileRegenerationTemplate = `Tool {{.FromTool}} failed multiple times with error:

{{.FormattedError}}

**Container Image Issue Detected**: After {{.FailureCount}} deployment failures, the issue may be with the container image itself rather than the Kubernetes manifests.

**Fixing Strategy**: Regenerating Dockerfile and rebuilding container image to address potential image-level issues (missing dependencies, wrong ports, incorrect startup commands, etc.)

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
1. Analyze the deployment failure to identify potential container image issues
2. Call tool "{{.RedirectTo}}" to regenerate the Dockerfile with corrections
3. Continue with build_image → deploy_application chain to test the fix`

	redirectSimpleTemplate = `Tool {{.FromTool}} failed: {{.Error}}

**Next Action:** 
1. Read the current files to understand existing configuration
2. Call tool "{{.RedirectTo}}" with appropriate parameters to fix the issue.

**Parameters:**
- session_id: {{.SessionID}}
- previous_error: {{.Error}}
- failed_tool: {{.FromTool}}
- fixing_mode: true`

	// Configuration constants
	errorDisplayThreshold = 200
	maxArrayDisplayItems  = 20
	maxMapDisplayFields   = 50
)

// WorkflowTemplateData handles all workflow-related responses (progress, completion, fallback)
type WorkflowTemplateData struct {
	// Common fields
	StepName  string
	SessionID string

	// Progress-specific fields (optional)
	CurrentStep          int
	TotalSteps           int
	StepSpecificSections []string
	NextStep             string
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

	// Failure tracking fields
	FailureCount int
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

// DeploymentFailureTracking tracks deployment failure attempts and redirect patterns
type DeploymentFailureTracking struct {
	ConsecutiveFailures map[string]int            `json:"consecutive_failures"`
	LastRedirectTo      string                    `json:"last_redirect_to"`
	TotalAttempts       int                       `json:"total_attempts"`
	FailureHistory      []DeploymentFailureRecord `json:"failure_history"`
}

// DeploymentFailureRecord records individual deployment failure details
type DeploymentFailureRecord struct {
	Timestamp  string `json:"timestamp"`
	RedirectTo string `json:"redirect_to"`
	Error      string `json:"error"`
}

// createRedirectResponse creates a response instructing the client to call a different tool
func (tr *ToolRegistrar) createRedirectResponse(fromTool, error string, sessionID string, stepResult ...map[string]any) (*mcp.CallToolResult, error) {
	config, hasRedirect := RedirectConfigs[fromTool]
	if !hasRedirect {
		return tr.createErrorResult(fmt.Sprintf("Tool %s failed: %s", fromTool, error))
	}

	// Special handling for deploy_application - use proper failure tracking
	if fromTool == "deploy_application" {
		redirectTo, err := tr.determineDeploymentRedirectWithTracking(sessionID, config.RedirectTo, error)
		if err != nil {
			// Fallback to default behavior if tracking fails
			tr.logger.Warn("Failed to load deployment failure tracking, using default redirect", "error", err)
			redirectTo = config.RedirectTo
		}

		if redirectTo != config.RedirectTo {
			// Create a modified config for dockerfile regeneration
			config = RedirectConfig{
				RedirectTo:   redirectTo,
				MaxRedirects: config.MaxRedirects,
				Reason:       "Multiple deployment failures detected - rebuilding container image to fix potential image issues",
			}
		}
	}

	aiPrompt := tr.generateAIFixingPrompt(fromTool, config.RedirectTo, error, sessionID)
	responseText := tr.buildRedirectResponseText(fromTool, error, sessionID, config.RedirectTo, aiPrompt, stepResult...)

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

	// Generate step-specific sections based on the workflow step
	stepSpecificSections := tr.generateStepSpecificSections(stepName, responseData)

	responseText := tr.buildResponseTextWithSections(stepName, currentIndex, stepSpecificSections, sessionID)

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

// buildResponseTextWithSections constructs the appropriate response text with step-specific sections
func (tr *ToolRegistrar) buildResponseTextWithSections(stepName string, currentIndex int, stepSpecificSections []string, sessionID string) string {
	// Handle workflow completion
	if currentIndex == len(WorkflowSequence)-1 {
		return tr.buildCompletedResponseWithSections(stepName, stepSpecificSections)
	}

	// Handle step in progress
	if currentIndex >= 0 && currentIndex < len(WorkflowSequence)-1 {
		return tr.buildProgressResponseWithSections(stepName, currentIndex, stepSpecificSections, sessionID)
	}

	// Handle unknown/invalid step
	return tr.buildFallbackResponseWithSections(stepName, stepSpecificSections, sessionID)
}

// buildCompletedResponseWithSections builds response for completed workflow with step-specific sections
func (tr *ToolRegistrar) buildCompletedResponseWithSections(stepName string, stepSpecificSections []string) string {
	data := WorkflowTemplateData{
		StepName:             stepName,
		TotalSteps:           len(WorkflowSequence),
		StepSpecificSections: stepSpecificSections,
	}
	return tr.executeTemplate(workflowCompletedTemplate, data)
}

// buildFallbackResponseWithSections builds response for unknown steps with step-specific sections
func (tr *ToolRegistrar) buildFallbackResponseWithSections(stepName string, stepSpecificSections []string, sessionID string) string {
	data := WorkflowTemplateData{
		StepName:             stepName,
		SessionID:            sessionID,
		StepSpecificSections: stepSpecificSections,
	}
	return tr.executeTemplate(fallbackTemplate, data)
}

// executeTemplate safely executes a template with given data
func (tr *ToolRegistrar) executeTemplate(templateStr string, data any) string {
	if strings.TrimSpace(templateStr) == "" {
		return "Error: empty template provided"
	}

	if data == nil {
		return "Error: template data cannot be nil"
	}

	tmpl, err := template.New("response").Parse(templateStr)
	if err != nil {
		return fmt.Sprintf("Template parsing error: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Sprintf("Template execution error: %v", err)
	}

	return buf.String()
}

// buildRedirectResponseText constructs redirect response text using templates
func (tr *ToolRegistrar) buildRedirectResponseText(fromTool, error, sessionID, redirectTo string, aiPrompt *AIFixingPrompt, stepResult ...map[string]any) string {
	baseResponse := tr.buildBaseRedirectResponse(fromTool, error, sessionID, redirectTo, aiPrompt)

	// Add step result context if available
	if len(stepResult) > 0 && stepResult[0] != nil {
		contextSection := tr.buildStepResultContext(stepResult[0])
		if contextSection != "" {
			return baseResponse + "\n\n" + contextSection
		}
	}

	return baseResponse
}

// buildBaseRedirectResponse builds the base redirect response without step context
func (tr *ToolRegistrar) buildBaseRedirectResponse(fromTool, error, sessionID, redirectTo string, aiPrompt *AIFixingPrompt) string {
	if aiPrompt != nil {
		// Check if this is a deployment failure redirecting to dockerfile regeneration
		if fromTool == "deploy_application" && redirectTo == "generate_dockerfile" {
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
				FailureCount:   2, // Indicating multiple failures
			}
			return tr.executeTemplate(redirectDockerfileRegenerationTemplate, data)
		}

		// Standard AI redirect template
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

// buildStepResultContext creates a formatted context section from step result data
func (tr *ToolRegistrar) buildStepResultContext(stepResultData map[string]any) string {
	if len(stepResultData) == 0 {
		return ""
	}

	context := []string{"**Step Context Available:**"}

	// Add deployment diagnostics if available (common for deploy failures)
	if diagnostics, exists := stepResultData["deployment_diagnostics"]; exists {
		if diagMap, ok := diagnostics.(map[string]interface{}); ok {
			if logs, exists := diagMap["pod_logs"]; exists && logs != nil {
				context = append(context, fmt.Sprintf("- Pod logs: %v", logs))
			}
			if errors, exists := diagMap["errors"]; exists && errors != nil {
				context = append(context, fmt.Sprintf("- Deployment errors: %v", errors))
			}
			if events, exists := diagMap["recent_events"]; exists && events != nil {
				context = append(context, fmt.Sprintf("- Kubernetes events: %v", events))
			}
		}
	}

	// Add other relevant data
	for key, value := range stepResultData {
		if key == "deployment_diagnostics" {
			continue
		}
		if value != nil && value != "" {
			context = append(context, fmt.Sprintf("- %s: %v", key, value))
		}
	}

	if len(context) > 1 {
		return strings.Join(context, "\n")
	}

	return ""
}

// buildProgressResponseWithSections builds response for workflow in progress with step-specific sections
func (tr *ToolRegistrar) buildProgressResponseWithSections(stepName string, currentIndex int, stepSpecificSections []string, sessionID string) string {
	nextStep := WorkflowSequence[currentIndex+1]

	data := WorkflowTemplateData{
		StepName:             stepName,
		SessionID:            sessionID,
		CurrentStep:          currentIndex + 1,
		TotalSteps:           len(WorkflowSequence),
		StepSpecificSections: stepSpecificSections,
		NextStep:             nextStep,
	}

	return tr.executeTemplate(stepCompletedTemplate, data)
}

// Auto-formatting labels for each step - no templates needed
var stepLabels = map[string]string{
	"analyze_repository":     "Repository Analysis",
	"generate_dockerfile":    "Dockerfile Generated",
	"build_image":            "Build Result",
	"scan_image":             "Security Scan",
	"tag_image":              "Image Tagged",
	"push_image":             "Image Pushed",
	"generate_k8s_manifests": "Manifests Generated",
	"prepare_cluster":        "Cluster Ready",
	"deploy_application":     "Deployment",
	"verify_deployment":      "Verification",
}

// generateStepSpecificSections displays data only if execute methods explicitly return it
func (tr *ToolRegistrar) generateStepSpecificSections(stepName string, responseData map[string]any) []string {
	label, exists := stepLabels[stepName]
	if !exists {
		return nil
	}

	// Extract step data directly from response
	var stepData map[string]any
	if stepResult, ok := responseData["step_result"].(map[string]any); ok {
		if data, ok := stepResult["data"].(map[string]any); ok && len(data) > 0 {
			stepData = data
		}
	}

	if len(stepData) == 0 {
		return nil
	}

	// For Dockerfile generation, render the file content as a fenced code block
	// This is to fix the issue in the LLM Host where '*' would cause formatting issues
	if stepName == "generate_dockerfile" {
		if content, ok := stepData["content"].(string); ok && strings.TrimSpace(content) != "" {
			path := "Dockerfile"
			if p, ok := stepData["path"].(string); ok && p != "" {
				path = p
			}
			// Build a dedicated section with proper code fencing for Dockerfile
			var b strings.Builder
			b.WriteString("\n**" + label + ":**\n")
			b.WriteString("Path: ")
			b.WriteString(path)
			b.WriteString("\n\n")
			b.WriteString("```dockerfile\n")
			b.WriteString(content)
			if !strings.HasSuffix(content, "\n") {
				b.WriteString("\n")
			}
			b.WriteString("```\n")
			return []string{"\n" + b.String() + "\n"}
		}
	}

	// Format the data as a display section
	section := tr.formatStepData(label, stepData)
	if section != "" {
		return []string{"\n" + section + "\n"}
	}

	return nil
}

// formatStepData formats step data into a readable display section
func (tr *ToolRegistrar) formatStepData(label string, data map[string]any) string {
	if len(data) == 0 {
		return ""
	}

	parts := []string{fmt.Sprintf("**%s:**", label)}

	// Format each key-value pair in the data
	for key, value := range data {
		if value == nil {
			continue
		}
		if content := tr.formatValue(key, value); content != "" {
			parts = append(parts, content)
		}
	}

	// Only return if we have actual data to display
	if len(parts) > 1 {
		return strings.Join(parts, " ")
	}

	return ""
}

// formatArray formats small arrays with actual values for better readability
func (tr *ToolRegistrar) formatArray(arr []any) string {
	if len(arr) == 0 {
		return "[]"
	}

	var builder strings.Builder
	builder.WriteByte('[')

	totalLength := 0
	for i, item := range arr {
		if i > 0 {
			builder.WriteString(", ")
			totalLength += 2
		}
		formatted := tr.formatValueOnly(item)
		builder.WriteString(formatted)
		totalLength += len(formatted)
	}

	builder.WriteByte(']')

	// If content is too long, reformat with newlines
	if totalLength > 100 {
		builder.Reset()
		builder.WriteString("[\n")
		for i, item := range arr {
			if i > 0 {
				builder.WriteString(",\n")
			}
			builder.WriteString("  ")
			builder.WriteString(tr.formatValueOnly(item))
		}
		builder.WriteString("\n]")
	}

	return builder.String()
}

// formatValue formats a single value based on its type with improved readability
func (tr *ToolRegistrar) formatValue(key string, value any) string {
	valueStr := tr.formatValueOnly(value)
	if valueStr == "" {
		return ""
	}
	if key == "" {
		return valueStr
	}
	return fmt.Sprintf("%s: %s", key, valueStr)
}

// formatValueOnly formats just the value part without key prefix
func (tr *ToolRegistrar) formatValueOnly(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case bool:
		return fmt.Sprintf("%t", v)
	case float64:
		return fmt.Sprintf("%.0f", v)
	case int:
		return fmt.Sprintf("%d", v)
	case []any:
		if len(v) <= maxArrayDisplayItems {
			return tr.formatArray(v)
		}
		return fmt.Sprintf("[%d items]", len(v))
	case map[string]any:
		if len(v) == 0 {
			return ""
		}
		if len(v) <= maxMapDisplayFields {
			var items []string
			for k, val := range v {
				if formatted := tr.formatValue(k, val); formatted != "" {
					items = append(items, formatted)
				}
			}
			return strings.Join(items, ", ")
		}
		return fmt.Sprintf("{%d fields}", len(v))
	default:
		return fmt.Sprintf("%v", v)
	}
}

// formatErrorForDisplay formats error messages for better readability in tool responses
func (tr *ToolRegistrar) formatErrorForDisplay(error string) string {
	if error == "" {
		return "Unknown error"
	}

	if tr.shouldFormatAsCodeBlock(error) {
		return fmt.Sprintf("```\n%s\n```", error)
	}
	return error
}

// shouldFormatAsCodeBlock determines if error should be formatted as code block
func (tr *ToolRegistrar) shouldFormatAsCodeBlock(error string) bool {
	if error == "" {
		return false
	}
	return len(error) > errorDisplayThreshold || strings.Contains(error, "\n")
}

// determineDeploymentRedirectWithTracking decides where to redirect deploy_application using proper failure tracking
func (tr *ToolRegistrar) determineDeploymentRedirectWithTracking(sessionID, defaultRedirect, error string) (string, error) {
	// Load current failure tracking
	tracking, err := tr.loadDeploymentFailureTracking(sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to load deployment failure tracking: %w", err)
	}

	// Determine redirect target
	redirectTo := defaultRedirect

	// Get current consecutive failures for this redirect path
	currentConsecutiveFailures := tracking.ConsecutiveFailures[defaultRedirect]

	// If we've had 2+ consecutive failures with current redirect approach, try the other approach
	if currentConsecutiveFailures >= 2 {
		if defaultRedirect == "generate_k8s_manifests" {
			redirectTo = "generate_dockerfile"
		} else {
			redirectTo = "generate_k8s_manifests"
		}
		// Reset consecutive failures for the current approach since we're switching
		tracking.ConsecutiveFailures[defaultRedirect] = 0
	}

	// Update tracking for the chosen redirect target
	tracking.ConsecutiveFailures[redirectTo]++
	tracking.LastRedirectTo = redirectTo
	tracking.TotalAttempts++
	tracking.FailureHistory = append(tracking.FailureHistory, DeploymentFailureRecord{
		Timestamp:  time.Now().Format(time.RFC3339),
		RedirectTo: redirectTo,
		Error:      error,
	})

	// Save updated tracking
	if err := tr.saveDeploymentFailureTracking(sessionID, tracking); err != nil {
		tr.logger.Warn("Failed to save deployment failure tracking", "session_id", sessionID, "error", err)
	}

	tr.logger.Info("Deployment failure tracking updated",
		"session_id", sessionID,
		"consecutive_failures_for_redirect", tracking.ConsecutiveFailures[redirectTo],
		"redirect_to", redirectTo,
		"total_attempts", tracking.TotalAttempts)

	return redirectTo, nil
}

// loadDeploymentFailureTracking loads failure tracking from session state
func (tr *ToolRegistrar) loadDeploymentFailureTracking(sessionID string) (*DeploymentFailureTracking, error) {
	// Get session artifacts
	session, err := tr.sessionManager.Get(context.Background(), sessionID)
	if err != nil {
		// If session doesn't exist, return default tracking
		return &DeploymentFailureTracking{
			ConsecutiveFailures: make(map[string]int),
			LastRedirectTo:      "",
			TotalAttempts:       0,
			FailureHistory:      []DeploymentFailureRecord{},
		}, nil
	}

	// Try to get tracking from session metadata
	if session.Metadata != nil {
		if trackingData, exists := session.Metadata["deployment_failure_tracking"]; exists {
			if trackingBytes, ok := trackingData.([]byte); ok {
				var tracking DeploymentFailureTracking
				if err := json.Unmarshal(trackingBytes, &tracking); err == nil {
					// Initialize map if nil
					if tracking.ConsecutiveFailures == nil {
						tracking.ConsecutiveFailures = make(map[string]int)
					}
					return &tracking, nil
				}
			}
			// Try to handle as string (JSON)
			if trackingStr, ok := trackingData.(string); ok {
				var tracking DeploymentFailureTracking
				if err := json.Unmarshal([]byte(trackingStr), &tracking); err == nil {
					// Initialize map if nil
					if tracking.ConsecutiveFailures == nil {
						tracking.ConsecutiveFailures = make(map[string]int)
					}
					return &tracking, nil
				}
			}
		}
	}

	// Return default tracking if not found or parsing failed
	return &DeploymentFailureTracking{
		ConsecutiveFailures: make(map[string]int),
		LastRedirectTo:      "",
		TotalAttempts:       0,
		FailureHistory:      []DeploymentFailureRecord{},
	}, nil
}

// saveDeploymentFailureTracking saves failure tracking to session state
func (tr *ToolRegistrar) saveDeploymentFailureTracking(sessionID string, tracking *DeploymentFailureTracking) error {
	trackingBytes, err := json.Marshal(tracking)
	if err != nil {
		return fmt.Errorf("failed to marshal tracking data: %w", err)
	}

	// Update session with proper update function
	return tr.sessionManager.Update(context.Background(), sessionID, func(sessionState *session.SessionState) error {
		if sessionState.Metadata == nil {
			sessionState.Metadata = make(map[string]interface{})
		}
		sessionState.Metadata["deployment_failure_tracking"] = string(trackingBytes)
		return nil
	})
}

// Legacy method - keeping for backwards compatibility but updating to use new tracking
func (tr *ToolRegistrar) determineDeploymentRedirect(sessionID, defaultRedirect string) string {
	// Use new tracking method with empty error
	redirectTo, err := tr.determineDeploymentRedirectWithTracking(sessionID, defaultRedirect, "")
	if err != nil {
		tr.logger.Warn("Failed to use tracking-based redirect, falling back to default", "error", err)
		return defaultRedirect
	}
	return redirectTo
}

// Legacy method - updated to return actual failure count
func (tr *ToolRegistrar) getDeploymentFailureCount(sessionID string) int {
	tracking, err := tr.loadDeploymentFailureTracking(sessionID)
	if err != nil {
		return 0
	}
	return tracking.TotalAttempts
}
