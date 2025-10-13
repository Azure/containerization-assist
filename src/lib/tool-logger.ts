import { writeFile, mkdir } from 'fs/promises';
import { join } from 'path';
import { config } from '@/config';
import type { Logger } from 'pino';
import type { ErrorGuidance } from '@/types';

export interface ToolLogEntry {
  timestamp: string;
  toolName: string;
  sessionId: string;
  input: unknown;
  output: unknown;
  success: boolean;
  durationMs?: number;
  error?: string;
  errorGuidance?: ErrorGuidance;
}

/**
 * Validate tool logging path at startup
 * Returns error message if validation fails, undefined if successful
 */
export async function validateToolLoggingPath(logger?: Logger): Promise<string | undefined> {
  if (!config.toolLogging.enabled || !config.toolLogging.path) {
    return undefined;
  }

  try {
    // Try to create directory
    await mkdir(config.toolLogging.path, { recursive: true });

    // Verify we can write to it by creating a test file
    const testFile = join(config.toolLogging.path, '.write-test');
    await writeFile(testFile, 'test', 'utf-8');

    // Clean up test file
    const { unlink } = await import('fs/promises');
    await unlink(testFile);

    logger?.info({ path: config.toolLogging.path }, 'Tool logging directory validated');

    return undefined;
  } catch (error) {
    const errorMsg = `Tool logging enabled but directory '${config.toolLogging.path}' is not writable: ${(error as Error).message}`;
    logger?.warn({ error, path: config.toolLogging.path }, errorMsg);
    return errorMsg;
  }
}

export async function logToolExecution(entry: ToolLogEntry, logger?: Logger): Promise<void> {
  if (!config.toolLogging.enabled || !config.toolLogging.path) {
    return;
  }

  try {
    await mkdir(config.toolLogging.path, { recursive: true });

    const date = new Date(entry.timestamp);
    const timeStr = date.toISOString().replace(/[:.]/g, '-');
    const filename = `${timeStr}_${entry.toolName}_${entry.sessionId}.json`;
    const filepath = join(config.toolLogging.path, filename);

    await writeFile(filepath, JSON.stringify(entry, null, 2), 'utf-8');

    logger?.debug({ filepath, toolName: entry.toolName }, 'Tool execution logged to file');
  } catch (error) {
    logger?.warn({ error, toolName: entry.toolName }, 'Failed to write tool execution log');
  }
}
