// Mock implementations for independent development
// These will be replaced with real implementations

import { Result, Success } from "../../src/types"
import type { Logger } from 'pino';
import type {
  IntelligentTool,
  ToolResult,
  Candidate,
  ScoredCandidate,
  GenerationContext,
} from './types.js';
import type { Resource } from '../../resources/types.js';
import type { ProgressNotifier } from '../../mcp/events/types.js';
import type {
  CandidateGenerator,
  CandidateScorer,
  WinnerSelector,
} from '../../lib/sampling.js';

// Mock Resource Manager (MCP dependency) - implements interface methods only
export const createMockResourceManager = (logger: Logger): ResourceManager => ({
  async publish(uri: string, _content: unknown, ttl?: number): Promise<Result<string>> {
    logger.debug({ uri, ttl }, 'Mock: Publishing resource');
    return Success(uri);
  },

  async read(uri: string): Promise<Result<Resource | null>> {
    logger.debug({ uri }, 'Mock: Reading resource');
    const resource: Resource = {
      uri,
      content: { mockContent: `Content for ${uri}`, timestamp: new Date() },
      mimeType: 'application/json',
      createdAt: new Date(),
    };
    return Success(resource);
  },

  async invalidate(pattern: string): Promise<Result<void>> {
    logger.debug({ pattern }, 'Mock: Invalidating resources');
    return Success(undefined);
  },

  async list(pattern: string): Promise<Result<string[]>> {
    logger.debug({ pattern }, 'Mock: Listing resources');
    return Success([`resource://mock/${pattern}/1`, `resource://mock/${pattern}/2`]);
  },

  async cleanup(): Promise<Result<void>> {
    logger.debug('Mock: Cleaning up old resources');
    return Success(undefined);
  },

  async getMetadata(uri: string): Promise<Result<Omit<Resource, 'content'> | null>> {
    logger.debug({ uri }, 'Mock: Getting resource metadata');
    const metadata = {
      uri,
      mimeType: 'application/json',
      createdAt: new Date(),
    };
    return Success(metadata);
  },
});

// Mock Progress Notifier (MCP dependency)
export const createMockProgressNotifier = (logger: Logger): ProgressNotifier => ({
  notifyProgress(progress: { token: string; value: number; message?: string }): void {
    logger.info({
      token: progress.token,
      progress: progress.value,
      message: progress.message,
    }, 'Mock: Progress notification');
  },

  notifyComplete(token: string, _result?: unknown): void {
    logger.info({ token }, 'Mock: Progress complete');
  },

  notifyError(token: string, error: string): void {
    logger.error({ token, error }, 'Mock: Progress error');
  },

  subscribe(_callback: (event: any) => void): () => void {
    logger.debug('Mock: Progress subscription created');
    return () => logger.debug('Mock: Progress subscription removed');
  },

  generateToken(): string {
    return `mock-token-${Date.now()}`;
  },
});

// Mock Dockerfile Candidate Generator (sampling dependency)
export const createMockDockerfileCandidateGenerator = (
  logger: Logger,
): CandidateGenerator<string> => ({
  name: 'mock-dockerfile-generator',
  supportedTypes: ['dockerfile'],

  async generate(context: GenerationContext, count = 3): Promise<Result<Candidate<string>[]>> {
    logger.debug({ context, count }, 'Mock: Generating Dockerfile candidates');

    const candidates: Candidate<string>[] = [];

    for (let i = 0; i < count; i++) {
      candidates.push({
        id: `dockerfile_candidate_${i}`,
        content: `# Mock Dockerfile Candidate ${i}
FROM node:18-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
EXPOSE 3000
CMD ["npm", "start"]
# Strategy: ${i === 0 ? 'security-optimized' : i === 1 ? 'performance-optimized' : 'development-friendly'}`,
        metadata: {
          strategy: i === 0 ? 'security-optimized' : i === 1 ? 'performance-optimized' : 'development-friendly',
          source: 'mock-generator',
          confidence: 0.8,
          estimatedSize: 150 + i * 50,
        },
        generatedAt: new Date(),
      });
    }

    return Success(candidates);
  },

  async validate(_candidate: Candidate<string>): Promise<Result<boolean>> {
    return Success(true);
  },
});

// Mock Candidate Scorer (sampling dependency)
export const createMockCandidateScorer = <T>(logger: Logger): CandidateScorer<T> => ({
  name: 'mock-scorer',
  weights: { security: 0.4, performance: 0.3, standards: 0.2, maintainability: 0.1 },

  async score(candidates: Candidate<T>[]): Promise<Result<ScoredCandidate<T>[]>> {
    logger.debug({ candidateCount: candidates.length }, 'Mock: Scoring candidates');

    const scoredCandidates = candidates.map((candidate, index) => {
      const baseScore = 70 + (index * 10) + Math.random() * 10;
      const scoreBreakdown = {
        security: Math.max(0, baseScore - 10 + Math.random() * 20),
        performance: Math.max(0, baseScore - 5 + Math.random() * 15),
        standards: Math.max(0, baseScore + Math.random() * 10),
        maintainability: Math.max(0, baseScore - 15 + Math.random() * 25),
      };

      return {
        ...candidate,
        score: Math.min(100, Object.values(scoreBreakdown).reduce((a, b) => a + b, 0) / 4),
        scoreBreakdown,
        rank: index,
        rationale: `Mock scoring: Strategy ${candidate.metadata?.strategy} selected based on ${
          scoreBreakdown.security > 80 ? 'security excellence' :
          scoreBreakdown.performance > 80 ? 'performance optimization' :
          'balanced approach'
        }`,
      };
    });

    return Success(scoredCandidates);
  },

  updateWeights(newWeights: Record<string, number>): void {
    logger.debug({ newWeights }, 'Mock: Updating scoring weights');
    Object.assign(this.weights, newWeights);
  },
});

// Mock Winner Selector (sampling dependency)
export const createMockWinnerSelector = <T>(logger: Logger): WinnerSelector<T> => ({
  strategy: 'highest-score',

  select(scored: ScoredCandidate<T>[]): Result<ScoredCandidate<T>> {
    const winner = scored.reduce((best, current) =>
      current.score > best.score ? current : best,
    );

    logger.info({
      winnerId: winner.id,
      winnerScore: winner.score,
      totalCandidates: scored.length,
    }, 'Mock: Selected winner');

    return Success(winner);
  },

  selectTop(scored: ScoredCandidate<T>[], count: number): Result<ScoredCandidate<T>[]> {
    const sorted = scored.sort((a, b) => b.score - a.score);
    const top = sorted.slice(0, count);
    logger.debug({ count, selected: top.length }, 'Mock: Selected top candidates');
    return Success(top);
  },
});

// Mock Enhanced Tools (workflow dependency)
export const createMockIntelligentTools = (logger: Logger): Record<string, IntelligentTool> => ({
  analyze_repository: {
    name: 'analyze_repository',
    supportsSampling: false,
    async execute(args: Record<string, unknown>): Promise<Result<ToolResult>> {
      logger.info({ args }, 'Mock: Analyzing repository');

      await new Promise(resolve => setTimeout(resolve, 2000)); // Simulate work

      return Success({
        ok: true,
        content: {
          language: 'javascript',
          framework: 'express',
          packageManager: 'npm',
          buildSystem: 'npm',
          hasDockerfile: false,
          hasTests: true,
          dependencies: ['express', 'cors', 'helmet'],
          recommendedStrategy: 'multi-stage-node',
        },
        resources: {
          summary: 'resource://analysis/summary',
          dependencies: 'resource://analysis/dependencies',
          recommendations: 'resource://analysis/recommendations',
        },
        metadata: {
          analysisTime: 2.1,
          confidence: 0.95,
        },
      });
    },
  },

  generate_dockerfile: {
    name: 'generate_dockerfile',
    supportsSampling: true,
    samplingConfig: {
      maxCandidates: 5,
      scoringWeights: {
        security: 0.4,
        performance: 0.25,
        standards: 0.2,
        maintainability: 0.15,
      },
    },
    async execute(args: Record<string, unknown>): Promise<Result<ToolResult>> {
      logger.info({ args }, 'Mock: Generating Dockerfile');

      const useSampling = args.useSampling as boolean;
      await new Promise(resolve => setTimeout(resolve, useSampling ? 5000 : 1000));

      if (useSampling) {
        return Success({
          ok: true,
          content: {
            winner: 'resource://dockerfile/winner',
            candidates: ['resource://dockerfile/candidate_0', 'resource://dockerfile/candidate_1', 'resource://dockerfile/candidate_2'],
            candidateCount: 3,
            winnerScore: 87.5,
          },
          resources: {
            winner: 'resource://dockerfile/winner',
            candidate_0: 'resource://dockerfile/candidate_0',
            candidate_1: 'resource://dockerfile/candidate_1',
            candidate_2: 'resource://dockerfile/candidate_2',
            comparison: 'resource://dockerfile/comparison',
          },
        });
      } else {
        return Success({
          ok: true,
          content: 'resource://dockerfile/basic',
          resources: {
            dockerfile: 'resource://dockerfile/basic',
          },
        });
      }
    },
  },

  build_image: {
    name: 'build_image',
    supportsSampling: false,
    async execute(args: Record<string, unknown>): Promise<Result<ToolResult>> {
      logger.info({ args }, 'Mock: Building image');

      await new Promise(resolve => setTimeout(resolve, 8000)); // Simulate build time

      return Success({
        ok: true,
        content: {
          imageId: 'sha256:mock123456',
          imageSize: '187MB',
          buildTime: 8.2,
          layers: 12,
        },
        resources: {
          logs: 'resource://build/logs',
          metadata: 'resource://build/metadata',
        },
      });
    },
  },

  scan_image: {
    name: 'scan_image',
    supportsSampling: false,
    async execute(args: Record<string, unknown>): Promise<Result<ToolResult>> {
      logger.info({ args }, 'Mock: Scanning image');

      await new Promise(resolve => setTimeout(resolve, 3000));

      // Simulate some vulnerabilities
      const vulnerabilities = {
        critical: Math.floor(Math.random() * 2), // 0-1 critical
        high: Math.floor(Math.random() * 3),     // 0-2 high
        medium: Math.floor(Math.random() * 5),   // 0-4 medium
        low: Math.floor(Math.random() * 8),       // 0-7 low
      };

      return Success({
        ok: true,
        content: {
          vulnerabilities,
          riskScore: vulnerabilities.critical * 10 + vulnerabilities.high * 5 +
            vulnerabilities.medium * 2 + vulnerabilities.low * 0.5,
          needsRemediation: vulnerabilities.critical > 0 || vulnerabilities.high > 2,
        },
        resources: {
          report: 'resource://scan/report',
          details: 'resource://scan/details',
        },
      });
    },
  },

  generate_k8s_manifests: {
    name: 'generate_k8s_manifests',
    supportsSampling: true,
    samplingConfig: {
      maxCandidates: 3,
      scoringWeights: {
        security: 0.35,
        scalability: 0.25,
        reliability: 0.25,
        efficiency: 0.15,
      },
    },
    async execute(args: Record<string, unknown>): Promise<Result<ToolResult>> {
      logger.info({ args }, 'Mock: Generating K8s manifests');

      const useSampling = args.useSampling as boolean;
      await new Promise(resolve => setTimeout(resolve, useSampling ? 3000 : 1000));

      return Success({
        ok: true,
        content: useSampling ? {
          winner: 'resource://k8s/winner',
          candidates: ['resource://k8s/candidate_0', 'resource://k8s/candidate_1', 'resource://k8s/candidate_2'],
          strategy: 'rolling-deployment',
        } : {
          manifests: 'resource://k8s/basic',
        },
        resources: useSampling ? {
          winner: 'resource://k8s/winner',
          candidate_0: 'resource://k8s/candidate_0',
          candidate_1: 'resource://k8s/candidate_1',
          candidate_2: 'resource://k8s/candidate_2',
        } : {
          manifests: 'resource://k8s/basic',
        },
      });
    },
  },

  deploy_application: {
    name: 'deploy_application',
    supportsSampling: false,
    async execute(args: Record<string, unknown>): Promise<Result<ToolResult>> {
      logger.info({ args }, 'Mock: Deploying application');

      await new Promise(resolve => setTimeout(resolve, 6000));

      return Success({
        ok: true,
        content: {
          deploymentName: 'mock-app-deployment',
          serviceName: 'mock-app-service',
          namespace: 'default',
          replicas: 3,
          readyReplicas: 3,
        },
        resources: {
          status: 'resource://deploy/status',
          events: 'resource://deploy/events',
        },
      });
    },
  },

  verify_deployment: {
    name: 'verify_deployment',
    supportsSampling: false,
    async execute(args: Record<string, unknown>): Promise<Result<ToolResult>> {
      logger.info({ args }, 'Mock: Verifying deployment');

      await new Promise(resolve => setTimeout(resolve, 4000));

      return Success({
        ok: true,
        content: {
          healthy: true,
          endpoints: ['http://mock-app-service:3000/health'],
          responseTime: 145,
          uptime: '100%',
        },
        resources: {
          healthChecks: 'resource://verify/health',
          performance: 'resource://verify/performance',
        },
      });
    },
  },
});

// Mock remediation tool (conditional)
export const createMockRemediationTool = (logger: Logger): IntelligentTool => ({
  name: 'remediate_vulnerabilities',
  supportsSampling: true,
  samplingConfig: {
    maxCandidates: 3,
    scoringWeights: {
      security: 0.6,
      stability: 0.3,
      compatibility: 0.1,
    },
  },
  async execute(args: Record<string, unknown>): Promise<Result<ToolResult>> {
    logger.info({ args }, 'Mock: Remediating vulnerabilities');

    await new Promise(resolve => setTimeout(resolve, 4000));

    return Success({
      ok: true,
      content: {
        remediatedDockerfile: 'resource://remediation/dockerfile',
        changesApplied: [
          'Updated base image from node:18 to node:18.19-alpine',
          'Added security patches for npm vulnerabilities',
          'Updated express to version 4.18.2',
        ],
        vulnerabilitiesFixed: {
          critical: 1,
          high: 2,
          medium: 1,
        },
      },
      resources: {
        dockerfile: 'resource://remediation/dockerfile',
        changelog: 'resource://remediation/changelog',
        verification: 'resource://remediation/verification',
      },
    });
  },
});

