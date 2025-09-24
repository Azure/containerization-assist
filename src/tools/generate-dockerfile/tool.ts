import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import { promptTemplates, type DockerfilePromptParams } from '@/prompts/templates';
import { buildMessages, toMCPMessages } from '@/ai/prompt-engine';
import { generateDockerfileSchema, type GenerateDockerfileParams } from './schema';
import type { AIResponse } from '../ai-response-types';
import { getSession, updateSession } from '@/mcp/tool-session-helpers';
import { promises as fs } from 'node:fs';
import nodePath from 'node:path';
import { DockerfileParser } from 'dockerfile-ast';

export async function generateDockerfile(
  params: GenerateDockerfileParams,
  context: ToolContext,
): Promise<Result<AIResponse>> {
  const validatedParams = generateDockerfileSchema.parse(params);
  const { baseImage, multistage, securityHardening, optimization, sessionId, path } =
    validatedParams;

  // Retrieve repository analysis from session if sessionId is provided
  let language = 'auto-detect';
  let framework = null;
  let dependencies: string[] = [];
  let ports = [8080];
  let promptPrefix = '';

  if (sessionId) {
    const sessionResult = await getSession(sessionId, context);
    if (sessionResult.ok && sessionResult.value.state.metadata?.repositoryAnalysis) {
      const analysis = sessionResult.value.state.metadata.repositoryAnalysis as Record<
        string,
        unknown
      >;

      // Use actual values from the analysis
      language = typeof analysis.language === 'string' ? analysis.language : 'auto-detect';
      framework = typeof analysis.framework === 'string' ? analysis.framework : null;
      dependencies = Array.isArray(analysis.dependencies)
        ? (analysis.dependencies as string[])
        : [];
      ports = Array.isArray(analysis.suggestedPorts)
        ? (analysis.suggestedPorts as number[])
        : [8080];

      // Create a detailed prompt with the actual analysis data
      promptPrefix = `Repository Analysis Results:
`;
      promptPrefix += `- Language: ${language} ${typeof analysis.languageVersion === 'string' ? `(${analysis.languageVersion})` : ''}\n`;
      if (framework) {
        promptPrefix += `- Framework: ${framework} ${typeof analysis.frameworkVersion === 'string' ? `(${analysis.frameworkVersion})` : ''}\n`;
      }
      const buildSystem = analysis.buildSystem as Record<string, unknown> | undefined;
      promptPrefix += `- Build System: ${buildSystem?.type || 'unknown'}\n`;
      if (dependencies.length > 0) {
        promptPrefix += `- Key Dependencies: ${dependencies.slice(0, 5).join(', ')}${dependencies.length > 5 ? '...' : ''}\n`;
      }
      if (typeof analysis.entryPoint === 'string') {
        promptPrefix += `- Entry Point: ${analysis.entryPoint}\n`;
      }
      promptPrefix += `\nGenerate a Dockerfile based on this analysis.\n\n`;

      context.logger.info(
        { sessionId, language, framework },
        'Retrieved repository analysis from session',
      );
    } else {
      context.logger.warn({ sessionId }, 'Session not found or no analysis data available');
      promptPrefix = `Warning: Could not retrieve analysis data from session ${sessionId}. Analyze the repository to determine the best configuration.\n\n`;
    }
  } else if (path) {
    promptPrefix = `Analyze the repository at ${path} to detect the technology stack, dependencies, and requirements.\n\n`;
  }

  const promptParams = {
    language,
    framework,
    dependencies,
    ports,
    optimization: optimization === 'size' || optimization === 'balanced',
    securityHardening,
    multistage,
    baseImage,
    requirements: promptPrefix,
  } as DockerfilePromptParams;
  const basePrompt = promptTemplates.dockerfile(promptParams);

  // Build messages using the new prompt engine - minimize for speed
  const messages = await buildMessages({
    basePrompt,
    topic: 'generate_dockerfile',
    tool: 'generate-dockerfile',
    environment: validatedParams.environment || 'production',
    contract: {
      name: 'dockerfile_v1',
      description: 'Generate a Dockerfile',
    },
    knowledgeBudget: 500, // Minimal knowledge to prevent timeout
  });

  // Execute via AI with structured messages
  const mcpMessages = toMCPMessages(messages);

  // Log message size for debugging
  context.logger.info(
    {
      messageSize: JSON.stringify(mcpMessages).length,
      messageCount: mcpMessages.messages.length,
    },
    'Sending Dockerfile generation request',
  );

  const response = await context.sampling.createMessage({
    ...mcpMessages, // Spreads the MCP-compatible messages
    maxTokens: 3000, // Slightly increased for complete Dockerfiles
    modelPreferences: {
      hints: [{ name: 'dockerfile-generation' }],
    },
  });

  // Return result with workflow hints
  try {
    const responseText = response.content[0]?.text || '';

    // Parse the response to extract the actual Dockerfile
    let dockerfileContent = '';

    // First check if it's already a plain Dockerfile (starts with FROM)
    if (responseText.trim().startsWith('FROM')) {
      dockerfileContent = responseText.trim();
    } else {
      // Try to parse as JSON
      try {
        const parsed = JSON.parse(responseText);

        // Extract Dockerfile from various possible formats
        if (parsed.dockerfile_v1) {
          // Format: { dockerfile_v1: { Dockerfile: [...] } }
          if (Array.isArray(parsed.dockerfile_v1.Dockerfile)) {
            dockerfileContent = parsed.dockerfile_v1.Dockerfile.join('\n');
          } else if (typeof parsed.dockerfile_v1.Dockerfile === 'string') {
            dockerfileContent = parsed.dockerfile_v1.Dockerfile;
          } else if (typeof parsed.dockerfile_v1 === 'string') {
            dockerfileContent = parsed.dockerfile_v1;
          }
        } else if (parsed.Dockerfile) {
          // Format: { Dockerfile: [...] } or { Dockerfile: "..." }
          if (Array.isArray(parsed.Dockerfile)) {
            dockerfileContent = parsed.Dockerfile.join('\n');
          } else {
            dockerfileContent = parsed.Dockerfile;
          }
        } else if (parsed.dockerfile) {
          // Format: { dockerfile: "..." }
          dockerfileContent = parsed.dockerfile;
        } else if (typeof parsed === 'string') {
          // The JSON might just be a string
          dockerfileContent = parsed;
        }
      } catch {
        // Not JSON, might have markdown fences or other formatting
        // Try to extract content between ```dockerfile and ```
        const dockerfileMatch = responseText.match(/```dockerfile?\n([\s\S]*?)```/);
        if (dockerfileMatch?.[1]) {
          dockerfileContent = dockerfileMatch[1].trim();
        } else {
          // Last resort: use as-is but clean up common issues
          dockerfileContent = responseText
            .replace(/```/g, '')
            .replace(/^dockerfile\s*\n/i, '')
            .trim();
        }
      }
    }

    // Validate the extracted Dockerfile using proper parser
    try {
      const dockerfile = DockerfileParser.parse(dockerfileContent);

      // Check for basic requirements
      const instructions = dockerfile.getInstructions();
      const hasFrom = instructions.some((i) => i.getInstruction() === 'FROM');

      if (!hasFrom) {
        context.logger.error(
          {
            responseText: responseText.substring(0, 500),
            extractedContent: dockerfileContent.substring(0, 500),
          },
          'Dockerfile missing FROM instruction',
        );
        return Failure('Generated Dockerfile is missing FROM instruction');
      }

      // Check for syntax errors
      const syntaxErrors = dockerfile.getComments().filter((c) => c.getContent().includes('error'));
      if (syntaxErrors.length > 0) {
        context.logger.warn(
          { errors: syntaxErrors.map((e) => e.getContent()) },
          'Dockerfile has potential syntax issues',
        );
      }
    } catch (parseError) {
      context.logger.error(
        {
          error: parseError instanceof Error ? parseError.message : String(parseError),
          dockerfileContent: dockerfileContent.substring(0, 500),
        },
        'Failed to parse Dockerfile',
      );
      return Failure(
        `Generated Dockerfile has invalid syntax: ${parseError instanceof Error ? parseError.message : 'Unknown error'}`,
      );
    }

    // Log what we extracted
    context.logger.info(
      {
        originalLength: responseText.length,
        extractedLength: dockerfileContent.length,
        preview: dockerfileContent.substring(0, 100),
      },
      'Extracted Dockerfile content',
    );

    // Determine where to write the Dockerfile
    let dockerfilePath = '';
    if (sessionId) {
      // Get the analyzed path from session
      const sessionResult = await getSession(sessionId, context);
      if (sessionResult.ok && sessionResult.value.state.metadata?.analyzedPath) {
        dockerfilePath = nodePath.join(
          sessionResult.value.state.metadata.analyzedPath as string,
          'Dockerfile',
        );
      }
    } else if (validatedParams.path) {
      // Use the provided path
      let targetPath = validatedParams.path;
      if (!nodePath.isAbsolute(targetPath)) {
        targetPath = nodePath.resolve(process.cwd(), targetPath);
      }
      dockerfilePath = nodePath.join(targetPath, 'Dockerfile');
    } else {
      // Default to current directory
      dockerfilePath = nodePath.join(process.cwd(), 'Dockerfile');
    }

    // Write the Dockerfile to disk
    await fs.writeFile(dockerfilePath, dockerfileContent, 'utf-8');
    context.logger.info({ dockerfilePath }, 'Dockerfile written to disk');

    // Store the Dockerfile path in session if we have a session
    if (sessionId) {
      await updateSession(
        sessionId,
        {
          metadata: {
            dockerfilePath,
            dockerfileContent,
          },
          current_step: 'generate-dockerfile',
        },
        context,
      );
    }

    return Success({
      dockerfile: dockerfileContent,
      dockerfilePath,
      sessionId: sessionId || validatedParams.sessionId,
      workflowHints: {
        nextStep: 'build-image',
        message: `Dockerfile generated and saved to ${dockerfilePath}. Use "build-image" with the sessionId to build the Docker image.`,
      },
    });
  } catch (e) {
    const error = e as Error;
    return Failure(`Dockerfile generation failed: ${error.message}`);
  }
}

export const metadata = {
  name: 'generate-dockerfile',
  description: 'Generate optimized Dockerfiles for containerization',
  version: '2.1.0',
  aiDriven: true,
  knowledgeEnhanced: true,
};
