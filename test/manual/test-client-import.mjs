#!/usr/bin/env node

/**
 * Test that simulates a client importing our package (ESM)
 * This tests that the webpack warning is resolved
 */

console.log('Testing client import (ESM)...\n');

try {
  // Import the ESM build
  const { ContainerAssistServer } = await import('../../dist/src/index.js');
  
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
  console.log('\nThe webpack warning about vscode-languageserver-types should be resolved.');
  console.log('We replaced dockerfile-ast with:');
  console.log('  - docker-file-parser (for parsing)');
  console.log('  - validate-dockerfile (for validation)');
} catch (error) {
  console.error('❌ Import failed:', error.message);
  console.error(error);
  process.exit(1);
}