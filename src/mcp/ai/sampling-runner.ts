/**
 * Deterministic AI Sampling Runner
 *
 * Implements single-candidate generation with optional scoring for quality assurance.
 * Aligned with deterministic requirements for the single-operator containerization workflow.
 *
 * Key features:
 * - Single-candidate generation (count: 1) for deterministic behavior
 * - Optional scoring for quality logging and validation
 * - Configurable model preferences and generation parameters
 * - Structured logging for debugging and analysis
 */

import type { ToolContext, SamplingRequest, SamplingResponse } from '@/mcp/context';
import { Success, Failure, type Result } from '@/types';
import { extractErrorMessage } from '@/lib/error-utils';
import { TOKEN_CONFIG } from '@/config/tokens';

export interface GenerateOptions {
  /** Maximum tokens per candidate (default standard) */
  maxTokens?: number;
  /** Stop sequences to end generation */
  stopSequences?: string[];
  /** Model preferences for the request */
  modelPreferences?: {
    /** Hints about the type of response needed */
    hints?: Array<{ name: string }>;
    /** Cost optimization priority (0-1) */
    costPriority?: number;
    /** Speed optimization priority (0-1) */
    speedPriority?: number;
    /** Intelligence/quality priority (0-1) */
    intelligencePriority?: number;
  };
}

export interface SamplingResult {
  /** The generated text content */
  text: string;
  /** The score for the generated content (if scoring provided) */
  score?: number;
  /** Detailed score breakdown (if scoring provided) */
  scoreBreakdown?: Record<string, number>;
  /** Token usage statistics */
  usage?: {
    inputTokens?: number;
    outputTokens?: number;
    totalTokens?: number;
  };
  /** Model used for generation */
  model?: string;
}

/**
 * Deterministic single-candidate sampling with optional scoring
 *
 * Generates a single AI response for deterministic behavior. Optional scoring
 * is provided for quality validation and logging purposes only.
 *
 * @param ctx - Tool context for AI sampling and logging
 * @param buildSamplingRequest - Function to build sampling request
 * @param score - Optional scoring function for quality logging
 * @param opts - Generation options
 * @returns Result containing the generated content and metadata
 */
export async function sampleWithRerank(
  ctx: ToolContext,
  buildSamplingRequest: (attemptIndex: number) => Promise<SamplingRequest>,
  score?: (text: string) => number | Record<string, number>,
  opts: GenerateOptions = {},
): Promise<Result<SamplingResult>> {
  const config = {
    maxTokens: opts.maxTokens ?? TOKEN_CONFIG.STANDARD,
    stopSequences: opts.stopSequences ?? ['```', '\n\n```', '\n\n# ', '\n\n---'],
    modelPreferences: opts.modelPreferences,
  };

  const startTime = Date.now();

  try {
    ctx.logger.info(
      {
        maxTokens: config.maxTokens,
        deterministic: true,
      },
      'Starting deterministic AI sampling',
    );

    // Build the sampling request
    const req = await buildSamplingRequest(0);
    const enhancedReq: SamplingRequest = {
      ...req,
      maxTokens: req.maxTokens ?? config.maxTokens,
      stopSequences: req.stopSequences ?? config.stopSequences,
      ...(config.modelPreferences && { modelPreferences: config.modelPreferences }),
    };

    ctx.logger.debug(
      {
        messageCount: req.messages.length,
        includeContext: req.includeContext,
      },
      'Generating AI response',
    );

    // Call MCP host for AI generation
    const res: SamplingResponse = await ctx.sampling.createMessage(enhancedReq);
    const text = res.content?.[0]?.text?.trim() ?? '';

    if (!text) {
      return Failure('Empty response from AI');
    }

    const duration = Date.now() - startTime;

    // Optional scoring for quality logging
    let contentScore: number | undefined;
    let scoreBreakdown: Record<string, number> | undefined;

    if (score) {
      const scoreResult = score(text);
      contentScore =
        typeof scoreResult === 'number'
          ? scoreResult
          : Object.values(scoreResult).reduce((a, b) => a + b, 0) / Object.keys(scoreResult).length;

      scoreBreakdown = typeof scoreResult === 'object' ? scoreResult : { overall: scoreResult };

      ctx.logger.info(
        {
          score: contentScore,
          scoreBreakdown,
          textLength: text.length,
          duration,
        },
        'AI response generated and scored',
      );
    } else {
      ctx.logger.info(
        {
          textLength: text.length,
          duration,
        },
        'AI response generated',
      );
    }

    return Success({
      text,
      ...(contentScore !== undefined && { score: contentScore }),
      ...(scoreBreakdown && { scoreBreakdown }),
      ...(res.metadata?.usage && { usage: res.metadata.usage }),
      ...(res.metadata?.model && { model: res.metadata.model }),
    });
  } catch (error) {
    const duration = Date.now() - startTime;
    ctx.logger.error(
      {
        duration,
        error: extractErrorMessage(error),
      },
      'sampleWithRerank failed',
    );
    return Failure(`Sampling failed: ${extractErrorMessage(error)}`);
  }
}
