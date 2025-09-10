/**
 * Unit Tests: Fix Dockerfile Tool
 * Tests the fix dockerfile tool functionality with ToolContext
 */

import { jest } from '@jest/globals';
import { fixDockerfile as fixDockerfileTool } from '../../../src/tools/fix-dockerfile/tool';
import type { FixDockerfileParams } from '../../../src/tools/fix-dockerfile/schema';
import { createMockLogger, createSuccessResult, createFailureResult } from '../../__support__/utilities/mock-infrastructure';
import type { ToolContext, SamplingRequest, SamplingResponse, PromptWithMessages } from '../../../src/mcp/context/types';
import { promises as fs } from 'node:fs';

// Mock fs promises
jest.mock('node:fs', () => ({
  promises: {
    readFile: jest.fn(),
    writeFile: jest.fn(),
    access: jest.fn(),
  },
}));

// Mock lib modules
const mockSessionManager = {
  create: jest.fn().mockResolvedValue({
    sessionId: 'test-session-123',
    metadata: {},
    completed_steps: [],
    errors: {},
    current_step: null,
    createdAt: '2025-09-08T11:12:40.362Z',
    updatedAt: '2025-09-08T11:12:40.362Z'
  }),
  get: jest.fn(),
  update: jest.fn(),
};

const mockMCPHostAI = {
  analyzeContent: jest.fn(),
  fixContent: jest.fn(),
};

// Create mock ToolContext
function createMockToolContext(
  responses: Record<string, string> = {},
  shouldFail = false
): ToolContext {
  return {
    logger: createMockLogger(),
    progressReporter: jest.fn(),
    sampling: {
      createMessage: jest.fn().mockImplementation(async (request: SamplingRequest) => {
        if (shouldFail) {
          throw new Error('Sampling failed');
        }
        
        const response: SamplingResponse = {
          role: 'assistant',
          content: [{ 
            type: 'text', 
            text: responses.defaultResponse || `FROM node:18-alpine\nWORKDIR /app\nCOPY package*.json ./\nRUN npm ci --production\nCOPY . .\nUSER node\nCMD ["node", "server.js"]` 
          }],
          metadata: {
            model: 'test-model',
            usage: { inputTokens: 100, outputTokens: 200, totalTokens: 300 }
          }
        };
        return response;
      })
    },
    getPrompt: jest.fn().mockImplementation(async (name: string, args?: Record<string, unknown>) => {
      if (shouldFail) {
        throw new Error('Prompt not found');
      }
      
      const prompt: PromptWithMessages = {
        description: `Mock prompt for ${name}`,
        messages: [{
          role: 'user',
          content: [{ type: 'text', text: `Fix this dockerfile with args: ${JSON.stringify(args)}` }]
        }]
      };
      return prompt;
    }),
    signal: undefined,
    progress: undefined
  };
}

const mockTimer = {
  end: jest.fn(),
  error: jest.fn(),
};

jest.mock('@lib/session', () => ({
  createSessionManager: jest.fn(() => mockSessionManager),
}));

// Mock MCP helper modules
jest.mock('@mcp/tools/session-helpers');


// Mock the text processing utilities
jest.mock('@lib/text-processing', () => ({
  stripFencesAndNoise: jest.fn((text: string) => {
    // Simple mock that removes code fences
    return text.replace(/```[a-z]*\n?/gi, '').replace(/```$/g, '').trim();
  }),
  isValidDockerfileContent: jest.fn((content: string) => {
    return content.includes('FROM ');
  }),
}));

jest.mock('@lib/logger', () => ({
  createTimer: jest.fn(() => mockTimer),
  createLogger: jest.fn(() => createMockLogger()),
}));

describe('fixDockerfileTool', () => {
  let mockLogger: ReturnType<typeof createMockLogger>;
  let config: FixDockerfileParams;
  let mockGetSession: jest.Mock;
  let mockUpdateSession: jest.Mock;

  beforeEach(() => {
    mockLogger = createMockLogger();
    config = {
      sessionId: 'test-session-123',
      error: 'Failed to build Docker image',
      dockerfile: 'FROM node:latest\nCOPY . .\nRUN npm install\nCMD node server.js',
    };

    // Get mocked functions
    const sessionHelpers = require('@mcp/tools/session-helpers');
    mockGetSession = sessionHelpers.getSession = jest.fn();
    mockUpdateSession = sessionHelpers.updateSession = jest.fn();

    // Reset all mocks
    jest.clearAllMocks();
    mockSessionManager.update.mockResolvedValue(true);
    
    // Setup default session helper mocks
    mockGetSession.mockResolvedValue({
      ok: true,
      value: {
        id: 'test-session-123',
        state: {
          sessionId: 'test-session-123',
          analysis_result: { language: 'javascript', framework: 'express' },
          dockerfile_result: { content: 'FROM node:latest\nCOPY . .\nRUN npm install' },
          build_result: { error: 'npm install failed' },
          metadata: {},
          completed_steps: [],
        },
        isNew: false,
      },
    });
    mockUpdateSession.mockResolvedValue({ ok: true });
    // Mock AI responses (legacy - for fallback compatibility)
    mockMCPHostAI.isAvailable = jest.fn().mockReturnValue(true);
    mockMCPHostAI.submitPrompt = jest.fn().mockResolvedValue({
      ok: true,
      value: '```dockerfile\nFROM node:18-alpine\nWORKDIR /app\nCOPY package*.json ./\nRUN npm ci --production\nCOPY . .\nUSER node\nCMD ["node", "server.js"]\n```',
    });
  });

  describe('Successful Dockerfile fixing', () => {
    beforeEach(() => {
      mockSessionManager.get.mockResolvedValue({
        sessionId: 'test-session-123',
        results: {
          dockerfile_result: {
            content: 'FROM node:latest\nCOPY . .\nRUN npm install',
          },
          build_result: {
            error: 'npm install failed',
          },
        },
        analysis_result: {
          language: 'javascript',
          framework: 'express'
        },
        metadata: {},
      });
    });

    it('should successfully fix Dockerfile issues with AI context', async () => {
      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.dockerfile).toContain('FROM');
        expect(result.value.fixes).toBeDefined();
        expect(Array.isArray(result.value.fixes)).toBe(true);
      }
    });

    it('should successfully fix Dockerfile issues without context (fallback)', async () => {
      const result = await fixDockerfileTool(config, {} as any);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.dockerfile).toContain('FROM');
        expect(result.value.fixes).toBeDefined();
        expect(Array.isArray(result.value.fixes)).toBe(true);
      }
    });

    it('should fix Dockerfile from session with AI context', async () => {
      const mockContext = createMockToolContext();
      // Don't provide dockerfile in config, should use from session
      const sessionConfig = { sessionId: 'test-session-123' };
      
      const result = await fixDockerfileTool(sessionConfig, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        expect(result.value.dockerfile).toContain('FROM');
        expect(result.value.fixes).toBeDefined();
        expect(Array.isArray(result.value.fixes)).toBe(true);
      }
    });

    it('should update session with fix results', async () => {
      const mockContext = createMockToolContext();
      await fixDockerfileTool(config, mockContext);

      expect(mockUpdateSession).toHaveBeenCalledWith(
        'test-session-123',
        expect.objectContaining({
          dockerfile_result: expect.objectContaining({
            content: expect.any(String),
            path: './Dockerfile',
          }),
          completed_steps: expect.arrayContaining(['fix-dockerfile']),
          metadata: expect.objectContaining({
            dockerfile_fixed: true,
            dockerfile_fixes: expect.any(Array),
          }),
        }),
        expect.any(Object)  // context
      );
    });
  });


  describe('Error handling', () => {
    it('should return error when no Dockerfile found', async () => {
      mockSessionManager.get.mockResolvedValue({
        sessionId: 'test-session-123',
        results: {},
        metadata: {},
      });
      
      // Mock getSession to return session without dockerfile_result
      mockGetSession.mockResolvedValueOnce({
        ok: true,
        value: {
          id: 'test-session-123',
          state: {
            sessionId: 'test-session-123',
            // No dockerfile_result
            metadata: {},
            completed_steps: [],
          },
          isNew: false,
        },
      });
      
      const noDockerfileConfig = { sessionId: 'test-session-123' };
      const mockContext = createMockToolContext();

      const result = await fixDockerfileTool(noDockerfileConfig, mockContext);

      expect(result.ok).toBe(false);
      if (!result.ok) {
        expect(result.error).toContain('No Dockerfile found to fix. Provide dockerfile parameter or run generate-dockerfile tool first.');
      }
    });

    it('should handle AI failures with fallback', async () => {
      const mockContext = createMockToolContext({}, true); // Should fail
      
      const result = await fixDockerfileTool(config, mockContext);

      expect(result.ok).toBe(true);
      if (result.ok) {
        // Should fallback to rule-based fix
        expect(result.value.dockerfile).toContain('FROM');
        expect(result.value.fixes).toBeDefined();
      }
    });

    it('should work without context at all (legacy mode)', async () => {
      const result = await fixDockerfileTool(config, {} as any);

      expect(result.ok).toBe(true);
      if (result.ok) {
        // Should use rule-based fixes
        expect(result.value.dockerfile).toContain('FROM');
        expect(result.value.fixes).toBeDefined();
      }
    });
  });


  describe('Session management', () => {
    it('should create new session if not exists', async () => {
      // Mock getSession to simulate creating a new session
      mockGetSession.mockResolvedValueOnce({
        ok: true,
        value: {
          id: 'test-session-123',
          state: {
            sessionId: 'test-session-123',
            dockerfile_result: { content: 'FROM node:latest\nCOPY . .\nRUN npm install' },
            metadata: {},
            completed_steps: [],
          },
          isNew: true,
        },
      });

      const mockContext = createMockToolContext();
      const result = await fixDockerfileTool(config, mockContext);

      expect(result.ok).toBe(true);
      expect(mockGetSession).toHaveBeenCalledWith('test-session-123', mockContext);
    });
    
    it('should work with AI context and proper prompt integration', async () => {
      const mockContext = createMockToolContext();
      
      const result = await fixDockerfileTool(config, mockContext);
      
      expect(result.ok).toBe(true);
      expect(mockContext.getPrompt).toHaveBeenCalledWith('fix-dockerfile', expect.objectContaining({
        dockerfileContent: expect.any(String),
        buildError: 'Failed to build Docker image'
      }));
      expect(mockContext.sampling.createMessage).toHaveBeenCalledWith(expect.objectContaining({
        messages: expect.any(Array),
        includeContext: 'thisServer',
        modelPreferences: expect.objectContaining({
          hints: [{ name: 'code' }]
        }),
        maxTokens: 2048
      }));
    });
  });
});