/**
 * Unified Mock Factory
 * Consolidated mock system for all testing needs
 */

import { jest } from '@jest/globals';
import { nanoid } from 'nanoid';
import { EnvironmentCapabilities } from '../utilities/environment-detector';
import { Success, Failure } from '../../../src/types';

export type MockBehavior = 'success' | 'failure' | 'timeout' | 'partial';
export type SecurityFindingsLevel = 'none' | 'low' | 'medium' | 'high' | 'critical';

export interface MockScenario {
  name: string;
  behavior: MockBehavior;
  data?: unknown;
  error?: string;
  delay?: number;
}

/**
 * Unified Mock Factory - Single source for all mocking needs
 */
export class UnifiedMockFactory {
  private sessionId: string;

  constructor(sessionId: string = nanoid(8)) {
    this.sessionId = sessionId;
  }

  // ================================
  // Infrastructure Mocks
  // ================================

  /**
   * Create Docker client mock
   */
  createDockerMock(scenario: MockBehavior = 'success') {
    const mock = {
      buildImage: jest.fn(),
      pushImage: jest.fn(),
      tagImage: jest.fn(),
      pullImage: jest.fn(),
      removeImage: jest.fn(),
      listImages: jest.fn(),
      ping: jest.fn(),
      info: jest.fn(),
      version: jest.fn(),
    };

    switch (scenario) {
      case 'success':
        mock.buildImage.mockResolvedValue(Success({
          imageId: `sha256:${nanoid(12)}`,
          tags: ['test:latest'],
          logs: ['Step 1/5 : FROM node:18', 'Successfully built'],
          size: 123456789,
        }));
        mock.pushImage.mockResolvedValue(Success({
          registry: 'docker.io',
          repository: 'test/app',
          tag: 'latest',
          digest: `sha256:${nanoid(12)}`,
        }));
        mock.tagImage.mockResolvedValue(Success({
          sourceImage: `sha256:${nanoid(12)}`,
          targetTag: 'test:v1.0',
        }));
        mock.ping.mockResolvedValue(Success({ ok: true }));
        mock.info.mockResolvedValue(Success({
          containers: 5,
          images: 10,
          serverVersion: '24.0.0',
          architecture: 'x86_64',
        }));
        break;

      case 'failure':
        mock.buildImage.mockResolvedValue(Failure('Docker build failed: invalid Dockerfile'));
        mock.pushImage.mockResolvedValue(Failure('Push failed: authentication required'));
        mock.tagImage.mockResolvedValue(Failure('Tag failed: image not found'));
        mock.ping.mockResolvedValue(Failure('Docker daemon not available'));
        break;

      case 'timeout':
        mock.buildImage.mockImplementation(() => new Promise((_, reject) => 
          setTimeout(() => reject(new Error('Build timeout')), 100)
        ));
        mock.pushImage.mockImplementation(() => new Promise((_, reject) => 
          setTimeout(() => reject(new Error('Push timeout')), 100)
        ));
        break;
    }

    return mock;
  }

  /**
   * Create Kubernetes client mock
   */
  createKubernetesMock(clusterType: 'kind' | 'minikube' | 'remote' | 'unavailable' = 'kind') {
    const mock = {
      applyManifest: jest.fn(),
      deleteManifest: jest.fn(),
      getDeployment: jest.fn(),
      getService: jest.fn(),
      getPods: jest.fn(),
      waitForDeployment: jest.fn(),
      checkCluster: jest.fn(),
      getCurrentContext: jest.fn(),
    };

    if (clusterType === 'unavailable') {
      Object.values(mock).forEach(fn => {
        fn.mockResolvedValue(Failure('Kubernetes cluster not available'));
      });
      return mock;
    }

    // Success scenarios based on cluster type
    mock.applyManifest.mockResolvedValue(Success({
      applied: ['deployment/test-app', 'service/test-app-service'],
      namespace: 'default',
    }));

    mock.getDeployment.mockResolvedValue(Success({
      name: 'test-app',
      namespace: 'default',
      replicas: { desired: 3, ready: 3, available: 3 },
      status: 'Ready',
    }));

    mock.checkCluster.mockResolvedValue(Success({
      connected: true,
      version: '1.28.0',
      context: `${clusterType}-test`,
      type: clusterType,
    }));

    return mock;
  }

  /**
   * Create Trivy security scanner mock
   */
  createTrivyMock(findingsLevel: SecurityFindingsLevel = 'none') {
    const mock = {
      scanImage: jest.fn(),
      scanFilesystem: jest.fn(),
      scanRepository: jest.fn(),
      version: jest.fn(),
    };

    const generateFindings = (level: SecurityFindingsLevel) => {
      const baseFindings = {
        summary: { total: 0, critical: 0, high: 0, medium: 0, low: 0 },
        vulnerabilities: [] as any[],
      };

      switch (level) {
        case 'critical':
          baseFindings.summary = { total: 5, critical: 2, high: 2, medium: 1, low: 0 };
          baseFindings.vulnerabilities = [
            { 
              id: 'CVE-2023-1234', 
              severity: 'CRITICAL',
              title: 'Remote Code Execution in library',
              description: 'Critical security vulnerability'
            },
            {
              id: 'CVE-2023-5678',
              severity: 'HIGH', 
              title: 'SQL Injection vulnerability',
              description: 'High severity security issue'
            }
          ];
          break;
        case 'high':
          baseFindings.summary = { total: 3, critical: 0, high: 2, medium: 1, low: 0 };
          break;
        case 'medium':
          baseFindings.summary = { total: 2, critical: 0, high: 0, medium: 2, low: 0 };
          break;
        case 'low':
          baseFindings.summary = { total: 1, critical: 0, high: 0, medium: 0, low: 1 };
          break;
        case 'none':
        default:
          // Keep default empty findings
          break;
      }

      return baseFindings;
    };

    const findings = generateFindings(findingsLevel);
    
    mock.scanImage.mockResolvedValue(Success(findings));
    mock.scanFilesystem.mockResolvedValue(Success(findings));
    mock.scanRepository.mockResolvedValue(Success(findings));
    mock.version.mockResolvedValue(Success({ version: '0.45.0' }));

    return mock;
  }

  // ================================
  // Tool Mocks
  // ================================

  /**
   * Create tool mock with specified behavior
   */
  createToolMock<TInput, TOutput>(
    toolName: string,
    scenario: MockScenario
  ) {
    const mock = {
      execute: jest.fn(),
      validate: jest.fn(),
      getName: jest.fn().mockReturnValue(toolName),
    };

    switch (scenario.behavior) {
      case 'success':
        mock.execute.mockResolvedValue(Success(scenario.data));
        mock.validate.mockResolvedValue(Success(true));
        break;
      case 'failure':
        mock.execute.mockResolvedValue(Failure(scenario.error || `${toolName} execution failed`));
        mock.validate.mockResolvedValue(Failure('Validation failed'));
        break;
      case 'timeout':
        mock.execute.mockImplementation(() => 
          new Promise((_, reject) => 
            setTimeout(() => reject(new Error(`${toolName} timeout`)), scenario.delay || 100)
          )
        );
        break;
      case 'partial':
        mock.execute.mockResolvedValue(Success({
          ...scenario.data,
          warnings: [`${toolName} completed with warnings`],
        }));
        break;
    }

    return mock;
  }

  /**
   * Create analyze-repo tool mock
   */
  createAnalyzeRepoMock(projectType: 'java' | 'node' | 'dotnet' | 'python' = 'java') {
    return this.createToolMock('analyze-repo', {
      name: 'analyze-repo-success',
      behavior: 'success',
      data: {
        projectType,
        buildTool: projectType === 'java' ? 'maven' : projectType === 'node' ? 'npm' : 'dotnet',
        dependencies: [
          { name: 'express', version: '4.18.0', type: 'production' },
          { name: 'jest', version: '29.0.0', type: 'development' }
        ],
        ports: [8080],
        environments: ['development', 'production'],
        recommendations: ['Use multi-stage build', 'Add security scanning'],
      }
    });
  }

  /**
   * Create build-image tool mock
   */
  createBuildImageMock(scenario: MockBehavior = 'success') {
    const scenarios: Record<MockBehavior, MockScenario> = {
      success: {
        name: 'build-success',
        behavior: 'success',
        data: {
          imageId: `sha256:${nanoid(12)}`,
          tags: ['test-app:latest', 'test-app:v1.0'],
          size: 256789012,
          layers: 12,
          buildTime: 45.6,
          logs: [
            'Step 1/12 : FROM node:18-alpine',
            'Step 12/12 : CMD ["npm", "start"]',
            'Successfully built'
          ]
        }
      },
      failure: {
        name: 'build-failure',
        behavior: 'failure',
        error: 'Docker build failed: COPY failed - file not found'
      },
      timeout: {
        name: 'build-timeout',
        behavior: 'timeout',
        delay: 150
      },
      partial: {
        name: 'build-partial',
        behavior: 'partial',
        data: {
          imageId: `sha256:${nanoid(12)}`,
          tags: ['test-app:latest'],
          warnings: ['Layer cache miss', 'Large image size detected']
        }
      }
    };

    return this.createToolMock('build-image', scenarios[scenario]);
  }

  // ================================
  // Workflow Mocks
  // ================================

  /**
   * Create workflow orchestration mock
   */
  createWorkflowMock(
    steps: string[],
    outcomes: MockBehavior[]
  ) {
    const mock = {
      execute: jest.fn(),
      getSteps: jest.fn().mockReturnValue(steps),
      getStatus: jest.fn(),
      cancel: jest.fn(),
    };

    // Create outcomes for each step
    const stepResults = steps.map((step, index) => {
      const outcome = outcomes[index] || 'success';
      return {
        step,
        status: outcome === 'success' ? 'completed' : 'failed',
        result: outcome === 'success' 
          ? Success({ step, completed: true }) 
          : Failure(`Step ${step} failed`)
      };
    });

    mock.execute.mockResolvedValue(Success({
      workflowId: `wf-${this.sessionId}`,
      steps: stepResults,
      status: stepResults.every(s => s.status === 'completed') ? 'completed' : 'failed',
      duration: 120.5,
    }));

    return mock;
  }

  // ================================
  // Environment-Aware Mocks
  // ================================

  /**
   * Create environment-aware mock based on capabilities
   */
  createEnvironmentalMock(capabilities: EnvironmentCapabilities) {
    return {
      docker: capabilities.docker.available 
        ? this.createDockerMock('success')
        : this.createDockerMock('failure'),
      
      kubernetes: capabilities.kubernetes.available
        ? this.createKubernetesMock(capabilities.kubernetes.type as any)
        : this.createKubernetesMock('unavailable'),
        
      trivy: capabilities.trivy.available
        ? this.createTrivyMock('none')
        : this.createTrivyMock('none'), // Mock still works, just returns empty results
    };
  }

  // ================================
  // Utility Methods
  // ================================

  /**
   * Reset all mocks
   */
  resetAllMocks() {
    jest.clearAllMocks();
  }

  /**
   * Get session ID for this factory instance
   */
  getSessionId() {
    return this.sessionId;
  }

  /**
   * Create mock with delay for testing timeouts/async behavior
   */
  createDelayedMock<T>(data: T, delay: number = 100) {
    return jest.fn().mockImplementation(() => 
      new Promise(resolve => setTimeout(() => resolve(Success(data)), delay))
    );
  }

  /**
   * Create mock that fails after N calls (for testing retry logic)
   */
  createFailAfterMock<T>(successData: T, failAfter: number = 2) {
    let callCount = 0;
    return jest.fn().mockImplementation(() => {
      callCount++;
      if (callCount <= failAfter) {
        return Promise.resolve(Success(successData));
      } else {
        return Promise.resolve(Failure(`Mock failed after ${failAfter} calls`));
      }
    });
  }
}

/**
 * Global factory instance for convenience
 */
export const unifiedMockFactory = new UnifiedMockFactory();

/**
 * Convenience functions for common mocking patterns
 */
export const mockDocker = (scenario: MockBehavior = 'success') => 
  unifiedMockFactory.createDockerMock(scenario);

export const mockKubernetes = (clusterType: 'kind' | 'minikube' | 'remote' | 'unavailable' = 'kind') => 
  unifiedMockFactory.createKubernetesMock(clusterType);

export const mockTrivy = (findings: SecurityFindingsLevel = 'none') => 
  unifiedMockFactory.createTrivyMock(findings);

export const mockTool = <TInput, TOutput>(toolName: string, scenario: MockScenario) => 
  unifiedMockFactory.createToolMock<TInput, TOutput>(toolName, scenario);

export const mockEnvironment = (capabilities: EnvironmentCapabilities) => 
  unifiedMockFactory.createEnvironmentalMock(capabilities);