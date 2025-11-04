/**
 * Telemetry Sanitization Utilities
 *
 * Provides utilities to safely hash/obfuscate customer data for telemetry.
 *
 * Security Principle: "When in doubt, hash it out"
 */

import { createHash } from 'node:crypto';

import type { ToolName } from '@/tools';

/**
 * Hash a string using SHA256 and return first 16 characters
 * Provides enough uniqueness for grouping while preventing reverse lookup
 */
export function hashValue(value: string): string {
  return createHash('sha256').update(value).digest('hex').substring(0, 16);
}

/**
 * Sanitize a file path for telemetry
 * Replaces path segments with hashed values
 *
 * Security strategy:
 * - Relative paths (./ or ../): Keep as-is (no customer data)
 * - Short paths (<=4 segments): Hash all (too risky to preserve any)
 * - Long paths (5+ segments): Hash all except last 2 (preserve structure like src/index.ts)
 *
 * @example
 * sanitizePath('/home/user/myapp/src/index.ts')
 * // Returns: '<hash1>/<hash2>/<hash3>/src/index.ts'
 *
 * sanitizePath('/home/user/myapp/src')
 * // Returns: '<hash1>/<hash2>/<hash3>/<hash4>' (all hashed - too short, 4 segments)
 */
export function sanitizePath(path: string): string {
  // For relative paths starting with ./ or ../, preserve as-is (no customer data)
  if (path.startsWith('./') || path.startsWith('../')) {
    return path;
  }

  const parts = path.split('/');

  // Filter out empty parts (from leading /) and count real segments
  const nonEmptyParts = parts.filter((p) => p !== '' && p !== '.' && p !== '..');

  // For paths with 4 or fewer segments, hash everything (security-first approach)
  // This includes most user/project paths like /home/user/project or /etc/config
  if (nonEmptyParts.length <= 4) {
    return parts
      .map((part) => {
        if (part === '' || part === '.' || part === '..') {
          return part;
        }
        return `<${hashValue(part)}>`;
      })
      .join('/');
  }

  // For longer paths (5+ segments), hash all but the last 2 real segments
  // This preserves structural context like "src/index.ts" while hiding customer identifiers
  let realSegmentCount = 0;
  const sanitizedParts = parts.map((part) => {
    // Keep empty parts, dots, and parent refs as-is
    if (part === '' || part === '.' || part === '..') {
      return part;
    }

    realSegmentCount++;
    // Hash all segments except the last 2 real ones
    if (realSegmentCount > nonEmptyParts.length - 2) {
      return part; // Preserve last 2 real segments
    }
    return `<${hashValue(part)}>`;
  });

  return sanitizedParts.join('/');
}

/**
 * Sanitize tool input parameters for telemetry
 * Removes or hashes sensitive fields
 */
export function sanitizeToolInput(params: Record<string, unknown>): Record<string, unknown> {
  const sanitized: Record<string, unknown> = {};

  // Fields containing file paths - always hash
  const pathFields = ['path', 'dockerfile', 'manifestPath', 'contextPath'];

  // Fields containing customer identifiers - always hash
  const sensitiveFields = [
    'imageName',
    'registryUrl',
    'deploymentName',
    'namespace',
    'serviceName',
    'clusterName',
  ];

  // Known safe string fields (enums, constants, flags)
  // These can be logged as-is because they don't contain customer data
  const safeStringFields = [
    'severity',
    'platform',
    'pullPolicy',
    'restartPolicy',
    'strategy',
    'protocol',
    'vendor',
    'format',
    'outputFormat',
    'chainHintsMode',
  ];

  for (const [key, value] of Object.entries(params)) {
    if (pathFields.includes(key) && typeof value === 'string') {
      // Hash file paths
      sanitized[key] = sanitizePath(value);
    } else if (sensitiveFields.includes(key) && typeof value === 'string') {
      // Hash sensitive identifiers
      sanitized[key] = hashValue(value);
    } else if (typeof value === 'boolean' || typeof value === 'number') {
      // Booleans and numbers are safe (flags, counts, sizes)
      sanitized[key] = value;
    } else if (typeof value === 'string' && safeStringFields.includes(key)) {
      // Known safe string fields (enums, constants)
      sanitized[key] = value;
    } else if (typeof value === 'string') {
      // Unknown string fields - hash to be safe
      sanitized[key] = hashValue(value);
    } else {
      // Complex objects - mark as present but don't log
      sanitized[key] = '<omitted>';
    }
  }

  return sanitized;
}

/**
 * Extract safe telemetry metrics from tool results
 * Returns only aggregate metrics, no sensitive data
 */
export function extractSafeTelemetryMetrics(
  toolName: ToolName,
  result: Record<string, unknown>,
): Record<string, unknown> {
  const metrics: Record<string, unknown> = {};

  // Tool-specific safe metrics
  switch (toolName) {
    case 'analyze-repo':
      // Safe: Framework types, languages (enums)
      if ('framework' in result) metrics.framework = result.framework;
      if ('language' in result) metrics.language = result.language;
      if ('moduleCount' in result) metrics.moduleCount = result.moduleCount;
      break;

    case 'build-image':
      // Safe: Build duration, image size (aggregates)
      if ('buildTime' in result) metrics.buildTimeMs = result.buildTime;
      if ('size' in result) metrics.imageSizeBytes = result.size;
      // Unsafe: imageId, tags (contain customer names)
      break;

    case 'scan-image':
      // Safe: Vulnerability counts (aggregates)
      if ('summary' in result && result.summary) {
        const summary = result.summary as Record<string, number>;
        metrics.criticalVulns = summary.critical || 0;
        metrics.highVulns = summary.high || 0;
        metrics.mediumVulns = summary.medium || 0;
        metrics.lowVulns = summary.low || 0;
      }
      break;

    case 'verify-deploy':
      // Safe: Replica counts, status (aggregates)
      if ('replicas' in result) metrics.replicas = result.replicas;
      if ('readyReplicas' in result) metrics.readyReplicas = result.readyReplicas;
      if ('status' in result) metrics.status = result.status;
      // Unsafe: namespace, deploymentName (customer identifiers)
      break;

    default:
      // For unknown tools, only log success/failure
      metrics.hasResult = true;
  }

  return metrics;
}

/**
 * Safe telemetry event structure
 */
export interface SafeTelemetryEvent {
  toolName: ToolName;
  success: boolean;
  durationMs?: number;
  sanitizedInput: Record<string, unknown>;
  metrics: Record<string, unknown>;
  errorType?: string; // Error constructor name only, not message
}

/**
 * Create a safe telemetry event from tool execution
 */
export function createSafeTelemetryEvent(
  toolName: ToolName,
  params: Record<string, unknown>,
  result: { ok: boolean; value?: Record<string, unknown>; error?: string },
  durationMs?: number,
): SafeTelemetryEvent {
  const event: SafeTelemetryEvent = {
    toolName,
    success: result.ok,
    sanitizedInput: sanitizeToolInput(params),
    metrics: result.ok && result.value ? extractSafeTelemetryMetrics(toolName, result.value) : {},
  };

  // Only include optional properties if they have values
  if (durationMs !== undefined) {
    event.durationMs = durationMs;
  }
  if (result.error) {
    event.errorType = 'Error';
  }

  return event;
}
