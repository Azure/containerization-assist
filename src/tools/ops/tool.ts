/**
 * Ops Tool - Modernized Implementation
 *
 * Provides operational utilities like ping and server status
 * Follows the new Tool interface pattern
 */

import * as os from 'os';
import { extractErrorMessage } from '@/lib/error-utils';
import { createToolTimer } from '@/lib/tool-helpers';
import { Success, Failure, type Result } from '@/types';
import type { ToolContext } from '@/mcp/context';
import type { Tool } from '@/types/tool';
import { opsToolSchema } from './schema';
import type { z } from 'zod';

interface PingConfig {
  message?: string;
}

interface PingResult {
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

/**
 * Ping operation - test server connectivity
 * @public
 */
export async function ping(config: PingConfig, context: ToolContext): Promise<Result<PingResult>> {
  const timer = createToolTimer(context.logger, 'ops-ping');

  try {
    const { message = 'ping' } = config;

    context.logger.info({ message }, 'Processing ping request');

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

    timer.end();
    return Success(result);
  } catch (error) {
    timer.error(error);
    context.logger.error({ error }, 'Ping failed');
    return Failure(extractErrorMessage(error));
  }
}

interface ServerStatusConfig {
  details?: boolean;
}

interface ServerStatusResult {
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
  const timer = createToolTimer(context.logger, 'ops-server-status');

  try {
    const { details = false } = config;

    context.logger.info({ details }, 'Server status requested');

    const uptime = Math.floor(process.uptime());
    const version = '2.0.0';
    const totalMem = os.totalmem();
    const freeMem = os.freemem();
    const usedMem = totalMem - freeMem;
    const memPercentage = Math.round((usedMem / totalMem) * 100);

    const cpus = os.cpus();
    const loadAverage = os.loadavg();

    const migratedToolCount = 12;

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
        count: 14,
        migrated: migratedToolCount,
      },
    };

    context.logger.info(
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
    context.logger.error({ error }, 'Error collecting server status');
    return Failure(extractErrorMessage(error));
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
async function run(
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
      return Failure(`Unknown operation: ${input.operation}`);
  }
}

/**
 * Ops tool conforming to Tool interface
 */
const tool: Tool<typeof opsToolSchema, OpsResult> = {
  name: 'ops',
  description: 'Operational utilities for ping and server status',
  version: '2.0.0',
  schema: opsToolSchema,
  metadata: {
    aiDriven: false,
    knowledgeEnhanced: false,
    samplingStrategy: 'none',
    enhancementCapabilities: [],
  },
  run,
};

export default tool;
