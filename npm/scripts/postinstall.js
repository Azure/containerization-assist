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

// Helper function to find node_modules directory
function findNodeModules() {
  let dir = __dirname;
  
  // Walk up the directory tree looking for node_modules
  while (dir !== path.parse(dir).root) {
    const nodeModulesPath = path.join(dir, 'node_modules');
    if (fs.existsSync(nodeModulesPath)) {
      // Check if we're inside node_modules
      if (dir.includes('node_modules')) {
        // We're inside node_modules, go up to find the parent node_modules
        const parts = dir.split(path.sep);
        const nodeModulesIndex = parts.lastIndexOf('node_modules');
        return parts.slice(0, nodeModulesIndex + 1).join(path.sep);
      }
      return nodeModulesPath;
    }
    dir = path.dirname(dir);
  }
  
  // Fallback to expected location
  return path.join(__dirname, '..', '..', '..');
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

// Find the node_modules directory
const nodeModulesDir = findNodeModules();
log(`Checking for platform package in: ${nodeModulesDir}`, 'cyan');

// Try to find the binary from the platform-specific package
const platformPackageDir = path.join(nodeModulesDir, platformPackage);
let binaryPath = path.join(platformPackageDir, 'bin', platformDir, binaryName);

// Create symlink/copy in main package bin directory
const mainPackageDir = path.join(__dirname, '..');
const mainBinDir = path.join(mainPackageDir, 'bin');
const linkPath = path.join(mainBinDir, binaryName);

// Debug output
log(`Platform package: ${platformPackage}`, 'cyan');
log(`Binary path: ${binaryPath}`, 'cyan');

// Check if binary exists
if (!fs.existsSync(binaryPath)) {
  log(`⚠ Platform-specific package not installed: ${platformPackage}`, 'yellow');
  log(`  Binary not found at: ${binaryPath}`, 'yellow');
  
  // Try alternative paths - including direct node_modules path without @thgamble scope
  const altPaths = [
    // Direct in node_modules (for npm flat structure)
    path.join(nodeModulesDir, `containerization-assist-mcp-${platformDir}`, 'bin', platformDir, binaryName),
    // Check if platform package is a sibling (for local development)
    path.join(nodeModulesDir, '..', platformPackage, 'bin', platformDir, binaryName),
    // Check scoped package location
    path.join(nodeModulesDir, '@thgamble', `containerization-assist-mcp-${platformDir}`, 'bin', platformDir, binaryName),
    // Check parent node_modules (when installed globally)
    path.join(mainPackageDir, 'node_modules', platformPackage, 'bin', platformDir, binaryName),
    // Check without full path structure
    path.join(platformPackageDir, binaryName),
    // Check direct bin folder
    path.join(platformPackageDir, 'bin', binaryName),
  ];
  
  let foundPath = null;
  for (const altPath of altPaths) {
    log(`  Checking: ${altPath}`, 'cyan');
    if (fs.existsSync(altPath)) {
      foundPath = altPath;
      log(`  ✓ Found binary at: ${altPath}`, 'green');
      break;
    }
  }
  
  if (!foundPath) {
    // List available packages for debugging
    log('\n  Checking for installed platform packages...', 'cyan');
    try {
      // Check main package node_modules
      const mainNodeModules = path.join(mainPackageDir, 'node_modules');
      if (fs.existsSync(mainNodeModules)) {
        log(`  Checking in: ${mainNodeModules}`, 'cyan');
        const packages = fs.readdirSync(mainNodeModules)
          .filter(dir => dir.includes('containerization-assist-mcp'));
        
        // Also check scoped packages
        const scopedPath = path.join(mainNodeModules, '@thgamble');
        if (fs.existsSync(scopedPath)) {
          const scopedPackages = fs.readdirSync(scopedPath)
            .filter(dir => dir.includes('containerization-assist-mcp'))
            .map(dir => `@thgamble/${dir}`);
          packages.push(...scopedPackages);
        }
        
        if (packages.length > 0) {
          log('  Found packages in main node_modules:', 'cyan');
          packages.forEach(pkg => {
            log(`    - ${pkg}`, 'yellow');
            // Try to find binary in each package
            const pkgBinPath = path.join(mainNodeModules, pkg, 'bin');
            if (fs.existsSync(pkgBinPath)) {
              log(`      Has bin directory`, 'green');
              const binContents = fs.readdirSync(pkgBinPath);
              binContents.forEach(item => log(`        - ${item}`, 'cyan'));
            }
          });
        }
      }
      
      // Also check global node_modules
      if (fs.existsSync(nodeModulesDir)) {
        log(`  Checking in: ${nodeModulesDir}`, 'cyan');
        const packages = fs.readdirSync(nodeModulesDir)
          .filter(dir => dir.includes('containerization-assist-mcp'));
        
        // Also check scoped packages
        const scopedPath = path.join(nodeModulesDir, '@thgamble');
        if (fs.existsSync(scopedPath)) {
          const scopedPackages = fs.readdirSync(scopedPath)
            .filter(dir => dir.includes('containerization-assist-mcp'))
            .map(dir => `@thgamble/${dir}`);
          packages.push(...scopedPackages);
        }
        
        if (packages.length > 0) {
          log('  Found packages in global node_modules:', 'cyan');
          packages.forEach(pkg => log(`    - ${pkg}`, 'yellow'));
        }
      }
    } catch (err) {
      log(`  Error listing packages: ${err.message}`, 'yellow');
    }
    
    // Don't fail installation, just warn
    log('\n  The binary will not be available until the platform package is installed.', 'yellow');
    log(`  You can install it manually with: npm install ${platformPackage}`, 'yellow');
    process.exit(0);
  } else {
    // Use the found alternative path
    binaryPath = foundPath;
  }
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
    log(`✓ Copied binary to: bin/${binaryName}`, 'green');
    
    // Make sure it's executable on Windows too
    try {
      fs.chmodSync(linkPath, 0o755);
    } catch (err) {
      // Windows might not support chmod, that's okay
    }
  } else {
    // Create symlink for Unix-like systems (use absolute path for reliability)
    fs.symlinkSync(binaryPath, linkPath);
    log(`✓ Created symlink: bin/${binaryName} -> ${binaryPath}`, 'green');
  }
  
  // Verify the link/copy works
  if (!fs.existsSync(linkPath)) {
    throw new Error('Failed to create binary link/copy');
  }
  
  // Check file size to ensure it's not empty
  const stats = fs.statSync(linkPath);
  if (stats.size === 0) {
    throw new Error('Binary file is empty');
  }
  
  log(`✓ Binary size: ${(stats.size / 1024 / 1024).toFixed(2)} MB`, 'green');
  
} catch (err) {
  log(`✗ Could not create binary link: ${err.message}`, 'red');
  log(`  Manual fix: Copy ${binaryPath} to ${linkPath}`, 'yellow');
  
  // Try to provide more specific help for Windows
  if (platform === 'win32') {
    log(`  On Windows, you may need to:`, 'yellow');
    log(`    1. Run as Administrator`, 'yellow');
    log(`    2. Or manually copy the file`, 'yellow');
  }
  
  process.exit(1);
}

// Success message
console.log('');
log('╔═══════════════════════════════════════════════════════════╗', 'bright');
log('║  Containerization Assist MCP Server ready! 🚀              ║', 'bright');
log('╚═══════════════════════════════════════════════════════════╝', 'bright');
console.log('');
log(`  Platform:     ${platform} (${arch})`, 'cyan');
log(`  Package:      ${platformPackage}`, 'cyan');
log(`  Binary:       bin/${binaryName}`, 'cyan');
console.log('');
log('  Usage:', 'green');
log('    npx containerization-assist-mcp --version', 'yellow');
log('    npx ckmcp --version', 'yellow');
console.log('');
log('  Documentation: https://github.com/Azure/containerization-assist', 'cyan');
console.log('');