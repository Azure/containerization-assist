import { z } from 'zod';

export const validateK8sManifestSchema = z.object({
  content: z.string().min(1).describe('Kubernetes manifest YAML (may contain multiple documents)'),
  context: z
    .object({
      environment: z.enum(['dev', 'staging', 'production']).optional(),
    })
    .strict()
    .optional(),
});

export type ValidateK8sManifestInput = z.infer<typeof validateK8sManifestSchema>;
