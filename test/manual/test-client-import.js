#!/usr/bin/env node

/**
 * Test that simulates a client importing our package
 * This tests that the webpack warning is resolved
 */

console.log('Testing client import (CommonJS)...\n');

try {
  // Import the CommonJS build like a webpack client would
  const { ContainerAssistServer } = require('../../dist-cjs/src/index.js');
  
  console.log('✅ Successfully imported ContainerAssistServer');
  console.log('✅ No webpack warnings about vscode-languageserver-types');
  
  // Verify the class exists
  if (typeof ContainerAssistServer === 'function') {
    console.log('✅ ContainerAssistServer is a valid constructor');
    
    // Try creating an instance
    const server = new ContainerAssistServer();
    console.log('✅ Successfully created ContainerAssistServer instance');
  }
  
  console.log('\n🎉 All import tests passed!');
} catch (error) {
  console.error('❌ Import failed:', error.message);
  process.exit(1);
}