/**
 * Idempotent Kubernetes resource application using server-side apply
 *
 * Note: This function should be called sequentially per resource.
 * Kubernetes server-side apply handles concurrent updates safely at the API level.
 */

import * as k8s from '@kubernetes/client-node';
import * as yaml from 'js-yaml';
import type { Logger } from 'pino';
import { type Result } from '@/types';
import { applyResource as applyK8sResource } from './resource-operations';

export interface ApplyOptions {
  dryRun?: boolean;
  force?: boolean;
  fieldManager?: string;
}

export interface K8sResource {
  apiVersion: string;
  kind: string;
  metadata: {
    name: string;
    namespace?: string;
    labels?: Record<string, string>;
    annotations?: Record<string, string>;
  };
  spec?: Record<string, unknown>;
  data?: Record<string, unknown>;
}

/**
 * Creates an idempotent K8s apply function using the consolidated resource-operations module
 *
 * Note: This function uses the consolidated resource-operations module which handles
 * idempotent apply using server-side apply. Kubernetes server-side apply handles
 * concurrent updates safely at the API level.
 */
export function createIdempotentApply(
  logger: Logger,
  kubeconfig?: string,
): (resource: K8sResource, options?: ApplyOptions) => Promise<Result<K8sResource>> {
  const kc = new k8s.KubeConfig();

  if (kubeconfig) {
    kc.loadFromString(kubeconfig);
  } else {
    kc.loadFromDefault();
  }

  /**
   * Apply K8s resource using the consolidated resource-operations module
   */
  return async function applyResource(
    resource: K8sResource,
    _options: ApplyOptions = {},
  ): Promise<Result<K8sResource>> {
    // Use the consolidated resource operations module
    return applyK8sResource(kc, resource, logger);
  };
}

/**
 * Parse YAML manifests into K8s resources
 */
export function parseManifests(yamlContent: string): K8sResource[] {
  try {
    // In js-yaml v4, loadAll is safe by default (no code execution)
    const docs = yaml.loadAll(yamlContent);
    return docs.filter((doc: unknown) => {
      const resource = doc as Record<string, unknown>;
      return resource?.kind && resource.apiVersion;
    }) as K8sResource[];
  } catch {
    return [];
  }
}
