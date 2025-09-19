/**
 * Integrated Configuration-Driven Scoring System
 *
 * Stateless scoring system without global state.
 * Each consumer creates their own instance.
 */

import type { Logger } from 'pino';
import type { ToolContext } from '@/mcp/context';
import { Result, Success, Failure } from '@/types';
import { createConfigurationManager } from '../sampling-config';
import { createConfigScoringEngine } from './internal/config-scoring-engine';
import { createStrategyResolver, type StrategyContext } from '../strategy-resolver';
import type { AIGenerateOptions } from '@/mcp/tool-ai-generation';
import type { SamplingCandidate } from '@/lib/sampling';
import { extractErrorMessage } from '../error-utils';

export type { SamplingCandidate };

/**
 * Scoring result type
 */
export interface ScoringResult {
  total: number;
  breakdown: Record<string, number>;
  categoryScores?: Record<string, number>;
  matchedRules?: string[];
  appliedPenalties?: string[];
}

/**
 * Scoring engine interface
 */
export interface ScoringEngine {
  scoreContent(content: string, profileName: string): Result<ScoringResult>;
  scoreCandidates(
    candidates: string[],
    expectation: AIGenerateOptions['expectation'],
    environment?: string,
    logger?: Logger,
  ): Promise<Result<SamplingCandidate[]>>;
  getStrategies(
    contentType: string,
    context: ToolContext & StrategyContext,
    logger?: Logger,
  ): Result<string[]>;
  getFormattedStrategy(strategy: string, variables: Record<string, string>): Result<string>;
}

/**
 * Create a new scoring engine instance (stateless factory)
 */
export async function createScoringEngine(configPath?: string): Promise<ScoringEngine> {
  const configManager = createConfigurationManager(configPath);
  const scoringEngine = createConfigScoringEngine(process.env.NODE_ENV === 'development');
  const strategyResolver = createStrategyResolver(configManager);

  // Load configuration eagerly
  const loadResult = await configManager.loadConfiguration();
  if (!loadResult.ok) {
    throw new Error(`Failed to load configuration: ${loadResult.error}`);
  }

  return {
    scoreContent(content: string, profileName: string): Result<ScoringResult> {
      const config = configManager.resolveForEnvironment(process.env.NODE_ENV || 'development');
      const profile = config.scoring[profileName];

      if (!profile) {
        return Failure(`No scoring profile found for: ${profileName}`);
      }

      const result = scoringEngine.score(content, profile);
      if (!result.ok) {
        return Failure(result.error);
      }

      return Success({
        total: result.value.total,
        breakdown: result.value.breakdown,
        categoryScores: result.value.categoryScores,
        matchedRules: result.value.matchedRules,
        appliedPenalties: result.value.appliedPenalties,
      });
    },

    async scoreCandidates(
      candidates: string[],
      expectation: AIGenerateOptions['expectation'],
      environment: string = 'development',
      logger?: Logger,
    ): Promise<Result<SamplingCandidate[]>> {
      try {
        const config = configManager.resolveForEnvironment(environment);

        // Map expectation to profile name
        let profileName = 'generic';
        if (expectation === 'dockerfile') {
          profileName = 'dockerfile';
        } else if (expectation === 'yaml') {
          profileName = 'k8s';
        }

        const profile = config.scoring[profileName];
        if (!profile) {
          return Failure(`No scoring profile found for: ${profileName}`);
        }

        // Score all candidates
        const scoredCandidates: SamplingCandidate[] = [];

        for (let i = 0; i < candidates.length; i++) {
          const content = candidates[i];
          if (!content) continue;

          const scoreResult = scoringEngine.score(content, profile);

          if (scoreResult.ok) {
            const { total, breakdown, matchedRules } = scoreResult.value;

            scoredCandidates.push({
              id: `candidate-${i + 1}`,
              content,
              score: total,
              scoreBreakdown: breakdown,
            });

            if (logger) {
              logger.debug(
                {
                  candidateId: i + 1,
                  score: total,
                  matchedRules: matchedRules.length,
                  profile: profileName,
                  environment,
                },
                'Candidate scored',
              );
            }
          } else {
            // Fallback to minimum score
            scoredCandidates.push({
              id: `candidate-${i + 1}`,
              content,
              score: profile.base_score,
              scoreBreakdown: { fallback: profile.base_score },
            });

            if (logger) {
              logger.warn(
                {
                  candidateId: i + 1,
                  error: scoreResult.error,
                },
                'Candidate scoring failed, using fallback',
              );
            }
          }
        }

        return Success(scoredCandidates);
      } catch (error) {
        if (logger) {
          logger.error({ error }, 'Candidate scoring error');
        }
        return Failure(`Candidate scoring failed: ${extractErrorMessage(error)}`);
      }
    },

    getStrategies(
      contentType: string,
      context: ToolContext & StrategyContext,
      logger?: Logger,
    ): Result<string[]> {
      try {
        const strategies = strategyResolver.resolveStrategy(contentType, context);

        if (logger) {
          logger.debug(
            {
              contentType,
              environment: context.environment,
              strategiesFound: strategies.length,
            },
            'Strategies resolved from configuration',
          );
        }

        return Success(strategies);
      } catch (error) {
        return Failure(`Strategy resolution failed: ${extractErrorMessage(error)}`);
      }
    },

    getFormattedStrategy(strategy: string, variables: Record<string, string>): Result<string> {
      return Success(strategyResolver.getFormattedStrategy(strategy, variables));
    },
  };
}

/**
 * Legacy compatibility functions
 * These maintain backward compatibility but create instances on demand
 */

// Create a default instance for backward compatibility
let defaultEngine: ScoringEngine | null = null;
let enginePromise: Promise<ScoringEngine> | null = null;

async function getDefaultEngine(): Promise<ScoringEngine> {
  if (!defaultEngine) {
    if (!enginePromise) {
      enginePromise = createScoringEngine();
    }
    defaultEngine = await enginePromise;
  }
  return defaultEngine;
}

/**
 * Score candidates using configuration-driven system (legacy)
 */
export async function scoreConfigCandidates(
  candidates: string[],
  expectation: AIGenerateOptions['expectation'],
  environment: string = 'development',
  logger?: Logger,
): Promise<Result<SamplingCandidate[]>> {
  const engine = await getDefaultEngine();
  return engine.scoreCandidates(candidates, expectation, environment, logger);
}

/**
 * Get sampling strategies from configuration (legacy)
 */
export async function getConfigStrategies(
  contentType: string,
  context: ToolContext & StrategyContext,
  logger?: Logger,
): Promise<Result<string[]>> {
  const engine = await getDefaultEngine();
  const result = engine.getStrategies(contentType, context, logger);
  return Success(result.ok ? result.value : []);
}

/**
 * Get formatted strategy with variable substitution (legacy)
 */
export async function getFormattedConfigStrategy(
  strategy: string,
  variables: Record<string, string>,
): Promise<Result<string>> {
  const engine = await getDefaultEngine();
  return engine.getFormattedStrategy(strategy, variables);
}
