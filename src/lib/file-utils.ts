import { promises as fs, createWriteStream } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import https from 'node:https';
import http from 'node:http';
import { URL } from 'node:url';
import crypto from 'node:crypto';

import { Failure, Success, type Result } from '@/types';
import { extractErrorMessage } from '@/lib/errors';
import { validatePathOrFail } from '@/lib/validation-helpers';

/**
 * Download a file from URL to destination path
 */
export async function downloadFile(url: string, dest: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const parsedUrl = new URL(url);
    const client = parsedUrl.protocol === 'https:' ? https : http;

    const file = createWriteStream(dest);
    const request = client.get(url, (response) => {
      response.pipe(file);
      file.on('finish', () => {
        file.close();
        resolve();
      });
    });

    request.on('error', (err) => {
      fs.unlink(dest).catch(() => {}); // Delete the file on error
      reject(err);
    });
  });
}

/**
 * Make a file executable (Unix-like systems only)
 */
export async function makeExecutable(filePath: string): Promise<void> {
  await fs.chmod(filePath, 0o755);
}

/**
 * Create a temporary file with content
 */
export async function createTempFile(content: string, extension: string = ''): Promise<string> {
  const tempPath = path.join(tmpdir(), `temp-${crypto.randomUUID()}${extension}`);
  // Create file with permissions (readable/writable only by owner)
  await fs.writeFile(tempPath, content, { encoding: 'utf8', mode: 0o600 });
  return tempPath;
}

/**
 * Delete a temporary file (ignores errors)
 */
export async function deleteTempFile(filePath: string): Promise<void> {
  try {
    await fs.unlink(filePath);
  } catch {
    // Ignore errors when deleting temp files
  }
}

/**
 * Read Dockerfile from path or use provided content
 *
 * Consolidates duplicate Dockerfile reading logic across tools.
 * Handles both direct content and file path inputs.
 *
 * @param options - Either path to Dockerfile or content directly
 * @returns Result containing Dockerfile content
 *
 * @example
 * ```typescript
 * // Read from file path
 * const result = await readDockerfile({ path: './Dockerfile' });
 * if (!result.ok) return result;
 * const content = result.value;
 *
 * // Use provided content
 * const result2 = await readDockerfile({ content: 'FROM node:20\n...' });
 * ```
 */
export async function readDockerfile(options: {
  path?: string;
  content?: string;
}): Promise<Result<string>> {
  // Check if content was explicitly provided (even if empty)
  if (options.content !== undefined) {
    if (options.content.trim().length === 0) {
      return Failure('Dockerfile content is empty', {
        message: 'Dockerfile content is empty',
        hint: 'Dockerfile must contain at least one instruction',
        resolution: 'Provide valid Dockerfile content or path',
      });
    }
    return Success(options.content);
  }

  if (options.path) {
    const dockerfilePath = path.resolve(options.path);

    // Validate file exists and is readable
    const validation = await validatePathOrFail(dockerfilePath, {
      mustExist: true,
      mustBeFile: true,
      readable: true,
    });

    if (!validation.ok) {
      return validation;
    }

    try {
      const content = await fs.readFile(dockerfilePath, 'utf-8');

      if (content.trim().length === 0) {
        return Failure(`Dockerfile at ${dockerfilePath} is empty`, {
          message: 'Dockerfile is empty',
          hint: 'Dockerfile must contain at least one instruction',
          resolution: 'Add FROM instruction and other build steps',
        });
      }

      return Success(content);
    } catch (error) {
      return Failure(
        `Failed to read Dockerfile at ${dockerfilePath}: ${extractErrorMessage(error)}`,
        {
          message: 'Failed to read Dockerfile',
          hint: 'Check file permissions and path',
          resolution: `Ensure file is readable: ls -la ${dockerfilePath}`,
        },
      );
    }
  }

  return Failure("Either 'path' or 'content' must be provided", {
    message: 'Missing Dockerfile input',
    hint: 'Provide dockerfile content or path to Dockerfile',
    resolution: 'Add either path or content parameter',
  });
}

