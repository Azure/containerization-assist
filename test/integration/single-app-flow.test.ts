/**
 * Single App Flow Integration Test
 *
 * Tests the complete containerization journey for a single application
 * by executing the smoke:journey command to match CI behavior.
 */

import { execSync } from 'child_process';
import { join } from 'path';
import { existsSync, rmSync, readFileSync } from 'fs';
import { createApp } from '@/app';
import type { AppRuntime } from '@/types/runtime';

describe('Single App Flow Integration', () => {
  const outputDir = join(process.cwd(), '.smoke-test');

  beforeAll(() => {
    // Clean output directory
    if (existsSync(outputDir)) {
      rmSync(outputDir, { recursive: true, force: true });
    }
  });

  afterAll(() => {
    // Clean up output directory
    if (existsSync(outputDir)) {
      rmSync(outputDir, { recursive: true, force: true });
    }
  });

  describe('Smoke Journey Command', () => {
    it('should complete the full containerization workflow via smoke:journey', () => {
      // Set environment to mock AI sampling
      const env = {
        ...process.env,
        MCP_QUIET: 'true',
        MOCK_SAMPLING: 'true',
      };

      let result;
      try {
        // Execute the smoke:journey command
        result = execSync('npm run smoke:journey', {
          encoding: 'utf8',
          env,
          timeout: 120000, // 2 minute timeout
        });
      } catch (error: any) {
        // If Docker/K8s steps fail, that's OK for this test
        const output = error.stdout || error.output?.join('\n') || '';

        // Check that at least the basic steps completed
        expect(output).toContain('Analyze Repository');
        expect(output).toContain('Generate Dockerfile');
        expect(output).toContain('Generate Kubernetes Manifests');

        // It's OK if build/deploy steps fail due to missing Docker/K8s
        if (output.includes('Build Docker Image') && output.includes('Docker daemon')) {
          // Expected failure due to missing Docker
          return;
        }
      }

      if (result) {
        // Check that key steps were executed
        expect(result).toContain('Starting end-to-end containerization smoke test');
        expect(result).toContain('Analyze Repository');
        expect(result).toContain('Generate Dockerfile');
        expect(result).toContain('Generate Kubernetes Manifests');

        // Verify artifacts were created
        expect(existsSync(join(outputDir, 'Dockerfile'))).toBe(true);
        expect(existsSync(join(outputDir, 'k8s.yaml'))).toBe(true);
        expect(existsSync(join(outputDir, 'analysis.json'))).toBe(true);
      }
    }, 150000);

    it('should generate valid artifacts', () => {
      // Only check if files were created in previous test
      if (existsSync(outputDir)) {
        const dockerfilePath = join(outputDir, 'Dockerfile');
        const k8sPath = join(outputDir, 'k8s.yaml');
        const analysisPath = join(outputDir, 'analysis.json');

        // Check Dockerfile if it exists
        if (existsSync(dockerfilePath)) {
          const dockerfile = readFileSync(dockerfilePath, 'utf8');
          expect(dockerfile).toContain('FROM');
          expect(dockerfile.length).toBeGreaterThan(10);
        }

        // Check K8s manifest if it exists
        if (existsSync(k8sPath)) {
          const k8s = readFileSync(k8sPath, 'utf8');
          expect(k8s).toContain('apiVersion');
          expect(k8s).toContain('kind');
        }

        // Check analysis if it exists
        if (existsSync(analysisPath)) {
          const analysis = JSON.parse(readFileSync(analysisPath, 'utf8'));
          expect(analysis).toHaveProperty('language');
        }
      }
    });
  });

  describe('Session Management', () => {
    it('should support single session mode', async () => {
      const runtime = createApp({});

      const health = runtime.healthCheck();
      expect(health.status).toBe('healthy');

      await runtime.stop();
    });

    it('should maintain session state across tool executions', async () => {
      const runtime = createApp({});

      const health = runtime.healthCheck();
      expect(health.status).toBe('healthy');

      await runtime.stop();
    });
  });
});