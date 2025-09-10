/**
 * Test Container Setup
 * 
 * Provides service container for tests with mock services
 */

import { jest } from '@jest/globals';
import { createMockLogger } from '../utils/mock-factories';
import type { Logger } from 'pino';

export interface TestServiceBindings {
  DockerClient?: any;
  KubernetesClient?: any;
  AIService?: any;
  Logger?: Logger;
  SessionManager?: any;
  ConfigService?: any;
  CacheService?: any;
  SecurityScanner?: any;
}

/**
 * Simple service container for tests
 */
export class TestContainer {
  private services: Map<string, any> = new Map();

  bind(key: string, value: any): void {
    this.services.set(key, value);
  }

  get<T>(key: string): T {
    return this.services.get(key);
  }

  has(key: string): boolean {
    return this.services.has(key);
  }
}

/**
 * Create a test container with mock services
 */
export function createTestContainer(overrides?: TestServiceBindings): TestContainer {
  const container = new TestContainer();

  // Default bindings with mocks
  container.bind('Logger', overrides?.Logger || createMockLogger());

  if (overrides?.DockerClient) {
    container.bind('DockerClient', overrides.DockerClient);
  }

  if (overrides?.KubernetesClient) {
    container.bind('KubernetesClient', overrides.KubernetesClient);
  }

  if (overrides?.AIService) {
    container.bind('AIService', overrides.AIService);
  }

  if (overrides?.SessionManager) {
    container.bind('SessionManager', overrides.SessionManager);
  }

  if (overrides?.ConfigService) {
    container.bind('ConfigService', overrides.ConfigService);
  }

  if (overrides?.CacheService) {
    container.bind('CacheService', overrides.CacheService);
  }

  if (overrides?.SecurityScanner) {
    container.bind('SecurityScanner', overrides.SecurityScanner);
  }

  return container;
}

/**
 * Create an integration test container with real implementations where safe
 */
export function createIntegrationContainer(overrides?: TestServiceBindings): TestContainer {
  const container = new TestContainer();

  // Use real logger for integration tests
  container.bind('Logger', overrides?.Logger || createMockLogger());

  // Add other services as needed, preferring real implementations
  // but still allowing overrides for controlled testing

  if (overrides?.DockerClient) {
    container.bind('DockerClient', overrides.DockerClient);
  }

  if (overrides?.KubernetesClient) {
    container.bind('KubernetesClient', overrides.KubernetesClient);
  }

  return container;
}

/**
 * Test Server Helper interface for integration tests
 */
export interface TestServerHelper {
  cleanup(): Promise<void>;
  getContainer(): TestContainer;
  startServer(): Promise<any>;
  stopServer(): Promise<void>;
  getStatus(): { running: boolean; tools?: number; resources?: number; prompts?: number; workflows?: number;[key: string]: any };
  registerTestTool(name: string, description: string): void;
  registerTestPrompt(name: string, content: string): void;
  getTools(): Array<{ name: string; description: string }>;
  addTestResource(uri: string, content: string): void;
  deps: {
    resourceManager: { cleanup(): void };
    sessionManager: {
      createSession(id: string): { id: string };
      getSession(id: string): { id: string } | undefined;
    };
    promptRegistry: {
      listPrompts(): Promise<{ prompts: Array<{ name: string; content: string }> }>;
      getPrompt(name: string): Promise<{ name: string; content: string }>;
      hasPrompt(name: string): boolean;
    };
  };
}

/**
 * Create a test server helper for integration tests
 */
export function createTestServer(overrides?: TestServiceBindings): TestServerHelper {
  const container = createIntegrationContainer(overrides);
  let server: any = null;
  let running = false;
  const tools: Array<{ name: string; description: string }> = [];
  const resources: Map<string, string> = new Map();
  const sessions: Map<string, { id: string }> = new Map();
  const prompts: Map<string, { name: string; content: string }> = new Map();

  return {
    async cleanup() {
      if (server && running) {
        // Stop server if running
        running = false;
        server = null;
      }
    },
    getContainer() {
      return container;
    },
    async startServer() {
      if (!server) {
        // Create a mock server for testing
        server = {
          transport: {
            type: 'stdio',
            connected: true,
            close: jest.fn().mockResolvedValue(undefined)
          },
          capabilities: {
            tools: {},
            prompts: {},
            resources: {}
          },
          getWorkflows: () => [
            {
              name: 'start_workflow',
              description: 'Start a complete containerization workflow',
            },
            {
              name: 'workflow_status',
              description: 'Get the status of a running workflow',
            },
          ]
        };
        running = true;
      }
      return server;
    },
    async stopServer() {
      if (server && running) {
        running = false;
        if (server.transport?.close) {
          try {
            await server.transport.close();
          } catch (error) {
            // Gracefully handle transport errors during shutdown
            console.warn('Transport close error during shutdown:', error);
          }
        }
        server = null;
      }
    },
    getStatus() {
      return {
        running,
        tools: tools.length,
        resources: resources.size,
        prompts: prompts.size,
        workflows: 2,
        server,
        capabilities: server?.capabilities || {}
      };
    },
    registerTestTool(name: string, description: string) {
      tools.push({ name, description });
    },
    registerTestPrompt(name: string, content: string) {
      prompts.set(name, { name, content });
    },
    getTools() {
      return [...tools];
    },
    addTestResource(uri: string, content: string) {
      resources.set(uri, content);
    },
    deps: {
      resourceManager: {
        cleanup() {
          resources.clear();
        }
      },
      sessionManager: {
        createSession(id: string) {
          const session = { id };
          sessions.set(id, session);
          return session;
        },
        getSession(id: string) {
          return sessions.get(id);
        }
      },
      promptRegistry: {
        async listPrompts() {
          return {
            prompts: Array.from(prompts.values())
          };
        },
        async getPrompt(name: string) {
          const prompt = prompts.get(name);
          if (!prompt) {
            throw new Error(`Prompt not found: ${name}`);
          }
          return prompt;
        },
        hasPrompt(name: string) {
          return prompts.has(name);
        }
      }
    }
  };
}