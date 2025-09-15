/**
 * Mock tools for router integration testing
 */

import { Success, Failure, type Result } from '@types';
import { z } from 'zod';

export interface MockToolResult {
  tool: string;
  executed: boolean;
  params: Record<string, unknown>;
  timestamp: Date;
}

// Track execution order for verification
export const executionLog: MockToolResult[] = [];

// Reset execution log between tests
export const resetExecutionLog = () => {
  executionLog.length = 0;
};

// Mock tool handlers
export const mockAnalyzeRepo = {
  name: 'analyze_repo',
  schema: z.object({
    path: z.string().default('.'),
    sessionId: z.string().optional(),
  }),
  handler: async (params: Record<string, unknown>, _context?: any): Promise<Result<unknown>> => {
    executionLog.push({
      tool: 'analyze_repo',
      executed: true,
      params,
      timestamp: new Date(),
    });
    return Success({
      framework: 'node',
      packageManager: 'npm',
      entrypoint: 'src/index.ts',
    });
  },
};

export const mockResolveBaseImages = {
  name: 'resolve_base_images',
  schema: z.object({
    framework: z.string().optional(),
    version: z.string().optional(),
    sessionId: z.string().optional(),
  }),
  handler: async (params: Record<string, unknown>, _context?: any): Promise<Result<unknown>> => {
    executionLog.push({
      tool: 'resolve_base_images',
      executed: true,
      params,
      timestamp: new Date(),
    });
    return Success({
      baseImage: 'node:18-alpine',
      alternatives: ['node:18', 'node:18-slim'],
    });
  },
};

export const mockGenerateDockerfile = {
  name: 'generate_dockerfile',
  schema: z.object({
    path: z.string().default('.'),
    framework: z.string().optional(),
    baseImage: z.string().optional(),
    sessionId: z.string().optional(),
  }),
  handler: async (params: Record<string, unknown>, _context?: any): Promise<Result<unknown>> => {
    executionLog.push({
      tool: 'generate_dockerfile',
      executed: true,
      params,
      timestamp: new Date(),
    });
    return Success({
      dockerfilePath: './Dockerfile',
      content: 'FROM node:18-alpine\nWORKDIR /app\nCOPY . .\nCMD ["node", "src/index.js"]',
    });
  },
};

export const mockBuildImage = {
  name: 'build_image',
  schema: z.object({
    dockerfilePath: z.string().default('./Dockerfile'),
    imageName: z.string(),
    tag: z.string().default('latest'),
    sessionId: z.string().optional(),
  }),
  handler: async (params: Record<string, unknown>, _context?: any): Promise<Result<unknown>> => {
    executionLog.push({
      tool: 'build_image',
      executed: true,
      params,
      timestamp: new Date(),
    });
    return Success({
      imageId: 'sha256:abc123',
      imageName: params.imageName || 'test-app',
      tag: params.tag || 'latest',
    });
  },
};

export const mockPushImage = {
  name: 'push_image',
  schema: z.object({
    imageId: z.string(),
    registry: z.string().optional(),
    sessionId: z.string().optional(),
  }),
  handler: async (params: Record<string, unknown>, _context?: any): Promise<Result<unknown>> => {
    executionLog.push({
      tool: 'push_image',
      executed: true,
      params,
      timestamp: new Date(),
    });
    return Success({
      pushed: true,
      registry: params.registry || 'docker.io',
      imageId: params.imageId,
    });
  },
};

export const mockScanImage = {
  name: 'scan',
  schema: z.object({
    imageId: z.string(),
    sessionId: z.string().optional(),
  }),
  handler: async (params: Record<string, unknown>, _context?: any): Promise<Result<unknown>> => {
    executionLog.push({
      tool: 'scan',
      executed: true,
      params,
      timestamp: new Date(),
    });
    return Success({
      vulnerabilities: {
        critical: 0,
        high: 1,
        medium: 3,
        low: 5,
      },
      scanned: true,
    });
  },
};

export const mockPrepareCluster = {
  name: 'prepare_cluster',
  schema: z.object({
    context: z.string().optional(),
    sessionId: z.string().optional(),
  }),
  handler: async (params: Record<string, unknown>, _context?: any): Promise<Result<unknown>> => {
    executionLog.push({
      tool: 'prepare_cluster',
      executed: true,
      params,
      timestamp: new Date(),
    });
    return Success({
      clusterReady: true,
      context: params.context || 'default',
    });
  },
};

export const mockGenerateK8sManifests = {
  name: 'generate_k8s_manifests',
  schema: z.object({
    appName: z.string(),
    imageName: z.string(),
    sessionId: z.string().optional(),
  }),
  handler: async (params: Record<string, unknown>, _context?: any): Promise<Result<unknown>> => {
    executionLog.push({
      tool: 'generate_k8s_manifests',
      executed: true,
      params,
      timestamp: new Date(),
    });
    return Success({
      manifests: ['deployment.yaml', 'service.yaml'],
      path: './k8s',
    });
  },
};

export const mockDeploy = {
  name: 'deploy',
  schema: z.object({
    manifestPath: z.string(),
    namespace: z.string().default('default'),
    sessionId: z.string().optional(),
  }),
  handler: async (params: Record<string, unknown>, _context?: any): Promise<Result<unknown>> => {
    executionLog.push({
      tool: 'deploy',
      executed: true,
      params,
      timestamp: new Date(),
    });
    return Success({
      deployed: true,
      namespace: params.namespace || 'default',
    });
  },
};

// Tool that can fail for error testing
export const mockFailingTool = {
  name: 'failing-tool',
  schema: z.object({
    shouldFail: z.boolean().default(true),
    sessionId: z.string().optional(),
  }),
  handler: async (params: Record<string, unknown>, _context?: any): Promise<Result<unknown>> => {
    executionLog.push({
      tool: 'failing-tool',
      executed: true,
      params,
      timestamp: new Date(),
    });

    if (params.shouldFail !== false) {
      return Failure('Tool failed as expected', { recoverable: true });
    }

    return Success({ failed: false });
  },
};

// Create tools map for router
export const createMockToolsMap = () => {
  const tools = new Map();

  tools.set('analyze_repo', mockAnalyzeRepo);
  tools.set('resolve_base_images', mockResolveBaseImages);
  tools.set('generate_dockerfile', mockGenerateDockerfile);
  tools.set('build_image', mockBuildImage);
  tools.set('push_image', mockPushImage);
  tools.set('scan', mockScanImage);
  tools.set('prepare_cluster', mockPrepareCluster);
  tools.set('generate_k8s_manifests', mockGenerateK8sManifests);
  tools.set('deploy', mockDeploy);
  tools.set('failing-tool', mockFailingTool);

  return tools;
};