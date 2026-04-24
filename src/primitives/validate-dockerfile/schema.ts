import { z } from 'zod';

export const validateDockerfileSchema = z.object({
  content: z.string().min(1).describe('Dockerfile text to validate'),
  context: z
    .object({
      environment: z.enum(['dev', 'staging', 'production']).optional(),
      language: z.string().min(1).optional(),
    })
    .strict()
    .optional(),
});

export type ValidateDockerfileInput = z.infer<typeof validateDockerfileSchema>;
