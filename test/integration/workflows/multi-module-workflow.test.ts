/**
 * Integration Test: Multi-Module Workflow
 *
 * Tests containerization of applications with multiple modules (monorepos):
 * - Detects multiple modules in a single repository
 * - Generates Dockerfiles for each module
 * - Generates Kubernetes manifests for multi-service deployments
 * - Handles inter-module dependencies
 *
 * Prerequisites:
 * - Multi-module fixtures available
 * - Docker daemon running (optional, for build tests)
 */

import { describe, it, expect, beforeAll, afterAll } from '@jest/globals';
import { createApp } from '@/app';
import type { AppRuntime } from '@/types/runtime';
import { createLogger } from '@/lib/logger';
import { join } from 'node:path';
import { existsSync, mkdirSync, writeFileSync } from 'node:fs';
import { createTestTempDir } from '../../__support__/utilities/tmp-helpers';
import type { DirResult } from 'tmp';
import type { RepositoryAnalysis } from '@/tools/analyze-repo/schema';
import type { DockerfilePlan } from '@/tools/generate-dockerfile/schema';
import type { K8sManifestPlan } from '@/tools/generate-k8s-manifests/schema';

describe('Multi-Module Workflow Integration', () => {
  let runtime: AppRuntime;
  let testDir: DirResult;
  let cleanup: () => Promise<void>;
  const logger = createLogger({ level: 'silent' });
  const testTimeout = 90000; // 90 seconds

  beforeAll(async () => {
    runtime = createApp({ logger });

    // Create temporary directory for test outputs
    const result = createTestTempDir('multi-module-test-');
    testDir = result.dir;
    cleanup = result.cleanup;
  });

  afterAll(async () => {
    await cleanup();
    await runtime.stop();
  });

  describe('Monorepo Detection and Analysis', () => {
    it('should detect multiple modules in a monorepo structure', async () => {
      // Create a test monorepo structure
      const monorepoPath = join(testDir.name, 'test-monorepo');
      mkdirSync(monorepoPath, { recursive: true });

      // Create API service (Node.js)
      const apiPath = join(monorepoPath, 'api');
      mkdirSync(apiPath, { recursive: true });
      writeFileSync(
        join(apiPath, 'package.json'),
        JSON.stringify({
          name: 'api',
          version: '1.0.0',
          dependencies: { express: '^4.18.0' },
          scripts: { start: 'node index.js' },
        })
      );
      writeFileSync(join(apiPath, 'index.js'), 'console.log("API");');

      // Create Worker service (Node.js)
      const workerPath = join(monorepoPath, 'worker');
      mkdirSync(workerPath, { recursive: true });
      writeFileSync(
        join(workerPath, 'package.json'),
        JSON.stringify({
          name: 'worker',
          version: '1.0.0',
          dependencies: { bullmq: '^3.0.0' },
          scripts: { start: 'node worker.js' },
        })
      );
      writeFileSync(join(workerPath, 'worker.js'), 'console.log("Worker");');

      // Analyze the monorepo
      const result = await runtime.execute('analyze-repo', {
        repositoryPath: monorepoPath,
      });

      expect(result.ok).toBe(true);
      if (!result.ok) {
        console.error('Analysis failed:', result.error);
        return;
      }

      const analysis = result.value as RepositoryAnalysis;
      expect(analysis.isMonorepo).toBe(true);
      expect(analysis.modules).toBeDefined();
      expect(analysis.modules.length).toBeGreaterThanOrEqual(2);

      // Verify modules are detected
      const moduleNames = analysis.modules.map(m => m.name);
      expect(moduleNames).toContain('api');
      expect(moduleNames).toContain('worker');
    }, testTimeout);
  });

  describe('Multi-Module Dockerfile Generation', () => {
    it('should generate Dockerfiles for each module independently', async () => {
      const monorepoPath = join(testDir.name, 'multi-dockerfile-test');
      mkdirSync(monorepoPath, { recursive: true });

      // Create two services
      const service1Path = join(monorepoPath, 'service1');
      mkdirSync(service1Path, { recursive: true });
      writeFileSync(
        join(service1Path, 'package.json'),
        JSON.stringify({
          name: 'service1',
          dependencies: { express: '^4.18.0' },
        })
      );

      const service2Path = join(monorepoPath, 'service2');
      mkdirSync(service2Path, { recursive: true });
      writeFileSync(
        join(service2Path, 'package.json'),
        JSON.stringify({
          name: 'service2',
          dependencies: { koa: '^2.14.0' },
        })
      );

      const sessionId = 'multi-dockerfile-test';

      // Analyze
      const analysisResult = await runtime.execute('analyze-repo', {
        repositoryPath: monorepoPath,
      }, { sessionId });

      expect(analysisResult.ok).toBe(true);
      if (!analysisResult.ok) return;

      const analysis = analysisResult.value as RepositoryAnalysis;
      expect(analysis.modules.length).toBe(2);

      // Generate Dockerfile for first module
      const dockerfile1Result = await runtime.execute('generate-dockerfile', {
        repositoryPath: monorepoPath,
        modules: [analysis.modules[0]],
        outputPath: join(testDir.name, 'Dockerfile.service1'),
      }, { sessionId });

      expect(dockerfile1Result.ok).toBe(true);
      if (!dockerfile1Result.ok) return;

      const dockerfile1 = dockerfile1Result.value as DockerfilePlan;
      expect(dockerfile1.recommendations.dockerfile).toContain('FROM');
      expect(dockerfile1.recommendations.dockerfile).toContain('node');

      // Generate Dockerfile for second module
      const dockerfile2Result = await runtime.execute('generate-dockerfile', {
        repositoryPath: monorepoPath,
        modules: [analysis.modules[1]],
        outputPath: join(testDir.name, 'Dockerfile.service2'),
      }, { sessionId });

      expect(dockerfile2Result.ok).toBe(true);
      if (!dockerfile2Result.ok) return;

      const dockerfile2 = dockerfile2Result.value as DockerfilePlan;
      expect(dockerfile2.recommendations.dockerfile).toContain('FROM');
    }, testTimeout);
  });

  describe('Multi-Service Kubernetes Manifests', () => {
    it('should generate K8s manifests for multiple services', async () => {
      const monorepoPath = join(testDir.name, 'multi-k8s-test');
      mkdirSync(monorepoPath, { recursive: true });

      // Create frontend service
      const frontendPath = join(monorepoPath, 'frontend');
      mkdirSync(frontendPath, { recursive: true });
      writeFileSync(
        join(frontendPath, 'package.json'),
        JSON.stringify({
          name: 'frontend',
          dependencies: { react: '^18.0.0' },
          scripts: { start: 'react-scripts start' },
        })
      );

      // Create backend service
      const backendPath = join(monorepoPath, 'backend');
      mkdirSync(backendPath, { recursive: true });
      writeFileSync(
        join(backendPath, 'package.json'),
        JSON.stringify({
          name: 'backend',
          dependencies: { express: '^4.18.0' },
          scripts: { start: 'node server.js' },
        })
      );

      const sessionId = 'multi-k8s-test';

      // Analyze
      const analysisResult = await runtime.execute('analyze-repo', {
        repositoryPath: monorepoPath,
      }, { sessionId });

      expect(analysisResult.ok).toBe(true);
      if (!analysisResult.ok) return;

      const analysis = analysisResult.value as RepositoryAnalysis;
      expect(analysis.modules.length).toBe(2);

      // Generate K8s manifests for all modules
      const k8sResult = await runtime.execute('generate-k8s-manifests', {
        repositoryPath: monorepoPath,
        modules: analysis.modules,
        outputPath: join(testDir.name, 'k8s-multi-service.yaml'),
      }, { sessionId });

      expect(k8sResult.ok).toBe(true);
      if (!k8sResult.ok) {
        console.error('K8s generation failed:', k8sResult.error);
        return;
      }

      const k8sPlan = k8sResult.value as K8sManifestPlan;
      expect(k8sPlan.manifests).toBeDefined();
      expect(k8sPlan.manifests.length).toBeGreaterThan(0);

      // Should have Deployments for each service
      const deployments = k8sPlan.manifests.filter(m => m.kind === 'Deployment');
      expect(deployments.length).toBeGreaterThanOrEqual(2);

      // Should have Services for each deployment
      const services = k8sPlan.manifests.filter(m => m.kind === 'Service');
      expect(services.length).toBeGreaterThanOrEqual(2);
    }, testTimeout);
  });

  describe('Module Isolation and Independence', () => {
    it('should handle failures in one module without affecting others', async () => {
      // Create a monorepo with one valid and one invalid module
      const monorepoPath = join(testDir.name, 'partial-failure-test');
      mkdirSync(monorepoPath, { recursive: true });

      // Valid module
      const validPath = join(monorepoPath, 'valid-service');
      mkdirSync(validPath, { recursive: true });
      writeFileSync(
        join(validPath, 'package.json'),
        JSON.stringify({
          name: 'valid-service',
          dependencies: { express: '^4.18.0' },
        })
      );

      // Invalid module (malformed package.json)
      const invalidPath = join(monorepoPath, 'invalid-service');
      mkdirSync(invalidPath, { recursive: true });
      writeFileSync(join(invalidPath, 'package.json'), 'invalid json {{{');

      // Analysis should still work (may skip invalid module or handle gracefully)
      const result = await runtime.execute('analyze-repo', {
        repositoryPath: monorepoPath,
      });

      expect(result.ok).toBe(true);
      if (result.ok) {
        const analysis = result.value as RepositoryAnalysis;
        // Should detect at least the valid module
        expect(analysis.modules).toBeDefined();
        expect(analysis.modules.length).toBeGreaterThanOrEqual(1);
      }
    }, testTimeout);
  });
});
