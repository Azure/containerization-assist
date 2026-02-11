// Extract packages from Docker image filesystems (Maven, Debian, Alpine)

import type { Logger } from 'pino';
import type Dockerode from 'dockerode';
import { extractErrorMessage } from '@/lib/errors';
import type { ExtractedPackage } from './maven/types';
import { parseMavenPackages } from './maven/pom-parser';
import { parseDebianPackages } from './debian/dpkg-parser';
import { parseAlpinePackages } from './alpine/apk-parser';
import { parseTarStream } from './tar-utils';

const MAVEN_MANIFEST_PATHS = [
  '/pom.xml',
  '/app/pom.xml',
  '/usr/src/app/pom.xml',
  '/workspace/pom.xml',
  '/home/app/pom.xml',
];

const DEBIAN_STATUS_PATH = '/var/lib/dpkg/status';
const DEBIAN_OS_RELEASE_PATH = '/etc/os-release';
const ALPINE_INSTALLED_PATH = '/lib/apk/db/installed';
const ALPINE_RELEASE_PATH = '/etc/alpine-release';

const MAX_ARCHIVE_SIZE_BYTES = 20 * 1024 * 1024;

/**
 * Helper to detect OS release version from /etc/os-release
 */
async function detectOSRelease(
  container: Dockerode.Container,
  releasePath: string,
  logger: Logger,
): Promise<string | null> {
  try {
    const stream = await container.getArchive({ path: releasePath });
    const chunks: Buffer[] = [];

    for await (const chunk of stream) {
      chunks.push(chunk as Buffer);
    }

    const buffer = Buffer.concat(chunks);
    const content = await parseTarStream(buffer, releasePath);
    if (!content) return null;

    // Parse /etc/os-release for VERSION_ID
    const versionMatch = content.match(/VERSION_ID="?([^"\n]+)"?/);
    if (versionMatch?.[1]) {
      return versionMatch[1];
    }

    return null;
  } catch (error) {
    logger.debug({ error, path: releasePath }, 'Could not detect OS release');
    return null;
  }
}

/**
 * Helper to detect Alpine release version from /etc/alpine-release
 */
async function detectAlpineRelease(
  container: Dockerode.Container,
  logger: Logger,
): Promise<string | null> {
  try {
    const stream = await container.getArchive({ path: ALPINE_RELEASE_PATH });
    const chunks: Buffer[] = [];

    for await (const chunk of stream) {
      chunks.push(chunk as Buffer);
    }

    const buffer = Buffer.concat(chunks);
    const content = await parseTarStream(buffer, ALPINE_RELEASE_PATH);
    if (!content) return null;

    // /etc/alpine-release contains version like "3.18.4"
    // Extract major.minor (e.g., "3.18")
    const versionMatch = content.trim().match(/^(\d+\.\d+)/);
    if (versionMatch?.[1]) {
      return versionMatch[1];
    }

    return null;
  } catch (error) {
    logger.debug({ error }, 'Could not detect Alpine release');
    return null;
  }
}

export async function extractPackagesFromImage(
  docker: Dockerode,
  imageId: string,
  logger: Logger,
): Promise<ExtractedPackage[]> {
  const packages: ExtractedPackage[] = [];

  try {
    const image = docker.getImage(imageId);
    const inspectData = await image.inspect();

    logger.debug({ imageId, layers: inspectData.RootFS?.Layers?.length }, 'Inspecting image');

    const container = await docker.createContainer({
      Image: imageId,
      AttachStdout: false,
      AttachStderr: false,
    });

    try {
      // Detect Debian release (best effort - continue even if detection fails)
      const debianRelease = await detectOSRelease(container, DEBIAN_OS_RELEASE_PATH, logger);
      if (debianRelease) {
        logger.debug({ release: debianRelease }, 'Detected Debian release');
      }

      // Detect Alpine release (best effort - continue even if detection fails)
      const alpineRelease = await detectAlpineRelease(container, logger);
      if (alpineRelease) {
        logger.debug({ release: alpineRelease }, 'Detected Alpine release');
      }

      // Extract Debian packages (try regardless of release detection)
      try {
        const stream = await container.getArchive({ path: DEBIAN_STATUS_PATH });
        const chunks: Buffer[] = [];
        let totalSize = 0;

        for await (const chunk of stream) {
          const chunkBuffer = chunk as Buffer;
          totalSize += chunkBuffer.length;

          if (totalSize > MAX_ARCHIVE_SIZE_BYTES) {
            logger.warn(
              { path: DEBIAN_STATUS_PATH, size: totalSize },
              'Archive exceeds size limit',
            );
            for await (const _ of stream) {
              // Drain stream
            }
            break;
          }

          chunks.push(chunkBuffer);
        }

        if (totalSize <= MAX_ARCHIVE_SIZE_BYTES) {
          const buffer = Buffer.concat(chunks);
          const extracted = await parseDebianPackages(
            buffer,
            DEBIAN_STATUS_PATH,
            logger,
            debianRelease ?? undefined,
          );
          packages.push(...extracted);
          logger.info({ count: extracted.length }, 'Extracted Debian packages');
        }
      } catch {
        logger.debug({ path: DEBIAN_STATUS_PATH }, 'Debian status file not found');
      }

      // Extract Alpine packages (try regardless of release detection)
      try {
        const stream = await container.getArchive({ path: ALPINE_INSTALLED_PATH });
        const chunks: Buffer[] = [];
        let totalSize = 0;

        for await (const chunk of stream) {
          const chunkBuffer = chunk as Buffer;
          totalSize += chunkBuffer.length;

          if (totalSize > MAX_ARCHIVE_SIZE_BYTES) {
            logger.warn(
              { path: ALPINE_INSTALLED_PATH, size: totalSize },
              'Archive exceeds size limit',
            );
            for await (const _ of stream) {
              // Drain stream
            }
            break;
          }

          chunks.push(chunkBuffer);
        }

        if (totalSize <= MAX_ARCHIVE_SIZE_BYTES) {
          const buffer = Buffer.concat(chunks);
          const extracted = await parseAlpinePackages(
            buffer,
            ALPINE_INSTALLED_PATH,
            logger,
            alpineRelease ?? undefined,
          );
          packages.push(...extracted);
          logger.info({ count: extracted.length }, 'Extracted Alpine packages');
        }
      } catch {
        logger.debug({ path: ALPINE_INSTALLED_PATH }, 'Alpine installed file not found');
      }

      // Extract Maven packages
      for (const manifestPath of MAVEN_MANIFEST_PATHS) {
        try {
          const stream = await container.getArchive({ path: manifestPath });
          const chunks: Buffer[] = [];
          let totalSize = 0;

          for await (const chunk of stream) {
            const chunkBuffer = chunk as Buffer;
            totalSize += chunkBuffer.length;

            if (totalSize > MAX_ARCHIVE_SIZE_BYTES) {
              logger.warn({ path: manifestPath, size: totalSize }, 'Archive exceeds size limit');
              for await (const _ of stream) {
                // Drain stream
              }
              break;
            }

            chunks.push(chunkBuffer);
          }

          if (totalSize <= MAX_ARCHIVE_SIZE_BYTES) {
            const buffer = Buffer.concat(chunks);
            const extracted = await parseMavenPackages(buffer, manifestPath, logger);
            packages.push(...extracted);
          }
        } catch {
          logger.debug({ path: manifestPath }, 'pom.xml not found');
        }
      }
    } finally {
      try {
        await container.remove();
      } catch (cleanupErr) {
        logger.warn({ error: extractErrorMessage(cleanupErr) }, 'Failed to cleanup container');
      }
    }

    logger.info({ packageCount: packages.length }, 'Extracted all packages');
    return packages;
  } catch (error) {
    logger.error({ error: extractErrorMessage(error) }, 'Failed to inspect image');
    throw error;
  }
}
