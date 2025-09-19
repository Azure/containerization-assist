import { z } from 'zod';

/**
 * Schema for individual knowledge pack entries (matches actual JSON structure)
 */
export const KnowledgePackEntrySchema = z.object({
  id: z.string().min(1, 'ID is required'),
  category: z.enum(['dockerfile', 'kubernetes', 'security', 'performance', 'general']),
  pattern: z.string().min(1, 'Pattern is required'),
  recommendation: z.string().min(1, 'Recommendation is required'),
  example: z.string().min(1, 'Example is required'),
  severity: z.enum(['low', 'medium', 'high', 'critical']),
  tags: z.array(z.string()).min(1, 'At least one tag is required'),
  description: z.string().min(1, 'Description is required'),
  rationale: z.string().optional(),
  tradeoffs: z.string().optional(),
  alternatives: z.array(z.string()).optional(),
  metrics: z
    .object({
      sizeImpact: z.string().optional(),
      buildTimeImpact: z.string().optional(),
      securityScore: z.string().optional(),
    })
    .optional(),
});

/**
 * Schema for knowledge pack files (arrays of entries)
 */
export const KnowledgePackFileSchema = z.array(KnowledgePackEntrySchema);

/**
 * Consolidated schema for simpler knowledge entries (used by pack-loader)
 */
export const KnowledgeEntrySchema = z.object({
  id: z.string().min(1),
  category: z.enum(['dockerfile', 'kubernetes', 'security']),
  pattern: z.string().min(1),
  recommendation: z.string().min(1),
  example: z.string().optional(),
  severity: z.enum(['high', 'medium', 'low']).optional(),
  tags: z.array(z.string()).optional(),
  description: z.string().optional(),
});

/**
 * Schema for knowledge queries
 */
export const KnowledgeQuerySchema = z.object({
  category: z.enum(['dockerfile', 'kubernetes', 'security']).optional(),
  text: z.string().optional(),
  language: z.string().optional(),
  framework: z.string().optional(),
  environment: z.string().optional(),
  tags: z.array(z.string()).optional(),
  limit: z.number().min(1).max(100).optional(),
});

/**
 * Knowledge Schema for advanced knowledge snippets
 */
export const KnowledgeSnippetSchema = z.object({
  id: z.string(),
  category: z.enum([
    'languages',
    'frameworks',
    'databases',
    'security',
    'monitoring',
    'ci-cd',
    'cloud',
    'orchestration',
    'ml',
  ]),
  tags: z.array(z.string()).optional(),

  // Content
  title: z.string(),
  description: z.string().optional(),
  data: z.record(z.unknown()),

  // Metadata
  version: z.string().optional(),
  source: z.string().optional(),
  author: z.string().optional(),

  // TTL for volatile data
  ttl: z
    .object({
      seconds: z.number().positive(),
      volatile: z.boolean(),
      lastUpdated: z.string().datetime().optional(),
    })
    .optional(),

  // Usage hints
  applicableTo: z.array(z.string()).optional(), // List of tools this knowledge applies to
  priority: z.number().min(0).max(100).optional(),
});

/**
 * Knowledge Document Schema - For longer-form documentation
 */
export const KnowledgeDocumentSchema = z.object({
  id: z.string(),
  category: z.string(),
  title: z.string(),
  content: z.string(), // Markdown content
  tags: z.array(z.string()).optional(),

  // Metadata
  version: z.string().optional(),
  lastUpdated: z.string().datetime().optional(),
  author: z.string().optional(),

  // References
  references: z
    .array(
      z.object({
        title: z.string(),
        url: z.string().url(),
      }),
    )
    .optional(),
});

/**
 * Knowledge Pack Schema - Collection of related knowledge
 */
export const KnowledgePackSchema = z.object({
  id: z.string(),
  name: z.string(),
  description: z.string(),
  version: z.string(),
  category: z.string(),

  // Content
  snippets: z.array(KnowledgeSnippetSchema).optional(),
  documents: z.array(z.string()).optional(), // References to document IDs

  // Dependencies
  requires: z.array(z.string()).optional(), // Other pack IDs

  // Metadata
  created: z.string().datetime().optional(),
  updated: z.string().datetime().optional(),
});

/**
 * Type definitions
 */
export type KnowledgePackEntry = z.infer<typeof KnowledgePackEntrySchema>;
export type KnowledgePackFile = z.infer<typeof KnowledgePackFileSchema>;
export type ValidatedKnowledgeEntry = z.infer<typeof KnowledgeEntrySchema>;
export type ValidatedKnowledgeQuery = z.infer<typeof KnowledgeQuerySchema>;
export type KnowledgeSnippet = z.infer<typeof KnowledgeSnippetSchema>;
export type KnowledgeDocument = z.infer<typeof KnowledgeDocumentSchema>;
export type KnowledgePack = z.infer<typeof KnowledgePackSchema>;

/**
 * Validate a knowledge pack file
 */
export function validateKnowledgePackFile(data: unknown): KnowledgePackFile {
  return KnowledgePackFileSchema.parse(data);
}

/**
 * Validate a single knowledge entry
 */
export function validateKnowledgePackEntry(data: unknown): KnowledgePackEntry {
  return KnowledgePackEntrySchema.parse(data);
}

/**
 * Check if knowledge is still fresh based on TTL
 */
export function isKnowledgeFresh(snippet: KnowledgeSnippet): boolean {
  if (!snippet.ttl) {
    return true; // No TTL means always fresh
  }

  if (!snippet.ttl.lastUpdated) {
    return false; // No update time means stale
  }

  const lastUpdated = new Date(snippet.ttl.lastUpdated);
  const now = new Date();
  const ageSeconds = (now.getTime() - lastUpdated.getTime()) / 1000;

  return ageSeconds < snippet.ttl.seconds;
}

/**
 * Filter knowledge by category
 */
export function filterByCategory(
  snippets: KnowledgeSnippet[],
  category: KnowledgeSnippet['category'],
): KnowledgeSnippet[] {
  return snippets.filter((s) => s.category === category);
}

/**
 * Filter knowledge by tags
 */
export function filterByTags(snippets: KnowledgeSnippet[], tags: string[]): KnowledgeSnippet[] {
  return snippets.filter((s) => s.tags?.some((tag) => tags.includes(tag)));
}

/**
 * Get knowledge applicable to a specific tool
 */
export function getApplicableKnowledge(
  snippets: KnowledgeSnippet[],
  toolName: string,
): KnowledgeSnippet[] {
  return snippets.filter((s) => s.applicableTo?.includes(toolName));
}

/**
 * Sort knowledge by priority
 */
export function sortByPriority(snippets: KnowledgeSnippet[]): KnowledgeSnippet[] {
  return [...snippets].sort((a, b) => (b.priority ?? 50) - (a.priority ?? 50));
}
