/**
 * Integrated Configuration-Driven Scoring System
 *
 * Combines configuration loading, scoring, and strategy resolution
 */

import type { Logger } from 'pino';
import type { ToolContext } from '@mcp/context';
import { Result, Success, Failure } from '@types';
import { createConfigurationManager, type ConfigurationManagerInterface } from './sampling-config';
import { createConfigScoringEngine } from './config-scoring-engine';
import { createStrategyResolver, type StrategyContext } from './strategy-resolver';
import type { AIGenerateOptions } from '@mcp/tool-ai-generation';
import type { SamplingCandidate } from '@lib/sampling';
import { extractErrorMessage } from './error-utils';

// Global instances (initialized lazily)
let configManager: ConfigurationManagerInterface | null = null;
let scoringEngine: ReturnType<typeof createConfigScoringEngine> | null = null;
let strategyResolver: ReturnType<typeof createStrategyResolver> | null = null;
let isConfigLoaded = false;

/**
 * Initialize the configuration-driven scoring system
 */
export async function initializeConfigSystem(configPath?: string): Promise<Result<void>> {
  try {
    // Initialize configuration manager
    configManager = createConfigurationManager(configPath);

    // Load configuration
    const loadResult = await configManager.loadConfiguration();
    if (!loadResult.ok) {
      return Failure(`Failed to load configuration: ${loadResult.error}`);
    }

    // Initialize scoring engine and strategy resolver
    scoringEngine = createConfigScoringEngine(process.env.NODE_ENV === 'development');
    strategyResolver = createStrategyResolver(configManager);

    isConfigLoaded = true;
    return Success(undefined);
  } catch (error) {
    return Failure(`Configuration system initialization failed: ${extractErrorMessage(error)}`);
  }
}

/**
 * Ensure configuration system is initialized
 */
async function ensureInitialized(): Promise<Result<void>> {
  if (!isConfigLoaded) {
    return initializeConfigSystem();
  }
  return Success(undefined);
}

/**
 * Score candidates using configuration-driven system
 */
export async function scoreConfigCandidates(
  candidates: string[],
  expectation: AIGenerateOptions['expectation'],
  environment: string = 'development',
  logger?: Logger,
): Promise<Result<SamplingCandidate[]>> {
  const initResult = await ensureInitialized();
  if (!initResult.ok) {
    return Failure(initResult.error);
  }

  if (!configManager || !scoringEngine) {
    return Failure('Configuration system not properly initialized');
  }

  try {
    // Get environment-specific configuration
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
}

/**
 * Get sampling strategies from configuration
 */
export async function getConfigStrategies(
  contentType: string,
  context: ToolContext & StrategyContext,
  logger?: Logger,
): Promise<Result<string[]>> {
  const initResult = await ensureInitialized();
  if (!initResult.ok) {
    return Failure(initResult.error);
  }

  if (!strategyResolver) {
    return Failure('Strategy resolver not initialized');
  }

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
}

/**
 * Get formatted strategy with variable substitution
 */
export function getFormattedConfigStrategy(
  strategy: string,
  variables: Record<string, string>,
): Result<string> {
  if (!strategyResolver) {
    return Failure('Strategy resolver not initialized');
  }

  try {
    const formatted = strategyResolver.getFormattedStrategy(strategy, variables);
    return Success(formatted);
  } catch (error) {
    return Failure(`Strategy formatting failed: ${extractErrorMessage(error)}`);
  }
}

/**
 * Get configuration validation status
 */
export async function validateConfigurationSystem(): Promise<
  Result<{
    isValid: boolean;
    errors: string[];
    profilesLoaded: string[];
    environmentsLoaded: string[];
  }>
> {
  const initResult = await ensureInitialized();
  if (!initResult.ok) {
    return Success({
      isValid: false,
      errors: [initResult.error],
      profilesLoaded: [],
      environmentsLoaded: [],
    });
  }

  if (!configManager) {
    return Success({
      isValid: false,
      errors: ['Configuration manager not initialized'],
      profilesLoaded: [],
      environmentsLoaded: [],
    });
  }

  try {
    const config = configManager.getConfiguration();
    const validation = await configManager.validateConfiguration(config);

    return Success({
      isValid: validation.isValid,
      errors: validation.errors,
      profilesLoaded: Object.keys(config.scoring),
      environmentsLoaded: Object.keys(config.environments),
    });
  } catch (error) {
    return Success({
      isValid: false,
      errors: [`Validation failed: ${extractErrorMessage(error)}`],
      profilesLoaded: [],
      environmentsLoaded: [],
    });
  }
}

/**
 * Quick score for early stopping (using configuration)
 */
export async function quickConfigScore(
  content: string,
  expectation: AIGenerateOptions['expectation'],
  environment: string = 'development',
): Promise<number> {
  const initResult = await ensureInitialized();
  if (!initResult.ok) {
    // Fallback to simple heuristic
    return quickHeuristicScore(content, expectation);
  }

  if (!configManager || !scoringEngine) {
    return quickHeuristicScore(content, expectation);
  }

  try {
    const config = configManager.resolveForEnvironment(environment);

    let profileName = 'generic';
    if (expectation === 'dockerfile') {
      profileName = 'dockerfile';
    } else if (expectation === 'yaml') {
      profileName = 'k8s';
    }

    const profile = config.scoring[profileName];
    if (!profile) {
      return quickHeuristicScore(content, expectation);
    }

    const scoreResult = scoringEngine.score(content, profile);
    if (scoreResult.ok) {
      return scoreResult.value.total;
    }

    return quickHeuristicScore(content, expectation);
  } catch {
    return quickHeuristicScore(content, expectation);
  }
}

/**
 * Fallback heuristic scoring for quick evaluation
 */
function quickHeuristicScore(content: string, expectation?: string): number {
  let score = 50; // Base score

  if (expectation === 'dockerfile') {
    if (content.includes('FROM')) score += 20;
    if (content.includes('WORKDIR')) score += 10;
    if (content.includes('COPY') || content.includes('ADD')) score += 10;
    if (content.includes('USER')) score += 5;
    if (content.includes('--no-cache') || content.includes('--frozen-lockfile')) score += 5;
  } else if (expectation === 'yaml') {
    if (content.includes('apiVersion:')) score += 20;
    if (content.includes('kind:')) score += 10;
    if (content.includes('metadata:')) score += 10;
    if (content.includes('resources:')) score += 5;
  }

  return Math.min(score, 100);
}
