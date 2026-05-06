import { z } from 'zod';

/**
 * Discriminator for the kind of artifact being validated.
 * The Rego evaluator dispatches on document shape rather than this hint,
 * but the field is part of the public contract so callers can be explicit
 * (and so future evaluators can specialise on it).
 */
export const validateKindSchema = z.enum(['dockerfile', 'k8s-manifest', 'compose']);
export type ValidateKind = z.infer<typeof validateKindSchema>;

export const validateSchema = z.object({
  kind: validateKindSchema.describe(
    'The kind of artifact in `content`: "dockerfile", "k8s-manifest", or "compose".',
  ),
  content: z
    .string()
    .min(1)
    .describe(
      'Raw text of the artifact to validate (Dockerfile, Kubernetes manifest YAML, or docker-compose YAML).',
    ),
});

export type ValidateInput = z.infer<typeof validateSchema>;
