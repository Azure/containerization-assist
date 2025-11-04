/**
 * Unit tests for dockerignore parser
 */

import { describe, it, expect } from '@jest/globals';
import {
  parseDockerignore,
  matchPattern,
  shouldIgnore,
  createIgnoreFunction,
} from '../../../src/lib/dockerignore-parser';

describe('dockerignore-parser', () => {
  describe('parseDockerignore', () => {
    it('should parse patterns, exceptions, and skip comments', () => {
      const content = `
# Comment
node_modules
*.log
!README.md
      `.trim();

      const result = parseDockerignore(content);

      expect(result.patterns).toEqual(['node_modules', '*.log']);
      expect(result.exceptions).toEqual(['README.md']);
    });
  });

  describe('matchPattern', () => {
    it('should match wildcard patterns', () => {
      expect(matchPattern('test.log', '*.log')).toBe(true);
      expect(matchPattern('test.txt', '*.log')).toBe(false);
    });

    it('should match patterns without slash at any level', () => {
      expect(matchPattern('node_modules', 'node_modules')).toBe(true);
      expect(matchPattern('src/node_modules', 'node_modules')).toBe(true);
    });

    it('should match patterns with leading slash only at root level', () => {
      expect(matchPattern('temp', '/temp')).toBe(true);
      expect(matchPattern('src/temp', '/temp')).toBe(false);
    });

    it('should match double star patterns', () => {
      expect(matchPattern('a/b/c/file.txt', '**/*.txt')).toBe(true);
      expect(matchPattern('a/file.txt', '**/*.txt')).toBe(true);
    });

    it('should handle Windows-style paths', () => {
      expect(matchPattern('src\\temp\\file.txt', 'temp/')).toBe(true);
    });
  });

  describe('shouldIgnore', () => {
    it('should handle patterns and exceptions', () => {
      const patterns = ['*.md', '*.log'];
      const exceptions = ['README.md'];

      expect(shouldIgnore('test.log', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('README.md', patterns, exceptions)).toBe(false);
      expect(shouldIgnore('CHANGELOG.md', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('src/file.txt', patterns, exceptions)).toBe(false);
    });
  });

  describe('createIgnoreFunction', () => {
    it('should respect alwaysInclude list', () => {
      const patterns = ['Dockerfile*', '*.md'];
      const exceptions: string[] = [];
      const alwaysInclude = ['Dockerfile'];
      const ignoreFn = createIgnoreFunction(patterns, exceptions, alwaysInclude);

      expect(ignoreFn('Dockerfile')).toBe(false);
      expect(ignoreFn('Dockerfile.dev')).toBe(true);
      expect(ignoreFn('README.md')).toBe(true);
    });
  });

  describe('real-world patterns', () => {
    it('should handle typical .dockerignore with nested patterns', () => {
      const content = `
node_modules
*.md
!README.md
temp/**
!temp/important/**
      `.trim();

      const { patterns, exceptions } = parseDockerignore(content);

      expect(shouldIgnore('node_modules', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('src/node_modules', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('README.md', patterns, exceptions)).toBe(false);
      expect(shouldIgnore('CHANGELOG.md', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('temp/cache/data', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('temp/important/keep.txt', patterns, exceptions)).toBe(false);
    });
  });
});
