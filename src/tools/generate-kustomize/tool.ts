/**
 * Generate Kustomize Tool
 *
 * Generates Kustomize directory structures with base and overlay configurations
 * for multi-environment Kubernetes deployments. Supports environment-specific
 * configurations, patches, and resource management.
 *
 * @category kubernetes
 * @version 1.0.0
 */

import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { MCPTool } from '@/types/tool';
import { generateKustomizeSchema } from './schema';
import { promises as fs } from 'node:fs';
import path from 'node:path';
import { parseAllDocuments, stringify as yamlStringify } from 'yaml';
import type { z } from 'zod';

type GenerateKustomizeParams = z.infer<typeof generateKustomizeSchema>;

/**
 * Type for environment patch configuration
 */
type PatchConfig = NonNullable<
  NonNullable<GenerateKustomizeParams['envConfig']>[string]['patches']
>[number];

const name = 'generate-kustomize';
const description =
  'Generate Kustomize structure from Kubernetes manifests for multi-environment deployments';
const version = '1.0.0';

/**
 * Represents a Kubernetes resource object
 */
interface K8sResource {
  apiVersion: string;
  kind: string;
  metadata: {
    name: string;
    namespace?: string;
    [key: string]: unknown;
  };
  [key: string]: unknown;
}

/**
 * Configuration for a Kustomize kustomization.yaml file
 */
interface KustomizationConfig {
  apiVersion?: string;
  kind?: string;
  metadata?: {
    name?: string;
    namespace?: string;
  };
  resources?: string[];
  patchesStrategicMerge?: string[];
  patches?: Array<{
    path?: string;
    patch?: string;
    target?: {
      group?: string;
      version?: string;
      kind?: string;
      name?: string;
    };
  }>;
  images?: Array<{
    name: string;
    newName?: string;
    newTag?: string;
  }>;
  replicas?: Array<{
    name: string;
    count: number;
  }>;
  [key: string]: unknown;
}

/**
 * Complete Kustomize directory structure with base and overlays
 */
interface KustomizeStructure {
  base: {
    kustomization: KustomizationConfig;
    resources: Array<{ filename: string; content: string }>;
  };
  overlays: Record<
    string,
    {
      kustomization: KustomizationConfig;
      patches?: Array<{ filename: string; content: string }>;
    }
  >;
}

/**
 * Generates a Kustomize directory structure with base and environment overlays
 *
 * Parses input Kubernetes manifests and creates a structured Kustomize layout
 * with a base directory containing common resources and overlay directories
 * for environment-specific configurations and patches.
 *
 * @param input - Configuration including manifests, output path, and environments
 * @param ctx - Tool execution context with logger and session
 * @returns Result containing the generated structure and written file paths
 */
async function run(
  input: z.infer<typeof generateKustomizeSchema>,
  ctx: ToolContext,
): Promise<Result<{ structure: KustomizeStructure; files: string[]; sessionId?: string }>> {
  const {
    baseManifests,
    outputPath,
    environments,
    sessionId,
    namespace,
    namePrefix,
    nameSuffix,
    commonLabels,
    commonAnnotations,
    envConfig = {},
  } = input;

  try {
    // Parse manifests into individual resources
    const docs = parseAllDocuments(baseManifests);
    const resources = docs.map((d) => d.toJS()).filter(Boolean);

    if (resources.length === 0) {
      return Failure('No valid Kubernetes resources found in input manifests');
    }

    // Group resources by kind for organized file structure
    const resourcesByKind = resources.reduce(
      (acc, resource: K8sResource) => {
        const kind = resource.kind?.toLowerCase() || 'unknown';
        if (!acc[kind]) acc[kind] = [];
        acc[kind].push(resource);
        return acc;
      },
      {} as Record<string, K8sResource[]>,
    );

    ctx.logger.info(
      {
        totalResources: resources.length,
        kinds: Object.keys(resourcesByKind),
        environments: environments.length,
      },
      'Parsed manifests into resources for Kustomize structure',
    );

    // Create base kustomization
    const baseKustomization: KustomizationConfig = {
      apiVersion: 'kustomize.config.k8s.io/v1beta1',
      kind: 'Kustomization',
      resources: Object.keys(resourcesByKind).map((kind) => `${kind}.yaml`),
    };

    // Add optional base fields
    if (namespace) baseKustomization.namespace = namespace;
    if (namePrefix) baseKustomization.namePrefix = namePrefix;
    if (nameSuffix) baseKustomization.nameSuffix = nameSuffix;
    if (commonLabels && Object.keys(commonLabels).length > 0) {
      baseKustomization.commonLabels = commonLabels;
    }
    if (commonAnnotations && Object.keys(commonAnnotations).length > 0) {
      baseKustomization.commonAnnotations = commonAnnotations;
    }

    // Create base resources
    const baseResources = Object.entries(resourcesByKind).map(([kind, kindResources]) => ({
      filename: `${kind}.yaml`,
      content: (kindResources as Record<string, unknown>[])
        .map((r) => yamlStringify(r))
        .join('---\n'),
    }));

    // Create environment overlays
    const overlays: Record<
      string,
      {
        kustomization: KustomizationConfig;
        patches?: Array<{ filename: string; content: string }>;
      }
    > = {};

    for (const env of environments) {
      const envOverrides =
        envConfig[env] || ({} as NonNullable<GenerateKustomizeParams['envConfig']>[string]);

      const overlay: KustomizationConfig = {
        apiVersion: 'kustomize.config.k8s.io/v1beta1',
        kind: 'Kustomization',
        resources: ['../../base'],
      };

      // Environment-specific namespace
      if (envOverrides.namespace) {
        overlay.namespace = envOverrides.namespace;
      } else if (namespace) {
        overlay.namespace = `${namespace}-${env}`;
      }

      // Replica overrides
      if (envOverrides.replicas !== undefined) {
        overlay.replicas = [{ name: '*', count: envOverrides.replicas }];
      } else {
        // Default replica counts by environment
        const defaultReplicas = {
          dev: 1,
          test: 1,
          staging: 2,
          prod: 3,
        };
        overlay.replicas = [{ name: '*', count: defaultReplicas[env] || 1 }];
      }

      // Resource patches
      const patches: Array<{
        target?: {
          group?: string;
          version?: string;
          kind?: string;
          name?: string;
        };
        patch?: string;
        path?: string;
      }> = [];

      // Resource limits patch
      if (envOverrides.resources) {
        patches.push({
          target: { kind: 'Deployment' },
          patch: yamlStringify([
            {
              op: 'add',
              path: '/spec/template/spec/containers/0/resources',
              value: envOverrides.resources,
            },
          ]),
        });
      }

      // Environment-specific patches
      if (envOverrides.patches) {
        // Transform patches to match KustomizationConfig interface
        patches.push(
          ...envOverrides.patches.map((p: PatchConfig) => ({
            target: {
              ...(p.target.kind && { kind: p.target.kind }),
              ...(p.target.name && { name: p.target.name }),
            },
            patch: p.patch,
          })),
        );
      }

      // Default production hardening
      if (env === 'prod') {
        // Find deployment resources to get app labels
        const deployments = resources.filter((r: K8sResource) => r.kind === 'Deployment');
        const appLabel =
          deployments.length > 0
            ? deployments[0].metadata?.labels?.app || deployments[0].metadata?.name || 'app'
            : 'app';

        patches.push({
          target: { kind: 'Deployment' },
          patch: yamlStringify([
            {
              op: 'add',
              path: '/spec/template/spec/affinity',
              value: {
                podAntiAffinity: {
                  preferredDuringSchedulingIgnoredDuringExecution: [
                    {
                      weight: 100,
                      podAffinityTerm: {
                        topologyKey: 'kubernetes.io/hostname',
                        labelSelector: {
                          matchLabels: { app: appLabel },
                        },
                      },
                    },
                  ],
                },
              },
            },
          ]),
        });
      }

      if (patches.length > 0) {
        overlay.patches = patches;
      }

      overlays[env] = {
        kustomization: overlay,
        patches: [], // Individual patch files if needed
      };
    }

    const structure: KustomizeStructure = {
      base: {
        kustomization: baseKustomization,
        resources: baseResources,
      },
      overlays,
    };

    // Write files if output path provided
    const writtenFiles: string[] = [];

    if (outputPath) {
      try {
        // Create base directory
        const basePath = path.join(outputPath, 'base');
        await fs.mkdir(basePath, { recursive: true });

        // Write base resources
        for (const resource of baseResources) {
          const filePath = path.join(basePath, resource.filename);
          await fs.writeFile(filePath, resource.content, 'utf-8');
          writtenFiles.push(filePath);
        }

        // Write base kustomization
        const baseKustomizationPath = path.join(basePath, 'kustomization.yaml');
        await fs.writeFile(baseKustomizationPath, yamlStringify(baseKustomization), 'utf-8');
        writtenFiles.push(baseKustomizationPath);

        // Write overlays
        for (const [env, overlayConfig] of Object.entries(overlays)) {
          const overlayPath = path.join(outputPath, 'overlays', env);
          await fs.mkdir(overlayPath, { recursive: true });

          const overlayKustomizationPath = path.join(overlayPath, 'kustomization.yaml');
          await fs.writeFile(
            overlayKustomizationPath,
            yamlStringify(overlayConfig.kustomization),
            'utf-8',
          );
          writtenFiles.push(overlayKustomizationPath);

          // Write individual patch files if needed
          if (overlayConfig.patches) {
            for (const patch of overlayConfig.patches) {
              const patchPath = path.join(overlayPath, patch.filename);
              await fs.writeFile(patchPath, patch.content, 'utf-8');
              writtenFiles.push(patchPath);
            }
          }
        }

        ctx.logger.info(
          {
            baseFiles: baseResources.length + 1, // +1 for kustomization.yaml
            environments: environments.length,
            totalFiles: writtenFiles.length,
            outputPath,
          },
          'Kustomize structure written successfully',
        );
      } catch (error) {
        return Failure(`Failed to write Kustomize structure: ${error}`);
      }
    }

    return Success({
      structure,
      files: writtenFiles,
      ...(sessionId && { sessionId }),
    });
  } catch (error) {
    ctx.logger.error({ error }, 'Failed to generate Kustomize structure');
    return Failure(`Kustomize generation failed: ${error}`);
  }
}

const tool: MCPTool<
  typeof generateKustomizeSchema,
  { structure: KustomizeStructure; files: string[]; sessionId?: string }
> = {
  name,
  description,
  category: 'kubernetes',
  version,
  schema: generateKustomizeSchema,
  metadata: {
    knowledgeEnhanced: false,
    samplingStrategy: 'none',
    enhancementCapabilities: [],
  },
  run,
};

export default tool;
