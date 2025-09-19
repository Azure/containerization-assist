/**
 * Scoring for knowledge entries
 */
import type { LoadedEntry, KnowledgeMatch } from './types';

export interface ScoringContext {
  text: string;
  language?: string;
  framework?: string;
  environment?: string;
}

/**
 * Score a knowledge entry against context
 */
export function scoreEntry(
  entry: LoadedEntry,
  context: ScoringContext,
): { score: number; reasons: string[] } {
  let score = 0;
  const reasons: string[] = [];

  // Pattern matching (0-40 points)
  if (entry._compiled?.pattern && context.text) {
    const matches = context.text.match(entry._compiled.pattern);
    if (matches) {
      score += Math.min(40, matches.length * 10);
      reasons.push(`Pattern matched ${matches.length} time(s)`);
    }
  }

  // Tag matching (0-30 points)
  if (entry.tags?.length) {
    let tagBonus = 0;

    if (context.language && entry.tags.includes(context.language.toLowerCase())) {
      tagBonus += 15;
      reasons.push(`Language: ${context.language}`);
    }

    if (context.framework && entry.tags.includes(context.framework.toLowerCase())) {
      tagBonus += 15;
      reasons.push(`Framework: ${context.framework}`);
    }

    score += tagBonus;
  }

  // Severity bonus (0-20 points)
  if (entry.severity) {
    const severityBonus =
      {
        critical: 20,
        high: 15,
        medium: 10,
        low: 5,
      }[entry.severity] || 10;

    score += severityBonus;
    reasons.push(`Severity: ${entry.severity}`);
  }

  // Environment match (0-10 points)
  if (
    context.environment === 'production' &&
    (entry.tags?.includes('production') || entry.description?.includes('production'))
  ) {
    score += 10;
    reasons.push('Production relevance');
  }

  return { score: Math.min(100, score), reasons };
}

/**
 * Rank matches by score
 */
export function rankMatches(matches: KnowledgeMatch[], limit: number = 10): KnowledgeMatch[] {
  return matches.sort((a, b) => b.score - a.score).slice(0, limit);
}

/**
 * Group matches by category
 */
export function groupByCategory(matches: KnowledgeMatch[]): Map<string, KnowledgeMatch[]> {
  const grouped = new Map<string, KnowledgeMatch[]>();

  for (const match of matches) {
    const category = match.entry.category;
    if (!grouped.has(category)) {
      grouped.set(category, []);
    }
    grouped.get(category)?.push(match);
  }

  return grouped;
}
