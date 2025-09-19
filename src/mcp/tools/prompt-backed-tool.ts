/**
 * Prompt-Backed Tool Factory - AI-powered tool implementation pattern
 *
 * Creates MCP tools that leverage AI templates and knowledge packs for decision making,
 * keeping only infrastructure operations in TypeScript.
 */

import { z } from 'zod';
import type { Logger } from 'pino';
import { getPromptMetadata } from '../../prompts/prompt-registry.js';
import { getSession } from '../tool-session-helpers.js';
import { parseAndValidateJson } from '../ai/json-output.js';
import { makeProvenance, shouldIncludeProvenance } from '../ai/prompt-provenance.js';
import { getPolicyRules } from '../../lib/policy-loader.js';
import { getKnowledgeRecommendations } from '../../knowledge/index.js';
import type { ToolContext, SamplingRequest } from '@mcp/context';
import {
  Success,
  Failure,
  type Result,
  type DockerClient,
  type KubernetesClient,
  type PromptTemplate,
  type ConfigurationObject,
} from '@types';
import { extractErrorMessage } from '@lib/error-utils';

// Explicit dependencies instead of global DI
export interface ToolDeps {
  logger: Logger;
  fs?: typeof import('fs');
  docker?: DockerClient;
  k8s?: KubernetesClient;
}

export interface PromptBackedToolOptions<TIn, TOut> {
  name: string;
  description: string;
  inputSchema: z.ZodSchema<TIn>;
  outputSchema: z.ZodSchema<TOut>;
  promptId: string;
  knowledge?: {
    category?: 'dockerfile' | 'kubernetes' | 'security';
    textSelector?: (params: TIn) => string | undefined;
    context?: (params: TIn) => Record<string, string | undefined>;
    limit?: number; // default 4
  };
  policy?: {
    tool?: string; // override tool name for policy lookup
    extractor?: (params: TIn) => Record<string, unknown>;
  };
  // Retries: 2 attempts with fixed delays (500ms, 1000ms)
}

// Simple retry with fixed backoff (500ms, 1000ms)
async function withRetry<T>(fn: () => Promise<T>, attempts = 2, logger?: Logger): Promise<T> {
  const delays = [500, 1000]; // Fixed delays
  let last: unknown;
  for (let i = 0; i < attempts; i++) {
    try {
      return await fn();
    } catch (e) {
      last = e;
      if (i < attempts - 1) {
        const delay = delays[i] || 1000; // Use fixed delay or fallback to 1000ms
        logger?.debug(`Retry ${i + 1}/${attempts} after ${delay}ms`);
        await new Promise((r) => setTimeout(r, delay));
      }
    }
  }
  throw last;
}

// Mock sampling config loader - replace with actual implementation
async function samplingConfigFor(
  _promptId: string,
): Promise<{ maxTokens: number; stopSequences: string[] }> {
  return {
    maxTokens: 2048,
    stopSequences: [] as string[],
  };
}

// Helper to call MCP host's AI via context
async function runHostAssist(
  prompt: string,
  context: ToolContext,
  options: { maxTokens?: number; stopSequences?: string[]; outputFormat?: string },
): Promise<string> {
  // If JSON output is expected, wrap the prompt with strong JSON instructions
  const finalPrompt =
    options.outputFormat === 'json'
      ? `IMPORTANT: You MUST respond with ONLY valid JSON. Do not write any text before or after the JSON. Do not explain. Do not acknowledge. Just output the raw JSON object.

${prompt}

Remember: Output ONLY the JSON object, nothing else. Start your response with { and end with }`
      : prompt;

  const response = await context.sampling.createMessage({
    messages: [
      {
        role: 'user',
        content: [{ type: 'text', text: finalPrompt }],
      },
    ],
    ...(options.maxTokens && { maxTokens: options.maxTokens }),
    ...(options.stopSequences && { stopSequences: options.stopSequences }),
  });

  // Extract text from response
  const text = response.content
    .filter((c) => c.type === 'text')
    .map((c) => c.text)
    .join('');

  return text;
}

// Build prompt from template with variables
function buildPromptFromTemplate(
  prompt: PromptTemplate | ConfigurationObject,
  variables: Record<string, unknown>,
): string {
  let result = '';

  // Concatenate template (system instructions) and user prompt
  // Both may contain variables that need substitution
  if ('template' in prompt && typeof prompt.template === 'string') {
    result = prompt.template;
  }

  if ('user' in prompt && typeof prompt.user === 'string') {
    // Add user prompt after template (system instructions)
    if (result) {
      result += '\n\n';
    }
    result += prompt.user;
  } else if ('system' in prompt && typeof prompt.system === 'string' && !result) {
    // Fallback to system field if no template and no user
    result = prompt.system;
  }

  // Replace template variables
  for (const [key, value] of Object.entries(variables)) {
    const placeholder = `{{${key}}}`;
    const replacement =
      value === undefined ? '' : typeof value === 'string' ? value : JSON.stringify(value, null, 2);
    result = result.replace(new RegExp(placeholder, 'g'), replacement);
  }

  return result;
}

// Main factory (~150 lines)
export function createPromptBackedTool<TIn, TOut>(
  options: PromptBackedToolOptions<TIn, TOut>,
): {
  name: string;
  description: string;
  inputSchema: z.ZodSchema<TIn>;
  outputSchema?: z.ZodSchema<TOut>;
  execute: (rawParams: unknown, deps: ToolDeps, context: ToolContext) => Promise<Result<TOut>>;
} {
  const { name, description, inputSchema, outputSchema, promptId, knowledge, policy } = options;

  return {
    name,
    description,
    inputSchema,
    outputSchema,

    async execute(rawParams: unknown, deps: ToolDeps, context: ToolContext): Promise<Result<TOut>> {
      const { logger } = deps;

      try {
        // 1. Validate input
        const params: TIn = inputSchema.parse(rawParams);
        logger.debug({ params, tool: name }, 'Executing prompt-backed tool');

        // 2. Load prompt from registry
        const prompt = await getPromptMetadata(promptId);
        if (!prompt) {
          throw new Error(`Prompt not found: ${promptId}`);
        }

        // 3. Get sampling config
        const sampling = await samplingConfigFor(promptId);

        // 4. Fetch knowledge (deterministic)
        let kbSnippets: unknown[] = [];
        if (knowledge?.category) {
          const query: Record<string, unknown> = {
            category: knowledge.category,
            ...knowledge.context?.(params),
            limit: knowledge.limit ?? 4,
          };
          const text = knowledge.textSelector?.(params);
          if (text) {
            query.text = text;
          }
          kbSnippets = await getKnowledgeRecommendations(query);
        }

        // 5. Get policy rules (deterministic ordering)
        const policyRules = policy
          ? getPolicyRules(
              policy.tool ?? name,
              (policy.extractor?.(params) ?? params) as Record<string, unknown>,
              { max: 4 },
            )
          : [];

        // 6. Fetch session data if sessionId is provided
        let sessionData: Record<string, unknown> = {};
        const paramsObj = params as Record<string, unknown>;
        if ('sessionId' in paramsObj && paramsObj.sessionId) {
          logger.info(
            { sessionId: paramsObj.sessionId, tool: name },
            'About to fetch session data',
          );

          const sessionResult = await getSession(paramsObj.sessionId as string, context);

          logger.info(
            {
              sessionId: paramsObj.sessionId,
              tool: name,
              sessionResultOk: sessionResult.ok,
              sessionResultError: sessionResult.ok ? null : sessionResult.error,
            },
            'Session fetch result',
          );

          if (sessionResult.ok && sessionResult.value.state) {
            sessionData = sessionResult.value.state as Record<string, unknown>;
            logger.info(
              {
                sessionId: paramsObj.sessionId,
                tool: name,
                sessionKeys: Object.keys(sessionData),
                hasAnalyzeRepo: 'analyzeRepoResult' in sessionData,
                analyzeRepoKeys: sessionData['analyzeRepoResult']
                  ? Object.keys(sessionData['analyzeRepoResult'] as object)
                  : [],
                analyzeRepoData: sessionData['analyzeRepoResult']
                  ? JSON.stringify(sessionData['analyzeRepoResult']).substring(0, 200)
                  : null,
              },
              'Loaded session data',
            );
            // If we have analyzeRepoResult data, extract key fields for the prompt
            if (sessionData['analyzeRepoResult']) {
              const analyzeData = sessionData['analyzeRepoResult'] as Record<string, unknown>;
              // Make analyze-repo data available at top level for easier template access
              if (analyzeData.language) sessionData.detectedLanguage = analyzeData.language;
              if (analyzeData.languageVersion)
                sessionData.languageVersion = analyzeData.languageVersion;
              if (analyzeData.framework) sessionData.detectedFramework = analyzeData.framework;
              if (analyzeData.buildSystem) {
                sessionData.buildSystem = analyzeData.buildSystem;
                // Extract buildSystemType for template compatibility
                const buildSys = analyzeData.buildSystem as Record<string, unknown>;
                if (buildSys?.type) sessionData.buildSystemType = buildSys.type;
              }

              logger.info(
                {
                  sessionId: paramsObj.sessionId,
                  tool: name,
                  extractedLanguage: sessionData.detectedLanguage,
                  extractedFramework: sessionData.detectedFramework,
                  extractedVersion: sessionData.languageVersion,
                },
                'Extracted session data for prompt variables',
              );
            } else {
              logger.warn(
                { sessionId: paramsObj.sessionId, tool: name },
                'Session data found but no analyzeRepoResult',
              );
            }
          } else {
            logger.warn(
              {
                sessionId: paramsObj.sessionId,
                tool: name,
                ok: sessionResult.ok,
                error: sessionResult.ok ? null : sessionResult.error,
              },
              'Failed to load session data',
            );
          }
        } else {
          logger.info({ tool: name }, 'No sessionId provided, skipping session data fetch');
        }

        // 7. Build prompt with all context including session data
        const variables = {
          ...sessionData, // Session data first
          ...(params as Record<string, unknown>), // Then params to override
          knowledge: kbSnippets,
          policy: policyRules,
        };

        // Debug: Log session data and variables for dockerfile-generation
        if (promptId === 'dockerfile-generation') {
          logger.info(
            {
              sessionDataKeys: Object.keys(sessionData),
              hasAnalyzeRepoResult: !!sessionData['analyzeRepoResult'],
              analyzeRepoResult: sessionData['analyzeRepoResult'],
              detectedLanguage: (variables as any).detectedLanguage,
              detectedFramework: (variables as any).detectedFramework,
              allVariableKeys: Object.keys(variables),
            },
            'Session data and variables for dockerfile generation',
          );
        }

        const resolved = buildPromptFromTemplate(prompt, variables);
        const version =
          'version' in prompt && typeof prompt.version === 'string' ? prompt.version : '1.0.0';
        const provenance = makeProvenance(promptId, resolved, version);

        // Log the actual prompt being sent to AI
        logger.info(
          {
            tool: name,
            promptId,
            detectedLanguage: (variables as any).detectedLanguage,
            detectedFramework: (variables as any).detectedFramework,
            buildSystemType: (variables as any).buildSystemType,
            promptPreview: `${resolved.substring(0, 500)}...`,
            fullPromptLength: resolved.length,
          },
          'Sending prompt to AI',
        );

        // For dockerfile generation, log more of the prompt to see variable substitution
        if (promptId === 'dockerfile-generation') {
          const contextSection = resolved.substring(
            resolved.indexOf('**Context:**'),
            resolved.indexOf('**Context:**') + 1000,
          );
          logger.info(
            {
              tool: name,
              contextSection: contextSection || 'Context section not found',
            },
            'Dockerfile generation context section',
          );
        }

        // Log provenance at INFO level (always)
        logger.info({ provenance, tool: name }, 'Executing AI prompt');

        // 8. Call model with simple retry (always 2 attempts with fixed delays)
        // Check if prompt expects JSON output
        const outputFormat = 'format' in prompt && prompt.format === 'json' ? 'json' : 'text';
        const raw = await withRetry(
          () =>
            runHostAssist(resolved, context, {
              maxTokens: sampling.maxTokens ?? 2048,
              stopSequences: sampling.stopSequences ?? [],
              outputFormat,
            }),
          2, // Always use 2 attempts, ignore maxRetries
          logger,
        );

        // 9. Parse, repair once if needed, validate, reject novel fields
        const adaptedContext = context
          ? {
              sampling: {
                createMessage: async (req: unknown) => {
                  const response = await context.sampling.createMessage(req as SamplingRequest);
                  return { content: response.content.map((c) => ({ text: c.text })) };
                },
              },
            }
          : undefined;
        const value = await parseAndValidateJson(raw, outputSchema, logger, adaptedContext);

        // 10. Include provenance if env flag set
        const includeProvenance = shouldIncludeProvenance();
        const result = includeProvenance ? ({ ...value, _provenance: provenance } as TOut) : value;

        return Success(result);
      } catch (err) {
        const msg = extractErrorMessage(err);
        logger.error({ err: msg, tool: name }, 'Prompt-backed tool failed');
        return Failure(msg);
      }
    },
  };
}

// Helper to create a simple text-output tool
export function createTextGenerationTool<TIn>(
  options: Omit<PromptBackedToolOptions<TIn, { content: string }>, 'outputSchema'>,
): ReturnType<typeof createPromptBackedTool<TIn, { content: string }>> {
  return createPromptBackedTool({
    ...options,
    outputSchema: z.object({
      content: z.string(),
    }),
  });
}

// Helper to create a structured JSON-output tool
export function createStructuredTool<TIn, TOut extends z.ZodObject<z.ZodRawShape>>(
  options: PromptBackedToolOptions<TIn, TOut>,
): ReturnType<typeof createPromptBackedTool<TIn, TOut>> {
  return createPromptBackedTool(options);
}

// Export types for tools
export type { Result } from '@types';
