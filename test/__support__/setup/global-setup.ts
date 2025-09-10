import { exec } from 'child_process';
import { promisify } from 'util';

const execAsync = promisify(exec);

export default async function globalSetup() {
  console.log('🏗️  Setting up global test environment...');
  
  try {
    // Verify Docker is available (but don't fail if not available for unit tests)
    try {
      await execAsync('docker --version');
      console.log('✅ Docker is available');
    } catch (error) {
      console.log('⚠️  Docker not available - some integration tests may be skipped');
    }
    
    // Verify Kubernetes tools if needed
    if (process.env.TEST_K8S) {
      try {
        await execAsync('kubectl version --client');
        console.log('✅ Kubernetes tools available');
      } catch (error) {
        console.log('⚠️  Kubernetes tools not available - some tests may be skipped');
      }
    }
    
    // Create test fixtures directory if it doesn't exist
    await execAsync('mkdir -p test/fixtures').catch(() => {});
    console.log('✅ Test fixtures directory ready');
    
  } catch (error: any) {
    console.error('❌ Global setup warning:', error.message);
    // Don't exit on setup warnings for unit tests
  }
  
  console.log('🚀 Global test environment ready\n');
}