/**
 * Semantic Sampling Plans
 *
 * Provides semantic, intent-driven sampling configurations that replace
 * ad-hoc sampling options with meaningful plans and consistent behavior.
 */

import type { GenerateOptions } from '@/mcp/ai/sampling-runner';
import { SAMPLING_CONFIG } from '@/config/sampling';
import { SCORING_CONFIG } from '@/config/scoring';
import { TOKEN_CONFIG } from '@/config/tokens';

/**
 * Semantic sampling plan types with clear intent
 */
export type SamplingPlan =
  | { kind: 'fast'; maxTokens?: number }
  | {
      kind: 'balanced';
      candidates?: 3 | 5;
      stopAt?: number;
      maxTokens?: number;
    }
  | {
      kind: 'thorough';
      candidates?: 5 | 8;
      stopAt?: number;
      maxTokens?: number;
    };

/**
 * Sampling plan configuration with sensible defaults
 */
export interface SamplingPlanConfig {
  /** Plan type with semantic meaning */
  kind: 'fast' | 'balanced' | 'thorough';
  /** Override default candidate count */
  candidates?: number;
  /** Override default stop threshold */
  stopAt?: number;
  /** Override default max tokens */
  maxTokens?: number;
  /** Return all candidates for debugging */
  returnAll?: boolean;
}

/**
 * Pre-defined sampling plan configurations
 */
const PLAN_DEFAULTS = {
  fast: {
    candidates: SAMPLING_CONFIG.CANDIDATES.FAST,
    stopAt: SCORING_CONFIG.THRESHOLDS.FAST,
    maxTokens: TOKEN_CONFIG.STANDARD,
  },
  balanced: {
    candidates: SAMPLING_CONFIG.CANDIDATES.BALANCED,
    stopAt: SCORING_CONFIG.THRESHOLDS.STANDARD,
    maxTokens: TOKEN_CONFIG.STANDARD,
  },
  thorough: {
    candidates: SAMPLING_CONFIG.CANDIDATES.THOROUGH,
    stopAt: SCORING_CONFIG.THRESHOLDS.HIGH_QUALITY,
    maxTokens: TOKEN_CONFIG.EXTENDED,
  },
} as const;

/**
 * Create a semantic sampling plan with intent-based configuration
 *
 * @param intent - The sampling strategy intent
 * @param overrides - Optional overrides for specific parameters
 * @returns Configured sampling plan
 */
export function createSamplingPlan(
  intent: 'fast' | 'balanced' | 'thorough',
  overrides: Partial<SamplingPlanConfig> = {},
): SamplingPlan {
  const defaults = PLAN_DEFAULTS[intent];

  switch (intent) {
    case 'fast':
      return {
        kind: 'fast',
        maxTokens: overrides.maxTokens ?? defaults.maxTokens,
      };

    case 'balanced':
      return {
        kind: 'balanced',
        candidates: (overrides.candidates as 3 | 5) ?? (defaults.candidates as 3 | 5),
        stopAt: overrides.stopAt ?? defaults.stopAt,
        maxTokens: overrides.maxTokens ?? defaults.maxTokens,
      };

    case 'thorough':
      return {
        kind: 'thorough',
        candidates: (overrides.candidates as 5 | 8) ?? (defaults.candidates as 5 | 8),
        stopAt: overrides.stopAt ?? defaults.stopAt,
        maxTokens: overrides.maxTokens ?? defaults.maxTokens,
      };

    default:
      throw new Error(`Unknown sampling intent: ${intent}`);
  }
}

/**
 * Convert semantic sampling plan to GenerateOptions for the runner
 *
 * @param plan - The semantic sampling plan
 * @param additionalOptions - Additional options to merge
 * @returns GenerateOptions for sampling-runner
 */
export function planToRunnerOptions(
  plan: SamplingPlan,
  additionalOptions: Partial<GenerateOptions> = {},
): GenerateOptions {
  const baseOptions: GenerateOptions = {
    maxTokens: plan.maxTokens ?? TOKEN_CONFIG.STANDARD,
    ...additionalOptions,
  };

  switch (plan.kind) {
    case 'fast':
      return {
        ...baseOptions,
        count: SAMPLING_CONFIG.CANDIDATES.FAST,
        stopAt: SCORING_CONFIG.THRESHOLDS.FAST,
      };

    case 'balanced':
      return {
        ...baseOptions,
        count: plan.candidates ?? SAMPLING_CONFIG.CANDIDATES.BALANCED,
        stopAt: plan.stopAt ?? SCORING_CONFIG.THRESHOLDS.STANDARD,
      };

    case 'thorough':
      return {
        ...baseOptions,
        count: plan.candidates ?? SAMPLING_CONFIG.CANDIDATES.THOROUGH,
        stopAt: plan.stopAt ?? SCORING_CONFIG.THRESHOLDS.HIGH_QUALITY,
      };

    default:
      throw new Error(`Unknown plan kind: ${(plan as { kind: string }).kind}`);
  }
}

/**
 * Create context-aware sampling plan based on content type and requirements
 *
 * @param contentType - Type of content being processed
 * @param priority - Priority level (affects quality vs speed trade-off)
 * @param overrides - Optional parameter overrides
 * @returns Optimized sampling plan for the context
 */
export function createContextAwarePlan(
  contentType: 'validation' | 'knowledge' | 'enhancement' | 'generation',
  priority: 'speed' | 'balanced' | 'quality' = 'balanced',
  overrides: Partial<SamplingPlanConfig> = {},
): SamplingPlan {
  // Map content types to base plans
  const contentBasePlans: Record<string, 'fast' | 'balanced' | 'thorough'> = {
    validation: 'balanced', // Validation needs accuracy but not excessive candidates
    knowledge: 'thorough', // Knowledge enhancement benefits from thorough sampling
    enhancement: 'balanced', // Enhancement needs balance of quality and speed
    generation: 'thorough', // Generation benefits from multiple candidates
  };

  // Adjust plan based on priority
  let basePlan = contentBasePlans[contentType] || 'balanced';

  if (priority === 'speed') {
    basePlan = basePlan === 'thorough' ? 'balanced' : 'fast';
  } else if (priority === 'quality') {
    basePlan = basePlan === 'fast' ? 'balanced' : 'thorough';
  }

  return createSamplingPlan(basePlan, overrides);
}

/**
 * Get recommended sampling plan for specific use cases
 */
export const RECOMMENDED_PLANS = {
  /** Fast validation with single candidate */
  quickValidation: () => createSamplingPlan('fast'),

  /** Standard validation with moderate sampling */
  standardValidation: () => createSamplingPlan('balanced'),

  /** High-quality validation with extensive sampling */
  thoroughValidation: () => createSamplingPlan('thorough'),

  /** Knowledge enhancement with quality focus */
  knowledgeEnhancement: () =>
    createSamplingPlan('thorough', { stopAt: SCORING_CONFIG.THRESHOLDS.HIGH_QUALITY }),

  /** Content enhancement with balanced approach */
  contentEnhancement: () =>
    createSamplingPlan('balanced', { stopAt: SCORING_CONFIG.THRESHOLDS.STANDARD }),

  /** Development/debugging with all candidates returned */
  debugging: () => createSamplingPlan('balanced', { stopAt: SCORING_CONFIG.THRESHOLDS.FAST }),

  /** Production generation with high quality bar */
  productionGeneration: () =>
    createSamplingPlan('thorough', {
      candidates: SAMPLING_CONFIG.CANDIDATES.THOROUGH,
      stopAt: SCORING_CONFIG.THRESHOLDS.EXCELLENT,
      maxTokens: TOKEN_CONFIG.EXTENDED,
    }),
} as const;

/**
 * Utility to get plan description for logging/debugging
 */
export function describePlan(plan: SamplingPlan): string {
  switch (plan.kind) {
    case 'fast':
      return `Fast (${SAMPLING_CONFIG.CANDIDATES.FAST} candidate, ${plan.maxTokens || TOKEN_CONFIG.STANDARD} tokens)`;
    case 'balanced':
      return `Balanced (${plan.candidates || SAMPLING_CONFIG.CANDIDATES.BALANCED} candidates, stop@${plan.stopAt || SCORING_CONFIG.THRESHOLDS.STANDARD}, ${plan.maxTokens || TOKEN_CONFIG.STANDARD} tokens)`;
    case 'thorough':
      return `Thorough (${plan.candidates || SAMPLING_CONFIG.CANDIDATES.THOROUGH} candidates, stop@${plan.stopAt || SCORING_CONFIG.THRESHOLDS.HIGH_QUALITY}, ${plan.maxTokens || TOKEN_CONFIG.EXTENDED} tokens)`;
    default:
      return 'Unknown plan';
  }
}

/**
 * Type guard for sampling plans
 */
export function isSamplingPlan(value: unknown): value is SamplingPlan {
  if (typeof value !== 'object' || value === null) {
    return false;
  }

  const plan = value as { kind?: string };
  return plan.kind !== undefined && ['fast', 'balanced', 'thorough'].includes(plan.kind);
}
