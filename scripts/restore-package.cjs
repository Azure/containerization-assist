#!/usr/bin/env node

/**
 * Script to restore package.json after CommonJS-only publishing
 * This is run automatically by npm after the publish process
 */

const fs = require('fs');
const path = require('path');

const packagePath = path.join(__dirname, '..', 'package.json');
const backupPath = path.join(__dirname, '..', '.package.json.backup');

// Only restore if backup exists
if (fs.existsSync(backupPath)) {
  console.log('📦 Restoring original package.json...');
  
  // Restore from backup
  const backup = fs.readFileSync(backupPath, 'utf8');
  fs.writeFileSync(packagePath, backup);
  
  // Remove backup file
  fs.unlinkSync(backupPath);
  
  console.log('✅ package.json restored');
} else {
  console.log('No backup found, skipping restore');
}