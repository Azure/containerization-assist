/**
 * Integration Test: Session State Alignment
 *
 * Validates that the canonical session structure (metadata.results) works
 * correctly for the analyze-repo → generate-dockerfile workflow.
 *
 * This test ensures:
 * 1. analyze-repo stores results in metadata.results
 * 2. generate-dockerfile can read those results via SessionFacade.getResult
 * 3. No legacy fallback paths are used
 */

import { SessionManager } from '@/session/core';
import { createLogger } from '@/lib/logger';
import { updateSessionResults } from '@/lib/tool-helpers';
import type { WorkflowState } from '@/types';

describe('Session State Alignment: analyze-repo → generate-dockerfile', () => {
  let sessionManager: SessionManager;
  const logger = createLogger({ name: 'test-session-alignment' });

  beforeEach(() => {
    sessionManager = new SessionManager(logger);
  });

  afterEach(() => {
    sessionManager.close();
  });

  it('should store and retrieve analysis results using canonical structure', async () => {
    // 1. Create a session (simulating orchestrator behavior)
    const createResult = await sessionManager.create();
    expect(createResult.ok).toBe(true);

    if (!createResult.ok) return;
    const session = createResult.value;
    const sessionId = session.sessionId;

    // Verify canonical structure is initialized
    expect(session.metadata).toBeDefined();
    expect(session.metadata?.results).toEqual({});

    // 2. Simulate analyze-repo storing results
    const analysisResults = {
      language: 'Java',
      framework: 'Spring Boot',
      buildTool: 'Maven',
      detectedPort: 8080,
      analyzedPath: '/test/repo',
    };

    updateSessionResults(session, 'analyze-repo', analysisResults);

    // Persist the session
    const updateResult = await sessionManager.update(sessionId, session);
    expect(updateResult.ok).toBe(true);

    // 3. Simulate generate-dockerfile reading results
    const getResult = await sessionManager.get(sessionId);
    expect(getResult.ok).toBe(true);

    if (!getResult.ok) return;
    const retrievedSession = getResult.value;

    // Verify results are in canonical location
    expect(retrievedSession?.metadata?.results).toBeDefined();
    const results = retrievedSession?.metadata?.results as Record<string, unknown>;
    expect(results['analyze-repo']).toEqual(analysisResults);

    // 4. Verify SessionFacade.getResult pattern works (what tools actually use)
    const mockSessionFacade = {
      getResult<T = unknown>(toolName: string): T | undefined {
        const results = retrievedSession?.metadata?.results;
        if (!results || typeof results !== 'object') {
          return undefined;
        }
        return (results as Record<string, unknown>)[toolName] as T | undefined;
      },
    };

    const retrievedAnalysis = mockSessionFacade.getResult<typeof analysisResults>('analyze-repo');
    expect(retrievedAnalysis).toEqual(analysisResults);
    expect(retrievedAnalysis?.language).toBe('Java');
    expect(retrievedAnalysis?.framework).toBe('Spring Boot');
  });

  it('should prevent writes to legacy top-level results field', async () => {
    const createResult = await sessionManager.create();
    expect(createResult.ok).toBe(true);

    if (!createResult.ok) return;
    const session = createResult.value;

    // Attempt to write to legacy location (should NOT be supported)
    const sessionWithLegacy = session as WorkflowState & { results?: unknown };
    sessionWithLegacy.results = { 'analyze-repo': { legacy: true } };

    // Update session
    await sessionManager.update(session.sessionId, sessionWithLegacy);

    // Retrieve and verify legacy field is not in canonical location
    const getResult = await sessionManager.get(session.sessionId);
    expect(getResult.ok).toBe(true);

    if (!getResult.ok) return;
    const retrieved = getResult.value;

    // Canonical location should be empty (not populated from legacy field)
    expect(retrieved?.metadata?.results).toEqual({});
  });

  it('should handle missing analysis gracefully in generate-dockerfile pattern', async () => {
    const createResult = await sessionManager.create();
    expect(createResult.ok).toBe(true);

    if (!createResult.ok) return;
    const session = createResult.value;

    // Simulate SessionFacade.getResult when no results exist
    const mockSessionFacade = {
      getResult<T = unknown>(toolName: string): T | undefined {
        const results = session.metadata?.results;
        if (!results || typeof results !== 'object') {
          return undefined;
        }
        return (results as Record<string, unknown>)[toolName] as T | undefined;
      },
    };

    // Should return undefined when tool hasn't run yet
    const analysisResult = mockSessionFacade.getResult('analyze-repo');
    expect(analysisResult).toBeUndefined();
  });

  it('should maintain data integrity across multiple tool executions', async () => {
    const createResult = await sessionManager.create();
    expect(createResult.ok).toBe(true);

    if (!createResult.ok) return;
    const session = createResult.value;
    const sessionId = session.sessionId;

    // Simulate multiple tools storing results
    const analysisResults = { language: 'Python', framework: 'FastAPI' };
    const buildResults = { imageId: 'sha256:abc123', tags: ['myapp:latest'] };
    const scanResults = { vulnerabilities: 0, passed: true };

    updateSessionResults(session, 'analyze-repo', analysisResults);
    await sessionManager.update(sessionId, session);

    updateSessionResults(session, 'build-image', buildResults);
    await sessionManager.update(sessionId, session);

    updateSessionResults(session, 'scan', scanResults);
    await sessionManager.update(sessionId, session);

    // Retrieve and verify all results are present
    const getResult = await sessionManager.get(sessionId);
    expect(getResult.ok).toBe(true);

    if (!getResult.ok) return;
    const retrieved = getResult.value;
    const results = retrieved?.metadata?.results as Record<string, unknown>;

    expect(results['analyze-repo']).toEqual(analysisResults);
    expect(results['build-image']).toEqual(buildResults);
    expect(results['scan']).toEqual(scanResults);
  });

  it('should throw descriptive error when session is invalid', () => {
    const invalidSession = null as unknown as WorkflowState;

    expect(() => {
      updateSessionResults(invalidSession, 'test-tool', { data: 'value' });
    }).toThrow(/Cannot update session results: session is null/);
  });

  it('should throw descriptive error when toolName is invalid', async () => {
    const createResult = await sessionManager.create();
    expect(createResult.ok).toBe(true);

    if (!createResult.ok) return;
    const session = createResult.value;

    expect(() => {
      updateSessionResults(session, '', { data: 'value' });
    }).toThrow(/Cannot update session results: toolName is invalid/);
  });

  it('should throw descriptive error when session lacks sessionId', () => {
    const invalidSession = {
      sessionId: '',
      metadata: {},
      createdAt: new Date(),
      updatedAt: new Date(),
    } as WorkflowState;

    expect(() => {
      updateSessionResults(invalidSession, 'test-tool', { data: 'value' });
    }).toThrow(/Cannot update session results: session\.sessionId is missing/);
  });
});
