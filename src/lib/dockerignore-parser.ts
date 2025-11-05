import { promises as fs } from 'fs';
import { glob } from 'fs/promises';
import path from 'path';

export async function getDockerBuildFiles(
  contextPath: string,
  dockerfilePath?: string,
): Promise<string[]> {
  const dockerignorePath = path.join(contextPath, '.dockerignore');

  let excludePatterns: string[] = [];
  try {
    const content = await fs.readFile(dockerignorePath, 'utf-8');
    excludePatterns = content
      .split('\n')
      .map((line) => line.trim())
      .filter((line) => line && !line.startsWith('#'));
  } catch {
    // No .dockerignore file exists
  }

  // Use multiple patterns to include hidden files/directories and regular files
  const patterns = ['**/*', '.*', '**/.*', '.*/**/*'];
  const fileSet = new Set<string>();

  for (const pattern of patterns) {
    const filesIterator = glob(pattern, {
      cwd: contextPath,
      exclude: excludePatterns,
      withFileTypes: false,
    });

    for await (const file of filesIterator) {
      if (file !== '.' && file !== '..') {
        fileSet.add(file);
      }
    }
  }

  if (dockerfilePath) {
    const relativeDockerfilePath = path.relative(contextPath, dockerfilePath);
    if (relativeDockerfilePath && !relativeDockerfilePath.startsWith('..')) {
      fileSet.add(relativeDockerfilePath);
    }
  }

  return Array.from(fileSet);
}
