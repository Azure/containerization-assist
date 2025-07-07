package conversation

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

type BuildErrorRecoveryPrompt struct {
	Dockerfile         string
	BuildErrors        string
	RepositoryContext  *RepositoryContext
	PreviousAttempts   []AttemptSummary
	AvailableTools     []string
	FileInvestigations []FileInvestigation
}
type RepositoryContext struct {
	WorkspaceDir        string
	ProjectType         string
	Language            string
	Dependencies        map[string]string
	BuildCommands       []string
	RuntimeRequirements []string
	DetectedIssues      []string
}
type AttemptSummary struct {
	AttemptNumber int
	ErrorType     string
	Fix           string
	Result        string
}
type FileInvestigation struct {
	FilePath string
	Purpose  string
	Finding  string
}
type ErrorRecoveryPromptBuilder struct {
	logger *slog.Logger
}

func NewErrorRecoveryPromptBuilder(logger *slog.Logger) *ErrorRecoveryPromptBuilder {
	return &ErrorRecoveryPromptBuilder{
		logger: logger,
	}
}
func (b *ErrorRecoveryPromptBuilder) BuildDockerErrorRecoveryPrompt(ctx context.Context, prompt BuildErrorRecoveryPrompt) string {
	var sb strings.Builder
	sb.WriteString("🔧 **DOCKER BUILD ERROR RECOVERY**\n\n")
	sb.WriteString("The Docker build has failed. You need to analyze the error, explore the repository for more context, and provide a fixed Dockerfile.\n\n")
	sb.WriteString("**CRITICAL**: Use the file access tools to investigate the repository and understand the root cause of the build failure.\n\n")
	sb.WriteString("## Current Dockerfile\n\n")
	sb.WriteString("```dockerfile\n")
	sb.WriteString(prompt.Dockerfile)
	sb.WriteString("\n```\n\n")
	sb.WriteString("## Build Error\n\n")
	sb.WriteString("```\n")
	sb.WriteString(prompt.BuildErrors)
	sb.WriteString("\n```\n\n")
	sb.WriteString("## Available Tools & Investigation Strategy\n\n")
	for _, tool := range prompt.AvailableTools {
		sb.WriteString(fmt.Sprintf("- `%s`", tool))

		switch tool {
		case "list_directory":
			sb.WriteString(" - Use to verify paths mentioned in COPY/ADD commands exist")
		case "read_file":
			sb.WriteString(" - Use to examine configuration files, build scripts, dependency files")
		case "scan_repository":
			sb.WriteString(" - Use to understand overall project structure and identify missing files")
		case "analyze_project":
			sb.WriteString(" - Use to get project type, language, and build requirements")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n")
	sb.WriteString("## Step-by-Step Investigation Process\n\n")
	b.addErrorSpecificGuidance(&sb, prompt.BuildErrors)

	sb.WriteString("### General Investigation Steps:\n\n")
	sb.WriteString("1. **Parse the Error Message** 📋\n")
	sb.WriteString("   - Extract the specific command that failed\n")
	sb.WriteString("   - Identify file paths mentioned in the error\n")
	sb.WriteString("   - Note the exit code if provided\n")
	sb.WriteString("   - Look for hints about what was expected vs. found\n\n")

	sb.WriteString("2. **Verify Repository Structure** 🔍\n")
	sb.WriteString("   - Use `scan_repository` or `list_directory` to understand the project layout\n")
	sb.WriteString("   - Check if all paths mentioned in COPY/ADD commands actually exist\n")
	sb.WriteString("   - Identify the main entry point (package.json, requirements.txt, go.mod, etc.)\n")
	sb.WriteString("   - ✅ **Success indicator**: You understand what files exist and where they are\n\n")

	sb.WriteString("3. **Analyze Project Dependencies** 📦\n")
	sb.WriteString("   - Read dependency files (package.json, requirements.txt, go.mod, pom.xml, etc.)\n")
	sb.WriteString("   - Check for build scripts or configuration files\n")
	sb.WriteString("   - Look for any special build requirements in README files\n")
	sb.WriteString("   - ✅ **Success indicator**: You know what the project needs to build and run\n\n")

	sb.WriteString("4. **Map Error to Root Cause** 🎯\n")
	sb.WriteString("   - Compare the error with what you learned about the project\n")
	sb.WriteString("   - Identify the disconnect between Dockerfile assumptions and reality\n")
	sb.WriteString("   - Consider the build context and working directory\n")
	sb.WriteString("   - ✅ **Success indicator**: You can explain exactly why the build failed\n\n")

	sb.WriteString("5. **Design and Validate Fix** 🔧\n")
	sb.WriteString("   - Plan your changes to address the root cause\n")
	sb.WriteString("   - Ensure your fix aligns with the project's actual structure\n")
	sb.WriteString("   - Double-check that all required files will be available at build time\n")
	sb.WriteString("   - ✅ **Success indicator**: Your fix addresses the specific error and project needs\n\n")
	if len(prompt.PreviousAttempts) > 0 {
		sb.WriteString("## Previous Recovery Attempts\n\n")
		for _, attempt := range prompt.PreviousAttempts {
			sb.WriteString(fmt.Sprintf("### Attempt %d\n", attempt.AttemptNumber))
			sb.WriteString(fmt.Sprintf("- **Error Type**: %s\n", attempt.ErrorType))
			sb.WriteString(fmt.Sprintf("- **Fix Applied**: %s\n", attempt.Fix))
			sb.WriteString(fmt.Sprintf("- **Result**: %s\n", attempt.Result))
			sb.WriteString("\n")
		}
		sb.WriteString("**Learn from these attempts** - don't repeat the same fixes that didn't work.\n\n")
	}
	if prompt.RepositoryContext != nil {
		b.addRepositoryContext(&sb, prompt.RepositoryContext)
	}
	if len(prompt.FileInvestigations) > 0 {
		sb.WriteString("## Previous File Investigations\n\n")
		for _, inv := range prompt.FileInvestigations {
			sb.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", inv.FilePath, inv.Purpose, inv.Finding))
		}
		sb.WriteString("\n")
	}
	b.addCommonErrorPatterns(&sb)
	sb.WriteString("## Required Action\n\n")
	sb.WriteString("Based on your analysis and repository exploration, provide a fixed Dockerfile that addresses the build error.\n\n")
	sb.WriteString("**Format your response as**:\n\n")
	sb.WriteString("1. **Error Analysis**: Brief explanation of what went wrong\n")
	sb.WriteString("2. **Files Investigated**: List files you examined and key findings\n")
	sb.WriteString("3. **Solution**: Explanation of how your fix addresses the issue\n")
	sb.WriteString("4. **Fixed Dockerfile**: Complete corrected Dockerfile between `<DOCKERFILE>` and `</DOCKERFILE>` tags\n\n")

	sb.WriteString("Remember: The goal is a working Docker image. Be thorough in your investigation and precise in your fix.\n")

	return sb.String()
}
func (b *ErrorRecoveryPromptBuilder) addErrorSpecificGuidance(sb *strings.Builder, buildErrors string) {
	errorLower := strings.ToLower(buildErrors)
	sb.WriteString("### 🎯 Targeted Guidance for This Error:\n\n")

	if strings.Contains(errorLower, "copy failed") || strings.Contains(errorLower, "no such file or directory") {
		sb.WriteString("**COPY/File Not Found Error Detected** 📁\n")
		sb.WriteString("- **Priority Action**: Use `list_directory` to check the exact structure\n")
		sb.WriteString("- **Key Investigation**: Verify each path in COPY commands actually exists\n")
		sb.WriteString("- **Common Cause**: Dockerfile assumes files are in different locations than reality\n")
		sb.WriteString("- **Tool Focus**: Start with `scan_repository` to understand the project layout\n\n")
	} else if strings.Contains(errorLower, "returned a non-zero code") || strings.Contains(errorLower, "command failed") {
		sb.WriteString("**Command Execution Error Detected** ⚡\n")
		sb.WriteString("- **Priority Action**: Identify which command failed and why\n")
		sb.WriteString("- **Key Investigation**: Check if the command is appropriate for the base image\n")
		sb.WriteString("- **Common Cause**: Missing dependencies, wrong package manager, or incorrect syntax\n")
		sb.WriteString("- **Tool Focus**: Use `read_file` to check dependency/config files for proper commands\n\n")
	} else if strings.Contains(errorLower, "permission denied") {
		sb.WriteString("**Permission Error Detected** 🔒\n")
		sb.WriteString("- **Priority Action**: Check file permissions and user context\n")
		sb.WriteString("- **Key Investigation**: Determine if files need specific permissions or ownership\n")
		sb.WriteString("- **Common Cause**: Restrictive file permissions or running as wrong user\n")
		sb.WriteString("- **Tool Focus**: Look for executable files that might need chmod commands\n\n")
	} else if strings.Contains(errorLower, "package not found") || strings.Contains(errorLower, "module not found") {
		sb.WriteString("**Dependency/Package Error Detected** 📦\n")
		sb.WriteString("- **Priority Action**: Examine dependency files to understand requirements\n")
		sb.WriteString("- **Key Investigation**: Check package names, versions, and installation commands\n")
		sb.WriteString("- **Common Cause**: Dependency files not copied before installation, or wrong package names\n")
		sb.WriteString("- **Tool Focus**: Use `read_file` on package.json, requirements.txt, go.mod, etc.\n\n")
	} else if strings.Contains(errorLower, "network") || strings.Contains(errorLower, "download") {
		sb.WriteString("**Network/Download Error Detected** 🌐\n")
		sb.WriteString("- **Priority Action**: Check if dependencies can be installed offline or cached\n")
		sb.WriteString("- **Key Investigation**: Look for proxy settings or alternative package sources\n")
		sb.WriteString("- **Common Cause**: Network restrictions or package source issues\n")
		sb.WriteString("- **Tool Focus**: Check for package-lock files or alternative installation methods\n\n")
	} else {
		sb.WriteString("**General Build Error** 🔍\n")
		sb.WriteString("- **Priority Action**: Carefully parse the error message for specific clues\n")
		sb.WriteString("- **Key Investigation**: Start with repository structure and project dependencies\n")
		sb.WriteString("- **Tool Focus**: Use `scan_repository` first, then drill down with specific file reads\n\n")
	}
}
func (b *ErrorRecoveryPromptBuilder) addRepositoryContext(sb *strings.Builder, ctx *RepositoryContext) {
	sb.WriteString("## Repository Context\n\n")

	sb.WriteString(fmt.Sprintf("- **Workspace**: %s\n", ctx.WorkspaceDir))
	if ctx.ProjectType != "" {
		sb.WriteString(fmt.Sprintf("- **Project Type**: %s\n", ctx.ProjectType))
	}
	if ctx.Language != "" {
		sb.WriteString(fmt.Sprintf("- **Language**: %s\n", ctx.Language))
	}

	if len(ctx.BuildCommands) > 0 {
		sb.WriteString("- **Known Build Commands**:\n")
		for _, cmd := range ctx.BuildCommands {
			sb.WriteString(fmt.Sprintf("  - %s\n", cmd))
		}
	}

	if len(ctx.RuntimeRequirements) > 0 {
		sb.WriteString("- **Runtime Requirements**:\n")
		for _, req := range ctx.RuntimeRequirements {
			sb.WriteString(fmt.Sprintf("  - %s\n", req))
		}
	}

	if len(ctx.DetectedIssues) > 0 {
		sb.WriteString("- **Detected Issues**:\n")
		for _, issue := range ctx.DetectedIssues {
			sb.WriteString(fmt.Sprintf("  - %s\n", issue))
		}
	}

	sb.WriteString("\n")
}
func (b *ErrorRecoveryPromptBuilder) addCommonErrorPatterns(sb *strings.Builder) {
	sb.WriteString("## Common Error Patterns\n\n")

	sb.WriteString("### File Not Found Errors\n")
	sb.WriteString("- **Pattern**: `COPY failed: stat /var/lib/docker/tmp/...`\n")
	sb.WriteString("- **Investigation**: Use `list_directory` to verify source paths exist\n")
	sb.WriteString("- **Common Fixes**: Adjust COPY paths, ensure files aren't in .dockerignore\n\n")

	sb.WriteString("### Command Failed Errors\n")
	sb.WriteString("- **Pattern**: `The command '/bin/sh -c ...' returned a non-zero code`\n")
	sb.WriteString("- **Investigation**: Check if commands are valid for the base image\n")
	sb.WriteString("- **Common Fixes**: Install missing tools, use correct package manager\n\n")

	sb.WriteString("### Permission Errors\n")
	sb.WriteString("- **Pattern**: `Permission denied`\n")
	sb.WriteString("- **Investigation**: Check file permissions and ownership requirements\n")
	sb.WriteString("- **Common Fixes**: Add chmod/chown commands, run as appropriate user\n\n")

	sb.WriteString("### Missing Dependencies\n")
	sb.WriteString("- **Pattern**: `Package not found`, `Module not found`\n")
	sb.WriteString("- **Investigation**: Read dependency files to see full requirements\n")
	sb.WriteString("- **Common Fixes**: Install system packages, copy dependency files before install\n\n")
}
func (b *ErrorRecoveryPromptBuilder) BuildDeploymentErrorRecoveryPrompt(ctx context.Context, deploymentError string, manifestContent string, dockerfileContent string) string {
	var sb strings.Builder

	sb.WriteString("🚀 **DEPLOYMENT ERROR RECOVERY**\n\n")
	sb.WriteString("The Kubernetes deployment has failed. This might indicate issues with the Docker image or the deployment manifest.\n\n")

	sb.WriteString("## Deployment Error\n\n")
	sb.WriteString("```\n")
	sb.WriteString(deploymentError)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Current Manifest\n\n")
	sb.WriteString("```yaml\n")
	sb.WriteString(manifestContent)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Current Dockerfile\n\n")
	sb.WriteString("```dockerfile\n")
	sb.WriteString(dockerfileContent)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Investigation Steps\n\n")
	sb.WriteString("1. **Analyze deployment error**: Is it an image pull error, crash loop, or configuration issue?\n")
	sb.WriteString("2. **Check container requirements**: Does the app need specific ports, volumes, or environment variables?\n")
	sb.WriteString("3. **Verify runtime behavior**: Does the container start correctly? Does it stay running?\n")
	sb.WriteString("4. **Examine startup sequence**: Are there initialization steps that might be failing?\n\n")

	sb.WriteString("## Common Deployment Issues\n\n")
	sb.WriteString("- **CrashLoopBackOff**: Application exits immediately - check CMD/ENTRYPOINT\n")
	sb.WriteString("- **ImagePullBackOff**: Can't pull image - check image name and registry\n")
	sb.WriteString("- **Port issues**: Application not listening on expected port\n")
	sb.WriteString("- **Missing environment**: Required environment variables not set\n")
	sb.WriteString("- **Health check failures**: Readiness/liveness probes failing\n\n")

	sb.WriteString("## Required Actions\n\n")
	sb.WriteString("Provide fixes for either or both:\n")
	sb.WriteString("1. **Updated Dockerfile**: If the issue is with the container image\n")
	sb.WriteString("2. **Updated Manifest**: If the issue is with Kubernetes configuration\n\n")
	sb.WriteString("Format your response with clear sections and use appropriate tags for the fixed files.\n")

	return sb.String()
}
func (b *ErrorRecoveryPromptBuilder) BuildGeneralErrorRecoveryPrompt(ctx context.Context, operation string, errorDetails string, context string) string {
	var sb strings.Builder

	sb.WriteString("⚠️ **ERROR RECOVERY ASSISTANCE**\n\n")
	sb.WriteString(fmt.Sprintf("An error occurred during: **%s**\n\n", operation))

	sb.WriteString("## Error Details\n\n")
	sb.WriteString("```\n")
	sb.WriteString(errorDetails)
	sb.WriteString("\n```\n\n")

	if context != "" {
		sb.WriteString("## Context\n\n")
		sb.WriteString(context)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Investigation Approach\n\n")
	sb.WriteString("1. **Understand the error**: What exactly failed and why?\n")
	sb.WriteString("2. **Explore related files**: Use file tools to investigate\n")
	sb.WriteString("3. **Identify root cause**: Don't just fix symptoms\n")
	sb.WriteString("4. **Propose solution**: Provide a comprehensive fix\n\n")

	sb.WriteString("## Your Task\n\n")
	sb.WriteString("1. Analyze the error and its context\n")
	sb.WriteString("2. Use available tools to investigate the issue\n")
	sb.WriteString("3. Provide a clear explanation of the problem\n")
	sb.WriteString("4. Suggest a solution with specific steps or fixed content\n\n")

	return sb.String()
}
