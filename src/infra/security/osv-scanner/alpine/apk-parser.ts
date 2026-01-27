import type { Logger } from 'pino';
import { parseTarStream } from '../tar-utils';
import type { ExtractedPackage } from '../maven/types';

/**
 * Parses Alpine APK package database format from /lib/apk/db/installed
 *
 * Format:
 * - Each package is a block of key:value lines
 * - Packages are separated by blank lines
 * - Key fields:
 *   P: - Package name
 *   V: - Version
 *   A: - Architecture
 *   o: - Origin package name
 *   c: - Commit/checksum
 *
 * Example:
 * P:musl
 * V:1.2.3-r0
 * A:x86_64
 *
 * P:busybox
 * V:1.35.0-r17
 * A:x86_64
 *
 * @param release - Alpine release version (e.g., "3.18" for Alpine 3.18)
 */
export async function parseAlpinePackages(
  tarBuffer: Buffer,
  filePath: string,
  logger: Logger,
  release?: string,
): Promise<ExtractedPackage[]> {
  const packages: ExtractedPackage[] = [];
  const ecosystem = release ? `Alpine:v${release}` : 'Alpine';

  try {
    const content = await parseTarStream(tarBuffer, filePath);
    if (!content) {
      logger.debug({ filePath }, 'APK installed file is empty');
      return packages;
    }

    const lines = content.split('\n');

    let currentPackage: {
      name?: string;
      version?: string;
      architecture?: string;
    } = {};

    for (const line of lines) {
      // Blank line indicates end of package block
      if (line.trim() === '') {
        if (currentPackage.name && currentPackage.version) {
          packages.push({
            name: currentPackage.name,
            version: currentPackage.version,
            ecosystem,
            metadata: {
              architecture: currentPackage.architecture,
              release,
            },
          });
        }
        currentPackage = {};
        continue;
      }

      // Parse key:value lines
      const colonIndex = line.indexOf(':');
      if (colonIndex === -1) continue;

      const key = line.substring(0, colonIndex);
      const value = line.substring(colonIndex + 1);

      switch (key) {
        case 'P':
          currentPackage.name = value;
          break;
        case 'V':
          currentPackage.version = value;
          break;
        case 'A':
          currentPackage.architecture = value;
          break;
      }
    }

    // Handle last package if file doesn't end with blank line
    if (currentPackage.name && currentPackage.version) {
      packages.push({
        name: currentPackage.name,
        version: currentPackage.version,
        ecosystem,
        metadata: {
          architecture: currentPackage.architecture,
          release,
        },
      });
    }

    logger.debug(
      { packageCount: packages.length, filePath, release, ecosystem },
      'Parsed Alpine packages',
    );
    return packages;
  } catch (error) {
    logger.warn({ error, filePath }, 'Failed to parse APK installed');
    return packages;
  }
}
