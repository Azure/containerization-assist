import { DockerTestCleaner } from '../utilities/docker-test-cleaner';

export default async function globalTeardown() {
  console.log('\n🧹 Cleaning up global test environment...');
  
  try {
    // Simple final cleanup - just remove any remaining test images
    await DockerTestCleaner.globalCleanup();
    console.log('✅ Docker cleanup completed');
  } catch (error: any) {
    console.error('❌ Global teardown error:', error.message);
    // Don't fail the test run due to cleanup errors
  }
  
  console.log('✅ Global teardown complete');
}
