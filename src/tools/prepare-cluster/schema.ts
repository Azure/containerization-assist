import { z } from 'zod';
import { environmentSchema } from '@/config/constants';
import {
  platform,
  workspacePath,
  type DockerPlatform,
  type ToolNextAction,
} from '../shared/schemas';
import type { PolicyValidationResult } from '@/lib/policy-helpers';

/**
 * Cluster type determines infrastructure setup behavior.
 * - `kind`: Create/manage a local Kind cluster with local Docker registry
 * - `generic`: Assume an existing cluster (AKS, EKS, GKE, minikube, etc.)
 */
export const clusterTypeSchema = z
  .enum(['kind', 'generic'])
  .optional()
  .describe(
    'Cluster type to prepare. "kind" creates a local Kind cluster with a local Docker registry. "generic" assumes an existing cluster (AKS, EKS, GKE, minikube, etc.). Defaults to inferring from environment if omitted.',
  );

export type ClusterType = z.infer<typeof clusterTypeSchema>;

export const prepareClusterSchema = z.object({
  clusterType: clusterTypeSchema,
  workspacePath: workspacePath.optional(),
  environment: environmentSchema
    .optional()
    .describe(
      'Target environment for knowledge filtering and policy context. Does not control cluster setup — use clusterType for that.',
    ),
  namespace: z.string().optional().describe('Kubernetes namespace'),
  targetPlatform: platform.describe(
    'Target platform for cluster validation. Ensures the cluster can run images built for this platform. Defaults to linux/amd64.',
  ),
  strictPlatformValidation: z
    .boolean()
    .optional()
    .default(true)
    .describe(
      'Fail if a detected cluster architecture does not match the target platform. When true, an incompatible cluster (whose platform was detected during probing) blocks the plan to prevent deploying images that would require emulation. If the cluster does not exist yet (platform not detectable), the tool stays advisory and returns guidance instead of failing. Set to false to allow emulation (may have performance impact).',
    ),
});

export type PrepareClusterParams = z.infer<typeof prepareClusterSchema>;

/**
 * A single read-only-derived setup command the agent should run to prepare the cluster.
 * The tool never executes these — it returns them for the calling agent to run via its
 * own terminal tools.
 */
export interface ClusterSetupCommand {
  /** Exact shell command to run (cluster name already validated/escaped) */
  command: string;
  /** Human-readable goal of this command (e.g., "Create the Kind cluster") */
  goal: string;
  /** Whether this command can be safely skipped (e.g., already-satisfied state) */
  optional?: boolean;
}

/**
 * A Kubernetes manifest the agent should apply (e.g., via `kubectl apply -f -`).
 * Emitted instead of the tool applying it directly.
 */
export interface ClusterManifestPlan {
  /** Kubernetes kind (e.g., "ServiceAccount", "ConfigMap") */
  kind: string;
  /** Target namespace for the manifest */
  namespace: string;
  /** Full manifest YAML to apply */
  yaml: string;
}

/**
 * Platform compatibility guidance between the target platform and the detected cluster.
 */
export interface ClusterPlatformGuidance {
  /** Target platform the images are built for (e.g., "linux/amd64") */
  target: DockerPlatform;
  /** Detected cluster platform, or null when no cluster exists / not detectable yet */
  cluster: DockerPlatform | null;
  /** Whether target and cluster platforms are compatible */
  compatible: boolean;
  /** Whether running the workload would require emulation */
  requiresEmulation: boolean;
  /** Human-readable note explaining the compatibility assessment */
  note: string;
}

/**
 * Advisory result of the prepare-cluster tool. Mirrors `DockerfilePlan`: instead of
 * executing, the tool probes read-only state and returns the goal + exact commands and
 * manifests the agent should run to prepare the cluster.
 */
export interface ClusterPlan {
  /** Next action directive — uses the existing `review-and-decide` action; commands are conveyed in the instruction + recommendations */
  nextAction: ToolNextAction;
  /** Resolved cluster type the plan targets */
  clusterType: 'kind' | 'generic';
  /** Read-only detected state of the host/cluster used to compute the plan */
  detected: {
    kindInstalled: boolean;
    clusterExists: boolean;
    namespaceExists: boolean;
    registryRunning: boolean;
    clusterPlatform: DockerPlatform | null;
  };
  recommendations: {
    /** Ordered commands the agent should run; only the ones needed for the detected state */
    setupCommands: ClusterSetupCommand[];
    /** Manifests the agent should apply */
    manifests: ClusterManifestPlan[];
    /** Platform compatibility guidance */
    platformGuidance: ClusterPlatformGuidance;
  };
  /** Non-fatal warnings surfaced to the agent */
  warnings?: string[];
  /** Confidence score (0–1) in the generated plan */
  confidence: number;
  /** Short, dense summary (e.g., "🔧 ACTION REQUIRED: Prepare cluster ...") */
  summary: string;
  /** Optional policy validation of the plan (deferred/future) */
  policyValidation?: PolicyValidationResult;
}
