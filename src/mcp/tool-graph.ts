/**
 * Tool dependency graph for router-first architecture
 */

/**
 * Execution steps that tools can require or provide
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
 * Tool dependency edge definition
 */
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
        buildParams: (params: any) => any;
      }
    >
  >;
}

/**
 * Complete tool dependency graph
 */
export const TOOL_GRAPH: Record<string, ToolEdge> = {
  'analyze-repo': {
    provides: ['analyzed_repo'],
  },

  'resolve-base-images': {
    requires: ['analyzed_repo'],
    provides: ['resolved_base_images'],
    autofix: {
      analyzed_repo: {
        tool: 'analyze-repo',
        buildParams: (p) => ({ path: p.path || '.' }),
      },
    },
  },

  'generate-dockerfile': {
    requires: ['analyzed_repo', 'resolved_base_images'],
    provides: ['dockerfile_generated'],
    autofix: {
      analyzed_repo: {
        tool: 'analyze-repo',
        buildParams: (p) => ({ path: p.path || '.' }),
      },
      resolved_base_images: {
        tool: 'resolve-base-images',
        buildParams: (p) => ({ path: p.path || '.' }),
      },
    },
  },

  'fix-dockerfile': {
    requires: ['dockerfile_generated'],
    provides: ['dockerfile_generated'],
    autofix: {
      dockerfile_generated: {
        tool: 'generate-dockerfile',
        buildParams: (p) => ({ path: p.path || '.' }),
      },
    },
  },

  'build-image': {
    requires: ['analyzed_repo', 'dockerfile_generated'],
    provides: ['built_image'],
    autofix: {
      analyzed_repo: {
        tool: 'analyze-repo',
        buildParams: (p) => ({ path: p.path || '.' }),
      },
      dockerfile_generated: {
        tool: 'generate-dockerfile',
        buildParams: (p) => ({ path: p.path || '.' }),
      },
    },
  },

  'scan-image': {
    requires: ['built_image'],
    provides: ['scanned_image'],
    autofix: {
      built_image: {
        tool: 'build-image',
        buildParams: (p) => ({
          path: p.path || '.',
          imageId: p.imageId,
        }),
      },
    },
  },

  'tag-image': {
    requires: ['built_image'],
  },

  'push-image': {
    requires: ['built_image'],
    provides: ['pushed_image'],
    autofix: {
      built_image: {
        tool: 'build-image',
        buildParams: (p) => ({
          path: p.path || '.',
          imageId: p.imageId,
        }),
      },
    },
  },

  'prepare-cluster': {
    provides: ['k8s_prepared'],
  },

  'generate-k8s-manifests': {
    requires: ['analyzed_repo'],
    provides: ['manifests_generated'],
    autofix: {
      analyzed_repo: {
        tool: 'analyze-repo',
        buildParams: (p) => ({ path: p.path || '.' }),
      },
    },
  },

  'generate-helm-charts': {
    requires: ['analyzed_repo'],
    provides: ['helm_charts_generated', 'manifests_generated'],
    autofix: {
      analyzed_repo: {
        tool: 'analyze-repo',
        buildParams: (p) => ({ path: p.path || '.' }),
      },
    },
  },

  'generate-aca-manifests': {
    requires: ['analyzed_repo'],
    provides: ['aca_manifests_generated', 'manifests_generated'],
    autofix: {
      analyzed_repo: {
        tool: 'analyze-repo',
        buildParams: (p) => ({ path: p.path || '.' }),
      },
    },
  },

  deploy: {
    requires: ['built_image', 'k8s_prepared', 'manifests_generated'],
    provides: ['deployed'],
    autofix: {
      built_image: {
        tool: 'build-image',
        buildParams: (p) => ({
          path: p.path || '.',
          imageId: p.imageId,
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
        }),
      },
    },
  },

  'verify-deployment': {
    requires: ['deployed'],
  },

  // Tools without dependencies
  workflow: {},
  ops: {},
  'inspect-session': {},
  'convert-aca-to-k8s': {},
};

/**
 * Get tool edge configuration
 */
export function getToolEdge(toolName: string): ToolEdge | undefined {
  return TOOL_GRAPH[toolName];
}

/**
 * Check if a step is satisfied in session
 */
export function isStepSatisfied(step: Step, completedSteps: Set<Step>): boolean {
  return completedSteps.has(step);
}

/**
 * Get missing preconditions for a tool
 */
export function getMissingPreconditions(toolName: string, completedSteps: Set<Step>): Step[] {
  const edge = getToolEdge(toolName);
  if (!edge?.requires) return [];

  return edge.requires.filter((step) => !isStepSatisfied(step, completedSteps));
}

/**
 * Determine execution order for missing preconditions
 */
export function getExecutionOrder(
  missingSteps: Step[],
  completedSteps: Set<Step>,
): { tool: string; step: Step }[] {
  const order: { tool: string; step: Step }[] = [];
  const toProcess = new Set(missingSteps);
  const processed = new Set<Step>(completedSteps);
  const expansionAttempts = new Map<Step, number>();
  const maxExpansions = 10; // Prevent infinite expansion

  while (toProcess.size > 0) {
    let madeProgress = false;

    for (const step of toProcess) {
      // Find a tool that provides this step
      const provider = Object.entries(TOOL_GRAPH).find(([_, edge]) =>
        edge.provides?.includes(step),
      );

      if (!provider) {
        throw new Error(`No tool provides step: ${step}`);
      }

      const [toolName, edge] = provider;

      // Check if all requirements are satisfied
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
      // Track expansion attempts to detect circular dependencies
      let newDepsAdded = false;

      // Find unsatisfied dependencies
      for (const step of toProcess) {
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

      // If no new dependencies were added and we still can't make progress
      if (!newDepsAdded && toProcess.size > 0) {
        throw new Error(
          `Circular dependency detected for steps: ${Array.from(toProcess).join(', ')}`,
        );
      }
    }
  }

  return order;
}
