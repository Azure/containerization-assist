/**
 * N-Best Sampling Runner for AI Enhancements
 *
 * Implements multi-candidate generation with scoring to improve AI output quality.
 *
 * Key features:
 * - Generates multiple candidates and selects the highest-scoring one
 * - Early stopping when a candidate meets the score threshold
 * - Configurable model preferences and generation parameters
 * - Structured logging for debugging and analysis
 */

import type { ToolContext, SamplingRequest, SamplingResponse } from '@/mcp/context';
import { type SamplingCandidate } from '@/lib/sampling';
import { Success, Failure, type Result } from '@/types';
import { extractErrorMessage } from '@/lib/error-utils';
import { planToRunnerOptions, type SamplingPlan } from '@/mcp/ai/sampling-plan';
import { SAMPLING_CONFIG } from '@/config/sampling';
import { SCORING_CONFIG } from '@/config/scoring';
import { TOKEN_CONFIG } from '@/config/tokens';
import crypto from 'node:crypto';

export interface GenerateOptions {
  /** Number of candidates to generate (default balanced) */
  count?: number;
  /** Score threshold for early stopping (default high quality) */
  stopAt?: number;
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
  /** Return all candidates for debugging (default false) */
  returnAll?: boolean;
}

export interface SamplingResult {
  /** The generated text content */
  text: string;
  /** The winning candidate with metadata */
  winner: SamplingCandidate;
  /** All candidates if returnAll=true */
  all?: SamplingCandidate[];
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
 * Sample multiple candidates using N-best sampling with scoring and early stopping
 *
 * @param ctx - Tool context for AI sampling and logging
 * @param buildSamplingRequest - Function to build sampling request for each attempt
 * @param score - Scoring function that takes text and returns numeric score or score breakdown
 * @param opts - Generation options
 * @returns Result containing the best candidate and metadata
 */
export async function sampleWithRerank(
  ctx: ToolContext,
  buildSamplingRequest: (attemptIndex: number) => Promise<SamplingRequest>,
  score: (text: string) => number | Record<string, number>,
  opts: GenerateOptions = {},
): Promise<Result<SamplingResult>> {
  const config = {
    count: opts.count ?? SAMPLING_CONFIG.CANDIDATES.BALANCED,
    stopAt: opts.stopAt ?? SCORING_CONFIG.THRESHOLDS.EXCELLENT,
    maxTokens: opts.maxTokens ?? TOKEN_CONFIG.STANDARD,
    stopSequences: opts.stopSequences ?? ['```', '\n\n```', '\n\n# ', '\n\n---'],
    modelPreferences: opts.modelPreferences,
    returnAll: opts.returnAll ?? false,
  };

  const startTime = Date.now();

  try {
    ctx.logger.info(
      {
        candidateCount: config.count,
        stopAtScore: config.stopAt,
        maxTokens: config.maxTokens,
      },
      'Starting N-best sampling',
    );

    const candidates: Array<{
      id: string;
      text: string;
      score: number;
      scoreBreakdown: Record<string, number>;
      usage?: { inputTokens?: number; outputTokens?: number; totalTokens?: number };
      model?: string;
    }> = [];

    const totalUsage = { inputTokens: 0, outputTokens: 0, totalTokens: 0 };
    let lastModel: string | undefined;

    // Generate candidates
    for (let i = 0; i < config.count; i++) {
      try {
        const req = await buildSamplingRequest(i);
        const enhancedReq: SamplingRequest = {
          ...req,
          maxTokens: req.maxTokens ?? config.maxTokens,
          stopSequences: req.stopSequences ?? config.stopSequences,
          ...(config.modelPreferences && { modelPreferences: config.modelPreferences }),
        };

        ctx.logger.debug(
          {
            attempt: i + 1,
            messageCount: req.messages.length,
            includeContext: req.includeContext,
          },
          'Generating candidate',
        );

        // Call MCP host for AI generation
        const res: SamplingResponse = await ctx.sampling.createMessage(enhancedReq);
        const text = res.content?.[0]?.text?.trim() ?? '';

        if (!text) {
          ctx.logger.warn({ attempt: i + 1 }, 'Empty AI response, skipping');
          continue;
        }

        // Score the candidate
        const scoreResult = score(text);
        const candidateScore =
          typeof scoreResult === 'number'
            ? scoreResult
            : Object.values(scoreResult).reduce((a, b) => a + b, 0) /
              Object.keys(scoreResult).length;

        const scoreBreakdown =
          typeof scoreResult === 'object' ? scoreResult : { overall: scoreResult };

        // Track usage if available
        if (res.metadata?.usage) {
          totalUsage.inputTokens += res.metadata.usage.inputTokens || 0;
          totalUsage.outputTokens += res.metadata.usage.outputTokens || 0;
          totalUsage.totalTokens += res.metadata.usage.totalTokens || 0;
        }

        // Track model
        if (res.metadata?.model) {
          lastModel = res.metadata.model;
        }

        candidates.push({
          id: crypto.randomUUID(),
          text,
          score: candidateScore,
          scoreBreakdown,
          ...(res.metadata?.usage && { usage: res.metadata.usage }),
          ...(res.metadata?.model && { model: res.metadata.model }),
        });

        ctx.logger.debug(
          {
            attempt: i + 1,
            score: candidateScore,
            scoreBreakdown,
            textLength: text.length,
            preview: text.slice(0, SCORING_CONFIG.QUALITY.PREVIEW_LENGTH).replace(/\n/g, ' '),
          },
          'Candidate generated and scored',
        );

        // Early stopping
        if (candidateScore >= config.stopAt) {
          ctx.logger.info(
            {
              score: candidateScore,
              attempt: i + 1,
              stoppedEarly: true,
            },
            'Early stop triggered - score threshold reached',
          );
          break;
        }
      } catch (error) {
        ctx.logger.warn(
          {
            attempt: i + 1,
            error: extractErrorMessage(error),
          },
          'Failed to generate candidate, continuing',
        );
        // Continue with next candidate
      }
    }

    // Check if we have any valid candidates
    if (candidates.length === 0) {
      return Failure('No valid candidates generated - all attempts failed');
    }

    // Sort candidates by score (highest first)
    const sorted = candidates.sort((a, b) => b.score - a.score);
    const winner = sorted[0];

    if (!winner) {
      return Failure('No candidates available after sorting');
    }

    // Convert to SamplingCandidate format
    const winnerCandidate: SamplingCandidate = {
      id: winner.id,
      content: winner.text,
      score: winner.score,
      scoreBreakdown: winner.scoreBreakdown,
      rank: 1,
    };

    const allCandidates: SamplingCandidate[] = config.returnAll
      ? sorted.map((c, index) => ({
          id: c.id,
          content: c.text,
          score: c.score,
          scoreBreakdown: c.scoreBreakdown,
          rank: index + 1,
        }))
      : [];

    const duration = Date.now() - startTime;

    ctx.logger.info(
      {
        candidatesGenerated: candidates.length,
        winnerScore: winner.score,
        duration,
        stoppedEarly: candidates.length < config.count,
      },
      'N-best sampling completed',
    );

    return Success({
      text: winner.text,
      winner: winnerCandidate,
      ...(config.returnAll && { all: allCandidates }),
      ...(totalUsage.totalTokens > 0 && { usage: totalUsage }),
      ...(lastModel && { model: lastModel }),
    });
  } catch (error) {
    const duration = Date.now() - startTime;
    ctx.logger.error(
      {
        duration,
        error: extractErrorMessage(error),
        candidateCount: config.count,
      },
      'sampleWithRerank failed',
    );
    return Failure(`Sampling failed: ${extractErrorMessage(error)}`);
  }
}

/**
 * Simple scoring function for general content quality
 * Used as a fallback when content-specific scoring isn't available
 */
export function scoreGenericContent(text: string): number {
  let score = 0;

  // Basic content quality checks
  if (text.length > SCORING_CONFIG.QUALITY.MIN_REASONABLE_LENGTH) score += 20; // Has reasonable length
  if (text.includes('\n')) score += 10; // Multi-line content
  if (!/lorem ipsum/i.test(text)) score += 20; // Not placeholder text
  if (!/TODO|FIXME|XXX/i.test(text)) score += 15; // No development markers
  if (!/Error|Exception|Failed/i.test(text)) score += 15; // No error indicators

  // Structure indicators
  if (/^\s*#/.test(text)) score += 10; // Has headers (markdown)
  if (/```/.test(text)) score += 10; // Has code blocks
  if (text.split('\n').length >= SCORING_CONFIG.QUALITY.MIN_MULTILINE_STRUCTURE) score += 10; // Multi-line structure

  return Math.min(score, SCORING_CONFIG.THRESHOLDS.PERFECT);
}

/**
 * Sample using a semantic sampling plan with structured configuration
 *
 * @param ctx - Tool context for AI sampling and logging
 * @param buildSamplingRequest - Function to build sampling request for each attempt
 * @param score - Scoring function that takes text and returns numeric score or score breakdown
 * @param plan - Semantic sampling plan with intent-driven configuration
 * @returns Result containing the best candidate and metadata
 */
export async function sampleWithPlan(
  ctx: ToolContext,
  buildSamplingRequest: (attemptIndex: number) => Promise<SamplingRequest>,
  score: (text: string) => number | Record<string, number>,
  plan: SamplingPlan,
): Promise<Result<SamplingResult>> {
  // Convert plan to GenerateOptions
  const options = planToRunnerOptions(plan);

  ctx.logger.debug(
    {
      planKind: plan.kind,
      generatedOptions: options,
    },
    'Using semantic sampling plan',
  );

  // Use existing sampleWithRerank with converted options
  return sampleWithRerank(ctx, buildSamplingRequest, score, options);
}
