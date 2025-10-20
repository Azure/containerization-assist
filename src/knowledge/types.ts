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
  BUILD: 'build',
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
  severity?: 'required' | 'high' | 'medium' | 'low';

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

/**
 * LoadedEntry is just an alias for KnowledgeEntry.
 * Regex patterns are compiled on-demand for simplicity.
 */
export type LoadedEntry = KnowledgeEntry;
