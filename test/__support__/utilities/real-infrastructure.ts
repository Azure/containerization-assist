/**
 * Real Infrastructure Helper
 * Provides real infrastructure connections for integration tests
 */

import { DockerTestCleaner } from './docker-test-cleaner';
import { createLogger } from '../../../src/lib/logger';

const logger = createLogger({ level: 'debug' });

export function createRealInfrastructure(testEnvironment: any) {
  const dockerTestCleaner = new DockerTestCleaner(logger);
  
  return {
    docker: testEnvironment.dockerClient,
    kubernetes: testEnvironment.kubernetesClient,
    dockerTestCleaner,
    cleanup: async () => {
      // Single cleanup call - no duplication
      await dockerTestCleaner.cleanup();
      
      // Clean up test environment resources if provided
      if (testEnvironment.cleanup) {
        await testEnvironment.cleanup();
      }
    },
  };
}

export {};