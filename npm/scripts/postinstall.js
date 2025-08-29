#!/usr/bin/env node

import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const { platform, arch } = process;

// ES module equivalent of __dirname
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// ANSI color codes for terminal output
const colors = {
  reset: '\x1b[0m',
  bright: '\x1b[1m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  red: '\x1b[31m',
  cyan: '\x1b[36m'
};

// Helper function for colored output
function log(message, color = 'reset') {
  console.log(`${colors[color]}${message}${colors.reset}`);
}

// Platform mapping for package names
const platformMap = {
  'darwin': 'darwin',
  'linux': 'linux',
  'win32': 'win32'
};

// Architecture mapping
const archMap = {
  'x64': 'x64',
  'arm64': 'arm64',
  'arm': 'arm64', // Map arm to arm64 as fallback
};

// Get platform and architecture
const mappedPlatform = platformMap[platform];
const mappedArch = archMap[arch] || arch;

if (!mappedPlatform) {
  log(`✗ Unsupported platform: ${platform}`, 'red');
  log(`  Supported platforms: macOS, Linux, Windows`, 'yellow');
  process.exit(1);
}

// Construct platform package name and binary name
const platformDir = `${mappedPlatform}-${mappedArch}`;
const platformPackage = `@thgamble/containerization-assist-mcp-${platformDir}`;
let binaryName = 'containerization-assist-mcp';
if (platform === 'win32') {
  binaryName += '.exe';
}

// Try to find the binary from the platform-specific package
const nodeModulesDir = path.join(__dirname, '..', '..'); 
const platformPackageDir = path.join(nodeModulesDir, platformPackage);
const binaryPath = path.join(platformPackageDir, 'bin', platformDir, binaryName);

// Create symlink in main package
const mainBinDir = path.join(__dirname, '..', 'bin');
const linkPath = path.join(mainBinDir, binaryName);

// Check if binary exists
if (!fs.existsSync(binaryPath)) {
  log(`⚠ Platform-specific package not installed: ${platformPackage}`, 'yellow');
  log(`  This is normal if you're developing or if the package is optional.`, 'yellow');
  log(`  Binary path checked: ${binaryPath}`, 'yellow');
  
  // Try to find any installed platform package
  log('\n  Checking for other platform packages...', 'cyan');
  try {
    const packages = fs.readdirSync(nodeModulesDir)
      .filter(dir => dir.startsWith('@thgamble') && dir.includes('containerization-assist-mcp-'));
    if (packages.length > 0) {
      log('  Found platform packages:', 'cyan');
      packages.forEach(pkg => log(`    - ${pkg}`, 'yellow'));
    } else {
      log('  No platform packages found.', 'yellow');
      log(`  Install manually: npm install ${platformPackage}`, 'yellow');
    }
  } catch (err) {
    // Ignore errors when listing
  }
  
  // Don't fail installation, just warn
  process.exit(0);
}

// Create bin directory if it doesn't exist
if (!fs.existsSync(mainBinDir)) {
  fs.mkdirSync(mainBinDir, { recursive: true });
}

// Make binary executable (Unix-like systems)
if (platform !== 'win32') {
  try {
    fs.chmodSync(binaryPath, 0o755);
    log(`✓ Made binary executable: ${binaryName}`, 'green');
  } catch (err) {
    log(`⚠ Could not make binary executable: ${err.message}`, 'yellow');
  }
}

// Create or update symlink/copy
try {
  // Remove existing link/file if it exists
  if (fs.existsSync(linkPath)) {
    fs.unlinkSync(linkPath);
  }

  // On Windows, copy the file instead of symlinking
  if (platform === 'win32') {
    fs.copyFileSync(binaryPath, linkPath);
    log(`✓ Copied binary from ${platformPackage}`, 'green');
  } else {
    // Create symlink for Unix-like systems
    fs.symlinkSync(binaryPath, linkPath);
    log(`✓ Created symlink to ${platformPackage}`, 'green');
  }
} catch (err) {
  log(`⚠ Could not create link: ${err.message}`, 'yellow');
  log(`  You can still use the binary directly from: ${binaryPath}`, 'yellow');
}

// Success message
console.log('');
log('╔═══════════════════════════════════════════════════════════╗', 'bright');
log('║  Containerization Assist MCP Server ready! 🚀              ║', 'bright');
log('╚═══════════════════════════════════════════════════════════╝', 'bright');
console.log('');
log(`  Platform:     ${platform} (${arch})`, 'cyan');
log(`  Package:      ${platformPackage}`, 'cyan');
log(`  Binary:       ${binaryName}`, 'cyan');
console.log('');
log('  Usage:', 'green');
log('    npx containerization-assist-mcp --help', 'yellow');
log('    npx ckmcp --version', 'yellow');
console.log('');
log('  Documentation: https://github.com/Azure/containerization-assist', 'cyan');
console.log('');