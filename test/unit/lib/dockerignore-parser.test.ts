import { describe, it, expect, beforeEach, afterEach } from '@jest/globals';
import { getDockerBuildFiles } from '../../../src/lib/dockerignore-parser';
import { promises as fs } from 'fs';
import path from 'path';
import os from 'os';

describe('getDockerBuildFiles', () => {
  let tempDir: string;

  beforeEach(async () => {
    tempDir = await fs.mkdtemp(path.join(os.tmpdir(), 'dockerignore-test-'));
  });

  afterEach(async () => {
    await fs.rm(tempDir, { recursive: true, force: true });
  });

  it('should return all files when no .dockerignore exists', async () => {
    await fs.writeFile(path.join(tempDir, 'test.txt'), 'content');
    await fs.writeFile(path.join(tempDir, 'Dockerfile'), 'FROM node');

    const files = await getDockerBuildFiles(tempDir);

    expect(files).toContain('test.txt');
    expect(files).toContain('Dockerfile');
  });

  it('should exclude files matching .dockerignore patterns', async () => {
    await fs.writeFile(path.join(tempDir, '.dockerignore'), 'node_modules\n*.log');
    await fs.writeFile(path.join(tempDir, 'test.txt'), 'content');
    await fs.writeFile(path.join(tempDir, 'test.log'), 'log');
    await fs.writeFile(path.join(tempDir, 'Dockerfile'), 'FROM node');

    const files = await getDockerBuildFiles(tempDir);

    expect(files).toContain('test.txt');
    expect(files).toContain('Dockerfile');
    expect(files).not.toContain('test.log');
  });

  it('should skip comments in .dockerignore', async () => {
    await fs.writeFile(path.join(tempDir, '.dockerignore'), '# comment\n*.log');
    await fs.writeFile(path.join(tempDir, 'test.log'), 'log');
    await fs.writeFile(path.join(tempDir, 'test.txt'), 'content');

    const files = await getDockerBuildFiles(tempDir);

    expect(files).toContain('test.txt');
    expect(files).not.toContain('test.log');
  });

  it('should handle nested directories', async () => {
    await fs.mkdir(path.join(tempDir, 'src'), { recursive: true });
    await fs.writeFile(path.join(tempDir, '.dockerignore'), 'node_modules');
    await fs.writeFile(path.join(tempDir, 'src', 'index.js'), 'code');
    await fs.writeFile(path.join(tempDir, 'Dockerfile'), 'FROM node');

    const files = await getDockerBuildFiles(tempDir);

    expect(files).toContain('Dockerfile');
    expect(files).toContain('src/index.js');
  });
});
