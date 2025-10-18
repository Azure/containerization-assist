/**
 * Unit Tests: Validate Dockerfile Tool
 * Tests the validate-dockerfile tool functionality with policy-based validation
 */

import { jest } from '@jest/globals';
import { vol } from 'memfs';
import * as yaml from 'js-yaml';
import type { Policy } from '../../../src/config/policy-schemas';

// Mock file system
jest.mock('node:fs', () => require('memfs').fs);
jest.mock('node:fs/promises', () => require('memfs').fs.promises);

jest.mock('../../../src/lib/logger', () => ({
  createLogger: jest.fn(() => ({
    info: jest.fn(),
    error: jest.fn(),
    warn: jest.fn(),
    debug: jest.fn(),
    trace: jest.fn(),
    fatal: jest.fn(),
    child: jest.fn().mockReturnThis(),
  })),
}));

import tool from '../../../src/tools/validate-dockerfile/tool';
import { createLogger } from '../../../src/lib/logger';
import { clearPolicyCache } from '../../../src/config/policy-io';

const mockLogger = (createLogger as jest.Mock)();

function createMockToolContext() {
  return {
    logger: mockLogger,
  } as any;
}

describe('validate-dockerfile tool', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    vol.reset();
    clearPolicyCache();
  });

  const sampleDockerfile = `FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
CMD ["npm", "start"]`;

  const createTestPolicy = (rules: Policy['rules']): Policy => ({
    version: '2.0',
    metadata: {
      name: 'Test Policy',
      description: 'Test policy for validation',
    },
    defaults: {
      enforcement: 'strict',
    },
    rules,
  });

  describe('policy-based validation', () => {
    it('should pass when no policies exist', async () => {
      const context = createMockToolContext();
      const result = await tool.handler(
        { dockerfile: sampleDockerfile },
        context,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.passed).toBe(true);
        expect(result.value.summary.totalRules).toBe(0);
      }
    });

    it('should detect blocking violations', async () => {
      const policy = createTestPolicy([
        {
          id: 'block-latest-tag',
          category: 'quality',
          priority: 80,
          description: 'Prevent :latest tag',
          conditions: [
            {
              kind: 'regex',
              pattern: 'FROM\\s+[^:]+:latest',
              flags: 'im',
            },
          ],
          actions: {
            block: true,
            message: 'Using :latest tag is not allowed',
          },
        },
      ]);

      vol.fromJSON({
        '/test/policies/test.yaml': yaml.dump(policy),
      });

      const dockerfileWithLatest = `FROM node:latest
CMD ["npm", "start"]`;

      const context = createMockToolContext();
      const result = await tool.handler(
        { dockerfile: dockerfileWithLatest, policyPath: '/test/policies/test.yaml' },
        context,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.passed).toBe(false);
        expect(result.value.violations).toHaveLength(1);
        expect(result.value.violations[0]?.severity).toBe('block');
        expect(result.value.violations[0]?.ruleId).toBe('block-latest-tag');
        expect(result.value.violations[0]?.message).toContain('latest');
      }
    });

    it('should detect warnings', async () => {
      const policy = createTestPolicy([
        {
          id: 'warn-npm-install',
          category: 'quality',
          priority: 70,
          description: 'Warn about npm install usage',
          conditions: [
            {
              kind: 'function',
              name: 'hasPattern',
              args: ['npm install', 'i'],
            },
          ],
          actions: {
            warn: true,
            message: 'Consider using npm ci for reproducible builds',
          },
        },
      ]);

      vol.fromJSON({
        '/test/policies/test.yaml': yaml.dump(policy),
      });

      const context = createMockToolContext();
      const result = await tool.handler(
        { dockerfile: sampleDockerfile, policyPath: '/test/policies/test.yaml' },
        context,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.passed).toBe(true); // Warnings don't fail validation
        expect(result.value.warnings).toHaveLength(1);
        expect(result.value.warnings[0]?.severity).toBe('warn');
      }
    });

    it('should detect suggestions', async () => {
      const policy = createTestPolicy([
        {
          id: 'suggest-workdir',
          category: 'performance',
          priority: 60,
          description: 'Recommend explicit WORKDIR',
          conditions: [
            {
              kind: 'function',
              name: 'hasPattern',
              args: ['^WORKDIR', 'im'],
            },
          ],
          actions: {
            suggest: true,
            message: 'Good practice: explicit WORKDIR is used',
          },
        },
      ]);

      vol.fromJSON({
        '/test/policies/test.yaml': yaml.dump(policy),
      });

      const context = createMockToolContext();
      const result = await tool.handler(
        { dockerfile: sampleDockerfile, policyPath: '/test/policies/test.yaml' },
        context,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.passed).toBe(true); // Suggestions don't fail validation
        expect(result.value.suggestions).toHaveLength(1);
        expect(result.value.suggestions[0]?.severity).toBe('suggest');
        expect(result.value.suggestions[0]?.message).toContain('WORKDIR');
      }
    });

    it('should classify multiple rule types correctly', async () => {
      const policy = createTestPolicy([
        {
          id: 'block-root',
          category: 'security',
          priority: 95,
          conditions: [{ kind: 'regex', pattern: '^USER\\s+(root|0)\\s*$', flags: 'm' }],
          actions: { block: true, message: 'Root user not allowed' },
        },
        {
          id: 'warn-copy-pattern',
          category: 'security',
          priority: 90,
          conditions: [{ kind: 'function', name: 'hasPattern', args: ['^COPY', 'm'] }],
          actions: { warn: true, message: 'Review COPY operations for security' },
        },
        {
          id: 'suggest-multistage',
          category: 'performance',
          priority: 65,
          conditions: [{ kind: 'regex', pattern: 'npm\\s+run\\s+build', flags: 'im' }],
          actions: { suggest: true, message: 'Consider multi-stage builds' },
        },
      ]);

      vol.fromJSON({
        '/test/policies/test.yaml': yaml.dump(policy),
      });

      const context = createMockToolContext();
      const result = await tool.handler(
        { dockerfile: sampleDockerfile, policyPath: '/test/policies/test.yaml' },
        context,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.summary.blockingViolations).toBe(0);
        expect(result.value.summary.warnings).toBe(1);
        expect(result.value.summary.suggestions).toBe(0);
      }
    });
  });

  describe('multiple policies', () => {
    it('should load and merge multiple policy files', async () => {
      const policy1 = createTestPolicy([
        {
          id: 'rule-1',
          priority: 80,
          conditions: [{ kind: 'regex', pattern: ':latest' }],
          actions: { block: true, message: 'No latest tags' },
        },
      ]);

      const policy2 = createTestPolicy([
        {
          id: 'rule-2',
          priority: 70,
          conditions: [{ kind: 'regex', pattern: 'USER root' }],
          actions: { warn: true, message: 'Avoid root user' },
        },
      ]);

      vol.fromJSON({
        '/test/policies/policy1.yaml': yaml.dump(policy1),
        '/test/policies/policy2.yaml': yaml.dump(policy2),
      });

      // Mock process.cwd() to return /test
      jest.spyOn(process, 'cwd').mockReturnValue('/test');

      const context = createMockToolContext();
      const result = await tool.handler(
        { dockerfile: sampleDockerfile },
        context,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.summary.totalRules).toBe(2);
      }

      jest.restoreAllMocks();
    });
  });

  describe('result format', () => {
    it('should return correct result structure', async () => {
      const policy = createTestPolicy([
        {
          id: 'test-rule',
          priority: 80,
          conditions: [{ kind: 'regex', pattern: 'FROM' }],
          actions: { suggest: true, message: 'Test message' },
        },
      ]);

      vol.fromJSON({
        '/test/policies/test.yaml': yaml.dump(policy),
      });

      const context = createMockToolContext();
      const result = await tool.handler(
        { dockerfile: sampleDockerfile, policyPath: '/test/policies/test.yaml' },
        context,
      );

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value).toHaveProperty('success');
        expect(result.value).toHaveProperty('passed');
        expect(result.value).toHaveProperty('violations');
        expect(result.value).toHaveProperty('warnings');
        expect(result.value).toHaveProperty('suggestions');
        expect(result.value).toHaveProperty('summary');
        expect(result.value.summary).toHaveProperty('totalRules');
        expect(result.value.summary).toHaveProperty('matchedRules');
        expect(result.value.summary).toHaveProperty('blockingViolations');
        expect(result.value.summary).toHaveProperty('warnings');
        expect(result.value.summary).toHaveProperty('suggestions');
      }
    });

    it('should include violation details', async () => {
      const policy = createTestPolicy([
        {
          id: 'security-rule',
          category: 'security',
          priority: 95,
          description: 'Security check',
          conditions: [{ kind: 'regex', pattern: 'FROM' }],
          actions: { block: true, message: 'Blocked by security rule' },
        },
      ]);

      vol.fromJSON({
        '/test/policies/test.yaml': yaml.dump(policy),
      });

      const context = createMockToolContext();
      const result = await tool.handler(
        { dockerfile: sampleDockerfile, policyPath: '/test/policies/test.yaml' },
        context,
      );

      expect(result.ok).toBe(true);
      if (result.ok && result.value.violations.length > 0) {
        const violation = result.value.violations[0];
        expect(violation).toHaveProperty('ruleId');
        expect(violation).toHaveProperty('category');
        expect(violation).toHaveProperty('priority');
        expect(violation).toHaveProperty('severity');
        expect(violation).toHaveProperty('message');
        expect(violation?.ruleId).toBe('security-rule');
        expect(violation?.category).toBe('security');
        expect(violation?.priority).toBe(95);
        expect(violation?.severity).toBe('block');
      }
    });
  });

  describe('edge cases', () => {
    it('should handle empty dockerfile', async () => {
      const context = createMockToolContext();
      const result = await tool.handler({}, context);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Either path or dockerfile content is required');
      }
    });

    it('should handle dockerfile without FROM', async () => {
      const policy = createTestPolicy([]);
      vol.fromJSON({
        '/test/policies/test.yaml': yaml.dump(policy),
      });

      const dockerfileWithoutFrom = `WORKDIR /app
COPY . .`;

      const context = createMockToolContext();
      const result = await tool.handler(
        { dockerfile: dockerfileWithoutFrom, policyPath: '/test/policies/test.yaml' },
        context,
      );

      // Should succeed since policies evaluate the content even without FROM
      expect(result.ok).toBe(true);
    });

    it('should handle invalid policy file by falling back to default', async () => {
      vol.fromJSON({
        '/test/policies/invalid.yaml': 'invalid: yaml: content:',
      });

      const context = createMockToolContext();
      const result = await tool.handler(
        { dockerfile: sampleDockerfile, policyPath: '/test/policies/invalid.yaml' },
        context,
      );

      // Policy loader falls back to default policy when YAML parsing fails
      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.passed).toBe(true);
        // Default policy has rules, so totalRules should be > 0
        expect(result.value.summary.totalRules).toBeGreaterThan(0);
      }
    });
  });

  describe('file path handling', () => {
    it('should read Dockerfile from path', async () => {
      const policy = createTestPolicy([]);
      vol.fromJSON({
        '/test/Dockerfile': sampleDockerfile,
        '/test/policies/test.yaml': yaml.dump(policy),
      });

      const context = createMockToolContext();
      const result = await tool.handler(
        { path: '/test/Dockerfile', policyPath: '/test/policies/test.yaml' },
        context,
      );

      expect(result.ok).toBe(true);
    });

    it('should handle missing Dockerfile path', async () => {
      const context = createMockToolContext();
      const result = await tool.handler(
        { path: '/nonexistent/Dockerfile' },
        context,
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Failed to read Dockerfile');
      }
    });
  });
});
