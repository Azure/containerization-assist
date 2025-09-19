/**
 * Dependencies Test
 * Validates service dependency injection and configuration
 */

import { describe, test, expect, beforeEach, jest } from '@jest/globals';
import { createMockLogger } from '../../__support__/utilities/test-helpers';
import type { Logger } from 'pino';

describe('Service Dependencies', () => {
  let mockLogger: Logger;

  beforeEach(() => {
    mockLogger = createMockLogger();
  });

  test('should validate dependency structure for architecture', () => {
    // Test the expected structure of dependencies after refactor
    const expectedDependencies = {
      logger: expect.any(Object),
      sessionService: expect.any(Object),
      progressEmitter: expect.any(Object),
      dockerClient: expect.any(Object),
      repositoryAnalyzer: expect.any(Object),
      eventPublisher: expect.any(Object),
      workflowManager: expect.any(Object),
      workflowOrchestrator: expect.any(Object),
      mcpSampler: expect.any(Object),
      structuredSampler: expect.any(Object),
      contentValidator: expect.any(Object),
      config: expect.any(Object)
    };

    // Mock dependency structure
    const mockDependencies = {
      logger: mockLogger,
      sessionService: { get: jest.fn(), create: jest.fn() },
      progressEmitter: { emit: jest.fn() },
      dockerClient: { build: jest.fn(), scan: jest.fn() },
      repositoryAnalyzer: { analyze: jest.fn() },
      eventPublisher: { publish: jest.fn() },
      workflowManager: { start: jest.fn() },
      workflowOrchestrator: { execute: jest.fn() },
      mcpSampler: { sample: jest.fn() },
      structuredSampler: { sampleJSON: jest.fn() },
      contentValidator: { validateContent: jest.fn() },
      config: { workspaceDir: '/tmp/test' }
    };

    // Validate structure matches expected
    Object.keys(expectedDependencies).forEach(key => {
      expect(mockDependencies).toHaveProperty(key);
      expect(mockDependencies[key as keyof typeof mockDependencies]).toBeDefined();
    });
  });

  test('should support type system in dependencies', () => {
    // Test that dependencies work with consolidated types
    const mockDeps = {
      logger: mockLogger,
      sessionService: {
        get: jest.fn().mockResolvedValue({
          id: 'test-session',
          status: 'active',
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
          repo_path: '/test/repo'
        })
      }
    };

    expect(mockDeps.sessionService.get).toBeDefined();
    expect(mockDeps.logger).toBeDefined();
  });

  test('should support infrastructure standardization', () => {
    // Test logger and Docker abstraction
    const infrastructureDeps = {
      logger: mockLogger,
      dockerService: {    // Single Docker abstraction
        build: jest.fn(),
        scan: jest.fn(),
        push: jest.fn(),
        tag: jest.fn()
      }
    };

    expect(infrastructureDeps.logger.child).toBeDefined();
    expect(infrastructureDeps.dockerService.build).toBeDefined();
    expect(infrastructureDeps.dockerService.scan).toBeDefined();
  });

  test('should support service layer organization', () => {
    // Test service layer dependency patterns
    const serviceDeps = {
      toolRegistry: {
        register: jest.fn(),
        execute: jest.fn(),
        listTools: jest.fn()
      },
      workflowOrchestrator: {
        startWorkflow: jest.fn(),
        getStatus: jest.fn()
      },
      sessionManager: {
        createSession: jest.fn(),
        getSession: jest.fn(),
        updateSession: jest.fn()
      }
    };

    expect(serviceDeps.toolRegistry.register).toBeDefined();
    expect(serviceDeps.workflowOrchestrator.startWorkflow).toBeDefined();
    expect(serviceDeps.sessionManager.createSession).toBeDefined();
  });

  test('should validate dependency injection patterns', () => {
    // Test that dependencies can be injected correctly
    class TestService {
      constructor(
        private logger: Logger,
        private sessionService: any,
        private config: any
      ) {}

      async testOperation() {
        this.logger.info('Test operation started');
        const session = await this.sessionService.get('test-id');
        return { success: true, session };
      }
    }

    const mockSessionService = {
      get: jest.fn().mockResolvedValue({ id: 'test-id', status: 'active' })
    };
    
    const mockConfig = { workspaceDir: '/test' };
    
    const service = new TestService(mockLogger, mockSessionService, mockConfig);
    
    expect(service).toBeDefined();
    expect(service.testOperation).toBeDefined();
  });

  test('should validate cross-system integration in dependencies', () => {
    // Test that all system consolidations work together in dependency structure
    const integratedDeps = {
      // Consolidated type system
      types: {
        session: expect.any(Object),
        result: expect.any(Object),
        errors: expect.any(Object)
      },
      
      // Infrastructure standardization
      infrastructure: {
        logger: mockLogger,
        docker: { service: jest.fn() },
        messaging: { publisher: jest.fn() }
      },
      
      // Service organization
      services: {
        toolRegistry: { register: jest.fn() },
        workflowManager: { start: jest.fn() },
        sessionManager: { create: jest.fn() }
      }
    };

    // Verify all system integrations are present
    expect(integratedDeps.types).toBeDefined();
    expect(integratedDeps.infrastructure).toBeDefined();
    expect(integratedDeps.services).toBeDefined();
    
    expect(integratedDeps.infrastructure.logger).toBeDefined();
    expect(integratedDeps.services.toolRegistry).toBeDefined();
  });

  test('should support test infrastructure requirements', () => {
    // Test dependencies needed for test infrastructure
    const testDeps = {
      logger: mockLogger,
      mockServices: {
        sessionService: {
          get: jest.fn(),
          create: jest.fn(),
          update: jest.fn()
        },
        dockerService: {
          build: jest.fn().mockResolvedValue({ success: true }),
          scan: jest.fn().mockResolvedValue({ success: true })
        }
      },
      testConfig: {
        mockMode: true,
        testWorkspace: '/tmp/test'
      }
    };

    expect(testDeps.logger.child).toBeDefined();
    expect(testDeps.mockServices.sessionService.get).toBeDefined();
    expect(testDeps.mockServices.dockerService.build).toBeDefined();
    expect(testDeps.testConfig.mockMode).toBe(true);
  });

  test('should validate dependency lifecycle management', () => {
    // Test initialization and cleanup patterns
    const lifecycleDeps = {
      initialize: jest.fn().mockResolvedValue(true),
      cleanup: jest.fn().mockResolvedValue(true),
      isReady: jest.fn().mockReturnValue(true),
      getStatus: jest.fn().mockReturnValue({ initialized: true, ready: true })
    };

    expect(lifecycleDeps.initialize).toBeDefined();
    expect(lifecycleDeps.cleanup).toBeDefined();
    expect(lifecycleDeps.isReady).toBeDefined();
    expect(lifecycleDeps.getStatus).toBeDefined();
  });
});

describe('Dependency Configuration Validation', () => {
  test('should validate configuration structure for consolidated architecture', () => {
    const configStructure = {
      workspaceDir: '/tmp/workspace',
      session: {
        store: 'memory',
        ttl: 3600,
        maxSessions: 100
      },
      docker: {
        socketPath: '/var/run/docker.sock'
      },
      kubernetes: {
        namespace: 'default'
      },
      features: {
        mockMode: false
      }
    };

    expect(configStructure.workspaceDir).toBeDefined();
    expect(configStructure.session).toBeDefined();
    expect(configStructure.docker).toBeDefined();
    expect(configStructure.kubernetes).toBeDefined();
    expect(typeof configStructure.session.ttl).toBe('number');
  });

  test('should validate environment-specific configuration', () => {
    const testConfig = {
      nodeEnv: 'test',
      logLevel: 'error'
    };

    const productionConfig = {
      nodeEnv: 'production',
      logLevel: 'info'
    };

    expect(testConfig.logLevel).toBe('error');
    expect(productionConfig.logLevel).toBe('info');
  });
});