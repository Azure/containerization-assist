import { z } from 'zod';

export const generateKustomizeSchema = z.object({
  baseManifests: z.string().describe('Base YAML manifests to organize with Kustomize'),
  outputPath: z.string().optional().describe('Directory to write Kustomize structure (optional)'),
  environments: z
    .array(z.enum(['dev', 'staging', 'test', 'prod']))
    .default(['dev', 'prod'])
    .describe('Environments to create overlays for'),
  sessionId: z.string().optional().describe('Session ID for workflow continuity'),

  // Base configuration
  namespace: z.string().optional().describe('Default namespace for all resources'),
  namePrefix: z.string().optional().describe('Prefix to add to all resource names'),
  nameSuffix: z.string().optional().describe('Suffix to add to all resource names'),
  commonLabels: z
    .record(z.string(), z.string())
    .optional()
    .describe('Labels to add to all resources'),
  commonAnnotations: z
    .record(z.string(), z.string())
    .optional()
    .describe('Annotations to add to all resources'),

  // Environment-specific overrides
  envConfig: z
    .record(
      z.string(),
      z.object({
        namespace: z.string().optional(),
        replicas: z.number().optional(),
        resources: z
          .object({
            requests: z
              .object({
                cpu: z.string().optional(),
                memory: z.string().optional(),
              })
              .optional(),
            limits: z
              .object({
                cpu: z.string().optional(),
                memory: z.string().optional(),
              })
              .optional(),
          })
          .optional(),
        patches: z
          .array(
            z.object({
              target: z.object({
                kind: z.string().optional(),
                name: z.string().optional(),
              }),
              patch: z.string(),
            }),
          )
          .optional(),
      }),
    )
    .optional()
    .describe('Environment-specific configuration overrides'),
});
