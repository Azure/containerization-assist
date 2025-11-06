import { createLogger } from '@/lib/logger';
import type { Topic } from '@/types/topics';
import type { KnowledgeQuery, KnowledgeMatch, LoadedEntry, KnowledgeCategory } from './types';
import type { KnowledgeSnippet } from './schemas';
import { loadKnowledgeData } from './loader';

const logger = createLogger().child({ module: 'knowledge-matcher' });

// Scoring weights for different match types
const SCORING = {
  CATEGORY: 20,
  PATTERN: 30,
  TAG: 10,
  LANGUAGE: 15,
  LANGUAGE_VERSION_EXACT: 20, // Exact version match
  LANGUAGE_VERSION_COMPATIBLE: 10, // Compatible version (within major version)
  FRAMEWORK: 10,
  ENVIRONMENT: 8,
  TOOL: 25, // High weight for tool-specific matches
  VENDOR: 12, // Medium weight for vendor matches
  SEVERITY: {
    required: 50,
    high: 15,
    medium: 10,
    low: 5,
  },
} as const;

const SEVERITY_TIERS: Record<string, number> = {
  required: 4,
  high: 3,
  medium: 2,
  low: 1,
} as const;

// Language keyword mappings for context matching
const LANGUAGE_KEYWORDS: Record<string, string[]> = {
  javascript: ['node', 'nodejs', 'npm', 'js', 'javascript'],
  typescript: ['node', 'nodejs', 'npm', 'ts', 'typescript'],
  python: ['python', 'pip', 'django', 'flask', 'fastapi', 'gunicorn'],
  java: ['java', 'openjdk', 'maven', 'gradle', 'spring', 'quarkus', 'micronaut'],
  go: ['golang', 'go'],
  'c#': ['dotnet', 'aspnet', 'csharp', 'blazor'],
  php: ['php', 'composer', 'laravel'],
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

const TAG_ALIASES: Record<string, string> = {
  // Languages
  nodejs: 'node',
  javascript: 'node',
  js: 'node',
  typescript: 'node',
  ts: 'node',
  golang: 'go',
  py: 'python',
  csharp: 'dotnet',

  // Image types
  minimal: 'distroless',

  // Vendor
  mcr: 'microsoft',
  msft: 'microsoft',
  gcp: 'google',
  gcr: 'google',
  eks: 'aws',
  ecr: 'aws',
  aks: 'azure',
  acr: 'azure',

  // Build tools
  mvn: 'maven',
  gradlew: 'gradle',
  cargo: 'rust',
} as const;

/**
 * Normalize a tag to its canonical form
 * @param tag - Tag to normalize
 * @returns Canonical tag name (lowercase)
 */
const normalizeTag = (tag: string): string => {
  const lower = tag.toLowerCase();
  return TAG_ALIASES[lower] || lower;
};

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
 * Evaluate pattern match by compiling regex on-demand
 */
const evaluatePatternMatch = (
  entry: LoadedEntry,
  query: KnowledgeQuery,
): { score: number; reasons: string[] } => {
  if (!query.text || !entry.pattern) return { score: 0, reasons: [] };

  try {
    // Compile regex on-demand (regex compilation is fast)
    const regex = new RegExp(entry.pattern, 'gmi');
    if (regex.test(query.text)) {
      return {
        score: SCORING.PATTERN,
        reasons: ['Pattern match'],
      };
    }
  } catch (error) {
    // Skip entries with invalid patterns
    logger.debug(
      { entryId: entry.id, pattern: entry.pattern, error },
      'Skipping entry with invalid pattern',
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

  const normalizedQueryTags = query.tags.map(normalizeTag);
  const normalizedEntryTags = entry.tags.map(normalizeTag);

  const matchedTags = normalizedQueryTags.filter((tag) => normalizedEntryTags.includes(tag));
  if (matchedTags.length > 0) {
    return {
      score: matchedTags.length * SCORING.TAG,
      reasons: [`Tags: ${matchedTags.join(', ')}`],
    };
  }

  return { score: 0, reasons: [] };
};

/**
 * Extract version number from text (e.g., "openjdk:17" -> "17", "jdk:21-azurelinux" -> "21")
 */
const extractVersionFromText = (text: string): string | null => {
  // Match version patterns like :17, :21, :3.11, openjdk-17, java17, etc.
  const versionPatterns = [
    /:(\d+(?:\.\d+)?)/i, // :17, :21, :3.11
    /openjdk[:-]?(\d+)/i, // openjdk-17, openjdk:17
    /jdk[:-]?(\d+)/i, // jdk-21, jdk:21
    /java[:-]?(\d+)/i, // java-17, java:17
    /python[:-]?(\d+(?:\.\d+)?)/i, // python-3.11, python:3.11
    /node[:-]?(\d+)/i, // node-20, node:20
  ];

  for (const pattern of versionPatterns) {
    const match = text.match(pattern);
    if (match?.[1]) {
      return match[1];
    }
  }

  return null;
};

/**
 * Check if two version strings are compatible (same major version)
 */
const areVersionsCompatible = (version1: string, version2: string): boolean => {
  const major1 = version1.split('.')[0];
  const major2 = version2.split('.')[0];
  return major1 === major2;
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
  let score = 0;
  const reasons: string[] = [];

  // Fast path: Check tags first (most common match)
  if (entry.tags) {
    const tagMatch = languageKeywords.some((keyword) => entry.tags?.includes(keyword));
    if (tagMatch) {
      score += SCORING.LANGUAGE;
      reasons.push(`Language: ${query.language} (tag match)`);
    }
  }

  // Slower path: Check text content only if no tag match
  if (score === 0) {
    const textMatch = languageKeywords.some((keyword) => {
      const lowerKeyword = keyword.toLowerCase();
      return (
        entry.recommendation.toLowerCase().includes(lowerKeyword) ||
        entry.pattern.toLowerCase().includes(lowerKeyword)
      );
    });

    if (textMatch) {
      score += SCORING.LANGUAGE;
      reasons.push(`Language: ${query.language} (text match)`);
    }
  }

  // Language version matching
  if (query.languageVersion && score > 0) {
    const recommendationVersion = extractVersionFromText(entry.recommendation);
    const exampleVersion = entry.example ? extractVersionFromText(entry.example) : null;

    const matchedVersion = recommendationVersion || exampleVersion;

    if (matchedVersion) {
      if (matchedVersion === query.languageVersion) {
        score += SCORING.LANGUAGE_VERSION_EXACT;
        reasons.push(`Version: ${query.languageVersion} (exact match)`);
      } else if (areVersionsCompatible(matchedVersion, query.languageVersion)) {
        score += SCORING.LANGUAGE_VERSION_COMPATIBLE;
        reasons.push(`Version: ${matchedVersion} (compatible with ${query.languageVersion})`);
      }
    }
  }

  return score > 0 ? { score, reasons } : { score: 0, reasons: [] };
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
 * Evaluate tool context match scoring
 */
const evaluateToolMatch = (
  entry: LoadedEntry,
  query: KnowledgeQuery,
): { score: number; reasons: string[] } => {
  if (!query.tool || !entry.tags) return { score: 0, reasons: [] };

  const normalizedTool = normalizeTag(query.tool);
  const normalizedEntryTags = entry.tags.map(normalizeTag);

  if (normalizedEntryTags.includes(normalizedTool)) {
    return {
      score: SCORING.TOOL,
      reasons: [`Tool: ${query.tool}`],
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
 * Language, framework, and technology tags that identify context-specific entries.
 * If an entry has any of these tags, it should only match queries for that context.
 * This prevents irrelevant recommendations (e.g., Flask for Java, PostgreSQL for non-DB apps)
 */
const LANGUAGE_TAGS = [
  // Languages
  'java',
  'python',
  'node',
  'nodejs',
  'javascript',
  'typescript',
  'go',
  'golang',
  'dotnet',
  'csharp',
  'php',
  'ruby',
  'rust',
  // Language-specific frameworks
  'spring',
  'quarkus',
  'micronaut',
  'django',
  'flask',
  'fastapi',
  'gunicorn',
  'express',
  'react',
  'vue',
  'angular',
  'aspnet',
  'blazor',
  'laravel',
  'rails',
  // Database technologies (only match if explicitly requested)
  'postgres',
  'postgresql',
  'mongodb',
  'mongo',
  'mysql',
  'mariadb',
  'redis',
  'elasticsearch',
  'cassandra',
  'dynamodb',
  'sqlserver',
  'vitess',
  'cockroachdb',
  'migrations',
  'schema',
  // .NET specific
  'dotnet-framework',
  'entity-framework',
  'ef-core',
  'worker-service',
  // Database server infrastructure (not client libraries)
  'database-server',
] as const;

/**
 * Check if an entry is tagged with a specific context (language/framework/tech) that conflicts with the query
 */
const hasConflictingLanguageTag = (
  entry: LoadedEntry,
  queryLanguage: string,
  queryTags?: string[],
): boolean => {
  if (!entry.tags || entry.tags.length === 0) {
    return false; // Entries without tags are considered generic
  }

  const normalizedQueryLanguage = normalizeTag(queryLanguage);
  const queryLanguageKeywords = getLanguageKeywords(queryLanguage);

  // Normalize all query tags for comparison
  const normalizedQueryTags = queryTags ? queryTags.map(normalizeTag) : [];

  // Check if entry has any context-specific tags
  const entryContextTags = entry.tags
    .map(normalizeTag)
    .filter((tag) => LANGUAGE_TAGS.includes(tag as (typeof LANGUAGE_TAGS)[number]));

  if (entryContextTags.length === 0) {
    return false; // Entry has no context tags, it's generic
  }

  // Check if any of the entry's context tags match the query language/framework/dependencies
  const hasMatchingContext = entryContextTags.some(
    (tag) =>
      tag === normalizedQueryLanguage ||
      queryLanguageKeywords.includes(tag) ||
      normalizedQueryTags.includes(tag),
  );

  // If entry has context tags but none match the query, it's a conflict
  return !hasMatchingContext;
};

/**
 * Evaluate how well an entry matches the query using pure scoring functions
 * Internal function used by findKnowledgeMatches
 */
const evaluateEntry = (entry: LoadedEntry, query: KnowledgeQuery): KnowledgeMatch | null => {
  // Early exit for category mismatch
  if (query.category && entry.category !== query.category) {
    return null;
  }

  // Early exit for language conflict
  // If query specifies a language and entry has conflicting language tags, exclude it
  if (query.language && hasConflictingLanguageTag(entry, query.language, query.tags)) {
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
    evaluateToolMatch(entry, query),
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
 * Get severity tier for sorting priority.
 */
const getSeverityTier = (entry: LoadedEntry): number => {
  if (!entry.severity) return 0;
  return SEVERITY_TIERS[entry.severity] ?? 0;
};

/**
 * Find matching knowledge entries for a query using functional composition.
 */
export const findKnowledgeMatches = (
  entries: LoadedEntry[],
  query: KnowledgeQuery,
): KnowledgeMatch[] => {
  const matches = entries
    .map((entry) => evaluateEntry(entry, query))
    .filter((match): match is KnowledgeMatch => match !== null && match.score > 0)
    .sort((a, b) => {
      const tierDiff = getSeverityTier(b.entry) - getSeverityTier(a.entry);
      if (tierDiff !== 0) return tierDiff;

      return b.score - a.score;
    });

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
  languageVersion?: string;
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
    const knowledgeData = await loadKnowledgeData();

    const queryTextParts: string[] = [topic];
    if (options.detectedDependencies && options.detectedDependencies.length > 0) {
      queryTextParts.push(...options.detectedDependencies);
    }

    const queryTags: string[] = [options.tool];
    if (options.language) queryTags.push(options.language);
    if (options.framework) queryTags.push(options.framework);
    if (options.environment) queryTags.push(options.environment);
    if (options.detectedDependencies) {
      // Add detected dependencies as tags for better context matching
      queryTags.push(...options.detectedDependencies);
    }

    const query: KnowledgeQuery = {
      text: queryTextParts.join(' '),
      environment: options.environment,
      tool: options.tool,
      ...(options.language && { language: options.language }),
      ...(options.languageVersion && { languageVersion: options.languageVersion }),
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
      ...(match.entry.severity && { severity: match.entry.severity }),
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
 * Extract FROM lines from Dockerfile examples
 */
function extractFromLines(example: string): string[] {
  const lines = example.split('\n');
  return lines
    .filter((line) => line.trim().toUpperCase().startsWith('FROM '))
    .map((line) => line.trim());
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

  // Add example if present
  if (entry.example) {
    if (entry.example.length <= 200) {
      // Short examples: include as-is
      parts.push(`Example: ${entry.example}`);
    } else {
      // Long examples: extract FROM lines for base image identification
      const fromLines = extractFromLines(entry.example);
      if (fromLines.length > 0) {
        parts.push(`Example: ${fromLines.join(' ')}`);
      }
    }
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
  const selectedIds = new Set<string>();
  let currentChars = 0;

  const requiredSnippets = snippets.filter((s) => s.severity === 'required');

  for (const snippet of requiredSnippets) {
    selected.push(snippet);
    selectedIds.add(snippet.id);
    currentChars += snippet.text.length;
  }

  for (const snippet of snippets) {
    if (selectedIds.has(snippet.id)) {
      continue;
    }

    const snippetLength = snippet.text.length;

    if (currentChars + snippetLength <= maxChars) {
      selected.push(snippet);
      selectedIds.add(snippet.id);
      currentChars += snippetLength;
    } else if (selected.length === requiredSnippets.length && snippetLength > maxChars) {
      // If first non-required snippet exceeds budget, truncate it
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
