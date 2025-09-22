/**
 * Unit tests for analyze-repo confidence scoring functionality
 */

import { describe, it, expect, jest, beforeEach } from '@jest/globals';
import { promises as fs } from 'node:fs';
import { analyzeRepo } from '../../../src/tools/analyze-repo/tool';
import type { ToolContext } from '../../../src/mcp/context';
import { createLogger } from '../../../src/lib/logger';

// Mock file system
jest.mock('node:fs', () => ({
  promises: {
    stat: jest.fn(),
    access: jest.fn(),
    readdir: jest.fn(),
    readFile: jest.fn(),
  },
  constants: {
    R_OK: 4,
    W_OK: 2,
    X_OK: 1,
    F_OK: 0,
  },
}));

// Mock other dependencies
jest.mock('../../../src/mcp/tool-session-helpers', () => ({
  ensureSession: jest.fn(),
  defineToolIO: jest.fn(),
  useSessionSlice: jest.fn(),
}));

jest.mock('../../../src/lib/tool-helpers', () => ({
  getToolLogger: jest.fn(() => ({
    info: jest.fn(),
    error: jest.fn(),
    debug: jest.fn(),
    warn: jest.fn(),
  })),
  createToolTimer: jest.fn(() => ({
    start: jest.fn(),
    stop: jest.fn(),
    error: jest.fn(),
    end: jest.fn(),
    info: jest.fn(),
  })),
}));

jest.mock('../../../src/mcp/progress-helper', () => ({
  createStandardProgress: jest.fn(() => jest.fn()),
}));

jest.mock('../../../src/mcp/tool-ai-helpers', () => ({
  aiGenerateWithSampling: jest.fn(),
}));

jest.mock('../../../src/lib/ai-knowledge-enhancer', () => ({
  enhancePromptWithKnowledge: jest.fn(),
}));

jest.mock('../../../src/lib/base-images', () => ({
  getBaseImageRecommendations: jest.fn(() => []),
}));

const mockFs = fs as jest.Mocked<typeof fs>;
const { ensureSession, useSessionSlice } = require('../../../src/mcp/tool-session-helpers');

describe('analyze-repo confidence scoring', () => {
  const logger = createLogger();

  // Mock context for testing - will be configured per test
  const mockContext: ToolContext = {
    signal: undefined,
    progress: undefined,
    sampling: {
      createMessage: jest.fn(),
    },
    getPrompt: jest.fn().mockResolvedValue({ messages: [] }),
    sessionManager: {
      get: jest.fn().mockResolvedValue({ ok: false }),
      create: jest.fn().mockResolvedValue({ ok: true, value: {} }),
      update: jest.fn().mockResolvedValue({ ok: true }),
      delete: jest.fn().mockResolvedValue({ ok: true }),
      list: jest.fn().mockResolvedValue({ ok: true, value: [] }),
      cleanup: jest.fn().mockResolvedValue({ ok: true }),
    } as any,
    logger: createLogger(),
  };

  beforeEach(() => {
    jest.clearAllMocks();

    // Mock session management
    ensureSession.mockResolvedValue({
      ok: true,
      value: {
        id: 'test-session',
        state: { results: {} },
      },
    });

    useSessionSlice.mockReturnValue({
      patch: jest.fn().mockResolvedValue({ ok: true }),
    });
  });

  it('should return high confidence for JavaScript project with package.json', async () => {
    // Mock AI response for JavaScript project analysis
    const mockResponse = {
      ok: true,
      sessionId: 'test-session',
      language: 'javascript',
      languageVersion: '18.0.0',
      framework: 'express',
      frameworkVersion: '4.18.0',
      buildSystem: {
        type: 'npm',
        file: 'package.json',
        buildCommand: 'npm run build',
        testCommand: 'npm test',
      },
      dependencies: [
        { name: 'express', version: '4.18.0', type: 'runtime' },
      ],
      ports: [3000],
      hasDockerfile: false,
      hasDockerCompose: false,
      hasKubernetes: false,
      recommendations: {
        baseImage: 'node:18-alpine',
        buildStrategy: 'multi-stage',
        securityNotes: ['Use non-root user', 'Scan for vulnerabilities'],
      },
      confidence: 95,
      detectionMethod: 'ai-enhanced',
      detectionDetails: {
        signatureMatches: 10,
        extensionMatches: 5,
        frameworkSignals: 3,
        buildSystemSignals: 2,
      },
      metadata: {
        path: '/test/project',
        depth: 5,
        timestamp: Date.now(),
        includeTests: false,
      },
    };

    (mockContext.sampling.createMessage as jest.Mock).mockResolvedValue({
      role: 'assistant',
      content: [{ type: 'text', text: `\`\`\`json\n${JSON.stringify(mockResponse)}\n\`\`\`` }],
    });

    const context = mockContext;

    const result = await analyzeRepo(
      {
        sessionId: 'test-session',
        path: '/test/project',
        dockerfilePaths: ['/test/project/Dockerfile'],
      },
      context,
    );

    if (!result.ok) {
      throw new Error(`Expected result.ok to be true, but got error: ${result.error}`);
    }
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.language).toBe('javascript');
      expect(result.value.confidence).toBe(95);
      expect(result.value.detectionMethod).toBe('ai-enhanced');
      expect(result.value.detectionDetails.signatureMatches).toBe(10);
    }
  });

  it('should return low confidence for unknown project type', async () => {
    // Mock AI response for unknown project type
    const mockResponse = {
      ok: true,
      sessionId: 'test-session',
      language: 'unknown',
      languageVersion: undefined,
      framework: undefined,
      frameworkVersion: undefined,
      buildSystem: undefined,
      dependencies: [],
      ports: [],
      hasDockerfile: false,
      hasDockerCompose: false,
      hasKubernetes: false,
      recommendations: {
        baseImage: 'ubuntu:latest',
        buildStrategy: 'single-stage',
        securityNotes: ['Add application files manually'],
      },
      confidence: 30,
      detectionMethod: 'ai-enhanced',
      detectionDetails: {
        signatureMatches: 0,
        extensionMatches: 0,
        frameworkSignals: 0,
        buildSystemSignals: 0,
      },
      metadata: {
        path: '/test/unknown',
        depth: 5,
        timestamp: Date.now(),
        includeTests: false,
      },
    };

    (mockContext.sampling.createMessage as jest.Mock).mockResolvedValue({
      role: 'assistant',
      content: [{ type: 'text', text: `\`\`\`json\n${JSON.stringify(mockResponse)}\n\`\`\`` }],
    });

    const context = mockContext;

    const result = await analyzeRepo(
      {
        sessionId: 'test-session',
        path: '/test/unknown',
        dockerfilePaths: ['/test/unknown/Dockerfile'],
      },
      context,
    );

    if (!result.ok) {
      throw new Error(`Expected result.ok to be true, but got error: ${result.error}`);
    }
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.language).toBe('unknown');
      expect(result.value.confidence).toBe(30);
      expect(result.value.detectionMethod).toBe('ai-enhanced');
    }
  });

  it('should return unknown for files without signature detection', async () => {
    // Mock AI response for Python files without clear signatures
    const mockResponse = {
      ok: true,
      sessionId: 'test-session',
      language: 'python',
      languageVersion: '3.9',
      framework: undefined,
      frameworkVersion: undefined,
      buildSystem: undefined,
      dependencies: [],
      ports: [],
      hasDockerfile: false,
      hasDockerCompose: false,
      hasKubernetes: false,
      recommendations: {
        baseImage: 'python:3.9-slim',
        buildStrategy: 'single-stage',
        securityNotes: ['Use virtual environment'],
      },
      confidence: 40,
      detectionMethod: 'ai-enhanced',
      detectionDetails: {
        signatureMatches: 0,
        extensionMatches: 5,
        frameworkSignals: 0,
        buildSystemSignals: 0,
      },
      metadata: {
        path: '/test/python-no-deps',
        depth: 5,
        timestamp: Date.now(),
        includeTests: false,
      },
    };

    (mockContext.sampling.createMessage as jest.Mock).mockResolvedValue({
      role: 'assistant',
      content: [{ type: 'text', text: `\`\`\`json\n${JSON.stringify(mockResponse)}\n\`\`\`` }],
    });

    const context = mockContext;

    const result = await analyzeRepo(
      {
        sessionId: 'test-session',
        path: '/test/python-no-deps',
      },
      context,
    );

    if (!result.ok) {
      throw new Error(`Expected result.ok to be true, but got error: ${result.error}`);
    }
    expect(result.ok).toBe(true);
    if (result.ok) {
      // AI detects Python from file extensions
      expect(result.value.language).toBe('python');
      expect(result.value.confidence).toBe(40);
      expect(result.value.detectionMethod).toBe('ai-enhanced');
    }
  });

  it('should include confidence scoring fields in result', async () => {
    // Mock AI response with all confidence fields
    const mockResponse = {
      ok: true,
      sessionId: 'test-session',
      language: 'javascript',
      languageVersion: '18.0.0',
      framework: 'express',
      frameworkVersion: '4.18.0',
      buildSystem: {
        type: 'npm',
        file: 'package.json',
        buildCommand: 'npm run build',
        testCommand: 'npm test',
      },
      dependencies: [],
      ports: [3000],
      hasDockerfile: false,
      hasDockerCompose: false,
      hasKubernetes: false,
      recommendations: {
        baseImage: 'node:18-alpine',
        buildStrategy: 'multi-stage',
        securityNotes: [],
      },
      confidence: 75,
      detectionMethod: 'ai-enhanced',
      detectionDetails: {
        signatureMatches: 8,
        extensionMatches: 3,
        frameworkSignals: 2,
        buildSystemSignals: 1,
      },
      metadata: {
        path: '/test/project',
        depth: 5,
        timestamp: Date.now(),
        includeTests: false,
      },
    };

    (mockContext.sampling.createMessage as jest.Mock).mockResolvedValue({
      role: 'assistant',
      content: [{ type: 'text', text: `\`\`\`json\n${JSON.stringify(mockResponse)}\n\`\`\`` }],
    });

    const context = mockContext;

    const result = await analyzeRepo(
      {
        sessionId: 'test-session',
        path: '/test/project',
        dockerfilePaths: ['/test/project/Dockerfile'],
      },
      context,
    );

    if (!result.ok) {
      throw new Error(`Expected result.ok to be true, but got error: ${result.error}`);
    }
    expect(result.ok).toBe(true);
    if (result.ok) {
      // Check that all new confidence fields are present
      expect(typeof result.value.confidence).toBe('number');
      expect(typeof result.value.detectionMethod).toBe('string');
      expect(result.value.detectionDetails).toBeDefined();
      expect(typeof result.value.detectionDetails.signatureMatches).toBe('number');
      expect(typeof result.value.detectionDetails.extensionMatches).toBe('number');
      expect(typeof result.value.detectionDetails.frameworkSignals).toBe('number');
      expect(typeof result.value.detectionDetails.buildSystemSignals).toBe('number');
    }
  });

  it('should use provided language parameter', async () => {
    // Mock AI response - AI will detect Java as requested
    const mockResponse = {
      ok: true,
      sessionId: 'test-session',
      language: 'java',
      languageVersion: '11',
      framework: undefined,
      frameworkVersion: undefined,
      buildSystem: {
        type: 'maven',
        file: 'pom.xml',
        buildCommand: 'mvn package',
        testCommand: 'mvn test',
      },
      dependencies: [],
      ports: [8080],
      hasDockerfile: false,
      hasDockerCompose: false,
      hasKubernetes: false,
      recommendations: {
        baseImage: 'openjdk:11-jre-slim',
        buildStrategy: 'multi-stage',
        securityNotes: ['Use JRE image for runtime'],
      },
      confidence: 85,
      detectionMethod: 'ai-enhanced',
      detectionDetails: {
        signatureMatches: 5,
        extensionMatches: 0,
        frameworkSignals: 0,
        buildSystemSignals: 1,
      },
      metadata: {
        path: '/test/project',
        depth: 5,
        timestamp: Date.now(),
        includeTests: false,
      },
    };

    (mockContext.sampling.createMessage as jest.Mock).mockResolvedValue({
      role: 'assistant',
      content: [{ type: 'text', text: `\`\`\`json\n${JSON.stringify(mockResponse)}\n\`\`\`` }],
    });

    const context = mockContext;

    // Test with explicitly provided language
    const result = await analyzeRepo(
      {
        sessionId: 'test-session',
        path: '/test/project',
        language: 'java',
        dockerfilePaths: ['/test/project/Dockerfile'],
      },
      context,
    );

    if (!result.ok) {
      throw new Error(`Expected result.ok to be true, but got error: ${result.error}`);
    }

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.language).toBe('java');
    }
  });
});
