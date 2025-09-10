/**
 * Test Environment Setup
 * Manages real infrastructure for integration tests
 */

import { exec } from 'child_process';
import { promisify } from 'util';

const execAsync = promisify(exec);

export async function setupTestEnvironment() {
  return {
    dockerClient: null,
    kubernetesClient: null,
    cleanup: async () => {
      // Clean up Docker test resources
      try {
        // Clean up test images
        const { stdout } = await execAsync('docker images --format "{{.Repository}}:{{.Tag}}" | grep "^test-" || true');
        if (stdout.trim()) {
          const images = stdout.trim().split('\n').filter(img => img.length > 0);
          for (const image of images) {
            try {
              await execAsync(`docker rmi "${image}" -f`);
            } catch (error) {
              // Ignore cleanup errors
            }
          }
        }
        
        // Clean up test containers
        const containerResult = await execAsync('docker ps -aq --filter "name=test-" || true');
        if (containerResult.stdout.trim()) {
          await execAsync('docker rm $(docker ps -aq --filter "name=test-") -f');
        }
        
        // Clean up dangling images
        await execAsync('docker image prune -f');
        
      } catch (error) {
        // Ignore cleanup errors in test environment
        console.debug('Test environment cleanup error:', error);
      }
    },
  };
}

export async function cleanupTestEnvironment(testEnvironment: any) {
  if (testEnvironment?.cleanup) {
    await testEnvironment.cleanup();
  }
}

export {};