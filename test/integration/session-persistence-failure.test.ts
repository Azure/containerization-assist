/**
 * Integration test: Session persistence failure propagation
 *
 * Tests that session storage failures are properly detected and propagated
 * through the tool execution stack, resulting in tool failure rather than
 * silent data loss.
 */

import { describe, it, expect, beforeEach } from '@jest/globals';
import { randomUUID } from 'node:crypto';
import { createLogger } from '@/lib/logger';
import { SessionManager } from '@/session/core';
import { storeToolResults } from '@/lib/tool-helpers';
import type { ToolContext } from '@/mcp/context';
import type { WorkflowState } from '@/types';

describe('Session persistence failure propagation', () => {
  let sessionManager: SessionManager;
  let sessionId: string;
  const logger = createLogger({ name: 'test-session-persistence' });

  beforeEach(async () => {
    sessionManager = new SessionManager(logger);
    sessionId = randomUUID();
    await sessionManager.create(sessionId);
  });

  afterEach(() => {
    sessionManager.close();
  });

  it('should fail when session does not exist', async () => {
    const nonExistentSessionId = randomUUID();
    const ctx: ToolContext = {
      logger,
      sessionManager,
    } as ToolContext;

    const result = await storeToolResults(ctx, nonExistentSessionId, 'analyze-repo', {
      language: 'typescript',
      framework: 'express',
    });

    expect(result.ok).toBe(false);
    expect(result.error).toContain('does not exist');
    expect(result.error).toContain('must be created before tool execution');
  });

  it('should fail when session update returns failure', async () => {
    // Create a failing session manager by mocking the update method
    const originalUpdate = sessionManager.update.bind(sessionManager);
    sessionManager.update = async () => ({
      ok: false,
      error: 'Simulated persistence failure: disk full',
    });

    const ctx: ToolContext = {
      logger,
      sessionManager,
    } as ToolContext;

    const result = await storeToolResults(ctx, sessionId, 'build-image', {
      imageId: 'sha256:abc123',
      tags: ['myapp:latest'],
    });

    expect(result.ok).toBe(false);
    expect(result.error).toContain('Session update failed');
    expect(result.error).toContain('disk full');

    // Restore original method
    sessionManager.update = originalUpdate;
  });

  it('should propagate failure through full workflow', async () => {
    // Simulate a complete workflow with persistence failure
    const ctx: ToolContext = {
      logger,
      sessionManager,
    } as ToolContext;

    // Step 1: Store analyze-repo results - should succeed
    const analyzeResult = await storeToolResults(ctx, sessionId, 'analyze-repo', {
      language: 'typescript',
      framework: 'nestjs',
      ports: [3000],
    });
    expect(analyzeResult.ok).toBe(true);

    // Step 2: Inject failure for next update
    const originalUpdate = sessionManager.update.bind(sessionManager);
    sessionManager.update = async () => ({
      ok: false,
      error: 'Database connection lost',
    });

    // Step 3: Try to store build-image results - should fail
    const buildResult = await storeToolResults(ctx, sessionId, 'build-image', {
      imageId: 'sha256:def456',
      tags: ['myapp:v1.0.0'],
    });

    expect(buildResult.ok).toBe(false);
    expect(buildResult.error).toContain('Session update failed');
    expect(buildResult.error).toContain('Database connection lost');

    // Restore original method
    sessionManager.update = originalUpdate;

    // Step 4: Verify that the first result was stored but second was not
    const sessionResult = await sessionManager.get(sessionId);
    expect(sessionResult.ok).toBe(true);
    expect(sessionResult.value).not.toBeNull();

    const session = sessionResult.value as WorkflowState;
    const results = session.metadata?.results as Record<string, any>;

    expect(results?.['analyze-repo']).toBeDefined();
    expect(results?.['analyze-repo']?.language).toBe('typescript');

    // The build-image result should NOT be present due to failure
    expect(results?.['build-image']).toBeUndefined();
  });

  it('should handle exception thrown by session manager', async () => {
    const originalUpdate = sessionManager.update.bind(sessionManager);
    sessionManager.update = async () => {
      throw new Error('Unexpected network error');
    };

    const ctx: ToolContext = {
      logger,
      sessionManager,
    } as ToolContext;

    const result = await storeToolResults(ctx, sessionId, 'deploy', {
      namespace: 'default',
      deploymentName: 'myapp',
    });

    expect(result.ok).toBe(false);
    expect(result.error).toContain('Failed to store results');
    expect(result.error).toContain('Unexpected network error');

    // Restore original method
    sessionManager.update = originalUpdate;
  });

  it('should successfully store when session exists and update succeeds', async () => {
    const ctx: ToolContext = {
      logger,
      sessionManager,
    } as ToolContext;

    // Store multiple tool results sequentially
    const analyzeResult = await storeToolResults(ctx, sessionId, 'analyze-repo', {
      language: 'java',
      framework: 'spring-boot',
    });
    expect(analyzeResult.ok).toBe(true);

    const buildResult = await storeToolResults(ctx, sessionId, 'build-image', {
      imageId: 'sha256:123abc',
      tags: ['app:latest'],
    });
    expect(buildResult.ok).toBe(true);

    const deployResult = await storeToolResults(ctx, sessionId, 'deploy', {
      namespace: 'production',
      ready: true,
    });
    expect(deployResult.ok).toBe(true);

    // Verify all results were stored
    const sessionResult = await sessionManager.get(sessionId);
    expect(sessionResult.ok).toBe(true);

    const session = sessionResult.value as WorkflowState;
    const results = session.metadata?.results as Record<string, any>;

    expect(results?.['analyze-repo']).toBeDefined();
    expect(results?.['build-image']).toBeDefined();
    expect(results?.['deploy']).toBeDefined();
  });

  it('should fail gracefully when sessionManager is missing from context', async () => {
    const ctx: ToolContext = {
      logger,
      sessionManager: undefined,
    } as ToolContext;

    // Should return success (no-op) when sessionManager is not available
    const result = await storeToolResults(ctx, sessionId, 'test-tool', { foo: 'bar' });

    expect(result.ok).toBe(true);
  });

  it('should fail gracefully when sessionId is undefined', async () => {
    const ctx: ToolContext = {
      logger,
      sessionManager,
    } as ToolContext;

    // Should return success (no-op) when sessionId is undefined
    const result = await storeToolResults(ctx, undefined, 'test-tool', { foo: 'bar' });

    expect(result.ok).toBe(true);
  });

  it('should merge results without overwriting existing data', async () => {
    const ctx: ToolContext = {
      logger,
      sessionManager,
    } as ToolContext;

    // Store first tool result
    await storeToolResults(ctx, sessionId, 'tool-a', {
      dataA: 'value-a',
      shared: 'original',
    });

    // Store second tool result
    await storeToolResults(ctx, sessionId, 'tool-b', {
      dataB: 'value-b',
    });

    // Update first tool result (should replace tool-a but not affect tool-b)
    await storeToolResults(ctx, sessionId, 'tool-a', {
      dataA: 'updated-value-a',
      shared: 'updated',
      newField: 'new',
    });

    // Verify final state
    const sessionResult = await sessionManager.get(sessionId);
    expect(sessionResult.ok).toBe(true);

    const session = sessionResult.value as WorkflowState;
    const results = session.metadata?.results as Record<string, any>;

    expect(results?.['tool-a']).toEqual({
      dataA: 'updated-value-a',
      shared: 'updated',
      newField: 'new',
    });

    expect(results?.['tool-b']).toEqual({
      dataB: 'value-b',
    });
  });
});
