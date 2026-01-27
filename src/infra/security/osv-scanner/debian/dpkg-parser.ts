/**
 * Debian package parser for dpkg status files
 * Parses /var/lib/dpkg/status to extract installed packages
 */

import type { Logger } from 'pino';
import { parseTarStream } from '../tar-utils';
import type { ExtractedPackage } from '../maven/types';

/**
 * Parse Debian dpkg status file format
 *
 * Format:
 * Package: package-name
 * Status: install ok installed
 * Version: 1.2.3-4
 * Architecture: amd64
 * ...
 * (blank line separates packages)
 *
 * @param release - Debian release version (e.g., "9" for Stretch, "11" for Bullseye)
 */
export async function parseDebianPackages(
  tarBuffer: Buffer,
  filePath: string,
  logger: Logger,
  release?: string,
): Promise<ExtractedPackage[]> {
  const packages: ExtractedPackage[] = [];
  const ecosystem = release ? (`Debian:${release}` as const) : 'Debian';

  try {
    const content = await parseTarStream(tarBuffer, filePath);
    if (!content) {
      logger.debug({ filePath }, 'dpkg status file is empty');
      return packages;
    }

    const packageBlocks = content.split('\n\n');

    for (const block of packageBlocks) {
      if (!block.trim()) continue;

      const lines = block.split('\n');
      const pkgData: Record<string, string> = {};

      for (const line of lines) {
        const colonIndex = line.indexOf(':');
        if (colonIndex === -1) continue;

        const key = line.substring(0, colonIndex).trim();
        const value = line.substring(colonIndex + 1).trim();
        pkgData[key] = value;
      }

      // Only include installed packages
      const status = pkgData['Status'];
      if (!status?.includes('install ok installed')) {
        continue;
      }

      const name = pkgData['Package'];
      const version = pkgData['Version'];
      const architecture = pkgData['Architecture'];

      if (name && version) {
        packages.push({
          name,
          version,
          ecosystem,
          metadata: {
            architecture,
            source: pkgData['Source'],
            release,
          },
        });
      }
    }

    logger.debug(
      { packageCount: packages.length, filePath, release, ecosystem },
      'Parsed Debian packages',
    );
    return packages;
  } catch (error) {
    logger.warn({ error, filePath }, 'Failed to parse dpkg status');
    return packages;
  }
}
