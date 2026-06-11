/**
 * Tests for natural language formatters
 */

import { describe, it, expect } from '@jest/globals';
import {
  formatScanImageNarrative,
  formatDockerfilePlanNarrative,
  formatBuildImageNarrative,
  formatAnalyzeRepoNarrative,
  formatGithubWorkflowNarrative,
} from '@/mcp/formatters/natural-language-formatters';
import type { ScanImageResult } from '@/tools/scan-image/tool';
import type { DockerfilePlan } from '@/tools/generate-dockerfile/schema';
import type { BuildImageResult } from '@/tools/build-image-context/schema';
import type { RepositoryAnalysis } from '@/tools/analyze-repo/schema';
import type { GithubWorkflowPlan } from '@/tools/generate-github-workflow/schema';

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

    it('should omit next steps when chainHintsMode is disabled', () => {
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
        remediationGuidance: [],
      };

      const narrative = formatScanImageNarrative(result, 'disabled');

      expect(narrative).toContain('❌ Security Scan FAILED');
      expect(narrative).toContain('🔴 Critical: 2');
      expect(narrative).not.toContain('Next Steps:');
      expect(narrative).not.toContain('Review and address critical/high vulnerabilities');
    });

    it('should display fixable vulnerabilities with upgrade paths', () => {
      const result: ScanImageResult = {
        success: true,
        scanner: 'osv',
        vulnerabilities: {
          critical: 2,
          high: 3,
          medium: 5,
          low: 10,
          negligible: 0,
          unknown: 0,
          total: 20,
        },
        vulnerabilityDetails: [
          {
            id: 'CVE-2023-1234',
            severity: 'CRITICAL',
            package: 'openssl',
            version: '1.1.1',
            description: 'Critical vulnerability',
            fixedVersion: '1.1.1t',
          },
          {
            id: 'CVE-2023-5678',
            severity: 'HIGH',
            package: 'libssl',
            version: '3.0.0',
            description: 'High severity vulnerability',
            fixedVersion: '3.0.8',
          },
          {
            id: 'CVE-2023-9999',
            severity: 'MEDIUM',
            package: 'curl',
            version: '7.68.0',
            description: 'Medium severity vulnerability',
            fixedVersion: '7.88.0',
          },
          {
            id: 'CVE-2023-8888',
            severity: 'LOW',
            package: 'zlib',
            version: '1.2.11',
            description: 'Low severity without fix',
            // No fixedVersion
          },
        ],
        scanTime: '2025-01-22T10:00:00Z',
        passed: false,
      };

      const narrative = formatScanImageNarrative(result);

      expect(narrative).toContain('❌ Security Scan FAILED');
      expect(narrative).toContain('**Fixable Vulnerabilities:** (3 of 20)');
      expect(narrative).toContain('[CRITICAL] openssl: 1.1.1 → 1.1.1t');
      expect(narrative).toContain('ID: CVE-2023-1234');
      expect(narrative).toContain('[HIGH] libssl: 3.0.0 → 3.0.8');
      expect(narrative).toContain('[MEDIUM] curl: 7.68.0 → 7.88.0');
      expect(narrative).not.toContain('zlib');
    });

    it('should not show fixable vulnerabilities section when none have fixes', () => {
      const result: ScanImageResult = {
        success: true,
        scanner: 'osv',
        vulnerabilities: {
          critical: 2,
          high: 3,
          medium: 0,
          low: 0,
          negligible: 0,
          unknown: 0,
          total: 5,
        },
        vulnerabilityDetails: [
          {
            id: 'CVE-2023-1234',
            severity: 'CRITICAL',
            package: 'openssl',
            version: '1.1.1',
            description: 'Critical vulnerability without fix',
            // No fixedVersion
          },
          {
            id: 'CVE-2023-5678',
            severity: 'HIGH',
            package: 'libssl',
            version: '3.0.0',
            description: 'High severity vulnerability without fix',
            // No fixedVersion
          },
        ],
        scanTime: '2025-01-22T10:00:00Z',
        passed: false,
      };

      const narrative = formatScanImageNarrative(result);

      expect(narrative).toContain('❌ Security Scan FAILED');
      expect(narrative).not.toContain('**Fixable Vulnerabilities:**');
    });

    it('should display recommendedActions before detailed vulnerabilities', () => {
      const result: ScanImageResult = {
        success: true,
        scanner: 'osv',
        vulnerabilities: {
          critical: 1,
          high: 1,
          medium: 0,
          low: 0,
          negligible: 0,
          unknown: 0,
          total: 2,
        },
        recommendedActions: [
          {
            type: 'UPGRADE_PACKAGE',
            action: 'Upgrade openssl',
            current: 'openssl: 1.1.1',
            recommended: 'openssl: 1.1.1t',
            package: 'openssl',
            vulnerabilitiesFixed: 2,
            severityCounts: { critical: 1, high: 1, medium: 0, low: 0, negligible: 0, unknown: 0 },
            vulnerabilityIds: ['CVE-2023-1', 'CVE-2023-2'],
          },
        ],
        vulnerabilityDetails: [
          {
            id: 'CVE-2023-1',
            severity: 'CRITICAL',
            package: 'openssl',
            version: '1.1.1',
            description: 'Critical',
            fixedVersion: '1.1.1t',
          },
          {
            id: 'CVE-2023-2',
            severity: 'HIGH',
            package: 'openssl',
            version: '1.1.1',
            description: 'High',
            fixedVersion: '1.1.1t',
          },
        ],
        scanTime: '2025-01-22T10:00:00Z',
        passed: false,
      };

      const narrative = formatScanImageNarrative(result);

      expect(narrative).toContain('**Recommended Actions:** (1 action fixes 2 vulnerabilities)');
      expect(narrative).toContain('1. Upgrade openssl - Fixes 2 (1 critical)');
      expect(narrative).toContain('openssl: 1.1.1');
      expect(narrative).toContain('→ openssl: 1.1.1t');

      const actionsIndex = narrative.indexOf('**Recommended Actions:**');
      const fixableIndex = narrative.indexOf('**Fixable Vulnerabilities:**');
      expect(actionsIndex).toBeLessThan(fixableIndex);
    });
  });

  describe('formatDockerfilePlanNarrative', () => {
    it('should format complete Dockerfile plan', () => {
      const plan: DockerfilePlan = {
        nextAction: {
          action: 'create-files',
          instruction:
            'Create a new Dockerfile at ./Dockerfile using the base images, security considerations, optimizations, and best practices from recommendations.',
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
        summary:
          '🔨 ACTION REQUIRED: Create Dockerfile\nPath: ./Dockerfile\nLanguage: javascript 18.0.0 (Express)\nStrategy: Multi-stage build\n✅ Ready to create Dockerfile based on recommendations.',
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
          instruction:
            'Update the existing Dockerfile at ./Dockerfile by applying the enhancement recommendations.',
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
        summary:
          '🔨 ACTION REQUIRED: Update Dockerfile\nPath: ./Dockerfile\nLanguage: python 3.11\n✅ Ready to update Dockerfile with enhancements.',
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
          instruction:
            'Create a new Dockerfile at ./Dockerfile using the base images and recommendations.',
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
        summary:
          '🔨 ACTION REQUIRED: Create Dockerfile\nPath: ./Dockerfile\nLanguage: java\n✅ Ready to create Dockerfile based on recommendations.',
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

    it('should omit next steps when chainHintsMode is disabled', () => {
      const plan: DockerfilePlan = {
        nextAction: {
          action: 'create-files',
          instruction:
            'Create a new Dockerfile at ./Dockerfile using the base images and recommendations.',
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
          securityConsiderations: [],
          optimizations: [],
          bestPractices: [],
        },
        confidence: 0.9,
        summary: '🔨 ACTION REQUIRED: Create Dockerfile',
      };

      const narrative = formatDockerfilePlanNarrative(plan, 'disabled');

      expect(narrative).toContain('✨ CREATE DOCKERFILE');
      expect(narrative).toContain('node:18-alpine');
      expect(narrative).not.toContain('Next Steps:');
      expect(narrative).not.toContain('Build image with build-image-context tool');
    });
  });

  describe('formatBuildImageNarrative', () => {
    it('should format successful build with all details', () => {
      const result: BuildImageResult = {
        summary: 'Build context ready for myapp with 3 tags',
        context: {
          buildContextPath: '/app',
          dockerfilePath: '/app/Dockerfile',
          dockerfileRelative: 'Dockerfile',
          hasDockerignore: true,
        },
        securityAnalysis: {
          warnings: [
            {
              id: 'ROOT_USER',
              severity: 'medium',
              message: 'Running as root user',
              line: 15,
              remediation: 'Add USER directive',
            },
          ],
          riskLevel: 'medium',
          recommendations: ['Add non-root user'],
        },
        buildConfig: {
          finalTags: ['myapp:latest', 'myapp:1.0.0', 'myapp:production'],
          buildArgs: {},
          platform: 'linux/amd64',
        },
        buildKitAnalysis: {
          features: {
            cacheMount: false,
            secretMount: false,
            sshMount: false,
            multiStage: true,
            stageCount: 2,
            copyFrom: true,
            heredoc: false,
          },
          recommended: true,
          recommendations: ['Use BuildKit for multi-stage builds'],
        },
        dockerfileAnalysis: {
          baseImages: ['node:18-alpine'],
          exposedPorts: [3000],
          finalUser: undefined,
          hasHealthcheck: true,
          layerCount: 12,
        },
        nextAction: {
          action: 'execute-build',
          preChecks: ['Verify Docker daemon is running'],
          buildCommand: {
            command: 'docker build -t myapp:latest -t myapp:1.0.0 -t myapp:production .',
            parts: {
              executable: 'docker',
              subcommand: 'build',
              flags: ['-t', 'myapp:latest'],
              context: '.',
            },
            environment: { DOCKER_BUILDKIT: '1' },
          },
          postBuildSteps: ['Scan for vulnerabilities'],
        },
      };

      const narrative = formatBuildImageNarrative(result);

      expect(narrative).toContain('📦 Build Context Ready');
      expect(narrative).toContain('**Tags:** myapp:latest, myapp:1.0.0, myapp:production');
      expect(narrative).toContain('**Platform:** linux/amd64');
      expect(narrative).toContain('Estimated Layers: 12');
      expect(narrative).toContain('Next Steps:');
      expect(narrative).toContain('Scan built image for vulnerabilities');
    });

    it('should handle minimal build result', () => {
      const result: BuildImageResult = {
        summary: 'Minimal build context ready',
        context: {
          buildContextPath: '/app',
          dockerfilePath: '/app/Dockerfile',
          dockerfileRelative: 'Dockerfile',
          hasDockerignore: false,
        },
        securityAnalysis: {
          warnings: [],
          riskLevel: 'low',
          recommendations: [],
        },
        buildConfig: {
          finalTags: [],
          buildArgs: {},
          platform: 'linux/amd64',
        },
        buildKitAnalysis: {
          features: {
            cacheMount: false,
            secretMount: false,
            sshMount: false,
            multiStage: false,
            stageCount: 1,
            copyFrom: false,
            heredoc: false,
          },
          recommended: false,
          recommendations: [],
        },
        dockerfileAnalysis: {
          baseImages: ['alpine:latest'],
          exposedPorts: [],
          hasHealthcheck: false,
          layerCount: 3,
        },
        nextAction: {
          action: 'execute-build',
          preChecks: [],
          buildCommand: {
            command: 'docker build .',
            parts: {
              executable: 'docker',
              subcommand: 'build',
              flags: [],
              context: '.',
            },
            environment: {},
          },
          postBuildSteps: [],
        },
      };

      const narrative = formatBuildImageNarrative(result);

      expect(narrative).toContain('📦 Build Context Ready');
      expect(narrative).toContain('**Summary:** Minimal build context ready');
      expect(narrative).not.toContain('**Tags:**');
      expect(narrative).toContain('Estimated Layers: 3');
    });

    it('should omit next steps when chainHintsMode is disabled', () => {
      const result: BuildImageResult = {
        summary: 'Build context ready',
        context: {
          buildContextPath: '/app',
          dockerfilePath: '/app/Dockerfile',
          dockerfileRelative: 'Dockerfile',
          hasDockerignore: true,
        },
        securityAnalysis: {
          warnings: [],
          riskLevel: 'low',
          recommendations: [],
        },
        buildConfig: {
          finalTags: ['myapp:latest'],
          buildArgs: {},
          platform: 'linux/amd64',
        },
        buildKitAnalysis: {
          features: {
            cacheMount: false,
            secretMount: false,
            sshMount: false,
            multiStage: false,
            stageCount: 1,
            copyFrom: false,
            heredoc: false,
          },
          recommended: false,
          recommendations: [],
        },
        dockerfileAnalysis: {
          baseImages: ['node:18-alpine'],
          exposedPorts: [3000],
          hasHealthcheck: false,
          layerCount: 8,
        },
        nextAction: {
          action: 'execute-build',
          preChecks: ['Verify Docker daemon'],
          buildCommand: {
            command: 'docker build -t myapp:latest .',
            parts: {
              executable: 'docker',
              subcommand: 'build',
              flags: ['-t', 'myapp:latest'],
              context: '.',
            },
            environment: {},
          },
          postBuildSteps: [],
        },
      };

      const narrative = formatBuildImageNarrative(result, 'disabled');

      expect(narrative).toContain('📦 Build Context Ready');
      expect(narrative).toContain('**Tags:** myapp:latest');
      expect(narrative).not.toContain('Next Steps:');
      expect(narrative).not.toContain('Scan built image for vulnerabilities');
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

    it('should omit next steps when chainHintsMode is disabled', () => {
      const result: RepositoryAnalysis = {
        modules: [
          {
            name: 'main',
            modulePath: '/app',
            language: 'python',
            frameworks: [{ name: 'Django', version: '4.2.0' }],
            ports: [8000],
          },
        ],
        isMonorepo: false,
        analyzedPath: '/app',
      };

      const narrative = formatAnalyzeRepoNarrative(result, 'disabled');

      expect(narrative).toContain('✅ Repository Analysis Complete');
      expect(narrative).toContain('**Path:** /app');
      expect(narrative).not.toContain('Next Steps:');
      expect(narrative).not.toContain('Use generate-dockerfile');
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

  describe('formatGithubWorkflowNarrative', () => {
    const makePlan = (): GithubWorkflowPlan => ({
      nextAction: {
        action: 'create-files',
        instruction:
          'Create a new GitHub Actions workflow at .github/workflows/deploy.yml. Use az acr build ONLY. Do NOT add an environment: key to any job. Use literal job keys buildImage and deploy.',
        files: [
          {
            path: '.github/workflows/deploy.yml',
            purpose: 'GitHub Actions CI/CD workflow',
          },
        ],
      },
      workflowJobs: [
        {
          name: 'buildImage',
          runsOn: 'ubuntu-latest',
          steps: ['actions/checkout@v6', 'az acr build'],
        },
        {
          name: 'deploy',
          runsOn: 'ubuntu-latest',
          steps: ['Azure/k8s-deploy@v6'],
        },
      ],
      secretsRequired: ['AZURE_CLIENT_ID', 'AZURE_TENANT_ID', 'AZURE_SUBSCRIPTION_ID'],
      variablesRequired: [],
      summary: 'Knowledge snippets applied: 13',
      attributionLabels: {
        annotations: {
          'com.azure.containerizationassist/workflow-generator': 'generate-github-workflow',
        },
      },
    });

    it('surfaces the full nextAction.instruction (not just the summary)', () => {
      const narrative = formatGithubWorkflowNarrative(makePlan());

      expect(narrative).toContain('az acr build ONLY');
      expect(narrative).toContain('Do NOT add an environment:');
      expect(narrative).toContain('buildImage and deploy');
    });

    it('lists the workflow file, jobs, and required secrets', () => {
      const narrative = formatGithubWorkflowNarrative(makePlan());

      expect(narrative).toContain('.github/workflows/deploy.yml');
      expect(narrative).toContain('buildImage');
      expect(narrative).toContain('deploy');
      expect(narrative).toContain('AZURE_CLIENT_ID');
      expect(narrative).toContain('AZURE_TENANT_ID');
      expect(narrative).toContain('AZURE_SUBSCRIPTION_ID');
    });

    it('includes next steps when chain hints are enabled', () => {
      const narrative = formatGithubWorkflowNarrative(makePlan());
      expect(narrative).toContain('**Next Steps:**');
    });
  });
});
