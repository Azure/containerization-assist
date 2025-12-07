/**
 * Ops Tool - MCP Server Diagnostics
 *
 * Provides health monitoring and connectivity testing for the MCP server itself.
 * This tool operates on the server infrastructure, not on user applications.
 *
 * **Use Cases:**
 * - Health checks for monitoring systems
 * - Connectivity testing during troubleshooting
 * - Resource usage monitoring (memory, CPU)
 * - Server diagnostics and metadata
 *
 * **NOT for:**
 * - Application containerization (use build-image, etc.)
 * - Docker operations (use Docker tools)
 * - Kubernetes operations (use K8s tools)
 *
 * @packageDocumentation
 */

import * as os from 'os';
import { extractErrorMessage } from '@/lib/errors';
import { setupToolContext } from '@/lib/tool-context-helpers';
import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/core/context';
import { opsToolSchema } from './schema';
import type { z } from 'zod';
import { formatDuration, formatTimestamp } from '@/lib/summary-helpers';

interface PingConfig {
  message?: string;
}

export interface PingResult {
  /**
   * Natural language summary for user display.
   * 1-3 sentences describing the ping result.
   * @example "✅ Server is responsive. Ping successful at 2025-01-15T10:30:00Z."
   */
  summary?: string;
  success: boolean;
  message: string;
  timestamp: string;
  server: {
    name: string;
    version: string;
    uptime: number;
    pid: number;
  };
  capabilities: {
    tools: boolean;
    progress: boolean;
  };
}

/**
 * Ping operation - test server connectivity
 * @public
 */
export async function ping(config: PingConfig, context: ToolContext): Promise<Result<PingResult>> {
  const { logger, timer } = setupToolContext(context, 'ops-ping');

  try {
    const { message = 'ping' } = config;

    logger.info({ message }, 'Processing ping request');

    const timestamp = new Date().toISOString();
    const summary = `✅ Server is responsive. Ping successful at ${formatTimestamp(timestamp)}.`;

    const result: PingResult = {
      summary,
      success: true,
      message: `pong: ${message}`,
      timestamp,
      server: {
        name: 'containerization-assist-mcp',
        version: '2.0.0',
        uptime: process.uptime(),
        pid: process.pid,
      },
      capabilities: {
        tools: true,
        progress: true,
      },
    };

    timer.end();
    return Success(result);
  } catch (error) {
    timer.error(error);
    logger.error({ error }, 'Ping failed');
    return Failure(extractErrorMessage(error), {
      message: extractErrorMessage(error),
      hint: 'An unexpected error occurred during the ping operation',
      resolution: 'Check the server logs for details. This is typically a server-side issue',
    });
  }
}

interface ServerStatusConfig {
  details?: boolean;
}

export interface ServerStatusResult {
  /**
   * Natural language summary for user display.
   * 1-3 sentences describing the server status.
   * @example "✅ Server healthy. Running for 2h 15m. Memory: 45% used, CPU: 4 cores."
   */
  summary?: string;
  success: boolean;
  version: string;
  uptime: number;
  memory: {
    used: number;
    total: number;
    free: number;
    percentage: number;
  };
  cpu: {
    model: string;
    cores: number;
    loadAverage: number[];
  };
  system: {
    platform: string;
    release: string;
    hostname: string;
  };
  tools: {
    count: number;
    migrated: number;
  };
  sessions?: number;
}

/**
 * Get server status
 * @public
 */
export async function serverStatus(
  config: ServerStatusConfig,
  context: ToolContext,
): Promise<Result<ServerStatusResult>> {
  const { logger, timer } = setupToolContext(context, 'ops-server-status');

  try {
    const { details = false } = config;

    logger.info({ details }, 'Server status requested');

    const uptime = Math.floor(process.uptime());
    const version = '2.0.0';
    const totalMem = os.totalmem();
    const freeMem = os.freemem();
    const usedMem = totalMem - freeMem;
    const memPercentage = Math.round((usedMem / totalMem) * 100);

    const cpus = os.cpus();
    const loadAverage = os.loadavg();

    const migratedToolCount = 12;

    // Generate summary
    const uptimeStr = formatDuration(uptime);
    const summary = `✅ Server healthy. Running for ${uptimeStr}. Memory: ${memPercentage}% used, CPU: ${cpus.length} cores.`;

    const status: ServerStatusResult = {
      summary,
      success: true,
      version,
      uptime,
      memory: {
        used: usedMem,
        total: totalMem,
        free: freeMem,
        percentage: memPercentage,
      },
      cpu: {
        model: cpus[0]?.model ?? 'unknown',
        cores: cpus.length,
        loadAverage,
      },
      system: {
        platform: os.platform(),
        release: os.release(),
        hostname: os.hostname(),
      },
      tools: {
        count: 14,
        migrated: migratedToolCount,
      },
    };

    logger.info(
      {
        uptime,
        memoryUsed: usedMem,
        memoryPercentage: memPercentage,
        toolsMigrated: migratedToolCount,
      },
      'Server status compiled',
    );

    timer.end();
    return Success(status);
  } catch (error) {
    timer.error(error);
    logger.error({ error }, 'Error collecting server status');
    return Failure(extractErrorMessage(error), {
      message: extractErrorMessage(error),
      hint: 'An unexpected error occurred while collecting server status',
      resolution: 'Check the server logs for details. This is typically a server-side issue',
    });
  }
}

// Combined ops interface
/** @public */
export interface OpsConfig {
  operation: 'ping' | 'status';
  message?: string;
  details?: boolean;
}

export type OpsResult = PingResult | ServerStatusResult;

/**
 * Main ops implementation
 */
async function handleOps(
  input: z.infer<typeof opsToolSchema>,
  context: ToolContext,
): Promise<Result<OpsResult>> {
  const { operation } = input;

  switch (operation) {
    case 'ping':
      return ping({ ...(input.message !== undefined && { message: input.message }) }, context);
    case 'status':
      return serverStatus(
        { ...(input.details !== undefined && { details: input.details }) },
        context,
      );
    default:
      return Failure(`Unknown operation: ${input.operation}`, {
        message: `Unknown operation: ${input.operation}`,
        hint: 'The requested operation is not supported',
        resolution: 'Use one of the supported operations: "ping" for connectivity testing or "status" for server information',
      });
  }
}

/**
 * Ops tool conforming to Tool interface
 */
import { tool } from '@/types/tool';

export default tool({
  name: 'ops',
  description: 'MCP server diagnostics: ping for connectivity testing, status for health metrics (memory, CPU, uptime). Use this for server monitoring, not application containerization.',
  category: 'utility',
  version: '2.0.0',
  schema: opsToolSchema,
  metadata: {
    knowledgeEnhanced: false,
  },
  handler: handleOps,
});
