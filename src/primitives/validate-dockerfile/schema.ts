import { z } from 'zod';

export const validateDockerfileSchema = z.object({
  content: z.string().min(1).describe('Dockerfile text to validate'),
});

export type ValidateDockerfileInput = z.infer<typeof validateDockerfileSchema>;
