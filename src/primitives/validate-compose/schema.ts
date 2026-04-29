import { z } from 'zod';

export const validateComposeSchema = z.object({
  content: z.string().min(1).describe('docker-compose YAML'),
});

export type ValidateComposeInput = z.infer<typeof validateComposeSchema>;
