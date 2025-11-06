#!/usr/bin/env tsx
/**
 * Toggle MCP Configuration Script
 * Switches between local development and packaged testing modes
 *
 * Usage:
 *   npm run mcp:local    - Switch to local development mode
 *   npm run mcp:packaged - Switch to packaged mode (builds tarball)
 *   npm run mcp:status   - Show current mode
 */

import { readFileSync, writeFileSync, existsSync, unlinkSync, readdirSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { execSync } from 'node:child_process';

const MCP_CONFIG_PATH = resolve(process.cwd(), '.vscode/mcp.json');
const PACKAGE_JSON_PATH = resolve(process.cwd(), 'package.json');

interface MCPServerConfig {
  command: string;
  args: string[];
  env?: Record<string, string>;
}

interface MCPConfig {
  servers: {
    [key: string]: MCPServerConfig;
  };
}

interface PackageJson {
  name: string;
  version: string;
}

function readMCPConfig(): MCPConfig {
  if (!existsSync(MCP_CONFIG_PATH)) {
    console.error('❌ .vscode/mcp.json not found');
    process.exit(1);
  }
  return JSON.parse(readFileSync(MCP_CONFIG_PATH, 'utf-8'));
}

function writeMCPConfig(config: MCPConfig): void {
  writeFileSync(MCP_CONFIG_PATH, JSON.stringify(config, null, 2) + '\n');
}

function readPackageJson(): PackageJson {
  return JSON.parse(readFileSync(PACKAGE_JSON_PATH, 'utf-8'));
}

function getCurrentMode(config: MCPConfig): 'local' | 'packaged' | 'unknown' {
  const server = config.servers['containerization-assist-dev'];
  if (!server) return 'unknown';

  if (server.command === 'npx' && server.args[0] === 'tsx') {
    return 'local';
  }

  if (server.command === 'node' && server.args[0]?.includes('dist/src/cli/cli.js')) {
    return 'packaged';
  }

  return 'unknown';
}

function buildAndPack(): string {
  console.log('🔨 Building project...');
  execSync('npm run build', { stdio: 'inherit' });

  console.log('📦 Creating package tarball...');

  // Clean up any existing tarballs
  const existingTarballs = readdirSync(process.cwd())
    .filter(file => file.startsWith('containerization-assist-mcp-') && file.endsWith('.tgz'));
  existingTarballs.forEach(tarball => {
    console.log(`🗑️  Removing old tarball: ${tarball}`);
    unlinkSync(join(process.cwd(), tarball));
  });

  const output = execSync('npm pack', { encoding: 'utf-8' });
  const tarballName = output.trim().split('\n').pop()?.trim();

  if (!tarballName) {
    console.error('❌ Failed to create tarball');
    process.exit(1);
  }

  const tarballPath = resolve(process.cwd(), tarballName);
  console.log(`✅ Created: ${tarballName}`);

  return tarballPath;
}

function switchToLocal(): void {
  console.log('🔄 Switching to LOCAL development mode...\n');

  const config = readMCPConfig();
  const currentMode = getCurrentMode(config);

  if (currentMode === 'local') {
    console.log('✅ Already in LOCAL mode');
    showCurrentConfig(config);
    return;
  }

  // Preserve environment variables
  const server = config.servers['containerization-assist-dev'];
  const env = server?.env || {
    MCP_MODE: 'true',
    MCP_QUIET: 'true',
    NODE_ENV: 'development',
  };

  config.servers['containerization-assist-dev'] = {
    command: 'npx',
    args: ['tsx', './src/cli/cli.ts'],
    env,
  };

  writeMCPConfig(config);
  console.log('✅ Switched to LOCAL mode');
  showCurrentConfig(config);
}

function switchToPackaged(): void {
  console.log('🔄 Switching to PACKAGED testing mode...\n');

  const config = readMCPConfig();
  const currentMode = getCurrentMode(config);

  if (currentMode === 'packaged') {
    console.log('ℹ️  Already in PACKAGED mode, rebuilding...\n');
  }

  const tarballPath = buildAndPack();

  // Preserve environment variables
  const server = config.servers['containerization-assist-dev'];
  const env = server?.env || {
    MCP_MODE: 'true',
    MCP_QUIET: 'true',
  };

  config.servers['containerization-assist-dev'] = {
    command: 'node',
    args: [resolve(process.cwd(), 'dist/src/cli/cli.js')],
    env,
  };

  writeMCPConfig(config);
  console.log(`✅ Switched to PACKAGED mode`);
  console.log(`📦 Tarball available: ${tarballPath}`);
  console.log(`\nℹ️  Note: Using built dist/ directly (simulates installed package structure)`);
  showCurrentConfig(config);
}

function showStatus(): void {
  const config = readMCPConfig();
  const mode = getCurrentMode(config);
  const pkg = readPackageJson();

  console.log('📊 MCP Configuration Status\n');
  console.log(`Package: ${pkg.name}@${pkg.version}`);
  console.log(`Current Mode: ${mode.toUpperCase()}\n`);

  showCurrentConfig(config);
}

function showCurrentConfig(config: MCPConfig): void {
  const server = config.servers['containerization-assist-dev'];

  console.log('\n📋 Current Configuration:');
  console.log(`  Command: ${server.command}`);
  console.log(`  Args: ${server.args.join(' ')}`);

  if (server.env) {
    console.log('\n  Environment:');
    Object.entries(server.env).forEach(([key, value]) => {
      console.log(`    ${key}=${value}`);
    });
  }

  console.log('\n💡 Next Steps:');
  console.log('  1. Restart your MCP client (VS Code, Claude Desktop, etc.)');
  console.log('  2. Check the MCP server logs for policy discovery messages');
  console.log('  3. Look for "Discovered built-in policies" with searchPaths');
}

// Main CLI
const command = process.argv[2];

switch (command) {
  case 'local':
    switchToLocal();
    break;
  case 'packaged':
    switchToPackaged();
    break;
  case 'status':
    showStatus();
    break;
  default:
    console.log('Usage:');
    console.log('  npm run mcp:local    - Switch to local development mode');
    console.log('  npm run mcp:packaged - Switch to packaged mode');
    console.log('  npm run mcp:status   - Show current mode');
    process.exit(1);
}
