/**
 * Mock session manager for router integration testing
 */

import type { SessionManager } from '@lib/session';
import type { WorkflowState } from '@types';

export class MockSessionManager implements SessionManager {
  private sessions: Map<string, WorkflowState> = new Map();
  private sessionCounter = 0;

  async create(): Promise<WorkflowState> {
    const sessionId = `test-session-${++this.sessionCounter}`;
    const session: WorkflowState = {
      sessionId,
      createdAt: new Date(),
      updatedAt: new Date(),
      completed_steps: [],
      results: {},
    };
    this.sessions.set(sessionId, session);
    return session;
  }

  async get(sessionId: string): Promise<WorkflowState | null> {
    return this.sessions.get(sessionId) || null;
  }

  async update(
    sessionId: string,
    updates: Partial<WorkflowState>,
  ): Promise<WorkflowState | null> {
    const session = this.sessions.get(sessionId);
    if (!session) {
      return null;
    }

    const updated = {
      ...session,
      ...updates,
      updatedAt: new Date(),
    };
    this.sessions.set(sessionId, updated);
    return updated;
  }

  async delete(sessionId: string): Promise<boolean> {
    return this.sessions.delete(sessionId);
  }

  async list(): Promise<WorkflowState[]> {
    return Array.from(this.sessions.values());
  }

  // Test helper methods
  async clear(): Promise<void> {
    this.sessions.clear();
  }

  async createWithState(state: Partial<WorkflowState>): Promise<WorkflowState> {
    const sessionId = state.sessionId || `test-session-${++this.sessionCounter}`;
    const session: WorkflowState = {
      sessionId,
      createdAt: new Date(),
      updatedAt: new Date(),
      completed_steps: [],
      results: {},
      ...state,
    };
    this.sessions.set(sessionId, session);
    return session;
  }

  getSessionCount(): number {
    return this.sessions.size;
  }
}