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
 * @samplingStrategy single
 */

import { Success, Failure, type Result, TOPICS } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { Tool } from '@/types/tool';
import { generateDockerfileSchema } from './schema';
import type { AIResponse } from '../ai-response-types';
import type { ModuleInfo } from '@/tools/analyze-repo/schema';
import { promises as fs } from 'node:fs';
import nodePath from 'node:path';
import { DockerfileParser } from 'dockerfile-ast';
import { promptTemplates, type DockerfilePromptParams } from '@/ai/prompt-templates';
import { buildMessages } from '@/ai/prompt-engine';
import { toMCPMessages } from '@/mcp/ai/message-converter';
import { sampleWithRerank } from '@/mcp/ai/sampling-runner';
import { createDockerfileScoringFunction } from '@/lib/scoring';
import { validateDockerfileContent } from '@/validation/dockerfile-validator';
import type { KnowledgeEnhancementResult } from '@/mcp/ai/knowledge-enhancement';
import { extractDockerfileContent } from '@/lib/content-extraction';

import type { z } from 'zod';

const name = 'generate-dockerfile';
const description = 'Generate optimized Dockerfiles for containerizing applications';
const version = '3.0.0';

/**
 * Generate Dockerfile for a single module or app
 */
async function generateSingleDockerfile(
  input: z.infer<typeof generateDockerfileSchema>,
  ctx: ToolContext,
  targetModule?: ModuleInfo,
): Promise<Result<AIResponse>>;

async function generateSingleDockerfile(
  input: z.infer<typeof generateDockerfileSchema>,
  ctx: ToolContext,
  targetModule?: ModuleInfo,
): Promise<Result<AIResponse>> {
  const { multistage, securityHardening, optimization, sessionId, baseImagePreference } = input;

  // Determine repository path: use module path if available, otherwise input.path
  const path = targetModule?.path ?? input.path;
  const targetModulePath = targetModule?.path;
  const targetDockerfilePath = targetModule?.dockerfilePath;

  // Initialize variables from module data if available
  let language = 'auto-detect';
  let framework: string | undefined;
  let dependencies: string[] = [];
  let ports: number[] = [8080];
  let requirements: string | undefined;
  const baseImage: string | undefined = input.baseImage;

  // If generating for a specific module, use module-specific data
  if (targetModule) {
    language = targetModule.language || 'auto-detect';
    framework = targetModule.framework;
    dependencies = targetModule.dependencies || [];
    ports = targetModule.ports || [8080];

    // Build requirements from module analysis
    const reqParts: string[] = [];
    reqParts.push(`Module: ${targetModule.name}`);
    reqParts.push(`Path: ${targetModule.path}`);
    if (language)
      reqParts.push(
        `Language: ${language}${targetModule.languageVersion ? ` (${targetModule.languageVersion})` : ''}`,
      );
    if (framework)
      reqParts.push(
        `Framework: ${framework}${targetModule.frameworkVersion ? ` (${targetModule.frameworkVersion})` : ''}`,
      );
    if (targetModule.buildSystem?.type)
      reqParts.push(`Build System: ${targetModule.buildSystem.type}`);
    if (dependencies.length > 0) {
      reqParts.push(
        `Key Dependencies: ${dependencies.slice(0, 5).join(', ')}${dependencies.length > 5 ? '...' : ''}`,
      );
    }
    if (targetModule.entryPoint) reqParts.push(`Entry Point: ${targetModule.entryPoint}`);
    requirements = reqParts.join('\n');

    ctx.logger.info(
      { sessionId, moduleName: targetModule.name, language, framework },
      'Using module-specific analysis data',
    );
  } else if (path) {
    // No module data provided, analyze repository directly
    requirements = `Analyze the repository at ${path} to detect the technology stack, dependencies, and requirements.`;
  }

  // Path is required (either from module or input)
  if (!path) {
    return Failure(
      'Repository path is required. Provide either the path parameter or modules with path fields.',
    );
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

    // Use deterministic sampling with optional scoring for quality validation
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
      createDockerfileScoringFunction(),
      {},
    );

    if (!samplingResult.ok) {
      return Failure(`Failed to generate Dockerfile: ${samplingResult.error}`);
    }

    const extraction = extractDockerfileContent(samplingResult.value.text);
    const dockerfileContent =
      extraction.success && extraction.content ? extraction.content : samplingResult.value.text;

    ctx.logger.info(
      {
        score: samplingResult.value.score,
        model: samplingResult.value.model,
      },
      'Deterministic sampling completed for Dockerfile generation',
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
      const errorMsg = `Generated Dockerfile has invalid syntax: ${parseError instanceof Error ? parseError.message : 'Unknown error'}`;
      ctx.logger.error(
        {
          error: parseError instanceof Error ? parseError.message : String(parseError),
          dockerfileContent: dockerfileContent.substring(0, 500),
        },
        'Failed to parse Dockerfile',
      );
      return Failure(errorMsg);
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
          const repairedExtraction = extractDockerfileContent(repairedText);
          const repairedDockerfile =
            repairedExtraction.success && repairedExtraction.content
              ? repairedExtraction.content
              : repairedText;

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

    if (targetDockerfilePath) {
      // Module specified a custom dockerfile path
      dockerfilePath = nodePath.isAbsolute(targetDockerfilePath)
        ? targetDockerfilePath
        : nodePath.resolve(process.cwd(), targetDockerfilePath);
      ctx.logger.info({ dockerfilePath }, 'Using custom Dockerfile path from module');
    } else if (targetModulePath) {
      // Use module path
      const resolvedModulePath = nodePath.isAbsolute(targetModulePath)
        ? targetModulePath
        : nodePath.resolve(process.cwd(), targetModulePath);
      dockerfilePath = nodePath.join(resolvedModulePath, 'Dockerfile');
      ctx.logger.debug({ dockerfilePath }, 'Using module path for Dockerfile');
    } else if (path) {
      // Use repository path
      const resolvedPath = nodePath.isAbsolute(path) ? path : nodePath.resolve(process.cwd(), path);
      dockerfilePath = nodePath.join(resolvedPath, 'Dockerfile');
      ctx.logger.debug({ dockerfilePath }, 'Using repository path for Dockerfile');
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

/**
 * Main run function - orchestrates single or multi-module generation
 */
async function run(
  input: z.infer<typeof generateDockerfileSchema>,
  ctx: ToolContext,
): Promise<Result<AIResponse>> {
  const { modules } = input;

  // Check for multi-module/monorepo scenario
  if (modules && modules.length > 0) {
    if (modules.length === 1) {
      const targetModule = modules[0];
      if (!targetModule) {
        return Failure('Module array contains undefined element');
      }
      ctx.logger.info(
        { moduleName: targetModule.name, modulePath: targetModule.path },
        'Generating Dockerfile for single module',
      );
      return generateSingleDockerfile(input, ctx, targetModule as ModuleInfo);
    }

    // Multiple modules - generate for all modules
    ctx.logger.info({ moduleCount: modules.length }, 'Generating Dockerfiles for multiple modules');

    const results: Array<{ module: string; success: boolean; path?: string; error?: string }> = [];
    const dockerfiles: Array<{ module: string; content: string; path?: string }> = [];

    for (const module of modules) {
      ctx.logger.info({ moduleName: module.name }, 'Generating Dockerfile for module');

      const result = await generateSingleDockerfile(input, ctx, module as ModuleInfo);

      if (result.ok) {
        const value = result.value as {
          content?: string;
          workflowHints?: { message?: string };
        };
        const extractedPath = value.workflowHints?.message?.match(/written to: (.+)/)?.[1];
        results.push({
          module: module.name,
          success: true,
          ...(extractedPath ? { path: extractedPath } : {}),
        });
        dockerfiles.push({
          module: module.name,
          content: value.content || '',
          ...(extractedPath ? { path: extractedPath } : {}),
        });
        ctx.logger.info({ moduleName: module.name }, 'Dockerfile generated successfully');
      } else {
        results.push({
          module: module.name,
          success: false,
          error: result.error,
        });
        ctx.logger.warn(
          { moduleName: module.name, error: result.error },
          'Failed to generate Dockerfile for module',
        );
      }
    }

    const successCount = results.filter((r) => r.success).length;
    const failureCount = results.filter((r) => !r.success).length;

    if (successCount === 0) {
      return Failure(
        `Failed to generate Dockerfiles for all ${modules.length} modules:\n${results.map((r) => `- ${r.module}: ${r.error}`).join('\n')}`,
      );
    }

    // Build summary response
    const summary = `Generated Dockerfiles for ${successCount}/${modules.length} modules:\n${results
      .filter((r) => r.success)
      .map((r) => `✅ ${r.module}${r.path ? `: ${r.path}` : ''}`)
      .join('\n')}${
      failureCount > 0
        ? `\n\n⚠️  Failed modules (${failureCount}):\n${results
            .filter((r) => !r.success)
            .map((r) => `❌ ${r.module}: ${r.error}`)
            .join('\n')}`
        : ''
    }`;

    return Success({
      content: summary,
      language: 'text',
      confidence: successCount / modules.length,
      suggestions: [
        `Successfully generated ${successCount} Dockerfile(s)`,
        failureCount > 0 ? `${failureCount} module(s) failed` : 'All modules successful',
      ],
      analysis: {
        enhancementAreas: [],
        confidence: successCount / modules.length,
        knowledgeApplied: [],
      },
      workflowHints: {
        nextStep: 'build-image',
        message: `Dockerfiles generated for ${successCount} module(s). Use "build-image" for each module to build container images.`,
      },
    });
  }

  // Single-module repository - generate for single app
  return generateSingleDockerfile(input, ctx);
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
    samplingStrategy: 'single',
    enhancementCapabilities: ['content-generation', 'validation', 'optimization', 'self-repair'],
  },
  run,
};

export default tool;
