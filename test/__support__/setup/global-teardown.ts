import { DockerTestCleaner } from '../utilities/docker-test-cleaner';
import { createDockerClient } from '../../../dist/src/services/docker-client.js';
import { createLogger } from '../../../dist/src/lib/logger.js';

export default async function globalTeardown() {
  console.log('\n🧹 Cleaning up global test environment...');
  
  try {
    // Create a Docker client for cleanup
    const logger = createLogger({ level: 'error' }); // Minimal logging for cleanup
    const dockerClient = createDockerClient(logger);
    
    // Simple final cleanup - just remove any remaining test images
    await DockerTestCleaner.globalCleanup(dockerClient);
    console.log('✅ Docker cleanup completed');
  } catch (error: any) {
    console.error('❌ Global teardown error:', error.message);
    // Don't fail the test run due to cleanup errors
  }
  
  console.log('✅ Global teardown complete');
}
