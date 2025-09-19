/**
 * Ops Tool - Simplified Single-File Implementation
 * Provides operational utilities like ping and server status
 */

import * as os from 'node:os';
import { z } from 'zod';
import type { Logger } from 'pino';
import type { ToolContext } from '@mcp/context';
import { Success, Failure, type Result } from '@types';
import { parsePackageJsonSync } from '@lib/parsing-package-json';
import { getToolRegistry } from '@mcp/tools/registry';

// Schema definition
export const opsToolSchema = z.object({
  sessionId: z.string().optional().describe('Session identifier for tracking operations'),
  operation: z.enum(['ping', 'status']).describe('Operation to perform'),
  message: z.string().optional().describe('Message for ping operation'),
  details: z.boolean().optional().describe('Include detailed information in status'),
});

export type OpsToolParams = z.infer<typeof opsToolSchema>;

// Result types
export interface PingResult {
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
    sampling: boolean;
    progress: boolean;
  };
}

export interface ServerStatusResult {
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

export type OpsResult = PingResult | ServerStatusResult;

/**
 * Get package version from package.json
 */
function getPackageVersion(): string {
  try {
    const pkg = parsePackageJsonSync(process.cwd());
    return pkg.version || 'unknown';
  } catch {
    return 'unknown';
  }
}

/**
 * Get current tool count from registry
 */
function getToolCount(): number {
  try {
    const registry = getToolRegistry();
    return registry.size;
  } catch {
    return 0;
  }
}

/**
 * Ping operation - test server connectivity
 */
function pingOperation(message = 'ping', logger: Logger): Result<PingResult> {
  try {
    logger.info({ message }, 'Processing ping request');

    const result: PingResult = {
      success: true,
      message: `pong: ${message}`,
      timestamp: new Date().toISOString(),
      server: {
        name: 'containerization-assist-mcp',
        version: getPackageVersion(),
        uptime: process.uptime(),
        pid: process.pid,
      },
      capabilities: {
        tools: true,
        sampling: true,
        progress: true,
      },
    };

    return Success(result);
  } catch (error) {
    const msg = error instanceof Error ? error.message : String(error);
    logger.error({ error: msg }, 'Ping failed');
    return Failure(`Ping failed: ${msg}`);
  }
}

/**
 * Get server status
 */
function serverStatusOperation(details: boolean, logger: Logger): Result<ServerStatusResult> {
  try {
    logger.info({ details }, 'Server status requested');

    const uptime = Math.floor(process.uptime());
    const version = getPackageVersion();
    const totalMem = os.totalmem();
    const freeMem = os.freemem();
    const usedMem = totalMem - freeMem;
    const memPercentage = Math.round((usedMem / totalMem) * 100);

    const cpus = os.cpus();
    const loadAverage = os.loadavg();

    // Get dynamic tool counts
    const toolCount = getToolCount();

    const status: ServerStatusResult = {
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
        count: toolCount,
        migrated: toolCount,
      },
    };

    logger.info(
      {
        uptime,
        memoryUsed: usedMem,
        memoryPercentage: memPercentage,
        toolCount,
      },
      'Server status compiled',
    );

    return Success(status);
  } catch (error) {
    const msg = error instanceof Error ? error.message : String(error);
    logger.error({ error: msg }, 'Error collecting server status');
    return Failure(`Error collecting server status: ${msg}`);
  }
}

/**
 * Ops tool handler
 */
async function opsHandler(params: OpsToolParams, context: ToolContext): Promise<Result<OpsResult>> {
  const start = Date.now();
  const { logger } = context;
  const { operation } = params;

  try {
    let result: Result<OpsResult>;

    switch (operation) {
      case 'ping':
        result = pingOperation(params.message, logger);
        break;
      case 'status':
        result = serverStatusOperation(params.details ?? false, logger);
        break;
      default:
        result = Failure(`Unknown operation: ${operation}`);
    }

    const duration = Date.now() - start;
    logger.info({ duration, tool: 'ops', operation }, 'Tool execution complete');
    return result;
  } catch (error) {
    const duration = Date.now() - start;
    const message = error instanceof Error ? error.message : String(error);
    logger.error({ duration, error: message, tool: 'ops' }, 'Tool execution failed');
    return Failure(`Operation failed: ${message}`);
  }
}

/**
 * Standard tool export for MCP server integration
 */
export const ops = {
  type: 'standard' as const,
  name: 'ops',
  description: 'Operational utilities like ping and server status',
  inputSchema: opsToolSchema,
  handler: opsHandler,
};

// Default export for convenience
export default ops;
