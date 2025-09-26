/**
 * Generate Dockerfile tool using the new Tool pattern
 */

import { Success, Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { Tool } from '@/types/tool';
import { generateDockerfileSchema } from './schema';
import type { AIResponse } from '../ai-response-types';
import { getSession, updateSession } from '@/mcp/tool-session-helpers';
import { promises as fs } from 'node:fs';
import nodePath from 'node:path';
import { DockerfileParser } from 'dockerfile-ast';
import { promptTemplates } from '@/ai/prompt-templates';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import type { z } from 'zod';

const name = 'generate-dockerfile';
const description = 'Generate optimized Dockerfiles for containerizing applications';
const version = '3.0.0';

/**
 * Extract Dockerfile content from AI response
 */
function extractDockerfileContent(responseText: string): string {
  // Try to extract content between triple backticks
  const codeBlockMatch = responseText.match(/```(?:dockerfile)?\s*\n([\s\S]*?)```/);
  if (codeBlockMatch?.[1]) {
    return codeBlockMatch[1].trim();
  }

  // Try to extract content between JSON-like structure
  const jsonMatch = responseText.match(/"dockerfile":\s*"([^"]+)"/);
  if (jsonMatch?.[1]) {
    // Unescape the JSON string
    return jsonMatch[1].replace(/\\n/g, '\n').replace(/\\"/g, '"').trim();
  }

  // Look for FROM statement and extract from there
  const fromMatch = responseText.match(/(FROM\s+[\s\S]*)/);
  if (fromMatch?.[1]) {
    // Find the end of the Dockerfile (usually ends with CMD, ENTRYPOINT, or end of text)
    const dockerContent = fromMatch[1];
    const endMatch = dockerContent.match(/([\s\S]*?(?:CMD|ENTRYPOINT)[^\n]*)/);
    if (endMatch?.[1]) {
      return endMatch[1].trim();
    }
    return dockerContent.trim();
  }

  // If all else fails, return the entire response (it might be the raw Dockerfile)
  return responseText.trim();
}

async function run(
  input: z.infer<typeof generateDockerfileSchema>,
  ctx: ToolContext,
): Promise<Result<AIResponse>> {
  const { multistage, securityHardening, optimization, sessionId, path, baseImagePreference } =
    input;

  // Retrieve repository analysis from session if sessionId is provided
  let language = 'auto-detect';
  let framework: string | undefined;
  let dependencies: string[] = [];
  let ports: number[] = [8080];
  let requirements: string | undefined;
  const baseImage: string | undefined = input.baseImage;

  if (sessionId) {
    const sessionResult = await getSession(sessionId, ctx);
    if (sessionResult.ok && sessionResult.value.state.metadata?.repositoryAnalysis) {
      const analysis = sessionResult.value.state.metadata.repositoryAnalysis as Record<
        string,
        unknown
      >;

      // Use actual values from the analysis
      language = typeof analysis.language === 'string' ? analysis.language : 'auto-detect';
      framework = typeof analysis.framework === 'string' ? analysis.framework : undefined;
      dependencies = Array.isArray(analysis.dependencies)
        ? (analysis.dependencies as string[])
        : [];
      ports = Array.isArray(analysis.suggestedPorts)
        ? (analysis.suggestedPorts as number[])
        : [8080];

      // Build requirements from analysis
      const reqParts: string[] = [];
      reqParts.push(
        `Language: ${language} ${typeof analysis.languageVersion === 'string' ? `(${analysis.languageVersion})` : ''}`,
      );
      if (framework) {
        reqParts.push(
          `Framework: ${framework} ${typeof analysis.frameworkVersion === 'string' ? `(${analysis.frameworkVersion})` : ''}`,
        );
      }
      const buildSystem = analysis.buildSystem as Record<string, unknown> | undefined;
      if (buildSystem?.type) {
        reqParts.push(`Build System: ${buildSystem.type}`);
      }
      if (dependencies.length > 0) {
        reqParts.push(
          `Key Dependencies: ${dependencies.slice(0, 5).join(', ')}${dependencies.length > 5 ? '...' : ''}`,
        );
      }
      if (typeof analysis.entryPoint === 'string') {
        reqParts.push(`Entry Point: ${analysis.entryPoint}`);
      }
      requirements = reqParts.join('\n');

      ctx.logger.info(
        { sessionId, language, framework },
        'Retrieved repository analysis from session',
      );
    } else {
      ctx.logger.warn({ sessionId }, 'Session not found or no analysis data available');
      requirements = `Note: Could not retrieve analysis data from session ${sessionId}. Please analyze the repository to determine the best configuration.`;
    }
  } else if (path) {
    requirements = `Analyze the repository at ${path} to detect the technology stack, dependencies, and requirements.`;
  }

  // Add custom instructions if provided
  if (input.customInstructions) {
    requirements = requirements
      ? `${requirements}\n\nAdditional instructions:\n${input.customInstructions}`
      : input.customInstructions;
  }

  // Add base image preference if provided
  if (baseImagePreference && !baseImage) {
    requirements = requirements
      ? `${requirements}\n\nBase image preference: ${baseImagePreference}`
      : `Base image preference: ${baseImagePreference}`;
  }

  const environment = input.environment || 'production';

  ctx.logger.info(
    { language, framework, multistage, optimization, path },
    'Starting Dockerfile generation with prompt templates',
  );

  try {
    // Use the prompt template from @/ai/prompt-templates
    const dockerfileParams: any = {
      language,
      dependencies,
      ports,
      optimization:
        optimization === true ||
        (typeof optimization === 'string' &&
          ['size', 'performance', 'balanced'].includes(optimization)),
    };
    if (framework) dockerfileParams.framework = framework;
    if (requirements) dockerfileParams.requirements = requirements;
    if (baseImage) dockerfileParams.baseImage = baseImage;
    if (securityHardening !== undefined) dockerfileParams.securityHardening = securityHardening;
    if (multistage !== undefined) dockerfileParams.multistage = multistage;

    const basePrompt = promptTemplates.dockerfile(dockerfileParams);

    // Use knowledge injection via buildMessages
    const messages = await buildMessages({
      basePrompt,
      topic: TOPICS.DOCKERFILE_GENERATION,
      tool: name,
      environment,
      contract: {
        name: 'dockerfile_v1',
        description: 'Generate optimized Dockerfile',
      },
      knowledgeBudget: 5000,
    });

    // Convert and call AI
    const mcpMessages = toMCPMessages(messages);
    const response = await ctx.sampling.createMessage({
      ...mcpMessages,
      maxTokens: 4096,
      modelPreferences: {
        hints: [{ name: 'dockerfile-generation' }],
      },
    });

    // Extract content from response
    const responseText = response.content[0]?.text || '';
    const dockerfileContent = extractDockerfileContent(responseText);

    // Validate the extracted Dockerfile using proper parser
    try {
      const dockerfile = DockerfileParser.parse(dockerfileContent);

      // Check for basic requirements
      const instructions = dockerfile.getInstructions();
      const hasFrom = instructions.some((i) => i.getInstruction() === 'FROM');

      if (!hasFrom) {
        ctx.logger.error(
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
        ctx.logger.warn(
          { errors: syntaxErrors.map((e) => e.getContent()) },
          'Dockerfile has potential syntax issues',
        );
      }
    } catch (parseError) {
      ctx.logger.error(
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
    ctx.logger.info(
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
      const sessionResult = await getSession(sessionId, ctx);
      if (sessionResult.ok && sessionResult.value.state.metadata?.analyzedPath) {
        dockerfilePath = nodePath.join(
          sessionResult.value.state.metadata.analyzedPath as string,
          'Dockerfile',
        );
      }
    } else if (input.path) {
      // Use the provided path
      let targetPath = input.path;
      if (!nodePath.isAbsolute(targetPath)) {
        targetPath = nodePath.resolve(process.cwd(), targetPath);
      }
      dockerfilePath = nodePath.join(targetPath, 'Dockerfile');
    }

    // Write Dockerfile if we have a path
    let written = false;
    if (dockerfilePath) {
      try {
        await fs.writeFile(dockerfilePath, dockerfileContent, 'utf-8');
        written = true;
        ctx.logger.info({ path: dockerfilePath }, 'Dockerfile written successfully');
      } catch (writeError) {
        ctx.logger.error(
          {
            error: writeError instanceof Error ? writeError.message : String(writeError),
            path: dockerfilePath,
          },
          'Failed to write Dockerfile',
        );
      }
    }

    // Update session if available
    if (sessionId) {
      const updateResult = await updateSession(
        sessionId,
        {
          metadata: {
            dockerfileGenerated: true,
            dockerfilePath: written ? dockerfilePath : undefined,
            dockerfileContent,
            baseImage: dockerfileContent.match(/FROM\s+([^\s]+)/)?.[1],
            multistage,
            securityHardening,
            optimization,
          },
        },
        ctx,
      );

      if (!updateResult.ok) {
        ctx.logger.warn({ sessionId }, 'Failed to update session with Dockerfile info');
      }
    }

    // Build workflow hints
    const workflowHints: string[] = [];
    workflowHints.push(
      written
        ? `✅ Dockerfile written to: ${dockerfilePath}`
        : '✅ Dockerfile generated (not written to disk)',
    );
    workflowHints.push(`\n📋 Next steps:`);
    workflowHints.push(`1. Review and customize the generated Dockerfile`);
    workflowHints.push(`2. Build the image: docker build -t my-app:latest .`);
    workflowHints.push(`3. Test locally: docker run -p 8080:8080 my-app:latest`);
    if (sessionId) {
      workflowHints.push(`4. Generate K8s manifests: use generate-k8s-manifests with sessionId`);
    }

    return Success({
      content: dockerfileContent,
      language: 'dockerfile',
      analysis: undefined,
      confidence: 0.9,
      suggestions: workflowHints,
    });
  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : String(error);
    ctx.logger.error({ error: errorMessage }, 'Dockerfile generation failed');
    return Failure(`Dockerfile generation failed: ${errorMessage}`);
  }
}

const tool: Tool<typeof generateDockerfileSchema, AIResponse> = {
  name,
  description,
  category: 'docker',
  version,
  schema: generateDockerfileSchema,
  run,
};

export default tool;
