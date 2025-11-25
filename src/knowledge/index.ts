/**
 * Knowledge Base – minimal public API
 */
export type { KnowledgeEntry, KnowledgeQuery, KnowledgeMatch, LoadedEntry } from './types';
export type {
  KnowledgeFilter,
  KnowledgeFilterContext,
  FilteredKnowledgeResult,
} from './policy-types';

import { findKnowledgeMatches } from './matcher';
import { loadKnowledgeBase, getAllEntries, isKnowledgeLoaded } from './loader';
import type { KnowledgeQuery, KnowledgeMatch } from './types';
import {
  findPolicyAwareKnowledgeMatches,
  getPolicyAwareKnowledgeSnippets,
} from './policy-aware-matcher';

// Export policy-aware functions
export { findPolicyAwareKnowledgeMatches, getPolicyAwareKnowledgeSnippets };

// Internal helper - only used by getKnowledgeForCategory
function getKnowledgeRecommendations(query: KnowledgeQuery): KnowledgeMatch[] {
  if (!isKnowledgeLoaded()) loadKnowledgeBase();
  return findKnowledgeMatches(getAllEntries(), query);
}

export function getKnowledgeForCategory(
  category: 'dockerfile' | 'kubernetes' | 'security',
  text?: string,
  context?: { language?: string; framework?: string; environment?: string },
): KnowledgeMatch[] {
  const query: KnowledgeQuery = {
    category,
    ...(text && { text }),
    ...(context?.language && { language: context.language }),
    ...(context?.framework && { framework: context.framework }),
    ...(context?.environment && { environment: context.environment }),
    limit: 5,
  };
  return getKnowledgeRecommendations(query);
}
