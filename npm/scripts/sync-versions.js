#!/usr/bin/env node

import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Read main package.json
const packagePath = path.join(__dirname, '..', 'package.json');
const packageJson = JSON.parse(fs.readFileSync(packagePath, 'utf8'));
const version = packageJson.version;

console.log(`Syncing version ${version} across all packages...`);

// Update optionalDependencies versions in main package.json
const platforms = [
  'darwin-x64',
  'darwin-arm64',
  'linux-x64',
  'linux-arm64',
  'win32-x64',
  'win32-arm64'
];

// Update optionalDependencies
if (packageJson.optionalDependencies) {
  platforms.forEach(platform => {
    const depName = `@thgamble/containerization-assist-mcp-${platform}`;
    if (packageJson.optionalDependencies[depName]) {
      packageJson.optionalDependencies[depName] = version;
    }
  });
  
  // Write back the updated package.json
  fs.writeFileSync(packagePath, JSON.stringify(packageJson, null, 2) + '\n');
  console.log('✓ Updated optionalDependencies in package.json');
}

console.log(`✓ Version sync complete: ${version}`);