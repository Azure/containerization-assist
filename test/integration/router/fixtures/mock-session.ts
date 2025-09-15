/**
 * Mock session manager for router integration testing
 */

import type { SessionManager } from '@lib/session';
import type { WorkflowState, Result } from '@types';
import { Success, Failure } from '../../../../src/types';

export class MockSessionManager implements SessionManager {
  private sessions: Map<string, WorkflowState> = new Map();
  private sessionCounter = 0;

  async create(sessionId?: string): Promise<Result<WorkflowState>> {
    const id = sessionId || `test-session-${++this.sessionCounter}`;
    const session: WorkflowState = {
      sessionId: id,
      createdAt: new Date(),
      updatedAt: new Date(),
      completed_steps: [],
      results: {},
    };
    this.sessions.set(id, session);
    return Success(session);
  }

  async get(sessionId: string): Promise<Result<WorkflowState | null>> {
    return Success(this.sessions.get(sessionId) || null);
  }

  async update(
    sessionId: string,
    updates: Partial<WorkflowState>,
  ): Promise<Result<WorkflowState>> {
    const session = this.sessions.get(sessionId);
    if (!session) {
      return Failure(`Session ${sessionId} not found`);
    }

    const updated = {
      ...session,
      ...updates,
      updatedAt: new Date(),
    };
    this.sessions.set(sessionId, updated);
    return Success(updated);
  }

  async delete(sessionId: string): Promise<Result<void>> {
    this.sessions.delete(sessionId);
    return Success(undefined);
  }

  async list(): Promise<Result<string[]>> {
    return Success(Array.from(this.sessions.keys()));
  }

  async cleanup(olderThan: Date): Promise<Result<void>> {
    for (const [id, session] of this.sessions.entries()) {
      if (session.createdAt && session.createdAt < olderThan) {
        this.sessions.delete(id);
      }
    }
    return Success(undefined);
  }

  close(): void {
    // No-op for mock
  }

  // Test helper methods
  async clear(): Promise<void> {
    this.sessions.clear();
  }

  async createWithState(state: Partial<WorkflowState>): Promise<Result<WorkflowState>> {
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
    return Success(session);
  }

  getSessionCount(): number {
    return this.sessions.size;
  }
}