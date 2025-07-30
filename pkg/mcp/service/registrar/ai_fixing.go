package registrar

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/container-kit/pkg/mcp/service/tools"
)

// generateAIFixingPrompt creates AI prompts for fixing specific tool failures
func (tr *ToolRegistrar) generateAIFixingPrompt(failedTool, fixingTool, error, sessionID string) *AIFixingPrompt {
	switch failedTool {
	case "build_image":
		return tr.generateDockerfileFix(error, sessionID)
	case "deploy_application":
		return tr.generateManifestFix(error, sessionID) 
	case "push_image":
		return tr.generateImageBuildFix(error, sessionID)
	case "scan_image":
		return tr.generateSecurityFix(error, sessionID)
	default:
		return tr.generateGenericFix(failedTool, fixingTool, error, sessionID)
	}
}

// generateDockerfileFix creates AI prompt for fixing Dockerfile issues
func (tr *ToolRegistrar) generateDockerfileFix(error, sessionID string) *AIFixingPrompt {
	// Get existing analysis and dockerfile from session
	context := tr.getSessionContext(sessionID)
	
	systemPrompt := `You are an expert Docker containerization specialist. A Dockerfile build has failed and you need to analyze the error and generate a corrected Dockerfile.

Key principles:
- Use multi-stage builds when appropriate
- Optimize for security and size
- Follow Docker best practices
- Use appropriate base images
- Handle dependencies correctly
- Set proper working directories and permissions

You will receive:
1. The original error from docker build
2. Repository analysis results (language, framework, dependencies)
3. The current Dockerfile content (if available)

Generate a corrected Dockerfile that addresses the specific error while maintaining best practices.`

	userPrompt := fmt.Sprintf(`Docker build failed with this error:
%s

Please analyze this error and generate a corrected Dockerfile. Focus on:
1. Fixing the specific issue that caused the build failure
2. Maintaining compatibility with the detected language/framework
3. Following Docker best practices
4. Optimizing for the application type

Repository context:
%s

Current Dockerfile (if available):
%s

Generate a complete, working Dockerfile that fixes this issue.`, 
		error, 
		tr.formatContextForAI(context, "analysis"),
		tr.formatContextForAI(context, "dockerfile"))

	return &AIFixingPrompt{
		SystemPrompt:   systemPrompt,
		UserPrompt:     userPrompt,
		Context:        context,
		ExpectedOutput: "A complete Dockerfile that addresses the build failure",
		FixingStrategy: "dockerfile_regeneration",
	}
}

// generateManifestFix creates AI prompt for fixing Kubernetes manifest issues
func (tr *ToolRegistrar) generateManifestFix(error, sessionID string) *AIFixingPrompt {
	context := tr.getSessionContext(sessionID)
	
	systemPrompt := `You are an expert Kubernetes engineer. A Kubernetes deployment has failed and you need to analyze the error and generate corrected manifests.

Key principles:
- Use proper resource limits and requests
- Set appropriate security contexts
- Configure health checks correctly
- Use proper labeling and selectors
- Handle configuration and secrets properly
- Follow Kubernetes best practices

You will receive:
1. The deployment/kubectl error
2. Application analysis (ports, requirements)
3. Current manifest content (if available)
4. Container image information

Generate corrected Kubernetes manifests that address the specific deployment issue.`

	userPrompt := fmt.Sprintf(`Kubernetes deployment failed with this error:
%s

Please analyze this error and generate corrected Kubernetes manifests. Focus on:
1. Fixing the specific deployment issue
2. Ensuring proper resource configuration
3. Setting up health checks if needed
4. Following Kubernetes best practices

Application context:
%s

Current manifests (if available):
%s

Container image info:
%s

Generate complete, working Kubernetes manifests that fix this deployment issue.`,
		error,
		tr.formatContextForAI(context, "analysis"),
		tr.formatContextForAI(context, "k8s_manifests"),
		tr.formatContextForAI(context, "build_result"))

	return &AIFixingPrompt{
		SystemPrompt:   systemPrompt,
		UserPrompt:     userPrompt,
		Context:        context,
		ExpectedOutput: "Complete Kubernetes manifests (Deployment, Service, etc.) that fix the deployment issue",
		FixingStrategy: "manifest_regeneration",
	}
}

// generateImageBuildFix creates AI prompt for fixing image build issues
func (tr *ToolRegistrar) generateImageBuildFix(error, sessionID string) *AIFixingPrompt {
	context := tr.getSessionContext(sessionID)
	
	systemPrompt := `You are a Docker expert specializing in container image optimization and troubleshooting. An image build or push operation has failed and you need to provide guidance for fixing it.

Focus areas:
- Image size optimization
- Layer caching strategies  
- Registry authentication issues
- Network connectivity problems
- Build context optimization
- Multi-platform builds if needed`

	userPrompt := fmt.Sprintf(`Image build/push failed with error:
%s

Context:
%s

Provide specific steps to fix this issue and regenerate the image successfully.`,
		error,
		tr.formatContextForAI(context, "all"))

	return &AIFixingPrompt{
		SystemPrompt:   systemPrompt,
		UserPrompt:     userPrompt,
		Context:        context,
		ExpectedOutput: "Specific instructions for fixing the image build issue",
		FixingStrategy: "image_rebuild",
	}
}

// generateSecurityFix creates AI prompt for fixing security scan issues
func (tr *ToolRegistrar) generateSecurityFix(error, sessionID string) *AIFixingPrompt {
	context := tr.getSessionContext(sessionID)
	
	systemPrompt := `You are a container security expert. A security scan has found vulnerabilities and you need to provide guidance for creating a more secure container image.

Security best practices:
- Use minimal base images
- Update packages to latest secure versions
- Remove unnecessary packages and files
- Use non-root users
- Set proper file permissions
- Avoid embedding secrets`

	userPrompt := fmt.Sprintf(`Security scan failed or found issues:
%s

Current build context:
%s

Provide specific recommendations to address these security issues and regenerate a more secure image.`,
		error,
		tr.formatContextForAI(context, "all"))

	return &AIFixingPrompt{
		SystemPrompt:   systemPrompt,
		UserPrompt:     userPrompt,
		Context:        context,
		ExpectedOutput: "Updated Dockerfile with security improvements",
		FixingStrategy: "security_hardening",
	}
}

// generateGenericFix creates a generic AI prompt for any tool failure
func (tr *ToolRegistrar) generateGenericFix(failedTool, fixingTool, error, sessionID string) *AIFixingPrompt {
	context := tr.getSessionContext(sessionID)
	
	systemPrompt := fmt.Sprintf(`You are a containerization expert helping to fix issues in a workflow. The %s step failed and needs to be addressed by regenerating/fixing the %s step.

Analyze the error and provide specific guidance for fixing the issue.`, failedTool, fixingTool)

	userPrompt := fmt.Sprintf(`The %s step failed with error:
%s

Context:
%s

Please provide specific steps to fix this issue.`,
		failedTool,
		error,
		tr.formatContextForAI(context, "all"))

	return &AIFixingPrompt{
		SystemPrompt:   systemPrompt,
		UserPrompt:     userPrompt,
		Context:        context,
		ExpectedOutput: fmt.Sprintf("Instructions for fixing the %s failure", failedTool),
		FixingStrategy: "generic_fix",
	}
}

// getSessionContext retrieves session context for AI prompts
func (tr *ToolRegistrar) getSessionContext(sessionID string) map[string]interface{} {
	// Load session state to get context
	ctx := context.Background()
	simpleState, err := tools.LoadWorkflowState(ctx, tr.sessionManager, sessionID)
	if err != nil {
		tr.logger.Warn("Failed to load session context for AI prompt", "session_id", sessionID, "error", err)
		return map[string]interface{}{
			"error": "Failed to load session context",
		}
	}

	context := map[string]interface{}{
		"session_id":       sessionID,
		"repo_path":        simpleState.RepoPath,
		"completed_steps":  simpleState.CompletedSteps,
		"current_step":     simpleState.CurrentStep,
		"status":           simpleState.Status,
	}

	// Add artifacts as context
	for key, value := range simpleState.Artifacts {
		context[key] = value
	}

	return context
}

// formatContextForAI formats specific context data for AI prompts
func (tr *ToolRegistrar) formatContextForAI(context map[string]interface{}, contextType string) string {
	switch contextType {
	case "analysis":
		if analyzeData, exists := context["analyze_result"]; exists {
			if data, _ := json.MarshalIndent(analyzeData, "", "  "); data != nil {
				return fmt.Sprintf("Repository Analysis:\n%s", string(data))
			}
		}
		return "Repository analysis not available"
		
	case "dockerfile":
		if dockerfileData, exists := context["dockerfile_result"]; exists {
			if dockerfileMap, ok := dockerfileData.(map[string]interface{}); ok {
				if content, ok := dockerfileMap["content"].(string); ok {
					return fmt.Sprintf("Current Dockerfile:\n%s", content)
				}
			}
		}
		return "Dockerfile content not available"
		
	case "k8s_manifests":
		if k8sData, exists := context["k8s_result"]; exists {
			if data, _ := json.MarshalIndent(k8sData, "", "  "); data != nil {
				return fmt.Sprintf("Current K8s Manifests:\n%s", string(data))
			}
		}
		return "Kubernetes manifests not available"
		
	case "build_result":
		if buildData, exists := context["build_result"]; exists {
			if data, _ := json.MarshalIndent(buildData, "", "  "); data != nil {
				return fmt.Sprintf("Build Information:\n%s", string(data))
			}
		}
		return "Build information not available"
		
	case "all":
		if data, _ := json.MarshalIndent(context, "", "  "); data != nil {
			return string(data)
		}
		return "Context not available"
		
	default:
		return "Unknown context type"
	}
}