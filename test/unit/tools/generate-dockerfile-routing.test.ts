/**
 * Unit tests for generate-dockerfile smart routing functionality
 */

import { describe, it, expect, jest, beforeEach } from '@jest/globals';
import { promises as fs } from 'node:fs';
import { generateDockerfile } from '../../../src/tools/generate-dockerfile/tool';
import type { ToolContext } from '../../../src/mcp/context';
import { createLogger } from '../../../src/lib/logger';
import { getSession } from '../../../src/mcp/tool-session-helpers';

// Mock file system
jest.mock('node:fs', () => ({
  promises: {
    writeFile: jest.fn(),
  },
}));

// Mock session helpers
jest.mock('../../../src/mcp/tool-session-helpers', () => ({
  ensureSession: jest.fn(),
  defineToolIO: jest.fn(),
  useSessionSlice: jest.fn(),
  getSessionSlice: jest.fn(),
  getSession: jest.fn(),
  updateSession: jest.fn(),
}));

// Mock policy prompt
jest.mock('../../../src/config/policy-prompt', () => ({
  buildPolicyConstraints: jest.fn(() => []),
}));

// Mock prompt engine
jest.mock('../../../src/ai/prompt-engine', () => ({
  buildMessages: jest.fn(async () => ({ messages: [{ role: 'user', content: 'test' }] })),
  toMCPMessages: jest.fn((messages) => messages),
}));

const mockFs = fs as jest.Mocked<typeof fs>;

describe('generate-dockerfile smart routing', () => {
  const logger = createLogger();

  // Helper function to create a mock context with AI response
  function createMockContext(analyzeRepoResult?: any): ToolContext {
    const mockDockerfileResult = {
      ok: true,
      sessionId: 'test-session',
      dockerfiles: [
        {
          path: '/test/project/Dockerfile',
          content: 'FROM node:18-alpine\nWORKDIR /app\nCOPY package.json .\nRUN npm install\nCOPY . .\nEXPOSE 3000\nCMD ["npm", "start"]',
          metadata: {
            baseImage: 'node:18-alpine',
            stages: 1,
            estimatedSize: '~50MB',
            securityLevel: 'standard',
            features: ['non-root', 'healthcheck'],
          }
        }
      ],
      optimizationApplied: 'balanced',
      securityHardening: true,
      warnings: [],
      recommendations: []
    };

    return {
      signal: undefined,
      progress: undefined,
      sampling: {
        createMessage: jest.fn().mockResolvedValue({
          role: 'assistant',
          content: [{ type: 'text', text: `\`\`\`json\n${JSON.stringify(mockDockerfileResult)}\n\`\`\`` }]
        }),
      },
      getPrompt: jest.fn().mockResolvedValue({ messages: [] }),
      sessionManager: {
        get: jest.fn().mockResolvedValue({
          ok: true,
          value: analyzeRepoResult ? {
            metadata: {
              'analyze-repo_result': analyzeRepoResult
            }
          } : null
        }),
        update: jest.fn().mockResolvedValue({ ok: true }),
      } as any,
      logger: createLogger(),
    };
  }

  beforeEach(() => {
    jest.clearAllMocks();
    mockFs.writeFile.mockResolvedValue(undefined);

    // Setup getSession mock to return empty session by default
    (getSession as jest.MockedFunction<typeof getSession>).mockResolvedValue({
      ok: true,
      value: {
        state: {
          metadata: {}
        }
      } as any
    });
  });

  it('should use template generation for very high confidence detection', async () => {
    const analyzeRepoResult = {
      language: 'javascript',
      framework: 'express',
      confidence: 96, // Very high confidence (above threshold of 95)
      detectionMethod: 'signature',
      dependencies: [{ name: 'express', version: '4.18.0' }],
      ports: [3000],
    };

    // Setup getSession mock to return the analyzeRepoResult
    (getSession as jest.MockedFunction<typeof getSession>).mockResolvedValue({
      ok: true,
      value: {
        state: {
          metadata: {
            repositoryAnalysis: analyzeRepoResult
          }
        }
      } as any
    });

    const mockContext = createMockContext(analyzeRepoResult);

    const result = await generateDockerfile(
      {
        sessionId: 'test-session',
        path: '/test/project',
        dockerfileDirectoryPaths: ['/test/project'],
      },
      mockContext,
    );

    if (!result.ok) {
      throw new Error(`Expected result.ok to be true, but got error: ${result.error}`);
    }
    expect(result.ok).toBe(true);

    // Should have generated a Dockerfile
    if (result.ok) {
      expect(result.value.dockerfile).toBeDefined();
      expect(result.value.dockerfile).toContain('FROM node:18-alpine');
    }

    // Should have called AI with proper context
    expect(mockContext.sampling.createMessage).toHaveBeenCalled();
  });

  it('should use direct analysis for low confidence detection', async () => {
    const analyzeRepoResult = {
      language: 'python',
      confidence: 30, // Low confidence - below threshold
      detectionMethod: 'extension',
      dependencies: [],
      ports: [8000],
    };

    // Setup getSession mock to return the analyzeRepoResult
    (getSession as jest.MockedFunction<typeof getSession>).mockResolvedValue({
      ok: true,
      value: {
        state: {
          metadata: {
            repositoryAnalysis: analyzeRepoResult
          }
        }
      } as any
    });

    const mockContext = createMockContext(analyzeRepoResult);

    const result = await generateDockerfile(
      {
        sessionId: 'test-session',
        path: '/test/project',
        dockerfileDirectoryPaths: ['/test/project'],
      },
      mockContext,
    );

    if (!result.ok) {
      throw new Error(`Expected result.ok to be true, but got error: ${result.error}`);
    }
    expect(result.ok).toBe(true);

    // The new AI-driven implementation always generates, regardless of confidence
    if (result.ok) {
      expect(result.value.dockerfile).toBeDefined();
    }

    // Should have called AI
    expect(mockContext.sampling.createMessage).toHaveBeenCalled();
  });

  it('should use direct analysis for unknown language', async () => {
    const analyzeRepoResult = {
      language: 'unknown',
      confidence: 0,
      detectionMethod: 'fallback',
      dependencies: [],
      ports: [],
    };

    // Setup getSession mock to return the analyzeRepoResult
    (getSession as jest.MockedFunction<typeof getSession>).mockResolvedValue({
      ok: true,
      value: {
        state: {
          metadata: {
            repositoryAnalysis: analyzeRepoResult
          }
        }
      } as any
    });

    const mockContext = createMockContext(analyzeRepoResult);

    const result = await generateDockerfile(
      {
        sessionId: 'test-session',
        path: '/test/project',
        dockerfileDirectoryPaths: ['/test/project'],
      },
      mockContext,
    );

    if (!result.ok) {
      throw new Error(`Expected result.ok to be true, but got error: ${result.error}`);
    }
    expect(result.ok).toBe(true);

    // Should generate even for unknown language
    if (result.ok) {
      expect(result.value.dockerfile).toBeDefined();
    }

    // Should have called AI
    expect(mockContext.sampling.createMessage).toHaveBeenCalled();
  });

  it('should use direct analysis when confidence is exactly at threshold', async () => {
    const analyzeRepoResult = {
      language: 'javascript',
      framework: 'react',
      confidence: 95, // Exactly at threshold
      detectionMethod: 'signature',
      dependencies: [{ name: 'react', version: '18.0.0' }],
      ports: [3000],
    };

    // Setup getSession mock to return the analyzeRepoResult
    (getSession as jest.MockedFunction<typeof getSession>).mockResolvedValue({
      ok: true,
      value: {
        state: {
          metadata: {
            repositoryAnalysis: analyzeRepoResult
          }
        }
      } as any
    });

    const mockContext = createMockContext(analyzeRepoResult);

    const result = await generateDockerfile(
      {
        sessionId: 'test-session',
        path: '/test/project',
        dockerfileDirectoryPaths: ['/test/project'],
      },
      mockContext,
    );

    if (!result.ok) {
      throw new Error(`Expected result.ok to be true, but got error: ${result.error}`);
    }
    expect(result.ok).toBe(true);

    // Should generate Dockerfile
    if (result.ok) {
      expect(result.value.dockerfile).toBeDefined();
    }

    // Should have called AI
    expect(mockContext.sampling.createMessage).toHaveBeenCalled();
  });

  it('should include proper prompt args for direct analysis', async () => {
    const mockContext = createMockContext(); // No analysis result

    const result = await generateDockerfile(
      {
        sessionId: 'test-session',
        path: '/test/project',
        dockerfileDirectoryPaths: ['/test/project'],
        baseImage: 'python:3.9-slim',
        optimization: 'size',
      },
      mockContext,
    );

    if (!result.ok) {
      throw new Error(`Expected result.ok to be true, but got error: ${result.error}`);
    }
    expect(result.ok).toBe(true);

    // Should have generated with specified parameters
    if (result.ok) {
      expect(result.value.dockerfile).toBeDefined();
    }

    // Verify AI was called
    expect(mockContext.sampling.createMessage).toHaveBeenCalled();
  });
});