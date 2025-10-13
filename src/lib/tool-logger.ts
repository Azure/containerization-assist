import { writeFileSync, mkdirSync } from 'fs';
import { appendFile } from 'fs/promises';
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

export function createToolLogEntry(
  toolName: string,
  sessionId: string,
  input: unknown,
): ToolLogEntry {
  return {
    timestamp: new Date().toISOString(),
    toolName,
    sessionId,
    input,
    output: undefined,
    success: false,
  };
}

let logFileName: string | null = null;

function isToolLoggingEnabled(): boolean {
  return config.toolLogging.enabled && !!config.toolLogging.dirPath;
}

export function getLogFilePath(): string {
  const dirPath = config.toolLogging.dirPath;
  if (!dirPath) {
    throw new Error('Tool logging directory path is not configured');
  }
  if (logFileName) {
    return join(dirPath, logFileName);
  }

  const date = new Date();
  const timestamp = date.toISOString();
  logFileName = `ca-tool-logs-${timestamp}.jsonl`;
  return join(dirPath, logFileName);
}

export function createToolLoggerFile(logger?: Logger): void {
  if (!isToolLoggingEnabled()) {
    return;
  }

  const dirPath = config.toolLogging.dirPath;
  if (!dirPath) {
    logger?.warn('Tool logging directory path is not configured');
    return;
  }

  try {
    // Ensure directory exists
    mkdirSync(dirPath, { recursive: true });

    const logFilePath = getLogFilePath();
    writeFileSync(logFilePath, '', 'utf-8');
    logger?.info({ path: logFilePath }, 'Tool logging file created');
  } catch (error) {
    const errorMsg = `Tool logging directory '${dirPath}' is not writable: ${(error as Error).message}`;
    logger?.warn({ error, path: dirPath }, errorMsg);
  }
}

export async function logToolExecution(entry: ToolLogEntry, logger?: Logger): Promise<void> {
  if (!isToolLoggingEnabled()) {
    return;
  }

  try {
    const filepath = getLogFilePath();
    const logLine = `${JSON.stringify(entry)}\n`;
    await appendFile(filepath, logLine, 'utf-8');

    logger?.debug({ filepath, toolName: entry.toolName }, 'Tool execution logged to file');
  } catch (error) {
    logger?.warn({ error, toolName: entry.toolName }, 'Failed to write tool execution log');
  }
}
