import { createLogger } from '@/lib/logger';
import type { Topic } from '@/types/topics';
import type { KnowledgeQuery, KnowledgeMatch, LoadedEntry, KnowledgeCategory } from './types';
import type { KnowledgeSnippet } from './schemas';

const logger = createLogger().child({ module: 'knowledge-matcher' });

// Scoring weights for different match types
const SCORING = {
  CATEGORY: 20,
  PATTERN: 30,
  TAG: 10,
  LANGUAGE: 15,
  FRAMEWORK: 10,
  ENVIRONMENT: 8,
  SEVERITY: {
    required: 50,
    high: 15,
    medium: 10,
    low: 5,
  },
} as const;

// Language keyword mappings for context matching
const LANGUAGE_KEYWORDS: Record<string, string[]> = {
  javascript: ['node', 'nodejs', 'npm', 'js', 'javascript'],
  typescript: ['node', 'nodejs', 'npm', 'ts', 'typescript'],
  python: ['python', 'pip', 'django', 'flask', 'fastapi'],
  java: ['java', 'openjdk', 'maven', 'gradle', 'spring'],
  go: ['golang', 'go'],
  'c#': ['dotnet', 'aspnet', 'csharp'],
  php: ['php', 'composer'],
  ruby: ['ruby', 'rails', 'gem'],
  rust: ['rust', 'cargo'],
} as const;

// Framework keyword mappings
const FRAMEWORK_KEYWORDS: Record<string, string[]> = {
  express: ['express', 'node'],
  react: ['react', 'node', 'npm'],
  vue: ['vue', 'node', 'npm'],
  angular: ['angular', 'node', 'npm'],
  django: ['django', 'python'],
  flask: ['flask', 'python'],
  fastapi: ['fastapi', 'python'],
  spring: ['spring', 'java'],
  rails: ['rails', 'ruby'],
  laravel: ['laravel', 'php'],
  'aspnet-core': ['aspnet', 'dotnet', 'core'],
  'aspnet-webapi': ['aspnet', 'dotnet', 'webapi', 'framework'],
  'aspnet-mvc': ['aspnet', 'dotnet', 'mvc', 'framework'],
  'aspnet-framework': ['aspnet', 'dotnet', 'framework'],
  'dotnet-framework': ['dotnet', 'framework', 'windows'],
  'dotnet-core': ['dotnet', 'core'],
} as const;

// Environment keyword mappings
const ENVIRONMENT_KEYWORDS: Record<string, string[]> = {
  production: ['prod', 'production', 'alpine', 'slim', 'distroless'],
  development: ['dev', 'development'],
  testing: ['test', 'testing'],
  staging: ['staging', 'stage'],
} as const;

/**
 * Get keywords associated with a programming language
 */
const getLanguageKeywords = (language: string): string[] => {
  return LANGUAGE_KEYWORDS[language.toLowerCase()] || [language.toLowerCase()];
};

/**
 * Get keywords associated with a framework
 */
const getFrameworkKeywords = (framework: string): string[] => {
  return FRAMEWORK_KEYWORDS[framework.toLowerCase()] || [framework.toLowerCase()];
};

/**
 * Get keywords associated with an environment
 */
const getEnvironmentKeywords = (environment: string): string[] => {
  return ENVIRONMENT_KEYWORDS[environment.toLowerCase()] || [environment.toLowerCase()];
};

/**
 * Evaluate pattern match using precompiled regex
 */
const evaluatePatternMatch = (
  entry: LoadedEntry,
  query: KnowledgeQuery,
): { score: number; reasons: string[] } => {
  if (!query.text || !entry.pattern) return { score: 0, reasons: [] };

  // Use precompiled pattern if available
  if (entry.compiledCache?.pattern) {
    // Reset regex lastIndex for stateful regex
    entry.compiledCache.pattern.lastIndex = 0;

    if (entry.compiledCache.pattern.test(query.text)) {
      return {
        score: SCORING.PATTERN,
        reasons: ['Pattern match (precompiled)'],
      };
    }
  } else if (entry.compiledCache?.compilationError) {
    // Pattern failed to compile during load, skip
    logger.debug(
      { entryId: entry.id, error: entry.compiledCache.compilationError },
      'Skipping entry with compilation error',
    );
  }

  return { score: 0, reasons: [] };
};

/**
 * Evaluate tag match scoring
 */
const evaluateTagMatch = (
  entry: LoadedEntry,
  query: KnowledgeQuery,
): { score: number; reasons: string[] } => {
  if (!query.tags || !entry.tags) return { score: 0, reasons: [] };

  const matchedTags = query.tags.filter((tag) => entry.tags?.includes(tag));
  if (matchedTags.length > 0) {
    return {
      score: matchedTags.length * SCORING.TAG,
      reasons: [`Tags: ${matchedTags.join(', ')}`],
    };
  }

  return { score: 0, reasons: [] };
};

/**
 * Evaluate language context match scoring
 */
const evaluateLanguageMatch = (
  entry: LoadedEntry,
  query: KnowledgeQuery,
): { score: number; reasons: string[] } => {
  if (!query.language) return { score: 0, reasons: [] };

  const languageKeywords = getLanguageKeywords(query.language);

  // Fast path: Check tags first (most common match)
  if (entry.tags) {
    const tagMatch = languageKeywords.some((keyword) => entry.tags?.includes(keyword));
    if (tagMatch) {
      return {
        score: SCORING.LANGUAGE,
        reasons: [`Language: ${query.language} (tag match)`],
      };
    }
  }

  // Slower path: Check text content only if no tag match
  const textMatch = languageKeywords.some((keyword) => {
    const lowerKeyword = keyword.toLowerCase();
    return (
      entry.recommendation.toLowerCase().includes(lowerKeyword) ||
      entry.pattern.toLowerCase().includes(lowerKeyword)
    );
  });

  if (textMatch) {
    return {
      score: SCORING.LANGUAGE,
      reasons: [`Language: ${query.language} (text match)`],
    };
  }

  return { score: 0, reasons: [] };
};

/**
 * Evaluate framework context match scoring
 */
const evaluateFrameworkMatch = (
  entry: LoadedEntry,
  query: KnowledgeQuery,
): { score: number; reasons: string[] } => {
  if (!query.framework || !entry.tags) return { score: 0, reasons: [] };

  const frameworkKeywords = getFrameworkKeywords(query.framework);
  const hasMatch = frameworkKeywords.some(
    (keyword) =>
      entry.tags?.includes(keyword) || entry.recommendation.toLowerCase().includes(keyword),
  );

  if (hasMatch) {
    return {
      score: SCORING.FRAMEWORK,
      reasons: [`Framework: ${query.framework}`],
    };
  }

  return { score: 0, reasons: [] };
};

/**
 * Evaluate environment context match scoring
 */
const evaluateEnvironmentMatch = (
  entry: LoadedEntry,
  query: KnowledgeQuery,
): { score: number; reasons: string[] } => {
  if (!query.environment) return { score: 0, reasons: [] };

  const envKeywords = getEnvironmentKeywords(query.environment);
  const hasMatch = envKeywords.some(
    (keyword) =>
      entry.tags?.includes(keyword) || entry.recommendation.toLowerCase().includes(keyword),
  );

  if (hasMatch) {
    return {
      score: SCORING.ENVIRONMENT,
      reasons: [`Environment: ${query.environment}`],
    };
  }

  return { score: 0, reasons: [] };
};

/**
 * Evaluate severity scoring boost
 */
const evaluateSeverity = (entry: LoadedEntry): number => {
  if (!entry.severity) return 0;
  return SCORING.SEVERITY[entry.severity] || SCORING.SEVERITY.medium;
};

/**
 * Evaluate how well an entry matches the query using pure scoring functions
 */
export const evaluateEntry = (entry: LoadedEntry, query: KnowledgeQuery): KnowledgeMatch | null => {
  // Early exit for category mismatch
  if (query.category && entry.category !== query.category) {
    return null;
  }

  let totalScore = 0;
  const allReasons: string[] = [];

  // Skip category evaluation since we already checked
  const evaluations = [
    query.category
      ? { score: SCORING.CATEGORY, reasons: [`Category: ${query.category}`] }
      : { score: 0, reasons: [] },
    evaluatePatternMatch(entry, query),
    evaluateTagMatch(entry, query),
    evaluateLanguageMatch(entry, query),
    evaluateFrameworkMatch(entry, query),
    evaluateEnvironmentMatch(entry, query),
  ];

  // Accumulate scores and reasons
  for (const evaluation of evaluations) {
    totalScore += evaluation.score;
    allReasons.push(...evaluation.reasons);
  }

  // Add severity bonus
  totalScore += evaluateSeverity(entry);

  // Return null for zero scores to enable filtering
  if (totalScore === 0) return null;

  return { entry, score: totalScore, reasons: allReasons };
};

/**
 * Find matching knowledge entries for a query using functional composition
 */
export const findKnowledgeMatches = (
  entries: LoadedEntry[],
  query: KnowledgeQuery,
): KnowledgeMatch[] => {
  const matches = entries
    .map((entry) => evaluateEntry(entry, query))
    .filter((match): match is KnowledgeMatch => match !== null && match.score > 0)
    .sort((a, b) => b.score - a.score);

  const limit = query.limit || 5;
  return matches.slice(0, limit);
};

/**
 * Options for knowledge snippet retrieval.
 */
export interface KnowledgeSnippetOptions {
  environment: string;
  tool: string;
  language?: string;
  framework?: string;
  category?: KnowledgeCategory;
  maxChars?: number;
  maxSnippets?: number;
  detectedDependencies?: string[];
}

/**
 * Get weighted knowledge snippets for selective injection.
 *
 * @param topic - Topic to search for
 * @param options - Options for snippet selection
 * @returns Promise resolving to weighted snippets
 */
export async function getKnowledgeSnippets(
  topic: Topic,
  options: KnowledgeSnippetOptions,
): Promise<KnowledgeSnippet[]> {
  try {
    // Import loader dynamically to avoid circular dependencies
    const { loadKnowledgeData } = await import('./loader');
    const knowledgeData = await loadKnowledgeData();

    const queryTextParts: string[] = [topic];
    if (options.detectedDependencies && options.detectedDependencies.length > 0) {
      queryTextParts.push(...options.detectedDependencies);
    }

    const queryTags: string[] = [options.tool];
    if (options.language) queryTags.push(options.language);
    if (options.framework) queryTags.push(options.framework);
    if (options.environment) queryTags.push(options.environment);

    const query: KnowledgeQuery = {
      text: queryTextParts.join(' '),
      environment: options.environment,
      ...(options.language && { language: options.language }),
      ...(options.framework && { framework: options.framework }),
      ...(options.category && { category: options.category }),
      tags: queryTags,
      limit: options.maxSnippets || 10,
    };

    // Find matches
    const matches = findKnowledgeMatches(knowledgeData.entries, query);

    // Convert matches to snippets
    const snippets: KnowledgeSnippet[] = matches.map((match, index) => ({
      id: `${match.entry.id}:${index}`,
      text: formatEntryAsSnippet(match.entry),
      weight: match.score,
      ...(match.entry.tags && { tags: match.entry.tags }),
      category: match.entry.category,
      source: match.entry.id,
    }));

    // Apply character budget if specified
    if (options.maxChars && options.maxChars > 0) {
      return applyCharacterBudget(snippets, options.maxChars);
    }

    return snippets;
  } catch (error) {
    logger.error({ error, topic, options }, 'Failed to get knowledge snippets');
    return [];
  }
}

/**
 * Formats a knowledge entry as a concise snippet.
 *
 * @param entry - Knowledge entry to format
 * @returns Formatted snippet text
 */
function formatEntryAsSnippet(entry: LoadedEntry): string {
  const parts: string[] = [];

  // Add recommendation (primary content)
  parts.push(entry.recommendation);

  // Add example if present and concise
  if (entry.example && entry.example.length <= 200) {
    parts.push(`Example: ${entry.example}`);
  }

  return parts.join(' ');
}

/**
 * Apply character budget to snippets, selecting highest weighted ones.
 *
 * @param snippets - Snippets to budget
 * @param maxChars - Maximum character count
 * @returns Snippets within budget
 */
function applyCharacterBudget(snippets: KnowledgeSnippet[], maxChars: number): KnowledgeSnippet[] {
  const selected: KnowledgeSnippet[] = [];
  let currentChars = 0;

  // Snippets are already sorted by weight/score
  for (const snippet of snippets) {
    const snippetLength = snippet.text.length;

    if (currentChars + snippetLength <= maxChars) {
      selected.push(snippet);
      currentChars += snippetLength;
    } else if (currentChars === 0 && snippetLength > maxChars) {
      // If first snippet exceeds budget, truncate it
      selected.push({
        ...snippet,
        text: `${snippet.text.substring(0, maxChars - 3)}...`,
      });
      break;
    } else {
      // Budget exhausted
      break;
    }
  }

  return selected;
}
