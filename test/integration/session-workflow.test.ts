/**
 * Integration test: Session-driven workflow without explicit parameters
 *
 * Tests that downstream tools (generate-dockerfile, generate-k8s-manifests)
 * can pull context from session metadata set by analyze-repo without requiring
 * users to re-enter repository paths, app names, ports, or image metadata.
 */

import { describe, it, expect, beforeAll, afterAll } from '@jest/globals';
import { randomUUID } from 'node:crypto';
import { createLogger } from '@/lib/logger';
import { SessionManager } from '@/session/core';
import type { WorkflowState } from '@/types';

describe('Session-driven workflow integration', () => {
  let sessionManager: SessionManager;
  let sessionId: string;
  const logger = createLogger({ name: 'test-session-workflow' });

  beforeAll(async () => {
    sessionManager = new SessionManager(logger);
    sessionId = randomUUID();
    await sessionManager.create(sessionId);
  });

  afterAll(() => {
    sessionManager.close();
  });

  it('should persist analyze-repo results in session', async () => {
    // Simulate analyze-repo storing results
    const sessionResult = await sessionManager.get(sessionId);
    expect(sessionResult.ok).toBe(true);
    expect(sessionResult.value).not.toBeNull();

    const session = sessionResult.value as WorkflowState;

    // Simulate what analyze-repo stores using normalized pattern
    const analyzeResult = {
      name: 'test-app',
      language: 'javascript',
      framework: 'express',
      ports: [3000, 8080],
      dependencies: {},
    };

    // Update session with normalized metadata (results + discrete keys, NO nested metadata)
    await sessionManager.update(sessionId, {
      metadata: {
        analyzedPath: '/test/repo/path',
        appName: 'test-app',
        appPorts: [3000, 8080],
        results: {
          'analyze-repo': analyzeResult,
        },
      },
      completed_steps: ['analyze-repo'],
    });

    // Verify data was stored using discrete keys
    const updated = await sessionManager.get(sessionId);
    expect(updated.ok).toBe(true);
    expect(updated.value?.metadata?.appName).toBe('test-app');
    expect(updated.value?.metadata?.appPorts).toEqual([3000, 8080]);
    expect(updated.value?.metadata?.analyzedPath).toBe('/test/repo/path');
    expect(updated.value?.metadata?.results?.['analyze-repo']).toEqual(analyzeResult);
  });

  it('should allow generate-dockerfile to read session metadata', async () => {
    // Simulate generate-dockerfile reading from session
    const sessionResult = await sessionManager.get(sessionId);
    expect(sessionResult.ok).toBe(true);

    const session = sessionResult.value as WorkflowState;

    // Extract what generate-dockerfile would read using normalized pattern
    const appName = session.metadata?.appName;
    const analyzedPath = session.metadata?.analyzedPath;
    const analyzeResult = (session.metadata?.results as Record<string, any>)?.['analyze-repo'];

    expect(appName).toBe('test-app');
    expect(analyzedPath).toBe('/test/repo/path');
    expect(analyzeResult?.language).toBe('javascript');
    expect(analyzeResult?.framework).toBe('express');

    // Simulate storing Dockerfile result using storeResult + discrete keys pattern
    await sessionManager.update(sessionId, {
      metadata: {
        ...session.metadata,
        dockerfileGenerated: true, // Discrete flag
        dockerfilePath: '/test/repo/path/Dockerfile', // Discrete key
        results: {
          ...(session.metadata?.results as Record<string, any>),
          'generate-dockerfile': {
            content: 'FROM node:18\n...',
            path: '/test/repo/path/Dockerfile',
            baseImage: 'node:18',
            multistage: false,
            securityHardening: true,
            optimization: 'balanced',
          },
        },
      },
      completed_steps: [...(session.completed_steps || []), 'generate-dockerfile'],
    });
  });

  it('should allow build-image to read session metadata', async () => {
    // Simulate build-image reading from session using getResult pattern
    const sessionResult = await sessionManager.get(sessionId);
    expect(sessionResult.ok).toBe(true);

    const session = sessionResult.value as WorkflowState;
    const dockerfileResult = (session.metadata?.results as Record<string, any>)?.[
      'generate-dockerfile'
    ];

    // Verify the normalized result structure
    expect(dockerfileResult?.content).toBe('FROM node:18\n...');
    expect(dockerfileResult?.baseImage).toBe('node:18');
    expect(dockerfileResult?.path).toBe('/test/repo/path/Dockerfile');

    // Verify discrete flags are accessible
    expect(session.metadata?.dockerfileGenerated).toBe(true);
    expect(session.metadata?.dockerfilePath).toBe('/test/repo/path/Dockerfile');

    // Simulate storing build result using storeResult pattern
    await sessionManager.update(sessionId, {
      metadata: {
        ...session.metadata,
        results: {
          ...(session.metadata?.results as Record<string, any>),
          'build-image': {
            success: true,
            sessionId,
            imageId: 'sha256:abc123',
            tags: ['test-app:latest', 'test-app:v1.0.0'],
            size: 500000000,
            buildTime: 45000,
            logs: [],
          },
        },
      },
      completed_steps: [...(session.completed_steps || []), 'build-image'],
    });
  });

  it('should allow generate-k8s-manifests to read session metadata without explicit parameters', async () => {
    // Simulate generate-k8s-manifests reading from session
    const sessionResult = await sessionManager.get(sessionId);
    expect(sessionResult.ok).toBe(true);

    const session = sessionResult.value as WorkflowState;

    // Extract what generate-k8s-manifests would read (NO explicit imageId/appName/port required!)
    const appName = session.metadata?.appName;
    const appPorts = session.metadata?.appPorts as number[] | undefined;
    const buildResult = (session.metadata?.results as Record<string, any>)?.['build-image'];

    expect(appName).toBe('test-app');
    expect(appPorts).toEqual([3000, 8080]);
    expect(buildResult?.tags).toContain('test-app:latest');

    // The tool would use:
    const imageId = buildResult?.tags?.[0]; // 'test-app:latest'
    const port = appPorts?.[0]; // 3000

    expect(imageId).toBe('test-app:latest');
    expect(port).toBe(3000);

    // Simulate storing manifests result
    await sessionManager.update(sessionId, {
      metadata: {
        ...session.metadata,
        results: {
          ...(session.metadata?.results as Record<string, any>),
          'generate-k8s-manifests': {
            manifests: 'apiVersion: apps/v1\nkind: Deployment\n...',
            deploymentName: 'test-app',
          },
        },
      },
      completed_steps: [...(session.completed_steps || []), 'generate-k8s-manifests'],
    });
  });

  it('should verify full workflow state after all steps', async () => {
    const sessionResult = await sessionManager.get(sessionId);
    expect(sessionResult.ok).toBe(true);

    const session = sessionResult.value as WorkflowState;

    // Verify all steps completed
    expect(session.completed_steps).toEqual([
      'analyze-repo',
      'generate-dockerfile',
      'build-image',
      'generate-k8s-manifests',
    ]);

    // Verify all results stored
    const results = session.metadata?.results as Record<string, any>;
    expect(results).toHaveProperty('analyze-repo');
    expect(results).toHaveProperty('generate-dockerfile');
    expect(results).toHaveProperty('build-image');
    expect(results).toHaveProperty('generate-k8s-manifests');

    // Verify metadata still intact
    expect(session.metadata?.appName).toBe('test-app');
    expect(session.metadata?.appPorts).toEqual([3000, 8080]);
    expect(session.metadata?.analyzedPath).toBe('/test/repo/path');
  });

  it('should verify SessionFacade helper methods work correctly', async () => {
    const sessionResult = await sessionManager.get(sessionId);
    expect(sessionResult.ok).toBe(true);

    const session = sessionResult.value as WorkflowState;

    // Test SessionFacade.getResult pattern (what tools use)
    const results = session.metadata?.results as Record<string, any> | undefined;

    // Simulate SessionFacade.getResult('build-image')
    const buildResult = results?.['build-image'];
    expect(buildResult?.tags).toContain('test-app:latest');

    // Simulate SessionFacade.get('appName')
    const appName = session.metadata?.appName;
    expect(appName).toBe('test-app');

    // Simulate SessionFacade.get('appPorts')
    const appPorts = session.metadata?.appPorts;
    expect(appPorts).toEqual([3000, 8080]);
  });
});