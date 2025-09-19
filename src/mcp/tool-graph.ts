/**
 * Tool dependency graph enabling automatic precondition satisfaction.
 *
 * Invariant: Graph must be acyclic to prevent infinite execution loops
 * Trade-off: Static graph definition vs runtime discovery - chose static for predictability
 */

import { ToolName } from '@/exports/tools';

/**
 * Execution steps representing workflow state transitions.
 * Each step marks a completed capability that downstream tools can depend on.
 */
export type Step =
  | 'analyzed_repo'
  | 'resolved_base_images'
  | 'dockerfile_generated'
  | 'built_image'
  | 'scanned_image'
  | 'pushed_image'
  | 'k8s_prepared'
  | 'manifests_generated'
  | 'helm_charts_generated'
  | 'aca_manifests_generated'
  | 'deployed';

/**
 * Dependency edge defining tool's preconditions and effects.
 * Enables automatic workflow orchestration through declarative dependencies.
 */
// Common parameters that most tools accept
interface CommonToolParams {
  path?: string;
  sessionId?: string;
  imageId?: string;
  imageName?: string;
  tag?: string;
  registry?: string;
  namespace?: string;
  technology?: string;
  language?: string;
  framework?: string;
  [key: string]: unknown;
}

export interface ToolEdge {
  /** Preconditions that must be satisfied before tool execution */
  requires?: Step[];

  /** Effects provided when tool succeeds */
  provides?: Step[];

  /** Auto-correction mapping for missing preconditions */
  autofix?: Partial<
    Record<
      Step,
      {
        tool: string;
        buildParams: (params: CommonToolParams) => Record<string, unknown>;
      }
    >
  >;

  /** Suggested next tools after successful execution */
  nextSteps?: {
    tool: string;
    description: string;
    buildParams?: (params: CommonToolParams) => Record<string, unknown>;
  }[];
}

/**
 * Static dependency graph mapping tools to their workflow requirements.
 * Central source of truth for tool orchestration and auto-correction.
 */
export const TOOL_GRAPH: Record<ToolName, ToolEdge> = {
  'analyze-repo': {
    provides: ['analyzed_repo'],
    nextSteps: [
      {
        tool: 'generate-dockerfile',
        description: 'Generate optimized Dockerfile based on the repository analysis',
        buildParams: (p) => ({
          path: p.path || '.',
          sessionId: p.sessionId, // Pass the same sessionId to share analysis data
        }),
      },
    ],
  },

  'resolve-base-images': {
    requires: ['analyzed_repo'],
    provides: ['resolved_base_images'],
    autofix: {
      analyzed_repo: {
        tool: 'analyze-repo',
        buildParams: (p) => ({
          path: p.path || '.',
          sessionId: p.sessionId,
        }),
      },
    },
    nextSteps: [
      {
        tool: 'generate-dockerfile',
        description: 'Generate Dockerfile using the resolved base images',
        buildParams: (p) => ({
          path: p.path || '.',
          sessionId: p.sessionId,
        }),
      },
    ],
  },

  'generate-dockerfile': {
    requires: ['analyzed_repo', 'resolved_base_images'],
    provides: ['dockerfile_generated'],
    autofix: {
      analyzed_repo: {
        tool: 'analyze-repo',
        buildParams: (p) => ({
          path: p.path || '.',
          sessionId: p.sessionId,
        }),
      },
      resolved_base_images: {
        tool: 'resolve-base-images',
        buildParams: (p) => ({
          path: p.path || '.',
          technology: p.technology,
          language: p.language,
          framework: p.framework,
          sessionId: p.sessionId,
        }),
      },
    },
    nextSteps: [
      {
        tool: 'build-image',
        description: 'Build Docker image from the generated Dockerfile',
        buildParams: (p) => ({
          path: p.path || '.',
          imageName: p.imageName || p.imageId || 'app:latest',
          sessionId: p.sessionId,
        }),
      },
      {
        tool: 'fix-dockerfile',
        description: 'Optimize or fix issues in the generated Dockerfile',
        buildParams: (p) => ({
          path: p.path || '.',
          sessionId: p.sessionId,
        }),
      },
    ],
  },

  'fix-dockerfile': {
    requires: ['dockerfile_generated'],
    provides: ['dockerfile_generated'],
    autofix: {
      dockerfile_generated: {
        tool: 'generate-dockerfile',
        buildParams: (p) => ({
          path: p.path || '.',
          sessionId: p.sessionId,
        }),
      },
    },
    nextSteps: [
      {
        tool: 'build-image',
        description: 'Build Docker image from the fixed Dockerfile',
        buildParams: (p) => ({
          path: p.path || '.',
          imageName: p.imageName || 'app:latest',
          sessionId: p.sessionId,
        }),
      },
    ],
  },

  'build-image': {
    requires: ['analyzed_repo', 'dockerfile_generated'],
    provides: ['built_image'],
    autofix: {
      analyzed_repo: {
        tool: 'analyze-repo',
        buildParams: (p) => ({
          path: p.path || '.',
          sessionId: p.sessionId,
        }),
      },
      dockerfile_generated: {
        tool: 'generate-dockerfile',
        buildParams: (p) => ({
          path: p.path || '.',
          sessionId: p.sessionId,
        }),
      },
    },
    nextSteps: [
      {
        tool: 'scan',
        description: 'Scan the built image for security vulnerabilities',
        buildParams: (p) => ({
          imageId: p.imageId || p.imageName || p.tag,
          sessionId: p.sessionId,
        }),
      },
      {
        tool: 'push-image',
        description: 'Push the built image to a container registry',
        buildParams: (p) => ({
          imageId: p.imageId || p.imageName || p.tag,
          registry: p.registry,
          sessionId: p.sessionId,
        }),
      },
      {
        tool: 'deploy',
        description: 'Deploy the containerized application to Kubernetes',
        buildParams: (p) => ({
          imageId: p.imageId || p.imageName || p.tag,
          namespace: p.namespace || 'default',
          sessionId: p.sessionId,
        }),
      },
    ],
  },

  scan: {
    requires: ['built_image'],
    provides: ['scanned_image'],
    autofix: {
      built_image: {
        tool: 'build-image',
        buildParams: (p) => ({
          path: p.path || '.',
          imageName: p.imageId || p.imageName || 'app:latest',
          sessionId: p.sessionId,
        }),
      },
    },
    nextSteps: [
      {
        tool: 'push-image',
        description: 'Push the scanned image to a container registry',
        buildParams: (p) => ({
          imageId: p.imageId || p.imageName,
          registry: p.registry,
          sessionId: p.sessionId,
        }),
      },
      {
        tool: 'deploy',
        description: 'Deploy the scanned application to Kubernetes',
        buildParams: (p) => ({
          imageId: p.imageId || p.imageName,
          namespace: p.namespace || 'default',
          sessionId: p.sessionId,
        }),
      },
    ],
  },

  'tag-image': {
    requires: ['built_image'],
    nextSteps: [
      {
        tool: 'push-image',
        description: 'Push the tagged image to a container registry',
        buildParams: (p) => ({
          imageId: p.imageId || p.imageName,
          registry: p.registry,
          sessionId: p.sessionId,
        }),
      },
    ],
  },

  'push-image': {
    requires: ['built_image'],
    provides: ['pushed_image'],
    autofix: {
      built_image: {
        tool: 'build-image',
        buildParams: (p) => ({
          path: p.path || '.',
          imageName: p.imageId || p.imageName || 'app:latest',
          sessionId: p.sessionId,
        }),
      },
    },
    nextSteps: [
      {
        tool: 'deploy',
        description: 'Deploy the pushed image to Kubernetes',
        buildParams: (p) => ({
          imageId: p.imageId || p.imageName,
          namespace: p.namespace || 'default',
          sessionId: p.sessionId,
        }),
      },
    ],
  },

  'prepare-cluster': {
    provides: ['k8s_prepared'],
    nextSteps: [
      {
        tool: 'generate-k8s-manifests',
        description: 'Generate Kubernetes manifests for deployment',
        buildParams: (p) => ({
          path: p.path || '.',
          sessionId: p.sessionId,
        }),
      },
    ],
  },

  'generate-k8s-manifests': {
    requires: ['analyzed_repo'],
    provides: ['manifests_generated'],
    autofix: {
      analyzed_repo: {
        tool: 'analyze-repo',
        buildParams: (p) => ({
          path: p.path || '.',
          sessionId: p.sessionId,
        }),
      },
    },
    nextSteps: [
      {
        tool: 'deploy',
        description: 'Deploy the application using the generated manifests',
        buildParams: (p) => ({
          path: p.path || '.',
          namespace: p.namespace || 'default',
          sessionId: p.sessionId,
        }),
      },
    ],
  },

  'generate-helm-charts': {
    requires: ['analyzed_repo'],
    provides: ['helm_charts_generated', 'manifests_generated'],
    autofix: {
      analyzed_repo: {
        tool: 'analyze-repo',
        buildParams: (p) => ({
          path: p.path || '.',
          sessionId: p.sessionId,
        }),
      },
    },
    nextSteps: [
      {
        tool: 'deploy',
        description: 'Deploy the application using the generated Helm charts',
        buildParams: (p) => ({
          path: p.path || '.',
          namespace: p.namespace || 'default',
          sessionId: p.sessionId,
        }),
      },
    ],
  },

  'generate-aca-manifests': {
    requires: ['analyzed_repo'],
    provides: ['aca_manifests_generated', 'manifests_generated'],
    autofix: {
      analyzed_repo: {
        tool: 'analyze-repo',
        buildParams: (p) => ({
          path: p.path || '.',
          sessionId: p.sessionId,
        }),
      },
    },
    nextSteps: [
      {
        tool: 'deploy',
        description: 'Deploy the application using the generated Azure Container Apps manifests',
        buildParams: (p) => ({
          path: p.path || '.',
          sessionId: p.sessionId,
        }),
      },
    ],
  },

  deploy: {
    requires: ['built_image', 'k8s_prepared', 'manifests_generated'],
    provides: ['deployed'],
    autofix: {
      built_image: {
        tool: 'build-image',
        buildParams: (p) => ({
          path: p.path || '.',
          imageName: p.imageId || p.imageName || 'app:latest',
          sessionId: p.sessionId,
        }),
      },
      k8s_prepared: {
        tool: 'prepare-cluster',
        buildParams: () => ({}),
      },
      manifests_generated: {
        tool: 'generate-k8s-manifests',
        buildParams: (p) => ({
          path: p.path || '.',
          imageId: p.imageId,
          sessionId: p.sessionId,
        }),
      },
    },
    nextSteps: [
      {
        tool: 'verify-deploy',
        description: 'Verify that the deployment is running successfully',
        buildParams: (p) => ({
          namespace: p.namespace || 'default',
          sessionId: p.sessionId,
        }),
      },
    ],
  },

  'verify-deploy': {
    requires: ['deployed'],
  },

  ops: {},
  'inspect-session': {},
  'convert-aca-to-k8s': {},
};

/**
 * Retrieves dependency configuration for a tool.
 * Returns undefined for tools without workflow dependencies.
 */
export function getToolEdge(toolName: ToolName): ToolEdge | undefined {
  return TOOL_GRAPH[toolName];
}

/**
 * Verifies if a workflow step has been completed.
 *
 * Precondition: completedSteps must be accurately maintained by session manager
 */
export function isStepSatisfied(step: Step, completedSteps: Set<Step>): boolean {
  return completedSteps.has(step);
}

/**
 * Identifies unsatisfied preconditions blocking tool execution.
 *
 * Postcondition: Returns empty array if tool can execute immediately
 */
export function getMissingPreconditions(toolName: ToolName, completedSteps: Set<Step>): Step[] {
  const edge = getToolEdge(toolName);
  if (!edge?.requires) return [];

  return edge.requires.filter((step) => !isStepSatisfied(step, completedSteps));
}

/**
 * Computes topological ordering of tools to satisfy preconditions.
 *
 * Invariant: Throws if circular dependencies detected (max 10 expansion attempts)
 * Failure Mode: Circular dependency results in explicit error, not infinite loop
 */
export function getExecutionOrder(
  missingSteps: Step[],
  completedSteps: Set<Step>,
): { tool: ToolName; step: Step }[] {
  const order: { tool: ToolName; step: Step }[] = [];
  const toProcess = new Set(missingSteps);
  const processed = new Set<Step>(completedSteps);
  const expansionAttempts = new Map<Step, number>();
  const maxExpansions = 10; // Prevent infinite expansion

  while (toProcess.size > 0) {
    let madeProgress = false;

    for (const step of Array.from(toProcess)) {
      const provider = Object.entries(TOOL_GRAPH).find(([_, edge]) =>
        edge.provides?.includes(step),
      ) as [ToolName, ToolEdge] | undefined;

      if (!provider) {
        throw new Error(`No tool provides step: ${step}`);
      }

      const [toolName, edge] = provider;

      const toolRequirements = edge.requires || [];
      const allSatisfied = toolRequirements.every((req) => processed.has(req));

      if (allSatisfied) {
        order.push({ tool: toolName, step });
        toProcess.delete(step);
        processed.add(step);
        madeProgress = true;
      }
    }

    if (!madeProgress) {
      // Expansion tracking prevents infinite loops from circular dependencies
      let newDepsAdded = false;

      for (const step of Array.from(toProcess)) {
        const attempts = expansionAttempts.get(step) || 0;
        if (attempts >= maxExpansions) {
          throw new Error(`Circular dependency detected: ${step} expanded ${attempts} times`);
        }
        expansionAttempts.set(step, attempts + 1);

        const provider = Object.entries(TOOL_GRAPH).find(([_, edge]) =>
          edge.provides?.includes(step),
        );

        if (provider) {
          const [_, edge] = provider;
          const missing = edge.requires?.filter((req) => !processed.has(req)) || [];
          missing.forEach((m) => {
            if (!toProcess.has(m) && !processed.has(m)) {
              toProcess.add(m);
              newDepsAdded = true;
            }
          });
        }
      }

      // Deadlock detection: no progress and no new dependencies = circular reference
      if (!newDepsAdded && toProcess.size > 0) {
        throw new Error(
          `Circular dependency detected for steps: ${Array.from(toProcess).join(', ')}`,
        );
      }
    }
  }

  return order;
}
