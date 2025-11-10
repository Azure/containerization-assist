/**
 * Policy-based knowledge filtering types
 *
 * These types define the interface between policy evaluation and knowledge matching,
 * allowing policies to control which knowledge snippets are surfaced and how they're weighted.
 */

import type { KnowledgeCategory } from './types';

/**
 * Knowledge filter configuration returned by policy
 *
 * Policies can exclude specific snippets, boost/suppress weights,
 * and adjust category priorities based on context.
 */
export interface KnowledgeFilter {
  /**
   * Snippet IDs to exclude from matching
   * These snippets will be filtered out before scoring
   */
  excludeSnippets?: string[];

  /**
   * Weight multipliers for specific snippets
   * Key: snippet ID, Value: multiplier (0.5 = half weight, 2.0 = double weight)
   * Multiplier of 0 is equivalent to exclusion
   */
  snippetWeights?: Record<string, number>;

  /**
   * Category weight multipliers
   * Applies to all snippets in the category
   * Example: { security: 2.0, optimization: 0.5 }
   */
  categoryWeights?: Record<KnowledgeCategory, number>;

  /**
   * Tag-based weight multipliers
   * Applies to all snippets with matching tags
   * Example: { "distroless": 1.5, "alpine": 1.2 }
   */
  tagWeights?: Record<string, number>;

  /**
   * Registry filters for base-image knowledge
   * Only snippets recommending these registries will be included
   * Example: ["mcr.microsoft.com", "gcr.io"]
   */
  allowedRegistries?: string[];

  /**
   * Base image category filters
   * Only snippets in these categories will be included
   * Example: ["official", "distroless"]
   */
  allowedBaseImageCategories?: string[];

  /**
   * Minimum snippet score threshold
   * Snippets below this score will be excluded (applied after weight adjustments)
   */
  minScore?: number;

  /**
   * Maximum number of snippets to return (overrides query limit)
   */
  maxSnippets?: number;
}

/**
 * Input context for knowledge filter policy queries
 */
export interface KnowledgeFilterContext extends Record<string, unknown> {
  /** Environment (development, production, staging, etc.) */
  environment?: string;

  /** Tool requesting knowledge (generate-dockerfile, fix-dockerfile, etc.) */
  tool?: string;

  /** Programming language */
  language?: string;

  /** Programming language version */
  languageVersion?: string;

  /** Framework */
  framework?: string;

  /** Knowledge category being queried */
  category?: KnowledgeCategory;

  /** Tags from the query */
  tags?: string[];

  /** Detected dependencies */
  dependencies?: string[];
}

/**
 * Result of applying policy filters to knowledge matches
 */
export interface FilteredKnowledgeResult {
  /** Filtered and weighted snippet IDs that were excluded */
  excluded: string[];

  /** Snippet IDs that had weights boosted */
  boosted: string[];

  /** Snippet IDs that had weights reduced */
  reduced: string[];

  /** Total snippets processed */
  totalProcessed: number;

  /** Total snippets returned */
  totalReturned: number;

  /** Whether policy filtering was applied */
  policyApplied: boolean;
}
