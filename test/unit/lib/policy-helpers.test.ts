/**
 * Tests for policy-helpers utilities
 */

import {
  mapRegoPolicyViolation,
  validateContentAgainstPolicy,
  type PolicyViolation,
  type PolicyValidationResult,
} from '@/lib/policy-helpers';
import type { RegoPolicyViolation, RegoEvaluator, RegoPolicyResult } from '@/config/policy-rego';
import { createLogger } from '@/lib/logger';
import type { Logger } from 'pino';

describe('policy-helpers', () => {
  let logger: Logger;

  beforeEach(() => {
    logger = createLogger({ name: 'test-policy-helpers', level: 'silent' });
  });

  describe('mapRegoPolicyViolation', () => {
    it('should map blocking violations with all fields', () => {
      const regoViolation: RegoPolicyViolation = {
        rule: 'block-root-user',
        category: 'security',
        priority: 95,
        message: 'Running as root user is not allowed',
        severity: 'block',
        description: 'Containers should not run as root',
      };

      const result = mapRegoPolicyViolation(regoViolation);

      expect(result).toEqual({
        ruleId: 'block-root-user',
        category: 'security',
        priority: 95,
        message: 'Running as root user is not allowed',
        severity: 'block',
        description: 'Containers should not run as root',
      });
    });

    it('should map warnings without optional fields', () => {
      const regoViolation: RegoPolicyViolation = {
        rule: 'require-healthcheck',
        category: 'quality',
        message: 'HEALTHCHECK directive is recommended',
        severity: 'warn',
      };

      const result = mapRegoPolicyViolation(regoViolation);

      expect(result).toEqual({
        ruleId: 'require-healthcheck',
        category: 'quality',
        message: 'HEALTHCHECK directive is recommended',
        severity: 'warn',
      });
      expect(result.priority).toBeUndefined();
      expect(result.description).toBeUndefined();
    });

    it('should map suggestions with priority but no description', () => {
      const regoViolation: RegoPolicyViolation = {
        rule: 'use-specific-tags',
        category: 'best-practice',
        priority: 50,
        message: 'Consider using specific image tags',
        severity: 'suggest',
      };

      const result = mapRegoPolicyViolation(regoViolation);

      expect(result).toEqual({
        ruleId: 'use-specific-tags',
        category: 'best-practice',
        priority: 50,
        message: 'Consider using specific image tags',
        severity: 'suggest',
      });
    });
  });

  describe('validateContentAgainstPolicy', () => {
    it('should validate content and return passed result when no violations', async () => {
      const mockEvaluator: RegoEvaluator = {
        evaluate: jest.fn().mockResolvedValue({
          allow: true,
          violations: [],
          warnings: [],
          suggestions: [],
          summary: {
            total_violations: 0,
            total_warnings: 0,
            total_suggestions: 0,
          },
        } as RegoPolicyResult),
        close: jest.fn(),
      };

      const content = `FROM node:20-alpine
USER node
HEALTHCHECK CMD curl --fail http://localhost:8080/health || exit 1`;

      const result = await validateContentAgainstPolicy(
        content,
        mockEvaluator,
        logger,
        'Dockerfile',
      );

      expect(mockEvaluator.evaluate).toHaveBeenCalledWith(content);
      expect(result.passed).toBe(true);
      expect(result.violations).toHaveLength(0);
      expect(result.warnings).toHaveLength(0);
      expect(result.suggestions).toHaveLength(0);
      expect(result.summary).toEqual({
        totalRules: 0,
        matchedRules: 0,
        blockingViolations: 0,
        warnings: 0,
        suggestions: 0,
      });
    });

    it('should detect blocking violations and return failed result', async () => {
      const mockEvaluator: RegoEvaluator = {
        evaluate: jest.fn().mockResolvedValue({
          allow: false,
          violations: [
            {
              rule: 'block-root-user',
              category: 'security',
              priority: 95,
              message: 'Running as root user is not allowed',
              severity: 'block',
            },
            {
              rule: 'block-secrets-in-env',
              category: 'security',
              priority: 100,
              message: 'Secrets in environment variables are not allowed',
              severity: 'block',
            },
          ],
          warnings: [],
          suggestions: [],
          summary: {
            total_violations: 2,
            total_warnings: 0,
            total_suggestions: 0,
          },
        } as RegoPolicyResult),
        close: jest.fn(),
      };

      const content = 'FROM node:20-alpine\nUSER root\nENV PASSWORD=secret123';

      const result = await validateContentAgainstPolicy(
        content,
        mockEvaluator,
        logger,
        'Dockerfile',
      );

      expect(result.passed).toBe(false);
      expect(result.violations).toHaveLength(2);
      expect(result.violations[0]).toEqual({
        ruleId: 'block-root-user',
        category: 'security',
        priority: 95,
        message: 'Running as root user is not allowed',
        severity: 'block',
      });
      expect(result.violations[1]).toEqual({
        ruleId: 'block-secrets-in-env',
        category: 'security',
        priority: 100,
        message: 'Secrets in environment variables are not allowed',
        severity: 'block',
      });
      expect(result.summary.blockingViolations).toBe(2);
    });

    it('should detect warnings without blocking', async () => {
      const mockEvaluator: RegoEvaluator = {
        evaluate: jest.fn().mockResolvedValue({
          allow: true,
          violations: [],
          warnings: [
            {
              rule: 'require-healthcheck',
              category: 'quality',
              priority: 75,
              message: 'HEALTHCHECK directive is recommended',
              severity: 'warn',
            },
          ],
          suggestions: [],
          summary: {
            total_violations: 0,
            total_warnings: 1,
            total_suggestions: 0,
          },
        } as RegoPolicyResult),
        close: jest.fn(),
      };

      const content = 'FROM node:20-alpine\nUSER node\nCMD ["node", "app.js"]';

      const result = await validateContentAgainstPolicy(
        content,
        mockEvaluator,
        logger,
        'Dockerfile',
      );

      expect(result.passed).toBe(true);
      expect(result.violations).toHaveLength(0);
      expect(result.warnings).toHaveLength(1);
      expect(result.warnings[0]).toEqual({
        ruleId: 'require-healthcheck',
        category: 'quality',
        priority: 75,
        message: 'HEALTHCHECK directive is recommended',
        severity: 'warn',
      });
      expect(result.summary.warnings).toBe(1);
    });

    it('should detect suggestions', async () => {
      const mockEvaluator: RegoEvaluator = {
        evaluate: jest.fn().mockResolvedValue({
          allow: true,
          violations: [],
          warnings: [],
          suggestions: [
            {
              rule: 'use-specific-tags',
              category: 'best-practice',
              priority: 50,
              message: 'Consider using specific image tags',
              severity: 'suggest',
            },
          ],
          summary: {
            total_violations: 0,
            total_warnings: 0,
            total_suggestions: 1,
          },
        } as RegoPolicyResult),
        close: jest.fn(),
      };

      const content = 'FROM node:latest\nUSER node';

      const result = await validateContentAgainstPolicy(
        content,
        mockEvaluator,
        logger,
        'Dockerfile',
      );

      expect(result.passed).toBe(true);
      expect(result.violations).toHaveLength(0);
      expect(result.warnings).toHaveLength(0);
      expect(result.suggestions).toHaveLength(1);
      expect(result.suggestions[0].ruleId).toBe('use-specific-tags');
      expect(result.summary.suggestions).toBe(1);
    });

    it('should handle mixed violations, warnings, and suggestions', async () => {
      const mockEvaluator: RegoEvaluator = {
        evaluate: jest.fn().mockResolvedValue({
          allow: false,
          violations: [
            {
              rule: 'block-root-user',
              category: 'security',
              priority: 95,
              message: 'Running as root user is not allowed',
              severity: 'block',
            },
          ],
          warnings: [
            {
              rule: 'require-healthcheck',
              category: 'quality',
              priority: 75,
              message: 'HEALTHCHECK directive is recommended',
              severity: 'warn',
            },
          ],
          suggestions: [
            {
              rule: 'use-specific-tags',
              category: 'best-practice',
              priority: 50,
              message: 'Consider using specific image tags',
              severity: 'suggest',
            },
          ],
          summary: {
            total_violations: 1,
            total_warnings: 1,
            total_suggestions: 1,
          },
        } as RegoPolicyResult),
        close: jest.fn(),
      };

      const content = 'FROM node:latest\nUSER root';

      const result = await validateContentAgainstPolicy(
        content,
        mockEvaluator,
        logger,
        'Dockerfile',
      );

      expect(result.passed).toBe(false);
      expect(result.violations).toHaveLength(1);
      expect(result.warnings).toHaveLength(1);
      expect(result.suggestions).toHaveLength(1);
      expect(result.summary).toEqual({
        totalRules: 3,
        matchedRules: 3,
        blockingViolations: 1,
        warnings: 1,
        suggestions: 1,
      });
    });

    it('should use default content type if not provided', async () => {
      const mockEvaluator: RegoEvaluator = {
        evaluate: jest.fn().mockResolvedValue({
          allow: true,
          violations: [],
          warnings: [],
          suggestions: [],
          summary: {
            total_violations: 0,
            total_warnings: 0,
            total_suggestions: 0,
          },
        } as RegoPolicyResult),
        close: jest.fn(),
      };

      const content = 'test content';

      const result = await validateContentAgainstPolicy(content, mockEvaluator, logger);

      expect(result).toBeDefined();
      expect(mockEvaluator.evaluate).toHaveBeenCalledWith(content);
    });

    it('should validate Kubernetes manifests', async () => {
      const mockEvaluator: RegoEvaluator = {
        evaluate: jest.fn().mockResolvedValue({
          allow: false,
          violations: [
            {
              rule: 'block-privileged',
              category: 'security',
              priority: 95,
              message: 'Privileged containers are not allowed',
              severity: 'block',
            },
          ],
          warnings: [],
          suggestions: [],
          summary: {
            total_violations: 1,
            total_warnings: 0,
            total_suggestions: 0,
          },
        } as RegoPolicyResult),
        close: jest.fn(),
      };

      const k8sManifest = `
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: app
    securityContext:
      privileged: true
`;

      const result = await validateContentAgainstPolicy(
        k8sManifest,
        mockEvaluator,
        logger,
        'K8s manifest',
      );

      expect(result.passed).toBe(false);
      expect(result.violations).toHaveLength(1);
      expect(result.violations[0].ruleId).toBe('block-privileged');
    });
  });
});
