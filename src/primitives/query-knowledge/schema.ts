import { z } from 'zod';

export const queryKnowledgeSchema = z.object({
  tags: z
    .array(z.string())
    .min(1)
    .describe('Knowledge tags to match (e.g., ["generate-dockerfile", "node"])'),
  context: z
    .object({
      language: z.string().optional(),
      framework: z.string().optional(),
      toolName: z.string().optional(),
    })
    .optional(),
  limit: z.number().int().positive().max(100).default(10),
});

export type QueryKnowledgeInput = z.infer<typeof queryKnowledgeSchema>;
