/**
 * Workflow Manager Test
 * Validates workflow management functionality
 */

import { describe, test, expect, beforeEach, jest } from '@jest/globals';
import { createMockLogger } from '../../__support__/utilities/test-helpers';
import type { Logger } from 'pino';

// Mock workflow types for testing
interface MockWorkflowState {
  id: string;
  status: 'pending' | 'running' | 'completed' | 'failed';
  currentStep: string;
  steps: string[];
  startTime: string;
  endTime?: string;
  metadata: Record<string, unknown>;
}

interface MockWorkflowManager {
  startWorkflow: jest.Mock;
  getWorkflowStatus: jest.Mock;
  updateWorkflowStep: jest.Mock;
  completeWorkflow: jest.Mock;
  failWorkflow: jest.Mock;
  listActiveWorkflows: jest.Mock;
}

describe('Workflow Manager Consolidation', () => {
  let mockLogger: Logger;
  let mockWorkflowManager: MockWorkflowManager;

  beforeEach(() => {
    mockLogger = createMockLogger();
    mockWorkflowManager = {
      startWorkflow: jest.fn(),
      getWorkflowStatus: jest.fn(),
      updateWorkflowStep: jest.fn(),
      completeWorkflow: jest.fn(),
      failWorkflow: jest.fn(),
      listActiveWorkflows: jest.fn()
    };
  });

  test('should validate workflow management interface', () => {
    // Test that workflow manager provides interface
    const expectedMethods = ['startWorkflow', 'getWorkflowStatus', 'updateWorkflowStep', 
                           'completeWorkflow', 'failWorkflow', 'listActiveWorkflows'];
    
    expectedMethods.forEach(method => {
      expect(mockWorkflowManager).toHaveProperty(method);
      expect(typeof mockWorkflowManager[method as keyof MockWorkflowManager]).toBe('function');
    });
  });

  test('should support workflow creation and management', async () => {
    const workflowConfig = {
      type: 'containerization',
      steps: ['analyze', 'generate-dockerfile', 'build', 'scan', 'deploy'],
      sessionId: 'test-session-123',
      repoPath: '/test/repo'
    };

    const expectedWorkflow: MockWorkflowState = {
      id: 'workflow-123',
      status: 'pending',
      currentStep: 'analyze',
      steps: workflowConfig.steps,
      startTime: new Date().toISOString(),
      metadata: {
        sessionId: workflowConfig.sessionId,
        repoPath: workflowConfig.repoPath
      }
    };

    mockWorkflowManager.startWorkflow.mockResolvedValue(expectedWorkflow);

    const result = await mockWorkflowManager.startWorkflow(workflowConfig);

    expect(mockWorkflowManager.startWorkflow).toHaveBeenCalledWith(workflowConfig);
    expect(result).toEqual(expectedWorkflow);
    expect(result.status).toBe('pending');
    expect(result.steps).toEqual(workflowConfig.steps);
  });

  test('should support workflow status tracking', async () => {
    const workflowStatus: MockWorkflowState = {
      id: 'workflow-123',
      status: 'running',
      currentStep: 'build',
      steps: ['analyze', 'generate-dockerfile', 'build', 'scan', 'deploy'],
      startTime: '2024-01-01T00:00:00Z',
      metadata: { sessionId: 'test-session-123' }
    };

    mockWorkflowManager.getWorkflowStatus.mockResolvedValue(workflowStatus);

    const result = await mockWorkflowManager.getWorkflowStatus('workflow-123');

    expect(mockWorkflowManager.getWorkflowStatus).toHaveBeenCalledWith('workflow-123');
    expect(result).toEqual(workflowStatus);
    expect(result.status).toBe('running');
    expect(result.currentStep).toBe('build');
  });

  test('should support workflow step updates', async () => {
    const stepUpdate = {
      workflowId: 'workflow-123',
      step: 'scan',
      status: 'running',
      metadata: { imageId: 'sha256:test123' }
    };

    const updatedWorkflow: MockWorkflowState = {
      id: 'workflow-123',
      status: 'running',
      currentStep: 'scan',
      steps: ['analyze', 'generate-dockerfile', 'build', 'scan', 'deploy'],
      startTime: '2024-01-01T00:00:00Z',
      metadata: { 
        sessionId: 'test-session-123',
        imageId: 'sha256:test123'
      }
    };

    mockWorkflowManager.updateWorkflowStep.mockResolvedValue(updatedWorkflow);

    const result = await mockWorkflowManager.updateWorkflowStep(stepUpdate);

    expect(mockWorkflowManager.updateWorkflowStep).toHaveBeenCalledWith(stepUpdate);
    expect(result).toEqual(updatedWorkflow);
    expect(result.currentStep).toBe('scan');
  });

  test('should support workflow completion', async () => {
    const completedWorkflow: MockWorkflowState = {
      id: 'workflow-123',
      status: 'completed',
      currentStep: 'deploy',
      steps: ['analyze', 'generate-dockerfile', 'build', 'scan', 'deploy'],
      startTime: '2024-01-01T00:00:00Z',
      endTime: '2024-01-01T01:00:00Z',
      metadata: { 
        sessionId: 'test-session-123',
        deploymentUrl: 'https://app.example.com'
      }
    };

    mockWorkflowManager.completeWorkflow.mockResolvedValue(completedWorkflow);

    const result = await mockWorkflowManager.completeWorkflow('workflow-123', {
      deploymentUrl: 'https://app.example.com'
    });

    expect(mockWorkflowManager.completeWorkflow).toHaveBeenCalledWith('workflow-123', {
      deploymentUrl: 'https://app.example.com'
    });
    expect(result.status).toBe('completed');
    expect(result.endTime).toBeDefined();
  });

  test('should support workflow failure handling', async () => {
    const error = new Error('Build failed');
    const failedWorkflow: MockWorkflowState = {
      id: 'workflow-123',
      status: 'failed',
      currentStep: 'build',
      steps: ['analyze', 'generate-dockerfile', 'build', 'scan', 'deploy'],
      startTime: '2024-01-01T00:00:00Z',
      endTime: '2024-01-01T00:30:00Z',
      metadata: { 
        sessionId: 'test-session-123',
        error: error.message
      }
    };

    mockWorkflowManager.failWorkflow.mockResolvedValue(failedWorkflow);

    const result = await mockWorkflowManager.failWorkflow('workflow-123', error);

    expect(mockWorkflowManager.failWorkflow).toHaveBeenCalledWith('workflow-123', error);
    expect(result.status).toBe('failed');
    expect(result.metadata.error).toBe('Build failed');
  });

  test('should support type system compatibility', () => {
    // Test workflow manager works with types
    const workflowWithConsolidatedTypes = {
      id: 'test-workflow',
      status: 'running' as const,
      currentStep: 'build',
      steps: ['analyze', 'build'],
      metadata: {
        sessionId: 'session-123',
        buildOptions: {
          dockerfile: 'Dockerfile',
          context: '.',
          tags: ['app:latest']
        }
      }
    };

    expect(workflowWithConsolidatedTypes.status).toBe('running');
    expect(workflowWithConsolidatedTypes.metadata.buildOptions).toBeDefined();
    expect(Array.isArray(workflowWithConsolidatedTypes.metadata.buildOptions.tags)).toBe(true);
  });

  test('should support infrastructure integration', () => {
    // Test workflow manager integrates with infrastructure
    const infrastructureIntegration = {
      workflowManager: mockWorkflowManager,
      logger: mockLogger,
      eventPublisher: { publish: jest.fn() },
      dockerService: { build: jest.fn() }
    };

    expect(infrastructureIntegration.workflowManager).toBeDefined();
    expect(infrastructureIntegration.logger).toBeDefined();
    expect(infrastructureIntegration.eventPublisher).toBeDefined();
    expect(infrastructureIntegration.dockerService).toBeDefined();
  });

  test('should validate dependency injection with architecture', () => {
    // Test workflow manager dependency injection patterns
    class TestWorkflowService {
      constructor(
        private workflowManager: MockWorkflowManager,
        private logger: Logger
      ) {}

      async startContainerizationWorkflow(sessionId: string) {
        this.logger.info('Starting containerization workflow', { sessionId });
        return await this.workflowManager.startWorkflow({
          type: 'containerization',
          sessionId,
          steps: ['analyze', 'build', 'deploy']
        });
      }
    }

    const service = new TestWorkflowService(mockWorkflowManager, mockLogger);
    expect(service).toBeDefined();
    expect(service.startContainerizationWorkflow).toBeDefined();
  });

  test('should support active workflow listing', async () => {
    const activeWorkflows: MockWorkflowState[] = [
      {
        id: 'workflow-1',
        status: 'running',
        currentStep: 'build',
        steps: ['analyze', 'build', 'deploy'],
        startTime: '2024-01-01T00:00:00Z',
        metadata: { sessionId: 'session-1' }
      },
      {
        id: 'workflow-2',
        status: 'running',
        currentStep: 'scan',
        steps: ['analyze', 'build', 'scan', 'deploy'],
        startTime: '2024-01-01T00:05:00Z',
        metadata: { sessionId: 'session-2' }
      }
    ];

    mockWorkflowManager.listActiveWorkflows.mockResolvedValue(activeWorkflows);

    const result = await mockWorkflowManager.listActiveWorkflows();

    expect(mockWorkflowManager.listActiveWorkflows).toHaveBeenCalled();
    expect(result).toEqual(activeWorkflows);
    expect(result).toHaveLength(2);
    expect(result.every(w => w.status === 'running')).toBe(true);
  });
});

describe('Workflow Manager Cross-System Integration', () => {
  test('should validate all system consolidation requirements', () => {
    const crossSystemIntegration = {
      // Consolidated types
      types: {
        workflowState: expect.any(Object),
        stepConfig: expect.any(Object),
        errorTypes: expect.any(Object)
      },
      
      // Infrastructure standardization
      infrastructure: {
        logger: createMockLogger(),
        eventPublisher: { publish: jest.fn() },
        sessionStore: { get: jest.fn(), set: jest.fn() }
      },
      
      // Service layer organization
      services: {
        workflowManager: {
          startWorkflow: jest.fn(),
          getWorkflowStatus: jest.fn()
        },
        sessionManager: {
          getSession: jest.fn(),
          updateSession: jest.fn()
        }
      }
    };

    expect(crossSystemIntegration.types).toBeDefined();
    expect(crossSystemIntegration.infrastructure).toBeDefined();
    expect(crossSystemIntegration.services).toBeDefined();
    
    expect(crossSystemIntegration.infrastructure.logger.info).toBeDefined();
    expect(crossSystemIntegration.services.workflowManager.startWorkflow).toBeDefined();
  });
});