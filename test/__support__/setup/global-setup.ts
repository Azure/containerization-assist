import { exec } from 'child_process';
import { promisify } from 'util';

const execAsync = promisify(exec);

export default async function globalSetup() {
  console.log('🏗️  Setting up global test environment...');

  try {
    // Create test fixtures directory if it doesn't exist
    const mkdirProcess = await execAsync('mkdir -p test/fixtures').catch(() => {});
    // Ensure child process is fully cleaned up
    if (mkdirProcess?.child) {
      mkdirProcess.child.unref();
    }
    console.log('✅ Test fixtures directory ready');

    // Verify Kubernetes tools if needed
    if (process.env.TEST_K8S) {
      try {
        const kubectlProcess = await execAsync('kubectl version --client');
        // Ensure child process is fully cleaned up
        if (kubectlProcess?.child) {
          kubectlProcess.child.unref();
        }
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