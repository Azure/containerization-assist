import { exec } from 'child_process';
import { promisify } from 'util';

const execAsync = promisify(exec);

export default async function globalSetup() {
  console.log('🏗️  Setting up global test environment...');
  
  try {
    // Verify Docker is available and working
    try {
      await execAsync('docker --version');
      await execAsync('docker info');
      console.log('✅ Docker is available');
      
      // Clean up any leftover test resources from previous runs
      try {
        const { stdout } = await execAsync('docker images --format "{{.Repository}}:{{.Tag}}" | grep "^test-" || true');
        if (stdout.trim()) {
          const images = stdout.trim().split('\n').filter(img => img.length > 0);
          let cleanedCount = 0;
          for (const image of images) {
            try {
              await execAsync(`docker rmi "${image}" -f`);
              cleanedCount++;
            } catch (error) {
              // Ignore cleanup errors
            }
          }
          if (cleanedCount > 0) {
            console.log(`🧹 Pre-cleaned ${cleanedCount} leftover test images`);
          }
        }
      } catch (error) {
        // Ignore pre-cleanup errors
      }
      
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