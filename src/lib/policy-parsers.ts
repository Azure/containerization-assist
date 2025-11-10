/**
 * Policy Parsing Utilities
 *
 * Reusable parsing functions for semantic validation in OPA Rego policies.
 * These utilities handle common format conversions (CPU, memory, etc.) to
 * enable semantic analysis beyond simple pattern matching.
 */

import { Success, Failure, type Result } from '@/types';

/**
 * Parse CPU resource string to millicores.
 *
 * Supports formats:
 * - "1000m" -> 1000
 * - "1.5" -> 1500
 * - "1" -> 1000
 * - "0.5" -> 500
 *
 * @param cpu - CPU string in Kubernetes format
 * @returns Millicores as a number
 *
 * @example
 * parseCPU("1000m") // 1000
 * parseCPU("1.5")   // 1500
 * parseCPU("2")     // 2000
 */
export function parseCPU(cpu: string): Result<number> {
  const trimmed = cpu.trim();

  // Handle millicores format (e.g., "1000m")
  if (trimmed.endsWith('m')) {
    const value = parseFloat(trimmed.slice(0, -1));
    if (isNaN(value) || value < 0) {
      return Failure('Invalid CPU value', {
        message: `Cannot parse CPU value: ${cpu}`,
        hint: 'Expected format: "1000m", "1.5", or "2"',
      });
    }
    return Success(value);
  }

  // Handle core format (e.g., "1.5" or "2")
  const value = parseFloat(trimmed);
  if (isNaN(value) || value < 0) {
    return Failure('Invalid CPU value', {
      message: `Cannot parse CPU value: ${cpu}`,
      hint: 'Expected format: "1000m", "1.5", or "2"',
    });
  }

  // Convert cores to millicores
  return Success(value * 1000);
}

/**
 * Parse memory resource string to bytes.
 *
 * Supports formats:
 * - "512Mi" -> 536870912
 * - "2Gi" -> 2147483648
 * - "1G" -> 1000000000
 * - "512M" -> 512000000
 * - "1024" -> 1024 (bytes)
 *
 * @param memory - Memory string in Kubernetes format
 * @returns Bytes as a number
 *
 * @example
 * parseMemory("512Mi") // 536870912
 * parseMemory("2Gi")   // 2147483648
 * parseMemory("1G")    // 1000000000
 */
export function parseMemory(memory: string): Result<number> {
  const trimmed = memory.trim();

  // Memory unit multipliers
  const units: Record<string, number> = {
    // Binary units (powers of 1024)
    Ki: 1024,
    Mi: 1024 ** 2,
    Gi: 1024 ** 3,
    Ti: 1024 ** 4,
    Pi: 1024 ** 5,
    Ei: 1024 ** 6,
    // Decimal units (powers of 1000)
    k: 1000,
    M: 1000 ** 2,
    G: 1000 ** 3,
    T: 1000 ** 4,
    P: 1000 ** 5,
    E: 1000 ** 6,
  };

  // Try to match unit suffix
  for (const [unit, multiplier] of Object.entries(units)) {
    if (trimmed.endsWith(unit)) {
      const value = parseFloat(trimmed.slice(0, -unit.length));
      if (isNaN(value) || value < 0) {
        return Failure('Invalid memory value', {
          message: `Cannot parse memory value: ${memory}`,
          hint: 'Expected format: "512Mi", "2Gi", "1G", etc.',
        });
      }
      return Success(value * multiplier);
    }
  }

  // No unit suffix - assume bytes
  const value = parseFloat(trimmed);
  if (isNaN(value) || value < 0) {
    return Failure('Invalid memory value', {
      message: `Cannot parse memory value: ${memory}`,
      hint: 'Expected format: "512Mi", "2Gi", "1G", or number in bytes',
    });
  }

  return Success(value);
}

/**
 * Extract image name from Dockerfile content.
 *
 * Looks for the final stage's FROM instruction or the last FROM in single-stage builds.
 * Does NOT include the image name defined by build output (that's in build metadata).
 *
 * @param dockerfile - Dockerfile content as string
 * @returns Base image name
 *
 * @example
 * extractImageName("FROM node:18-alpine\nRUN npm install")
 * // "node:18-alpine"
 */
export function extractImageName(dockerfile: string): Result<string> {
  const lines = dockerfile.split('\n');
  let lastFrom: string | null = null;

  for (const line of lines) {
    const trimmed = line.trim();
    // Match FROM instruction (ignoring AS stage_name)
    const fromMatch = trimmed.match(/^FROM\s+([^\s]+)(?:\s+AS\s+[^\s]+)?/i);
    if (fromMatch?.[1]) {
      lastFrom = fromMatch[1];
    }
  }

  if (!lastFrom) {
    return Failure('No FROM instruction found', {
      message: 'Cannot extract base image from Dockerfile',
      hint: 'Dockerfile must contain at least one FROM instruction',
    });
  }

  return Success(lastFrom);
}

/**
 * Extract EXPOSE ports from Dockerfile content.
 *
 * Returns all exposed ports as numbers.
 *
 * @param dockerfile - Dockerfile content as string
 * @returns Array of port numbers
 *
 * @example
 * extractPorts("FROM node:18\nEXPOSE 3000\nEXPOSE 8080")
 * // [3000, 8080]
 */
export function extractPorts(dockerfile: string): Result<number[]> {
  const lines = dockerfile.split('\n');
  const ports: number[] = [];

  for (const line of lines) {
    const trimmed = line.trim();
    // Match EXPOSE instruction
    const exposeMatch = trimmed.match(/^EXPOSE\s+(.+)/i);
    if (exposeMatch?.[1]) {
      const portSpec = exposeMatch[1].trim();
      // Split by whitespace to handle multiple ports in one EXPOSE
      const portStrings = portSpec.split(/\s+/);

      for (const portStr of portStrings) {
        // Remove protocol suffix if present (e.g., "3000/tcp")
        const portMatch = portStr.match(/^(\d+)(?:\/\w+)?$/);
        if (portMatch?.[1]) {
          const port = parseInt(portMatch[1], 10);
          if (!isNaN(port) && port > 0 && port <= 65535) {
            ports.push(port);
          }
        }
      }
    }
  }

  return Success(ports);
}

/**
 * Calculate resource efficiency ratio (limit/request).
 *
 * Higher ratios indicate potential over-provisioning.
 * Ratios > 4x typically suggest wasteful allocation.
 *
 * @param limit - Resource limit value
 * @param request - Resource request value
 * @returns Ratio as a number
 *
 * @example
 * calculateResourceRatio(4000, 500) // 8.0 (over-provisioned!)
 * calculateResourceRatio(2000, 1000) // 2.0 (reasonable)
 */
export function calculateResourceRatio(limit: number, request: number): Result<number> {
  if (request <= 0) {
    return Failure('Invalid request value', {
      message: 'Resource request must be greater than 0',
      hint: 'Check your resource configuration',
    });
  }

  if (limit < request) {
    return Failure('Invalid limit value', {
      message: 'Resource limit cannot be less than request',
      hint: 'Limit should be >= request',
    });
  }

  return Success(limit / request);
}

/**
 * Extract container image from Kubernetes manifest.
 *
 * Looks for the image field in container specs.
 *
 * @param manifest - Kubernetes manifest as object
 * @returns Array of container images
 *
 * @example
 * extractManifestImages({ spec: { containers: [{ image: "myapp:v1" }] } })
 * // ["myapp:v1"]
 */
export function extractManifestImages(manifest: unknown): Result<string[]> {
  if (typeof manifest !== 'object' || manifest === null) {
    return Failure('Invalid manifest', {
      message: 'Manifest must be an object',
    });
  }

  const images: string[] = [];

  // Helper to extract images from container specs
  const extractFromContainers = (containers: unknown): void => {
    if (!Array.isArray(containers)) return;

    for (const container of containers) {
      if (
        typeof container === 'object' &&
        container !== null &&
        'image' in container &&
        typeof container.image === 'string'
      ) {
        images.push(container.image);
      }
    }
  };

  // Check spec.containers (Deployment, StatefulSet, DaemonSet, etc.)
  if (
    'spec' in manifest &&
    typeof manifest.spec === 'object' &&
    manifest.spec !== null
  ) {
    const spec = manifest.spec as Record<string, unknown>;

    // Direct containers array (Pod)
    if ('containers' in spec) {
      extractFromContainers(spec.containers);
    }

    // Nested in template.spec (Deployment, StatefulSet, etc.)
    if (
      'template' in spec &&
      typeof spec.template === 'object' &&
      spec.template !== null &&
      'spec' in spec.template &&
      typeof (spec.template as Record<string, unknown>).spec === 'object'
    ) {
      const templateSpec = (spec.template as Record<string, unknown>).spec as Record<
        string,
        unknown
      >;
      if ('containers' in templateSpec) {
        extractFromContainers(templateSpec.containers);
      }
    }
  }

  return Success(images);
}

/**
 * Extract target ports from Kubernetes Service manifest.
 *
 * @param service - Kubernetes Service manifest
 * @returns Array of target port numbers
 *
 * @example
 * extractServicePorts({ spec: { ports: [{ targetPort: 3000 }] } })
 * // [3000]
 */
export function extractServicePorts(service: unknown): Result<number[]> {
  if (typeof service !== 'object' || service === null) {
    return Failure('Invalid service', {
      message: 'Service must be an object',
    });
  }

  if (!('spec' in service)) {
    return Failure('Invalid service', {
      message: 'Service must have a spec field',
    });
  }

  const spec = service.spec as Record<string, unknown>;
  if (!('ports' in spec) || !Array.isArray(spec.ports)) {
    return Success([]); // No ports defined
  }

  const targetPorts: number[] = [];
  for (const portSpec of spec.ports) {
    if (typeof portSpec === 'object' && portSpec !== null && 'targetPort' in portSpec) {
      const targetPort = portSpec.targetPort;
      // targetPort can be a number or a string (named port)
      if (typeof targetPort === 'number') {
        targetPorts.push(targetPort);
      } else if (typeof targetPort === 'string') {
        const parsed = parseInt(targetPort, 10);
        if (!isNaN(parsed)) {
          targetPorts.push(parsed);
        }
      }
    }
  }

  return Success(targetPorts);
}
