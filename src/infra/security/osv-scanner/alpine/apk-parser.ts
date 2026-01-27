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
  const seenSourcePackages = new Set<string>(); // Deduplicate source packages
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
      origin?: string; // Source package name (o: field)
    } = {};

    for (const line of lines) {
      // Blank line indicates end of package block
      if (line.trim() === '') {
        if (currentPackage.name && currentPackage.version) {
          // Use origin (source package) name if available, otherwise use package name
          // OSV tracks vulnerabilities at source package level (e.g., "openssl" not "libcrypto1.1")
          const packageName = currentPackage.origin || currentPackage.name;

          // Deduplicate: only add each source package once
          // Multiple binary packages (libcrypto1.1, libssl1.1) can come from same source (openssl)
          const packageKey = `${packageName}@${currentPackage.version}`;
          if (!seenSourcePackages.has(packageKey)) {
            seenSourcePackages.add(packageKey);

            packages.push({
              name: packageName,
              version: currentPackage.version,
              ecosystem,
              metadata: {
                architecture: currentPackage.architecture,
                release,
                binaryPackage: currentPackage.origin ? currentPackage.name : undefined,
              },
            });
          }
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
        case 'o':
          currentPackage.origin = value; // Source package for OSV queries
          break;
      }
    }

    // Handle last package if file doesn't end with blank line
    if (currentPackage.name && currentPackage.version) {
      // Use origin (source package) name if available, otherwise use package name
      const packageName = currentPackage.origin || currentPackage.name;

      // Deduplicate: only add each source package once
      const packageKey = `${packageName}@${currentPackage.version}`;
      if (!seenSourcePackages.has(packageKey)) {
        seenSourcePackages.add(packageKey);

        packages.push({
          name: packageName,
          version: currentPackage.version,
          ecosystem,
          metadata: {
            architecture: currentPackage.architecture,
            release,
            binaryPackage: currentPackage.origin ? currentPackage.name : undefined,
          },
        });
      }
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
