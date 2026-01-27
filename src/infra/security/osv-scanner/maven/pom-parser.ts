/**
 * Maven POM Parser
 * Parses pom.xml from Docker tar archives and resolves dependencies.
 * Note: Only resolves local properties - no remote parent POM fetching.
 *
 * Security: Protects against XML bomb attacks (billion laughs, quadratic blowup)
 * by enforcing size limits on XML content.
 */

import type { Logger } from 'pino';
import * as tar from 'tar';
import { parseString } from 'xml2js';
import { extractErrorMessage } from '@/lib/errors';
import type { ExtractedPackage, ParseResult } from './types';
import {
  extractLocalProperties,
  extractLocalDependencyManagement,
  resolvePropertyValue,
} from './property-resolver';

/**
 * Maximum allowed size for pom.xml files (10 MB)
 * This protects against:
 * - Billion laughs attack (exponential entity expansion)
 * - Quadratic blowup attack (nested entities)
 * - Memory exhaustion via large XML files
 *
 * Real-world pom.xml files are typically <100KB. The largest legitimate
 * pom.xml files rarely exceed 1MB. Setting 10MB provides generous headroom
 * while still protecting against malicious XML bombs.
 */
const MAX_XML_SIZE_BYTES = 10 * 1024 * 1024; // 10 MB

/** Parse Maven pom.xml from Docker tar archive */
export async function parseMavenPackages(
  buffer: Buffer,
  path: string,
  logger: Logger,
): Promise<ExtractedPackage[]> {
  try {
    const pomContent = await extractFileFromTar(buffer, path);
    if (!pomContent) {
      logger.debug({ path }, 'pom.xml not found in tar');
      return [];
    }

    const packages = await parsePomXml(pomContent, logger);
    logger.debug({ path, packageCount: packages.length }, 'Parsed Maven dependencies');
    return packages;
  } catch (err) {
    logger.warn({ path, error: extractErrorMessage(err) }, 'Failed to parse pom.xml');
    return [];
  }
}

/** Extract file from tar buffer with size limit protection */
async function extractFileFromTar(buffer: Buffer, targetPath: string): Promise<string | null> {
  const { Readable } = await import('stream');
  const { pipeline } = await import('stream/promises');
  let content: string | null = null;
  let sizeError: Error | null = null;
  const fileName = targetPath.split('/').pop() || targetPath;

  const stream = Readable.from(buffer);

  try {
    await pipeline(
      stream,
      tar.list({
        onentry: (entry) => {
          // Prevent path traversal attacks
          if (entry.path.includes('..')) {
            entry.resume();
            return;
          }

          const entryFileName = entry.path.split('/').pop() || entry.path;
          if (entryFileName === fileName) {
            // Check file size before extracting to prevent XML bomb attacks
            if (entry.size > MAX_XML_SIZE_BYTES) {
              sizeError = new Error(
                `pom.xml file too large (${entry.size} bytes, max ${MAX_XML_SIZE_BYTES}). ` +
                  'Possible XML bomb attack (billion laughs, quadratic blowup).',
              );
              entry.resume(); // Skip this entry
              return;
            }

            const chunks: Buffer[] = [];
            let totalSize = 0;

            entry.on('data', (chunk: Buffer) => {
              totalSize += chunk.length;
              // Double-check accumulated size to catch decompression bombs
              if (totalSize > MAX_XML_SIZE_BYTES) {
                sizeError = new Error(
                  `pom.xml content exceeded size limit during extraction (${totalSize} bytes, max ${MAX_XML_SIZE_BYTES}). ` +
                    'Possible XML bomb attack.',
                );
                entry.resume(); // Stop reading
                return;
              }
              chunks.push(chunk);
            });

            entry.on('end', () => {
              if (!sizeError) {
                content = Buffer.concat(chunks).toString('utf-8');
              }
            });
          } else {
            entry.resume();
          }
        },
      }) as unknown as NodeJS.WritableStream,
    );
  } catch {
    // Ignore pipeline errors unless it's our size error
    if (sizeError) {
      throw sizeError;
    }
  }

  // Throw size error if detected
  if (sizeError) {
    throw sizeError;
  }

  return content;
}

/** Parse pom.xml and extract dependencies with local property resolution only */
async function parsePomXml(xmlContent: string, logger: Logger): Promise<ExtractedPackage[]> {
  // Final size check before parsing (defense in depth)
  // This catches cases where the content wasn't extracted from tar
  const contentSizeBytes = Buffer.byteLength(xmlContent, 'utf-8');
  if (contentSizeBytes > MAX_XML_SIZE_BYTES) {
    throw new Error(
      `pom.xml content too large (${contentSizeBytes} bytes, max ${MAX_XML_SIZE_BYTES}). ` +
        'Possible XML bomb attack (billion laughs, quadratic blowup).',
    );
  }

  return new Promise((resolve, reject) => {
    parseString(
      xmlContent,
      {
        trim: true,
        explicitArray: false,
        strict: true,
        xmlns: false,
        explicitCharkey: false,
      },
      (err: Error | null, result: ParseResult) => {
        if (err) {
          reject(err);
          return;
        }

        try {
          const project = result.project;

          if (!project) {
            logger.debug('No project element in pom.xml');
            resolve([]);
            return;
          }

          const properties = extractLocalProperties(project, logger);
          const dependencyManagement = extractLocalDependencyManagement(project);
          const packages: ExtractedPackage[] = [];
          let skippedDeps = 0;

          // Extract dependencies
          if (project.dependencies?.dependency) {
            const deps = Array.isArray(project.dependencies.dependency)
              ? project.dependencies.dependency
              : [project.dependencies.dependency];

            for (const dep of deps) {
              if (dep.groupId && dep.artifactId) {
                let version = dep.version;

                // Try to resolve from dependency management if no explicit version
                if (!version || version.includes('${')) {
                  const key = `${dep.groupId}:${dep.artifactId}`;
                  version = dependencyManagement.get(key);
                }

                // Try to resolve property placeholders
                if (version?.includes('${')) {
                  version = resolvePropertyValue(version, properties, project);
                }

                // Only include if version is fully resolved
                if (version && !version.includes('${')) {
                  packages.push({
                    name: `${dep.groupId}:${dep.artifactId}`,
                    version,
                    ecosystem: 'Maven' as const,
                  });
                } else {
                  skippedDeps++;
                  logger.debug(
                    { dependency: `${dep.groupId}:${dep.artifactId}`, version: dep.version },
                    'Skipping dependency with unresolved version (parent POM needed)',
                  );
                }
              }
            }
          }

          // Extract dependency management
          if (project.dependencyManagement?.dependencies?.dependency) {
            const deps = Array.isArray(project.dependencyManagement.dependencies.dependency)
              ? project.dependencyManagement.dependencies.dependency
              : [project.dependencyManagement.dependencies.dependency];

            for (const dep of deps) {
              if (dep.groupId && dep.artifactId && dep.version) {
                let version = dep.version;

                if (version.includes('${')) {
                  version = resolvePropertyValue(version, properties, project);
                }

                if (!version.includes('${')) {
                  packages.push({
                    name: `${dep.groupId}:${dep.artifactId}`,
                    version,
                    ecosystem: 'Maven' as const,
                  });
                } else {
                  skippedDeps++;
                }
              }
            }
          }

          if (skippedDeps > 0) {
            logger.info(
              { skippedCount: skippedDeps },
              'Skipped dependencies with unresolved versions - parent POM resolution disabled',
            );
          }

          resolve(packages);
        } catch (parseErr) {
          reject(parseErr);
        }
      },
    );
  });
}
