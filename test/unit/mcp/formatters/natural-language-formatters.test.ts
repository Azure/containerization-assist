/**
 * Tests for natural language formatters
 */

import { describe, it, expect } from '@jest/globals';
import {
  formatScanImageNarrative,
  formatDockerfilePlanNarrative,
  formatBuildImageNarrative,
  formatAnalyzeRepoNarrative,
} from '@/mcp/formatters/natural-language-formatters';
import type { ScanImageResult } from '@/tools/scan-image/tool';
import type { DockerfilePlan } from '@/tools/generate-dockerfile/schema';
import type { BuildImageResult } from '@/tools/build-image/tool';
import type { RepositoryAnalysis } from '@/tools/analyze-repo/schema';

describe('natural-language-formatters', () => {
  describe('formatScanImageNarrative', () => {
    it('should format successful scan with no vulnerabilities', () => {
      const result: ScanImageResult = {
        success: true,
        vulnerabilities: {
          critical: 0,
          high: 0,
          medium: 0,
          low: 5,
          negligible: 10,
          unknown: 0,
          total: 15,
        },
        scanTime: '2025-01-22T10:00:00Z',
        passed: true,
        remediationGuidance: [],
      };

      const narrative = formatScanImageNarrative(result);

      expect(narrative).toContain('✅ Security Scan PASSED');
      expect(narrative).toContain('Vulnerabilities:');
      expect(narrative).toContain('Next Steps:');
      expect(narrative).toContain('Proceed with image tagging');
    });

    it('should format failed scan with critical vulnerabilities', () => {
      const result: ScanImageResult = {
        success: true,
        vulnerabilities: {
          critical: 2,
          high: 5,
          medium: 12,
          low: 34,
          negligible: 89,
          unknown: 0,
          total: 142,
        },
        scanTime: '2025-01-22T10:00:00Z',
        passed: false,
        remediationGuidance: [
          {
            vulnerability: 'CVE-2023-1234',
            recommendation: 'Upgrade base image to latest version',
            severity: 'critical',
            example: 'FROM node:18-alpine',
          },
        ],
      };

      const narrative = formatScanImageNarrative(result);

      expect(narrative).toContain('❌ Security Scan FAILED');
      expect(narrative).toContain('🔴 Critical: 2');
      expect(narrative).toContain('🟠 High: 5');
      expect(narrative).toContain('🟡 Medium: 12');
      expect(narrative).toContain('Remediation Recommendations:');
      expect(narrative).toContain('Upgrade base image');
      expect(narrative).toContain('Review and address critical/high vulnerabilities');
    });

    it('should truncate remediation guidance after 5 items', () => {
      const remediations = Array.from({ length: 8 }, (_, i) => ({
        vulnerability: `CVE-2023-${i}`,
        recommendation: `Fix vulnerability ${i}`,
        severity: 'high' as const,
      }));

      const result: ScanImageResult = {
        success: true,
        vulnerabilities: {
          critical: 0,
          high: 8,
          medium: 0,
          low: 0,
          negligible: 0,
          unknown: 0,
          total: 8,
        },
        scanTime: '2025-01-22T10:00:00Z',
        passed: false,
        remediationGuidance: remediations,
      };

      const narrative = formatScanImageNarrative(result);

      expect(narrative).toContain('... and 3 more recommendations');
    });
  });

  describe('formatDockerfilePlanNarrative', () => {
    it('should format complete Dockerfile plan', () => {
      const plan: DockerfilePlan = {
        nextAction: {
          action: 'create-files',
          instruction: 'Create a new Dockerfile at ./Dockerfile using the base images, security considerations, optimizations, and best practices from recommendations.',
          files: [
            {
              path: './Dockerfile',
              purpose: 'Container build configuration',
            },
          ],
        },
        repositoryInfo: {
          name: 'my-app',
          language: 'javascript',
          languageVersion: '18.0.0',
          frameworks: [{ name: 'Express', version: '4.18.0' }],
        },
        recommendations: {
          baseImages: [
            {
              image: 'node:18-alpine',
              reason: 'Lightweight Alpine-based image',
              category: 'size',
              matchScore: 95,
              size: '50MB',
            },
          ],
          buildStrategy: {
            multistage: true,
            reason: 'Optimized for production deployment',
          },
          securityConsiderations: [
            {
              id: 'sec-1',
              category: 'security',
              recommendation: 'Run as non-root user',
              severity: 'high',
              matchScore: 90,
            },
          ],
          optimizations: [
            {
              id: 'opt-1',
              category: 'optimization',
              recommendation: 'Use .dockerignore to exclude unnecessary files',
              matchScore: 85,
            },
          ],
          bestPractices: [],
        },
        confidence: 0.9,
        summary: '🔨 ACTION REQUIRED: Create Dockerfile\nPath: ./Dockerfile\nLanguage: javascript 18.0.0 (Express)\nStrategy: Multi-stage build\n✅ Ready to create Dockerfile based on recommendations.',
      };

      const narrative = formatDockerfilePlanNarrative(plan);

      expect(narrative).toContain('✨ CREATE DOCKERFILE');
      expect(narrative).toContain('**Action:**');
      expect(narrative).toContain('**Files:**');
      expect(narrative).toContain('./Dockerfile');
      expect(narrative).toContain('**Project:** my-app');
      expect(narrative).toContain('**Language:** javascript (Express)');
      expect(narrative).toContain('**Strategy:** Multi-stage build');
      expect(narrative).toContain('node:18-alpine');
      expect(narrative).toContain('**Security Considerations:**');
      expect(narrative).toContain('**Optimizations:**');
      expect(narrative).toContain('Next Steps:');
    });

    it('should handle existing Dockerfile analysis', () => {
      const plan: DockerfilePlan = {
        nextAction: {
          action: 'update-files',
          instruction: 'Update the existing Dockerfile at ./Dockerfile by applying the enhancement recommendations.',
          files: [
            {
              path: './Dockerfile',
              purpose: 'Container build configuration (enhancement)',
            },
          ],
        },
        repositoryInfo: {
          name: 'my-app',
          language: 'python',
          languageVersion: '3.11',
        },
        recommendations: {
          baseImages: [],
          buildStrategy: {
            multistage: false,
            reason: 'Single-stage build sufficient for interpreted languages',
          },
          securityConsiderations: [],
          optimizations: [],
          bestPractices: [],
        },
        confidence: 0.85,
        summary: '🔨 ACTION REQUIRED: Update Dockerfile\nPath: ./Dockerfile\nLanguage: python 3.11\n✅ Ready to update Dockerfile with enhancements.',
        existingDockerfile: {
          path: '/app/Dockerfile',
          content: 'FROM python:3.11\nWORKDIR /app',
          analysis: {
            complexity: 'simple',
            securityPosture: 'needs-improvement',
            isMultistage: false,
            baseImages: ['python:3.11'],
            hasHealthCheck: false,
            hasNonRootUser: false,
            instructionCount: 2,
          },
          guidance: {
            strategy: 'moderate-refactor',
            preserve: ['Base image selection', 'Working directory'],
            improve: ['Add non-root user', 'Add healthcheck'],
            addMissing: [],
          },
        },
      };

      const narrative = formatDockerfilePlanNarrative(plan);

      expect(narrative).toContain('🔧 UPDATE DOCKERFILE');
      expect(narrative).toContain('**Action:**');
      expect(narrative).toContain('**Existing Dockerfile Analysis:**');
      expect(narrative).toContain('Path: /app/Dockerfile');
      expect(narrative).toContain('Complexity: simple');
      expect(narrative).toContain('Security: needs-improvement');
      expect(narrative).toContain('Enhancement Strategy: moderate-refactor');
      expect(narrative).toContain('**Preserve:**');
      expect(narrative).toContain('**Improve:**');
    });

    it('should display policy validation results', () => {
      const plan: DockerfilePlan = {
        nextAction: {
          action: 'create-files',
          instruction: 'Create a new Dockerfile at ./Dockerfile using the base images and recommendations.',
          files: [
            {
              path: './Dockerfile',
              purpose: 'Container build configuration',
            },
          ],
        },
        repositoryInfo: {
          name: 'my-app',
          language: 'java',
        },
        recommendations: {
          baseImages: [],
          buildStrategy: {
            multistage: true,
            reason: 'Multi-stage build recommended for compiled languages',
          },
          securityConsiderations: [],
          optimizations: [],
          bestPractices: [],
        },
        confidence: 0.8,
        summary: '🔨 ACTION REQUIRED: Create Dockerfile\nPath: ./Dockerfile\nLanguage: java\n✅ Ready to create Dockerfile based on recommendations.',
        policyValidation: {
          passed: false,
          violations: [
            {
              ruleId: 'require-health-check',
              message: 'Dockerfile must include HEALTHCHECK instruction',
              severity: 'blocking',
              line: 0,
            },
          ],
          warnings: [
            {
              ruleId: 'prefer-specific-versions',
              message: 'Consider using specific version tags',
              severity: 'warning',
              line: 0,
            },
          ],
          suggestions: [],
        },
      };

      const narrative = formatDockerfilePlanNarrative(plan);

      expect(narrative).toContain('**Policy Validation:** ❌ Failed');
      expect(narrative).toContain('Violations: 1');
      expect(narrative).toContain('Warnings: 1');
    });
  });


  describe('formatBuildImageNarrative', () => {
    it('should format successful build with all details', () => {
      const result: BuildImageResult = {
        success: true,
        imageId: 'sha256:abc123def456',
        requestedTags: ['myapp:latest', 'myapp:1.0.0', 'myapp:production'],
        createdTags: ['myapp:latest', 'myapp:1.0.0', 'myapp:production'],
        size: 245000000,
        buildTime: 45000,
        layers: 12,
        logs: [],
      };

      const narrative = formatBuildImageNarrative(result);

      expect(narrative).toContain('✅ Image Built Successfully');
      expect(narrative).toContain('**Image:** sha256:abc123def456');
      expect(narrative).toContain('**Tags Created:** myapp:latest, myapp:1.0.0, myapp:production');
      expect(narrative).toContain('**Size:** 234MB'); // 245000000 bytes = 234MB
      expect(narrative).toContain('**Build Time:** 45s');
      expect(narrative).toContain('**Layers:** 12');
      expect(narrative).toContain('Next Steps:');
      expect(narrative).toContain('Scan image for vulnerabilities');
    });

    it('should handle minimal build result', () => {
      const result: BuildImageResult = {
        success: true,
        imageId: 'sha256:minimal',
        requestedTags: [],
        createdTags: [],
        size: 100000000,
        buildTime: 30000,
        logs: [],
      };

      const narrative = formatBuildImageNarrative(result);

      expect(narrative).toContain('✅ Image Built Successfully');
      expect(narrative).toContain('**Image:** sha256:minimal');
      expect(narrative).not.toContain('**Tags Created:**');
      expect(narrative).not.toContain('**Layers:**');
    });
  });

  describe('formatAnalyzeRepoNarrative', () => {
    it('should format single-module repository', () => {
      const result: RepositoryAnalysis = {
        modules: [
          {
            name: 'main',
            modulePath: '/app',
            language: 'python',
            frameworks: [
              { name: 'Django', version: '4.2.0' },
              { name: 'DRF', version: '3.14.0' },
            ],
            buildSystems: [
              {
                type: 'pip',
                languageVersion: '3.11',
              },
            ],
            entryPoint: 'manage.py',
            ports: [8000],
          },
        ],
        isMonorepo: false,
        analyzedPath: '/app',
      };

      const narrative = formatAnalyzeRepoNarrative(result);

      expect(narrative).toContain('✅ Repository Analysis Complete');
      expect(narrative).toContain('**Path:** /app');
      expect(narrative).toContain('**Type:** Single-module project');
      expect(narrative).toContain('**Modules Found:** 1');
      expect(narrative).toContain('1. **main**');
      expect(narrative).toContain('Language: python');
      expect(narrative).toContain('Frameworks: Django, DRF');
      expect(narrative).toContain('Build System: pip (python 3.11)');
      expect(narrative).toContain('Entry Point: manage.py');
      expect(narrative).toContain('Ports: 8000');
      expect(narrative).toContain('Use generate-dockerfile to create container configuration');
    });

    it('should format monorepo with multiple modules', () => {
      const result: RepositoryAnalysis = {
        modules: [
          {
            name: 'frontend',
            modulePath: '/app/frontend',
            language: 'typescript',
            frameworks: [{ name: 'React', version: '18.2.0' }],
            ports: [3000],
          },
          {
            name: 'backend',
            modulePath: '/app/backend',
            language: 'go',
            ports: [8080],
          },
        ],
        isMonorepo: true,
        analyzedPath: '/app',
      };

      const narrative = formatAnalyzeRepoNarrative(result);

      expect(narrative).toContain('**Type:** Monorepo');
      expect(narrative).toContain('**Modules Found:** 2');
      expect(narrative).toContain('1. **frontend**');
      expect(narrative).toContain('2. **backend**');
      expect(narrative).toContain('Consider creating separate Dockerfiles for each module');
    });

    it('should handle empty modules list', () => {
      const result: RepositoryAnalysis = {
        modules: [],
        isMonorepo: false,
        analyzedPath: '/app',
      };

      const narrative = formatAnalyzeRepoNarrative(result);

      expect(narrative).toContain('**Modules Found:** 0');
      expect(narrative).toContain('No modules detected in repository');
    });

    it('should handle undefined modules', () => {
      const result: RepositoryAnalysis = {
        isMonorepo: false,
        analyzedPath: '/app',
      };

      const narrative = formatAnalyzeRepoNarrative(result);

      expect(narrative).toContain('**Modules Found:** 0');
      expect(narrative).toContain('No modules detected in repository');
    });
  });
});
