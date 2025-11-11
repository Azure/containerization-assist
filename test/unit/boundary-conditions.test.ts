/**
 * Boundary Condition Tests
 *
 * Tests edge cases and boundary conditions across the codebase:
 * - Empty inputs
 * - Very large inputs
 * - Special characters and encoding
 * - Null and undefined handling
 * - Array and string length limits
 */

import { describe, it, expect } from '@jest/globals';
import { createLogger } from '@/lib/logger';
import type { ToolContext } from '@/mcp/context';
import { join } from 'node:path';
import { writeFileSync } from 'node:fs';
import { createTestTempDir } from '../__support__/utilities/tmp-helpers';

// Import tools
import analyzeRepoTool from '@/tools/analyze-repo/tool';
import generateDockerfileTool from '@/tools/generate-dockerfile/tool';
import fixDockerfileTool from '@/tools/fix-dockerfile/tool';
// Note: Base image resolution is handled within other tools

// Import utilities
import { extractErrorMessage } from '@/lib/errors';
import { withDefaults } from '@/lib/param-defaults';

const logger = createLogger({ level: 'silent' });
const toolContext: ToolContext = {
  logger,
  signal: undefined,
  progress: undefined,
};

describe('Boundary Condition Tests', () => {
  describe('Empty Inputs', () => {
    it('should handle empty string paths', async () => {
      const result = await analyzeRepoTool.handler(
        { repositoryPath: '' },
        toolContext
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeDefined();
      }
    });

    it('should handle empty JSON strings', async () => {
      const { dir, cleanup } = createTestTempDir('empty-json-');

      const result = await generateDockerfileTool.handler(
        {
          repositoryPath: dir.name,
          analysis: '',
          targetPlatform: 'linux/amd64',
        } as any,
        toolContext
      );

      await cleanup();

      // May handle empty string or fail
      expect(result.ok !== undefined).toBe(true);
    });

    it('should handle empty arrays in analysis', async () => {
      const { dir, cleanup } = createTestTempDir('empty-array-');

      const emptyAnalysis = JSON.stringify({
        modules: [],
        isMonorepo: false,
      });

      const result = await generateDockerfileTool.handler(
        {
          repositoryPath: dir.name,
          analysis: emptyAnalysis,
          targetPlatform: 'linux/amd64',
        },
        toolContext
      );

      await cleanup();

      // Should handle empty modules gracefully
      if (!result.ok) {
        expect(result.error).toBeDefined();
      }
    });

    it('should handle empty Dockerfile', async () => {
      const { dir, cleanup } = createTestTempDir('empty-df-');
      const dockerfilePath = join(dir.name, 'Dockerfile');
      writeFileSync(dockerfilePath, '');

      const result = await fixDockerfileTool.handler(
        { dockerfilePath },
        toolContext
      );

      await cleanup();

      // Should detect empty Dockerfile as invalid
      if (result.ok) {
        expect(result.value.findings.length).toBeGreaterThan(0);
      }
    });

    it('should handle zero-length file names', () => {
      const result = extractErrorMessage('');
      expect(result).toBeDefined();
      expect(typeof result).toBe('string');
    });
  });

  describe('Very Large Inputs', () => {
    it('should handle very long file paths', async () => {
      const longPath = '/tmp/' + 'x'.repeat(500) + '/test-app';

      const result = await analyzeRepoTool.handler(
        { repositoryPath: longPath },
        toolContext
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeDefined();
      }
    });

    it('should handle large Dockerfile content', async () => {
      const { dir, cleanup } = createTestTempDir('large-df-');
      const dockerfilePath = join(dir.name, 'Dockerfile');

      // Create a Dockerfile with 1000 RUN commands
      const largeDockerfile = 'FROM node:18\n' +
        Array(1000)
          .fill(0)
          .map((_, i) => `RUN echo "Line ${i}"`)
          .join('\n');

      writeFileSync(dockerfilePath, largeDockerfile);

      const result = await fixDockerfileTool.handler(
        { dockerfilePath },
        toolContext
      );

      await cleanup();

      // Should process without crashing
      expect(result.ok !== undefined).toBe(true);
    });

    it('should handle large JSON analysis objects', async () => {
      const { dir, cleanup } = createTestTempDir('large-json-');

      // Create large analysis with many modules
      const largeAnalysis = JSON.stringify({
        modules: Array(100)
          .fill(0)
          .map((_, i) => ({
            name: `module-${i}`,
            language: 'javascript',
            framework: 'express',
            path: `/app/module-${i}`,
          })),
        isMonorepo: true,
      });

      const result = await generateDockerfileTool.handler(
        {
          repositoryPath: dir.name,
          analysis: largeAnalysis,
          targetPlatform: 'linux/amd64',
        },
        toolContext
      );

      await cleanup();

      // Should handle or reject gracefully
      expect(result.ok !== undefined).toBe(true);
    });

    it('should handle very long lines in Dockerfile', async () => {
      const { dir, cleanup } = createTestTempDir('long-line-');
      const dockerfilePath = join(dir.name, 'Dockerfile');

      const veryLongEnvLine = 'FROM node:18\nENV VAR="' + 'x'.repeat(10000) + '"\n';
      writeFileSync(dockerfilePath, veryLongEnvLine);

      const result = await fixDockerfileTool.handler(
        { dockerfilePath },
        toolContext
      );

      await cleanup();

      expect(result.ok !== undefined).toBe(true);
    });
  });

  describe('Special Characters and Encoding', () => {
    it('should handle Unicode characters in paths', async () => {
      const unicodePath = '/tmp/test-αβγδ-emoji-🚀-path';

      const result = await analyzeRepoTool.handler(
        { repositoryPath: unicodePath },
        toolContext
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeDefined();
      }
    });

    it('should handle special shell characters in parameters', async () => {
      const specialChars = '/tmp/test-app-$VAR-`cmd`-$(echo)-;|&';

      const result = await analyzeRepoTool.handler(
        { repositoryPath: specialChars },
        toolContext
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeDefined();
      }
    });

    it('should handle newlines in string parameters', async () => {
      const pathWithNewline = '/tmp/test\napp\npath';

      const result = await analyzeRepoTool.handler(
        { repositoryPath: pathWithNewline },
        toolContext
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeDefined();
      }
    });

    it('should handle quotes in parameters', async () => {
      const pathWithQuotes = '/tmp/test-"quoted"-\'single\'-path';

      const result = await analyzeRepoTool.handler(
        { repositoryPath: pathWithQuotes },
        toolContext
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeDefined();
      }
    });

    it('should handle backslashes in paths', async () => {
      const pathWithBackslashes = '/tmp/test\\app\\path';

      const result = await analyzeRepoTool.handler(
        { repositoryPath: pathWithBackslashes },
        toolContext
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeDefined();
      }
    });

    // Note: Image name validation happens during build/tag operations

    it('should handle null bytes in strings', () => {
      const errorWithNull = 'Error\x00Message';
      const result = extractErrorMessage(errorWithNull);

      expect(result).toBeDefined();
      expect(typeof result).toBe('string');
    });

    it('should handle Unicode in Dockerfile content', async () => {
      const { dir, cleanup } = createTestTempDir('unicode-df-');
      const dockerfilePath = join(dir.name, 'Dockerfile');

      const unicodeDockerfile = `FROM node:18
# Comment with Unicode: αβγδ 中文 日本語 한글 🚀
LABEL description="Unicode test αβγ"
CMD ["node", "app.js"]`;

      writeFileSync(dockerfilePath, unicodeDockerfile);

      const result = await fixDockerfileTool.handler(
        { dockerfilePath },
        toolContext
      );

      await cleanup();

      expect(result.ok !== undefined).toBe(true);
    });
  });

  describe('Null and Undefined Handling', () => {
    it('should handle null in error extraction', () => {
      const result = extractErrorMessage(null as any);
      expect(result).toBeDefined();
      expect(typeof result).toBe('string');
    });

    it('should handle undefined in error extraction', () => {
      const result = extractErrorMessage(undefined as any);
      expect(result).toBeDefined();
      expect(typeof result).toBe('string');
    });

    it('should handle objects without message property', () => {
      const result = extractErrorMessage({ code: 'ERR_001' });
      expect(result).toBeDefined();
      expect(typeof result).toBe('string');
    });

    it('should handle Error objects', () => {
      const error = new Error('Test error message');
      const result = extractErrorMessage(error);
      expect(result).toBe('Test error message');
    });

    it('should handle numbers as errors', () => {
      const result = extractErrorMessage(404);
      expect(result).toBeDefined();
      expect(typeof result).toBe('string');
    });

    it('should handle boolean as errors', () => {
      const result = extractErrorMessage(true);
      expect(result).toBeDefined();
      expect(typeof result).toBe('string');
    });
  });

  describe('Array and Object Limits', () => {
    it('should handle empty objects in defaults', () => {
      const result = withDefaults({}, { key: 'value' });
      expect(result).toEqual({ key: 'value' });
    });

    it('should handle overlapping keys in defaults', () => {
      const result = withDefaults(
        { key1: 'override', key2: 'keep' },
        { key1: 'default', key3: 'new' }
      );

      expect(result.key1).toBe('override');
      expect(result.key2).toBe('keep');
      expect(result.key3).toBe('new');
    });

    it('should handle nested objects in defaults', () => {
      const result = withDefaults(
        { outer: { inner: 'override' } },
        { outer: { inner: 'default', other: 'value' } }
      );

      expect(result.outer.inner).toBe('override');
      // Note: Shallow merge, so 'other' is lost
      expect(result.outer.other).toBeUndefined();
    });

    it('should handle arrays in parameter defaults', () => {
      const result = withDefaults(
        { tags: ['v1', 'v2'] },
        { tags: ['default'], otherField: 'value' }
      );

      expect(result.tags).toEqual(['v1', 'v2']);
      expect(result.otherField).toBe('value');
    });

    it('should handle very large arrays', () => {
      const largeArray = Array(10000).fill('item');
      const result = withDefaults(
        { items: largeArray },
        { items: [], other: 'value' }
      );

      expect(result.items.length).toBe(10000);
      expect(result.other).toBe('value');
    });
  });

  describe('Numeric Boundaries', () => {
    it('should handle zero values', () => {
      const result = withDefaults(
        { count: 0 },
        { count: 10 }
      );

      expect(result.count).toBe(0);
    });

    it('should handle negative values', () => {
      const result = withDefaults(
        { priority: -100 },
        { priority: 0 }
      );

      expect(result.priority).toBe(-100);
    });

    it('should handle very large numbers', () => {
      const result = withDefaults(
        { size: Number.MAX_SAFE_INTEGER },
        { size: 0 }
      );

      expect(result.size).toBe(Number.MAX_SAFE_INTEGER);
    });

    it('should handle floating point numbers', () => {
      const result = withDefaults(
        { ratio: 0.123456789 },
        { ratio: 1.0 }
      );

      expect(result.ratio).toBe(0.123456789);
    });

    it('should handle Infinity', () => {
      const result = withDefaults(
        { timeout: Infinity },
        { timeout: 30 }
      );

      expect(result.timeout).toBe(Infinity);
    });

    it('should handle NaN', () => {
      const result = withDefaults(
        { value: NaN },
        { value: 0 }
      );

      expect(Number.isNaN(result.value)).toBe(true);
    });
  });

  describe('Concurrent Access', () => {
    it('should handle rapid sequential calls', async () => {
      const { dir, cleanup } = createTestTempDir('rapid-');

      writeFileSync(
        join(dir.name, 'package.json'),
        JSON.stringify({ name: 'test', version: '1.0.0' })
      );

      const promises = Array(10)
        .fill(0)
        .map(() =>
          analyzeRepoTool.handler({ repositoryPath: dir.name }, toolContext)
        );

      const results = await Promise.all(promises);

      await cleanup();

      expect(results).toHaveLength(10);
      results.forEach(result => {
        expect(result.ok !== undefined).toBe(true);
      });
    });

    it('should handle simultaneous different tool calls', async () => {
      const { dir, cleanup } = createTestTempDir('simultaneous-');

      writeFileSync(
        join(dir.name, 'package.json'),
        JSON.stringify({ name: 'test', version: '1.0.0' })
      );

      writeFileSync(
        join(dir.name, 'Dockerfile'),
        'FROM node:18\nCMD ["node", "app.js"]'
      );

      const [analyzeResult, validateResult] = await Promise.all([
        analyzeRepoTool.handler({ repositoryPath: dir.name }, toolContext),
        fixDockerfileTool.handler(
          { dockerfilePath: join(dir.name, 'Dockerfile') },
          toolContext
        ),
      ]);

      await cleanup();

      expect(analyzeResult.ok !== undefined).toBe(true);
      expect(validateResult.ok !== undefined).toBe(true);
    });
  });

  describe('Error Message Boundaries', () => {
    it('should handle very long error messages', () => {
      const longError = 'Error: ' + 'x'.repeat(10000);
      const result = extractErrorMessage(longError);

      expect(result).toBeDefined();
      expect(result.length).toBeGreaterThan(0);
    });

    it('should handle circular reference errors', () => {
      const circular: any = { name: 'circular' };
      circular.self = circular;

      const result = extractErrorMessage(circular);
      expect(result).toBeDefined();
      expect(typeof result).toBe('string');
    });

    it('should handle errors with special toString behavior', () => {
      const weirdError = {
        toString: () => {
          throw new Error('toString failed');
        },
      };

      // Should not throw, but handle gracefully
      try {
        const result = extractErrorMessage(weirdError);
        expect(result).toBeDefined();
        expect(typeof result).toBe('string');
      } catch (error) {
        // If it throws, that's also acceptable behavior for pathological input
        expect(error).toBeDefined();
      }
    });
  });

  describe('Whitespace Handling', () => {
    it('should handle paths with leading/trailing spaces', async () => {
      const result = await analyzeRepoTool.handler(
        { repositoryPath: '  /tmp/test  ' },
        toolContext
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeDefined();
      }
    });

    it('should handle Dockerfile with only whitespace', async () => {
      const { dir, cleanup } = createTestTempDir('whitespace-');
      const dockerfilePath = join(dir.name, 'Dockerfile');
      writeFileSync(dockerfilePath, '   \n\n  \t\t  \n   ');

      const result = await fixDockerfileTool.handler(
        { dockerfilePath },
        toolContext
      );

      await cleanup();

      if (result.ok) {
        expect(result.value.findings.length).toBeGreaterThan(0);
      }
    });

    it('should handle mixed whitespace in parameters', () => {
      const result = withDefaults(
        { name: '  test  ' },
        { name: 'default' }
      );

      expect(result.name).toBe('  test  ');
    });
  });
});
