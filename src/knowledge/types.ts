/**
 * Knowledge Base Types
 *
 * Simple, focused knowledge system for containerization best practices
 */

/**
 * Knowledge category constants
 */
export const CATEGORY = {
  API: 'api',
  ARCHITECTURE: 'architecture',
  CACHING: 'caching',
  CONFIGURATION: 'configuration',
  DOCKERFILE: 'dockerfile',
  FEATURES: 'features',
  GENERIC: 'generic',
  KUBERNETES: 'kubernetes',
  OPTIMIZATION: 'optimization',
  RELIABILITY: 'reliability',
  RESILIENCE: 'resilience',
  SECURITY: 'security',
  STREAMING: 'streaming',
  VALIDATION: 'validation',
} as const;

export type KnowledgeCategory = (typeof CATEGORY)[keyof typeof CATEGORY];

export interface KnowledgeEntry {
  /** Unique identifier */
  id: string;

  /** Main category */
  category: KnowledgeCategory;

  /** Simple regex pattern to match against */
  pattern: string;

  /** Recommendation text */
  recommendation: string;

  /** Optional code example */
  example?: string;

  /** Severity level */
  severity?: 'high' | 'medium' | 'low';

  /** Tags for additional filtering */
  tags?: string[];

  /** Description of what this knowledge addresses */
  description?: string;
}

export interface KnowledgeQuery {
  /** Category to search in */
  category?: KnowledgeCategory;

  /** Text to match patterns against */
  text?: string;

  /** Programming language context */
  language?: string;

  /** Framework context */
  framework?: string;

  /** Environment context */
  environment?: string;

  /** Specific tags to filter by */
  tags?: string[];

  /** Maximum number of results */
  limit?: number;
}

export interface KnowledgeMatch {
  /** The matched entry */
  entry: KnowledgeEntry;

  /** Match score (higher is better) */
  score: number;

  /** Reasons why this matched */
  reasons: string[];
}

export interface LoadedEntry extends KnowledgeEntry {
  /** Precompiled regex patterns for performance */
  compiledCache?: {
    pattern: RegExp | null;
    lastCompiled: number;
    compilationError?: string;
  };
}

export interface CompilationStats {
  totalEntries: number;
  compiledSuccessfully: number;
  compilationErrors: number;
  avgCompilationTime: number;
}

export interface KnowledgeStats {
  /** Total number of entries */
  totalEntries: number;

  /** Entries by category */
  byCategory: Record<string, number>;

  /** Entries by severity */
  bySeverity: Record<string, number>;

  /** Top tags */
  topTags: Array<{ tag: string; count: number }>;
}
