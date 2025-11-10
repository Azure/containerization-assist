/**
 * Policy-Aware Knowledge Matcher
 *
 * Wraps the standard knowledge matcher with policy-based filtering and weighting.
 * Policies can exclude snippets, boost/suppress weights, and adjust category priorities
 * based on environment, tool context, and other factors.
 */

import { createLogger } from '@/lib/logger';
import type { RegoEvaluator } from '@/config/policy-rego';
import type { KnowledgeQuery, KnowledgeMatch, LoadedEntry } from './types';
import type {
  KnowledgeFilter,
  KnowledgeFilterContext,
  FilteredKnowledgeResult,
} from './policy-types';
import { findKnowledgeMatches } from './matcher';

const logger = createLogger().child({ module: 'policy-aware-matcher' });

/**
 * Package name for knowledge filter policy queries
 * This targets the 'result' rule within the knowledge_filter package
 */
const KNOWLEDGE_FILTER_PACKAGE = 'containerization.knowledge_filter.result';

/**
 * Query policy for knowledge filters
 *
 * @param policy - Rego policy evaluator
 * @param context - Context for the filter query
 * @returns Knowledge filter configuration or null if not defined
 */
async function queryKnowledgeFilter(
  policy: RegoEvaluator,
  context: KnowledgeFilterContext,
): Promise<KnowledgeFilter | null> {
  try {
    logger.debug({ context }, 'Querying policy for knowledge filters');

    const startTime = Date.now();

    // Query the policy for knowledge filter configuration
    const filter = await policy.queryConfig<KnowledgeFilter>(
      KNOWLEDGE_FILTER_PACKAGE,
      context,
    );

    const elapsed = Date.now() - startTime;

    if (filter) {
      logger.debug(
        {
          elapsed,
          excludeCount: filter.excludeSnippets?.length ?? 0,
          snippetWeightCount: Object.keys(filter.snippetWeights ?? {}).length,
          categoryWeightCount: Object.keys(filter.categoryWeights ?? {}).length,
          tagWeightCount: Object.keys(filter.tagWeights ?? {}).length,
          allowedRegistries: filter.allowedRegistries?.length ?? 0,
        },
        'Knowledge filter policy query completed',
      );
    } else {
      logger.debug({ elapsed }, 'No knowledge filter policy defined');
    }

    return filter;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    logger.warn({ error: message }, 'Failed to query knowledge filter policy, proceeding without filters');
    return null;
  }
}

/**
 * Apply policy exclusion filters to entries
 *
 * @param entries - Knowledge entries to filter
 * @param filter - Policy filter configuration
 * @returns Filtered entries and excluded IDs
 */
function applyExclusionFilters(
  entries: LoadedEntry[],
  filter: KnowledgeFilter,
): { filtered: LoadedEntry[]; excluded: string[] } {
  const excluded: string[] = [];
  const excludeSet = new Set(filter.excludeSnippets ?? []);

  // Apply registry filters if specified
  let registryFiltered = entries;
  if (filter.allowedRegistries && filter.allowedRegistries.length > 0) {
    registryFiltered = entries.filter((entry) => {
      // Only filter base-image related entries
      if (!entry.tags?.some((tag) => ['base-image', 'from', 'image'].includes(tag.toLowerCase()))) {
        return true; // Keep non-base-image entries
      }

      // Check if the entry recommends an allowed registry
      // Convert to lowercase once for performance
      const recommendationLower = entry.recommendation.toLowerCase();
      const exampleLower = entry.example?.toLowerCase();
      const allowedRegistries = filter.allowedRegistries ?? [];
      const hasAllowedRegistry = allowedRegistries.some((registry) => {
        const registryLower = registry.toLowerCase();
        return recommendationLower.includes(registryLower) ||
          exampleLower?.includes(registryLower);
      });

      if (!hasAllowedRegistry) {
        excluded.push(entry.id);
      }

      return hasAllowedRegistry;
    });
  }

  // Apply base image category filters if specified
  let categoryFiltered = registryFiltered;
  if (filter.allowedBaseImageCategories && filter.allowedBaseImageCategories.length > 0) {
    categoryFiltered = registryFiltered.filter((entry) => {
      // Only filter base-image related entries
      if (!entry.tags?.some((tag) => ['base-image', 'from', 'image'].includes(tag.toLowerCase()))) {
        return true; // Keep non-base-image entries
      }

      // Check if the entry has an allowed base image category tag
      const allowedCategories = filter.allowedBaseImageCategories ?? [];
      const hasAllowedCategory = entry.tags?.some((tag) =>
        allowedCategories.includes(tag),
      );

      if (!hasAllowedCategory) {
        excluded.push(entry.id);
      }

      return hasAllowedCategory;
    });
  }

  // Apply snippet exclusion list
  const filtered = categoryFiltered.filter((entry) => {
    if (excludeSet.has(entry.id)) {
      excluded.push(entry.id);
      return false;
    }
    return true;
  });

  return { filtered, excluded };
}

/**
 * Apply policy weight multipliers to matches
 *
 * @param matches - Knowledge matches to adjust
 * @param filter - Policy filter configuration
 * @returns Adjusted matches and boost/reduce tracking
 */
function applyWeightMultipliers(
  matches: KnowledgeMatch[],
  filter: KnowledgeFilter,
): {
  adjusted: KnowledgeMatch[];
  boosted: string[];
  reduced: string[];
} {
  const boosted: string[] = [];
  const reduced: string[] = [];

  const adjusted = matches.map((match) => {
    let multiplier = 1.0;
    const appliedMultipliers: string[] = [];

    // Apply snippet-specific weight
    if (filter.snippetWeights?.[match.entry.id] !== undefined) {
      const snippetMultiplier = filter.snippetWeights[match.entry.id] ?? 1.0;
      multiplier *= snippetMultiplier;
      appliedMultipliers.push(`snippet:${snippetMultiplier}`);
    }

    // Apply category weight
    if (filter.categoryWeights?.[match.entry.category] !== undefined) {
      const categoryMultiplier = filter.categoryWeights[match.entry.category];
      multiplier *= categoryMultiplier;
      appliedMultipliers.push(`category:${categoryMultiplier}`);
    }

    // Apply tag weights
    if (filter.tagWeights && match.entry.tags) {
      for (const tag of match.entry.tags) {
        if (filter.tagWeights[tag] !== undefined) {
          const tagMultiplier = filter.tagWeights[tag];
          multiplier *= tagMultiplier;
          appliedMultipliers.push(`tag(${tag}):${tagMultiplier}`);
        }
      }
    }

    // Track boost/reduce
    if (multiplier > 1.0) {
      boosted.push(match.entry.id);
    } else if (multiplier < 1.0 && multiplier > 0) {
      reduced.push(match.entry.id);
    }

    // Apply multiplier
    const newScore = match.score * multiplier;

    if (appliedMultipliers.length > 0) {
      logger.debug(
        {
          entryId: match.entry.id,
          originalScore: match.score,
          multiplier,
          newScore,
          appliedMultipliers,
        },
        'Applied weight multipliers to snippet',
      );
    }

    return {
      ...match,
      score: newScore,
    };
  });

  return { adjusted, boosted, reduced };
}

/**
 * Apply minimum score threshold
 *
 * @param matches - Matches to filter
 * @param minScore - Minimum score threshold
 * @returns Filtered matches
 */
function applyMinScore(
  matches: KnowledgeMatch[],
  minScore: number,
): KnowledgeMatch[] {
  return matches.filter((match) => match.score >= minScore);
}

/**
 * Find knowledge matches with policy-based filtering and weighting
 *
 * This function wraps the standard knowledge matcher with policy intelligence,
 * allowing policies to control which snippets are surfaced and how they're prioritized.
 *
 * @param entries - All knowledge entries
 * @param query - Knowledge query
 * @param policy - Optional policy evaluator for filtering/weighting
 * @returns Filtered and weighted matches with metadata
 */
export async function findPolicyAwareKnowledgeMatches(
  entries: LoadedEntry[],
  query: KnowledgeQuery,
  policy?: RegoEvaluator,
): Promise<{
  matches: KnowledgeMatch[];
  filterResult: FilteredKnowledgeResult;
}> {
  // If no policy, use standard matcher
  if (!policy) {
    const matches = findKnowledgeMatches(entries, query);
    return {
      matches,
      filterResult: {
        excluded: [],
        boosted: [],
        reduced: [],
        totalProcessed: entries.length,
        totalReturned: matches.length,
        policyApplied: false,
      },
    };
  }

  const totalProcessed = entries.length;

  // Build context for policy query
  const context: KnowledgeFilterContext = {
    ...(query.environment && { environment: query.environment }),
    ...(query.tool && { tool: query.tool }),
    ...(query.language && { language: query.language }),
    ...(query.languageVersion && { languageVersion: query.languageVersion }),
    ...(query.framework && { framework: query.framework }),
    ...(query.category && { category: query.category }),
    ...(query.tags && { tags: query.tags }),
  };

  // Query policy for filters
  const filter = await queryKnowledgeFilter(policy, context);

  // If no filter returned, use standard matcher
  if (!filter) {
    const matches = findKnowledgeMatches(entries, query);
    return {
      matches,
      filterResult: {
        excluded: [],
        boosted: [],
        reduced: [],
        totalProcessed,
        totalReturned: matches.length,
        policyApplied: false,
      },
    };
  }

  // Apply exclusion filters BEFORE matching (more efficient)
  const { filtered: filteredEntries, excluded } = applyExclusionFilters(entries, filter);

  logger.debug(
    {
      originalCount: entries.length,
      filteredCount: filteredEntries.length,
      excludedCount: excluded.length,
    },
    'Applied exclusion filters',
  );

  // Run standard matching on filtered entries
  let matches = findKnowledgeMatches(filteredEntries, query);

  // Apply weight multipliers
  const { adjusted, boosted, reduced } = applyWeightMultipliers(matches, filter);
  matches = adjusted;

  // Re-sort after weight adjustment (scores may have changed)
  matches.sort((a, b) => b.score - a.score);

  // Apply minimum score threshold if specified
  if (filter.minScore !== undefined && filter.minScore > 0) {
    const beforeCount = matches.length;
    matches = applyMinScore(matches, filter.minScore);
    const afterCount = matches.length;

    if (beforeCount !== afterCount) {
      logger.debug(
        {
          minScore: filter.minScore,
          beforeCount,
          afterCount,
          removed: beforeCount - afterCount,
        },
        'Applied minimum score threshold',
      );
    }
  }

  // Apply max snippets limit if specified (overrides query limit)
  const limit = filter.maxSnippets ?? query.limit ?? 5;
  matches = matches.slice(0, limit);

  const filterResult: FilteredKnowledgeResult = {
    excluded,
    boosted,
    reduced,
    totalProcessed,
    totalReturned: matches.length,
    policyApplied: true,
  };

  logger.info(
    {
      totalProcessed,
      excluded: excluded.length,
      boosted: boosted.length,
      reduced: reduced.length,
      returned: matches.length,
    },
    'Policy-aware knowledge matching completed',
  );

  return { matches, filterResult };
}

/**
 * Get policy-aware knowledge snippets for selective injection
 *
 * This is a convenience wrapper around findPolicyAwareKnowledgeMatches
 * that returns snippets in the format expected by tools.
 *
 * @param entries - All knowledge entries
 * @param query - Knowledge query
 * @param policy - Optional policy evaluator
 * @returns Knowledge snippets with policy filtering applied
 */
export async function getPolicyAwareKnowledgeSnippets(
  entries: LoadedEntry[],
  query: KnowledgeQuery,
  policy?: RegoEvaluator,
): Promise<{
  snippets: Array<{
    id: string;
    text: string;
    weight: number;
    tags?: string[];
    category?: string;
    source?: string;
    severity?: 'required' | 'high' | 'medium' | 'low';
  }>;
  filterResult: FilteredKnowledgeResult;
}> {
  const { matches, filterResult } = await findPolicyAwareKnowledgeMatches(entries, query, policy);

  const snippets = matches.map((match, index) => ({
    id: `${match.entry.id}:${index}`,
    text: formatEntryAsSnippet(match.entry),
    weight: match.score,
    ...(match.entry.tags && { tags: match.entry.tags }),
    category: match.entry.category,
    source: match.entry.id,
    ...(match.entry.severity && { severity: match.entry.severity }),
  }));

  return { snippets, filterResult };
}

/**
 * Formats a knowledge entry as a concise snippet.
 * (Copied from matcher.ts for independence)
 */
function formatEntryAsSnippet(entry: LoadedEntry): string {
  const parts: string[] = [];

  parts.push(entry.recommendation);

  if (entry.example) {
    if (entry.example.length <= 200) {
      parts.push(`Example: ${entry.example}`);
    } else {
      const fromLines = extractFromLines(entry.example);
      if (fromLines.length > 0) {
        parts.push(`Example: ${fromLines.join(' ')}`);
      }
    }
  }

  return parts.join(' ');
}

/**
 * Extract FROM lines from Dockerfile examples
 * (Copied from matcher.ts for independence)
 */
function extractFromLines(example: string): string[] {
  const lines = example.split('\n');
  return lines
    .filter((line) => line.trim().toUpperCase().startsWith('FROM '))
    .map((line) => line.trim());
}
