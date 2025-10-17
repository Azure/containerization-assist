import { promises as fs, createWriteStream } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import https from 'node:https';
import http from 'node:http';
import { URL } from 'node:url';
import crypto from 'node:crypto';

export async function fileExists(filePath: string): Promise<boolean> {
  try {
    const stats = await fs.stat(filePath);
    return stats.isFile();
  } catch {
    return false;
  }
}

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
