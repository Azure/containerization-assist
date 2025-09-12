/**
 * Content Analysis Utilities
 *
 * Helper functions for analyzing and validating Dockerfiles, YAML, and other content.
 * Focused on parsing and validation without AI-specific logic.
 */

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
 * Validate YAML syntax (basic check)
 */
export function validateYamlSyntax(content: string): boolean {
  // Basic YAML validation
  if (content.includes('\t')) {
    return false; // YAML doesn't allow tabs
  }

  // Check for basic YAML structure
  if (!content.match(/^[\w-]+:/m) && !content.startsWith('---')) {
    return false;
  }

  // Check for consistent indentation
  const lines = content.split('\n');
  for (const line of lines) {
    if (line.trim() === '') continue;
    const indent = line.match(/^(\s*)/)?.[1]?.length || 0;
    if (indent % 2 !== 0) {
      return false; // YAML typically uses 2-space indentation
    }
  }

  return true;
}

/**
 * Extract Kubernetes resource specifications from YAML
 */
export function extractK8sResources(content: string): ResourceSpec[] {
  const resources: ResourceSpec[] = [];
  const documents = content.split(/^---$/m);

  for (const doc of documents) {
    if (!doc.trim()) continue;

    const spec: ResourceSpec = {};

    // Extract basic fields
    const kindMatch = doc.match(/^kind:\s*(.+)$/m);
    if (kindMatch?.[1]) spec.kind = kindMatch[1].trim();

    const apiVersionMatch = doc.match(/^apiVersion:\s*(.+)$/m);
    if (apiVersionMatch?.[1]) spec.apiVersion = apiVersionMatch[1].trim();

    const nameMatch = doc.match(/^\s+name:\s*(.+)$/m);
    if (nameMatch?.[1]) spec.name = nameMatch[1].trim();

    const namespaceMatch = doc.match(/^\s+namespace:\s*(.+)$/m);
    if (namespaceMatch?.[1]) spec.namespace = namespaceMatch[1].trim();

    const replicasMatch = doc.match(/^\s+replicas:\s*(\d+)$/m);
    if (replicasMatch?.[1]) spec.replicas = parseInt(replicasMatch[1], 10);

    // Extract resource specifications
    if (doc.includes('resources:')) {
      spec.resources = {};

      const cpuLimitMatch = doc.match(/limits:[\s\S]*?cpu:\s*["']?([^"'\n]+)["']?/);
      const memLimitMatch = doc.match(/limits:[\s\S]*?memory:\s*["']?([^"'\n]+)["']?/);
      const cpuRequestMatch = doc.match(/requests:[\s\S]*?cpu:\s*["']?([^"'\n]+)["']?/);
      const memRequestMatch = doc.match(/requests:[\s\S]*?memory:\s*["']?([^"'\n]+)["']?/);

      if (cpuLimitMatch || memLimitMatch) {
        spec.resources.limits = {};
        if (cpuLimitMatch?.[1]) spec.resources.limits.cpu = cpuLimitMatch[1].trim();
        if (memLimitMatch?.[1]) spec.resources.limits.memory = memLimitMatch[1].trim();
      }

      if (cpuRequestMatch || memRequestMatch) {
        spec.resources.requests = {};
        if (cpuRequestMatch?.[1]) spec.resources.requests.cpu = cpuRequestMatch[1].trim();
        if (memRequestMatch?.[1]) spec.resources.requests.memory = memRequestMatch[1].trim();
      }
    }

    if (Object.keys(spec).length > 0) {
      resources.push(spec);
    }
  }

  return resources;
}
