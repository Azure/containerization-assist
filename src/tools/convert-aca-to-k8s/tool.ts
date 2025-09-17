/**
 * Convert Azure Container Apps to Kubernetes Tool
 *
 * Converts Azure Container Apps manifests to Kubernetes manifests
 * Simple, direct conversion without over-abstraction
 */

import { joinPaths } from '@/lib/path-utils';
import { getToolLogger, createToolTimer } from '@/lib/tool-helpers';
import { extractErrorMessage } from '@/lib/error-utils';
import { promises as fs } from 'node:fs';
import { ensureSession, defineToolIO, useSessionSlice } from '@/mcp/tool-session-helpers';
import type { ToolContext } from '@/mcp/context';
import { Success, Failure, type Result } from '@/types';
import * as yaml from 'js-yaml';
import { convertAcaToK8sSchema, type ConvertAcaToK8sParams } from './schema';
import { z } from 'zod';

// Define the result schema for type safety
const ConvertAcaToK8sResultSchema = z.object({
  manifests: z.string(),
  outputPath: z.string(),
  resourceCount: z.number(),
  resources: z.array(
    z.object({
      kind: z.string(),
      name: z.string(),
      namespace: z.string(),
    }),
  ),
  sessionId: z.string().optional(),
});

// Define tool IO for type-safe session operations
const io = defineToolIO(convertAcaToK8sSchema, ConvertAcaToK8sResultSchema);

// Tool-specific state schema
const StateSchema = z.object({
  lastConvertedAt: z.date().optional(),
  conversionCount: z.number().optional(),
  lastAppName: z.string().optional(),
  lastNamespace: z.string().optional(),
  resourceTypes: z.array(z.string()).optional(),
});

/**
 * Result from ACA to K8s conversion
 */
export interface ConvertAcaToK8sResult {
  /** Generated K8s manifests as YAML */
  manifests: string;
  /** Output directory path */
  outputPath: string;
  /** Number of resources generated */
  resourceCount: number;
  /** List of generated resources */
  resources: Array<{
    kind: string;
    name: string;
    namespace: string;
  }>;
  /** Session ID for reference */
  sessionId?: string;
}

/**
 * Kubernetes resource type
 */
interface K8sResource {
  apiVersion: string;
  kind: string;
  metadata: {
    name: string;
    namespace?: string;
    labels?: Record<string, string>;
    annotations?: Record<string, string>;
  };
  spec?: Record<string, unknown>;
  data?: Record<string, string>;
}

/**
 * Convert ACA to K8s implementation - single file, straightforward conversion
 */
async function convertAcaToK8sImpl(
  params: ConvertAcaToK8sParams,
  context: ToolContext,
): Promise<Result<ConvertAcaToK8sResult>> {
  const logger = getToolLogger(context, 'convert-aca-to-k8s');
  const timer = createToolTimer(logger, 'convert-aca-to-k8s');

  try {
    // Parse ACA manifest
    let aca: any;
    try {
      aca = JSON.parse(params.acaManifest);
    } catch {
      // Try YAML parsing as fallback
      try {
        aca = yaml.load(params.acaManifest) as any;
      } catch {
        return Failure('Invalid ACA manifest format - must be valid JSON or YAML');
      }
    }

    // Validate basic ACA structure
    if (!aca.name || !aca.properties?.template) {
      return Failure('Invalid ACA manifest - missing required properties');
    }

    // Direct conversion - no fancy converters
    const k8sResources: K8sResource[] = [];
    const resourceList: Array<{ kind: string; name: string; namespace: string }> = [];

    // 1. Create Deployment from ACA template
    const containers = aca.properties.template.containers || [];
    if (containers.length === 0) {
      return Failure('ACA manifest must have at least one container');
    }

    const deployment: K8sResource = {
      apiVersion: 'apps/v1',
      kind: 'Deployment',
      metadata: {
        name: aca.name,
        namespace: params.namespace ?? 'default',
        labels: { app: aca.name },
        ...(params.includeComments && {
          annotations: {
            'converted-from': 'azure-container-apps',
            'original-location': aca.location || 'unknown',
          },
        }),
      },
      spec: {
        replicas: aca.properties.template.scale?.minReplicas || 1,
        selector: { matchLabels: { app: aca.name } },
        template: {
          metadata: {
            labels: { app: aca.name },
            ...(aca.properties.configuration?.dapr?.enabled && {
              annotations: {
                'dapr.io/enabled': 'true',
                'dapr.io/app-id': aca.properties.configuration.dapr.appId || aca.name,
                'dapr.io/app-port': String(aca.properties.configuration.dapr.appPort || ''),
                'dapr.io/app-protocol': aca.properties.configuration.dapr.appProtocol || 'http',
              },
            }),
          },
          spec: {
            containers: containers.map((c: any) => {
              const livenessProbe = c.probes?.find((p: any) => p.type === 'liveness');
              const readinessProbe = c.probes?.find((p: any) => p.type === 'readiness');
              const startupProbe = c.probes?.find((p: any) => p.type === 'startup');
              return {
                name: c.name || aca.name,
                image: c.image,
                ...(c.resources && {
                  resources: {
                    requests: {
                      cpu: String(c.resources.cpu || 0.5),
                      memory: c.resources.memory || '1Gi',
                    },
                    limits: {
                      cpu: String(c.resources.cpu || 0.5),
                      memory: c.resources.memory || '1Gi',
                    },
                  },
                }),
                ...(c.env && { env: c.env }),
                ...(c.ports && {
                  ports: c.ports.map((port: number) => ({ containerPort: port })),
                }),
                ...(c.probes && {
                  livenessProbe: livenessProbe?.httpGet && {
                    httpGet: livenessProbe.httpGet,
                    initialDelaySeconds: livenessProbe.initialDelaySeconds || 30,
                    periodSeconds: livenessProbe.periodSeconds || 30,
                  },
                  readinessProbe: readinessProbe?.httpGet && {
                    httpGet: readinessProbe.httpGet,
                    initialDelaySeconds: readinessProbe.initialDelaySeconds || 5,
                    periodSeconds: readinessProbe.periodSeconds || 10,
                  },
                  startupProbe: startupProbe?.httpGet && {
                    httpGet: startupProbe.httpGet,
                    initialDelaySeconds: startupProbe.initialDelaySeconds || 0,
                    periodSeconds: startupProbe.periodSeconds || 10,
                  },
                }),
              };
            }),
          },
        },
      },
    };

    k8sResources.push(deployment);
    resourceList.push({
      kind: 'Deployment',
      name: aca.name,
      namespace: params.namespace ?? 'default',
    });

    // 2. Create Service if ingress exists
    if (aca.properties?.configuration?.ingress) {
      const ingress = aca.properties.configuration.ingress;
      const service: K8sResource = {
        apiVersion: 'v1',
        kind: 'Service',
        metadata: {
          name: aca.name,
          namespace: params.namespace ?? 'default',
          labels: { app: aca.name },
          ...(params.includeComments && {
            annotations: {
              'converted-from': 'aca-ingress',
              'original-external': String(ingress.external || false),
            },
          }),
        },
        spec: {
          type: ingress.external ? 'LoadBalancer' : 'ClusterIP',
          selector: { app: aca.name },
          ports: [
            {
              port: ingress.targetPort || 80,
              targetPort: ingress.targetPort || 80,
              protocol: ingress.transport === 'tcp' ? 'TCP' : 'TCP',
              name: ingress.transport === 'tcp' ? 'tcp' : 'http',
            },
          ],
        },
      };
      k8sResources.push(service);
      resourceList.push({
        kind: 'Service',
        name: aca.name,
        namespace: params.namespace ?? 'default',
      });

      // 3. Create Ingress for HTTP with external access
      if (ingress.external && ingress.transport !== 'tcp') {
        const k8sIngress: K8sResource = {
          apiVersion: 'networking.k8s.io/v1',
          kind: 'Ingress',
          metadata: {
            name: aca.name,
            namespace: params.namespace ?? 'default',
            labels: { app: aca.name },
            annotations: {
              'nginx.ingress.kubernetes.io/rewrite-target': '/',
              ...(params.includeComments && {
                'converted-from': 'aca-ingress-external',
              }),
            },
          },
          spec: {
            rules: [
              {
                host: `${aca.name}.example.com`,
                http: {
                  paths: [
                    {
                      path: '/',
                      pathType: 'Prefix',
                      backend: {
                        service: {
                          name: aca.name,
                          port: {
                            number: ingress.targetPort || 80,
                          },
                        },
                      },
                    },
                  ],
                },
              },
            ],
          },
        };
        k8sResources.push(k8sIngress);
        resourceList.push({
          kind: 'Ingress',
          name: aca.name,
          namespace: params.namespace ?? 'default',
        });
      }
    }

    // 4. Create HPA if scaling rules exist
    if (
      aca.properties?.template?.scale?.maxReplicas &&
      aca.properties.template.scale.maxReplicas > 1
    ) {
      const hpa: K8sResource = {
        apiVersion: 'autoscaling/v2',
        kind: 'HorizontalPodAutoscaler',
        metadata: {
          name: `${aca.name}-hpa`,
          namespace: params.namespace ?? 'default',
          labels: { app: aca.name },
          ...(params.includeComments && {
            annotations: {
              'converted-from': 'aca-scale-rules',
            },
          }),
        },
        spec: {
          scaleTargetRef: {
            apiVersion: 'apps/v1',
            kind: 'Deployment',
            name: aca.name,
          },
          minReplicas: aca.properties.template.scale.minReplicas || 1,
          maxReplicas: aca.properties.template.scale.maxReplicas,
          metrics: [
            {
              type: 'Resource',
              resource: {
                name: 'cpu',
                target: {
                  type: 'Utilization',
                  averageUtilization: 70,
                },
              },
            },
          ],
        },
      };
      k8sResources.push(hpa);
      resourceList.push({
        kind: 'HorizontalPodAutoscaler',
        name: `${aca.name}-hpa`,
        namespace: params.namespace ?? 'default',
      });
    }

    // 5. Create Secret if ACA has secrets
    if (aca.properties?.configuration?.secrets && aca.properties.configuration.secrets.length > 0) {
      const secret: K8sResource = {
        apiVersion: 'v1',
        kind: 'Secret',
        metadata: {
          name: `${aca.name}-secrets`,
          namespace: params.namespace ?? 'default',
          labels: { app: aca.name },
          ...(params.includeComments && {
            annotations: {
              'converted-from': 'aca-secrets',
            },
          }),
        },
        data: aca.properties.configuration.secrets.reduce((acc: Record<string, string>, s: any) => {
          // Base64 encode secret values
          acc[s.name] = Buffer.from(s.value || '').toString('base64');
          return acc;
        }, {}),
      };
      k8sResources.push(secret);
      resourceList.push({
        kind: 'Secret',
        name: `${aca.name}-secrets`,
        namespace: params.namespace ?? 'default',
      });
    }

    // Add comments if requested
    const comments = params.includeComments
      ? [
          '# Converted from Azure Container Apps manifest',
          `# Original app name: ${aca.name}`,
          `# Generated on: ${new Date().toISOString()}`,
          '# Note: Review and adjust values as needed for your K8s environment',
          '',
        ].join('\n')
      : '';

    // Convert to YAML
    const yamlContent =
      comments +
      k8sResources.map((r) => yaml.dump(r, { noRefs: true, lineWidth: -1 })).join('---\n');

    // Ensure session exists and get typed slice operations
    const sessionResult = await ensureSession(context, params.sessionId);
    if (!sessionResult.ok) {
      return Failure(sessionResult.error);
    }

    const { id: sessionId } = sessionResult.value;
    const slice = useSessionSlice('convert-aca-to-k8s', io, context, StateSchema);

    if (!slice) {
      return Failure('Session manager not available');
    }

    // Record input in session slice
    await slice.patch(sessionId, { input: params });

    // Write to file - use current directory as base
    const outputPath = joinPaths('.', 'k8s-converted');
    await fs.mkdir(outputPath, { recursive: true });
    await fs.writeFile(joinPaths(outputPath, 'manifests.yaml'), yamlContent, 'utf-8');

    // Prepare result
    const result: ConvertAcaToK8sResult = {
      manifests: yamlContent,
      outputPath,
      resourceCount: k8sResources.length,
      resources: resourceList,
      sessionId,
    };

    // Update typed session slice with output and state
    await slice.patch(sessionId, {
      output: result,
      state: {
        lastConvertedAt: new Date(),
        conversionCount: 1,
        lastAppName: aca.name,
        lastNamespace: params.namespace ?? 'default',
        resourceTypes: k8sResources.map((r) => r.kind),
      },
    });

    timer.end({ resourceCount: k8sResources.length });

    return Success(result);
  } catch (error) {
    timer.error(error);
    logger.error({ error }, 'ACA to K8s conversion failed');
    return Failure(extractErrorMessage(error));
  }
}

/**
 * Convert Azure Container Apps to Kubernetes tool
 */
export const convertAcaToK8s = convertAcaToK8sImpl;
