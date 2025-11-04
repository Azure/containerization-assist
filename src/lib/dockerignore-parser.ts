import { promises as fs } from 'fs';
import { glob } from 'fs/promises';
import path from 'path';

export async function getDockerBuildFiles(contextPath: string): Promise<string[]> {
  const dockerignorePath = path.join(contextPath, '.dockerignore');

  let excludePatterns: string[] = [];
  try {
    const content = await fs.readFile(dockerignorePath, 'utf-8');
    excludePatterns = content
      .split('\n')
      .map((line) => line.trim())
      .filter((line) => line && !line.startsWith('#') && line !== 'Dockerfile');
  } catch {
    // No .dockerignore file exists
  }

  const filesIterator = glob('**/*', {
    cwd: contextPath,
    exclude: excludePatterns,
    withFileTypes: false,
  });

  const files: string[] = [];
  for await (const file of filesIterator) {
    files.push(file);
  }

  return files;
}
