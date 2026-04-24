import { z } from 'zod';

export const validateComposeSchema = z.object({
  content: z.string().min(1).describe('docker-compose YAML'),
  context: z
    .object({
      environment: z.enum(['dev', 'staging', 'production']).optional(),
    })
    .strict()
    .optional(),
});

export type ValidateComposeInput = z.infer<typeof validateComposeSchema>;
