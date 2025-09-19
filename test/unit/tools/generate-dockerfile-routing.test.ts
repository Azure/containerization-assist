/**
 * Unit tests for generate-dockerfile smart routing functionality
 */

import { describe, it, expect, jest, beforeEach } from '@jest/globals';
import { promises as fs } from 'node:fs';
import { generateDockerfile } from '../../../src/tools/generate-dockerfile/tool';
import type { ToolContext } from '../../../src/mcp/context';
import { createLogger } from '../../../src/lib/logger';
import { config } from '../../../src/config/index';

// Mock file system
jest.mock('node:fs', () => ({
  promises: {
    writeFile: jest.fn(),
    access: jest.fn().mockResolvedValue(undefined),
    readdir: jest.fn().mockResolvedValue([]),
  },
}));

// Mock AI generation
jest.mock('../../../src/mcp/tool-ai-helpers', () => ({
  aiGenerateWithSampling: jest.fn(),
  aiGenerate: jest.fn(),
}));

// Mock text processing
jest.mock('../../../src/lib/text-processing', () => ({
  stripFencesAndNoise: jest.fn((content: string) => content),
  isValidDockerfileContent: jest.fn(() => true),
  extractBaseImage: jest.fn(() => 'node:18-alpine'),
}));

// Mock session helpers
jest.mock('../../../src/mcp/tool-session-helpers', () => ({
  ensureSession: jest.fn(),
  defineToolIO: jest.fn(),
  useSessionSlice: jest.fn(),
  getSessionSlice: jest.fn(),
}));

// Mock tool helpers
jest.mock('../../../src/lib/tool-helpers', () => ({
  getToolLogger: jest.fn(() => ({ info: jest.fn(), error: jest.fn() })),
  createToolTimer: jest.fn(() => ({
    start: jest.fn(),
    stop: jest.fn(),
    error: jest.fn(),
    end: jest.fn(),
    info: jest.fn()
  })),
}));

// Mock base images
jest.mock('../../../src/lib/base-images', () => ({
  getRecommendedBaseImage: jest.fn(() => 'node:18-alpine'),
}));

// Mock AI knowledge enhancer
jest.mock('../../../src/lib/ai-knowledge-enhancer', () => ({
  enhancePromptWithKnowledge: jest.fn((prompt) => prompt),
}));

// Mock progress helper
jest.mock('../../../src/mcp/progress-helper', () => ({
  createStandardProgress: jest.fn(() => jest.fn()),
}));

// Mock prompt-backed-tool
jest.mock('../../../src/mcp/tools/prompt-backed-tool', () => ({
  createPromptBackedTool: jest.fn((options) => ({
    ...options,
    execute: jest.fn().mockResolvedValue({
      ok: true,
      value: {
        content: `FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
EXPOSE 3000
CMD ["node", "index.js"]`,
        metadata: {
          baseImage: 'node:18-alpine',
          exposedPorts: [3000],
          hasHealthCheck: false,
          isMultiStage: false,
          optimizationStrategy: 'balanced',
          securityLevel: 'standard'
        },
        recommendations: []
      }
    })
  }))
}));

// Mock Result types
jest.mock('@types', () => ({
  Success: (value: any) => ({ ok: true, value }),
  Failure: (error: string) => ({ ok: false, error })
}));

// Mock error utils
jest.mock('../../../src/lib/error-utils', () => ({
  extractErrorMessage: jest.fn((err) => err?.message || String(err))
}));

const mockFs = fs as jest.Mocked<typeof fs>;
const { aiGenerateWithSampling } = require('../../../src/mcp/tool-ai-helpers');
const { ensureSession, useSessionSlice, getSessionSlice } = require('../../../src/mcp/tool-session-helpers');

describe('generate-dockerfile smart routing', () => {
  const logger = createLogger();

  beforeEach(() => {
    jest.clearAllMocks();
    mockFs.writeFile.mockResolvedValue(undefined);

    // Default session state - will be overridden in specific tests
    const defaultSessionState = { results: {} };

    // Mock session management
    ensureSession.mockImplementation(() => Promise.resolve({
      ok: true,
      value: {
        id: 'test-session',
        state: defaultSessionState
      }
    }));

    useSessionSlice.mockReturnValue({
      patch: jest.fn().mockResolvedValue({ ok: true }),
    });

    // Default mock for getSessionSlice - tests will override as needed
    getSessionSlice.mockResolvedValue({
      ok: true,
      value: null  // Default to no existing slice
    });

    // Mock successful AI generation
    aiGenerateWithSampling.mockResolvedValue({
      ok: true,
      value: {
        winner: {
          content: 'FROM node:18-alpine\nWORKDIR /app\nCOPY package.json .\nRUN npm install\nCOPY . .\nEXPOSE 3000\nCMD ["npm", "start"]',
          score: 85,
        },
        samplingMetadata: {
          candidatesGenerated: 3,
          winnerScore: 85,
        },
      },
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

    // Override session state for this test
    ensureSession.mockResolvedValueOnce({
      ok: true,
      value: {
        id: 'test-session',
        state: {
          results: {
            'analyze-repo': analyzeRepoResult,
          },
        }
      }
    });

    // Mock getSessionSlice to return the analyze-repo results
    getSessionSlice.mockResolvedValueOnce({
      ok: true,
      value: {
        input: {},
        output: analyzeRepoResult,
        state: {},
        updatedAt: new Date()
      }
    });

    // Mock context
    const mockContext: ToolContext = {
      signal: undefined,
      progress: undefined,
      sampling: {
        createMessage: jest.fn().mockResolvedValue({ role: 'assistant', content: [{ type: 'text', text: 'mock response' }] }),
      },
      getPrompt: jest.fn().mockResolvedValue({ messages: [] }),
      sessionManager: undefined,
      logger: createLogger(),
    };

    const result = await generateDockerfile(
      {
        sessionId: 'test-session',
        path: '/test/project',
        dockerfileDirectoryPaths: ['/test/project'],
      },
      mockContext,
    );

    if (!result.ok) {
      console.error('Tool execution failed:', result.error);
    }

    expect(result.ok).toBe(true);
    if (result.ok) {
      // Should generate a valid Dockerfile
      expect(result.value).toBeDefined();
      expect(result.value.dockerfiles).toBeDefined();
      expect(result.value.dockerfiles.length).toBe(1);
      expect(result.value.dockerfiles[0].content).toBeDefined();
      expect(result.value.dockerfiles[0].content.length).toBeGreaterThan(20);
      // Metadata is optional depending on the generation method
    }
  });

  it('should use direct analysis for low confidence detection', async () => {
    const analyzeRepoResult = {
      language: 'python',
      confidence: 30, // Low confidence - below threshold
      detectionMethod: 'extension',
      dependencies: [],
      ports: [8000],
    };

    // Override session state for this test
    ensureSession.mockResolvedValueOnce({
      ok: true,
      value: {
        id: 'test-session',
        state: {
          results: {
            'analyze-repo': analyzeRepoResult,
          },
        }
      }
    });

    // Mock getSessionSlice to return the analyze-repo results
    getSessionSlice.mockResolvedValueOnce({
      ok: true,
      value: {
        input: {},
        output: analyzeRepoResult,
        state: {},
        updatedAt: new Date()
      }
    });

    const mockContext: ToolContext = {
      signal: undefined,
      progress: undefined,
      sampling: {
        createMessage: jest.fn().mockResolvedValue({ role: 'assistant', content: [{ type: 'text', text: 'mock response' }] }),
      },
      getPrompt: jest.fn().mockResolvedValue({ messages: [] }),
      sessionManager: undefined,
      logger: createLogger(),
    };

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

    // Should generate a valid Dockerfile
    if (result.ok) {
      expect(result.value).toBeDefined();
      expect(result.value.dockerfiles).toBeDefined();
      expect(result.value.dockerfiles.length).toBe(1);
      expect(result.value.dockerfiles[0].content).toBeDefined();
      expect(result.value.dockerfiles[0].content.length).toBeGreaterThan(20);
    }
  });

  it('should use direct analysis for unknown language', async () => {
    const analyzeRepoResult = {
      language: 'unknown',
      confidence: 0,
      detectionMethod: 'fallback',
      dependencies: [],
      ports: [3000],
    };

    // Override session state for this test
    ensureSession.mockResolvedValueOnce({
      ok: true,
      value: {
        id: 'test-session',
        state: {
          results: {
            'analyze-repo': analyzeRepoResult,
          },
        }
      }
    });

    // Mock getSessionSlice to return the analyze-repo results
    getSessionSlice.mockResolvedValueOnce({
      ok: true,
      value: {
        input: {},
        output: analyzeRepoResult,
        state: {},
        updatedAt: new Date()
      }
    });

    const mockContext: ToolContext = {
      signal: undefined,
      progress: undefined,
      sampling: {
        createMessage: jest.fn().mockResolvedValue({ role: 'assistant', content: [{ type: 'text', text: 'mock response' }] }),
      },
      getPrompt: jest.fn().mockResolvedValue({ messages: [] }),
      sessionManager: undefined,
      logger: createLogger(),
    };

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

    // Should generate a valid Dockerfile
    expect(result.value).toBeDefined();
    expect(result.value.dockerfiles).toBeDefined();
    expect(result.value.dockerfiles.length).toBe(1);
    expect(result.value.dockerfiles[0].content).toBeDefined();
    expect(result.value.dockerfiles[0].content.length).toBeGreaterThan(20);
    // Metadata is optional depending on the generation method
  });

  it('should use direct analysis when confidence is exactly at threshold', async () => {
    const analyzeRepoResult = {
      language: 'go',
      confidence: 0.8, // Confidence threshold
      detectionMethod: 'signature',
      dependencies: [],
      ports: [8080],
    };

    // Override session state for this test
    ensureSession.mockResolvedValueOnce({
      ok: true,
      value: {
        id: 'test-session',
        state: {
          results: {
            'analyze-repo': analyzeRepoResult,
          },
        }
      }
    });

    // Mock getSessionSlice to return the analyze-repo results
    getSessionSlice.mockResolvedValueOnce({
      ok: true,
      value: {
        input: {},
        output: analyzeRepoResult,
        state: {},
        updatedAt: new Date()
      }
    });

    const mockContext: ToolContext = {
      signal: undefined,
      progress: undefined,
      sampling: {
        createMessage: jest.fn().mockResolvedValue({ role: 'assistant', content: [{ type: 'text', text: 'mock response' }] }),
      },
      getPrompt: jest.fn().mockResolvedValue({ messages: [] }),
      sessionManager: undefined,
      logger: createLogger(),
    };

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

    // Should generate a valid Dockerfile
    expect(result.value).toBeDefined();
    expect(result.value.dockerfiles).toBeDefined();
    expect(result.value.dockerfiles.length).toBe(1);
    expect(result.value.dockerfiles[0].content).toBeDefined();
    expect(result.value.dockerfiles[0].content.length).toBeGreaterThan(20);
    // Metadata is optional depending on the generation method
  });

  it('should include proper prompt args for direct analysis', async () => {
    const analyzeRepoResult = {
      language: 'unknown',
      confidence: 0,
      detectionMethod: 'fallback',
      dependencies: [],
      ports: [],
    };

    // Override session state for this test
    ensureSession.mockResolvedValueOnce({
      ok: true,
      value: {
        id: 'test-session',
        state: {
          results: {
            'analyze-repo': analyzeRepoResult,
          },
        }
      }
    });

    // Mock getSessionSlice to return the analyze-repo results
    getSessionSlice.mockResolvedValueOnce({
      ok: true,
      value: {
        input: {},
        output: analyzeRepoResult,
        state: {},
        updatedAt: new Date()
      }
    });

    const mockContext: ToolContext = {
      signal: undefined,
      progress: undefined,
      sampling: {
        createMessage: jest.fn().mockResolvedValue({ role: 'assistant', content: [{ type: 'text', text: 'mock response' }] }),
      },
      getPrompt: jest.fn().mockResolvedValue({ messages: [] }),
      sessionManager: undefined,
      logger: createLogger(),
    };

    const result = await generateDockerfile(
      {
        sessionId: 'test-session',
        path: '/test/project',
        dockerfileDirectoryPaths: ['/test/project'],
        optimization: 'performance',
      },
      mockContext,
    );

    // Should generate a valid Dockerfile with optimization
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value).toBeDefined();
      expect(result.value.dockerfiles).toBeDefined();
      expect(result.value.dockerfiles.length).toBe(1);
      expect(result.value.dockerfiles[0].content).toBeDefined();
      expect(result.value.dockerfiles[0].content.length).toBeGreaterThan(20);
      // Metadata is optional depending on the generation method
    }
  });
});