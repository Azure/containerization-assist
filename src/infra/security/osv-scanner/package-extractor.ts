/**
 * Package Extraction from Docker Images
 * Searches for pom.xml files in common Maven locations.
 */

import type { Logger } from 'pino';
import type Dockerode from 'dockerode';
import { extractErrorMessage } from '@/lib/errors';
import type { ExtractedPackage } from './maven/types';
import { parseMavenPackages } from './maven/pom-parser';

/** Common Maven pom.xml locations in Docker images */
const MAVEN_MANIFEST_PATHS = [
  '/pom.xml',
  '/app/pom.xml',
  '/usr/src/app/pom.xml',
  '/workspace/pom.xml',
  '/home/app/pom.xml',
];

/** Maximum size for tar archive stream (20MB - accounts for tar overhead) */
const MAX_ARCHIVE_SIZE_BYTES = 20 * 1024 * 1024;

/**
 * Extract Maven packages from Docker image filesystem
 */
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

    // Create container for file extraction only (never started)
    // No Cmd needed - works with all image types including distroless/scratch
    const container = await docker.createContainer({
      Image: imageId,
      AttachStdout: false,
      AttachStderr: false,
    });

    try {
      for (const manifestPath of MAVEN_MANIFEST_PATHS) {
        try {
          const stream = await container.getArchive({ path: manifestPath });
          const chunks: Buffer[] = [];
          let totalSize = 0;

          // Stream with size limit to prevent memory exhaustion
          for await (const chunk of stream) {
            const chunkBuffer = chunk as Buffer;
            totalSize += chunkBuffer.length;

            // Check if we're exceeding the size limit
            if (totalSize > MAX_ARCHIVE_SIZE_BYTES) {
              logger.warn(
                { path: manifestPath, size: totalSize, limit: MAX_ARCHIVE_SIZE_BYTES },
                'Archive stream exceeds size limit, skipping',
              );
              // Drain remaining stream to prevent connection issues
              for await (const _ of stream) {
                // Just consume remaining chunks
              }
              break;
            }

            chunks.push(chunkBuffer);
          }

          // Only parse if we didn't exceed the limit
          if (totalSize <= MAX_ARCHIVE_SIZE_BYTES) {
            const buffer = Buffer.concat(chunks);
            const extracted = await parseMavenPackages(buffer, manifestPath, logger);
            packages.push(...extracted);
          }
        } catch (err) {
          logger.debug(
            { path: manifestPath, error: extractErrorMessage(err) },
            'pom.xml not found',
          );
        }
      }
    } finally {
      // Always cleanup container, even if extraction fails
      try {
        await container.remove();
      } catch (cleanupErr) {
        logger.warn({ error: extractErrorMessage(cleanupErr) }, 'Failed to cleanup container');
      }
    }

    logger.info({ packageCount: packages.length }, 'Extracted Maven packages');
    return packages;
  } catch (error) {
    logger.error({ error: extractErrorMessage(error) }, 'Failed to inspect image');
    throw error;
  }
}
