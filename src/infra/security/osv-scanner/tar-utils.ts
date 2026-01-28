/**
 * Utility functions for parsing tar streams from Docker containers
 */

import * as tar from 'tar';
import { Readable } from 'stream';

/**
 * Parse a tar buffer and extract the content of a specific file
 *
 * @param tarBuffer Buffer containing tar archive data
 * @param targetPath Path to the file to extract (e.g., 'var/lib/dpkg/status')
 * @returns File content as string, or null if not found
 */
export async function parseTarStream(
  tarBuffer: Buffer,
  targetPath: string,
): Promise<string | null> {
  return new Promise((resolve, reject) => {
    const chunks: Buffer[] = [];
    let found = false;

    // Normalize path (remove leading slash if present)
    const normalizedPath = targetPath.startsWith('/') ? targetPath.substring(1) : targetPath;
    // Extract basename for matching (Docker getArchive returns just the filename)
    const basename = targetPath.split('/').pop() || targetPath;

    const stream = Readable.from(tarBuffer);

    stream.pipe(
      tar.t({
        onentry: (entry) => {
          // Path traversal protection (defense-in-depth)
          // While this function is currently only used with Docker-generated tar archives,
          // validating paths prevents potential directory traversal attacks
          if (entry.path.includes('..')) {
            entry.resume();
            return;
          }

          if (
            entry.path === normalizedPath ||
            entry.path === targetPath ||
            entry.path === basename
          ) {
            found = true;
            entry.on('data', (chunk) => chunks.push(chunk));
            entry.on('end', () => {
              const content = Buffer.concat(chunks).toString('utf-8');
              resolve(content);
            });
          } else {
            entry.resume(); // Skip other files
          }
        },
      }),
    );

    stream.on('end', () => {
      if (!found) {
        resolve(null);
      }
    });

    stream.on('error', (error) => {
      reject(error);
    });
  });
}
