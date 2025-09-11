import { z } from 'zod';

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

export const KnowledgeQuerySchema = z.object({
  category: z.enum(['dockerfile', 'kubernetes', 'security']).optional(),
  text: z.string().optional(),
  language: z.string().optional(),
  framework: z.string().optional(),
  environment: z.string().optional(),
  tags: z.array(z.string()).optional(),
  limit: z.number().min(1).max(100).optional(),
});

export type ValidatedKnowledgeEntry = z.infer<typeof KnowledgeEntrySchema>;
export type ValidatedKnowledgeQuery = z.infer<typeof KnowledgeQuerySchema>;
