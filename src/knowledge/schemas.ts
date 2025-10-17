import { z } from 'zod';

const KnowledgeCategorySchema = z.enum(['dockerfile', 'kubernetes', 'security', 'generic']);

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
