import { exec } from 'child_process';
import { promisify } from 'util';

const execAsync = promisify(exec);

export default async function globalSetup() {
  console.log('🏗️  Setting up global test environment...');
  
  try {
    // Create test fixtures directory if it doesn't exist
    await execAsync('mkdir -p test/fixtures').catch(() => {});
    console.log('✅ Test fixtures directory ready');
    
    // Verify Kubernetes tools if needed
    if (process.env.TEST_K8S) {
      try {
        await execAsync('kubectl version --client');
        console.log('✅ Kubernetes tools available');
      } catch (error) {
        console.log('⚠️  Kubernetes tools not available - some tests may be skipped');
      }
    }
    
  } catch (error: any) {
    console.error('❌ Global setup warning:', error.message);
  }
  
  console.log('🚀 Global test environment ready\n');
}