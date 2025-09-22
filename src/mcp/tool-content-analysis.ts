/**
 * Content Analysis Utilities
 *
 * Helper functions for analyzing and validating Dockerfiles, YAML, and other content.
 * Focused on parsing and validation without AI-specific logic.
 */

import yaml from 'js-yaml';

/**
 * Kubernetes resource specification interface
 */
export interface ResourceSpec {
  kind?: string;
  apiVersion?: string;
  name?: string;
  namespace?: string;
  replicas?: number;
  resources?: {
    limits?: { cpu?: string; memory?: string };
    requests?: { cpu?: string; memory?: string };
  };
}

/**
 * Detect if a Dockerfile uses multi-stage builds
 */
export function detectMultistageDocker(content: string): boolean {
  const fromMatches = content.match(/^FROM\s+/gm) || [];
  return fromMatches.length > 1;
}

/**
 * Count the number of layers in a Dockerfile
 */
export function countDockerLayers(content: string): number {
  const layerInstructions = [
    /^FROM\s+/gm,
    /^RUN\s+/gm,
    /^COPY\s+/gm,
    /^ADD\s+/gm,
    /^ENV\s+/gm,
    /^ARG\s+/gm,
    /^USER\s+/gm,
    /^WORKDIR\s+/gm,
  ];

  return layerInstructions.reduce((count, pattern) => {
    return count + (content.match(pattern) || []).length;
  }, 0);
}

/**
 * Extract the base image from a Dockerfile
 */
export function extractBaseImage(content: string): string | null {
  // Handle optional --platform flag: FROM --platform=linux/amd64 node:18
  const match = content.match(/^FROM\s+(?:--platform=[^\s]+\s+)?([^\s]+)/m);
  return match?.[1] ?? null;
}

/**
 * Detect potential secrets in content
 */
export function detectSecrets(content: string): string[] {
  const secrets: string[] = [];
  const secretPatterns = [
    { pattern: /password\s*[:=]\s*["'][^"']+["']/gi, type: 'password' },
    { pattern: /api[_-]?key\s*[:=]\s*["'][^"']+["']/gi, type: 'api_key' },
    { pattern: /secret\s*[:=]\s*["'][^"']+["']/gi, type: 'secret' },
    { pattern: /token\s*[:=]\s*["'][^"']+["']/gi, type: 'token' },
    { pattern: /BEGIN\s+(RSA|DSA|EC)\s+PRIVATE\s+KEY/gi, type: 'private_key' },
  ];

  for (const { pattern, type } of secretPatterns) {
    const matches = content.match(pattern);
    if (matches) {
      secrets.push(`${type}: ${matches.length} occurrence(s)`);
    }
  }

  return secrets;
}

/**
 * Validate YAML syntax using js-yaml
 */
export function validateYamlSyntax(content: string): boolean {
  try {
    // Try to load all documents in the content
    yaml.loadAll(content);
    return true;
  } catch {
    // Invalid YAML
    return false;
  }
}

/**
 * Extract Kubernetes resource specifications from YAML
 */
export function extractK8sResources(content: string): ResourceSpec[] {
  const resources: ResourceSpec[] = [];

  try {
    // Use js-yaml to properly parse all documents
    const documents = yaml.loadAll(content);

    for (const doc of documents) {
      if (!doc || typeof doc !== 'object') continue;

      const spec: ResourceSpec = {};
      const docObj = doc as any;

      // Extract basic fields
      if (docObj.kind) spec.kind = String(docObj.kind);
      if (docObj.apiVersion) spec.apiVersion = String(docObj.apiVersion);

      // Extract metadata fields
      if (docObj.metadata) {
        if (docObj.metadata.name) spec.name = String(docObj.metadata.name);
        if (docObj.metadata.namespace) spec.namespace = String(docObj.metadata.namespace);
      }

      // Extract spec fields
      if (docObj.spec) {
        if (typeof docObj.spec.replicas === 'number') {
          spec.replicas = docObj.spec.replicas;
        }

        // Extract resources from containers
        const containers = docObj.spec.template?.spec?.containers || docObj.spec.containers || [];
        for (const container of containers) {
          if (container.resources) {
            spec.resources = {};

            if (container.resources.limits) {
              spec.resources.limits = {};
              if (container.resources.limits.cpu) {
                spec.resources.limits.cpu = String(container.resources.limits.cpu);
              }
              if (container.resources.limits.memory) {
                spec.resources.limits.memory = String(container.resources.limits.memory);
              }
            }

            if (container.resources.requests) {
              spec.resources.requests = {};
              if (container.resources.requests.cpu) {
                spec.resources.requests.cpu = String(container.resources.requests.cpu);
              }
              if (container.resources.requests.memory) {
                spec.resources.requests.memory = String(container.resources.requests.memory);
              }
            }

            break; // Take resources from first container only
          }
        }
      }

      if (Object.keys(spec).length > 0) {
        resources.push(spec);
      }
    }
  } catch {
    // If YAML parsing fails, return empty array
    return [];
  }

  return resources;
}
