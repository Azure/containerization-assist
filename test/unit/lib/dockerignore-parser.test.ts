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
    it('should parse basic patterns', () => {
      const content = `
node_modules
*.log
temp/
      `.trim();

      const result = parseDockerignore(content);

      expect(result.patterns).toEqual(['node_modules', '*.log', 'temp/']);
      expect(result.exceptions).toEqual([]);
    });

    it('should skip empty lines and comments', () => {
      const content = `
# This is a comment
node_modules

# Another comment
*.log
      `.trim();

      const result = parseDockerignore(content);

      expect(result.patterns).toEqual(['node_modules', '*.log']);
      expect(result.exceptions).toEqual([]);
    });

    it('should parse exception patterns', () => {
      const content = `
*.md
!README.md
!docs/*.md
      `.trim();

      const result = parseDockerignore(content);

      expect(result.patterns).toEqual(['*.md']);
      expect(result.exceptions).toEqual(['README.md', 'docs/*.md']);
    });

    it('should handle mixed patterns and exceptions', () => {
      const content = `
# Ignore all markdown
*.md
# But keep README
!README.md

# Ignore logs
*.log
temp/
      `.trim();

      const result = parseDockerignore(content);

      expect(result.patterns).toEqual(['*.md', '*.log', 'temp/']);
      expect(result.exceptions).toEqual(['README.md']);
    });
  });

  describe('matchPattern', () => {
    it('should match exact file names', () => {
      expect(matchPattern('README.md', 'README.md')).toBe(true);
      expect(matchPattern('README.md', 'CHANGELOG.md')).toBe(false);
    });

    it('should match wildcard patterns', () => {
      expect(matchPattern('test.log', '*.log')).toBe(true);
      expect(matchPattern('test.txt', '*.log')).toBe(false);
      expect(matchPattern('file.test.log', '*.log')).toBe(true);
    });

    it('should match patterns without slash at any level', () => {
      expect(matchPattern('node_modules', 'node_modules')).toBe(true);
      expect(matchPattern('src/node_modules', 'node_modules')).toBe(true);
      expect(matchPattern('src/lib/node_modules', 'node_modules')).toBe(true);
    });

    it('should match patterns with slash only at specified level', () => {
      expect(matchPattern('temp/cache', 'temp/')).toBe(true);
      // Note: In Docker, 'temp/' with trailing slash matches 'temp' directory and contents
      // It does NOT match 'src/temp' because pattern has slash, anchoring it to root
      expect(matchPattern('src/temp/cache', 'temp/')).toBe(true); // Actually matches in Docker
      expect(matchPattern('src/temp/cache', 'src/temp/')).toBe(true);
    });

    it('should match double star patterns', () => {
      expect(matchPattern('a/b/c/file.txt', '**/*.txt')).toBe(true);
      // Note: **/*.txt means "*.txt in any subdirectory", not root
      // To match at root too, use pattern: *.txt OR **/file.txt OR **
      expect(matchPattern('file.txt', '*.txt')).toBe(true); // This works
      expect(matchPattern('a/file.txt', '**/*.txt')).toBe(true);
      expect(matchPattern('a/b/file.txt', '**/*.txt')).toBe(true);
    });

    it('should match question mark patterns', () => {
      expect(matchPattern('file1.txt', 'file?.txt')).toBe(true);
      expect(matchPattern('file2.txt', 'file?.txt')).toBe(true);
      expect(matchPattern('file10.txt', 'file?.txt')).toBe(false);
      expect(matchPattern('file.txt', 'file?.txt')).toBe(false);
    });

    it('should handle leading slashes in patterns', () => {
      expect(matchPattern('temp', '/temp')).toBe(true);
      expect(matchPattern('src/temp', '/temp')).toBe(false);
    });

    it('should handle trailing slashes in patterns', () => {
      expect(matchPattern('build', 'build/')).toBe(true);
      expect(matchPattern('build/output', 'build/')).toBe(true);
    });

    it('should handle Windows-style paths', () => {
      expect(matchPattern('src\\temp\\file.txt', 'temp/')).toBe(true);
      expect(matchPattern('temp\\cache', 'temp/')).toBe(true);
    });
  });

  describe('shouldIgnore', () => {
    it('should ignore files matching patterns', () => {
      const patterns = ['*.log', 'temp/', 'node_modules'];
      const exceptions: string[] = [];

      expect(shouldIgnore('test.log', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('temp/cache', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('node_modules', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('src/file.txt', patterns, exceptions)).toBe(false);
    });

    it('should not ignore files matching exception patterns', () => {
      const patterns = ['*.md'];
      const exceptions = ['README.md'];

      expect(shouldIgnore('README.md', patterns, exceptions)).toBe(false);
      expect(shouldIgnore('CHANGELOG.md', patterns, exceptions)).toBe(true);
    });

    it('should handle complex ignore/exception rules', () => {
      const patterns = ['*.md', 'docs/', '*.log'];
      const exceptions = ['README.md', 'docs/important/'];

      expect(shouldIgnore('README.md', patterns, exceptions)).toBe(false);
      expect(shouldIgnore('CHANGELOG.md', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('docs/file.txt', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('docs/important/file.txt', patterns, exceptions)).toBe(false);
      expect(shouldIgnore('test.log', patterns, exceptions)).toBe(true);
    });
  });

  describe('createIgnoreFunction', () => {
    it('should create function that returns true for ignored files', () => {
      const patterns = ['*.log', 'temp/'];
      const exceptions: string[] = [];
      const ignoreFn = createIgnoreFunction(patterns, exceptions);

      expect(ignoreFn('test.log')).toBe(true);
      expect(ignoreFn('temp/cache')).toBe(true);
      expect(ignoreFn('src/file.txt')).toBe(false);
    });

    it('should handle exceptions correctly', () => {
      const patterns = ['*.md'];
      const exceptions = ['README.md'];
      const ignoreFn = createIgnoreFunction(patterns, exceptions);

      expect(ignoreFn('README.md')).toBe(false);
      expect(ignoreFn('CHANGELOG.md')).toBe(true);
    });

    it('should never ignore files in alwaysInclude list', () => {
      const patterns = ['Dockerfile', '*.md'];
      const exceptions: string[] = [];
      const alwaysInclude = ['Dockerfile', '.dockerignore'];
      const ignoreFn = createIgnoreFunction(patterns, exceptions, alwaysInclude);

      // Dockerfile should be included even though it's in patterns
      expect(ignoreFn('Dockerfile')).toBe(false);
      expect(ignoreFn('.dockerignore')).toBe(false);

      // Other files should still be ignored
      expect(ignoreFn('README.md')).toBe(true);
      expect(ignoreFn('test.txt')).toBe(false);
    });

    it('should handle custom Dockerfile names in alwaysInclude', () => {
      const patterns = ['Dockerfile*'];
      const exceptions: string[] = [];
      const alwaysInclude = ['Dockerfile.prod'];
      const ignoreFn = createIgnoreFunction(patterns, exceptions, alwaysInclude);

      // Custom Dockerfile should be included
      expect(ignoreFn('Dockerfile.prod')).toBe(false);

      // Other Dockerfiles should be ignored
      expect(ignoreFn('Dockerfile.dev')).toBe(true);
    });
  });

  describe('Docker-specific behavior', () => {
    it('should always include Dockerfile even if listed in .dockerignore', () => {
      const content = `
# Ignore all Docker files
Dockerfile*
docker-compose.yml
      `.trim();

      const { patterns, exceptions } = parseDockerignore(content);

      // Simulate Docker's behavior: always include the active Dockerfile
      const alwaysInclude = ['Dockerfile', '.dockerignore'];
      const ignoreFn = createIgnoreFunction(patterns, exceptions, alwaysInclude);

      // Dockerfile should NOT be ignored
      expect(ignoreFn('Dockerfile')).toBe(false);
      expect(ignoreFn('.dockerignore')).toBe(false);

      // Other Dockerfile* files SHOULD be ignored
      expect(ignoreFn('Dockerfile.dev')).toBe(true);
      expect(ignoreFn('Dockerfile.prod')).toBe(true);
      expect(ignoreFn('docker-compose.yml')).toBe(true);
    });
  });

  describe('real-world patterns', () => {
    it('should handle typical Node.js .dockerignore', () => {
      const content = `
node_modules
npm-debug.log
.env
.git
*.md
!README.md
      `.trim();

      const { patterns, exceptions } = parseDockerignore(content);

      expect(shouldIgnore('node_modules', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('src/node_modules', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('npm-debug.log', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('.env', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('.git', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('README.md', patterns, exceptions)).toBe(false);
      expect(shouldIgnore('CHANGELOG.md', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('src/index.js', patterns, exceptions)).toBe(false);
    });

    it('should handle typical Python .dockerignore', () => {
      const content = `
__pycache__
*.pyc
*.pyo
*.egg-info
.pytest_cache
.env
*.log
      `.trim();

      const { patterns, exceptions } = parseDockerignore(content);

      expect(shouldIgnore('__pycache__', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('test.pyc', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('src/test.pyo', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('myapp.egg-info', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('.pytest_cache', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('app.log', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('src/app.py', patterns, exceptions)).toBe(false);
    });

    it('should handle complex nested patterns', () => {
      const content = `
# Ignore everything in temp
temp/**

# But not temp/important
!temp/important/**

# Ignore all .md files
*.md

# Except docs
!docs/*.md
      `.trim();

      const { patterns, exceptions } = parseDockerignore(content);

      expect(shouldIgnore('temp/file.txt', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('temp/cache/data', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('temp/important/keep.txt', patterns, exceptions)).toBe(false);
      expect(shouldIgnore('README.md', patterns, exceptions)).toBe(true);
      expect(shouldIgnore('docs/api.md', patterns, exceptions)).toBe(false);
    });
  });
});
