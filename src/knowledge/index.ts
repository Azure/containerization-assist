/**
 * Knowledge Base System
 *
 * Provides containerization best practices and recommendations
 */

export type {
  KnowledgeEntry,
  KnowledgeQuery,
  KnowledgeMatch,
  KnowledgeStats,
  LoadedEntry,
  CompilationStats,
} from './types';

export { KnowledgeEntrySchema, KnowledgeQuerySchema } from './schemas';
export type { ValidatedKnowledgeEntry, ValidatedKnowledgeQuery } from './schemas';

export {
  loadKnowledgeBase,
  getEntryById,
  getEntriesByCategory,
  getEntriesByTag,
  getAllEntries,
  getCompilationStats,
  isKnowledgeLoaded,
  reloadKnowledgeBase,
} from './loader';
export { findKnowledgeMatches, evaluateEntry } from './matcher';

import { findKnowledgeMatches } from './matcher';
import {
  loadKnowledgeBase as loadKnowledgeBaseInternal,
  getAllEntries as getAllEntriesInternal,
  getKnowledgeStats as getKnowledgeStatsInternal,
  isKnowledgeLoaded as isKnowledgeLoadedInternal,
} from './loader';
import type { KnowledgeQuery, KnowledgeMatch, KnowledgeStats } from './types';

/**
 * High-level function to get knowledge recommendations
 */
export async function getKnowledgeRecommendations(
  query: KnowledgeQuery,
): Promise<KnowledgeMatch[]> {
  // Ensure knowledge is loaded
  if (!isKnowledgeLoadedInternal()) {
    await loadKnowledgeBaseInternal();
  }

  const entries = getAllEntriesInternal();
  return findKnowledgeMatches(entries, query);
}

/**
 * Get knowledge recommendations for a specific category
 */
export async function getKnowledgeForCategory(
  category: 'dockerfile' | 'kubernetes' | 'security',
  text?: string,
  context?: { language?: string; framework?: string; environment?: string },
): Promise<KnowledgeMatch[]> {
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

/**
 * Get knowledge base statistics
 */
export async function getKnowledgeStats(): Promise<KnowledgeStats> {
  if (!isKnowledgeLoadedInternal()) {
    await loadKnowledgeBaseInternal();
  }

  return getKnowledgeStatsInternal();
}

/**
 * Initialize the knowledge base (call this on startup)
 */
export async function initializeKnowledge(): Promise<void> {
  await loadKnowledgeBaseInternal();
}
