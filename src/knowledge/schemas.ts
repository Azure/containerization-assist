import { z } from 'zod';

/**
 * Knowledge category schema - matches CATEGORY constants in types.ts
 */
const KnowledgeCategorySchema = z.enum([
  'api',
  'architecture',
  'build',
  'caching',
  'configuration',
  'dockerfile',
  'features',
  'generic',
  'kubernetes',
  'optimization',
  'reliability',
  'resilience',
  'security',
  'streaming',
  'validation',
]);

/**
 * Schema for individual knowledge entry
 */
export const KnowledgeEntrySchema = z.object({
  id: z.string().min(1),
  category: KnowledgeCategorySchema,
  pattern: z.string().min(1),
  recommendation: z.string().min(1),
  example: z.string().optional(),
  severity: z.enum(['required', 'high', 'medium', 'low']).optional(),
  tags: z.array(z.string()).optional(),
  description: z.string().optional(),
});

/**
 * Schema for knowledge pack metadata wrapper
 * Some packs are wrapped in an object with metadata
 */
const KnowledgePackMetadataSchema = z.object({
  name: z.string(),
  version: z.string(),
  description: z.string().optional(),
  triggers: z.array(z.string()).optional(),
  rules: z.array(KnowledgeEntrySchema).min(1),
});

/**
 * Schema for knowledge pack file structure
 * Supports both formats:
 * 1. Array of entries (flat format)
 * 2. Object with metadata and rules array (wrapped format)
 */
export const KnowledgePackSchema = z.union([
  z.array(KnowledgeEntrySchema).min(1),
  KnowledgePackMetadataSchema,
]);

/**
 * Weighted knowledge snippet for selective injection.
 */
export interface KnowledgeSnippet {
  id: string;
  text: string;
  weight: number;
  tags?: string[];
  category?: string;
  source?: string;
}
