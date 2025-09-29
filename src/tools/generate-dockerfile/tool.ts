/**
 * Generate Dockerfile Tool
 *
 * AI-powered Dockerfile generation for containerizing applications. Analyzes
 * project structure, dependencies, and best practices to create optimized
 * Dockerfiles with multi-stage builds, security hardening, and performance
 * optimizations.
 *
 * @category docker
 * @version 3.0.0
 * @aiDriven true
 * @knowledgeEnhanced true
 * @samplingStrategy rerank
 */

import { Success, Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { Tool } from '@/types/tool';
import { generateDockerfileSchema } from './schema';
import type { AIResponse } from '../ai-response-types';
import type { RepositoryAnalysis } from '@/tools/analyze-repo/schema';
import { promises as fs } from 'node:fs';
import nodePath from 'node:path';
import { DockerfileParser } from 'dockerfile-ast';
import { promptTemplates, type DockerfilePromptParams } from '@/ai/prompt-templates';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { sampleWithRerank } from '@/mcp/ai/sampling-runner';
import { validateDockerfileContent } from '@/validation/dockerfile-validator';
import type { KnowledgeEnhancementResult } from '@/mcp/ai/knowledge-enhancement';
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

  return responseText.trim();
}

/**
 * Score a Dockerfile for quality assessment
 * Returns a score from 0-100 based on parseability, validation, and best practices
 */
function scoreDockerfile(text: string): number {
  let score = 0;

  // Basic parseability (30 points)
  try {
    DockerfileParser.parse(text);
    score += 30;

    // Additional parsing checks
    if (text.includes('FROM ')) score += 5;
    if (!/^\s*$/.test(text)) score += 5; // Not empty
  } catch {
    // If it doesn't parse, it's fundamentally broken
    return 0;
  }

  // Best practices scoring (70 points total)
  // Security practices (25 points)
  if (/FROM\s+[^\s:]+:[^:\s]+(?:\s|$)/.test(text) && !/FROM\s+[^\s:]+:latest/.test(text)) {
    score += 10; // Pinned version, not latest
  }
  if (/USER\s+(?!root|0)\w+/.test(text)) {
    score += 10; // Non-root user
  }
  if (!/(password|secret|api_key|token)\s*[=:]\s*[^\s]+/i.test(text)) {
    score += 5; // No hardcoded secrets
  }

  // Multi-stage builds (15 points)
  const fromCount = (text.match(/^FROM\s+/gm) || []).length;
  if (fromCount > 1 || /FROM\s+.*\sAS\s+\w+/i.test(text)) {
    score += 15;
  }

  // Health checks (10 points)
  if (/HEALTHCHECK/i.test(text)) {
    score += 10;
  }

  // Working directory (5 points)
  if (/WORKDIR/i.test(text)) {
    score += 5;
  }

  // Port exposure (5 points)
  if (/EXPOSE\s+\d+/.test(text)) {
    score += 5;
  }

  // Layer optimization (10 points)
  if (/COPY\s+package.*\.json/.test(text) || /COPY\s+go\.mod\s+go\.sum/.test(text)) {
    score += 5; // Dependency files copied separately
  }
  if (!/RUN.*&&.*&&.*&&.*&&/m.test(text)) {
    score += 5; // Reasonable RUN command chaining
  }

  return Math.min(score, 100);
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

  // Try to get analysis from session if sessionId is provided
  if (sessionId && ctx.session) {
    const results = ctx.session.get('results');
    if (
      results &&
      typeof results === 'object' &&
      'analyze-repo' in results &&
      results['analyze-repo']
    ) {
      const analysis = results['analyze-repo'] as RepositoryAnalysis;

      // Use actual values from the analysis
      language = typeof analysis.language === 'string' ? analysis.language : 'auto-detect';
      framework = typeof analysis.framework === 'string' ? analysis.framework : undefined;
      dependencies = Array.isArray(analysis.dependencies) ? analysis.dependencies : [];
      ports = Array.isArray(analysis.suggestedPorts) ? analysis.suggestedPorts : [8080];

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
    const dockerfileParams: DockerfilePromptParams = {
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
      ...(language && { language }),
      ...(framework && { framework }),
      contract: {
        name: 'dockerfile_v1',
        description: 'Generate optimized Dockerfile',
      },
      knowledgeBudget: 5000,
    });

    // Use N-best sampling for better quality Dockerfiles
    const samplingResult = await sampleWithRerank(
      ctx,
      async (attempt) => ({
        ...toMCPMessages(messages),
        maxTokens: 4096,
        includeContext: attempt === 0 ? 'allServers' : 'thisServer',
        modelPreferences: {
          hints: [{ name: 'dockerfile-generate' }],
          intelligencePriority: 0.9,
          speedPriority: attempt === 0 ? 0.6 : 0.8,
          costPriority: 0.4,
        },
      }),
      (text) => scoreDockerfile(extractDockerfileContent(text)),
      { count: 3, stopAt: 95 },
    );

    if (!samplingResult.ok) {
      return Failure(`Failed to generate Dockerfile: ${samplingResult.error}`);
    }

    const dockerfileContent = extractDockerfileContent(samplingResult.value.text);

    ctx.logger.info(
      {
        candidatesGenerated: samplingResult.value.all?.length || 1,
        winnerScore: samplingResult.value.winner.score,
        model: samplingResult.value.model,
      },
      'N-best sampling completed for Dockerfile generation',
    );

    // Validate the extracted Dockerfile using proper parser
    try {
      const dockerfile = DockerfileParser.parse(dockerfileContent);

      // Check for basic requirements
      const instructions = dockerfile.getInstructions();
      const hasFrom = instructions.some((i) => i.getInstruction() === 'FROM');

      if (!hasFrom) {
        ctx.logger.error(
          {
            sampledText: samplingResult.value.text.substring(0, 500),
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

    // Self-repair loop - attempt to fix validation issues
    let finalDockerfileContent = dockerfileContent;
    let knowledgeEnhancement: KnowledgeEnhancementResult | undefined;

    const initialValidation = await validateDockerfileContent(dockerfileContent, {
      enableExternalLinter: false,
    });

    // Apply knowledge enhancement if there are validation issues or if requested
    if (initialValidation.score < 90) {
      try {
        const { enhanceWithKnowledge, createEnhancementFromValidation } = await import(
          '@/mcp/ai/knowledge-enhancement'
        );

        const enhancementRequest = createEnhancementFromValidation(
          dockerfileContent,
          'dockerfile',
          initialValidation.results
            .filter((r) => !r.passed)
            .map((r) => ({
              message: r.message || 'Validation issue',
              severity: r.metadata?.severity === 'error' ? 'error' : 'warning',
              category: r.ruleId?.split('-')[0] || 'general',
            })),
          'all',
        );

        // Add custom instructions as user query if available
        if (input.customInstructions) {
          enhancementRequest.userQuery = `Original requirements: ${input.customInstructions}`;
        }

        const enhancementResult = await enhanceWithKnowledge(enhancementRequest, ctx);

        if (enhancementResult.ok) {
          knowledgeEnhancement = enhancementResult.value;
          finalDockerfileContent = knowledgeEnhancement.enhancedContent;

          ctx.logger.info(
            {
              knowledgeAppliedCount: knowledgeEnhancement.knowledgeApplied.length,
              confidence: knowledgeEnhancement.confidence,
              enhancementAreas: knowledgeEnhancement.analysis.enhancementAreas.length,
            },
            'Knowledge enhancement applied to Dockerfile',
          );
        } else {
          ctx.logger.warn(
            { error: enhancementResult.error },
            'Knowledge enhancement failed, using original Dockerfile',
          );
        }
      } catch (enhancementError) {
        ctx.logger.debug(
          {
            error:
              enhancementError instanceof Error
                ? enhancementError.message
                : String(enhancementError),
          },
          'Knowledge enhancement threw exception, continuing without enhancement',
        );
      }
    }

    // Validate the enhanced/original Dockerfile and apply self-repair if needed
    const currentValidation = await validateDockerfileContent(finalDockerfileContent, {
      enableExternalLinter: false,
    });

    if (currentValidation.score < 80) {
      ctx.logger.warn(
        {
          currentScore: currentValidation.score,
          issues: currentValidation.results.filter((r) => !r.passed).length,
        },
        'Attempting self-repair due to validation issues',
      );

      // Build repair prompt with specific validation errors
      const validationErrors = currentValidation.results
        .filter((r) => !r.passed)
        .map((r) => `- ${r.message}`)
        .join('\n');

      const repairPrompt = `The generated Dockerfile has validation issues:
${validationErrors}

Please fix these issues and respond with ONLY the corrected Dockerfile in a \`\`\`dockerfile code block.

Current Dockerfile:
\`\`\`dockerfile
${finalDockerfileContent}
\`\`\``;

      try {
        const repairMessages = await buildMessages({
          basePrompt: repairPrompt,
          topic: 'dockerfile_repair',
          tool: name,
          environment,
          ...(language && { language }),
          ...(framework && { framework }),
          contract: {
            name: 'dockerfile_repair',
            description: 'Fix Dockerfile validation issues',
          },
          knowledgeBudget: 1500,
        });

        const repaired = await ctx.sampling.createMessage({
          ...toMCPMessages(repairMessages),
          maxTokens: 4096,
          includeContext: 'thisServer',
          modelPreferences: {
            hints: [{ name: 'dockerfile-fix' }],
            intelligencePriority: 0.95, // Higher for repairs
          },
        });

        const repairedText = repaired.content?.[0]?.text ?? '';
        if (repairedText) {
          const repairedDockerfile = extractDockerfileContent(repairedText);

          // Validate the repaired version
          const repairedValidation = await validateDockerfileContent(repairedDockerfile, {
            enableExternalLinter: false,
          });

          if (repairedValidation.score > currentValidation.score) {
            finalDockerfileContent = repairedDockerfile;
            ctx.logger.info(
              {
                currentScore: currentValidation.score,
                repairedScore: repairedValidation.score,
                improvement: repairedValidation.score - currentValidation.score,
              },
              'Self-repair successful - using improved Dockerfile',
            );
          } else {
            ctx.logger.info(
              {
                currentScore: currentValidation.score,
                repairedScore: repairedValidation.score,
              },
              'Self-repair did not improve score - using current',
            );
          }
        }
      } catch (repairError) {
        ctx.logger.warn(
          {
            error: repairError instanceof Error ? repairError.message : String(repairError),
          },
          'Self-repair failed - using original Dockerfile',
        );
      }
    }

    // Log what we extracted
    ctx.logger.info(
      {
        originalLength: samplingResult.value.text.length,
        extractedLength: finalDockerfileContent.length,
        preview: finalDockerfileContent.substring(0, 100),
      },
      'Extracted Dockerfile content',
    );

    // Determine where to write the Dockerfile
    let dockerfilePath = '';
    if (sessionId && ctx.session) {
      // Get the analyzed path from session
      const metadata = ctx.session.get('metadata');
      if (
        metadata &&
        typeof metadata === 'object' &&
        'analyzedPath' in metadata &&
        metadata.analyzedPath
      ) {
        dockerfilePath = nodePath.join(metadata.analyzedPath as string, 'Dockerfile');
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
        await fs.writeFile(dockerfilePath, finalDockerfileContent, 'utf-8');
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
    if (sessionId && ctx.session) {
      ctx.session.set('metadata', {
        dockerfileGenerated: true,
        dockerfilePath: written ? dockerfilePath : undefined,
        dockerfileContent: finalDockerfileContent,
        baseImage: finalDockerfileContent.match(/FROM\s+([^\s]+)/)?.[1],
        multistage,
        securityHardening,
        optimization,
      });
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

    // Build suggestions array with knowledge enhancement info
    const suggestions = [
      written
        ? `✅ Dockerfile written to: ${dockerfilePath}`
        : '✅ Dockerfile generated (not written to disk)',
    ];

    if (knowledgeEnhancement) {
      suggestions.push(
        `🧠 Enhanced with ${knowledgeEnhancement.knowledgeApplied.length} knowledge improvements`,
      );
    }

    return Success({
      content: finalDockerfileContent,
      language: 'dockerfile',
      analysis: knowledgeEnhancement
        ? {
            enhancementAreas: knowledgeEnhancement.analysis.enhancementAreas,
            confidence: knowledgeEnhancement.confidence,
            knowledgeApplied: knowledgeEnhancement.knowledgeApplied,
          }
        : undefined,
      confidence: knowledgeEnhancement ? knowledgeEnhancement.confidence : 0.9,
      suggestions,
      workflowHints: {
        nextStep: 'build-image',
        message: `Dockerfile generated successfully. Use "build-image" with sessionId ${sessionId || '<sessionId>'} to build your container image, or review and customize the Dockerfile first.`,
      },
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
  metadata: {
    aiDriven: true,
    knowledgeEnhanced: true,
    samplingStrategy: 'rerank',
    enhancementCapabilities: ['content-generation', 'validation', 'optimization', 'self-repair'],
  },
  run,
};

export default tool;
