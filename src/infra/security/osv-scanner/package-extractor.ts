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

    const container = await docker.createContainer({
      Image: imageId,
      Cmd: ['/bin/sh', '-c', 'exit 0'],
      AttachStdout: false,
      AttachStderr: false,
    });

    try {
      for (const manifestPath of MAVEN_MANIFEST_PATHS) {
        try {
          const stream = await container.getArchive({ path: manifestPath });
          const chunks: Buffer[] = [];

          for await (const chunk of stream) {
            chunks.push(chunk as Buffer);
          }

          const buffer = Buffer.concat(chunks);
          const extracted = await parseMavenPackages(buffer, manifestPath, logger);
          packages.push(...extracted);
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
