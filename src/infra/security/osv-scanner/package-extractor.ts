// Extract Maven packages from Docker image filesystems

import type { Logger } from 'pino';
import type Dockerode from 'dockerode';
import { extractErrorMessage } from '@/lib/errors';
import type { ExtractedPackage } from './maven/types';
import { parseMavenPackages } from './maven/pom-parser';

const MAVEN_MANIFEST_PATHS = [
  '/pom.xml',
  '/app/pom.xml',
  '/usr/src/app/pom.xml',
  '/workspace/pom.xml',
  '/home/app/pom.xml',
];

const MAX_ARCHIVE_SIZE_BYTES = 20 * 1024 * 1024;

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

    logger.info({ packageCount: packages.length }, 'Extracted Maven packages');
    return packages;
  } catch (error) {
    logger.error({ error: extractErrorMessage(error) }, 'Failed to inspect image');
    throw error;
  }
}
