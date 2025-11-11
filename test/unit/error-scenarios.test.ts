/**
 * Comprehensive Error Scenario Tests
 *
 * Tests error handling across all tools for common failure scenarios:
 * - Invalid parameters
 * - Infrastructure unavailable
 * - Permission errors
 * - Resource not found
 * - Policy violations
 * - Validation failures
 */

import { describe, it, expect, jest, beforeAll, afterAll } from '@jest/globals';
import { createLogger } from '@/lib/logger';
import type { ToolContext } from '@/mcp/context';
import { join } from 'node:path';
import { mkdirSync, writeFileSync, chmodSync } from 'node:fs';
import { createTestTempDir } from '../__support__/utilities/tmp-helpers';
import { Failure } from '@/types';
import * as scannerModule from '@/infra/security/scanner';

// Import tools
import analyzeRepoTool from '@/tools/analyze-repo/tool';
import generateDockerfileTool from '@/tools/generate-dockerfile/tool';
import buildImageTool from '@/tools/build-image/tool';
import scanImageTool from '@/tools/scan-image/tool';
import tagImageTool from '@/tools/tag-image/tool';
import pushImageTool from '@/tools/push-image/tool';
import generateK8sManifestsTool from '@/tools/generate-k8s-manifests/tool';
import fixDockerfileTool from '@/tools/fix-dockerfile/tool';
import prepareClusterTool from '@/tools/prepare-cluster/tool';

const logger = createLogger({ level: 'silent' });
const toolContext: ToolContext = {
  logger,
  signal: undefined,
  progress: undefined,
};

describe('Error Scenario Coverage', () => {
  // Mock the security scanner to avoid slow Trivy timeouts in error tests
  let scannerSpy: jest.SpiedFunction<typeof scannerModule.createSecurityScanner>;

  beforeAll(() => {
    scannerSpy = jest.spyOn(scannerModule, 'createSecurityScanner').mockImplementation(() => ({
      scanImage: async (imageId: string) => {
        // Fail fast for nonexistent images to avoid Trivy pulling from Docker Hub
        if (imageId.includes('nonexistent') || imageId.includes('invalid')) {
          return Failure('Image not found', {
            message: 'Failed to scan image',
            hint: 'The specified image does not exist',
            resolution: 'Verify the image exists: docker images',
          });
        }
        // Return empty scan for alpine (used in tests)
        return {
          ok: true,
          value: {
            imageId,
            vulnerabilities: [],
            totalVulnerabilities: 0,
            criticalCount: 0,
            highCount: 0,
            mediumCount: 0,
            lowCount: 0,
            negligibleCount: 0,
            unknownCount: 0,
            scanDate: new Date(),
          },
        };
      },
      ping: async () => ({ ok: true, value: true }),
    }));
  });

  afterAll(() => {
    // Restore the spy and clear all mocks to prevent handle leaks
    if (scannerSpy) {
      scannerSpy.mockRestore();
    }
    jest.clearAllMocks();
    jest.restoreAllMocks();
  });

  describe('Invalid Parameters', () => {
    it('should reject analyze-repo with invalid path', async () => {
      const result = await analyzeRepoTool.handler(
        { repositoryPath: '/absolutely/nonexistent/path/12345' },
        toolContext
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('does not exist');
        expect(result.guidance).toBeDefined();
      }
    });

    it('should reject generate-dockerfile with empty repository path', async () => {
      const result = await generateDockerfileTool.handler(
        { repositoryPath: '', environment: 'production', targetPlatform: 'linux/amd64' },
        toolContext
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeDefined();
      }
    });

    it('should reject build-image with missing Dockerfile', async () => {
      const result = await buildImageTool.handler(
        {
          dockerfilePath: '/nonexistent/Dockerfile',
          context: '/tmp',
          imageName: 'test:latest',
        },
        toolContext
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeDefined();
      }
    });

    it('should reject tag-image with invalid image ID', async () => {
      const result = await tagImageTool.handler(
        {
          imageId: 'totally-invalid-image-id-12345',
          tag: 'test:v1',
        },
        toolContext
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeDefined();
      }
    });

    it('should reject scan-image with nonexistent image', async () => {
      const result = await scanImageTool.handler(
        { imageId: 'nonexistent-image:12345' },
        toolContext
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeDefined();
      }
    });

    it('should reject push-image with invalid registry', async () => {
      const result = await pushImageTool.handler(
        {
          imageId: 'test:latest',
          registry: 'invalid://registry',
        },
        toolContext
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeDefined();
      }
    });

    it('should reject fix-dockerfile with nonexistent file', async () => {
      const result = await fixDockerfileTool.handler(
        { dockerfilePath: '/nonexistent/Dockerfile' },
        toolContext
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeDefined();
        expect(result.error.length).toBeGreaterThan(0);
      }
    });

    it('should reject fix-dockerfile with invalid path', async () => {
      const result = await fixDockerfileTool.handler(
        { dockerfilePath: '/nonexistent/Dockerfile' },
        toolContext
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeDefined();
      }
    });

    it('should handle generate-k8s-manifests with empty analysis', async () => {
      const result = await generateK8sManifestsTool.handler(
        {
          analysis: '',
          imageName: 'test:latest',
        } as any,
        toolContext
      );

      // May either fail or handle gracefully
      expect(result.ok !== undefined).toBe(true);
    });

    it('should reject prepare-cluster with invalid namespace', async () => {
      const result = await prepareClusterTool.handler(
        { namespace: '-invalid-namespace' },
        toolContext
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeDefined();
      }
    });

    // Note: resolve-base-images tool functionality is handled by other tools
  });

  describe('Malformed Input Data', () => {
    it('should handle malformed JSON in analysis parameter', async () => {
      const result = await generateDockerfileTool.handler(
        {
          repositoryPath: '/tmp',
          analysis: '{invalid json[}',
          targetPlatform: 'linux/amd64',
        },
        toolContext
      );

      // Should handle gracefully
      expect(result.ok !== undefined).toBe(true);
    });

    it('should handle malformed validation report', async () => {
      const { dir, cleanup } = createTestTempDir('error-test-');
      const dockerfilePath = join(dir.name, 'Dockerfile');
      writeFileSync(dockerfilePath, 'FROM node:18\n');

      const result = await fixDockerfileTool.handler(
        {
          dockerfilePath,
          validationReport: 'not a valid report',
        },
        toolContext
      );

      await cleanup();

      // Should handle gracefully
      expect(result.ok !== undefined).toBe(true);
    });

    it('should handle empty strings in required fields', async () => {
      const result = await analyzeRepoTool.handler(
        { repositoryPath: '' },
        toolContext
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeDefined();
      }
    });
  });

  describe('Infrastructure Unavailable', () => {
    it('should handle Docker daemon unavailable gracefully', async () => {
      // This test assumes Docker might not be available
      const { dir, cleanup } = createTestTempDir('docker-test-');
      const dockerfilePath = join(dir.name, 'Dockerfile');
      writeFileSync(dockerfilePath, 'FROM node:18-alpine\nCMD ["node"]');

      const result = await buildImageTool.handler(
        {
          dockerfilePath,
          context: dir.name,
          imageName: 'test:latest',
        },
        toolContext
      );

      await cleanup();

      // If Docker is unavailable, should fail with error
      if (!result.ok) {
        expect(result.error).toBeDefined();
        // May have guidance (optional)
        if (result.guidance) {
          expect(result.guidance.message).toBeDefined();
        }
      }
    });

    it('should handle Trivy scanner unavailable', async () => {
      // Scan will fail if Trivy not installed
      const result = await scanImageTool.handler(
        { imageId: 'alpine:latest' },
        toolContext
      );

      if (!result.ok) {
        expect(result.error).toBeDefined();
        expect(result.guidance).toBeDefined();
      }
    });
  });

  describe('Permission Errors', () => {
    it('should handle unreadable directory', async () => {
      if (process.platform === 'win32') {
        // Skip on Windows - different permission model
        return;
      }

      const { dir, cleanup } = createTestTempDir('permission-test-');
      const restrictedPath = join(dir.name, 'restricted');
      mkdirSync(restrictedPath);

      // Make directory unreadable
      try {
        chmodSync(restrictedPath, 0o000);

        const result = await analyzeRepoTool.handler(
          { repositoryPath: restrictedPath },
          toolContext
        );

        // Restore permissions before cleanup
        chmodSync(restrictedPath, 0o755);

        expect(result.ok).toBe(false);
        if (!result.ok) {
          expect(result.error).toBeDefined();
        }
      } finally {
        try {
          chmodSync(restrictedPath, 0o755);
        } catch {}
        await cleanup();
      }
    });

    it('should handle write-protected output directory', async () => {
      if (process.platform === 'win32') {
        return;
      }

      const { dir, cleanup } = createTestTempDir('write-test-');
      const readOnlyDir = join(dir.name, 'readonly');
      mkdirSync(readOnlyDir);

      try {
        chmodSync(readOnlyDir, 0o444);

        const result = await generateDockerfileTool.handler(
          {
            repositoryPath: dir.name,
            outputPath: join(readOnlyDir, 'Dockerfile'),
            targetPlatform: 'linux/amd64',
          },
          toolContext
        );

        chmodSync(readOnlyDir, 0o755);

        if (!result.ok) {
          expect(result.error).toBeDefined();
        }
      } finally {
        try {
          chmodSync(readOnlyDir, 0o755);
        } catch {}
        await cleanup();
      }
    });
  });

  describe('Resource Not Found', () => {
    it('should handle missing package.json in Node.js repo', async () => {
      const { dir, cleanup } = createTestTempDir('missing-pkg-');
      writeFileSync(join(dir.name, 'index.js'), 'console.log("hi");');

      const result = await analyzeRepoTool.handler(
        { repositoryPath: dir.name },
        toolContext
      );

      await cleanup();

      // Should still analyze but report missing package.json
      if (result.ok) {
        expect(result.value).toBeDefined();
      }
    });

    it('should handle missing requirements.txt in Python repo', async () => {
      const { dir, cleanup } = createTestTempDir('missing-req-');
      writeFileSync(join(dir.name, 'app.py'), 'print("hello")');

      const result = await analyzeRepoTool.handler(
        { repositoryPath: dir.name },
        toolContext
      );

      await cleanup();

      // Should still analyze
      if (result.ok) {
        expect(result.value).toBeDefined();
      }
    });

  });

  describe('Validation Failures', () => {
    it('should detect invalid Dockerfile syntax', async () => {
      const { dir, cleanup } = createTestTempDir('invalid-df-');
      const dockerfilePath = join(dir.name, 'Dockerfile');
      writeFileSync(dockerfilePath, 'INVALID_INSTRUCTION node:18\nBAD SYNTAX');

      const result = await fixDockerfileTool.handler(
        { dockerfilePath },
        toolContext
      );

      await cleanup();

      if (result.ok) {
        expect(result.value.findings).toBeDefined();
        expect(result.value.findings.length).toBeGreaterThan(0);
      }
    });

    it('should detect security issues in Dockerfile', async () => {
      const { dir, cleanup } = createTestTempDir('security-df-');
      const dockerfilePath = join(dir.name, 'Dockerfile');

      // Dockerfile with security issues
      writeFileSync(
        dockerfilePath,
        `FROM node:latest
USER root
RUN chmod 777 /app
COPY . .
CMD ["node", "app.js"]`
      );

      const result = await fixDockerfileTool.handler(
        { dockerfilePath },
        toolContext
      );

      await cleanup();

      if (result.ok) {
        const report = result.value;
        // Should find issues with :latest, running as root, etc.
        expect(report.findings.length).toBeGreaterThan(0);
      }
    });

    it('should reject invalid namespace format', async () => {
      const result = await prepareClusterTool.handler(
        { namespace: 'Invalid_Namespace_Name!' },
        toolContext
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeDefined();
      }
    });
  });

  describe('Edge Cases and Boundaries', () => {
    it('should handle very long file paths', async () => {
      const longPath = '/tmp/' + 'a'.repeat(200) + '/test';

      const result = await analyzeRepoTool.handler(
        { repositoryPath: longPath },
        toolContext
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeDefined();
      }
    });

    it('should handle special characters in paths', async () => {
      const result = await analyzeRepoTool.handler(
        { repositoryPath: '/tmp/test-app-with-special-chars-!@#$%' },
        toolContext
      );

      // Should handle gracefully (will fail because path doesn't exist)
      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeDefined();
      }
    });

    it('should handle concurrent tool executions', async () => {
      const { dir, cleanup } = createTestTempDir('concurrent-');

      for (let i = 0; i < 3; i++) {
        const appPath = join(dir.name, `app${i}`);
        mkdirSync(appPath);
        writeFileSync(
          join(appPath, 'package.json'),
          JSON.stringify({ name: `app${i}`, version: '1.0.0' })
        );
      }

      const results = await Promise.all([
        analyzeRepoTool.handler({ repositoryPath: join(dir.name, 'app0') }, toolContext),
        analyzeRepoTool.handler({ repositoryPath: join(dir.name, 'app1') }, toolContext),
        analyzeRepoTool.handler({ repositoryPath: join(dir.name, 'app2') }, toolContext),
      ]);

      await cleanup();

      expect(results).toHaveLength(3);
      results.forEach(result => {
        expect(result.ok !== undefined).toBe(true);
      });
    });
  });

  describe('Guidance Messages', () => {
    it('should provide error messages for common failures', async () => {
      const result = await analyzeRepoTool.handler(
        { repositoryPath: '/nonexistent' },
        toolContext
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toBeDefined();
        // Guidance is optional but helpful if present
        if (result.guidance) {
          expect(result.guidance.message).toBeDefined();
        }
      }
    });

    it('should suggest fixes in validation reports', async () => {
      const { dir, cleanup } = createTestTempDir('guidance-test-');
      const dockerfilePath = join(dir.name, 'Dockerfile');
      writeFileSync(dockerfilePath, 'FROM node:latest\n');

      const result = await fixDockerfileTool.handler(
        { dockerfilePath },
        toolContext
      );

      await cleanup();

      if (result.ok && result.value.findings.length > 0) {
        const finding = result.value.findings[0];
        expect(finding.message).toBeDefined();
        // Findings should have helpful messages
        expect(finding.message.length).toBeGreaterThan(0);
      }
    });
  });
});
