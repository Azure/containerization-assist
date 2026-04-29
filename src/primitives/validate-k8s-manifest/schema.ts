import { z } from 'zod';

export const validateK8sManifestSchema = z.object({
  content: z.string().min(1).describe('Kubernetes manifest YAML (may contain multiple documents)'),
});

export type ValidateK8sManifestInput = z.infer<typeof validateK8sManifestSchema>;
