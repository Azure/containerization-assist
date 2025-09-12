/**
 * Tests for Session Helpers Module
 */

import { describe, it, expect, jest, beforeEach, afterEach } from '@jest/globals';
import type { Logger } from 'pino';
import { 
  getSession,
  completeStep,
  createSession,
  updateSession
} from '@mcp/tool-session-helpers';
import type { SessionManager } from '@lib/session';
import type { WorkflowState } from '../../../src/types';
import type { ToolContext } from '@mcp/context';

// Mock the session module
jest.mock('@lib/session');

// Mock logger
const mockLogger: Logger = {
  info: jest.fn(),
  error: jest.fn(),
  warn: jest.fn(),
  debug: jest.fn(),
  trace: jest.fn(),
  fatal: jest.fn(),
  child: jest.fn(() => mockLogger),
} as any;

// Mock session manager with proper types
let sessionManager: jest.Mocked<SessionManager<WorkflowState>>;

beforeEach(() => {
  jest.clearAllMocks();
  
  // Create a fresh mock session manager for each test
  sessionManager = {
    get: jest.fn(),
    create: jest.fn(),
    update: jest.fn(),
    list: jest.fn(),
    delete: jest.fn(),
  } as any;
  
  // Mock the session module to return our mocked session manager
  const sessionModule = require('@lib/session');
  sessionModule.createSessionManager = jest.fn().mockReturnValue(sessionManager);
});

afterEach(() => {
  jest.clearAllMocks();
});

// Helper to create context with sessionManager
function createContext(): ToolContext {
  return {
    logger: mockLogger,
    sessionManager,
    getPrompt: jest.fn(),
    sampling: jest.fn(),
  } as any;
}

// Default session state
const defaultState: WorkflowState = {
  sessionId: 'test-session',
  workflow_state: {},
  metadata: {},
  completed_steps: [],
  current_step: null,
  errors: {},
  createdAt: new Date().toISOString(),
  updatedAt: new Date().toISOString(),
};

describe('Session Helpers', () => {
  describe('getSession', () => {
    it('should resolve existing session', async () => {
      // Mock existing session
      sessionManager.get.mockResolvedValue({
        ...defaultState,
        sessionId: 'existing-session'
      });
      
      const result = await getSession('existing-session', createContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.id).toBe('existing-session');
        expect(result.value.isNew).toBe(false);
        expect(result.value.state.sessionId).toBe('existing-session');
      }
    });

    it('should create new session when not found', async () => {
      // Mock session not found
      sessionManager.get.mockResolvedValue(null);
      sessionManager.create.mockResolvedValue({
        ...defaultState,
        sessionId: 'new-session'
      });
      
      const result = await getSession('new-session', createContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.id).toBe('new-session');
        expect(result.value.isNew).toBe(true);
        expect(sessionManager.create).toHaveBeenCalledWith('new-session');
      }
    });

    it('should generate random session ID when not provided', async () => {
      sessionManager.get.mockResolvedValue(null);
      sessionManager.create.mockImplementation(async (id) => ({
        ...defaultState,
        sessionId: id
      }));
      
      const result = await getSession(undefined, createContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.id).toBeTruthy();
        expect(result.value.id.length).toBeGreaterThan(0);
        expect(result.value.isNew).toBe(true);
      }
    });

    it('should return error when session manager not in context', async () => {
      const result = await getSession('test-session', undefined);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Session manager not available in context');
      }
    });
  });

  describe('completeStep', () => {
    it('should add step to completed steps', async () => {
      const existingState = {
        ...defaultState,
        sessionId: 'test-session',
        completed_steps: ['step1']
      };
      
      sessionManager.get.mockResolvedValue(existingState);
      sessionManager.update.mockResolvedValue(true);
      
      const result = await completeStep('test-session', 'step2', createContext());

      expect(result.ok).toBe(true);
      expect(sessionManager.update).toHaveBeenCalledWith(
        'test-session',
        expect.objectContaining({
          completed_steps: ['step1', 'step2']
        })
      );
    });

    it('should not duplicate steps', async () => {
      const existingState = {
        ...defaultState,
        sessionId: 'test-session',
        completed_steps: ['step1', 'step2']
      };
      
      sessionManager.get.mockResolvedValue(existingState);
      sessionManager.update.mockResolvedValue(true);
      
      const result = await completeStep('test-session', 'step2', createContext());

      expect(result.ok).toBe(true);
      expect(sessionManager.update).toHaveBeenCalledWith(
        'test-session',
        expect.objectContaining({
          completed_steps: ['step1', 'step2'] // No duplicate
        })
      );
    });

    it('should fail for non-existent session', async () => {
      sessionManager.get.mockResolvedValue(null);
      
      const result = await completeStep('missing-session', 'step1', createContext());

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Session missing-session not found');
      }
    });
  });

  describe('createSession', () => {
    it('should create new session with initial data', async () => {
      sessionManager.create.mockResolvedValue({
        ...defaultState,
        sessionId: 'new-session',
        workflow_state: { test: 'data' }
      });
      
      const result = await createSession('new-session', createContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.id).toBe('new-session');
        expect(result.value.state.sessionId).toBe('new-session');
      }
    });

    it('should generate ID if not provided', async () => {
      sessionManager.create.mockImplementation(async (id) => ({
        ...defaultState,
        sessionId: id
      }));
      
      const result = await createSession(undefined, createContext());

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.id).toBeTruthy();
      }
    });
  });

  describe('updateSession', () => {
    it('should update session with partial data', async () => {
      const existingState = {
        ...defaultState,
        sessionId: 'test-session',
        workflow_state: { existing: 'data' }
      };
      
      sessionManager.get.mockResolvedValue(existingState);
      sessionManager.update.mockResolvedValue(true);
      
      const result = await updateSession(
        'test-session',
        { new: 'data' },
        createContext()
      );

      expect(result.ok).toBe(true);
      expect(sessionManager.update).toHaveBeenCalledWith(
        'test-session',
        expect.objectContaining({
          new: 'data'
        })
      );
    });

    it('should handle update failure', async () => {
      sessionManager.get.mockResolvedValue(defaultState);
      sessionManager.update.mockRejectedValue(new Error('Update failed'));
      
      const result = await updateSession(
        'test-session',
        { test: 'data' },
        createContext()
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Failed to update session');
      }
    });

    it('should fail for non-existent session', async () => {
      sessionManager.get.mockResolvedValue(null);
      
      const result = await updateSession(
        'missing-session',
        { test: 'data' },
        createContext()
      );

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('Session missing-session not found');
      }
    });
  });
});