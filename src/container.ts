/**
 * Dependency Injection Container
 *
 * Simple dependency creation and management for the application.
 */

import type { Logger } from 'pino';
import { createLogger } from './lib/logger';
import { createSessionManager, SessionManager } from './lib/session';
import * as promptRegistry from './prompts/registry';
import { findPromptsDirectory } from '@/lib/find-prompts-dir';
import * as resourceManager from './resources/manager';
import type { AIService } from './types';
import { createAppConfig, type AppConfig } from './config/app-config';
import { createDockerClient, type DockerClient } from './services/docker-client';
import { createKubernetesClient, type KubernetesClient } from './services/kubernetes-client';

/**
 * Application dependencies
 */
export interface Dependencies {
  config: AppConfig;
  logger: Logger;
  sessionManager: SessionManager;
  dockerClient: DockerClient;
  kubernetesClient: KubernetesClient;
  promptRegistry: typeof promptRegistry;
  resourceManager: typeof import('./resources/manager');
  aiService?: AIService;
}

/**
 * Create application dependencies
 */
export function createDependencies(config?: AppConfig): Dependencies {
  const appConfig = config ?? createAppConfig();

  const logger = createLogger({
    name: appConfig.mcp.name,
    level: appConfig.server.logLevel,
  });

  const sessionManager = createSessionManager(logger, {
    ttl: appConfig.session.ttl,
    maxSessions: appConfig.session.maxSessions,
    cleanupIntervalMs: appConfig.session.cleanupInterval,
  });

  return {
    config: appConfig,
    logger,
    sessionManager,
    dockerClient: createDockerClient(logger),
    kubernetesClient: createKubernetesClient(logger),
    promptRegistry,
    resourceManager,
  };
}

/**
 * Initialize asynchronous dependencies
 */
export async function initializeDependencies(deps: Dependencies): Promise<void> {
  // Initialize prompt registry
  const promptsDir = findPromptsDirectory();
  await deps.promptRegistry.initializePrompts(promptsDir, deps.logger);

  deps.logger.info(
    {
      config: {
        nodeEnv: deps.config.server.nodeEnv,
        logLevel: deps.config.server.logLevel,
        port: deps.config.server.port,
        maxSessions: deps.config.mcp.maxSessions,
        dockerSocket: deps.config.services.docker.socketPath,
        k8sNamespace: deps.config.services.kubernetes.namespace,
      },
      services: {
        logger: true,
        sessionManager: true,
        dockerClient: true,
        kubernetesClient: true,
        promptRegistry: true,
        resourceManager: true,
      },
    },
    'Dependencies initialized',
  );
}

/**
 * Gracefully shutdown all services
 */
export async function shutdownDependencies(deps: Dependencies): Promise<void> {
  const { logger, sessionManager } = deps;

  logger.info('Shutting down services...');

  try {
    // Close session manager (stops cleanup timers)
    if ('close' in sessionManager && typeof sessionManager.close === 'function') {
      sessionManager.close();
    }

    // Clean up resource manager
    if ('cleanup' in deps.resourceManager) {
      await deps.resourceManager.cleanup();
    }

    logger.info('Shutdown complete');
  } catch (error) {
    logger.error({ error }, 'Error during shutdown');
    throw error;
  }
}

/**
 * Status information
 */
export interface SystemStatus {
  healthy: boolean;
  running: boolean;
  services: Record<string, boolean>;
  stats: {
    resources: number;
    prompts: number;
  };
}

/**
 * Get system status
 */
export function getSystemStatus(deps: Dependencies, serverRunning = false): SystemStatus {
  const services = {
    logger: deps.logger !== undefined,
    sessionManager: deps.sessionManager !== undefined,
    dockerClient: deps.dockerClient !== undefined,
    kubernetesClient: deps.kubernetesClient !== undefined,
    promptRegistry: deps.promptRegistry !== undefined,
    resourceManager: deps.resourceManager !== undefined,
  };

  const promptCount = deps.promptRegistry.getPromptNames().length;
  const resourceStats = deps.resourceManager.getStats();

  return {
    healthy: Object.values(services).every(Boolean),
    running: serverRunning,
    services,
    stats: {
      resources: resourceStats.total,
      prompts: promptCount,
    },
  };
}
