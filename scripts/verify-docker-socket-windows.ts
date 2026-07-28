/**
 * Windows Docker Socket Path Verification
 *
 * Verifies that socket path detection logic returns the correct Windows named
 * pipe (//./pipe/docker_engine). Connectivity to the Docker daemon is
 * best-effort: on GitHub-hosted `windows-latest` runners the Docker Engine
 * service is not always running, so a failed ping is reported as a warning
 * rather than a hard failure. This script's contract is: the code selects the
 * correct socket path on Windows.
 *
 * Usage:
 *   npm run build
 *   tsx scripts/verify-docker-socket-windows.ts
 */

import { createLogger } from '../dist/src/lib/logger.js';
import { createDockerClient } from '../dist/src/infra/docker/client.js';
import { autoDetectDockerSocket } from '../dist/src/infra/docker/socket-validation.js';

const logger = createLogger({ name: 'docker-socket-verify', level: 'info' });

const EXPECTED_SOCKET = '//./pipe/docker_engine';

async function main() {
  console.log('🔍 Verifying Docker Socket Path on Windows\n');
  console.log('='.repeat(60));

  console.log('\n📋 Step 1: Checking Docker socket path detection...\n');
  console.log(`   Expected socket path: ${EXPECTED_SOCKET}`);

  const detected = autoDetectDockerSocket();
  console.log(`   Detected socket path: ${detected}`);

  if (detected !== EXPECTED_SOCKET) {
    console.error(
      `\n❌ FAILED: Detected socket path (${detected}) does not match expected (${EXPECTED_SOCKET}).`,
    );
    process.exit(1);
  }
  console.log('   ✅ Socket path detection is correct');

  // Best-effort connectivity check. Windows GitHub runners frequently do not
  // have the Docker Engine service running; treat failure as a warning.
  console.log('\n📋 Step 2: Testing Docker daemon connectivity (best-effort)...\n');
  try {
    const dockerClient = createDockerClient(logger);
    const pingResult = await dockerClient.ping();

    if (pingResult.ok) {
      console.log('   ✅ Docker daemon is accessible');
    } else {
      console.warn('   ⚠️  Docker daemon ping failed (non-fatal):', pingResult.error);
      console.warn('       Path detection succeeded; daemon is simply not running.');
    }
  } catch (error) {
    console.warn(
      '   ⚠️  Docker connectivity check threw (non-fatal):',
      error instanceof Error ? error.message : 'Unknown error',
    );
  }

  console.log('\n' + '='.repeat(60));
  console.log('✅ SUCCESS: Docker socket path is correct!');
  process.exit(0);
}

main().catch((error) => {
  console.error('❌ Verification script failed:', error);
  process.exit(1);
});
