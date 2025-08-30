package registrar

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/containerization-assist/pkg/service/tools"
)

// AI prompt templates as constants for better maintainability
const (
	dockerSystemPrompt = `You are an expert Docker containerization specialist. A Dockerfile build has failed and you need to analyze the error and generate a corrected Dockerfile.

Key principles:
- Use multi-stage builds when appropriate
- Optimize for security and size
- Follow Docker best practices
- Use appropriate base images
- Handle dependencies correctly
- Set proper working directories and permissions

Inspect the following:
1. Error from docker build
2. Repository analysis results (language, framework, dependencies)
3. The current Dockerfile content (if available)

Generate a corrected Dockerfile that addresses the specific error while maintaining best practices.`

	k8sSystemPrompt = `You are an expert Kubernetes engineer. A Kubernetes deployment has failed and you need to analyze the error and generate corrected manifests.

Key principles:
- Use proper resource limits and requests
- Set appropriate security contexts
- Configure health checks correctly
- Use proper labeling and selectors
- Handle configuration and secrets properly
- Follow Kubernetes best practices

Inspect the following:
1. The deployment/kubectl error
2. Application analysis (ports, requirements)
3. Current manifest content (if available)
4. Container image information

Generate corrected Kubernetes manifests that address the specific deployment issue.`

	securitySystemPrompt = `You are a container security expert. A security scan has found vulnerabilities and you need to provide guidance for creating a more secure container image.

Security best practices:
- Use minimal base images
- Update packages to latest secure versions
- Remove unnecessary packages and files
- Use non-root users
- Set proper file permissions
- Avoid embedding secrets`
)

// generateAIFixingPrompt creates AI prompts for fixing specific tool failures
func (tr *ToolRegistrar) generateAIFixingPrompt(failedTool, fixingTool, error, sessionID string) *AIFixingPrompt {
	switch failedTool {
	case "scan_image":
		return tr.generateSecurityFix(error, sessionID)
	case "deploy_application":
		return tr.generateManifestFix(error, sessionID)
	default:
		return tr.generateGenericFix(failedTool, fixingTool, error, sessionID)
	}
}

// generateManifestFix creates AI prompt for fixing Kubernetes manifest issues
func (tr *ToolRegistrar) generateManifestFix(error, sessionID string) *AIFixingPrompt {
	userPrompt := tr.buildK8sUserPrompt(error, sessionID)

	return &AIFixingPrompt{
		SystemPrompt:   k8sSystemPrompt,
		UserPrompt:     userPrompt,
		Context:        map[string]interface{}{"session_id": sessionID}, // Minimal context
		ExpectedOutput: "Correct Kubernetes manifests (Deployment, Service, etc.) that fix the deployment issue",
		FixingStrategy: "manifest_regeneration",
	}
}

// generateSecurityFix creates AI prompt for fixing security scan issues
func (tr *ToolRegistrar) generateSecurityFix(error, sessionID string) *AIFixingPrompt {
	userPrompt := tr.buildSecurityUserPrompt(error, sessionID)

	return &AIFixingPrompt{
		SystemPrompt:   securitySystemPrompt,
		UserPrompt:     userPrompt,
		Context:        map[string]interface{}{"session_id": sessionID}, // Minimal context
		ExpectedOutput: "Updated Dockerfile with security improvements",
		FixingStrategy: "dockerfile_regeneration_with_security_hardening",
	}
}

// generateGenericFix creates a generic AI prompt for any tool failure
func (tr *ToolRegistrar) generateGenericFix(failedTool, fixingTool, error, sessionID string) *AIFixingPrompt {
	systemPrompt := fmt.Sprintf(`You are a containerization expert helping to fix issues in a workflow. The %s step failed and needs to be addressed by regenerating/fixing the %s step.

Analyze the error and provide specific guidance for fixing the issue.`, failedTool, fixingTool)

	userPrompt := fmt.Sprintf(`The %s step failed with error:
%s

Session: %s
Context: %s

Instructions:
1. Read current files to understand the configuration
2. Analyze the error and provide specific steps to fix this issue`,
		failedTool,
		error,
		sessionID,
		"Use MCP tools to read current files and session info")

	return &AIFixingPrompt{
		SystemPrompt:   systemPrompt,
		UserPrompt:     userPrompt,
		Context:        map[string]interface{}{"session_id": sessionID},
		ExpectedOutput: fmt.Sprintf("Instructions for fixing the %s failure", failedTool),
		FixingStrategy: "generic_fix",
	}
}

// getContext loads and formats specific context data on demand
func (tr *ToolRegistrar) getContext(sessionID, artifactKey string) string {
	ctx := context.Background()
	state, err := tools.LoadWorkflowState(ctx, tr.sessionManager, sessionID)
	if err != nil {
		return "Context not available"
	}

	// Direct artifact access based on artifact key
	var data interface{}
	switch artifactKey {
	case "analyze_result":
		data = state.Artifacts.AnalyzeResult
	case "dockerfile_result":
		data = state.Artifacts.DockerfileResult
	case "build_result":
		data = state.Artifacts.BuildResult
	case "k8s_result":
		data = state.Artifacts.K8sResult
	case "scan_result":
		data = state.Artifacts.ScanResult
	}

	if data != nil {
		if jsonData, err := json.MarshalIndent(data, "", "  "); err == nil {
			return string(jsonData)
		}
	}

	return "Data not available"
}

// getK8sDeploymentDiagnostics extracts deployment diagnostics from K8sResult metadata
func (tr *ToolRegistrar) getK8sDeploymentDiagnostics(sessionID string) string {
	ctx := context.Background()
	state, err := tools.LoadWorkflowState(ctx, tr.sessionManager, sessionID)
	if err != nil {
		return "Deployment diagnostics not available"
	}

	// Access K8sResult from typed artifacts
	if state.Artifacts == nil || state.Artifacts.K8sResult == nil {
		return "No deployment diagnostics available"
	}

	k8sResult := state.Artifacts.K8sResult
	if k8sResult.Metadata == nil {
		return "No deployment diagnostics available"
	}

	diagnostics, exists := k8sResult.Metadata["deployment_diagnostics"]
	if !exists {
		return "No deployment diagnostics available"
	}

	jsonData, err := json.MarshalIndent(diagnostics, "", "  ")
	if err != nil {
		return "No deployment diagnostics available"
	}

	return string(jsonData)
}

// Helper functions for building user prompts

// buildDockerUserPrompt creates user prompt for Docker build failures
func (tr *ToolRegistrar) buildDockerUserPrompt(error, sessionID string) string {
	return fmt.Sprintf(`Docker build failed with this error:
%s

Please analyze this error and generate a corrected Dockerfile. Focus on:
1. Read the current Dockerfile to understand the existing configuration
2. Fix the specific issue that caused the build failure
3. Maintain compatibility with the detected language/framework
4. Follow Docker best practices
5. Optimize for the application type

Repository context:
%s

Instructions:
1. First, read the current Dockerfile from the repository
2. Analyze the build error against the current Dockerfile
3. Generate a complete, working Dockerfile that fixes this issue`,
		error, tr.getContext(sessionID, "analyze_result"))
}

// buildK8sUserPrompt creates user prompt for Kubernetes deployment failures
func (tr *ToolRegistrar) buildK8sUserPrompt(error, sessionID string) string {
	// Get deployment diagnostics from K8sResult metadata
	deploymentDiagnostics := tr.getK8sDeploymentDiagnostics(sessionID)

	return fmt.Sprintf(`Kubernetes deployment failed with this error:
%s

Please analyze this error and generate corrected Kubernetes manifests. Focus on:
1. Read the current Kubernetes manifests to understand existing configuration
2. Fix the specific deployment issue
3. Ensure proper resource configuration
4. Set up health checks if needed
5. Follow Kubernetes best practices

Application context:
%s

Container image info:
%s

Deployment diagnostics:
%s

Instructions:
1. First, read the current Kubernetes manifests from the repository
2. Analyze the deployment error against the current manifests
3. Generate complete, working Kubernetes manifests that fix this deployment issue`,
		error,
		tr.getContext(sessionID, "analyze_result"),
		tr.getContext(sessionID, "build_result"),
		deploymentDiagnostics,
	)
}

// buildSecurityUserPrompt creates user prompt for security scan failures
func (tr *ToolRegistrar) buildSecurityUserPrompt(error, sessionID string) string {
	return fmt.Sprintf(`Security scan failed or found issues:
%s

Current build context:
%s
%s

Instructions:
1. Read the current Dockerfile to understand security issues
2. Provide specific recommendations to address these security issues
3. Regenerate a more secure image with hardened configuration`,
		error, tr.getContext(sessionID, "analyze_result"), tr.getContext(sessionID, "build_result"))
}
