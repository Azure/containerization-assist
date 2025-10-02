/**
 * Containerization Assist MCP Server - Direct Entry Point
 * Uses the simplified app architecture with bootstrap helper
 */

import { bootstrap } from './bootstrap';
import { createLogger } from '@/lib/logger';
import { readFileSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

// Load package.json for version info
const packageJsonPath = __dirname.includes('dist')
  ? join(__dirname, '../../../package.json') // dist/src/cli/ -> root
  : join(__dirname, '../../package.json'); // src/cli/ -> root

const packageJson = JSON.parse(readFileSync(packageJsonPath, 'utf-8'));

async function main(): Promise<void> {
  const logger = createLogger({
    name: 'mcp-server',
    level: process.env.LOG_LEVEL || 'info',
  });

  try {
    await bootstrap({
      appName: 'containerization-assist-mcp',
      version: packageJson.version,
      logger,
      policyPath: process.env.POLICY_PATH || 'config/policy.yaml',
      policyEnvironment: process.env.NODE_ENV || 'production',
      quiet: !!process.env.MCP_QUIET,
    });

    // Bootstrap handles:
    // - MCP_MODE setup
    // - App creation and server startup
    // - Shutdown handler installation (SIGTERM, SIGINT, uncaught errors)
    // - Startup/shutdown logging
  } catch (error) {
    logger.fatal({ error }, 'Failed to start server');
    process.exit(1);
  }
}

// Run the server
void main();
