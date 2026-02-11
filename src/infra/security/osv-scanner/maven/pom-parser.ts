// Maven POM parser with XML bomb protection (max 10MB)
// Local property resolution only - no remote parent POM fetching

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

const MAX_XML_SIZE_BYTES = 10 * 1024 * 1024;

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
          if (entry.path.includes('..')) {
            entry.resume();
            return;
          }

          const entryFileName = entry.path.split('/').pop() || entry.path;
          if (entryFileName === fileName) {
            if (entry.size > MAX_XML_SIZE_BYTES) {
              sizeError = new Error(
                `pom.xml file too large (${entry.size} bytes, max ${MAX_XML_SIZE_BYTES})`,
              );
              entry.resume();
              return;
            }

            const chunks: Buffer[] = [];
            let totalSize = 0;

            entry.on('data', (chunk: Buffer) => {
              totalSize += chunk.length;
              if (totalSize > MAX_XML_SIZE_BYTES) {
                sizeError = new Error(
                  `pom.xml content exceeded size limit (${totalSize} bytes, max ${MAX_XML_SIZE_BYTES})`,
                );
                entry.resume();
                return;
              }
              chunks.push(chunk);
            });

            entry.on('end', () => {
              if (!sizeError) content = Buffer.concat(chunks).toString('utf-8');
            });
          } else {
            entry.resume();
          }
        },
      }) as unknown as NodeJS.WritableStream,
    );
  } catch {
    if (sizeError) throw sizeError;
  }

  if (sizeError) throw sizeError;

  return content;
}

async function parsePomXml(xmlContent: string, logger: Logger): Promise<ExtractedPackage[]> {
  const contentSizeBytes = Buffer.byteLength(xmlContent, 'utf-8');
  if (contentSizeBytes > MAX_XML_SIZE_BYTES) {
    throw new Error(
      `pom.xml content too large (${contentSizeBytes} bytes, max ${MAX_XML_SIZE_BYTES})`,
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

          if (project.dependencies?.dependency) {
            const deps = Array.isArray(project.dependencies.dependency)
              ? project.dependencies.dependency
              : [project.dependencies.dependency];

            for (const dep of deps) {
              if (dep.groupId && dep.artifactId) {
                let version = dep.version;

                if (!version || version.includes('${')) {
                  const key = `${dep.groupId}:${dep.artifactId}`;
                  version = dependencyManagement.get(key);
                }

                if (version?.includes('${')) {
                  version = resolvePropertyValue(version, properties, project);
                }

                if (version && !version.includes('${')) {
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
              'Skipped dependencies with unresolved versions',
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
