import { applyFixes, applyAllFixes, hasFixForRule, getFixableRules } from '@/validation/dockerfile-fixer';

describe('Dockerfile Fixer', () => {
  describe('no-root-user fix', () => {
    it('should add non-root user when missing', () => {
      const input = 'FROM node:20\nCMD ["node", "app.js"]';
      const { fixed, applied } = applyFixes(input, ['no-root-user']);

      expect(fixed).toContain('USER appuser');
      expect(fixed).toContain('adduser');
      expect(applied).toContain('no-root-user');
    });

    it('should not add user if already present', () => {
      const input = 'FROM node:20\nUSER node\nCMD ["node", "app.js"]';
      const { fixed, applied } = applyFixes(input, ['no-root-user']);

      expect(fixed).toBe(input);
      expect(applied).toHaveLength(0);
    });

    it('should place USER before CMD/ENTRYPOINT', () => {
      const input = 'FROM alpine:3.19\nRUN apk add --no-cache nodejs\nCMD ["node", "app.js"]';
      const { fixed } = applyFixes(input, ['no-root-user']);

      const lines = fixed.split('\n');
      const userIndex = lines.findIndex((l) => l.includes('USER'));
      const cmdIndex = lines.findIndex((l) => l.includes('CMD'));

      expect(userIndex).toBeGreaterThan(0);
      expect(userIndex).toBeLessThan(cmdIndex);
    });

    it('should use appropriate user creation command for Alpine', () => {
      const input = 'FROM alpine:3.19\nCMD ["sh"]';
      const { fixed } = applyFixes(input, ['no-root-user']);

      expect(fixed).toContain('adduser -D -u 1001 appuser');
    });

    it('should use appropriate user creation command for Ubuntu/Debian', () => {
      const input = 'FROM ubuntu:24.04\nCMD ["bash"]';
      const { fixed } = applyFixes(input, ['no-root-user']);

      expect(fixed).toContain('useradd -m -u 1001');
    });
  });

  describe('specific-base-image fix', () => {
    it('should replace :latest tags', () => {
      const input = 'FROM node:latest';
      const { fixed, applied } = applyFixes(input, ['specific-base-image']);

      expect(fixed).toContain('FROM node:20-alpine');
      expect(applied).toContain('specific-base-image');
    });

    it('should handle multi-stage builds', () => {
      const input = 'FROM node:latest AS builder\nFROM nginx:latest';
      const { fixed } = applyFixes(input, ['specific-base-image']);

      expect(fixed).toContain('FROM node:20-alpine AS builder');
      expect(fixed).toContain('FROM nginx:1.25-alpine');
    });

    it('should not modify already pinned versions', () => {
      const input = 'FROM node:20.11.0-alpine';
      const { fixed, applied } = applyFixes(input, ['specific-base-image']);

      expect(fixed).toBe(input);
      expect(applied).toHaveLength(0);
    });

    it('should handle various base images', () => {
      const testCases = [
        { input: 'FROM python:latest', expected: 'python:3.12-slim' },
        { input: 'FROM ubuntu:latest', expected: 'ubuntu:24.04' },
        { input: 'FROM alpine:latest', expected: 'alpine:3.19' },
        { input: 'FROM redis:latest', expected: 'redis:7-alpine' },
      ];

      testCases.forEach(({ input, expected }) => {
        const { fixed } = applyFixes(input, ['specific-base-image']);
        expect(fixed).toContain(expected);
      });
    });
  });

  describe('optimize-package-install fix', () => {
    describe('apt-get optimization', () => {
      it('should add --no-install-recommends if missing', () => {
        const input = 'FROM ubuntu:24.04\nRUN apt-get install curl';
        const { fixed } = applyFixes(input, ['optimize-package-install']);

        expect(fixed).toContain('--no-install-recommends');
      });

      it('should add cleanup if missing', () => {
        const input = 'FROM ubuntu:24.04\nRUN apt-get install curl';
        const { fixed } = applyFixes(input, ['optimize-package-install']);

        expect(fixed).toContain('rm -rf /var/lib/apt/lists/*');
      });

      it('should add apt-get update if missing', () => {
        const input = 'FROM ubuntu:24.04\nRUN apt-get install curl';
        const { fixed } = applyFixes(input, ['optimize-package-install']);

        expect(fixed).toContain('apt-get update');
      });

      it('should not duplicate existing optimizations', () => {
        const input =
          'FROM ubuntu:24.04\nRUN apt-get update && apt-get install --no-install-recommends curl && rm -rf /var/lib/apt/lists/*';
        const { fixed, applied } = applyFixes(input, ['optimize-package-install']);

        expect(fixed).toBe(input);
        expect(applied).toHaveLength(0);
      });
    });

    describe('apk optimization', () => {
      it('should add --no-cache to apk add', () => {
        const input = 'FROM alpine:3.19\nRUN apk add curl';
        const { fixed } = applyFixes(input, ['optimize-package-install']);

        expect(fixed).toContain('apk add --no-cache');
      });

      it('should not duplicate --no-cache', () => {
        const input = 'FROM alpine:3.19\nRUN apk add --no-cache curl';
        const { fixed, applied } = applyFixes(input, ['optimize-package-install']);

        expect(fixed).toBe(input);
        expect(applied).toHaveLength(0);
      });
    });

    describe('yum/dnf optimization', () => {
      it('should add -y flag to yum install', () => {
        const input = 'FROM centos:8\nRUN yum install curl';
        const { fixed } = applyFixes(input, ['optimize-package-install']);

        expect(fixed).toContain('yum install -y');
      });

      it('should add cleanup for yum', () => {
        const input = 'FROM centos:8\nRUN yum install curl';
        const { fixed } = applyFixes(input, ['optimize-package-install']);

        expect(fixed).toContain('yum clean all');
      });

      it('should handle dnf commands', () => {
        const input = 'FROM fedora:39\nRUN dnf install curl';
        const { fixed } = applyFixes(input, ['optimize-package-install']);

        expect(fixed).toContain('dnf install -y');
        expect(fixed).toContain('dnf clean all');
      });
    });
  });

  describe('applyAllFixes', () => {
    it('should apply all available fixes', () => {
      const input = 'FROM node:latest\nRUN apt-get install curl';
      const { fixed, applied } = applyAllFixes(input);

      expect(fixed).toContain('node:20-alpine');
      expect(fixed).toContain('--no-install-recommends');
      expect(fixed).toContain('USER appuser');
      expect(applied).toHaveLength(3);
    });
  });

  describe('helper functions', () => {
    it('hasFixForRule should return true for fixable rules', () => {
      expect(hasFixForRule('no-root-user')).toBe(true);
      expect(hasFixForRule('specific-base-image')).toBe(true);
      expect(hasFixForRule('optimize-package-install')).toBe(true);
    });

    it('hasFixForRule should return false for non-fixable rules', () => {
      expect(hasFixForRule('non-existent-rule')).toBe(false);
    });

    it('getFixableRules should return all fixable rule IDs', () => {
      const rules = getFixableRules();

      expect(rules).toContain('no-root-user');
      expect(rules).toContain('specific-base-image');
      expect(rules).toContain('optimize-package-install');
      expect(rules).toHaveLength(3);
    });
  });

  describe('error handling', () => {
    it('should return original content on parse error', () => {
      // Use empty content which should not have fixes applied
      const invalidInput = '';
      const { fixed, applied } = applyFixes(invalidInput, ['no-root-user']);

      expect(fixed).toBe(invalidInput);
      expect(applied).toHaveLength(0);
    });
  });

  describe('idempotency', () => {
    it('should be idempotent for all fixes', () => {
      const input = 'FROM node:latest\nRUN apt-get install curl';
      const { fixed: firstPass } = applyAllFixes(input);
      const { fixed: secondPass, applied: secondApplied } = applyAllFixes(firstPass);

      expect(secondPass).toBe(firstPass);
      expect(secondApplied).toHaveLength(0);
    });
  });
});
