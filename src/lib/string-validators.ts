/**
 * String validation utilities for consistent validation across the codebase
 */

/**
 * Checks if a string is empty or contains only whitespace.
 */
export function isEmptyString(value: string | null | undefined): boolean {
  return !value || value.trim().length === 0;
}

/**
 * Validates Docker tag format.
 * @throws {Error} if tag is invalid
 */
export function validateDockerTag(tag: string): void {
  if (isEmptyString(tag)) {
    throw new Error('Docker tag cannot be empty');
  }

  // Docker tag rules:
  // - Must be lowercase
  // - Can contain letters, digits, periods, hyphens, underscores
  // - Cannot start/end with separator
  // - Cannot have consecutive separators
  const tagRegex = /^[a-z0-9]+(?:[._-][a-z0-9]+)*$/;

  if (!tagRegex.test(tag)) {
    throw new Error(`Invalid Docker tag format: ${tag}`);
  }
}

/**
 * Validates Docker image name (repository:tag format)
 */
export function validateDockerImageName(imageName: string): void {
  if (isEmptyString(imageName)) {
    throw new Error('Docker image name cannot be empty');
  }

  const parts = imageName.split(':');
  if (parts.length > 2) {
    throw new Error(`Invalid image name format: ${imageName}`);
  }

  // Validate repository part
  const repository = parts[0];
  if (!repository || !/^[a-z0-9]+(?:[._/-][a-z0-9]+)*$/.test(repository)) {
    throw new Error(`Invalid repository name: ${repository}`);
  }

  // Validate tag if present
  if (parts.length === 2 && parts[1]) {
    validateDockerTag(parts[1]);
  }
}

/**
 * Validates a registry URL format
 */
export function validateRegistryUrl(url: string): void {
  if (isEmptyString(url)) {
    throw new Error('Registry URL cannot be empty');
  }

  // Registry can be hostname:port or just hostname
  const registryRegex = /^([a-z0-9]+(?:[.-][a-z0-9]+)*(?::[0-9]+)?|\[[0-9a-f:]+\](?::[0-9]+)?)$/i;

  // Remove protocol if present
  const cleanUrl = url.replace(/^https?:\/\//, '').replace(/\/$/, '');

  if (!registryRegex.test(cleanUrl)) {
    throw new Error(`Invalid registry URL format: ${url}`);
  }
}

/**
 * Normalizes a registry URL by removing protocol and trailing slash
 */
export function normalizeRegistryUrl(registry: string): string {
  return registry.replace(/^https?:\/\//, '').replace(/\/$/, '');
}

/**
 * Validates Kubernetes namespace name
 */
export function validateK8sNamespace(namespace: string): void {
  if (isEmptyString(namespace)) {
    throw new Error('Namespace cannot be empty');
  }

  // Kubernetes namespace rules:
  // - Must be lowercase
  // - Can contain letters, numbers, hyphens
  // - Must start and end with alphanumeric
  // - Max 63 characters
  const namespaceRegex = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/;

  if (namespace.length > 63) {
    throw new Error(`Namespace name too long (max 63 characters): ${namespace}`);
  }

  if (!namespaceRegex.test(namespace)) {
    throw new Error(`Invalid namespace format: ${namespace}`);
  }
}

/**
 * Sanitizes a string for use in filenames
 */
export function sanitizeFilename(filename: string): string {
  return filename
    .replace(/[^a-zA-Z0-9._-]/g, '_')
    .replace(/_{2,}/g, '_')
    .replace(/^_+|_+$/g, '');
}

/**
 * Normalize a score to ensure it's within valid range
 */
export function normalizeScore(score: number, max: number = 100): number {
  return Math.min(Math.max(score, 0), max);
}

/**
 * Calculate weighted average of scores
 */
export function weightedAverage(
  scores: Record<string, number>,
  weights: Record<string, number>,
): number {
  let totalWeight = 0;
  let weightedSum = 0;

  for (const [criterion, score] of Object.entries(scores)) {
    const weight = weights[criterion] || 0;
    totalWeight += weight;
    weightedSum += score * weight;
  }

  if (totalWeight === 0) {
    // If no weights, return simple average
    const values = Object.values(scores);
    return values.reduce((sum, val) => sum + val, 0) / Math.max(values.length, 1);
  }

  return weightedSum / totalWeight;
}
