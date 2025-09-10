/**
 * Health and status helpers for MCP server
 */

export interface HealthStatus {
  healthy: boolean;
  services: Record<string, boolean>;
  details?: Record<string, unknown>;
}

export interface ServerStatus {
  running: boolean;
  healthy: boolean;
  stats: {
    tools: number;
    resources: number;
    prompts: number;
  };
}

/**
 * Creates a standard health check response
 */
export function createHealthStatus(
  services: Record<string, boolean>,
  details?: Record<string, unknown>,
): HealthStatus {
  return {
    healthy: Object.values(services).every(Boolean),
    services,
    ...(details && { details }),
  };
}

/**
 * Creates a server status response
 */
export function createServerStatus(
  running: boolean,
  healthy: boolean,
  stats: { tools: number; resources: number; prompts: number },
): ServerStatus {
  return {
    running,
    healthy,
    stats,
  };
}
