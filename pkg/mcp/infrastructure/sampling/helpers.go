package sampling

import (
	"context"
	"fmt"
	"strings"
)

// AnalyzeKubernetesManifest uses MCP sampling to analyze and fix Kubernetes manifests
func (c *Client) AnalyzeKubernetesManifest(ctx context.Context, manifestContent string, deploymentError error, dockerfileContent string, repoAnalysis string) (string, error) {
	c.logger.Info("Requesting AI assistance to fix Kubernetes manifest",
		"error_preview", deploymentError.Error()[:min(100, len(deploymentError.Error()))])

	prompt := fmt.Sprintf(`Analyze the following Kubernetes manifest file for errors and suggest fixes:

Current Manifest:
%s

Deployment Error:
%s

Reference Dockerfile for this application:
%s

Repository Analysis:
%s

Please:
1. Identify the issues causing the deployment error
2. Provide a fixed version of the manifest
3. Consider the Dockerfile when fixing the manifest (ports, environment variables, etc.)
4. Ensure health checks are appropriate for the application type
5. Use proper resource limits and requests
6. Verify image references are correct

IMPORTANT:
- Do NOT create brand new manifests - Only fix the provided manifest
- Verify that health check paths exist before using httpGet probe; if they don't, use a tcpSocket probe instead
- Prefer using secrets for sensitive information and configmaps for configuration
- For Spring Boot applications, use /actuator/health only if actuator is configured
- Keep the same app name and container image name

Return ONLY the fixed manifest content without any explanation or markdown formatting.`,
		manifestContent,
		deploymentError.Error(),
		dockerfileContent,
		repoAnalysis)

	request := SamplingRequest{
		Prompt:       prompt,
		MaxTokens:    2048,
		Temperature:  0.2, // Lower temperature for more deterministic fixes
		SystemPrompt: "You are a Kubernetes expert. Fix the manifest to resolve deployment errors while following best practices.",
	}

	response, err := c.Sample(ctx, request)
	if err != nil {
		return "", fmt.Errorf("failed to get AI fix for manifest: %w", err)
	}

	// Clean up the response
	fixedManifest := strings.TrimSpace(response.Content)
	return fixedManifest, nil
}

// AnalyzePodCrashLoop uses MCP sampling to diagnose and suggest fixes for pod crash loops
func (c *Client) AnalyzePodCrashLoop(ctx context.Context, podLogs string, manifestContent string, dockerfileContent string, errorDetails string) (*ErrorAnalysis, error) {
	c.logger.Info("Requesting AI assistance to diagnose pod crash loop")

	prompt := fmt.Sprintf(`Analyze this Kubernetes pod crash loop and suggest fixes:

Pod Logs:
%s

Error Details:
%s

Current Manifest:
%s

Dockerfile:
%s

Please analyze the crash loop and provide:

ROOT CAUSE:
[Identify the specific reason for the crash]

FIX STEPS:
- [Step 1 to fix the issue]
- [Step 2 to fix the issue]
- [Additional steps as needed]

ALTERNATIVES:
- [Alternative approach 1]
- [Alternative approach 2]

PREVENTION:
- [How to prevent this in the future]

Focus on actionable fixes that can be applied to the manifest or Dockerfile.`,
		podLogs,
		errorDetails,
		manifestContent,
		dockerfileContent)

	request := SamplingRequest{
		Prompt:       prompt,
		MaxTokens:    1500,
		Temperature:  0.3,
		SystemPrompt: "You are a Kubernetes debugging expert. Analyze pod crashes and provide actionable solutions.",
	}

	response, err := c.Sample(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze pod crash: %w", err)
	}

	return parseErrorAnalysis(response.Content), nil
}

// AnalyzeSecurityScan uses MCP sampling to analyze security scan results and suggest remediations
func (c *Client) AnalyzeSecurityScan(ctx context.Context, scanResults string, dockerfileContent string, criticalOnly bool) (string, error) {
	c.logger.Info("Requesting AI assistance to analyze security scan results")

	prompt := fmt.Sprintf(`Analyze these container security scan results and suggest remediations:

Security Scan Results:
%s

Current Dockerfile:
%s

Critical Issues Only: %v

Please provide:
1. Summary of the most critical vulnerabilities
2. Specific Dockerfile changes to remediate vulnerabilities
3. Alternative base images if current one has too many vulnerabilities
4. Best practices to improve container security

Focus on practical remediations that can be implemented in the Dockerfile.
If base image has critical vulnerabilities, suggest newer/more secure alternatives.

Return your analysis in a clear, actionable format.`,
		scanResults,
		dockerfileContent,
		criticalOnly)

	request := SamplingRequest{
		Prompt:       prompt,
		MaxTokens:    2000,
		Temperature:  0.3,
		SystemPrompt: "You are a container security expert. Analyze vulnerabilities and provide practical remediation strategies.",
	}

	response, err := c.Sample(ctx, request)
	if err != nil {
		return "", fmt.Errorf("failed to analyze security scan: %w", err)
	}

	return response.Content, nil
}

// ImproveRepositoryAnalysis uses MCP sampling to enhance repository analysis
func (c *Client) ImproveRepositoryAnalysis(ctx context.Context, initialAnalysis string, fileTree string, readmeContent string) (string, error) {
	c.logger.Info("Requesting AI assistance to improve repository analysis")

	prompt := fmt.Sprintf(`Improve this repository analysis by identifying additional details:

Initial Analysis:
%s

Repository Structure:
%s

README Content:
%s

Please enhance the analysis with:
1. More accurate language/framework detection
2. Identification of build tools and package managers
3. Detection of required services (databases, caches, message queues)
4. Application entry points and main executable
5. Environment variables and configuration requirements
6. Suggested port numbers based on framework conventions
7. Special build requirements or dependencies

Return an enhanced analysis that will help with containerization.`,
		initialAnalysis,
		fileTree,
		readmeContent)

	request := SamplingRequest{
		Prompt:       prompt,
		MaxTokens:    1500,
		Temperature:  0.3,
		SystemPrompt: "You are a software architecture expert. Analyze repositories to understand their structure and requirements.",
	}

	response, err := c.Sample(ctx, request)
	if err != nil {
		return "", fmt.Errorf("failed to improve repository analysis: %w", err)
	}

	return response.Content, nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
