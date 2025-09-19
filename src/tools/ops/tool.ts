/**
 * Ops Tool
 * Provides operational utilities like ping and server status
 */

import * as os from 'node:os';
import type { Logger } from 'pino';
import type { ToolContext } from '@mcp/context';
import { Success, Failure, type Result } from '@types';
import { opsToolSchema, type OpsToolParams } from './schema';

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

export interface OpsDeps {
  logger: Logger;
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
        version: '2.0.0',
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
    const version = '2.0.0';
    const totalMem = os.totalmem();
    const freeMem = os.freemem();
    const usedMem = totalMem - freeMem;
    const memPercentage = Math.round((usedMem / totalMem) * 100);

    const cpus = os.cpus();
    const loadAverage = os.loadavg();

    // These are hardcoded for now but could be dynamic
    const migratedToolCount = 14; // Updated based on actual progress

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
        count: 18, // Total tools
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

    return Success(status);
  } catch (error) {
    const msg = error instanceof Error ? error.message : String(error);
    logger.error({ error: msg }, 'Error collecting server status');
    return Failure(`Error collecting server status: ${msg}`);
  }
}

/**
 * Create ops tool with explicit dependencies
 */
export function createOpsTool(deps: OpsDeps) {
  return async (params: OpsToolParams, _context: ToolContext): Promise<Result<OpsResult>> => {
    const start = Date.now();
    const { logger } = deps;
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
  };
}

/**
 * Standard tool export for MCP server integration
 */
export const tool = {
  type: 'standard' as const,
  name: 'ops',
  description: 'Operational utilities like ping and server status',
  inputSchema: opsToolSchema,
  execute: createOpsTool,
};
