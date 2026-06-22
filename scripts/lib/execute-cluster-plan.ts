/**
 * Shared helper for integration tests: execute an advisory ClusterPlan.
 *
 * The prepare-cluster tool is advisory — it returns a ClusterPlan describing the
 * commands and manifests needed to prepare the cluster, but does NOT run them.
 * Integration tests that need a real kind cluster + local registry use this helper
 * to actually execute the plan against the host.
 */

import { execSync } from 'child_process';
import type { ClusterPlan } from '../../dist/src/tools/prepare-cluster/schema.js';

/**
 * Derive the local registry URL (host:port) from a kind ClusterPlan.
 * Reads the port from the local-registry-hosting ConfigMap, falling back to the
 * `docker run ... -p PORT:5000` setup command.
 */
export function deriveRegistryUrl(plan: ClusterPlan): string | undefined {
  const configMap = plan.recommendations.manifests.find(
    (m) => m.kind === 'ConfigMap' && m.namespace === 'kube-public',
  );
  if (configMap) {
    const match = configMap.yaml.match(/host:\s*"?(localhost:(\d+))"?/);
    if (match) {
      return match[1];
    }
  }

  const runCmd = plan.recommendations.setupCommands.find((c) => c.command.startsWith('docker run'));
  if (runCmd) {
    const portMatch = runCmd.command.match(/-p\s+(\d+):5000/);
    if (portMatch) {
      return `localhost:${portMatch[1]}`;
    }
  }

  return undefined;
}

/**
 * Execute every required setup command in a ClusterPlan, then attempt optional ones
 * (best-effort). Commands run via the host shell with inherited stdio.
 */
export function runSetupCommands(plan: ClusterPlan): void {
  for (const cmd of plan.recommendations.setupCommands) {
    console.log(`   → ${cmd.goal}`);
    console.log(`     $ ${cmd.command}`);
    try {
      execSync(cmd.command, { stdio: 'inherit' });
    } catch (error) {
      if (cmd.optional) {
        console.log(`     (optional command failed, continuing): ${String(error)}`);
        continue;
      }
      throw error;
    }
  }
}

/**
 * Apply every manifest in a ClusterPlan via `kubectl apply -f -`.
 */
export function applyManifests(plan: ClusterPlan): void {
  for (const manifest of plan.recommendations.manifests) {
    console.log(`   → Apply ${manifest.kind} (namespace: ${manifest.namespace})`);
    execSync('kubectl apply -f -', {
      input: manifest.yaml,
      stdio: ['pipe', 'inherit', 'inherit'],
    });
  }
}

/**
 * Execute a full ClusterPlan (setup commands + manifests) and return the derived
 * registry URL for kind plans.
 */
export function executeClusterPlan(plan: ClusterPlan): { registryUrl?: string } {
  runSetupCommands(plan);
  applyManifests(plan);
  return { registryUrl: deriveRegistryUrl(plan) };
}
